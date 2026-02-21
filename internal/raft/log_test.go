package raft

import (
	"bytes"
	"testing"
)

func TestLogEntrySerialization(t *testing.T) {
	entry := &LogEntry{
		Index:   10,
		Term:    5,
		Type:    LogEntryCommand,
		Command: []byte("test command data"),
	}

	// Serialize
	data := entry.Serialize()

	// Deserialize
	restored, err := DeserializeLogEntry(data)
	if err != nil {
		t.Fatalf("DeserializeLogEntry failed: %v", err)
	}

	// Verify
	if restored.Index != entry.Index {
		t.Errorf("Index mismatch: got %d, want %d", restored.Index, entry.Index)
	}
	if restored.Term != entry.Term {
		t.Errorf("Term mismatch: got %d, want %d", restored.Term, entry.Term)
	}
	if restored.Type != entry.Type {
		t.Errorf("Type mismatch: got %d, want %d", restored.Type, entry.Type)
	}
	if !bytes.Equal(restored.Command, entry.Command) {
		t.Errorf("Command mismatch: got %v, want %v", restored.Command, entry.Command)
	}
}

func TestLogEntrySerializationEmpty(t *testing.T) {
	entry := &LogEntry{
		Index:   1,
		Term:    1,
		Type:    LogEntryNoop,
		Command: nil,
	}

	data := entry.Serialize()
	restored, err := DeserializeLogEntry(data)
	if err != nil {
		t.Fatalf("DeserializeLogEntry failed: %v", err)
	}

	if restored.Index != entry.Index {
		t.Errorf("Index mismatch: got %d, want %d", restored.Index, entry.Index)
	}
	if len(restored.Command) != 0 {
		t.Errorf("Command should be empty, got %v", restored.Command)
	}
}

func TestDeserializeLogEntryCorrupted(t *testing.T) {
	// Too short
	_, err := DeserializeLogEntry([]byte{1, 2, 3})
	if err != ErrLogCorrupted {
		t.Errorf("Expected ErrLogCorrupted, got %v", err)
	}

	// Invalid command length
	data := make([]byte, 21)
	data[17] = 0xFF // Large command length
	data[18] = 0xFF
	data[19] = 0xFF
	data[20] = 0xFF
	_, err = DeserializeLogEntry(data)
	if err != ErrLogCorrupted {
		t.Errorf("Expected ErrLogCorrupted, got %v", err)
	}
}

func TestCommandSerialization(t *testing.T) {
	cmd := &Command{
		Type:      CmdPut,
		DN:        "cn=test,dc=example,dc=com",
		OldDN:     "",
		EntryDN:   "cn=test,dc=example,dc=com",
		EntryData: []byte("entry data here"),
	}

	// Serialize
	data, err := cmd.Serialize()
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	// Deserialize
	restored, err := DeserializeCommand(data)
	if err != nil {
		t.Fatalf("DeserializeCommand failed: %v", err)
	}

	// Verify
	if restored.Type != cmd.Type {
		t.Errorf("Type mismatch: got %d, want %d", restored.Type, cmd.Type)
	}
	if restored.DN != cmd.DN {
		t.Errorf("DN mismatch: got %s, want %s", restored.DN, cmd.DN)
	}
	if restored.OldDN != cmd.OldDN {
		t.Errorf("OldDN mismatch: got %s, want %s", restored.OldDN, cmd.OldDN)
	}
	if restored.EntryDN != cmd.EntryDN {
		t.Errorf("EntryDN mismatch: got %s, want %s", restored.EntryDN, cmd.EntryDN)
	}
	if !bytes.Equal(restored.EntryData, cmd.EntryData) {
		t.Errorf("EntryData mismatch")
	}
}

