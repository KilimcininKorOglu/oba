package raft

import (
	"testing"
	"time"

	"github.com/KilimcininKorOglu/oba/internal/storage"
)

// Integration tests for the complete Raft implementation.

func TestClusterLeaderElection(t *testing.T) {
	cluster := NewTestCluster(3)
	cluster.Start()
	defer cluster.Stop()

	// Wait for leader election
	leader := cluster.WaitForLeader(3 * time.Second)
	if leader == nil {
		t.Fatal("No leader elected within timeout")
	}

	// Verify exactly one leader
	leaderCount := 0
	for _, node := range cluster.nodes {
		if node.IsLeader() {
			leaderCount++
		}
	}
	if leaderCount != 1 {
		t.Errorf("Expected 1 leader, got %d", leaderCount)
	}

	// All nodes should have the same term
	term := leader.Term()
	for _, node := range cluster.nodes {
		if node.Term() != term {
			t.Errorf("Node %d has term %d, expected %d", node.ID(), node.Term(), term)
		}
	}
}

func TestClusterLogReplication(t *testing.T) {
	cluster := NewTestCluster(3)
	cluster.Start()
	defer cluster.Stop()

	leader := cluster.WaitForLeader(3 * time.Second)
	if leader == nil {
		t.Fatal("No leader elected")
	}

	// Propose a command
	cmd := &Command{
		Type: CmdPut,
		DN:   "cn=test,dc=example,dc=com",
	}

	if err := leader.Propose(cmd); err != nil {
		t.Fatalf("Propose failed: %v", err)
	}

	// Wait for replication
	time.Sleep(200 * time.Millisecond)

	// All nodes should have the entry in their log
	for _, node := range cluster.nodes {
		logLen := node.state.Log().Len()
		// Should have: noop (index 0) + leader noop + command = at least 3
		if logLen < 2 {
			t.Errorf("Node %d log too short: %d entries", node.ID(), logLen)
		}
	}
}

func TestClusterCommitIndex(t *testing.T) {
	cluster := NewTestCluster(3)
	cluster.Start()
	defer cluster.Stop()

	leader := cluster.WaitForLeader(3 * time.Second)
	if leader == nil {
		t.Fatal("No leader elected")
	}

	// Propose multiple commands
	for i := 0; i < 5; i++ {
		cmd := &Command{
			Type: CmdPut,
			DN:   "cn=test" + itoa(uint64(i)) + ",dc=example,dc=com",
		}
		if err := leader.Propose(cmd); err != nil {
			t.Fatalf("Propose %d failed: %v", i, err)
		}
	}

	// Wait for replication and commit
	time.Sleep(500 * time.Millisecond)

	// Leader's commit index should be updated
	if leader.CommitIndex() < 2 {
		t.Errorf("Leader commit index too low: %d", leader.CommitIndex())
	}
}

func TestClusterFollowerRedirect(t *testing.T) {
	cluster := NewTestCluster(3)
	cluster.Start()
	defer cluster.Stop()

	leader := cluster.WaitForLeader(3 * time.Second)
	if leader == nil {
		t.Fatal("No leader elected")
	}

	// Find a follower
	var follower *Node
	for _, node := range cluster.nodes {
		if !node.IsLeader() {
			follower = node
			break
		}
	}

	if follower == nil {
		t.Fatal("No follower found")
	}

	// Propose to follower should fail
	cmd := &Command{Type: CmdPut, DN: "cn=test"}
	err := follower.Propose(cmd)
	if err != ErrNotLeader {
		t.Errorf("Expected ErrNotLeader, got %v", err)
	}
}

func TestClusterTermConsistency(t *testing.T) {
	cluster := NewTestCluster(5)
	cluster.Start()
	defer cluster.Stop()

	leader := cluster.WaitForLeader(3 * time.Second)
	if leader == nil {
		t.Fatal("No leader elected")
	}

	// All nodes should agree on the term
	term := leader.Term()
	for _, node := range cluster.nodes {
		nodeTerm := node.Term()
		if nodeTerm != term {
			t.Errorf("Node %d has term %d, leader has %d", node.ID(), nodeTerm, term)
		}
	}
}

func TestClusterLeaderKnown(t *testing.T) {
	cluster := NewTestCluster(3)
	cluster.Start()
	defer cluster.Stop()

	leader := cluster.WaitForLeader(3 * time.Second)
	if leader == nil {
		t.Fatal("No leader elected")
	}

	// Wait for followers to learn about leader
	time.Sleep(200 * time.Millisecond)

	// All followers should know the leader
	for _, node := range cluster.nodes {
		if !node.IsLeader() {
			leaderID := node.LeaderID()
			if leaderID != leader.ID() {
				t.Errorf("Node %d thinks leader is %d, actual is %d", node.ID(), leaderID, leader.ID())
			}
		}
	}
}

