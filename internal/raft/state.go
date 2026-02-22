package raft

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Node states.
const (
	StateFollower uint8 = iota
	StateCandidate
	StateLeader
)

// StateString returns the string representation of a node state.
func StateString(state uint8) string {
	switch state {
	case StateFollower:
		return "follower"
	case StateCandidate:
		return "candidate"
	case StateLeader:
		return "leader"
	default:
		return "unknown"
	}
}

// Peer represents a remote node in the cluster.
type Peer struct {
	ID   uint64
	Addr string
}

// NodeConfig holds configuration for a Raft node.
type NodeConfig struct {
	ID               uint64        // Unique node ID
	Addr             string        // Raft RPC listen address
	Peers            []*Peer       // Cluster peers
	ElectionTimeout  time.Duration // Election timeout base
	HeartbeatTimeout time.Duration // Heartbeat interval
	DataDir          string        // Directory for persistent state
}

// DefaultNodeConfig returns default configuration.
func DefaultNodeConfig() *NodeConfig {
	return &NodeConfig{
		ElectionTimeout:  150 * time.Millisecond,
		HeartbeatTimeout: 50 * time.Millisecond,
	}
}

// Validate checks if the configuration is valid.
func (c *NodeConfig) Validate() error {
	if c.ID == 0 {
		return ErrInvalidConfig
	}
	if c.Addr == "" {
		return ErrInvalidConfig
	}
	if c.ElectionTimeout <= 0 {
		return ErrInvalidConfig
	}
	if c.HeartbeatTimeout <= 0 {
		return ErrInvalidConfig
	}
	if c.HeartbeatTimeout >= c.ElectionTimeout {
		return ErrInvalidConfig
	}
	return nil
}

// NodeState holds the state of a Raft node.
type NodeState struct {
	// Persistent state (must be saved to disk before responding to RPCs)
	currentTerm uint64
	votedFor    uint64 // 0 means not voted
	log         *RaftLog

	// Volatile state on all servers
	state       uint8
	commitIndex uint64
	lastApplied uint64

	// Volatile state on leaders (reinitialized after election)
	nextIndex  map[uint64]uint64 // peer ID -> next log index to send
	matchIndex map[uint64]uint64 // peer ID -> highest replicated index

	// Leader tracking
	leaderID uint64

	// Timing
	lastHeartbeat time.Time

	// Data directory for persistence
	dataDir string

	mu sync.RWMutex
}

// NewNodeState creates a new node state.
func NewNodeState() *NodeState {
	return &NodeState{
		currentTerm: 0,
		votedFor:    0,
		log:         NewRaftLog(),
		state:       StateFollower,
		commitIndex: 0,
		lastApplied: 0,
		nextIndex:   make(map[uint64]uint64),
		matchIndex:  make(map[uint64]uint64),
		leaderID:    0,
		dataDir:     "",
	}
}

// NewNodeStateWithDir creates a new node state with disk persistence.
func NewNodeStateWithDir(dataDir string) (*NodeState, error) {
	log, err := NewRaftLogWithDir(dataDir)
	if err != nil {
		return nil, err
	}

	s := &NodeState{
		currentTerm: 0,
		votedFor:    0,
		log:         log,
		state:       StateFollower,
		commitIndex: 0,
		lastApplied: 0,
		nextIndex:   make(map[uint64]uint64),
		matchIndex:  make(map[uint64]uint64),
		leaderID:    0,
		dataDir:     dataDir,
	}

	// Load persisted state
	if err := s.loadPersistedState(); err != nil {
		return nil, err
	}

	return s, nil
}

// loadPersistedState loads term and votedFor from disk.
// When persistent storage is present, we trust local DB state and resume from last log index.
func (s *NodeState) loadPersistedState() error {
	if s.dataDir == "" {
		return nil
	}

	// Load term and votedFor
	path := filepath.Join(s.dataDir, "term.dat")
	data, err := os.ReadFile(path)
	if err == nil && len(data) >= 16 {
		s.currentTerm = binary.LittleEndian.Uint64(data[0:8])
		s.votedFor = binary.LittleEndian.Uint64(data[8:16])
	}

	s.commitIndex = s.log.LastIndex()

	// Load last applied index from disk if available.
	// If missing, resume from commitIndex to avoid expensive full replays.
	path = filepath.Join(s.dataDir, "last_applied.dat")
	lastAppliedData, err := os.ReadFile(path)
	if err == nil && len(lastAppliedData) >= 8 {
		s.lastApplied = binary.LittleEndian.Uint64(lastAppliedData)
		if s.lastApplied > s.commitIndex {
			s.lastApplied = s.commitIndex
		}
	} else {
		s.lastApplied = s.commitIndex
	}

	return nil
}

// SetDataDir sets the data directory for persistence.
func (s *NodeState) SetDataDir(dir string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dataDir = dir
}

// LoadLastApplied loads the last applied index from disk.
func (s *NodeState) LoadLastApplied() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.dataDir == "" {
		return nil
	}

	path := filepath.Join(s.dataDir, "last_applied.dat")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if len(data) >= 8 {
		s.lastApplied = binary.LittleEndian.Uint64(data)
	}
	return nil
}

