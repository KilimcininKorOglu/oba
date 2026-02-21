// Package tx provides transaction management for ObaDB.
package tx

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/KilimcininKorOglu/oba/internal/storage"
)

// testWAL creates a temporary WAL for testing.
func testWAL(t *testing.T) (*storage.WAL, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "tx_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	walPath := filepath.Join(tmpDir, "test.wal")
	wal, err := storage.OpenWAL(walPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to open WAL: %v", err)
	}

	cleanup := func() {
		wal.Close()
		os.RemoveAll(tmpDir)
	}

	return wal, cleanup
}

// TestNewTransaction tests transaction creation.
func TestNewTransaction(t *testing.T) {
	tx := NewTransaction(1, 100)

	if tx.ID != 1 {
		t.Errorf("expected ID 1, got %d", tx.ID)
	}

	if tx.State != TxActive {
		t.Errorf("expected state TxActive, got %v", tx.State)
	}

	if tx.StartLSN != 100 {
		t.Errorf("expected StartLSN 100, got %d", tx.StartLSN)
	}

	if tx.Snapshot != 1 {
		t.Errorf("expected Snapshot 1, got %d", tx.Snapshot)
	}

	if len(tx.ReadSet) != 0 {
		t.Errorf("expected empty ReadSet, got %d items", len(tx.ReadSet))
	}

	if len(tx.WriteSet) != 0 {
		t.Errorf("expected empty WriteSet, got %d items", len(tx.WriteSet))
	}
}

// TestTransactionState tests transaction state methods.
func TestTransactionState(t *testing.T) {
	tx := NewTransaction(1, 100)

	// Initially active
	if !tx.IsActive() {
		t.Error("expected transaction to be active")
	}
	if tx.IsCommitted() {
		t.Error("expected transaction not to be committed")
	}
	if tx.IsAborted() {
		t.Error("expected transaction not to be aborted")
	}

	// Set to committed
	tx.SetState(TxCommitted)
	if tx.IsActive() {
		t.Error("expected transaction not to be active")
	}
	if !tx.IsCommitted() {
		t.Error("expected transaction to be committed")
	}

	// Set to aborted
	tx.SetState(TxAborted)
	if !tx.IsAborted() {
		t.Error("expected transaction to be aborted")
	}
}

// TestTransactionReadWriteSet tests read/write set operations.
func TestTransactionReadWriteSet(t *testing.T) {
	tx := NewTransaction(1, 100)

	// Add to read set
	tx.AddToReadSet(storage.PageID(10))
	tx.AddToReadSet(storage.PageID(20))
	tx.AddToReadSet(storage.PageID(10)) // Duplicate, should be ignored

	readSet := tx.GetReadSet()
	if len(readSet) != 2 {
		t.Errorf("expected 2 items in ReadSet, got %d", len(readSet))
	}

	// Add to write set
	tx.AddToWriteSet(storage.PageID(30))
	tx.AddToWriteSet(storage.PageID(40))
	tx.AddToWriteSet(storage.PageID(30)) // Duplicate, should be ignored

	writeSet := tx.GetWriteSet()
	if len(writeSet) != 2 {
		t.Errorf("expected 2 items in WriteSet, got %d", len(writeSet))
	}

	// Clear write set
	tx.ClearWriteSet()
	writeSet = tx.GetWriteSet()
	if len(writeSet) != 0 {
		t.Errorf("expected empty WriteSet after clear, got %d items", len(writeSet))
	}

	// Clear read set
	tx.ClearReadSet()
	readSet = tx.GetReadSet()
	if len(readSet) != 0 {
		t.Errorf("expected empty ReadSet after clear, got %d items", len(readSet))
	}
}

// TestTransactionClone tests transaction cloning.
func TestTransactionClone(t *testing.T) {
	tx := NewTransaction(1, 100)
	tx.AddToReadSet(storage.PageID(10))
	tx.AddToWriteSet(storage.PageID(20))

	clone := tx.Clone()

	if clone.ID != tx.ID {
		t.Errorf("expected clone ID %d, got %d", tx.ID, clone.ID)
	}

	if clone.State != tx.State {
		t.Errorf("expected clone State %v, got %v", tx.State, clone.State)
	}

	// Modify original, clone should not change
	tx.AddToReadSet(storage.PageID(30))
	if len(clone.ReadSet) != 1 {
		t.Errorf("clone ReadSet should not change, got %d items", len(clone.ReadSet))
	}
}