func TestCommandSerializationModifyDN(t *testing.T) {
	cmd := &Command{
		Type:      CmdModifyDN,
		DN:        "cn=newname,dc=example,dc=com",
		OldDN:     "cn=oldname,dc=example,dc=com",
		EntryDN:   "cn=newname,dc=example,dc=com",
		EntryData: []byte("updated entry"),
	}

	data, err := cmd.Serialize()
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	restored, err := DeserializeCommand(data)
	if err != nil {
		t.Fatalf("DeserializeCommand failed: %v", err)
	}

	if restored.OldDN != cmd.OldDN {
		t.Errorf("OldDN mismatch: got %s, want %s", restored.OldDN, cmd.OldDN)
	}
}

func TestCommandSerializationDelete(t *testing.T) {
	cmd := &Command{
		Type:      CmdDelete,
		DN:        "cn=todelete,dc=example,dc=com",
		EntryData: nil,
	}

	data, err := cmd.Serialize()
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	restored, err := DeserializeCommand(data)
	if err != nil {
		t.Fatalf("DeserializeCommand failed: %v", err)
	}

	if restored.Type != CmdDelete {
		t.Errorf("Type mismatch: got %d, want %d", restored.Type, CmdDelete)
	}
	if restored.EntryData != nil {
		t.Errorf("EntryData should be nil for delete")
	}
}

func TestRaftLog(t *testing.T) {
	log := NewRaftLog()

	// Initial state - has noop entry at index 0
	if log.Len() != 1 {
		t.Errorf("Initial log should have 1 entry, got %d", log.Len())
	}
	if log.LastIndex() != 0 {
		t.Errorf("Initial LastIndex should be 0, got %d", log.LastIndex())
	}

	// Append entries
	log.Append(&LogEntry{Index: 1, Term: 1, Type: LogEntryCommand, Command: []byte("cmd1")})
	log.Append(&LogEntry{Index: 2, Term: 1, Type: LogEntryCommand, Command: []byte("cmd2")})
	log.Append(&LogEntry{Index: 3, Term: 2, Type: LogEntryCommand, Command: []byte("cmd3")})

	if log.Len() != 4 {
		t.Errorf("Log should have 4 entries, got %d", log.Len())
	}
	if log.LastIndex() != 3 {
		t.Errorf("LastIndex should be 3, got %d", log.LastIndex())
	}
	if log.LastTerm() != 2 {
		t.Errorf("LastTerm should be 2, got %d", log.LastTerm())
	}

	// Get entry
	entry, err := log.Get(2)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if string(entry.Command) != "cmd2" {
		t.Errorf("Wrong entry at index 2")
	}

	// Get out of range
	_, err = log.Get(100)
	if err != ErrLogIndexOutOfRange {
		t.Errorf("Expected ErrLogIndexOutOfRange, got %v", err)
	}

	// GetFrom
	entries := log.GetFrom(2)
	if len(entries) != 2 {
		t.Errorf("GetFrom(2) should return 2 entries, got %d", len(entries))
	}

	// TermAt
	if log.TermAt(1) != 1 {
		t.Errorf("TermAt(1) should be 1, got %d", log.TermAt(1))
	}
	if log.TermAt(3) != 2 {
		t.Errorf("TermAt(3) should be 2, got %d", log.TermAt(3))
	}

	// TruncateFrom
	log.TruncateFrom(2)
	if log.Len() != 2 {
		t.Errorf("After truncate, log should have 2 entries, got %d", log.Len())
	}
	if log.LastIndex() != 1 {
		t.Errorf("After truncate, LastIndex should be 1, got %d", log.LastIndex())
	}
}

func TestRaftLogTruncateEmpty(t *testing.T) {
	log := NewRaftLog()
	log.Append(&LogEntry{Index: 1, Term: 1, Type: LogEntryCommand})

	// Truncate beyond length - should be no-op
	log.TruncateFrom(100)
	if log.Len() != 2 {
		t.Errorf("Truncate beyond length should be no-op")
	}

	// Truncate at 0 - removes everything except initial
	log.TruncateFrom(0)
	if log.Len() != 0 {
		t.Errorf("Truncate at 0 should remove all entries")
	}
}