// CurrentTerm returns the current term.
func (s *NodeState) CurrentTerm() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentTerm
}

// SetCurrentTerm sets the current term and persists it to disk.
func (s *NodeState) SetCurrentTerm(term uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentTerm = term
	s.persistTermAndVote()
}

// VotedFor returns the candidate ID this node voted for in current term.
func (s *NodeState) VotedFor() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.votedFor
}

// SetVotedFor sets the voted for candidate and persists it to disk.
func (s *NodeState) SetVotedFor(candidateID uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.votedFor = candidateID
	s.persistTermAndVote()
}

// persistTermAndVote saves term and votedFor to disk.
func (s *NodeState) persistTermAndVote() {
	if s.dataDir == "" {
		return
	}
	path := filepath.Join(s.dataDir, "term.dat")
	data := make([]byte, 16)
	binary.LittleEndian.PutUint64(data[0:8], s.currentTerm)
	binary.LittleEndian.PutUint64(data[8:16], s.votedFor)
	os.WriteFile(path, data, 0644)
}

// State returns the current node state.
func (s *NodeState) State() uint8 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

// SetState sets the node state.
func (s *NodeState) SetState(state uint8) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = state
}

// IsLeader returns true if this node is the leader.
func (s *NodeState) IsLeader() bool {
	return s.State() == StateLeader
}

// IsFollower returns true if this node is a follower.
func (s *NodeState) IsFollower() bool {
	return s.State() == StateFollower
}

// IsCandidate returns true if this node is a candidate.
func (s *NodeState) IsCandidate() bool {
	return s.State() == StateCandidate
}

// CommitIndex returns the commit index.
func (s *NodeState) CommitIndex() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.commitIndex
}

// SetCommitIndex sets the commit index.
func (s *NodeState) SetCommitIndex(index uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.commitIndex = index
}

// LastApplied returns the last applied index.
func (s *NodeState) LastApplied() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastApplied
}

// SetLastApplied sets and persists the last applied index.
func (s *NodeState) SetLastApplied(index uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastApplied = index
	s.persistLastApplied()
}

// persistLastApplied saves lastApplied to disk atomically.
func (s *NodeState) persistLastApplied() {
	if s.dataDir == "" {
		return
	}
	path := filepath.Join(s.dataDir, "last_applied.dat")
	tmpPath := path + ".tmp"

	data := make([]byte, 8)
	binary.LittleEndian.PutUint64(data, s.lastApplied)

	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return
	}
	_ = os.Rename(tmpPath, path)
}

// LeaderID returns the current leader's ID.
func (s *NodeState) LeaderID() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.leaderID
}

// SetLeaderID sets the leader ID.
func (s *NodeState) SetLeaderID(id uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.leaderID = id
}

// Log returns the Raft log.
func (s *NodeState) Log() *RaftLog {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.log
}

// LastHeartbeat returns the time of last heartbeat.
func (s *NodeState) LastHeartbeat() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastHeartbeat
}

// SetLastHeartbeat sets the last heartbeat time.
func (s *NodeState) SetLastHeartbeat(t time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastHeartbeat = t
}

// GetNextIndex returns the next index for a peer.
func (s *NodeState) GetNextIndex(peerID uint64) uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.nextIndex[peerID]
}

// SetNextIndex sets the next index for a peer.
func (s *NodeState) SetNextIndex(peerID uint64, index uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextIndex[peerID] = index
}

// GetMatchIndex returns the match index for a peer.
func (s *NodeState) GetMatchIndex(peerID uint64) uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.matchIndex[peerID]
}

// SetMatchIndex sets the match index for a peer.
func (s *NodeState) SetMatchIndex(peerID uint64, index uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.matchIndex[peerID] = index
}

// InitLeaderState initializes leader-specific state after election.
func (s *NodeState) InitLeaderState(peers []*Peer) {
	// Get lastIndex before acquiring lock to avoid deadlock with RaftLog.mu
	lastIndex := s.log.LastIndex()

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, peer := range peers {
		s.nextIndex[peer.ID] = lastIndex + 1
		s.matchIndex[peer.ID] = 0
	}
}

// BecomeFollower transitions to follower state.
func (s *NodeState) BecomeFollower(term uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.state = StateFollower
	s.currentTerm = term
	s.votedFor = 0
}

// BecomeCandidate transitions to candidate state.
func (s *NodeState) BecomeCandidate() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.state = StateCandidate
	s.currentTerm++
	s.leaderID = 0
	return s.currentTerm
}

// BecomeLeader transitions to leader state.
func (s *NodeState) BecomeLeader(nodeID uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.state = StateLeader
	s.leaderID = nodeID
}

// AppendEntry appends an entry to the log.
// AppendEntry appends an entry to the log.
func (s *NodeState) AppendEntry(entry *LogEntry) {
	// Append to log first (RaftLog has its own lock)
	s.log.Append(entry)
}

// GetMatchIndexes returns a copy of all match indexes.
func (s *NodeState) GetMatchIndexes() map[uint64]uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[uint64]uint64, len(s.matchIndex))
	for k, v := range s.matchIndex {
		result[k] = v
	}
	return result
}