// TestTxStateString tests TxState string representation.
func TestTxStateString(t *testing.T) {
	tests := []struct {
		state    TxState
		expected string
	}{
		{TxActive, "Active"},
		{TxCommitted, "Committed"},
		{TxAborted, "Aborted"},
		{TxState(99), "Unknown"},
	}

	for _, tt := range tests {
		if got := tt.state.String(); got != tt.expected {
			t.Errorf("TxState(%d).String() = %q, want %q", tt.state, got, tt.expected)
		}
	}
}

// TestNewTxManager tests transaction manager creation.
func TestNewTxManager(t *testing.T) {
	wal, cleanup := testWAL(t)
	defer cleanup()

	tm := NewTxManager(wal)

	if tm == nil {
		t.Fatal("expected non-nil TxManager")
	}

	if tm.NextTxID() != 1 {
		t.Errorf("expected NextTxID 1, got %d", tm.NextTxID())
	}

	if tm.ActiveCount() != 0 {
		t.Errorf("expected 0 active transactions, got %d", tm.ActiveCount())
	}
}

// TestTxManagerBegin tests beginning a transaction.
func TestTxManagerBegin(t *testing.T) {
	wal, cleanup := testWAL(t)
	defer cleanup()

	tm := NewTxManager(wal)

	tx, err := tm.Begin()
	if err != nil {
		t.Fatalf("Begin() failed: %v", err)
	}

	if tx == nil {
		t.Fatal("expected non-nil transaction")
	}

	if tx.ID != 1 {
		t.Errorf("expected transaction ID 1, got %d", tx.ID)
	}

	if !tx.IsActive() {
		t.Error("expected transaction to be active")
	}

	if tm.ActiveCount() != 1 {
		t.Errorf("expected 1 active transaction, got %d", tm.ActiveCount())
	}

	// Begin another transaction
	tx2, err := tm.Begin()
	if err != nil {
		t.Fatalf("Begin() failed: %v", err)
	}

	if tx2.ID != 2 {
		t.Errorf("expected transaction ID 2, got %d", tx2.ID)
	}

	if tm.ActiveCount() != 2 {
		t.Errorf("expected 2 active transactions, got %d", tm.ActiveCount())
	}
}

// TestTxManagerBeginNilWAL tests Begin with nil WAL.
func TestTxManagerBeginNilWAL(t *testing.T) {
	tm := NewTxManager(nil)

	_, err := tm.Begin()
	if err != ErrNilWAL {
		t.Errorf("expected ErrNilWAL, got %v", err)
	}
}

// TestTxManagerCommit tests committing a transaction.
func TestTxManagerCommit(t *testing.T) {
	wal, cleanup := testWAL(t)
	defer cleanup()

	tm := NewTxManager(wal)

	tx, err := tm.Begin()
	if err != nil {
		t.Fatalf("Begin() failed: %v", err)
	}

	// Add some pages to write set
	tx.AddToWriteSet(storage.PageID(10))
	tx.AddToWriteSet(storage.PageID(20))

	err = tm.Commit(tx)
	if err != nil {
		t.Fatalf("Commit() failed: %v", err)
	}

	if !tx.IsCommitted() {
		t.Error("expected transaction to be committed")
	}

	if tm.ActiveCount() != 0 {
		t.Errorf("expected 0 active transactions after commit, got %d", tm.ActiveCount())
	}

	// Transaction should no longer be in active set
	if tm.GetTransaction(tx.ID) != nil {
		t.Error("expected transaction to be removed from active set")
	}
}

// TestTxManagerCommitNilTransaction tests Commit with nil transaction.
func TestTxManagerCommitNilTransaction(t *testing.T) {
	wal, cleanup := testWAL(t)
	defer cleanup()

	tm := NewTxManager(wal)

	err := tm.Commit(nil)
	if err != ErrNilTransaction {
		t.Errorf("expected ErrNilTransaction, got %v", err)
	}
}

// TestTxManagerCommitNotActive tests Commit on non-active transaction.
func TestTxManagerCommitNotActive(t *testing.T) {
	wal, cleanup := testWAL(t)
	defer cleanup()

	tm := NewTxManager(wal)

	tx, err := tm.Begin()
	if err != nil {
		t.Fatalf("Begin() failed: %v", err)
	}

	// Commit first time
	err = tm.Commit(tx)
	if err != nil {
		t.Fatalf("Commit() failed: %v", err)
	}

	// Try to commit again
	err = tm.Commit(tx)
	if err != ErrTxNotActive {
		t.Errorf("expected ErrTxNotActive, got %v", err)
	}
}

