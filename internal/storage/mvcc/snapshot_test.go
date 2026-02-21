// Package mvcc provides Multi-Version Concurrency Control for ObaDB.
package mvcc

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/KilimcininKorOglu/oba/internal/storage"
	"github.com/KilimcininKorOglu/oba/internal/storage/tx"
)

// --- Snapshot Tests ---

// TestNewSnapshot tests the creation of a new snapshot.
func TestNewSnapshot(t *testing.T) {
	timestamp := uint64(100)
	activeTxIDs := []uint64{5, 3, 7, 1} // Unsorted
	txID := uint64(10)

	s := NewSnapshot(timestamp, activeTxIDs, txID)

	if s.Timestamp != timestamp {
		t.Errorf("expected Timestamp %d, got %d", timestamp, s.Timestamp)
	}
	if s.TxID != txID {
		t.Errorf("expected TxID %d, got %d", txID, s.TxID)
	}
	if len(s.ActiveTxIDs) != len(activeTxIDs) {
		t.Errorf("expected %d active tx IDs, got %d", len(activeTxIDs), len(s.ActiveTxIDs))
	}

	// Verify ActiveTxIDs are sorted
	for i := 1; i < len(s.ActiveTxIDs); i++ {
		if s.ActiveTxIDs[i] < s.ActiveTxIDs[i-1] {
			t.Error("ActiveTxIDs should be sorted")
		}
	}

	// Verify initial state
	if s.IsReleased() {
		t.Error("snapshot should not be released initially")
	}
	if s.RefCount() != 1 {
		t.Errorf("expected refCount 1, got %d", s.RefCount())
	}
}

// TestSnapshotWasActiveAtSnapshot tests the WasActiveAtSnapshot method.
func TestSnapshotWasActiveAtSnapshot(t *testing.T) {
	activeTxIDs := []uint64{10, 20, 30, 40, 50}
	s := NewSnapshot(100, activeTxIDs, 1)

	tests := []struct {
		txID     uint64
		expected bool
	}{
		{10, true},
		{20, true},
		{30, true},
		{40, true},
		{50, true},
		{5, false},
		{15, false},
		{25, false},
		{55, false},
		{100, false},
	}

	for _, tt := range tests {
		got := s.WasActiveAtSnapshot(tt.txID)
		if got != tt.expected {
			t.Errorf("WasActiveAtSnapshot(%d) = %v, want %v", tt.txID, got, tt.expected)
		}
	}
}

// TestSnapshotRefCount tests reference counting.
func TestSnapshotRefCount(t *testing.T) {
	s := NewSnapshot(100, nil, 1)

	if s.RefCount() != 1 {
		t.Errorf("expected initial refCount 1, got %d", s.RefCount())
	}

	s.AddRef()
	if s.RefCount() != 2 {
		t.Errorf("expected refCount 2 after AddRef, got %d", s.RefCount())
	}

	s.AddRef()
	if s.RefCount() != 3 {
		t.Errorf("expected refCount 3 after second AddRef, got %d", s.RefCount())
	}

	// Release should not mark as released until refCount reaches 0
	released := s.Release()
	if released {
		t.Error("Release should return false when refCount > 0")
	}
	if s.RefCount() != 2 {
		t.Errorf("expected refCount 2 after Release, got %d", s.RefCount())
	}

	s.Release()
	released = s.Release()
	if !released {
		t.Error("Release should return true when refCount reaches 0")
	}
	if !s.IsReleased() {
		t.Error("snapshot should be released when refCount reaches 0")
	}
}

// TestSnapshotClone tests snapshot cloning.
func TestSnapshotClone(t *testing.T) {
	activeTxIDs := []uint64{10, 20, 30}
	s := NewSnapshot(100, activeTxIDs, 5)

	clone := s.Clone()

	if clone.Timestamp != s.Timestamp {
		t.Errorf("Timestamp mismatch")
	}
	if clone.TxID != s.TxID {
		t.Errorf("TxID mismatch")
	}
	if len(clone.ActiveTxIDs) != len(s.ActiveTxIDs) {
		t.Errorf("ActiveTxIDs length mismatch")
	}

	// Verify clone is independent
	clone.ActiveTxIDs[0] = 999
	if s.ActiveTxIDs[0] == 999 {
		t.Error("Clone should be independent of original")
	}

	// Clone should have fresh refCount
	if clone.RefCount() != 1 {
		t.Errorf("Clone should have refCount 1, got %d", clone.RefCount())
	}
	if clone.IsReleased() {
		t.Error("Clone should not be released")
	}
}

