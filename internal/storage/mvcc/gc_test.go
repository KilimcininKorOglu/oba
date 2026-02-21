// Package mvcc provides Multi-Version Concurrency Control for ObaDB.
package mvcc

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/KilimcininKorOglu/oba/internal/storage"
	"github.com/KilimcininKorOglu/oba/internal/storage/tx"
)

// --- Test Helpers ---

// createTestGCEnvironment creates a complete test environment for GC testing.
func createTestGCEnvironment(t *testing.T) (*GarbageCollector, *VersionStore, *SnapshotManager, *tx.TxManager, string) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "gc_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	// Create page manager
	dataPath := filepath.Join(tmpDir, "data.oba")
	pm, err := storage.OpenPageManager(dataPath, storage.DefaultOptions())
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to create page manager: %v", err)
	}

	// Create WAL
	walPath := filepath.Join(tmpDir, "test.wal")
	wal, err := storage.OpenWAL(walPath)
	if err != nil {
		pm.Close()
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to create WAL: %v", err)
	}

	// Create transaction manager
	txMgr := tx.NewTxManager(wal)

	// Create snapshot manager
	sm := NewSnapshotManager(txMgr)

	// Create version store
	vs := NewVersionStore(pm)

	// Create garbage collector
	gc := NewGarbageCollector(vs, sm, pm)

	return gc, vs, sm, txMgr, tmpDir
}

// cleanupTestGCEnvironment cleans up the test environment.
func cleanupTestGCEnvironment(gc *GarbageCollector, tmpDir string) {
	if gc != nil {
		gc.Close()
	}
	os.RemoveAll(tmpDir)
}

// --- GarbageCollector Creation Tests ---

// TestNewGarbageCollector tests the creation of a new GarbageCollector.
func TestNewGarbageCollector(t *testing.T) {
	gc, _, _, _, tmpDir := createTestGCEnvironment(t)
	defer cleanupTestGCEnvironment(gc, tmpDir)

	if gc == nil {
		t.Fatal("expected non-nil GarbageCollector")
	}

	if gc.IsRunning() {
		t.Error("GC should not be running initially")
	}

	config := gc.GetConfig()
	if config.Interval != DefaultGCInterval {
		t.Errorf("expected default interval %v, got %v", DefaultGCInterval, config.Interval)
	}
}

// TestNewGarbageCollectorWithConfig tests creation with custom config.
func TestNewGarbageCollectorWithConfig(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gc_config_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := GCConfig{
		Interval:      5 * time.Second,
		MinVersionAge: 1 * time.Second,
		BatchSize:     100,
	}

	gc := NewGarbageCollectorWithConfig(nil, nil, nil, config)
	defer gc.Close()

	gotConfig := gc.GetConfig()
	if gotConfig.Interval != 5*time.Second {
		t.Errorf("expected interval 5s, got %v", gotConfig.Interval)
	}
	if gotConfig.MinVersionAge != 1*time.Second {
		t.Errorf("expected MinVersionAge 1s, got %v", gotConfig.MinVersionAge)
	}
	if gotConfig.BatchSize != 100 {
		t.Errorf("expected BatchSize 100, got %d", gotConfig.BatchSize)
	}
}

// TestNewGarbageCollectorWithZeroInterval tests that zero interval uses default.
func TestNewGarbageCollectorWithZeroInterval(t *testing.T) {
	config := GCConfig{
		Interval: 0,
	}

	gc := NewGarbageCollectorWithConfig(nil, nil, nil, config)
	defer gc.Close()

	gotConfig := gc.GetConfig()
	if gotConfig.Interval != DefaultGCInterval {
		t.Errorf("expected default interval for zero config, got %v", gotConfig.Interval)
	}
}

// --- Start/Stop Tests ---

// TestGCStartStop tests starting and stopping the GC.
func TestGCStartStop(t *testing.T) {
	gc, _, _, _, tmpDir := createTestGCEnvironment(t)
	defer cleanupTestGCEnvironment(gc, tmpDir)

	// Start GC
	err := gc.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if !gc.IsRunning() {
		t.Error("GC should be running after Start")
	}

	// Stop GC
	err = gc.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	if gc.IsRunning() {
		t.Error("GC should not be running after Stop")
	}
}

