package raft

import (
	"errors"
	"sync"
	"testing"
	"time"
)

// MockStateMachine implements StateMachine for testing.
type MockStateMachine struct {
	applied  []*Command
	snapshot []byte
	applyErr error
	mu       sync.Mutex
}

func NewMockStateMachine() *MockStateMachine {
	return &MockStateMachine{
		applied: make([]*Command, 0),
	}
}

func (m *MockStateMachine) Apply(cmd *Command) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.applied = append(m.applied, cmd)
	return m.applyErr
}

func (m *MockStateMachine) Snapshot() ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.snapshot, nil
}

func (m *MockStateMachine) Restore(data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.snapshot = data
	return nil
}

func (m *MockStateMachine) AppliedCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.applied)
}

func (m *MockStateMachine) SetApplyError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.applyErr = err
}

// TestCluster helps manage a cluster of Raft nodes for testing.
type TestCluster struct {
	nodes   []*Node
	network *InMemoryNetwork
}

func NewTestCluster(size int) *TestCluster {
	network := NewInMemoryNetwork()
	nodes := make([]*Node, size)

	// Create peer list
	peers := make([]*Peer, size)
	for i := 0; i < size; i++ {
		peers[i] = &Peer{
			ID:   uint64(i + 1),
			Addr: "node" + string(rune('0'+i+1)) + ":4445",
		}
	}

	// Create nodes
	for i := 0; i < size; i++ {
		cfg := &NodeConfig{
			ID:               uint64(i + 1),
			Addr:             "node" + string(rune('0'+i+1)) + ":4445",
			Peers:            peers,
			ElectionTimeout:  50 * time.Millisecond,
			HeartbeatTimeout: 20 * time.Millisecond,
		}

		transport := network.NewTransport(uint64(i+1), cfg.Addr)
		sm := NewMockStateMachine()

		node, err := NewNode(cfg, sm, transport)
		if err != nil {
			panic(err)
		}
		nodes[i] = node
	}

	return &TestCluster{
		nodes:   nodes,
		network: network,
	}
}

func (c *TestCluster) Start() {
	for _, node := range c.nodes {
		node.Start()
	}
}

func (c *TestCluster) Stop() {
	for _, node := range c.nodes {
		node.Stop()
	}
}

func (c *TestCluster) Leader() *Node {
	for _, node := range c.nodes {
		if node.IsLeader() {
			return node
		}
	}
	return nil
}

func (c *TestCluster) WaitForLeader(timeout time.Duration) *Node {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if leader := c.Leader(); leader != nil {
			return leader
		}
		time.Sleep(10 * time.Millisecond)
	}
	return nil
}

func TestNewNode(t *testing.T) {
	cfg := &NodeConfig{
		ID:               1,
		Addr:             "localhost:4445",
		ElectionTimeout:  150 * time.Millisecond,
		HeartbeatTimeout: 50 * time.Millisecond,
	}

	network := NewInMemoryNetwork()
	transport := network.NewTransport(1, "localhost:4445")
	sm := NewMockStateMachine()

	node, err := NewNode(cfg, sm, transport)
	if err != nil {
		t.Fatalf("NewNode failed: %v", err)
	}

	if node.ID() != 1 {
		t.Errorf("ID mismatch")
	}
	if node.State() != StateFollower {
		t.Errorf("Initial state should be Follower")
	}
	if node.Term() != 0 {
		t.Errorf("Initial term should be 0")
	}
	if node.IsLeader() {
		t.Errorf("Should not be leader initially")
	}
}

func TestNewNodeInvalidConfig(t *testing.T) {
	cfg := &NodeConfig{
		ID: 0, // Invalid
	}

	network := NewInMemoryNetwork()
	transport := network.NewTransport(1, "")

	_, err := NewNode(cfg, nil, transport)
	if err != ErrInvalidConfig {
		t.Errorf("Expected ErrInvalidConfig, got %v", err)
	}
}

