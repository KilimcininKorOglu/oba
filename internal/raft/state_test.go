package raft

import (
	"testing"
	"time"
)

func TestNodeConfig(t *testing.T) {
	cfg := DefaultNodeConfig()

	if cfg.ElectionTimeout != 150*time.Millisecond {
		t.Errorf("Default ElectionTimeout should be 150ms")
	}
	if cfg.HeartbeatTimeout != 50*time.Millisecond {
		t.Errorf("Default HeartbeatTimeout should be 50ms")
	}
}

func TestNodeConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *NodeConfig
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: &NodeConfig{
				ID:               1,
				Addr:             "localhost:4445",
				ElectionTimeout:  150 * time.Millisecond,
				HeartbeatTimeout: 50 * time.Millisecond,
			},
			wantErr: false,
		},
		{
			name: "missing ID",
			cfg: &NodeConfig{
				Addr:             "localhost:4445",
				ElectionTimeout:  150 * time.Millisecond,
				HeartbeatTimeout: 50 * time.Millisecond,
			},
			wantErr: true,
		},
		{
			name: "missing Addr",
			cfg: &NodeConfig{
				ID:               1,
				ElectionTimeout:  150 * time.Millisecond,
				HeartbeatTimeout: 50 * time.Millisecond,
			},
			wantErr: true,
		},
		{
			name: "heartbeat >= election",
			cfg: &NodeConfig{
				ID:               1,
				Addr:             "localhost:4445",
				ElectionTimeout:  50 * time.Millisecond,
				HeartbeatTimeout: 50 * time.Millisecond,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNodeState(t *testing.T) {
	state := NewNodeState()

	// Initial state
	if state.State() != StateFollower {
		t.Errorf("Initial state should be Follower")
	}
	if state.CurrentTerm() != 0 {
		t.Errorf("Initial term should be 0")
	}
	if state.VotedFor() != 0 {
		t.Errorf("Initial votedFor should be 0")
	}
	if state.CommitIndex() != 0 {
		t.Errorf("Initial commitIndex should be 0")
	}
	if state.LastApplied() != 0 {
		t.Errorf("Initial lastApplied should be 0")
	}
}

func TestNodeStateTransitions(t *testing.T) {
	state := NewNodeState()

	// Follower -> Candidate
	term := state.BecomeCandidate()
	if term != 1 {
		t.Errorf("Term after BecomeCandidate should be 1, got %d", term)
	}
	if !state.IsCandidate() {
		t.Error("Should be candidate")
	}
	if state.LeaderID() != 0 {
		t.Error("LeaderID should be 0 after becoming candidate")
	}

	// Candidate -> Leader
	state.BecomeLeader(1)
	if !state.IsLeader() {
		t.Error("Should be leader")
	}
	if state.LeaderID() != 1 {
		t.Errorf("LeaderID should be 1, got %d", state.LeaderID())
	}

	// Leader -> Follower (higher term discovered)
	state.BecomeFollower(5)
	if !state.IsFollower() {
		t.Error("Should be follower")
	}
	if state.CurrentTerm() != 5 {
		t.Errorf("Term should be 5, got %d", state.CurrentTerm())
	}
	if state.VotedFor() != 0 {
		t.Error("VotedFor should be reset to 0")
	}
}

func TestNodeStateLeaderInit(t *testing.T) {
	state := NewNodeState()

	// Add some log entries
	state.AppendEntry(&LogEntry{Index: 1, Term: 1, Type: LogEntryCommand})
	state.AppendEntry(&LogEntry{Index: 2, Term: 1, Type: LogEntryCommand})

	peers := []*Peer{
		{ID: 2, Addr: "node2:4445"},
		{ID: 3, Addr: "node3:4445"},
	}

	state.InitLeaderState(peers)

	// nextIndex should be lastLogIndex + 1
	if state.GetNextIndex(2) != 3 {
		t.Errorf("nextIndex[2] should be 3, got %d", state.GetNextIndex(2))
	}
	if state.GetNextIndex(3) != 3 {
		t.Errorf("nextIndex[3] should be 3, got %d", state.GetNextIndex(3))
	}

	// matchIndex should be 0
	if state.GetMatchIndex(2) != 0 {
		t.Errorf("matchIndex[2] should be 0, got %d", state.GetMatchIndex(2))
	}
}

func TestNodeStateVoting(t *testing.T) {
	state := NewNodeState()

	state.SetCurrentTerm(5)
	state.SetVotedFor(2)

	if state.CurrentTerm() != 5 {
		t.Errorf("CurrentTerm should be 5")
	}
	if state.VotedFor() != 2 {
		t.Errorf("VotedFor should be 2")
	}

	// Becoming follower with higher term resets votedFor
	state.BecomeFollower(6)
	if state.VotedFor() != 0 {
		t.Error("VotedFor should be reset after term change")
	}
}

func TestNodeStateCommitIndex(t *testing.T) {
	state := NewNodeState()

	state.SetCommitIndex(10)
	if state.CommitIndex() != 10 {
		t.Errorf("CommitIndex should be 10")
	}

	state.SetLastApplied(5)
	if state.LastApplied() != 5 {
		t.Errorf("LastApplied should be 5")
	}
}

func TestNodeStateHeartbeat(t *testing.T) {
	state := NewNodeState()

	now := time.Now()
	state.SetLastHeartbeat(now)

	if !state.LastHeartbeat().Equal(now) {
		t.Error("LastHeartbeat mismatch")
	}
}

func TestStateString(t *testing.T) {
	tests := []struct {
		state uint8
		want  string
	}{
		{StateFollower, "follower"},
		{StateCandidate, "candidate"},
		{StateLeader, "leader"},
		{99, "unknown"},
	}

	for _, tt := range tests {
		got := StateString(tt.state)
		if got != tt.want {
			t.Errorf("StateString(%d) = %s, want %s", tt.state, got, tt.want)
		}
	}
}

func TestNodeStateMatchIndexes(t *testing.T) {
	state := NewNodeState()

	state.SetMatchIndex(2, 10)
	state.SetMatchIndex(3, 15)

	indexes := state.GetMatchIndexes()

	if indexes[2] != 10 {
		t.Errorf("matchIndex[2] should be 10")
	}
	if indexes[3] != 15 {
		t.Errorf("matchIndex[3] should be 15")
	}

	// Verify it's a copy
	indexes[2] = 999
	if state.GetMatchIndex(2) != 10 {
		t.Error("GetMatchIndexes should return a copy")
	}
}