// TestGCStartAlreadyRunning tests starting when already running.
func TestGCStartAlreadyRunning(t *testing.T) {
	gc, _, _, _, tmpDir := createTestGCEnvironment(t)
	defer cleanupTestGCEnvironment(gc, tmpDir)

	err := gc.Start()
	if err != nil {
		t.Fatalf("First Start failed: %v", err)
	}

	err = gc.Start()
	if err != ErrGCAlreadyRunning {
		t.Errorf("expected ErrGCAlreadyRunning, got %v", err)
	}

	gc.Stop()
}

// TestGCStopNotRunning tests stopping when not running.
func TestGCStopNotRunning(t *testing.T) {
	gc, _, _, _, tmpDir := createTestGCEnvironment(t)
	defer cleanupTestGCEnvironment(gc, tmpDir)

	err := gc.Stop()
	if err != ErrGCNotRunning {
		t.Errorf("expected ErrGCNotRunning, got %v", err)
	}
}

// TestGCStartAfterClose tests starting after close.
func TestGCStartAfterClose(t *testing.T) {
	gc, _, _, _, tmpDir := createTestGCEnvironment(t)
	defer os.RemoveAll(tmpDir)

	gc.Close()

	err := gc.Start()
	if err != ErrGCClosed {
		t.Errorf("expected ErrGCClosed, got %v", err)
	}
}

// TestGCStopAfterClose tests stopping after close.
func TestGCStopAfterClose(t *testing.T) {
	gc, _, _, _, tmpDir := createTestGCEnvironment(t)
	defer os.RemoveAll(tmpDir)

	gc.Close()

	err := gc.Stop()
	if err != ErrGCClosed {
		t.Errorf("expected ErrGCClosed, got %v", err)
	}
}

// --- Collect Tests ---

// TestGCCollectEmpty tests collecting with no versions.
func TestGCCollectEmpty(t *testing.T) {
	gc, _, _, _, tmpDir := createTestGCEnvironment(t)
	defer cleanupTestGCEnvironment(gc, tmpDir)

	pagesFreed, err := gc.Collect()
	if err != nil {
		t.Fatalf("Collect failed: %v", err)
	}

	if pagesFreed != 0 {
		t.Errorf("expected 0 pages freed for empty store, got %d", pagesFreed)
	}

	stats := gc.Stats()
	if stats.TotalRuns != 1 {
		t.Errorf("expected TotalRuns 1, got %d", stats.TotalRuns)
	}
}

// TestGCCollectWithVersions tests collecting old versions.
func TestGCCollectWithVersions(t *testing.T) {
	gc, vs, sm, txMgr, tmpDir := createTestGCEnvironment(t)
	defer cleanupTestGCEnvironment(gc, tmpDir)

	// Create a transaction and add some versions
	tx1, err := txMgr.Begin()
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	// Create initial version
	err = vs.CreateVersion(tx1, "uid=alice,ou=users,dc=example,dc=com", []byte("version1"))
	if err != nil {
		t.Fatalf("CreateVersion failed: %v", err)
	}

	// Commit the transaction
	commitTS := sm.AdvanceTimestamp()
	vs.CommitVersion(tx1, commitTS)
	tx1.SetState(tx.TxCommitted)

	// Create another transaction with a new version
	tx2, err := txMgr.Begin()
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	err = vs.CreateVersion(tx2, "uid=alice,ou=users,dc=example,dc=com", []byte("version2"))
	if err != nil {
		t.Fatalf("CreateVersion failed: %v", err)
	}

	commitTS2 := sm.AdvanceTimestamp()
	vs.CommitVersion(tx2, commitTS2)
	tx2.SetState(tx.TxCommitted)

	// Verify we have a version chain
	chain := vs.GetVersionChain("uid=alice,ou=users,dc=example,dc=com")
	if len(chain) != 2 {
		t.Errorf("expected 2 versions in chain, got %d", len(chain))
	}

	// Run GC - should collect old versions since no active snapshots
	_, err = gc.Collect()
	if err != nil {
		t.Fatalf("Collect failed: %v", err)
	}

	stats := gc.Stats()
	if stats.TotalRuns != 1 {
		t.Errorf("expected TotalRuns 1, got %d", stats.TotalRuns)
	}
}

