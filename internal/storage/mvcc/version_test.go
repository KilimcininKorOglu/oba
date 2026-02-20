// Package mvcc provides Multi-Version Concurrency Control for ObaDB.
package mvcc

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/oba-ldap/oba/internal/storage"
	"github.com/oba-ldap/oba/internal/storage/tx"
)

// TestNewVersion tests the creation of a new version.
func TestNewVersion(t *testing.T) {
	data := []byte("test data")
	pageID := storage.PageID(42)
	slotID := uint16(7)
	txID := uint64(100)

	v := NewVersion(txID, data, pageID, slotID)

	if v.TxID != txID {
		t.Errorf("expected TxID %d, got %d", txID, v.TxID)
	}
	if v.CommitTS != 0 {
		t.Errorf("expected CommitTS 0, got %d", v.CommitTS)
	}
	if string(v.Data) != string(data) {
		t.Errorf("expected data %q, got %q", data, v.Data)
	}
	if v.PageID != pageID {
		t.Errorf("expected PageID %d, got %d", pageID, v.PageID)
	}
	if v.SlotID != slotID {
		t.Errorf("expected SlotID %d, got %d", slotID, v.SlotID)
	}
	if v.State != VersionActive {
		t.Errorf("expected state Active, got %s", v.State)
	}
	if v.Prev != nil {
		t.Error("expected Prev to be nil")
	}
}

// TestNewDeletedVersion tests the creation of a deleted version.
func TestNewDeletedVersion(t *testing.T) {
	pageID := storage.PageID(42)
	slotID := uint16(7)
	txID := uint64(100)

	v := NewDeletedVersion(txID, pageID, slotID)

	if v.TxID != txID {
		t.Errorf("expected TxID %d, got %d", txID, v.TxID)
	}
	if v.CommitTS != 0 {
		t.Errorf("expected CommitTS 0, got %d", v.CommitTS)
	}
	if v.Data != nil {
		t.Errorf("expected nil data, got %v", v.Data)
	}
	if v.State != VersionDeleted {
		t.Errorf("expected state Deleted, got %s", v.State)
	}
}

// TestVersionCommit tests committing a version.
func TestVersionCommit(t *testing.T) {
	v := NewVersion(1, []byte("data"), 1, 0)

	if v.IsCommitted() {
		t.Error("version should not be committed initially")
	}

	commitTS := uint64(500)
	v.Commit(commitTS)

	if !v.IsCommitted() {
		t.Error("version should be committed after Commit()")
	}
	if v.GetCommitTS() != commitTS {
		t.Errorf("expected CommitTS %d, got %d", commitTS, v.GetCommitTS())
	}
}

// TestVersionVisibility tests the visibility rules.
func TestVersionVisibility(t *testing.T) {
	tests := []struct {
		name        string
		txID        uint64
		commitTS    uint64
		snapshot    uint64
		activeTxID  uint64
		wantVisible bool
	}{
		{
			name:        "uncommitted visible to creator",
			txID:        100,
			commitTS:    0,
			snapshot:    200,
			activeTxID:  100,
			wantVisible: true,
		},
		{
			name:        "uncommitted not visible to others",
			txID:        100,
			commitTS:    0,
			snapshot:    200,
			activeTxID:  101,
			wantVisible: false,
		},
		{
			name:        "committed visible when commitTS <= snapshot",
			txID:        100,
			commitTS:    150,
			snapshot:    200,
			activeTxID:  0,
			wantVisible: true,
		},
		{
			name:        "committed visible when commitTS == snapshot",
			txID:        100,
			commitTS:    200,
			snapshot:    200,
			activeTxID:  0,
			wantVisible: true,
		},
		{
			name:        "committed not visible when commitTS > snapshot",
			txID:        100,
			commitTS:    250,
			snapshot:    200,
			activeTxID:  0,
			wantVisible: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewVersion(tt.txID, []byte("data"), 1, 0)
			if tt.commitTS > 0 {
				v.Commit(tt.commitTS)
			}

			got := v.IsVisibleTo(tt.snapshot, tt.activeTxID)
			if got != tt.wantVisible {
				t.Errorf("IsVisibleTo() = %v, want %v", got, tt.wantVisible)
			}
		})
	}
}