// TestTxManagerRollback tests rolling back a transaction.
func TestTxManagerRollback(t *testing.T) {
	wal, cleanup := testWAL(t)
	defer cleanup()

	tm := NewTxManager(wal)

	tx, err := tm.Begin()
	if err != nil {
		t.Fatalf("Begin() failed: %v", err)
	}

	// Add some pages to write set
	tx.AddToWriteSet(storage.PageID(10))
	tx.AddToWriteSet(storage.PageID(20))

	err = tm.Rollback(tx)
	if err != nil {
		t.Fatalf("Rollback() failed: %v", err)
	}

	if !tx.IsAborted() {
		t.Error("expected transaction to be aborted")
	}

	// Write set should be cleared
	if len(tx.GetWriteSet()) != 0 {
		t.Error("expected write set to be cleared after rollback")
	}

	if tm.ActiveCount() != 0 {
		t.Errorf("expected 0 active transactions after rollback, got %d", tm.ActiveCount())
	}
}

// TestTxManagerRollbackNilTransaction tests Rollback with nil transaction.
func TestTxManagerRollbackNilTransaction(t *testing.T) {
	wal, cleanup := testWAL(t)
	defer cleanup()

	tm := NewTxManager(wal)

	err := tm.Rollback(nil)
	if err != ErrNilTransaction {
		t.Errorf("expected ErrNilTransaction, got %v", err)
	}
}

// TestTxManagerRollbackNotActive tests Rollback on non-active transaction.
func TestTxManagerRollbackNotActive(t *testing.T) {
	wal, cleanup := testWAL(t)
	defer cleanup()

	tm := NewTxManager(wal)

	tx, err := tm.Begin()
	if err != nil {
		t.Fatalf("Begin() failed: %v", err)
	}

	// Rollback first time
	err = tm.Rollback(tx)
	if err != nil {
		t.Fatalf("Rollback() failed: %v", err)
	}

	// Try to rollback again
	err = tm.Rollback(tx)
	if err != ErrTxNotActive {
		t.Errorf("expected ErrTxNotActive, got %v", err)
	}
}

// TestTxManagerGetActiveTransactions tests getting active transactions.
func TestTxManagerGetActiveTransactions(t *testing.T) {
	wal, cleanup := testWAL(t)
	defer cleanup()

	tm := NewTxManager(wal)

	// No active transactions initially
	active := tm.GetActiveTransactions()
	if len(active) != 0 {
		t.Errorf("expected 0 active transactions, got %d", len(active))
	}

	// Begin some transactions
	tx1, _ := tm.Begin()
	tx2, _ := tm.Begin()
	tx3, _ := tm.Begin()

	active = tm.GetActiveTransactions()
	if len(active) != 3 {
		t.Errorf("expected 3 active transactions, got %d", len(active))
	}

	// Commit one
	tm.Commit(tx2)

	active = tm.GetActiveTransactions()
	if len(active) != 2 {
		t.Errorf("expected 2 active transactions, got %d", len(active))
	}

	// Rollback one
	tm.Rollback(tx1)

	active = tm.GetActiveTransactions()
	if len(active) != 1 {
		t.Errorf("expected 1 active transaction, got %d", len(active))
	}

	// Verify it's tx3
	if active[0].ID != tx3.ID {
		t.Errorf("expected active transaction ID %d, got %d", tx3.ID, active[0].ID)
	}
}

// TestTxManagerGetTransaction tests getting a specific transaction.
func TestTxManagerGetTransaction(t *testing.T) {
	wal, cleanup := testWAL(t)
	defer cleanup()

	tm := NewTxManager(wal)

	tx, _ := tm.Begin()

	// Get existing transaction
	found := tm.GetTransaction(tx.ID)
	if found == nil {
		t.Fatal("expected to find transaction")
	}
	if found.ID != tx.ID {
		t.Errorf("expected ID %d, got %d", tx.ID, found.ID)
	}

	// Get non-existing transaction
	notFound := tm.GetTransaction(999)
	if notFound != nil {
		t.Error("expected nil for non-existing transaction")
	}
}

// TestTxIDsAreUniqueAndMonotonic tests that transaction IDs are unique and monotonic.
func TestTxIDsAreUniqueAndMonotonic(t *testing.T) {
	wal, cleanup := testWAL(t)
	defer cleanup()

	tm := NewTxManager(wal)

	var prevID uint64 = 0
	ids := make(map[uint64]bool)

	for i := 0; i < 100; i++ {
		tx, err := tm.Begin()
		if err != nil {
			t.Fatalf("Begin() failed: %v", err)
		}

		// Check uniqueness
		if ids[tx.ID] {
			t.Errorf("duplicate transaction ID: %d", tx.ID)
		}
		ids[tx.ID] = true

		// Check monotonicity
		if tx.ID <= prevID {
			t.Errorf("transaction ID not monotonic: prev=%d, current=%d", prevID, tx.ID)
		}
		prevID = tx.ID

		// Commit to free up
		tm.Commit(tx)
	}
}

