package raft

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// Log entry types.
const (
	LogEntryCommand uint8 = iota // LDAP operation command
	LogEntryConfig               // Cluster configuration change
	LogEntryNoop                 // No-op entry for leader election
)

// LogEntry represents a single entry in the Raft log.
type LogEntry struct {
	Index   uint64 // Log index (1-based)
	Term    uint64 // Term when entry was created
	Type    uint8  // Entry type (LogEntryCommand, LogEntryConfig, LogEntryNoop)
	Command []byte // Serialized command data
}

// Serialize encodes the log entry to bytes.
// Format: [Index:8][Term:8][Type:1][CommandLen:4][Command:N]
func (e *LogEntry) Serialize() []byte {
	size := 8 + 8 + 1 + 4 + len(e.Command)
	buf := make([]byte, size)

	binary.LittleEndian.PutUint64(buf[0:8], e.Index)
	binary.LittleEndian.PutUint64(buf[8:16], e.Term)
	buf[16] = e.Type
	binary.LittleEndian.PutUint32(buf[17:21], uint32(len(e.Command)))
	copy(buf[21:], e.Command)

	return buf
}

// DeserializeLogEntry decodes a log entry from bytes.
func DeserializeLogEntry(data []byte) (*LogEntry, error) {
	if len(data) < 21 {
		return nil, ErrLogCorrupted
	}

	cmdLen := binary.LittleEndian.Uint32(data[17:21])
	if len(data) < 21+int(cmdLen) {
		return nil, ErrLogCorrupted
	}

	return &LogEntry{
		Index:   binary.LittleEndian.Uint64(data[0:8]),
		Term:    binary.LittleEndian.Uint64(data[8:16]),
		Type:    data[16],
		Command: data[21 : 21+cmdLen],
	}, nil
}

// Command types for LDAP operations and cluster config sync.
const (
	CmdPut           uint8 = iota // Add or modify entry
	CmdDelete                     // Delete entry
	CmdModifyDN                   // Rename/move entry
	CmdConfigUpdate               // Config section update
	CmdACLFullUpdate              // Full ACL config update
	CmdACLAddRule                 // Add single ACL rule
	CmdACLUpdateRule              // Update single ACL rule
	CmdACLDeleteRule              // Delete single ACL rule
	CmdACLSetDefault              // Set default ACL policy
)

// Database IDs for multi-database support.
const (
	DBMain uint8 = iota // Main LDAP database
	DBLog               // Log database
)

// ConfigCommand represents a config update command for Raft replication.
type ConfigCommand struct {
	Section string            // Config section: "logging", "server", "security.ratelimit", etc.
	Data    map[string]string // Section data as string key-value pairs
	Version uint64            // Config version for conflict detection
}