// TestVersionChain tests version chain operations.
func TestVersionChain(t *testing.T) {
	v1 := NewVersion(1, []byte("v1"), 1, 0)
	v2 := NewVersion(2, []byte("v2"), 2, 0)
	v3 := NewVersion(3, []byte("v3"), 3, 0)

	v2.SetPrev(v1)
	v3.SetPrev(v2)

	if v3.ChainLength() != 3 {
		t.Errorf("expected chain length 3, got %d", v3.ChainLength())
	}

	if v3.GetPrev() != v2 {
		t.Error("v3.Prev should be v2")
	}
	if v2.GetPrev() != v1 {
		t.Error("v2.Prev should be v1")
	}
	if v1.GetPrev() != nil {
		t.Error("v1.Prev should be nil")
	}
}

// TestVersionSerializeHeader tests header serialization.
func TestVersionSerializeHeader(t *testing.T) {
	v := NewVersion(12345, []byte("data"), storage.PageID(67890), 42)
	v.Commit(99999)
	v.State = VersionDeleted

	buf := v.SerializeHeader()
	if len(buf) != VersionHeaderSize {
		t.Errorf("expected header size %d, got %d", VersionHeaderSize, len(buf))
	}

	v2, err := DeserializeVersionHeader(buf)
	if err != nil {
		t.Fatalf("DeserializeVersionHeader failed: %v", err)
	}

	if v2.TxID != v.TxID {
		t.Errorf("TxID mismatch: got %d, want %d", v2.TxID, v.TxID)
	}
	if v2.CommitTS != v.CommitTS {
		t.Errorf("CommitTS mismatch: got %d, want %d", v2.CommitTS, v.CommitTS)
	}
	if v2.PageID != v.PageID {
		t.Errorf("PageID mismatch: got %d, want %d", v2.PageID, v.PageID)
	}
	if v2.SlotID != v.SlotID {
		t.Errorf("SlotID mismatch: got %d, want %d", v2.SlotID, v.SlotID)
	}
	if v2.State != v.State {
		t.Errorf("State mismatch: got %s, want %s", v2.State, v.State)
	}
}

// TestVersionClone tests version cloning.
func TestVersionClone(t *testing.T) {
	v := NewVersion(100, []byte("original data"), 42, 7)
	v.Commit(200)

	clone := v.Clone()

	if clone.TxID != v.TxID {
		t.Errorf("TxID mismatch")
	}
	if clone.CommitTS != v.CommitTS {
		t.Errorf("CommitTS mismatch")
	}
	if string(clone.Data) != string(v.Data) {
		t.Errorf("Data mismatch")
	}
	if clone.Prev != nil {
		t.Error("Clone should not copy Prev pointer")
	}

	// Verify data is a copy, not a reference
	clone.Data[0] = 'X'
	if v.Data[0] == 'X' {
		t.Error("Clone data should be independent")
	}
}

// TestVersionGetData tests that GetData returns a copy.
func TestVersionGetData(t *testing.T) {
	original := []byte("original data")
	v := NewVersion(1, original, 1, 0)

	data := v.GetData()
	data[0] = 'X'

	if v.Data[0] == 'X' {
		t.Error("GetData should return a copy, not the original")
	}
}

// TestVersionStateString tests VersionState string representation.
func TestVersionStateString(t *testing.T) {
	if VersionActive.String() != "Active" {
		t.Errorf("expected 'Active', got %q", VersionActive.String())
	}
	if VersionDeleted.String() != "Deleted" {
		t.Errorf("expected 'Deleted', got %q", VersionDeleted.String())
	}
	if VersionState(99).String() != "Unknown" {
		t.Errorf("expected 'Unknown', got %q", VersionState(99).String())
	}
}