func TestSingleNodeLeaderElection(t *testing.T) {
	cfg := &NodeConfig{
		ID:               1,
		Addr:             "localhost:4445",
		Peers:            []*Peer{{ID: 1, Addr: "localhost:4445"}},
		ElectionTimeout:  50 * time.Millisecond,
		HeartbeatTimeout: 20 * time.Millisecond,
	}

	network := NewInMemoryNetwork()
	transport := network.NewTransport(1, "localhost:4445")
	sm := NewMockStateMachine()

	node, _ := NewNode(cfg, sm, transport)
	node.Start()
	defer node.Stop()

	// Single node should become leader quickly
	time.Sleep(200 * time.Millisecond)

	if !node.IsLeader() {
		t.Errorf("Single node should become leader")
	}
}

func TestThreeNodeLeaderElection(t *testing.T) {
	cluster := NewTestCluster(3)
	cluster.Start()
	defer cluster.Stop()

	// Wait for leader election
	leader := cluster.WaitForLeader(2 * time.Second)
	if leader == nil {
		t.Fatal("No leader elected")
	}

	// Count leaders
	leaderCount := 0
	for _, node := range cluster.nodes {
		if node.IsLeader() {
			leaderCount++
		}
	}

	if leaderCount != 1 {
		t.Errorf("Expected exactly 1 leader, got %d", leaderCount)
	}
}

func TestPropose(t *testing.T) {
	cluster := NewTestCluster(3)
	cluster.Start()
	defer cluster.Stop()

	leader := cluster.WaitForLeader(2 * time.Second)
	if leader == nil {
		t.Fatal("No leader elected")
	}

	// Propose a command
	cmd := &Command{
		Type: CmdPut,
		DN:   "cn=test,dc=example,dc=com",
	}

	err := leader.Propose(cmd)
	if err != nil {
		t.Fatalf("Propose failed: %v", err)
	}

	// Log should have the entry
	if leader.state.Log().LastIndex() < 2 {
		t.Errorf("Log should have at least 2 entries (noop + command)")
	}
}

func TestProposeNotLeader(t *testing.T) {
	cfg := &NodeConfig{
		ID:               1,
		Addr:             "localhost:4445",
		ElectionTimeout:  1 * time.Hour, // Never timeout
		HeartbeatTimeout: 50 * time.Millisecond,
	}

	network := NewInMemoryNetwork()
	transport := network.NewTransport(1, "localhost:4445")
	sm := NewMockStateMachine()

	node, _ := NewNode(cfg, sm, transport)
	node.Start()
	defer node.Stop()

	// Node is follower, propose should fail
	cmd := &Command{Type: CmdPut, DN: "cn=test"}
	err := node.Propose(cmd)
	if err != ErrNotLeader {
		t.Errorf("Expected ErrNotLeader, got %v", err)
	}
}

func TestProposeReturnsApplyError(t *testing.T) {
	cfg := &NodeConfig{
		ID:               1,
		Addr:             "localhost:4445",
		Peers:            []*Peer{{ID: 1, Addr: "localhost:4445"}},
		ElectionTimeout:  50 * time.Millisecond,
		HeartbeatTimeout: 20 * time.Millisecond,
	}

	network := NewInMemoryNetwork()
	transport := network.NewTransport(1, "localhost:4445")
	sm := NewMockStateMachine()
	applyErr := errors.New("uid attribute must be unique")
	sm.SetApplyError(applyErr)

	node, _ := NewNode(cfg, sm, transport)
	node.Start()
	defer node.Stop()

	time.Sleep(200 * time.Millisecond)
	if !node.IsLeader() {
		t.Fatal("single node should become leader")
	}

	cmd := &Command{Type: CmdPut, DN: "uid=dup,dc=example,dc=com"}
	err := node.Propose(cmd)
	if !errors.Is(err, applyErr) {
		t.Fatalf("expected apply error %v, got %v", applyErr, err)
	}
}

func TestHandleRequestVote(t *testing.T) {
	cfg := &NodeConfig{
		ID:               1,
		Addr:             "localhost:4445",
		ElectionTimeout:  1 * time.Hour,
		HeartbeatTimeout: 50 * time.Millisecond,
	}

	network := NewInMemoryNetwork()
	transport := network.NewTransport(1, "localhost:4445")
	sm := NewMockStateMachine()

	node, _ := NewNode(cfg, sm, transport)

	// Request vote with higher term
	args := &RequestVoteArgs{
		Term:         5,
		CandidateID:  2,
		LastLogIndex: 0,
		LastLogTerm:  0,
	}

	respData := node.handleRequestVote(args.Serialize())
	reply, _ := DeserializeRequestVoteReply(respData)

	if !reply.VoteGranted {
		t.Errorf("Vote should be granted")
	}
	if node.state.VotedFor() != 2 {
		t.Errorf("VotedFor should be 2")
	}
}