// TestGCCollectPreservesActiveSnapshots tests that active snapshots are preserved.
func TestGCCollectPreservesActiveSnapshots(t *testing.T) {
	gc, vs, sm, txMgr, tmpDir := createTestGCEnvironment(t)
	defer cleanupTestGCEnvironment(gc, tmpDir)

	// Create initial version
	tx1, _ := txMgr.Begin()
	vs.CreateVersion(tx1, "uid=bob,ou=users,dc=example,dc=com", []byte("version1"))
	commitTS1 := sm.AdvanceTimestamp()
	vs.CommitVersion(tx1, commitTS1)
	tx1.SetState(tx.TxCommitted)

	// Create a snapshot that should see version1
	tx2, _ := txMgr.Begin()
	snapshot, _ := sm.CreateSnapshot(tx2)

	// Create a new version
	tx3, _ := txMgr.Begin()
	vs.CreateVersion(tx3, "uid=bob,ou=users,dc=example,dc=com", []byte("version2"))
	commitTS2 := sm.AdvanceTimestamp()
	vs.CommitVersion(tx3, commitTS2)
	tx3.SetState(tx.TxCommitted)

	// Run GC - should NOT collect version1 because snapshot still needs it
	_, err := gc.Collect()
	if err != nil {
		t.Fatalf("Collect failed: %v", err)
	}

	// Verify version1 is still accessible via the snapshot
	visible, err := vs.GetVisibleForTx("uid=bob,ou=users,dc=example,dc=com", snapshot.Timestamp, tx2.ID)
	if err != nil {
		t.Fatalf("GetVisibleForTx failed: %v", err)
	}

	if string(visible.GetData()) != "version1" {
		t.Errorf("expected version1 data, got %s", string(visible.GetData()))
	}

	// Release the snapshot
	sm.ReleaseSnapshot(snapshot)

	// Run GC again - now version1 can be collected
	_, err = gc.Collect()
	if err != nil {
		t.Fatalf("Second Collect failed: %v", err)
	}
}

// TestGCCollectAfterClose tests collecting after close.
func TestGCCollectAfterClose(t *testing.T) {
	gc, _, _, _, tmpDir := createTestGCEnvironment(t)
	defer os.RemoveAll(tmpDir)

	gc.Close()

	_, err := gc.Collect()
	if err != ErrGCClosed {
		t.Errorf("expected ErrGCClosed, got %v", err)
	}
}

// --- CollectEntry Tests ---

// TestGCCollectEntry tests collecting a specific entry.
func TestGCCollectEntry(t *testing.T) {
	gc, vs, sm, txMgr, tmpDir := createTestGCEnvironment(t)
	defer cleanupTestGCEnvironment(gc, tmpDir)

	dn := "uid=charlie,ou=users,dc=example,dc=com"

	// Create multiple versions
	tx1, _ := txMgr.Begin()
	vs.CreateVersion(tx1, dn, []byte("v1"))
	commitTS1 := sm.AdvanceTimestamp()
	vs.CommitVersion(tx1, commitTS1)
	tx1.SetState(tx.TxCommitted)

	tx2, _ := txMgr.Begin()
	vs.CreateVersion(tx2, dn, []byte("v2"))
	commitTS2 := sm.AdvanceTimestamp()
	vs.CommitVersion(tx2, commitTS2)
	tx2.SetState(tx.TxCommitted)

	// Collect the specific entry
	err := gc.CollectEntry(dn)
	if err != nil {
		t.Fatalf("CollectEntry failed: %v", err)
	}
}

// TestGCCollectEntryNotFound tests collecting a non-existent entry.
func TestGCCollectEntryNotFound(t *testing.T) {
	gc, _, _, _, tmpDir := createTestGCEnvironment(t)
	defer cleanupTestGCEnvironment(gc, tmpDir)

	err := gc.CollectEntry("uid=nonexistent,dc=example,dc=com")
	if err != ErrEntryNotFound {
		t.Errorf("expected ErrEntryNotFound, got %v", err)
	}
}

// TestGCCollectEntryAfterClose tests collecting entry after close.
func TestGCCollectEntryAfterClose(t *testing.T) {
	gc, _, _, _, tmpDir := createTestGCEnvironment(t)
	defer os.RemoveAll(tmpDir)

	gc.Close()

	err := gc.CollectEntry("uid=test,dc=example,dc=com")
	if err != ErrGCClosed {
		t.Errorf("expected ErrGCClosed, got %v", err)
	}
}

// --- Stats Tests ---