// TestConcurrentTransactions tests concurrent transaction operations.
func TestConcurrentTransactions(t *testing.T) {
	wal, cleanup := testWAL(t)
	defer cleanup()

	tm := NewTxManager(wal)

	const numGoroutines = 10
	const txPerGoroutine = 10

	var wg sync.WaitGroup
	ids := make(chan uint64, numGoroutines*txPerGoroutine)
	errors := make(chan error, numGoroutines*txPerGoroutine)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < txPerGoroutine; j++ {
				tx, err := tm.Begin()
				if err != nil {
					errors <- err
					continue
				}
				ids <- tx.ID

				// Add some work
				tx.AddToWriteSet(storage.PageID(tx.ID))

				// Randomly commit or rollback
				if tx.ID%2 == 0 {
					if err := tm.Commit(tx); err != nil {
						errors <- err
					}
				} else {
					if err := tm.Rollback(tx); err != nil {
						errors <- err
					}
				}
			}
		}()
	}

	wg.Wait()
	close(ids)
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("concurrent operation error: %v", err)
	}

	// Check all IDs are unique
	idSet := make(map[uint64]bool)
	for id := range ids {
		if idSet[id] {
			t.Errorf("duplicate transaction ID in concurrent test: %d", id)
		}
		idSet[id] = true
	}

	// All transactions should be completed
	if tm.ActiveCount() != 0 {
		t.Errorf("expected 0 active transactions, got %d", tm.ActiveCount())
	}
}

// TestTransactionDuration tests transaction duration tracking.
func TestTransactionDuration(t *testing.T) {
	tx := NewTransaction(1, 100)

	// Wait a bit
	time.Sleep(10 * time.Millisecond)

	duration := tx.Duration()
	if duration < 10*time.Millisecond {
		t.Errorf("expected duration >= 10ms, got %v", duration)
	}
}

// TestCommitWritesWALRecord tests that commit writes WAL record before returning.
func TestCommitWritesWALRecord(t *testing.T) {
	wal, cleanup := testWAL(t)
	defer cleanup()

	tm := NewTxManager(wal)

	tx, err := tm.Begin()
	if err != nil {
		t.Fatalf("Begin() failed: %v", err)
	}

	initialLSN := wal.CurrentLSN()

	err = tm.Commit(tx)
	if err != nil {
		t.Fatalf("Commit() failed: %v", err)
	}

	// WAL should have advanced (BEGIN + COMMIT records)
	finalLSN := wal.CurrentLSN()
	if finalLSN <= initialLSN {
		t.Error("expected WAL LSN to advance after commit")
	}
}

// TestRollbackCleansUpWriteSet tests that rollback cleans up write set.
func TestRollbackCleansUpWriteSet(t *testing.T) {
	wal, cleanup := testWAL(t)
	defer cleanup()

	tm := NewTxManager(wal)

	tx, err := tm.Begin()
	if err != nil {
		t.Fatalf("Begin() failed: %v", err)
	}

	// Add pages to write set
	tx.AddToWriteSet(storage.PageID(1))
	tx.AddToWriteSet(storage.PageID(2))
	tx.AddToWriteSet(storage.PageID(3))

	if len(tx.GetWriteSet()) != 3 {
		t.Errorf("expected 3 pages in write set, got %d", len(tx.GetWriteSet()))
	}

	err = tm.Rollback(tx)
	if err != nil {
		t.Fatalf("Rollback() failed: %v", err)
	}

	// Write set should be cleared
	if len(tx.GetWriteSet()) != 0 {
		t.Errorf("expected empty write set after rollback, got %d pages", len(tx.GetWriteSet()))
	}
}