func TestHandleRequestVoteLowerTerm(t *testing.T) {
	cfg := &NodeConfig{
		ID:               1,
		Addr:             "localhost:4445",
		ElectionTimeout:  1 * time.Hour,
		HeartbeatTimeout: 50 * time.Millisecond,
	}

	network := NewInMemoryNetwork()
	transport := network.NewTransport(1, "localhost:4445")
	sm := NewMockStateMachine()

	node, _ := NewNode(cfg, sm, transport)
	node.state.SetCurrentTerm(10)

	// Request vote with lower term
	args := &RequestVoteArgs{
		Term:        5,
		CandidateID: 2,
	}

	respData := node.handleRequestVote(args.Serialize())
	reply, _ := DeserializeRequestVoteReply(respData)

	if reply.VoteGranted {
		t.Errorf("Vote should not be granted for lower term")
	}
}

func TestHandleAppendEntries(t *testing.T) {
	cfg := &NodeConfig{
		ID:               1,
		Addr:             "localhost:4445",
		ElectionTimeout:  1 * time.Hour,
		HeartbeatTimeout: 50 * time.Millisecond,
	}

	network := NewInMemoryNetwork()
	transport := network.NewTransport(1, "localhost:4445")
	sm := NewMockStateMachine()

	node, _ := NewNode(cfg, sm, transport)

	// Heartbeat from leader
	args := &AppendEntriesArgs{
		Term:         1,
		LeaderID:     2,
		PrevLogIndex: 0,
		PrevLogTerm:  0,
		Entries:      nil,
		LeaderCommit: 0,
	}

	respData := node.handleAppendEntries(args.Serialize())
	reply, _ := DeserializeAppendEntriesReply(respData)

	if !reply.Success {
		t.Errorf("Heartbeat should succeed")
	}
	if node.state.LeaderID() != 2 {
		t.Errorf("LeaderID should be 2")
	}
}

func TestHandleAppendEntriesWithEntries(t *testing.T) {
	cfg := &NodeConfig{
		ID:               1,
		Addr:             "localhost:4445",
		ElectionTimeout:  1 * time.Hour,
		HeartbeatTimeout: 50 * time.Millisecond,
	}

	network := NewInMemoryNetwork()
	transport := network.NewTransport(1, "localhost:4445")
	sm := NewMockStateMachine()

	node, _ := NewNode(cfg, sm, transport)

	// AppendEntries with entries
	args := &AppendEntriesArgs{
		Term:         1,
		LeaderID:     2,
		PrevLogIndex: 0,
		PrevLogTerm:  0,
		Entries: []*LogEntry{
			{Index: 1, Term: 1, Type: LogEntryCommand, Command: []byte("cmd1")},
			{Index: 2, Term: 1, Type: LogEntryCommand, Command: []byte("cmd2")},
		},
		LeaderCommit: 2,
	}

	respData := node.handleAppendEntries(args.Serialize())
	reply, _ := DeserializeAppendEntriesReply(respData)

	if !reply.Success {
		t.Errorf("AppendEntries should succeed")
	}
	if node.state.Log().LastIndex() != 2 {
		t.Errorf("Log should have 2 entries, got %d", node.state.Log().LastIndex())
	}
	if node.state.CommitIndex() != 2 {
		t.Errorf("CommitIndex should be 2")
	}
}

func TestGetPeers(t *testing.T) {
	cfg := &NodeConfig{
		ID:   1,
		Addr: "localhost:4445",
		Peers: []*Peer{
			{ID: 1, Addr: "localhost:4445"},
			{ID: 2, Addr: "localhost:4446"},
			{ID: 3, Addr: "localhost:4447"},
		},
		ElectionTimeout:  150 * time.Millisecond,
		HeartbeatTimeout: 50 * time.Millisecond,
	}

	network := NewInMemoryNetwork()
	transport := network.NewTransport(1, "localhost:4445")

	node, _ := NewNode(cfg, nil, transport)
	peers := node.GetPeers()

	// Should not include self
	if len(peers) != 2 {
		t.Errorf("Expected 2 peers, got %d", len(peers))
	}
}