// TestGCStats tests statistics tracking.
func TestGCStats(t *testing.T) {
	gc, _, _, _, tmpDir := createTestGCEnvironment(t)
	defer cleanupTestGCEnvironment(gc, tmpDir)

	// Initial stats
	stats := gc.Stats()
	if stats.TotalRuns != 0 {
		t.Errorf("expected TotalRuns 0 initially, got %d", stats.TotalRuns)
	}

	// Run GC multiple times
	gc.Collect()
	gc.Collect()
	gc.Collect()

	stats = gc.Stats()
	if stats.TotalRuns != 3 {
		t.Errorf("expected TotalRuns 3, got %d", stats.TotalRuns)
	}

	if stats.LastRunTime.IsZero() {
		t.Error("LastRunTime should not be zero")
	}
}

// --- Configuration Tests ---

// TestGCSetInterval tests setting the GC interval.
func TestGCSetInterval(t *testing.T) {
	gc, _, _, _, tmpDir := createTestGCEnvironment(t)
	defer cleanupTestGCEnvironment(gc, tmpDir)

	newInterval := 10 * time.Second
	gc.SetInterval(newInterval)

	config := gc.GetConfig()
	if config.Interval != newInterval {
		t.Errorf("expected interval %v, got %v", newInterval, config.Interval)
	}
}

// --- TriggerCollect Tests ---

// TestGCTriggerCollect tests triggering an immediate GC.
func TestGCTriggerCollect(t *testing.T) {
	gc, _, _, _, tmpDir := createTestGCEnvironment(t)
	defer cleanupTestGCEnvironment(gc, tmpDir)

	pagesFreed, err := gc.TriggerCollect()
	if err != nil {
		t.Fatalf("TriggerCollect failed: %v", err)
	}

	if pagesFreed != 0 {
		t.Errorf("expected 0 pages freed, got %d", pagesFreed)
	}

	stats := gc.Stats()
	if stats.TotalRuns != 1 {
		t.Errorf("expected TotalRuns 1, got %d", stats.TotalRuns)
	}
}

// --- Getter Tests ---

// TestGCGetters tests getter methods.
func TestGCGetters(t *testing.T) {
	gc, vs, sm, _, tmpDir := createTestGCEnvironment(t)
	defer cleanupTestGCEnvironment(gc, tmpDir)

	if gc.GetVersionStore() != vs {
		t.Error("GetVersionStore returned wrong value")
	}

	if gc.GetSnapshotManager() != sm {
		t.Error("GetSnapshotManager returned wrong value")
	}

	if gc.GetPageManager() == nil {
		t.Error("GetPageManager returned nil")
	}
}

// --- Close Tests ---

// TestGCClose tests closing the GC.
func TestGCClose(t *testing.T) {
	gc, _, _, _, tmpDir := createTestGCEnvironment(t)
	defer os.RemoveAll(tmpDir)

	err := gc.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Double close should be safe
	err = gc.Close()
	if err != nil {
		t.Fatalf("Double Close failed: %v", err)
	}
}

// TestGCCloseWhileRunning tests closing while GC is running.
func TestGCCloseWhileRunning(t *testing.T) {
	gc, _, _, _, tmpDir := createTestGCEnvironment(t)
	defer os.RemoveAll(tmpDir)

	err := gc.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	err = gc.Close()
	if err != nil {
		t.Fatalf("Close while running failed: %v", err)
	}

	if gc.IsRunning() {
		t.Error("GC should not be running after Close")
	}
}

// --- Concurrency Tests ---

// TestGCConcurrentCollect tests concurrent GC operations.
func TestGCConcurrentCollect(t *testing.T) {
	gc, vs, sm, txMgr, tmpDir := createTestGCEnvironment(t)
	defer cleanupTestGCEnvironment(gc, tmpDir)

	// Create some versions
	for i := 0; i < 10; i++ {
		txn, _ := txMgr.Begin()
		dn := "uid=user" + string(rune('0'+i)) + ",ou=users,dc=example,dc=com"
		vs.CreateVersion(txn, dn, []byte("data"))
		commitTS := sm.AdvanceTimestamp()
		vs.CommitVersion(txn, commitTS)
		txn.SetState(tx.TxCommitted)
	}

	// Run concurrent GC operations
	var wg sync.WaitGroup
	numGoroutines := 5

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				gc.Collect()
			}
		}()
	}

	wg.Wait()

	stats := gc.Stats()
	expectedRuns := uint64(numGoroutines * 10)
	if stats.TotalRuns != expectedRuns {
		t.Errorf("expected TotalRuns %d, got %d", expectedRuns, stats.TotalRuns)
	}
}