// SerializeConfigCommand encodes a ConfigCommand to bytes.
func SerializeConfigCommand(cmd *ConfigCommand) ([]byte, error) {
	var buf bytes.Buffer

	// Section
	if err := writeString(&buf, cmd.Section); err != nil {
		return nil, err
	}

	// Version
	if err := binary.Write(&buf, binary.LittleEndian, cmd.Version); err != nil {
		return nil, err
	}

	// Data map length
	if err := binary.Write(&buf, binary.LittleEndian, uint16(len(cmd.Data))); err != nil {
		return nil, err
	}

	// Data key-value pairs
	for k, v := range cmd.Data {
		if err := writeString(&buf, k); err != nil {
			return nil, err
		}
		if err := writeString(&buf, v); err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}

// DeserializeConfigCommand decodes a ConfigCommand from bytes.
func DeserializeConfigCommand(data []byte) (*ConfigCommand, error) {
	if len(data) < 2 {
		return nil, ErrLogCorrupted
	}

	buf := bytes.NewReader(data)
	cmd := &ConfigCommand{
		Data: make(map[string]string),
	}

	// Section
	var err error
	cmd.Section, err = readString(buf)
	if err != nil {
		return nil, ErrLogCorrupted
	}

	// Version
	if err := binary.Read(buf, binary.LittleEndian, &cmd.Version); err != nil {
		return nil, ErrLogCorrupted
	}

	// Data map length
	var mapLen uint16
	if err := binary.Read(buf, binary.LittleEndian, &mapLen); err != nil {
		return nil, ErrLogCorrupted
	}

	// Data key-value pairs
	for i := uint16(0); i < mapLen; i++ {
		key, err := readString(buf)
		if err != nil {
			return nil, ErrLogCorrupted
		}
		value, err := readString(buf)
		if err != nil {
			return nil, ErrLogCorrupted
		}
		cmd.Data[key] = value
	}

	return cmd, nil
}

// ACLRuleData represents a single ACL rule for serialization.
type ACLRuleData struct {
	Target     string   // Target DN pattern
	Subject    string   // Subject DN pattern
	Scope      string   // Scope: "base", "one", "subtree"
	Rights     []string // Rights: "read", "write", "add", "delete", "search", "compare", "all"
	Attributes []string // Attribute filter (empty = all)
	Deny       bool     // Deny rule flag
}

// ACLCommand represents an ACL update command for Raft replication.
type ACLCommand struct {
	DefaultPolicy string        // Default policy: "allow" or "deny"
	Rules         []ACLRuleData // Full ruleset (for full update)
	Rule          *ACLRuleData  // Single rule (for add/update)
	RuleIndex     int           // Rule index (for update/delete)
	Version       uint64        // ACL version for conflict detection
}

// SerializeACLCommand encodes an ACLCommand to bytes.
func SerializeACLCommand(cmd *ACLCommand) ([]byte, error) {
	var buf bytes.Buffer

	// DefaultPolicy
	if err := writeString(&buf, cmd.DefaultPolicy); err != nil {
		return nil, err
	}

	// Version
	if err := binary.Write(&buf, binary.LittleEndian, cmd.Version); err != nil {
		return nil, err
	}

	// RuleIndex
	if err := binary.Write(&buf, binary.LittleEndian, int32(cmd.RuleIndex)); err != nil {
		return nil, err
	}

	// Rules count
	if err := binary.Write(&buf, binary.LittleEndian, uint16(len(cmd.Rules))); err != nil {
		return nil, err
	}

	// Rules
	for _, rule := range cmd.Rules {
		if err := serializeACLRule(&buf, &rule); err != nil {
			return nil, err
		}
	}

	// Single rule (for add/update)
	hasRule := cmd.Rule != nil
	if err := binary.Write(&buf, binary.LittleEndian, hasRule); err != nil {
		return nil, err
	}
	if hasRule {
		if err := serializeACLRule(&buf, cmd.Rule); err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}

// DeserializeACLCommand decodes an ACLCommand from bytes.
func DeserializeACLCommand(data []byte) (*ACLCommand, error) {
	if len(data) < 2 {
		return nil, ErrLogCorrupted
	}

	buf := bytes.NewReader(data)
	cmd := &ACLCommand{}

	// DefaultPolicy
	var err error
	cmd.DefaultPolicy, err = readString(buf)
	if err != nil {
		return nil, ErrLogCorrupted
	}

	// Version
	if err := binary.Read(buf, binary.LittleEndian, &cmd.Version); err != nil {
		return nil, ErrLogCorrupted
	}

	// RuleIndex
	var ruleIndex int32
	if err := binary.Read(buf, binary.LittleEndian, &ruleIndex); err != nil {
		return nil, ErrLogCorrupted
	}
	cmd.RuleIndex = int(ruleIndex)

	// Rules count
	var rulesCount uint16
	if err := binary.Read(buf, binary.LittleEndian, &rulesCount); err != nil {
		return nil, ErrLogCorrupted
	}

	// Rules
	cmd.Rules = make([]ACLRuleData, rulesCount)
	for i := uint16(0); i < rulesCount; i++ {
		rule, err := deserializeACLRule(buf)
		if err != nil {
			return nil, err
		}
		cmd.Rules[i] = *rule
	}

	// Single rule
	var hasRule bool
	if err := binary.Read(buf, binary.LittleEndian, &hasRule); err != nil {
		return nil, ErrLogCorrupted
	}
	if hasRule {
		cmd.Rule, err = deserializeACLRule(buf)
		if err != nil {
			return nil, err
		}
	}

	return cmd, nil
}

func serializeACLRule(buf *bytes.Buffer, rule *ACLRuleData) error {
	// Target
	if err := writeString(buf, rule.Target); err != nil {
		return err
	}

	// Subject
	if err := writeString(buf, rule.Subject); err != nil {
		return err
	}

	// Scope
	if err := writeString(buf, rule.Scope); err != nil {
		return err
	}

	// Rights count and values
	if err := binary.Write(buf, binary.LittleEndian, uint16(len(rule.Rights))); err != nil {
		return err
	}
	for _, r := range rule.Rights {
		if err := writeString(buf, r); err != nil {
			return err
		}
	}

	// Attributes count and values
	if err := binary.Write(buf, binary.LittleEndian, uint16(len(rule.Attributes))); err != nil {
		return err
	}
	for _, a := range rule.Attributes {
		if err := writeString(buf, a); err != nil {
			return err
		}
	}

	// Deny flag
	if err := binary.Write(buf, binary.LittleEndian, rule.Deny); err != nil {
		return err
	}

	return nil
}

func deserializeACLRule(buf *bytes.Reader) (*ACLRuleData, error) {
	rule := &ACLRuleData{}

	// Target
	var err error
	rule.Target, err = readString(buf)
	if err != nil {
		return nil, ErrLogCorrupted
	}

	// Subject
	rule.Subject, err = readString(buf)
	if err != nil {
		return nil, ErrLogCorrupted
	}

	// Scope
	rule.Scope, err = readString(buf)
	if err != nil {
		return nil, ErrLogCorrupted
	}

	// Rights
	var rightsCount uint16
	if err := binary.Read(buf, binary.LittleEndian, &rightsCount); err != nil {
		return nil, ErrLogCorrupted
	}
	rule.Rights = make([]string, rightsCount)
	for i := uint16(0); i < rightsCount; i++ {
		rule.Rights[i], err = readString(buf)
		if err != nil {
			return nil, ErrLogCorrupted
		}
	}

	// Attributes
	var attrsCount uint16
	if err := binary.Read(buf, binary.LittleEndian, &attrsCount); err != nil {
		return nil, ErrLogCorrupted
	}
	rule.Attributes = make([]string, attrsCount)
	for i := uint16(0); i < attrsCount; i++ {
		rule.Attributes[i], err = readString(buf)
		if err != nil {
			return nil, ErrLogCorrupted
		}
	}

	// Deny flag
	if err := binary.Read(buf, binary.LittleEndian, &rule.Deny); err != nil {
		return nil, ErrLogCorrupted
	}

	return rule, nil
}

// Command represents an LDAP operation or config/ACL change to be replicated.
type Command struct {
	Type       uint8  // CmdPut, CmdDelete, CmdModifyDN, CmdConfigUpdate, CmdACL*
	DatabaseID uint8  // Target database (DBMain, DBLog)
	DN         string // Target DN
	OldDN      string // Previous DN (for CmdModifyDN)
	EntryDN    string // Entry DN
	EntryData  []byte // Serialized entry data (for CmdPut)
	ConfigData []byte // Serialized ConfigCommand (for CmdConfigUpdate)
	ACLData    []byte // Serialized ACLCommand (for CmdACL*)
}

// Serialize encodes the command to bytes.
func (c *Command) Serialize() ([]byte, error) {
	var buf bytes.Buffer

	// Type
	buf.WriteByte(c.Type)

	// DatabaseID
	buf.WriteByte(c.DatabaseID)

	// DN
	if err := writeString(&buf, c.DN); err != nil {
		return nil, err
	}

	// OldDN
	if err := writeString(&buf, c.OldDN); err != nil {
		return nil, err
	}

	// EntryDN
	if err := writeString(&buf, c.EntryDN); err != nil {
		return nil, err
	}

	// EntryData
	if err := writeBytes(&buf, c.EntryData); err != nil {
		return nil, err
	}

	// ConfigData
	if err := writeBytes(&buf, c.ConfigData); err != nil {
		return nil, err
	}

	// ACLData
	if err := writeBytes(&buf, c.ACLData); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// DeserializeCommand decodes a command from bytes.
func DeserializeCommand(data []byte) (*Command, error) {
	if len(data) < 2 {
		return nil, ErrLogCorrupted
	}

	buf := bytes.NewReader(data)
	cmd := &Command{}

	// Type
	var err error
	cmd.Type, err = buf.ReadByte()
	if err != nil {
		return nil, ErrLogCorrupted
	}

	// DatabaseID
	cmd.DatabaseID, err = buf.ReadByte()
	if err != nil {
		return nil, ErrLogCorrupted
	}

	// DN
	cmd.DN, err = readString(buf)
	if err != nil {
		return nil, ErrLogCorrupted
	}

	// OldDN
	cmd.OldDN, err = readString(buf)
	if err != nil {
		return nil, ErrLogCorrupted
	}

	// EntryDN
	cmd.EntryDN, err = readString(buf)
	if err != nil {
		return nil, ErrLogCorrupted
	}

	// EntryData
	cmd.EntryData, err = readBytes(buf)
	if err != nil {
		return nil, ErrLogCorrupted
	}

	// ConfigData (may not exist in old format)
	cmd.ConfigData, err = readBytes(buf)
	if err != nil {
		// Old format without ConfigData, ignore
		cmd.ConfigData = nil
	}

	// ACLData (may not exist in old format)
	cmd.ACLData, err = readBytes(buf)
	if err != nil {
		// Old format without ACLData, ignore
		cmd.ACLData = nil
	}

	return cmd, nil
}

// Helper functions for serialization

func writeString(w io.Writer, s string) error {
	data := []byte(s)
	if err := binary.Write(w, binary.LittleEndian, uint16(len(data))); err != nil {
		return err
	}
	_, err := w.Write(data)
	return err
}

func readString(r io.Reader) (string, error) {
	var length uint16
	if err := binary.Read(r, binary.LittleEndian, &length); err != nil {
		return "", err
	}
	data := make([]byte, length)
	if _, err := io.ReadFull(r, data); err != nil {
		return "", err
	}
	return string(data), nil
}

func writeBytes(w io.Writer, data []byte) error {
	if err := binary.Write(w, binary.LittleEndian, uint32(len(data))); err != nil {
		return err
	}
	if len(data) > 0 {
		_, err := w.Write(data)
		return err
	}
	return nil
}

func readBytes(r io.Reader) ([]byte, error) {
	var length uint32
	if err := binary.Read(r, binary.LittleEndian, &length); err != nil {
		return nil, err
	}
	if length == 0 {
		return nil, nil
	}
	data := make([]byte, length)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, err
	}
	return data, nil
}

// RaftLog manages the Raft log entries with disk persistence.
type RaftLog struct {
	mu      sync.RWMutex
	entries []*LogEntry
	dataDir string   // Directory for persistence
	file    *os.File // Log file handle
}

// NewRaftLog creates a new Raft log with an initial noop entry at index 0.
func NewRaftLog() *RaftLog {
	return &RaftLog{
		entries: []*LogEntry{
			{Index: 0, Term: 0, Type: LogEntryNoop},
		},
	}
}

// NewRaftLogWithDir creates a new Raft log with disk persistence.
func NewRaftLogWithDir(dataDir string) (*RaftLog, error) {
	l := &RaftLog{
		entries: []*LogEntry{
			{Index: 0, Term: 0, Type: LogEntryNoop},
		},
		dataDir: dataDir,
	}

	if dataDir != "" {
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			return nil, err
		}

		// Open or create log file
		logPath := filepath.Join(dataDir, "raft.log")
		f, err := os.OpenFile(logPath, os.O_RDWR|os.O_CREATE, 0644)
		if err != nil {
			return nil, err
		}
		l.file = f

		// Load existing entries from disk
		if err := l.loadFromDisk(); err != nil {
			f.Close()
			return nil, err
		}
	}

	return l, nil
}

// loadFromDisk loads log entries from the log file.
func (l *RaftLog) loadFromDisk() error {
	if l.file == nil {
		return nil
	}

	// Seek to beginning
	if _, err := l.file.Seek(0, 0); err != nil {
		return err
	}

	// Read all entries into a map to handle duplicates
	entryMap := make(map[uint64]*LogEntry)
	var maxIndex uint64 = 0

	for {
		// Read entry length (4 bytes)
		lenBuf := make([]byte, 4)
		_, err := io.ReadFull(l.file, lenBuf)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		entryLen := binary.LittleEndian.Uint32(lenBuf)
		if entryLen == 0 || entryLen > 10*1024*1024 { // Max 10MB per entry
			break
		}

		// Read entry data
		entryData := make([]byte, entryLen)
		_, err = io.ReadFull(l.file, entryData)
		if err != nil {
			return err
		}

		// Deserialize entry
		entry, err := DeserializeLogEntry(entryData)
		if err != nil {
			return err
		}

		// Store in map (later entries overwrite earlier ones for same index)
		if entry.Index > 0 {
			entryMap[entry.Index] = entry
			if entry.Index > maxIndex {
				maxIndex = entry.Index
			}
		}
	}

	// Build entries array from map
	for i := uint64(1); i <= maxIndex; i++ {
		if entry, ok := entryMap[i]; ok {
			for uint64(len(l.entries)) <= i {
				l.entries = append(l.entries, nil)
			}
			l.entries[i] = entry
		}
	}

	// Compact the log file - rewrite without duplicates
	if err := l.compactLogFile(); err != nil {
		return err
	}

	return nil
}

// compactLogFile rewrites the log file without duplicates.
// For LDAP commands (PUT, DELETE, MODIFYDN), keeps only the last command for each DN.
func (l *RaftLog) compactLogFile() error {
	// DISABLED: Log compaction was re-indexing entries which breaks Raft protocol.
	// Raft log indices must be stable. Compaction should only remove old entries,
	// not re-index them.
	// TODO: Implement proper log compaction with snapshots.
	return nil
}

// persistEntry writes a single entry to disk.
func (l *RaftLog) persistEntry(entry *LogEntry) error {
	if l.file == nil {
		return nil
	}

	data := entry.Serialize()
	lenBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(lenBuf, uint32(len(data)))

	// Write length + data
	if _, err := l.file.Write(lenBuf); err != nil {
		return err
	}
	if _, err := l.file.Write(data); err != nil {
		return err
	}

	// Sync to disk
	return l.file.Sync()
}

// Close closes the log file.
func (l *RaftLog) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// Append adds a new entry to the log and persists it to disk.
func (l *RaftLog) Append(entry *LogEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, entry)

	// Persist to disk
	if l.file != nil {
		l.persistEntryLocked(entry)
	}
}

