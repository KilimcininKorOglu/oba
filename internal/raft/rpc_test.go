package raft

import (
	"bytes"
	"testing"
)

func TestRequestVoteSerialization(t *testing.T) {
	args := &RequestVoteArgs{
		Term:         5,
		CandidateID:  2,
		LastLogIndex: 100,
		LastLogTerm:  4,
	}

	data := args.Serialize()
	restored, err := DeserializeRequestVoteArgs(data)
	if err != nil {
		t.Fatalf("Deserialize failed: %v", err)
	}

	if restored.Term != args.Term {
		t.Errorf("Term mismatch: got %d, want %d", restored.Term, args.Term)
	}
	if restored.CandidateID != args.CandidateID {
		t.Errorf("CandidateID mismatch: got %d, want %d", restored.CandidateID, args.CandidateID)
	}
	if restored.LastLogIndex != args.LastLogIndex {
		t.Errorf("LastLogIndex mismatch: got %d, want %d", restored.LastLogIndex, args.LastLogIndex)
	}
	if restored.LastLogTerm != args.LastLogTerm {
		t.Errorf("LastLogTerm mismatch: got %d, want %d", restored.LastLogTerm, args.LastLogTerm)
	}
}

func TestRequestVoteReplySerialization(t *testing.T) {
	tests := []struct {
		name        string
		term        uint64
		voteGranted bool
	}{
		{"vote granted", 5, true},
		{"vote denied", 3, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reply := &RequestVoteReply{
				Term:        tt.term,
				VoteGranted: tt.voteGranted,
			}

			data := reply.Serialize()
			restored, err := DeserializeRequestVoteReply(data)
			if err != nil {
				t.Fatalf("Deserialize failed: %v", err)
			}

			if restored.Term != reply.Term {
				t.Errorf("Term mismatch")
			}
			if restored.VoteGranted != reply.VoteGranted {
				t.Errorf("VoteGranted mismatch")
			}
		})
	}
}

func TestAppendEntriesSerialization(t *testing.T) {
	args := &AppendEntriesArgs{
		Term:         10,
		LeaderID:     1,
		PrevLogIndex: 50,
		PrevLogTerm:  9,
		LeaderCommit: 45,
		Entries: []*LogEntry{
			{Index: 51, Term: 10, Type: LogEntryCommand, Command: []byte("cmd1")},
			{Index: 52, Term: 10, Type: LogEntryCommand, Command: []byte("cmd2")},
		},
	}

	data := args.Serialize()
	restored, err := DeserializeAppendEntriesArgs(data)
	if err != nil {
		t.Fatalf("Deserialize failed: %v", err)
	}

	if restored.Term != args.Term {
		t.Errorf("Term mismatch")
	}
	if restored.LeaderID != args.LeaderID {
		t.Errorf("LeaderID mismatch")
	}
	if restored.PrevLogIndex != args.PrevLogIndex {
		t.Errorf("PrevLogIndex mismatch")
	}
	if restored.PrevLogTerm != args.PrevLogTerm {
		t.Errorf("PrevLogTerm mismatch")
	}
	if restored.LeaderCommit != args.LeaderCommit {
		t.Errorf("LeaderCommit mismatch")
	}
	if len(restored.Entries) != len(args.Entries) {
		t.Fatalf("Entries count mismatch: got %d, want %d", len(restored.Entries), len(args.Entries))
	}

	for i, entry := range restored.Entries {
		if entry.Index != args.Entries[i].Index {
			t.Errorf("Entry[%d] Index mismatch", i)
		}
		if entry.Term != args.Entries[i].Term {
			t.Errorf("Entry[%d] Term mismatch", i)
		}
		if !bytes.Equal(entry.Command, args.Entries[i].Command) {
			t.Errorf("Entry[%d] Command mismatch", i)
		}
	}
}

func TestAppendEntriesSerializationHeartbeat(t *testing.T) {
	// Heartbeat has no entries
	args := &AppendEntriesArgs{
		Term:         10,
		LeaderID:     1,
		PrevLogIndex: 50,
		PrevLogTerm:  9,
		LeaderCommit: 45,
		Entries:      nil,
	}

	data := args.Serialize()
	restored, err := DeserializeAppendEntriesArgs(data)
	if err != nil {
		t.Fatalf("Deserialize failed: %v", err)
	}

	if len(restored.Entries) != 0 {
		t.Errorf("Heartbeat should have no entries")
	}
}