// --- VersionStore Tests ---

// createTestWAL creates a temporary WAL for testing.
func createTestWAL(t *testing.T) (*storage.WAL, string) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "mvcc_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	walPath := filepath.Join(tmpDir, "test.wal")
	wal, err := storage.OpenWAL(walPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to create WAL: %v", err)
	}

	return wal, tmpDir
}

// TestVersionStoreBasic tests basic VersionStore operations.
func TestVersionStoreBasic(t *testing.T) {
	vs := NewVersionStore(nil)

	if vs.EntryCount() != 0 {
		t.Errorf("expected 0 entries, got %d", vs.EntryCount())
	}

	if vs.HasEntry("cn=test,dc=example,dc=com") {
		t.Error("should not have entry before creation")
	}
}

// TestVersionStoreCreateAndGet tests creating and getting versions.
func TestVersionStoreCreateAndGet(t *testing.T) {
	wal, tmpDir := createTestWAL(t)
	defer os.RemoveAll(tmpDir)
	defer wal.Close()

	txMgr := tx.NewTxManager(wal)
	vs := NewVersionStore(nil)

	// Begin a transaction
	txn, err := txMgr.Begin()
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}

	dn := "cn=alice,ou=users,dc=example,dc=com"
	data := []byte("user data for alice")

	// Create a version
	err = vs.CreateVersion(txn, dn, data)
	if err != nil {
		t.Fatalf("CreateVersion failed: %v", err)
	}

	// Version should be visible to the creating transaction
	v, err := vs.GetVisibleForTx(dn, txn.Snapshot, txn.ID)
	if err != nil {
		t.Fatalf("GetVisibleForTx failed: %v", err)
	}
	if string(v.Data) != string(data) {
		t.Errorf("data mismatch: got %q, want %q", v.Data, data)
	}

	// Version should NOT be visible to other transactions (uncommitted)
	_, err = vs.GetVisibleForTx(dn, txn.Snapshot, txn.ID+1)
	if err != ErrVersionNotFound {
		t.Errorf("expected ErrVersionNotFound, got %v", err)
	}

	// Commit the transaction
	commitTS := uint64(1000)
	vs.CommitVersion(txn, commitTS)

	// Now version should be visible to snapshots >= commitTS
	v, err = vs.GetVisible(dn, commitTS)
	if err != nil {
		t.Fatalf("GetVisible failed after commit: %v", err)
	}
	if string(v.Data) != string(data) {
		t.Errorf("data mismatch after commit: got %q, want %q", v.Data, data)
	}

	// Version should NOT be visible to older snapshots
	_, err = vs.GetVisible(dn, commitTS-1)
	if err != ErrVersionNotFound {
		t.Errorf("expected ErrVersionNotFound for old snapshot, got %v", err)
	}
}

// TestVersionStoreDelete tests deleting versions.
func TestVersionStoreDelete(t *testing.T) {
	wal, tmpDir := createTestWAL(t)
	defer os.RemoveAll(tmpDir)
	defer wal.Close()

	txMgr := tx.NewTxManager(wal)
	vs := NewVersionStore(nil)

	dn := "cn=bob,ou=users,dc=example,dc=com"
	data := []byte("user data for bob")

	// Create initial version
	tx1, _ := txMgr.Begin()
	vs.CreateVersion(tx1, dn, data)
	vs.CommitVersion(tx1, 100)

	// Verify entry exists
	v, err := vs.GetVisible(dn, 100)
	if err != nil {
		t.Fatalf("GetVisible failed: %v", err)
	}
	if string(v.Data) != string(data) {
		t.Errorf("data mismatch")
	}

	// Delete the entry
	tx2, _ := txMgr.Begin()
	err = vs.DeleteVersion(tx2, dn)
	if err != nil {
		t.Fatalf("DeleteVersion failed: %v", err)
	}
	vs.CommitVersion(tx2, 200)

	// Entry should appear deleted for new snapshots
	_, err = vs.GetVisible(dn, 200)
	if err != ErrVersionDeleted {
		t.Errorf("expected ErrVersionDeleted, got %v", err)
	}

	// Old snapshots should still see the original version
	v, err = vs.GetVisible(dn, 100)
	if err != nil {
		t.Fatalf("GetVisible for old snapshot failed: %v", err)
	}
	if string(v.Data) != string(data) {
		t.Errorf("old snapshot should see original data")
	}
}

