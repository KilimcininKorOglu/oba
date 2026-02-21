package raft

import (
	"bytes"
	"encoding/binary"
	"io"
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

// Command types for LDAP operations.
const (
	CmdPut      uint8 = iota // Add or modify entry
	CmdDelete                // Delete entry
	CmdModifyDN              // Rename/move entry
)

// Database IDs for multi-database support.
const (
	DBMain uint8 = iota // Main LDAP database
	DBLog               // Log database
)

// Command represents an LDAP operation to be replicated.
type Command struct {
	Type      uint8  // CmdPut, CmdDelete, CmdModifyDN
	DatabaseID uint8 // Target database (DBMain, DBLog)
	DN        string // Target DN
	OldDN     string // Previous DN (for CmdModifyDN)
	EntryDN   string // Entry DN
	EntryData []byte // Serialized entry data (for CmdPut)
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

// RaftLog manages the Raft log entries.
type RaftLog struct {
	entries []*LogEntry
}

// NewRaftLog creates a new Raft log with an initial noop entry at index 0.
func NewRaftLog() *RaftLog {
	return &RaftLog{
		entries: []*LogEntry{
			{Index: 0, Term: 0, Type: LogEntryNoop},
		},
	}
}

// Append adds a new entry to the log.
func (l *RaftLog) Append(entry *LogEntry) {
	l.entries = append(l.entries, entry)
}

// Get returns the entry at the given index.
func (l *RaftLog) Get(index uint64) (*LogEntry, error) {
	if index >= uint64(len(l.entries)) {
		return nil, ErrLogIndexOutOfRange
	}
	return l.entries[index], nil
}

// LastIndex returns the index of the last entry.
func (l *RaftLog) LastIndex() uint64 {
	return uint64(len(l.entries) - 1)
}

// LastTerm returns the term of the last entry.
func (l *RaftLog) LastTerm() uint64 {
	if len(l.entries) == 0 {
		return 0
	}
	return l.entries[len(l.entries)-1].Term
}

// GetFrom returns all entries from the given index.
func (l *RaftLog) GetFrom(index uint64) []*LogEntry {
	if index >= uint64(len(l.entries)) {
		return nil
	}
	return l.entries[index:]
}

// TruncateFrom removes all entries from the given index onwards.
func (l *RaftLog) TruncateFrom(index uint64) {
	if index < uint64(len(l.entries)) {
		l.entries = l.entries[:index]
	}
}

// Len returns the number of entries in the log.
func (l *RaftLog) Len() int {
	return len(l.entries)
}

// TermAt returns the term of the entry at the given index.
func (l *RaftLog) TermAt(index uint64) uint64 {
	if index >= uint64(len(l.entries)) {
		return 0
	}
	return l.entries[index].Term
}