func TestAppendEntriesReplySerialization(t *testing.T) {
	tests := []struct {
		name          string
		term          uint64
		success       bool
		conflictTerm  uint64
		conflictIndex uint64
	}{
		{"success", 10, true, 0, 0},
		{"conflict", 10, false, 8, 40},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reply := &AppendEntriesReply{
				Term:          tt.term,
				Success:       tt.success,
				ConflictTerm:  tt.conflictTerm,
				ConflictIndex: tt.conflictIndex,
			}

			data := reply.Serialize()
			restored, err := DeserializeAppendEntriesReply(data)
			if err != nil {
				t.Fatalf("Deserialize failed: %v", err)
			}

			if restored.Term != reply.Term {
				t.Errorf("Term mismatch")
			}
			if restored.Success != reply.Success {
				t.Errorf("Success mismatch")
			}
			if restored.ConflictTerm != reply.ConflictTerm {
				t.Errorf("ConflictTerm mismatch")
			}
			if restored.ConflictIndex != reply.ConflictIndex {
				t.Errorf("ConflictIndex mismatch")
			}
		})
	}
}

func TestInstallSnapshotSerialization(t *testing.T) {
	args := &InstallSnapshotArgs{
		Term:              15,
		LeaderID:          1,
		LastIncludedIndex: 100,
		LastIncludedTerm:  14,
		Data:              []byte("snapshot data here"),
	}

	data := args.Serialize()
	restored, err := DeserializeInstallSnapshotArgs(data)
	if err != nil {
		t.Fatalf("Deserialize failed: %v", err)
	}

	if restored.Term != args.Term {
		t.Errorf("Term mismatch")
	}
	if restored.LeaderID != args.LeaderID {
		t.Errorf("LeaderID mismatch")
	}
	if restored.LastIncludedIndex != args.LastIncludedIndex {
		t.Errorf("LastIncludedIndex mismatch")
	}
	if restored.LastIncludedTerm != args.LastIncludedTerm {
		t.Errorf("LastIncludedTerm mismatch")
	}
	if !bytes.Equal(restored.Data, args.Data) {
		t.Errorf("Data mismatch")
	}
}

func TestInstallSnapshotSerializationEmpty(t *testing.T) {
	args := &InstallSnapshotArgs{
		Term:              15,
		LeaderID:          1,
		LastIncludedIndex: 100,
		LastIncludedTerm:  14,
		Data:              nil,
	}

	data := args.Serialize()
	restored, err := DeserializeInstallSnapshotArgs(data)
	if err != nil {
		t.Fatalf("Deserialize failed: %v", err)
	}

	if len(restored.Data) != 0 {
		t.Errorf("Data should be empty")
	}
}

func TestInstallSnapshotReplySerialization(t *testing.T) {
	reply := &InstallSnapshotReply{Term: 15}

	data := reply.Serialize()
	restored, err := DeserializeInstallSnapshotReply(data)
	if err != nil {
		t.Fatalf("Deserialize failed: %v", err)
	}

	if restored.Term != reply.Term {
		t.Errorf("Term mismatch")
	}
}

func TestDeserializeCorruptedData(t *testing.T) {
	shortData := []byte{1, 2, 3}

	_, err := DeserializeRequestVoteArgs(shortData)
	if err != ErrLogCorrupted {
		t.Errorf("Expected ErrLogCorrupted for RequestVoteArgs")
	}

	_, err = DeserializeRequestVoteReply(shortData)
	if err != ErrLogCorrupted {
		t.Errorf("Expected ErrLogCorrupted for RequestVoteReply")
	}

	_, err = DeserializeAppendEntriesArgs(shortData)
	if err != ErrLogCorrupted {
		t.Errorf("Expected ErrLogCorrupted for AppendEntriesArgs")
	}

	_, err = DeserializeAppendEntriesReply(shortData)
	if err != ErrLogCorrupted {
		t.Errorf("Expected ErrLogCorrupted for AppendEntriesReply")
	}

	_, err = DeserializeInstallSnapshotArgs(shortData)
	if err != ErrLogCorrupted {
		t.Errorf("Expected ErrLogCorrupted for InstallSnapshotArgs")
	}

	_, err = DeserializeInstallSnapshotReply(shortData)
	if err != ErrLogCorrupted {
		t.Errorf("Expected ErrLogCorrupted for InstallSnapshotReply")
	}
}