// TestVersionStoreRollback tests rolling back uncommitted versions.
func TestVersionStoreRollback(t *testing.T) {
	wal, tmpDir := createTestWAL(t)
	defer os.RemoveAll(tmpDir)
	defer wal.Close()

	txMgr := tx.NewTxManager(wal)
	vs := NewVersionStore(nil)

	dn := "cn=charlie,ou=users,dc=example,dc=com"
	data := []byte("user data for charlie")

	// Create a version
	txn, _ := txMgr.Begin()
	vs.CreateVersion(txn, dn, data)

	// Verify entry exists for the transaction
	_, err := vs.GetVisibleForTx(dn, txn.Snapshot, txn.ID)
	if err != nil {
		t.Fatalf("GetVisibleForTx failed: %v", err)
	}

	// Rollback the transaction
	vs.RollbackVersion(txn)

	// Entry should no longer exist
	_, err = vs.GetVisibleForTx(dn, txn.Snapshot, txn.ID)
	if err != ErrVersionNotFound {
		t.Errorf("expected ErrVersionNotFound after rollback, got %v", err)
	}
}

// TestVersionStoreMultipleVersions tests multiple versions of the same entry.
func TestVersionStoreMultipleVersions(t *testing.T) {
	wal, tmpDir := createTestWAL(t)
	defer os.RemoveAll(tmpDir)
	defer wal.Close()

	txMgr := tx.NewTxManager(wal)
	vs := NewVersionStore(nil)

	dn := "cn=dave,ou=users,dc=example,dc=com"

	// Create version 1
	tx1, _ := txMgr.Begin()
	vs.CreateVersion(tx1, dn, []byte("version 1"))
	vs.CommitVersion(tx1, 100)

	// Create version 2
	tx2, _ := txMgr.Begin()
	vs.CreateVersion(tx2, dn, []byte("version 2"))
	vs.CommitVersion(tx2, 200)

	// Create version 3
	tx3, _ := txMgr.Begin()
	vs.CreateVersion(tx3, dn, []byte("version 3"))
	vs.CommitVersion(tx3, 300)

	// Check version chain length
	chain := vs.GetVersionChain(dn)
	if len(chain) != 3 {
		t.Errorf("expected 3 versions in chain, got %d", len(chain))
	}

	// Snapshot at 100 should see version 1
	v, _ := vs.GetVisible(dn, 100)
	if string(v.Data) != "version 1" {
		t.Errorf("snapshot 100 should see version 1, got %q", v.Data)
	}

	// Snapshot at 200 should see version 2
	v, _ = vs.GetVisible(dn, 200)
	if string(v.Data) != "version 2" {
		t.Errorf("snapshot 200 should see version 2, got %q", v.Data)
	}

	// Snapshot at 300 should see version 3
	v, _ = vs.GetVisible(dn, 300)
	if string(v.Data) != "version 3" {
		t.Errorf("snapshot 300 should see version 3, got %q", v.Data)
	}

	// Snapshot at 150 should see version 1
	v, _ = vs.GetVisible(dn, 150)
	if string(v.Data) != "version 1" {
		t.Errorf("snapshot 150 should see version 1, got %q", v.Data)
	}
}

