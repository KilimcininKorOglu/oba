// Package tx provides transaction management for ObaDB.
package tx

import (
	"sync"
	"time"

	"github.com/oba-ldap/oba/internal/storage"
)

// TxState represents the state of a transaction.
type TxState int

const (
	// TxActive indicates the transaction is currently active.
	TxActive TxState = iota
	// TxCommitted indicates the transaction has been successfully committed.
	TxCommitted
	// TxAborted indicates the transaction has been rolled back.
	TxAborted
)

// String returns the string representation of a TxState.
func (s TxState) String() string {
	switch s {
	case TxActive:
		return "Active"
	case TxCommitted:
		return "Committed"
	case TxAborted:
		return "Aborted"
	default:
		return "Unknown"
	}
}

// Transaction represents a database transaction.
// It tracks the transaction lifecycle, read/write sets, and snapshot information.
type Transaction struct {
	// ID is the unique transaction identifier.
	ID uint64

	// State is the current state of the transaction.
	State TxState

	// StartTime is when the transaction began.
	StartTime time.Time

	// StartLSN is the WAL position at transaction start.
	StartLSN uint64

	// ReadSet contains the pages read during this transaction (for validation).
	ReadSet []storage.PageID

	// WriteSet contains the pages modified during this transaction.
	WriteSet []storage.PageID

	// Snapshot is the snapshot timestamp for MVCC.
	Snapshot uint64

	// mu protects concurrent access to the transaction.
	mu sync.RWMutex
}

// NewTransaction creates a new transaction with the given ID and start LSN.
func NewTransaction(id, startLSN uint64) *Transaction {
	return &Transaction{
		ID:        id,
		State:     TxActive,
		StartTime: time.Now(),
		StartLSN:  startLSN,
		ReadSet:   make([]storage.PageID, 0),
		WriteSet:  make([]storage.PageID, 0),
		Snapshot:  id, // Use transaction ID as snapshot timestamp
	}
}

// IsActive returns true if the transaction is still active.
func (tx *Transaction) IsActive() bool {
	tx.mu.RLock()
	defer tx.mu.RUnlock()
	return tx.State == TxActive
}

// IsCommitted returns true if the transaction has been committed.
func (tx *Transaction) IsCommitted() bool {
	tx.mu.RLock()
	defer tx.mu.RUnlock()
	return tx.State == TxCommitted
}

// IsAborted returns true if the transaction has been aborted.
func (tx *Transaction) IsAborted() bool {
	tx.mu.RLock()
	defer tx.mu.RUnlock()
	return tx.State == TxAborted
}

// AddToReadSet adds a page to the transaction's read set.
func (tx *Transaction) AddToReadSet(pageID storage.PageID) {
	tx.mu.Lock()
	defer tx.mu.Unlock()

	// Check if already in read set
	for _, p := range tx.ReadSet {
		if p == pageID {
			return
		}
	}
	tx.ReadSet = append(tx.ReadSet, pageID)
}

// AddToWriteSet adds a page to the transaction's write set.
func (tx *Transaction) AddToWriteSet(pageID storage.PageID) {
	tx.mu.Lock()
	defer tx.mu.Unlock()

	// Check if already in write set
	for _, p := range tx.WriteSet {
		if p == pageID {
			return
		}
	}
	tx.WriteSet = append(tx.WriteSet, pageID)
}

// GetReadSet returns a copy of the read set.
func (tx *Transaction) GetReadSet() []storage.PageID {
	tx.mu.RLock()
	defer tx.mu.RUnlock()

	result := make([]storage.PageID, len(tx.ReadSet))
	copy(result, tx.ReadSet)
	return result
}

// GetWriteSet returns a copy of the write set.
func (tx *Transaction) GetWriteSet() []storage.PageID {
	tx.mu.RLock()
	defer tx.mu.RUnlock()

	result := make([]storage.PageID, len(tx.WriteSet))
	copy(result, tx.WriteSet)
	return result
}

// ClearWriteSet clears the write set (used during rollback).
func (tx *Transaction) ClearWriteSet() {
	tx.mu.Lock()
	defer tx.mu.Unlock()
	tx.WriteSet = make([]storage.PageID, 0)
}

// ClearReadSet clears the read set.
func (tx *Transaction) ClearReadSet() {
	tx.mu.Lock()
	defer tx.mu.Unlock()
	tx.ReadSet = make([]storage.PageID, 0)
}

// SetState sets the transaction state.
func (tx *Transaction) SetState(state TxState) {
	tx.mu.Lock()
	defer tx.mu.Unlock()
	tx.State = state
}

// Duration returns the duration since the transaction started.
func (tx *Transaction) Duration() time.Duration {
	tx.mu.RLock()
	defer tx.mu.RUnlock()
	return time.Since(tx.StartTime)
}

// Clone creates a deep copy of the transaction (for inspection purposes).
func (tx *Transaction) Clone() *Transaction {
	tx.mu.RLock()
	defer tx.mu.RUnlock()

	clone := &Transaction{
		ID:        tx.ID,
		State:     tx.State,
		StartTime: tx.StartTime,
		StartLSN:  tx.StartLSN,
		Snapshot:  tx.Snapshot,
		ReadSet:   make([]storage.PageID, len(tx.ReadSet)),
		WriteSet:  make([]storage.PageID, len(tx.WriteSet)),
	}
	copy(clone.ReadSet, tx.ReadSet)
	copy(clone.WriteSet, tx.WriteSet)
	return clone
}