// persistEntryLocked writes a single entry to disk (must hold lock).
func (l *RaftLog) persistEntryLocked(entry *LogEntry) error {
	if l.file == nil {
		return nil
	}

	data := entry.Serialize()
	lenBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(lenBuf, uint32(len(data)))

	// Write length + data
	if _, err := l.file.Write(lenBuf); err != nil {
		return err
	}
	if _, err := l.file.Write(data); err != nil {
		return err
	}

	// Sync to disk
	return l.file.Sync()
}

// Get returns the entry at the given index.
func (l *RaftLog) Get(index uint64) (*LogEntry, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if index >= uint64(len(l.entries)) {
		return nil, ErrLogIndexOutOfRange
	}
	return l.entries[index], nil
}

// LastIndex returns the index of the last entry.
func (l *RaftLog) LastIndex() uint64 {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return uint64(len(l.entries) - 1)
}

// LastTerm returns the term of the last entry.
func (l *RaftLog) LastTerm() uint64 {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if len(l.entries) == 0 {
		return 0
	}
	lastEntry := l.entries[len(l.entries)-1]
	if lastEntry == nil {
		return 0
	}
	return lastEntry.Term
}

// GetFrom returns all entries from the given index.
func (l *RaftLog) GetFrom(index uint64) []*LogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if index >= uint64(len(l.entries)) {
		return nil
	}
	// Return a copy to avoid race conditions
	result := make([]*LogEntry, len(l.entries)-int(index))
	copy(result, l.entries[index:])
	return result
}