// TestSnapshotGetters tests getter methods.
func TestSnapshotGetters(t *testing.T) {
	activeTxIDs := []uint64{10, 20, 30}
	s := NewSnapshot(100, activeTxIDs, 5)

	if s.GetTimestamp() != 100 {
		t.Errorf("GetTimestamp() = %d, want 100", s.GetTimestamp())
	}
	if s.GetTxID() != 5 {
		t.Errorf("GetTxID() = %d, want 5", s.GetTxID())
	}

	gotActiveIDs := s.GetActiveTxIDs()
	if len(gotActiveIDs) != 3 {
		t.Errorf("GetActiveTxIDs() length = %d, want 3", len(gotActiveIDs))
	}

	// Verify returned slice is a copy
	gotActiveIDs[0] = 999
	if s.ActiveTxIDs[0] == 999 {
		t.Error("GetActiveTxIDs should return a copy")
	}
}

// --- SnapshotManager Tests ---

// createTestTxManager creates a TxManager for testing.
func createTestTxManager(t *testing.T) (*tx.TxManager, string) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "snapshot_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	walPath := filepath.Join(tmpDir, "test.wal")
	wal, err := storage.OpenWAL(walPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to create WAL: %v", err)
	}

	return tx.NewTxManager(wal), tmpDir
}

// TestNewSnapshotManager tests SnapshotManager creation.
func TestNewSnapshotManager(t *testing.T) {
	txMgr, tmpDir := createTestTxManager(t)
	defer os.RemoveAll(tmpDir)

	sm := NewSnapshotManager(txMgr)

	if sm.CurrentTimestamp() != 0 {
		t.Errorf("expected initial timestamp 0, got %d", sm.CurrentTimestamp())
	}
	if sm.ActiveSnapshotCount() != 0 {
		t.Errorf("expected 0 active snapshots, got %d", sm.ActiveSnapshotCount())
	}
}

// TestSnapshotManagerCreateSnapshot tests snapshot creation.
func TestSnapshotManagerCreateSnapshot(t *testing.T) {
	txMgr, tmpDir := createTestTxManager(t)
	defer os.RemoveAll(tmpDir)

	sm := NewSnapshotManager(txMgr)

	// Begin a transaction
	txn, err := txMgr.Begin()
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}

	// Create a snapshot
	snapshot, err := sm.CreateSnapshot(txn)
	if err != nil {
		t.Fatalf("CreateSnapshot failed: %v", err)
	}

	if snapshot.Timestamp != 1 {
		t.Errorf("expected timestamp 1, got %d", snapshot.Timestamp)
	}
	if snapshot.TxID != txn.ID {
		t.Errorf("expected TxID %d, got %d", txn.ID, snapshot.TxID)
	}
	if sm.ActiveSnapshotCount() != 1 {
		t.Errorf("expected 1 active snapshot, got %d", sm.ActiveSnapshotCount())
	}
}

// TestSnapshotManagerCreateSnapshotNilTx tests error handling for nil transaction.
func TestSnapshotManagerCreateSnapshotNilTx(t *testing.T) {
	sm := NewSnapshotManager(nil)

	_, err := sm.CreateSnapshot(nil)
	if err != ErrNilTransaction {
		t.Errorf("expected ErrNilTransaction, got %v", err)
	}
}

// TestSnapshotManagerReleaseSnapshot tests snapshot release.
func TestSnapshotManagerReleaseSnapshot(t *testing.T) {
	txMgr, tmpDir := createTestTxManager(t)
	defer os.RemoveAll(tmpDir)

	sm := NewSnapshotManager(txMgr)

	txn, _ := txMgr.Begin()
	snapshot, _ := sm.CreateSnapshot(txn)

	if sm.ActiveSnapshotCount() != 1 {
		t.Errorf("expected 1 active snapshot before release")
	}

	err := sm.ReleaseSnapshot(snapshot)
	if err != nil {
		t.Fatalf("ReleaseSnapshot failed: %v", err)
	}

	if sm.ActiveSnapshotCount() != 0 {
		t.Errorf("expected 0 active snapshots after release, got %d", sm.ActiveSnapshotCount())
	}
}

// TestSnapshotManagerReleaseSnapshotErrors tests error handling for release.
func TestSnapshotManagerReleaseSnapshotErrors(t *testing.T) {
	sm := NewSnapshotManager(nil)

	// Test nil snapshot
	err := sm.ReleaseSnapshot(nil)
	if err != ErrInvalidSnapshot {
		t.Errorf("expected ErrInvalidSnapshot for nil, got %v", err)
	}

	// Test already released snapshot
	snapshot := NewSnapshot(100, nil, 1)
	snapshot.Release() // Release it

	err = sm.ReleaseSnapshot(snapshot)
	if err != ErrSnapshotReleased {
		t.Errorf("expected ErrSnapshotReleased, got %v", err)
	}
}