// TestVersionStoreWriteConflict tests write-write conflict detection.
func TestVersionStoreWriteConflict(t *testing.T) {
	wal, tmpDir := createTestWAL(t)
	defer os.RemoveAll(tmpDir)
	defer wal.Close()

	txMgr := tx.NewTxManager(wal)
	vs := NewVersionStore(nil)

	dn := "cn=eve,ou=users,dc=example,dc=com"

	// Transaction 1 creates a version
	tx1, _ := txMgr.Begin()
	err := vs.CreateVersion(tx1, dn, []byte("tx1 data"))
	if err != nil {
		t.Fatalf("tx1 CreateVersion failed: %v", err)
	}

	// Transaction 2 tries to create a version for the same DN
	tx2, _ := txMgr.Begin()
	err = vs.CreateVersion(tx2, dn, []byte("tx2 data"))
	if err != ErrVersionConflict {
		t.Errorf("expected ErrVersionConflict, got %v", err)
	}

	// After tx1 commits, tx2 should still fail (tx1 already has uncommitted write)
	// Let's commit tx1 first
	vs.CommitVersion(tx1, 100)

	// Now tx2 should be able to create a version
	tx3, _ := txMgr.Begin()
	err = vs.CreateVersion(tx3, dn, []byte("tx3 data"))
	if err != nil {
		t.Errorf("tx3 CreateVersion should succeed after tx1 commit, got %v", err)
	}
}

// TestVersionStoreGarbageCollect tests garbage collection.
func TestVersionStoreGarbageCollect(t *testing.T) {
	wal, tmpDir := createTestWAL(t)
	defer os.RemoveAll(tmpDir)
	defer wal.Close()

	txMgr := tx.NewTxManager(wal)
	vs := NewVersionStore(nil)

	dn := "cn=frank,ou=users,dc=example,dc=com"

	// Create multiple versions
	for i := 1; i <= 5; i++ {
		txn, _ := txMgr.Begin()
		vs.CreateVersion(txn, dn, []byte("version"))
		vs.CommitVersion(txn, uint64(i*100))
	}

	// Verify chain length
	chain := vs.GetVersionChain(dn)
	if len(chain) != 5 {
		t.Errorf("expected 5 versions, got %d", len(chain))
	}

	// Garbage collect with oldest snapshot at 300
	removed := vs.GarbageCollect(300)
	if removed < 2 {
		t.Errorf("expected at least 2 versions removed, got %d", removed)
	}

	// Chain should be shorter now
	chain = vs.GetVersionChain(dn)
	if len(chain) >= 5 {
		t.Errorf("expected fewer than 5 versions after GC, got %d", len(chain))
	}
}

// TestVersionStoreStats tests statistics gathering.
func TestVersionStoreStats(t *testing.T) {
	wal, tmpDir := createTestWAL(t)
	defer os.RemoveAll(tmpDir)
	defer wal.Close()

	txMgr := tx.NewTxManager(wal)
	vs := NewVersionStore(nil)

	// Create some entries
	for i := 0; i < 3; i++ {
		txn, _ := txMgr.Begin()
		dn := "cn=user" + string(rune('a'+i)) + ",dc=example,dc=com"
		vs.CreateVersion(txn, dn, []byte("data"))
		vs.CommitVersion(txn, uint64((i+1)*100))
	}

	stats := vs.Stats()
	if stats.EntryCount != 3 {
		t.Errorf("expected 3 entries, got %d", stats.EntryCount)
	}
	if stats.TotalVersions != 3 {
		t.Errorf("expected 3 total versions, got %d", stats.TotalVersions)
	}
}

// TestVersionStoreClear tests clearing the store.
func TestVersionStoreClear(t *testing.T) {
	wal, tmpDir := createTestWAL(t)
	defer os.RemoveAll(tmpDir)
	defer wal.Close()

	txMgr := tx.NewTxManager(wal)
	vs := NewVersionStore(nil)

	// Create some entries
	txn, _ := txMgr.Begin()
	vs.CreateVersion(txn, "cn=test,dc=example,dc=com", []byte("data"))
	vs.CommitVersion(txn, 100)

	if vs.EntryCount() != 1 {
		t.Errorf("expected 1 entry, got %d", vs.EntryCount())
	}

	vs.Clear()

	if vs.EntryCount() != 0 {
		t.Errorf("expected 0 entries after clear, got %d", vs.EntryCount())
	}
}