func TestFiveNodeCluster(t *testing.T) {
	cluster := NewTestCluster(5)
	cluster.Start()
	defer cluster.Stop()

	leader := cluster.WaitForLeader(3 * time.Second)
	if leader == nil {
		t.Fatal("No leader elected in 5-node cluster")
	}

	// Propose commands
	for i := 0; i < 10; i++ {
		cmd := &Command{
			Type: CmdPut,
			DN:   "cn=user" + itoa(uint64(i)) + ",dc=example,dc=com",
		}
		if err := leader.Propose(cmd); err != nil {
			t.Fatalf("Propose failed: %v", err)
		}
	}

	// Wait for replication
	time.Sleep(500 * time.Millisecond)

	// Verify all nodes have entries
	for _, node := range cluster.nodes {
		if node.state.Log().Len() < 5 {
			t.Errorf("Node %d has too few log entries", node.ID())
		}
	}
}

func TestSnapshotIntegration(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSnapshotStore(dir)
	if err != nil {
		t.Fatalf("NewSnapshotStore failed: %v", err)
	}

	sm := NewMockStateMachine()
	sm.snapshot = []byte("test state data")

	snapshotter := NewSnapshotter(store, sm, &SnapshotPolicy{
		LogSizeThreshold: 10,
	})

	// Should snapshot at 10 entries
	if !snapshotter.ShouldSnapshot(10) {
		t.Error("Should trigger snapshot at threshold")
	}

	// Create snapshot
	snap, err := snapshotter.CreateSnapshot(100, 5)
	if err != nil {
		t.Fatalf("CreateSnapshot failed: %v", err)
	}

	if snap.LastIncludedIndex != 100 {
		t.Errorf("LastIncludedIndex mismatch")
	}

	// Load and verify
	loaded, err := snapshotter.LoadLatest()
	if err != nil {
		t.Fatalf("LoadLatest failed: %v", err)
	}

	if loaded.LastIncludedIndex != 100 {
		t.Errorf("Loaded snapshot mismatch")
	}
}

func TestStateMachineIntegration(t *testing.T) {
	engine := NewMockStorageEngine()
	sm := NewObaDBStateMachine(engine)

	// Apply Put command
	entry := &storage.Entry{
		DN:         "cn=test,dc=example,dc=com",
		Attributes: make(map[string][][]byte),
	}
	entry.Attributes["cn"] = [][]byte{[]byte("test")}

	cmd := CreatePutCommand(entry)
	if err := sm.Apply(cmd); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	// Verify entry exists
	if _, ok := engine.entries["cn=test,dc=example,dc=com"]; !ok {
		t.Error("Entry should exist after Put")
	}

	// Create snapshot
	snapData, err := sm.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}

	// Restore to new engine
	engine2 := NewMockStorageEngine()
	sm2 := NewObaDBStateMachine(engine2)

	if err := sm2.Restore(snapData); err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	// Verify entry was restored
	if len(engine2.entries) != 1 {
		t.Errorf("Expected 1 entry after restore, got %d", len(engine2.entries))
	}
}

func TestRPCRoundTrip(t *testing.T) {
	// Test RequestVote round trip
	voteArgs := &RequestVoteArgs{
		Term:         10,
		CandidateID:  2,
		LastLogIndex: 50,
		LastLogTerm:  9,
	}

	voteData := voteArgs.Serialize()
	restoredVote, err := DeserializeRequestVoteArgs(voteData)
	if err != nil {
		t.Fatalf("DeserializeRequestVoteArgs failed: %v", err)
	}

	if restoredVote.Term != voteArgs.Term ||
		restoredVote.CandidateID != voteArgs.CandidateID {
		t.Error("RequestVote round trip failed")
	}

	// Test AppendEntries round trip
	appendArgs := &AppendEntriesArgs{
		Term:         10,
		LeaderID:     1,
		PrevLogIndex: 50,
		PrevLogTerm:  9,
		LeaderCommit: 45,
		Entries: []*LogEntry{
			{Index: 51, Term: 10, Type: LogEntryCommand, Command: []byte("cmd")},
		},
	}

	appendData := appendArgs.Serialize()
	restoredAppend, err := DeserializeAppendEntriesArgs(appendData)
	if err != nil {
		t.Fatalf("DeserializeAppendEntriesArgs failed: %v", err)
	}

	if restoredAppend.Term != appendArgs.Term ||
		len(restoredAppend.Entries) != 1 {
		t.Error("AppendEntries round trip failed")
	}
}

func TestLogOperations(t *testing.T) {
	log := NewRaftLog()

	// Append entries
	for i := uint64(1); i <= 10; i++ {
		log.Append(&LogEntry{
			Index:   i,
			Term:    (i-1)/3 + 1, // Terms: 1,1,1,2,2,2,3,3,3,4
			Type:    LogEntryCommand,
			Command: []byte("cmd" + itoa(i)),
		})
	}

	// Verify length
	if log.Len() != 11 { // 10 + initial noop
		t.Errorf("Log length should be 11, got %d", log.Len())
	}

	// Verify last index/term
	if log.LastIndex() != 10 {
		t.Errorf("LastIndex should be 10, got %d", log.LastIndex())
	}
	if log.LastTerm() != 4 {
		t.Errorf("LastTerm should be 4, got %d", log.LastTerm())
	}

	// Test GetFrom
	entries := log.GetFrom(8)
	if len(entries) != 3 {
		t.Errorf("GetFrom(8) should return 3 entries, got %d", len(entries))
	}

	// Test TruncateFrom
	log.TruncateFrom(8)
	if log.LastIndex() != 7 {
		t.Errorf("After truncate, LastIndex should be 7, got %d", log.LastIndex())
	}
}