// TestSnapshotManagerGetOldestActiveSnapshot tests finding the oldest snapshot.
func TestSnapshotManagerGetOldestActiveSnapshot(t *testing.T) {
	txMgr, tmpDir := createTestTxManager(t)
	defer os.RemoveAll(tmpDir)

	sm := NewSnapshotManager(txMgr)

	// No snapshots
	if oldest := sm.GetOldestActiveSnapshot(); oldest != 0 {
		t.Errorf("expected 0 for no snapshots, got %d", oldest)
	}

	// Create multiple snapshots
	tx1, _ := txMgr.Begin()
	s1, _ := sm.CreateSnapshot(tx1)

	tx2, _ := txMgr.Begin()
	s2, _ := sm.CreateSnapshot(tx2)

	tx3, _ := txMgr.Begin()
	_, _ = sm.CreateSnapshot(tx3)

	// Oldest should be s1's timestamp
	if oldest := sm.GetOldestActiveSnapshot(); oldest != s1.Timestamp {
		t.Errorf("expected oldest %d, got %d", s1.Timestamp, oldest)
	}

	// Release s1, oldest should now be s2
	sm.ReleaseSnapshot(s1)
	if oldest := sm.GetOldestActiveSnapshot(); oldest != s2.Timestamp {
		t.Errorf("expected oldest %d after releasing s1, got %d", s2.Timestamp, oldest)
	}
}

// TestSnapshotManagerAdvanceTimestamp tests timestamp advancement.
func TestSnapshotManagerAdvanceTimestamp(t *testing.T) {
	sm := NewSnapshotManager(nil)

	ts1 := sm.AdvanceTimestamp()
	if ts1 != 1 {
		t.Errorf("expected timestamp 1, got %d", ts1)
	}

	ts2 := sm.AdvanceTimestamp()
	if ts2 != 2 {
		t.Errorf("expected timestamp 2, got %d", ts2)
	}

	if sm.CurrentTimestamp() != 2 {
		t.Errorf("expected current timestamp 2, got %d", sm.CurrentTimestamp())
	}
}

// TestSnapshotManagerSetTimestamp tests setting timestamp (for recovery).
func TestSnapshotManagerSetTimestamp(t *testing.T) {
	sm := NewSnapshotManager(nil)

	sm.SetTimestamp(1000)
	if sm.CurrentTimestamp() != 1000 {
		t.Errorf("expected timestamp 1000, got %d", sm.CurrentTimestamp())
	}
}

// TestSnapshotManagerCleanupReleasedSnapshots tests cleanup of released snapshots.
func TestSnapshotManagerCleanupReleasedSnapshots(t *testing.T) {
	txMgr, tmpDir := createTestTxManager(t)
	defer os.RemoveAll(tmpDir)

	sm := NewSnapshotManager(txMgr)

	// Create multiple snapshots with extra references
	tx1, _ := txMgr.Begin()
	s1, _ := sm.CreateSnapshot(tx1)
	s1.AddRef() // Add extra reference so release doesn't remove immediately

	tx2, _ := txMgr.Begin()
	_, _ = sm.CreateSnapshot(tx2)

	tx3, _ := txMgr.Begin()
	s3, _ := sm.CreateSnapshot(tx3)
	s3.AddRef() // Add extra reference

	// Release once (still has one reference)
	sm.ReleaseSnapshot(s1)
	sm.ReleaseSnapshot(s3)

	// Snapshots still in map but with one reference each
	if sm.ActiveSnapshotCount() != 3 {
		t.Errorf("expected 3 active snapshots before final release, got %d", sm.ActiveSnapshotCount())
	}

	// Release again to fully release
	sm.ReleaseSnapshot(s1)
	sm.ReleaseSnapshot(s3)

	// Now cleanup should find nothing (already removed on full release)
	removed := sm.CleanupReleasedSnapshots()
	if removed != 0 {
		t.Errorf("expected 0 removed (already cleaned up on release), got %d", removed)
	}

	if sm.ActiveSnapshotCount() != 1 {
		t.Errorf("expected 1 active snapshot after cleanup, got %d", sm.ActiveSnapshotCount())
	}
}