// TestWriteConflictDetection tests write conflict detection.
func TestWriteConflictDetection(t *testing.T) {
	wal, cleanup := testWAL(t)
	defer cleanup()

	tm := NewTxManager(wal)

	// Start two transactions
	tx1, _ := tm.Begin()
	tx2, _ := tm.Begin()

	// Both write to the same page
	tx1.AddToWriteSet(storage.PageID(100))
	tx2.AddToWriteSet(storage.PageID(100))

	// Both transactions are active with conflicting write sets.
	// Either commit should fail due to conflict with the other active transaction.
	err := tm.Commit(tx1)
	if err != ErrWriteConflict {
		t.Errorf("expected ErrWriteConflict, got %v", err)
	}

	// tx1 should still be active (commit failed)
	if !tx1.IsActive() {
		t.Error("expected tx1 to still be active after failed commit")
	}

	// Rollback tx1 to clear the conflict
	err = tm.Rollback(tx1)
	if err != nil {
		t.Fatalf("Rollback() failed: %v", err)
	}

	// Now tx2 should be able to commit (no other active transaction with same page)
	err = tm.Commit(tx2)
	if err != nil {
		t.Fatalf("tx2 Commit() failed: %v", err)
	}
}

// TestWriteConflictWithActiveTransactions tests conflict with active transactions.
func TestWriteConflictWithActiveTransactions(t *testing.T) {
	wal, cleanup := testWAL(t)
	defer cleanup()

	tm := NewTxManager(wal)

	// Start two transactions
	tx1, _ := tm.Begin()
	tx2, _ := tm.Begin()

	// Both write to the same page
	tx1.AddToWriteSet(storage.PageID(100))
	tx2.AddToWriteSet(storage.PageID(100))

	// Try to commit tx2 while tx1 is still active
	err := tm.Commit(tx2)
	if err != ErrWriteConflict {
		t.Errorf("expected ErrWriteConflict, got %v", err)
	}

	// tx2 should still be active (commit failed)
	if !tx2.IsActive() {
		t.Error("expected tx2 to still be active after failed commit")
	}

	// Rollback tx2 to clear the conflict
	err = tm.Rollback(tx2)
	if err != nil {
		t.Fatalf("Rollback tx2 failed: %v", err)
	}

	// tx1 should now be able to commit (no other active transaction with same page)
	err = tm.Commit(tx1)
	if err != nil {
		t.Fatalf("tx1 Commit() failed: %v", err)
	}
}

// TestCommitNotFoundTransaction tests committing a transaction not in active set.
func TestCommitNotFoundTransaction(t *testing.T) {
	wal, cleanup := testWAL(t)
	defer cleanup()

	tm := NewTxManager(wal)

	// Create a transaction manually (not through Begin)
	tx := NewTransaction(999, 0)

	err := tm.Commit(tx)
	if err != ErrTxNotFound {
		t.Errorf("expected ErrTxNotFound, got %v", err)
	}
}

// TestRollbackNotFoundTransaction tests rolling back a transaction not in active set.
func TestRollbackNotFoundTransaction(t *testing.T) {
	wal, cleanup := testWAL(t)
	defer cleanup()

	tm := NewTxManager(wal)

	// Create a transaction manually (not through Begin)
	tx := NewTransaction(999, 0)

	err := tm.Rollback(tx)
	if err != ErrTxNotFound {
		t.Errorf("expected ErrTxNotFound, got %v", err)
	}
}

// TestCommitNilWAL tests Commit with nil WAL.
func TestCommitNilWAL(t *testing.T) {
	tm := NewTxManager(nil)

	tx := NewTransaction(1, 0)

	err := tm.Commit(tx)
	if err != ErrNilWAL {
		t.Errorf("expected ErrNilWAL, got %v", err)
	}
}

// TestRollbackNilWAL tests Rollback with nil WAL.
func TestRollbackNilWAL(t *testing.T) {
	tm := NewTxManager(nil)

	tx := NewTransaction(1, 0)

	err := tm.Rollback(tx)
	if err != ErrNilWAL {
		t.Errorf("expected ErrNilWAL, got %v", err)
	}
}

// TestGetActiveTransactionsReturnsClones tests that GetActiveTransactions returns clones.
func TestGetActiveTransactionsReturnsClones(t *testing.T) {
	wal, cleanup := testWAL(t)
	defer cleanup()

	tm := NewTxManager(wal)

	tx, _ := tm.Begin()
	tx.AddToWriteSet(storage.PageID(10))

	active := tm.GetActiveTransactions()
	if len(active) != 1 {
		t.Fatalf("expected 1 active transaction, got %d", len(active))
	}

	// Modify the returned transaction
	active[0].AddToWriteSet(storage.PageID(20))

	// Original should not be affected
	original := tm.GetTransaction(tx.ID)
	if len(original.GetWriteSet()) != 1 {
		t.Errorf("original transaction should not be modified, got %d pages", len(original.GetWriteSet()))
	}
}
