// Package stream provides real-time change event streaming for ObaDB.
// It implements a pub/sub mechanism for LDAP entry changes with support
// for filtering, backpressure handling, and token-based resume.
package stream

import (
	"time"

	"github.com/KilimcininKorOglu/oba/internal/storage"
)

// OperationType represents the type of change operation.
type OperationType uint8

const (
	// OpInsert indicates a new entry was added.
	OpInsert OperationType = iota + 1
	// OpUpdate indicates an existing entry was modified.
	OpUpdate
	// OpDelete indicates an entry was removed.
	OpDelete
	// OpModifyDN indicates an entry's DN was changed.
	OpModifyDN
)

// String returns the string representation of the operation type.
func (op OperationType) String() string {
	switch op {
	case OpInsert:
		return "insert"
	case OpUpdate:
		return "update"
	case OpDelete:
		return "delete"
	case OpModifyDN:
		return "modifyDN"
	default:
		return "unknown"
	}
}

// ChangeEvent represents a single change to an LDAP entry.
type ChangeEvent struct {
	// Token is a monotonically increasing sequence number for resume support.
	Token uint64
	// Operation is the type of change (insert, update, delete, modifyDN).
	Operation OperationType
	// DN is the distinguished name of the affected entry.
	DN string
	// Entry contains the entry data (nil for delete operations).
	Entry *storage.Entry
	// OldDN contains the previous DN (only for modifyDN operations).
	OldDN string
	// Timestamp is when the event was published.
	Timestamp time.Time
}

// Clone creates a copy of the event.
func (e *ChangeEvent) Clone() *ChangeEvent {
	clone := &ChangeEvent{
		Token:     e.Token,
		Operation: e.Operation,
		DN:        e.DN,
		OldDN:     e.OldDN,
		Timestamp: e.Timestamp,
	}
	if e.Entry != nil {
		clone.Entry = e.Entry.Clone()
	}
	return clone
}