// TestSnapshotManagerStats tests statistics gathering.
func TestSnapshotManagerStats(t *testing.T) {
	txMgr, tmpDir := createTestTxManager(t)
	defer os.RemoveAll(tmpDir)

	sm := NewSnapshotManager(txMgr)

	// Create snapshots
	tx1, _ := txMgr.Begin()
	s1, _ := sm.CreateSnapshot(tx1)
	s1.AddRef() // Add extra reference so release doesn't remove immediately

	tx2, _ := txMgr.Begin()
	_, _ = sm.CreateSnapshot(tx2)

	// Release via manager (decrements ref count but doesn't remove since refCount > 0)
	sm.ReleaseSnapshot(s1)

	// Now s1 is still in the map but with refCount=1
	// Release again to mark as released
	sm.ReleaseSnapshot(s1)

	// Now s1 should be removed from the map
	stats := sm.Stats()
	if stats.TotalSnapshots != 1 {
		t.Errorf("expected TotalSnapshots 1, got %d", stats.TotalSnapshots)
	}
	if stats.ActiveSnapshots != 1 {
		t.Errorf("expected ActiveSnapshots 1, got %d", stats.ActiveSnapshots)
	}
	if stats.ReleasedSnapshots != 0 {
		t.Errorf("expected ReleasedSnapshots 0, got %d", stats.ReleasedSnapshots)
	}
}

// TestSnapshotManagerConcurrency tests concurrent snapshot operations.
func TestSnapshotManagerConcurrency(t *testing.T) {
	txMgr, tmpDir := createTestTxManager(t)
	defer os.RemoveAll(tmpDir)

	sm := NewSnapshotManager(txMgr)

	const numGoroutines = 10
	const snapshotsPerGoroutine = 5

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < snapshotsPerGoroutine; j++ {
				txn, err := txMgr.Begin()
				if err != nil {
					t.Errorf("Begin failed: %v", err)
					return
				}

				snapshot, err := sm.CreateSnapshot(txn)
				if err != nil {
					t.Errorf("CreateSnapshot failed: %v", err)
					return
				}

				// Do some work
				_ = sm.GetOldestActiveSnapshot()
				_ = sm.ActiveSnapshotCount()

				// Release the snapshot
				if err := sm.ReleaseSnapshot(snapshot); err != nil {
					t.Errorf("ReleaseSnapshot failed: %v", err)
				}
			}
		}()
	}

	wg.Wait()

	// All snapshots should be released
	if count := sm.ActiveSnapshotCount(); count != 0 {
		t.Errorf("expected 0 active snapshots after concurrent test, got %d", count)
	}
}

// TestSnapshotManagerGetAllActiveSnapshots tests getting all active snapshots.
func TestSnapshotManagerGetAllActiveSnapshots(t *testing.T) {
	txMgr, tmpDir := createTestTxManager(t)
	defer os.RemoveAll(tmpDir)

	sm := NewSnapshotManager(txMgr)

	// Create snapshots
	tx1, _ := txMgr.Begin()
	_, _ = sm.CreateSnapshot(tx1)

	tx2, _ := txMgr.Begin()
	_, _ = sm.CreateSnapshot(tx2)

	tx3, _ := txMgr.Begin()
	_, _ = sm.CreateSnapshot(tx3)

	snapshots := sm.GetAllActiveSnapshots()
	if len(snapshots) != 3 {
		t.Errorf("expected 3 active snapshots, got %d", len(snapshots))
	}
}

// TestSnapshotManagerWithActiveTxIDs tests that active transaction IDs are captured.
func TestSnapshotManagerWithActiveTxIDs(t *testing.T) {
	txMgr, tmpDir := createTestTxManager(t)
	defer os.RemoveAll(tmpDir)

	sm := NewSnapshotManager(txMgr)

	// Start multiple transactions
	tx1, _ := txMgr.Begin()
	tx2, _ := txMgr.Begin()
	tx3, _ := txMgr.Begin()

	// Create a snapshot for tx3
	snapshot, _ := sm.CreateSnapshot(tx3)

	// The snapshot should have tx1 and tx2 in its active list (but not tx3)
	activeIDs := snapshot.GetActiveTxIDs()

	// Check that tx1 and tx2 are in the active list
	hasTx1 := false
	hasTx2 := false
	hasTx3 := false
	for _, id := range activeIDs {
		if id == tx1.ID {
			hasTx1 = true
		}
		if id == tx2.ID {
			hasTx2 = true
		}
		if id == tx3.ID {
			hasTx3 = true
		}
	}

	if !hasTx1 {
		t.Error("expected tx1 to be in active list")
	}
	if !hasTx2 {
		t.Error("expected tx2 to be in active list")
	}
	if hasTx3 {
		t.Error("tx3 should not be in its own active list")
	}
}