// TestGCConcurrentStartStop tests concurrent start/stop operations.
func TestGCConcurrentStartStop(t *testing.T) {
	gc, _, _, _, tmpDir := createTestGCEnvironment(t)
	defer cleanupTestGCEnvironment(gc, tmpDir)

	var wg sync.WaitGroup
	numGoroutines := 10

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				// Ignore errors since concurrent operations may conflict
				_ = gc.Start()
				time.Sleep(1 * time.Millisecond)
				_ = gc.Stop()
			}
		}()
	}

	wg.Wait()

	// GC should be stopped at the end - try to stop if still running
	_ = gc.Stop()
}

// --- Background GC Tests ---

// TestGCBackgroundRun tests that background GC runs periodically.
func TestGCBackgroundRun(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gc_bg_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create GC with short interval
	config := GCConfig{
		Interval: 50 * time.Millisecond,
	}
	gc := NewGarbageCollectorWithConfig(nil, nil, nil, config)
	defer gc.Close()

	err = gc.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Wait for a few GC cycles
	time.Sleep(200 * time.Millisecond)

	err = gc.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	stats := gc.Stats()
	if stats.TotalRuns < 2 {
		t.Errorf("expected at least 2 background runs, got %d", stats.TotalRuns)
	}
}

// --- Nil Dependencies Tests ---

// TestGCWithNilDependencies tests GC with nil dependencies.
func TestGCWithNilDependencies(t *testing.T) {
	gc := NewGarbageCollector(nil, nil, nil)
	defer gc.Close()

	// Should not panic
	pagesFreed, err := gc.Collect()
	if err != nil {
		t.Fatalf("Collect with nil deps failed: %v", err)
	}

	if pagesFreed != 0 {
		t.Errorf("expected 0 pages freed with nil deps, got %d", pagesFreed)
	}
}

// TestGCCollectEntryWithNilVersionStore tests CollectEntry with nil version store.
func TestGCCollectEntryWithNilVersionStore(t *testing.T) {
	gc := NewGarbageCollector(nil, nil, nil)
	defer gc.Close()

	// Should return nil (no error) when version store is nil
	err := gc.CollectEntry("uid=test,dc=example,dc=com")
	if err != nil {
		t.Errorf("expected nil error with nil version store, got %v", err)
	}
}

// --- Deleted Entry Tests ---

// TestGCCollectDeletedEntry tests collecting a deleted entry.
func TestGCCollectDeletedEntry(t *testing.T) {
	gc, vs, sm, txMgr, tmpDir := createTestGCEnvironment(t)
	defer cleanupTestGCEnvironment(gc, tmpDir)

	dn := "uid=deleted,ou=users,dc=example,dc=com"

	// Create an entry
	tx1, _ := txMgr.Begin()
	vs.CreateVersion(tx1, dn, []byte("data"))
	commitTS1 := sm.AdvanceTimestamp()
	vs.CommitVersion(tx1, commitTS1)
	tx1.SetState(tx.TxCommitted)

	// Delete the entry
	tx2, _ := txMgr.Begin()
	vs.DeleteVersion(tx2, dn)
	commitTS2 := sm.AdvanceTimestamp()
	vs.CommitVersion(tx2, commitTS2)
	tx2.SetState(tx.TxCommitted)

	// Run GC - should clean up the deleted entry
	_, err := gc.Collect()
	if err != nil {
		t.Fatalf("Collect failed: %v", err)
	}
}

// --- DefaultGCConfig Tests ---

// TestDefaultGCConfig tests the default configuration.
func TestDefaultGCConfig(t *testing.T) {
	config := DefaultGCConfig()

	if config.Interval != DefaultGCInterval {
		t.Errorf("expected default interval %v, got %v", DefaultGCInterval, config.Interval)
	}

	if config.MinVersionAge != 0 {
		t.Errorf("expected MinVersionAge 0, got %v", config.MinVersionAge)
	}

	if config.BatchSize != 0 {
		t.Errorf("expected BatchSize 0, got %d", config.BatchSize)
	}
}
