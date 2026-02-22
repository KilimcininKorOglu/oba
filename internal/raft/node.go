package raft

import (
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// Logger interface for Raft logging
type Logger interface {
	Debug(msg string, args ...interface{})
	Info(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Error(msg string, args ...interface{})
}

// defaultLogger is a no-op logger
type defaultLogger struct{}

func (l *defaultLogger) Debug(msg string, args ...interface{}) {}
func (l *defaultLogger) Info(msg string, args ...interface{})  {}
func (l *defaultLogger) Warn(msg string, args ...interface{})  {}
func (l *defaultLogger) Error(msg string, args ...interface{}) {}

// StateMachine defines the interface for applying Raft commands.
type StateMachine interface {
	// Apply applies a command to the state machine.
	Apply(cmd *Command) error

	// Snapshot creates a snapshot of the current state.
	Snapshot() ([]byte, error)

	// Restore restores state from a snapshot.
	Restore(data []byte) error
}

// Node represents a Raft node in the cluster.
type Node struct {
	// Configuration
	id     uint64
	config *NodeConfig

	// State
	state *NodeState

	// Cluster
	peers map[uint64]*Peer

	// Components
	transport    Transport
	stateMachine StateMachine
	logger       Logger

	// Channels
	applyCh   chan *LogEntry // Committed entries to apply
	proposeCh chan *proposeRequest
	stopCh    chan struct{}

	// Pending proposals waiting for commit
	pendingMu       sync.Mutex
	pendingProposals map[uint64]*proposeRequest

	// Timers
	electionTimer  *time.Timer
	heartbeatTimer *time.Timer

	// Status
	running int32

	mu sync.RWMutex
}

type proposeRequest struct {
	cmd    *Command
	result chan error
	index  uint64 // Log index of the proposed command
}

// NewNode creates a new Raft node.
func NewNode(cfg *NodeConfig, sm StateMachine, transport Transport) (*Node, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	n := &Node{
		id:               cfg.ID,
		config:           cfg,
		state:            NewNodeState(),
		peers:            make(map[uint64]*Peer),
		transport:        transport,
		stateMachine:     sm,
		logger:           &defaultLogger{},
		applyCh:          make(chan *LogEntry, 256),
		proposeCh:        make(chan *proposeRequest, 256),
		stopCh:           make(chan struct{}),
		pendingProposals: make(map[uint64]*proposeRequest),
	}

	// Add peers
	for _, p := range cfg.Peers {
		if p.ID != cfg.ID {
			n.peers[p.ID] = p
		}
	}

	return n, nil
}

// SetLogger sets the logger for the node.
func (n *Node) SetLogger(logger Logger) {
	n.logger = logger
}

// ID returns the node's ID.
func (n *Node) ID() uint64 {
	return n.id
}

// State returns the current state (Follower, Candidate, Leader).
func (n *Node) State() uint8 {
	return n.state.State()
}

// IsLeader returns true if this node is the leader.
func (n *Node) IsLeader() bool {
	return n.state.IsLeader()
}

// Term returns the current term.
func (n *Node) Term() uint64 {
	return n.state.CurrentTerm()
}

// LeaderID returns the current leader's ID (0 if unknown).
func (n *Node) LeaderID() uint64 {
	return n.state.LeaderID()
}

// CommitIndex returns the commit index.
func (n *Node) CommitIndex() uint64 {
	return n.state.CommitIndex()
}

// LastApplied returns the last applied index.
func (n *Node) LastApplied() uint64 {
	return n.state.LastApplied()
}

// Start starts the Raft node.
func (n *Node) Start() error {
	if !atomic.CompareAndSwapInt32(&n.running, 0, 1) {
		return nil // Already running
	}

	// Start transport listener
	if n.transport != nil {
		if err := n.transport.Listen(n.handleRPC); err != nil {
			atomic.StoreInt32(&n.running, 0)
			return err
		}
	}

	// Start main loop
	go n.run()

	// Start apply loop
	go n.applyLoop()

	return nil
}

// Stop stops the Raft node.
func (n *Node) Stop() {
	if !atomic.CompareAndSwapInt32(&n.running, 1, 0) {
		return // Not running
	}

	close(n.stopCh)
	n.transport.Close()
}

// Propose proposes a new command to be replicated.
// Only the leader can accept proposals.
func (n *Node) Propose(cmd *Command) error {
	if !n.IsLeader() {
		return ErrNotLeader
	}

	req := &proposeRequest{
		cmd:    cmd,
		result: make(chan error, 1),
	}

	select {
	case n.proposeCh <- req:
	case <-n.stopCh:
		return ErrNodeStopped
	}

	select {
	case err := <-req.result:
		return err
	case <-n.stopCh:
		return ErrNodeStopped
	}
}

// run is the main loop for the Raft node.
func (n *Node) run() {
	n.resetElectionTimer()

	for {
		select {
		case <-n.stopCh:
			return
		default:
		}

		switch n.State() {
		case StateFollower:
			n.runFollower()
		case StateCandidate:
			n.runCandidate()
		case StateLeader:
			n.runLeader()
		}
	}
}

func (n *Node) runFollower() {
	for n.State() == StateFollower {
		select {
		case <-n.stopCh:
			return
		case <-n.electionTimer.C:
			n.state.BecomeCandidate()
			n.state.SetVotedFor(n.id)
			return
		}
	}
}

func (n *Node) runCandidate() {
	term := n.state.CurrentTerm()
	lastLogIndex := n.state.Log().LastIndex()
	lastLogTerm := n.state.Log().LastTerm()

	// Single node cluster - become leader immediately
	if len(n.peers) == 0 {
		n.becomeLeader()
		return
	}

	// Request votes from all peers
	votes := int32(1) // Vote for self
	voteCh := make(chan bool, len(n.peers))

	for peerID := range n.peers {
		go func(peerID uint64) {
			args := &RequestVoteArgs{
				Term:         term,
				CandidateID:  n.id,
				LastLogIndex: lastLogIndex,
				LastLogTerm:  lastLogTerm,
			}

			reply, err := n.sendRequestVote(peerID, args)
			if err != nil {
				voteCh <- false
				return
			}

			if reply.Term > term {
				n.state.BecomeFollower(reply.Term)
				voteCh <- false
				return
			}

			voteCh <- reply.VoteGranted
		}(peerID)
	}

	// Wait for votes with timeout
	n.resetElectionTimer()
	votesNeeded := (len(n.peers)+1)/2 + 1

	for i := 0; i < len(n.peers); i++ {
		select {
		case <-n.stopCh:
			return
		case <-n.electionTimer.C:
			// Election timeout - restart election with new term
			n.state.BecomeCandidate()
			n.state.SetVotedFor(n.id)
			return
		case granted := <-voteCh:
			if n.State() != StateCandidate {
				return // State changed
			}
			if granted {
				currentVotes := int(atomic.AddInt32(&votes, 1))
				if currentVotes >= votesNeeded {
					n.becomeLeader()
					return
				}
			}
		}
	}

	// Didn't get enough votes, restart election with new term
	n.state.BecomeCandidate()
	n.state.SetVotedFor(n.id)
}

func (n *Node) runLeader() {
	// Send initial heartbeat
	n.broadcastAppendEntries()
	n.resetHeartbeatTimer()

	for n.State() == StateLeader {
		select {
		case <-n.stopCh:
			// Cancel all pending proposals
			n.cancelPendingProposals(ErrNodeStopped)
			return
		case <-n.heartbeatTimer.C:
			n.broadcastAppendEntries()
			n.resetHeartbeatTimer()
		case req := <-n.proposeCh:
			if n.State() != StateLeader {
				req.result <- ErrNotLeader
				continue
			}
			// Append command and track for commit notification
			index := n.appendCommandAndTrack(req)
			req.index = index
		}
	}
	// No longer leader - cancel pending proposals
	n.cancelPendingProposals(ErrNotLeader)
}

func (n *Node) becomeLeader() {
	n.logger.Info("became leader", "nodeId", n.id, "term", n.state.CurrentTerm())
	n.state.BecomeLeader(n.id)

	// Initialize leader state
	peers := make([]*Peer, 0, len(n.peers))
	for _, p := range n.peers {
		peers = append(peers, p)
	}
	n.state.InitLeaderState(peers)

	// Append noop entry to establish leadership
	entry := &LogEntry{
		Index: n.state.Log().LastIndex() + 1,
		Term:  n.state.CurrentTerm(),
		Type:  LogEntryNoop,
	}
	n.state.AppendEntry(entry)
}

func (n *Node) resetElectionTimer() {
	timeout := n.randomElectionTimeout()
	if n.electionTimer == nil {
		n.electionTimer = time.NewTimer(timeout)
	} else {
		// Stop the timer and drain the channel if needed
		if !n.electionTimer.Stop() {
			select {
			case <-n.electionTimer.C:
			default:
			}
		}
		n.electionTimer.Reset(timeout)
	}
}

func (n *Node) resetHeartbeatTimer() {
	if n.heartbeatTimer == nil {
		n.heartbeatTimer = time.NewTimer(n.config.HeartbeatTimeout)
	} else {
		// Stop the timer and drain the channel if needed
		if !n.heartbeatTimer.Stop() {
			select {
			case <-n.heartbeatTimer.C:
			default:
			}
		}
		n.heartbeatTimer.Reset(n.config.HeartbeatTimeout)
	}
}

func (n *Node) randomElectionTimeout() time.Duration {
	return n.config.ElectionTimeout + time.Duration(rand.Int63n(int64(n.config.ElectionTimeout)))
}

// handleRPC handles incoming RPC messages.
func (n *Node) handleRPC(msgType uint8, data []byte) []byte {
	switch msgType {
	case RPCRequestVote:
		return n.handleRequestVote(data)
	case RPCAppendEntries:
		return n.handleAppendEntries(data)
	case RPCInstallSnapshot:
		return n.handleInstallSnapshot(data)
	default:
		return nil
	}
}

func (n *Node) handleRequestVote(data []byte) []byte {
	args, err := DeserializeRequestVoteArgs(data)
	if err != nil {
		return (&RequestVoteReply{Term: n.Term()}).Serialize()
	}

	reply := &RequestVoteReply{Term: n.Term()}

	// Reply false if term < currentTerm
	if args.Term < n.Term() {
		return reply.Serialize()
	}

	// Update term if needed
	if args.Term > n.Term() {
		n.state.BecomeFollower(args.Term)
		reply.Term = args.Term
	}

	// Check if we can vote for this candidate
	votedFor := n.state.VotedFor()
	if votedFor == 0 || votedFor == args.CandidateID {
		// Check if candidate's log is at least as up-to-date as ours
		lastLogIndex := n.state.Log().LastIndex()
		lastLogTerm := n.state.Log().LastTerm()

		if args.LastLogTerm > lastLogTerm ||
			(args.LastLogTerm == lastLogTerm && args.LastLogIndex >= lastLogIndex) {
			n.state.SetVotedFor(args.CandidateID)
			reply.VoteGranted = true
			n.resetElectionTimer()
		}
	}

	return reply.Serialize()
}

func (n *Node) handleAppendEntries(data []byte) []byte {
	args, err := DeserializeAppendEntriesArgs(data)
	if err != nil {
		return (&AppendEntriesReply{Term: n.Term()}).Serialize()
	}

	reply := &AppendEntriesReply{Term: n.Term()}

	// Reply false if term < currentTerm
	if args.Term < n.Term() {
		return reply.Serialize()
	}

	// Update term if needed
	if args.Term > n.Term() {
		n.state.BecomeFollower(args.Term)
		reply.Term = args.Term
	} else if n.State() == StateCandidate {
		// Valid leader exists, step down
		n.state.BecomeFollower(args.Term)
	}

	// Reset election timer (valid leader)
	n.resetElectionTimer()
	n.state.SetLeaderID(args.LeaderID)

	// Check log consistency
	log := n.state.Log()
	if args.PrevLogIndex > 0 {
		if args.PrevLogIndex > log.LastIndex() {
			reply.ConflictIndex = log.LastIndex() + 1
			return reply.Serialize()
		}
		if log.TermAt(args.PrevLogIndex) != args.PrevLogTerm {
			reply.ConflictTerm = log.TermAt(args.PrevLogIndex)
			// Find first index of conflict term
			for i := args.PrevLogIndex; i > 0; i-- {
				if log.TermAt(i) != reply.ConflictTerm {
					reply.ConflictIndex = i + 1
					break
				}
				if i == 1 {
					reply.ConflictIndex = 1
				}
			}
			return reply.Serialize()
		}
	}

	// Append new entries
	for i, entry := range args.Entries {
		idx := args.PrevLogIndex + uint64(i) + 1
		if idx <= log.LastIndex() {
			if log.TermAt(idx) != entry.Term {
				// Conflict - truncate log
				log.TruncateFrom(idx)
				n.state.AppendEntry(entry)
			}
		} else {
			n.state.AppendEntry(entry)
		}
	}

	// Update commit index
	if args.LeaderCommit > n.state.CommitIndex() {
		newCommit := args.LeaderCommit
		if log.LastIndex() < newCommit {
			newCommit = log.LastIndex()
		}
		n.state.SetCommitIndex(newCommit)
	}

	reply.Success = true
	return reply.Serialize()
}

func (n *Node) handleInstallSnapshot(data []byte) []byte {
	args, err := DeserializeInstallSnapshotArgs(data)
	if err != nil {
		return (&InstallSnapshotReply{Term: n.Term()}).Serialize()
	}

	reply := &InstallSnapshotReply{Term: n.Term()}

	// Reply if term < currentTerm
	if args.Term < n.Term() {
		return reply.Serialize()
	}

	// Update term if needed
	if args.Term > n.Term() {
		n.state.BecomeFollower(args.Term)
		reply.Term = args.Term
	}

	n.resetElectionTimer()
	n.state.SetLeaderID(args.LeaderID)

	// Apply snapshot to state machine
	if n.stateMachine != nil {
		if err := n.stateMachine.Restore(args.Data); err != nil {
			n.logger.Error("failed to restore snapshot", "error", err)
			return reply.Serialize()
		}
		n.logger.Info("snapshot restored successfully", "lastIncludedIndex", args.LastIncludedIndex)
	}

	// Update state - set both commit and applied to snapshot's last index
	// This prevents re-applying old log entries
	n.state.SetCommitIndex(args.LastIncludedIndex)
	n.state.SetLastApplied(args.LastIncludedIndex)
	
	// Truncate log entries that are included in the snapshot
	n.state.Log().TruncateBefore(args.LastIncludedIndex + 1)

	return reply.Serialize()
}

func (n *Node) sendRequestVote(peerID uint64, args *RequestVoteArgs) (*RequestVoteReply, error) {
	data := args.Serialize()
	resp, err := n.transport.Send(peerID, RPCRequestVote, data)
	if err != nil {
		return nil, err
	}
	return DeserializeRequestVoteReply(resp)
}

func (n *Node) sendAppendEntries(peerID uint64, args *AppendEntriesArgs) (*AppendEntriesReply, error) {
	data := args.Serialize()
	resp, err := n.transport.Send(peerID, RPCAppendEntries, data)
	if err != nil {
		return nil, err
	}
	return DeserializeAppendEntriesReply(resp)
}

func (n *Node) sendInstallSnapshot(peerID uint64, args *InstallSnapshotArgs) (*InstallSnapshotReply, error) {
	data := args.Serialize()
	resp, err := n.transport.Send(peerID, RPCInstallSnapshot, data)
	if err != nil {
		return nil, err
	}
	return DeserializeInstallSnapshotReply(resp)
}

// broadcastAppendEntries sends AppendEntries to all peers.
func (n *Node) broadcastAppendEntries() {
	for peerID := range n.peers {
		go n.replicateTo(peerID)
	}
}

func (n *Node) replicateTo(peerID uint64) {
	if n.State() != StateLeader {
		return
	}

	nextIndex := n.state.GetNextIndex(peerID)
	prevLogIndex := nextIndex - 1
	prevLogTerm := n.state.Log().TermAt(prevLogIndex)

	// Get entries to send
	entries := n.state.Log().GetFrom(nextIndex)

	args := &AppendEntriesArgs{
		Term:         n.Term(),
		LeaderID:     n.id,
		PrevLogIndex: prevLogIndex,
		PrevLogTerm:  prevLogTerm,
		Entries:      entries,
		LeaderCommit: n.state.CommitIndex(),
	}

	reply, err := n.sendAppendEntries(peerID, args)
	if err != nil {
		return
	}

	if reply.Term > n.Term() {
		n.state.BecomeFollower(reply.Term)
		return
	}

	if reply.Success {
		n.state.SetNextIndex(peerID, nextIndex+uint64(len(entries)))
		n.state.SetMatchIndex(peerID, nextIndex+uint64(len(entries))-1)
		n.updateCommitIndex()
		n.notifyCommittedProposals()
	} else {
		// Decrement nextIndex and retry
		if reply.ConflictTerm > 0 {
			n.state.SetNextIndex(peerID, reply.ConflictIndex)
		} else {
			newNext := n.state.GetNextIndex(peerID)
			if newNext > 1 {
				n.state.SetNextIndex(peerID, newNext-1)
			}
		}
	}
}

// updateCommitIndex updates commitIndex based on matchIndex.
func (n *Node) updateCommitIndex() {
	log := n.state.Log()
	currentTerm := n.Term()

	// Single node cluster - commit immediately
	if len(n.peers) == 0 {
		for idx := log.LastIndex(); idx > n.state.CommitIndex(); idx-- {
			if log.TermAt(idx) == currentTerm {
				n.state.SetCommitIndex(idx)
				break
			}
		}
		return
	}

	// Find the highest index replicated on majority
	for idx := log.LastIndex(); idx > n.state.CommitIndex(); idx-- {
		if log.TermAt(idx) != currentTerm {
			continue
		}

		count := 1 // Self
		matchIndexes := n.state.GetMatchIndexes()
		for _, matchIdx := range matchIndexes {
			if matchIdx >= idx {
				count++
			}
		}

		if count > (len(n.peers)+1)/2 {
			n.state.SetCommitIndex(idx)
			break
		}
	}
}

// sendSnapshotTo sends a snapshot to a follower that is too far behind.
func (n *Node) sendSnapshotTo(peerID uint64) {
	if n.stateMachine == nil {
		n.logger.Error("cannot send snapshot: state machine is nil")
		return
	}

	// Create snapshot
	snapshotData, err := n.stateMachine.Snapshot()
	if err != nil {
		n.logger.Error("failed to create snapshot", "errorMsg", err.Error())
		return
	}

	if len(snapshotData) == 0 {
		n.logger.Error("snapshot data is empty")
		return
	}

	// Use commitIndex as LastIncludedIndex since snapshot represents committed state
	commitIndex := n.state.CommitIndex()
	log := n.state.Log()
	args := &InstallSnapshotArgs{
		Term:              n.Term(),
		LeaderID:          n.id,
		LastIncludedIndex: commitIndex,
		LastIncludedTerm:  log.TermAt(commitIndex),
		Data:              snapshotData,
	}

	n.logger.Info("sending snapshot to follower", "peer", peerID, "size", len(snapshotData))

	reply, err := n.sendInstallSnapshot(peerID, args)
	if err != nil {
		n.logger.Error("failed to send snapshot", "peer", peerID, "error", err)
		return
	}

	if reply.Term > n.Term() {
		n.state.BecomeFollower(reply.Term)
		return
	}

	// Update nextIndex and matchIndex after successful snapshot
	n.state.SetNextIndex(peerID, args.LastIncludedIndex+1)
	n.state.SetMatchIndex(peerID, args.LastIncludedIndex)
	n.logger.Info("snapshot sent successfully", "peer", peerID)
}

// appendCommand appends a new command to the log.
func (n *Node) appendCommand(cmd *Command) {
	if n.State() != StateLeader {
		return
	}

	data, _ := cmd.Serialize()
	entry := &LogEntry{
		Index:   n.state.Log().LastIndex() + 1,
		Term:    n.Term(),
		Type:    LogEntryCommand,
		Command: data,
	}

	n.state.AppendEntry(entry)

	// Update commit index (for single node, commits immediately)
	n.updateCommitIndex()

	// Replicate to peers
	n.broadcastAppendEntries()
}

// appendCommandAndTrack appends a command and tracks it for commit notification.
func (n *Node) appendCommandAndTrack(req *proposeRequest) uint64 {
	if n.State() != StateLeader {
		return 0
	}

	data, _ := req.cmd.Serialize()
	entry := &LogEntry{
		Index:   n.state.Log().LastIndex() + 1,
		Term:    n.Term(),
		Type:    LogEntryCommand,
		Command: data,
	}

	n.state.AppendEntry(entry)

	// Track this proposal for commit notification
	n.pendingMu.Lock()
	n.pendingProposals[entry.Index] = req
	n.pendingMu.Unlock()

	// Update commit index (for single node, commits immediately)
	n.updateCommitIndex()

	// Notify any proposals that are now committed
	n.notifyCommittedProposals()

	// Replicate to peers
	n.broadcastAppendEntries()

	return entry.Index
}

// notifyCommittedProposals notifies pending proposals that have been committed.
func (n *Node) notifyCommittedProposals() {
	commitIndex := n.state.CommitIndex()

	n.pendingMu.Lock()
	defer n.pendingMu.Unlock()

	for index, req := range n.pendingProposals {
		if index <= commitIndex {
			req.result <- nil
			delete(n.pendingProposals, index)
		}
	}
}

// cancelPendingProposals cancels all pending proposals with the given error.
func (n *Node) cancelPendingProposals(err error) {
	n.pendingMu.Lock()
	defer n.pendingMu.Unlock()

	for index, req := range n.pendingProposals {
		req.result <- err
		delete(n.pendingProposals, index)
	}
}

// applyLoop applies committed entries to the state machine.
func (n *Node) applyLoop() {
	for {
		select {
		case <-n.stopCh:
			return
		default:
		}

		commitIndex := n.state.CommitIndex()
		lastApplied := n.state.LastApplied()

		for lastApplied < commitIndex {
			lastApplied++
			entry, err := n.state.Log().Get(lastApplied)
			if err != nil {
				break
			}

			if entry.Type == LogEntryCommand && n.stateMachine != nil {
				cmd, err := DeserializeCommand(entry.Command)
				if err == nil {
					n.stateMachine.Apply(cmd)
				}
			}

			n.state.SetLastApplied(lastApplied)
		}

		time.Sleep(10 * time.Millisecond)
	}
}

// GetPeers returns the list of peers.
func (n *Node) GetPeers() []*Peer {
	n.mu.RLock()
	defer n.mu.RUnlock()

	peers := make([]*Peer, 0, len(n.peers))
	for _, p := range n.peers {
		peers = append(peers, p)
	}
	return peers
}