// TruncateFrom removes all entries from the given index onwards.
func (l *RaftLog) TruncateFrom(index uint64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if index < uint64(len(l.entries)) {
		l.entries = l.entries[:index]
	}
}

// TruncateBefore removes all entries before the given index.
// Used after snapshot restore to discard entries included in the snapshot.
func (l *RaftLog) TruncateBefore(index uint64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if index > 0 && index <= uint64(len(l.entries)) {
		l.entries = l.entries[index:]
	} else if index > uint64(len(l.entries)) {
		// If index is beyond log, clear all entries
		l.entries = nil
	}
}

// Len returns the number of entries in the log.
func (l *RaftLog) Len() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.entries)
}

// TermAt returns the term of the entry at the given index.
func (l *RaftLog) TermAt(index uint64) uint64 {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if index >= uint64(len(l.entries)) {
		return 0
	}
	if l.entries[index] == nil {
		return 0
	}
	return l.entries[index].Term
}

// CreateConfigUpdateCommand creates a Raft command for config update.
func CreateConfigUpdateCommand(section string, data map[string]string, version uint64) (*Command, error) {
	configCmd := &ConfigCommand{
		Section: section,
		Data:    data,
		Version: version,
	}

	configData, err := SerializeConfigCommand(configCmd)
	if err != nil {
		return nil, err
	}

	return &Command{
		Type:       CmdConfigUpdate,
		ConfigData: configData,
	}, nil
}

