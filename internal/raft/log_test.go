package raft

import (
	"bytes"
	"os"
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

func TestConfigCommandSerialization(t *testing.T) {
	cmd := &ConfigCommand{
		Section: "logging",
		Data: map[string]string{
			"level":  "debug",
			"format": "json",
		},
		Version: 42,
	}

	// Serialize
	data, err := SerializeConfigCommand(cmd)
	if err != nil {
		t.Fatalf("SerializeConfigCommand failed: %v", err)
	}

	// Deserialize
	restored, err := DeserializeConfigCommand(data)
	if err != nil {
		t.Fatalf("DeserializeConfigCommand failed: %v", err)
	}

	// Verify
	if restored.Section != cmd.Section {
		t.Errorf("Section mismatch: got %s, want %s", restored.Section, cmd.Section)
	}
	if restored.Version != cmd.Version {
		t.Errorf("Version mismatch: got %d, want %d", restored.Version, cmd.Version)
	}
	if len(restored.Data) != len(cmd.Data) {
		t.Errorf("Data length mismatch: got %d, want %d", len(restored.Data), len(cmd.Data))
	}
	for k, v := range cmd.Data {
		if restored.Data[k] != v {
			t.Errorf("Data[%s] mismatch: got %s, want %s", k, restored.Data[k], v)
		}
	}
}

func TestACLCommandSerialization(t *testing.T) {
	cmd := &ACLCommand{
		DefaultPolicy: "deny",
		Rules: []ACLRuleData{
			{
				Target:     "dc=example,dc=com",
				Subject:    "cn=admin,dc=example,dc=com",
				Scope:      "subtree",
				Rights:     []string{"read", "write", "add", "delete"},
				Attributes: []string{"*"},
				Deny:       false,
			},
			{
				Target:     "ou=users,dc=example,dc=com",
				Subject:    "*",
				Scope:      "one",
				Rights:     []string{"read", "search"},
				Attributes: []string{"cn", "mail", "uid"},
				Deny:       false,
			},
		},
		Version: 10,
	}

	// Serialize
	data, err := SerializeACLCommand(cmd)
	if err != nil {
		t.Fatalf("SerializeACLCommand failed: %v", err)
	}

	// Deserialize
	restored, err := DeserializeACLCommand(data)
	if err != nil {
		t.Fatalf("DeserializeACLCommand failed: %v", err)
	}

	// Verify
	if restored.DefaultPolicy != cmd.DefaultPolicy {
		t.Errorf("DefaultPolicy mismatch: got %s, want %s", restored.DefaultPolicy, cmd.DefaultPolicy)
	}
	if restored.Version != cmd.Version {
		t.Errorf("Version mismatch: got %d, want %d", restored.Version, cmd.Version)
	}
	if len(restored.Rules) != len(cmd.Rules) {
		t.Fatalf("Rules length mismatch: got %d, want %d", len(restored.Rules), len(cmd.Rules))
	}

	for i, rule := range cmd.Rules {
		restoredRule := restored.Rules[i]
		if restoredRule.Target != rule.Target {
			t.Errorf("Rule[%d].Target mismatch: got %s, want %s", i, restoredRule.Target, rule.Target)
		}
		if restoredRule.Subject != rule.Subject {
			t.Errorf("Rule[%d].Subject mismatch: got %s, want %s", i, restoredRule.Subject, rule.Subject)
		}
		if restoredRule.Scope != rule.Scope {
			t.Errorf("Rule[%d].Scope mismatch: got %s, want %s", i, restoredRule.Scope, rule.Scope)
		}
		if restoredRule.Deny != rule.Deny {
			t.Errorf("Rule[%d].Deny mismatch: got %v, want %v", i, restoredRule.Deny, rule.Deny)
		}
		if len(restoredRule.Rights) != len(rule.Rights) {
			t.Errorf("Rule[%d].Rights length mismatch", i)
		}
		if len(restoredRule.Attributes) != len(rule.Attributes) {
			t.Errorf("Rule[%d].Attributes length mismatch", i)
		}
	}
}

func TestACLCommandWithSingleRule(t *testing.T) {
	rule := &ACLRuleData{
		Target:     "ou=groups,dc=example,dc=com",
		Subject:    "cn=groupadmin,dc=example,dc=com",
		Scope:      "subtree",
		Rights:     []string{"all"},
		Attributes: nil,
		Deny:       false,
	}

	cmd := &ACLCommand{
		Rule:      rule,
		RuleIndex: 5,
		Version:   15,
	}

	// Serialize
	data, err := SerializeACLCommand(cmd)
	if err != nil {
		t.Fatalf("SerializeACLCommand failed: %v", err)
	}

	// Deserialize
	restored, err := DeserializeACLCommand(data)
	if err != nil {
		t.Fatalf("DeserializeACLCommand failed: %v", err)
	}

	// Verify
	if restored.RuleIndex != cmd.RuleIndex {
		t.Errorf("RuleIndex mismatch: got %d, want %d", restored.RuleIndex, cmd.RuleIndex)
	}
	if restored.Rule == nil {
		t.Fatal("Rule should not be nil")
	}
	if restored.Rule.Target != rule.Target {
		t.Errorf("Rule.Target mismatch: got %s, want %s", restored.Rule.Target, rule.Target)
	}
}

func TestCreateConfigUpdateCommand(t *testing.T) {
	data := map[string]string{
		"level":  "info",
		"format": "text",
	}

	cmd, err := CreateConfigUpdateCommand("logging", data, 1)
	if err != nil {
		t.Fatalf("CreateConfigUpdateCommand failed: %v", err)
	}

	if cmd.Type != CmdConfigUpdate {
		t.Errorf("Type mismatch: got %d, want %d", cmd.Type, CmdConfigUpdate)
	}
	if len(cmd.ConfigData) == 0 {
		t.Error("ConfigData should not be empty")
	}

	// Verify we can deserialize the config data
	configCmd, err := DeserializeConfigCommand(cmd.ConfigData)
	if err != nil {
		t.Fatalf("DeserializeConfigCommand failed: %v", err)
	}
	if configCmd.Section != "logging" {
		t.Errorf("Section mismatch: got %s, want logging", configCmd.Section)
	}
}

func TestCreateACLFullUpdateCommand(t *testing.T) {
	rules := []ACLRuleData{
		{
			Target:  "dc=example,dc=com",
			Subject: "*",
			Scope:   "subtree",
			Rights:  []string{"read"},
		},
	}

	cmd, err := CreateACLFullUpdateCommand(rules, "deny", 1)
	if err != nil {
		t.Fatalf("CreateACLFullUpdateCommand failed: %v", err)
	}

	if cmd.Type != CmdACLFullUpdate {
		t.Errorf("Type mismatch: got %d, want %d", cmd.Type, CmdACLFullUpdate)
	}
	if len(cmd.ACLData) == 0 {
		t.Error("ACLData should not be empty")
	}
}

func TestCreateACLAddRuleCommand(t *testing.T) {
	rule := &ACLRuleData{
		Target:  "ou=users,dc=example,dc=com",
		Subject: "*",
		Scope:   "one",
		Rights:  []string{"read", "search"},
	}

	cmd, err := CreateACLAddRuleCommand(rule, 0, 1)
	if err != nil {
		t.Fatalf("CreateACLAddRuleCommand failed: %v", err)
	}

	if cmd.Type != CmdACLAddRule {
		t.Errorf("Type mismatch: got %d, want %d", cmd.Type, CmdACLAddRule)
	}
}

func TestCreateACLDeleteRuleCommand(t *testing.T) {
	cmd, err := CreateACLDeleteRuleCommand(3, 5)
	if err != nil {
		t.Fatalf("CreateACLDeleteRuleCommand failed: %v", err)
	}

	if cmd.Type != CmdACLDeleteRule {
		t.Errorf("Type mismatch: got %d, want %d", cmd.Type, CmdACLDeleteRule)
	}

	// Verify the index is preserved
	aclCmd, err := DeserializeACLCommand(cmd.ACLData)
	if err != nil {
		t.Fatalf("DeserializeACLCommand failed: %v", err)
	}
	if aclCmd.RuleIndex != 3 {
		t.Errorf("RuleIndex mismatch: got %d, want 3", aclCmd.RuleIndex)
	}
}

func TestCreateACLSetDefaultCommand(t *testing.T) {
	cmd, err := CreateACLSetDefaultCommand("allow", 1)
	if err != nil {
		t.Fatalf("CreateACLSetDefaultCommand failed: %v", err)
	}

	if cmd.Type != CmdACLSetDefault {
		t.Errorf("Type mismatch: got %d, want %d", cmd.Type, CmdACLSetDefault)
	}

	// Verify the policy is preserved
	aclCmd, err := DeserializeACLCommand(cmd.ACLData)
	if err != nil {
		t.Fatalf("DeserializeACLCommand failed: %v", err)
	}
	if aclCmd.DefaultPolicy != "allow" {
		t.Errorf("DefaultPolicy mismatch: got %s, want allow", aclCmd.DefaultPolicy)
	}
}

func TestCommandWithConfigData(t *testing.T) {
	configCmd := &ConfigCommand{
		Section: "server",
		Data: map[string]string{
			"maxConnections": "5000",
		},
		Version: 1,
	}

	configData, err := SerializeConfigCommand(configCmd)
	if err != nil {
		t.Fatalf("SerializeConfigCommand failed: %v", err)
	}

	cmd := &Command{
		Type:       CmdConfigUpdate,
		ConfigData: configData,
	}

	// Serialize full command
	data, err := cmd.Serialize()
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	// Deserialize
	restored, err := DeserializeCommand(data)
	if err != nil {
		t.Fatalf("DeserializeCommand failed: %v", err)
	}

	if restored.Type != CmdConfigUpdate {
		t.Errorf("Type mismatch: got %d, want %d", restored.Type, CmdConfigUpdate)
	}
	if !bytes.Equal(restored.ConfigData, configData) {
		t.Error("ConfigData mismatch")
	}
}

func TestCommandWithACLData(t *testing.T) {
	aclCmd := &ACLCommand{
		DefaultPolicy: "deny",
		Version:       1,
	}

	aclData, err := SerializeACLCommand(aclCmd)
	if err != nil {
		t.Fatalf("SerializeACLCommand failed: %v", err)
	}

	cmd := &Command{
		Type:    CmdACLSetDefault,
		ACLData: aclData,
	}

	// Serialize full command
	data, err := cmd.Serialize()
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	// Deserialize
	restored, err := DeserializeCommand(data)
	if err != nil {
		t.Fatalf("DeserializeCommand failed: %v", err)
	}

	if restored.Type != CmdACLSetDefault {
		t.Errorf("Type mismatch: got %d, want %d", restored.Type, CmdACLSetDefault)
	}
	if !bytes.Equal(restored.ACLData, aclData) {
		t.Error("ACLData mismatch")
	}
}

func TestLogBasicOperations(t *testing.T) {
	log := NewRaftLog()

	// Test initial state
	if log.LastIndex() != 0 {
		t.Errorf("Initial LastIndex should be 0, got %d", log.LastIndex())
	}

	// Append and verify
	log.Append(&LogEntry{Index: 1, Term: 1, Type: LogEntryCommand})
	if log.LastIndex() != 1 {
		t.Errorf("LastIndex should be 1, got %d", log.LastIndex())
	}
}

func TestRaftLogPersistence(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "raft-log-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create log with persistence
	log1, err := NewRaftLogWithDir(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Add some entries
	log1.Append(&LogEntry{Index: 1, Term: 1, Type: LogEntryCommand, Command: []byte("cmd1")})
	log1.Append(&LogEntry{Index: 2, Term: 1, Type: LogEntryCommand, Command: []byte("cmd2")})
	log1.Append(&LogEntry{Index: 3, Term: 2, Type: LogEntryCommand, Command: []byte("cmd3")})

	// Close the log
	log1.Close()

	// Reopen the log
	log2, err := NewRaftLogWithDir(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer log2.Close()

	// Verify entries were loaded
	if log2.LastIndex() != 3 {
		t.Errorf("Expected LastIndex 3, got %d", log2.LastIndex())
	}

	entry, err := log2.Get(1)
	if err != nil {
		t.Fatal(err)
	}
	if string(entry.Command) != "cmd1" {
		t.Errorf("Expected cmd1, got %s", string(entry.Command))
	}

	entry, err = log2.Get(3)
	if err != nil {
		t.Fatal(err)
	}
	if entry.Term != 2 {
		t.Errorf("Expected term 2, got %d", entry.Term)
	}
}