// TestVersionStoreNilTransaction tests error handling for nil transaction.
func TestVersionStoreNilTransaction(t *testing.T) {
	vs := NewVersionStore(nil)

	err := vs.CreateVersion(nil, "cn=test,dc=example,dc=com", []byte("data"))
	if err != ErrNilTransaction {
		t.Errorf("expected ErrNilTransaction, got %v", err)
	}

	err = vs.DeleteVersion(nil, "cn=test,dc=example,dc=com")
	if err != ErrNilTransaction {
		t.Errorf("expected ErrNilTransaction, got %v", err)
	}
}

// TestVersionStoreDeleteNonExistent tests deleting a non-existent entry.
func TestVersionStoreDeleteNonExistent(t *testing.T) {
	wal, tmpDir := createTestWAL(t)
	defer os.RemoveAll(tmpDir)
	defer wal.Close()

	txMgr := tx.NewTxManager(wal)
	vs := NewVersionStore(nil)

	txn, _ := txMgr.Begin()
	err := vs.DeleteVersion(txn, "cn=nonexistent,dc=example,dc=com")
	if err != ErrVersionNotFound {
		t.Errorf("expected ErrVersionNotFound, got %v", err)
	}
}

// TestVersionStoreGetNonExistent tests getting a non-existent entry.
func TestVersionStoreGetNonExistent(t *testing.T) {
	vs := NewVersionStore(nil)

	_, err := vs.GetVisible("cn=nonexistent,dc=example,dc=com", 100)
	if err != ErrVersionNotFound {
		t.Errorf("expected ErrVersionNotFound, got %v", err)
	}
}

// TestVersionStoreWithPageManager tests VersionStore with a real PageManager.
func TestVersionStoreWithPageManager(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mvcc_pm_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create PageManager
	pmPath := filepath.Join(tmpDir, "test.db")
	pm, err := storage.OpenPageManager(pmPath, storage.DefaultOptions())
	if err != nil {
		t.Fatalf("failed to create PageManager: %v", err)
	}
	defer pm.Close()

	// Create WAL
	walPath := filepath.Join(tmpDir, "test.wal")
	wal, err := storage.OpenWAL(walPath)
	if err != nil {
		t.Fatalf("failed to create WAL: %v", err)
	}
	defer wal.Close()

	txMgr := tx.NewTxManager(wal)
	vs := NewVersionStore(pm)

	dn := "cn=test,dc=example,dc=com"
	data := []byte("test data with page manager")

	// Create a version
	txn, _ := txMgr.Begin()
	err = vs.CreateVersion(txn, dn, data)
	if err != nil {
		t.Fatalf("CreateVersion with PageManager failed: %v", err)
	}
	vs.CommitVersion(txn, 100)

	// Verify the version
	v, err := vs.GetVisible(dn, 100)
	if err != nil {
		t.Fatalf("GetVisible failed: %v", err)
	}
	if string(v.Data) != string(data) {
		t.Errorf("data mismatch: got %q, want %q", v.Data, data)
	}

	// Verify page was allocated
	pageID, _ := v.GetLocation()
	if pageID == 0 {
		t.Error("expected non-zero PageID")
	}
}

// TestDeserializeVersionHeaderInvalid tests error handling for invalid header.
func TestDeserializeVersionHeaderInvalid(t *testing.T) {
	_, err := DeserializeVersionHeader([]byte{1, 2, 3}) // Too short
	if err != ErrInvalidVersion {
		t.Errorf("expected ErrInvalidVersion, got %v", err)
	}
}
