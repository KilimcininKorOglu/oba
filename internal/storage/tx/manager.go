// Package tx provides transaction management for ObaDB.
package tx

import (
	"errors"
	"sync"
	"sync/atomic"

	"github.com/oba-ldap/oba/internal/storage"
)

// Transaction manager errors.
var (
	ErrTxNotFound      = errors.New("transaction not found")
	ErrTxNotActive     = errors.New("transaction is not active")
	ErrTxAlreadyEnded  = errors.New("transaction has already ended")
	ErrWALWriteFailed  = errors.New("failed to write WAL record")
	ErrWALSyncFailed   = errors.New("failed to sync WAL")
	ErrWriteConflict   = errors.New("write conflict detected")
	ErrNilWAL          = errors.New("WAL is nil")
	ErrNilTransaction  = errors.New("transaction is nil")
)

// TxManager manages transaction lifecycle: begin, commit, and rollback.
// It assigns unique transaction IDs, tracks active transactions, and
// manages the commit protocol.
type TxManager struct {
	// nextTxID is the next transaction ID to assign (atomic).
	nextTxID uint64

	// activeTx maps transaction IDs to active transactions.
	activeTx map[uint64]*Transaction

	// wal is the Write-Ahead Log for durability.
	wal *storage.WAL

	// mu protects activeTx map.
	mu sync.RWMutex

	// commitMu serializes commits to prevent conflicts.
	commitMu sync.Mutex
}

// NewTxManager creates a new transaction manager with the given WAL.
func NewTxManager(wal *storage.WAL) *TxManager {
	return &TxManager{
		nextTxID: 1, // Start from 1, 0 is reserved
		activeTx: make(map[uint64]*Transaction),
		wal:      wal,
	}
}

// Begin starts a new transaction and returns it.
// The transaction is assigned a unique, monotonically increasing ID.
func (tm *TxManager) Begin() (*Transaction, error) {
	if tm.wal == nil {
		return nil, ErrNilWAL
	}

	// Atomically get and increment the next transaction ID
	txID := atomic.AddUint64(&tm.nextTxID, 1) - 1

	// Get current WAL LSN for the transaction start
	startLSN := tm.wal.CurrentLSN()

	// Create the transaction
	tx := NewTransaction(txID, startLSN)

	// Write BEGIN record to WAL
	beginRecord := storage.NewWALRecord(0, txID, storage.WALBegin)
	_, err := tm.wal.Append(beginRecord)
	if err != nil {
		return nil, ErrWALWriteFailed
	}

	// Add to active transactions
	tm.mu.Lock()
	tm.activeTx[txID] = tx
	tm.mu.Unlock()

	return tx, nil
}

// Commit commits the transaction, making all changes durable.
// The commit protocol:
// 1. Validate write set (no conflicts)
// 2. Write commit record to WAL
// 3. Sync WAL to disk
// 4. Mark transaction as committed
// 5. Remove from active transactions
func (tm *TxManager) Commit(tx *Transaction) error {
	if tx == nil {
		return ErrNilTransaction
	}

	if tm.wal == nil {
		return ErrNilWAL
	}

	// Check if transaction is active
	if !tx.IsActive() {
		return ErrTxNotActive
	}

	// Serialize commits to prevent conflicts
	tm.commitMu.Lock()
	defer tm.commitMu.Unlock()

	// Verify transaction is still in active set
	tm.mu.RLock()
	_, exists := tm.activeTx[tx.ID]
	tm.mu.RUnlock()

	if !exists {
		return ErrTxNotFound
	}

	// Validate write set (check for conflicts with other committed transactions)
	if err := tm.validateWriteSet(tx); err != nil {
		return err
	}

	// Write COMMIT record to WAL
	commitRecord := storage.NewWALRecord(0, tx.ID, storage.WALCommit)
	_, err := tm.wal.Append(commitRecord)
	if err != nil {
		return ErrWALWriteFailed
	}

	// Sync WAL to disk for durability
	if err := tm.wal.Sync(); err != nil {
		return ErrWALSyncFailed
	}

	// Mark transaction as committed
	tx.SetState(TxCommitted)

	// Remove from active transactions
	tm.mu.Lock()
	delete(tm.activeTx, tx.ID)
	tm.mu.Unlock()

	return nil
}

// Rollback aborts the transaction and undoes all changes.
// The rollback process:
// 1. Write abort record to WAL
// 2. Clear the write set
// 3. Mark transaction as aborted
// 4. Remove from active transactions
func (tm *TxManager) Rollback(tx *Transaction) error {
	if tx == nil {
		return ErrNilTransaction
	}

	if tm.wal == nil {
		return ErrNilWAL
	}

	// Check if transaction is active
	if !tx.IsActive() {
		return ErrTxNotActive
	}

	// Verify transaction is in active set
	tm.mu.RLock()
	_, exists := tm.activeTx[tx.ID]
	tm.mu.RUnlock()

	if !exists {
		return ErrTxNotFound
	}

	// Write ABORT record to WAL
	abortRecord := storage.NewWALRecord(0, tx.ID, storage.WALAbort)
	_, err := tm.wal.Append(abortRecord)
	if err != nil {
		return ErrWALWriteFailed
	}

	// Sync WAL to ensure abort is durable
	if err := tm.wal.Sync(); err != nil {
		return ErrWALSyncFailed
	}

	// Clear the write set (cleanup)
	tx.ClearWriteSet()

	// Mark transaction as aborted
	tx.SetState(TxAborted)

	// Remove from active transactions
	tm.mu.Lock()
	delete(tm.activeTx, tx.ID)
	tm.mu.Unlock()

	return nil
}

// GetActiveTransactions returns a list of all active transactions.
// The returned transactions are clones to prevent external modification.
func (tm *TxManager) GetActiveTransactions() []*Transaction {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	result := make([]*Transaction, 0, len(tm.activeTx))
	for _, tx := range tm.activeTx {
		result = append(result, tx.Clone())
	}
	return result
}

// GetTransaction returns the transaction with the given ID if it's active.
// Returns nil if the transaction is not found or not active.
func (tm *TxManager) GetTransaction(txID uint64) *Transaction {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.activeTx[txID]
}

// ActiveCount returns the number of active transactions.
func (tm *TxManager) ActiveCount() int {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return len(tm.activeTx)
}

// validateWriteSet checks for write conflicts with other transactions.
// In a simple implementation, we check if any page in the write set
// is also in another active transaction's write set.
func (tm *TxManager) validateWriteSet(tx *Transaction) error {
	writeSet := tx.GetWriteSet()
	if len(writeSet) == 0 {
		return nil
	}

	tm.mu.RLock()
	defer tm.mu.RUnlock()

	for _, otherTx := range tm.activeTx {
		if otherTx.ID == tx.ID {
			continue
		}

		otherWriteSet := otherTx.GetWriteSet()
		for _, pageID := range writeSet {
			for _, otherPageID := range otherWriteSet {
				if pageID == otherPageID {
					return ErrWriteConflict
				}
			}
		}
	}

	return nil
}

// NextTxID returns the next transaction ID that will be assigned.
// This is useful for testing and debugging.
func (tm *TxManager) NextTxID() uint64 {
	return atomic.LoadUint64(&tm.nextTxID)
}