// CreateACLFullUpdateCommand creates a Raft command for full ACL update.
func CreateACLFullUpdateCommand(rules []ACLRuleData, defaultPolicy string, version uint64) (*Command, error) {
	aclCmd := &ACLCommand{
		DefaultPolicy: defaultPolicy,
		Rules:         rules,
		Version:       version,
	}

	aclData, err := SerializeACLCommand(aclCmd)
	if err != nil {
		return nil, err
	}

	return &Command{
		Type:    CmdACLFullUpdate,
		ACLData: aclData,
	}, nil
}

// CreateACLAddRuleCommand creates a Raft command for adding an ACL rule.
func CreateACLAddRuleCommand(rule *ACLRuleData, index int, version uint64) (*Command, error) {
	aclCmd := &ACLCommand{
		Rule:      rule,
		RuleIndex: index,
		Version:   version,
	}

	aclData, err := SerializeACLCommand(aclCmd)
	if err != nil {
		return nil, err
	}

	return &Command{
		Type:    CmdACLAddRule,
		ACLData: aclData,
	}, nil
}

// CreateACLUpdateRuleCommand creates a Raft command for updating an ACL rule.
func CreateACLUpdateRuleCommand(rule *ACLRuleData, index int, version uint64) (*Command, error) {
	aclCmd := &ACLCommand{
		Rule:      rule,
		RuleIndex: index,
		Version:   version,
	}

	aclData, err := SerializeACLCommand(aclCmd)
	if err != nil {
		return nil, err
	}

	return &Command{
		Type:    CmdACLUpdateRule,
		ACLData: aclData,
	}, nil
}

// CreateACLDeleteRuleCommand creates a Raft command for deleting an ACL rule.
func CreateACLDeleteRuleCommand(index int, version uint64) (*Command, error) {
	aclCmd := &ACLCommand{
		RuleIndex: index,
		Version:   version,
	}

	aclData, err := SerializeACLCommand(aclCmd)
	if err != nil {
		return nil, err
	}

	return &Command{
		Type:    CmdACLDeleteRule,
		ACLData: aclData,
	}, nil
}

// CreateACLSetDefaultCommand creates a Raft command for setting default ACL policy.
func CreateACLSetDefaultCommand(defaultPolicy string, version uint64) (*Command, error) {
	aclCmd := &ACLCommand{
		DefaultPolicy: defaultPolicy,
		Version:       version,
	}

	aclData, err := SerializeACLCommand(aclCmd)
	if err != nil {
		return nil, err
	}

	return &Command{
		Type:    CmdACLSetDefault,
		ACLData: aclData,
	}, nil
}
