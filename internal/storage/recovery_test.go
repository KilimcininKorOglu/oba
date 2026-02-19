// Package storage provides the core storage engine components for ObaDB.
package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestTxStateString tests TxState string representation.
func TestTxStateString(t *testing.T) {
	tests := []struct {
		state    TxState
		expected string
	}{
		{TxStateActive, "Active"},
		{TxStateCommitted, "Committed"},
		{TxStateAborted, "Aborted"},
		{TxState(99), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.state.String(); got != tt.expected {
				t.Errorf("TxState.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestNewRecovery tests creating a new Recovery instance.
func TestNewRecovery(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "test.wal")
	dataPath := filepath.Join(tmpDir, "test.db")

	wal, err := OpenWAL(walPath)
	if err != nil {
		t.Fatalf("Failed to open WAL: %v", err)
	}
	defer wal.Close()

	pm, err := OpenPageManager(dataPath, DefaultOptions())
	if err != nil {
		t.Fatalf("Failed to open PageManager: %v", err)
	}
	defer pm.Close()

	recovery := NewRecovery(wal, pm)
	if recovery == nil {
		t.Fatal("NewRecovery returned nil")
	}

	if recovery.wal != wal {
		t.Error("WAL not set correctly")
	}
	if recovery.pageManager != pm {
		t.Error("PageManager not set correctly")
	}
	if recovery.activeTx == nil {
		t.Error("activeTx map not initialized")
	}
	if recovery.dirtyPages == nil {
		t.Error("dirtyPages map not initialized")
	}
}

// TestRecoveryWithNoWAL tests recovery fails without WAL.
func TestRecoveryWithNoWAL(t *testing.T) {
	tmpDir := t.TempDir()
	dataPath := filepath.Join(tmpDir, "test.db")

	pm, err := OpenPageManager(dataPath, DefaultOptions())
	if err != nil {
		t.Fatalf("Failed to open PageManager: %v", err)
	}
	defer pm.Close()

	recovery := NewRecovery(nil, pm)
	err = recovery.Recover()
	if err != ErrNoWAL {
		t.Errorf("Expected ErrNoWAL, got %v", err)
	}
}

// TestRecoveryWithNoPageManager tests recovery fails without PageManager.
func TestRecoveryWithNoPageManager(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "test.wal")

	wal, err := OpenWAL(walPath)
	if err != nil {
		t.Fatalf("Failed to open WAL: %v", err)
	}
	defer wal.Close()

	recovery := NewRecovery(wal, nil)
	err = recovery.Recover()
	if err != ErrNoPageManager {
		t.Errorf("Expected ErrNoPageManager, got %v", err)
	}
}

// TestRecoveryEmptyWAL tests recovery with an empty WAL.
func TestRecoveryEmptyWAL(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "test.wal")
	dataPath := filepath.Join(tmpDir, "test.db")

	wal, err := OpenWAL(walPath)
	if err != nil {
		t.Fatalf("Failed to open WAL: %v", err)
	}
	defer wal.Close()

	pm, err := OpenPageManager(dataPath, DefaultOptions())
	if err != nil {
		t.Fatalf("Failed to open PageManager: %v", err)
	}
	defer pm.Close()

	recovery := NewRecovery(wal, pm)
	err = recovery.Recover()
	if err != nil {
		t.Errorf("Recovery failed on empty WAL: %v", err)
	}
}

// TestRecoveryCommittedTransaction tests recovery with a committed transaction.
func TestRecoveryCommittedTransaction(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "test.wal")
	dataPath := filepath.Join(tmpDir, "test.db")

	// Create WAL and write committed transaction
	wal, err := OpenWAL(walPath)
	if err != nil {
		t.Fatalf("Failed to open WAL: %v", err)
	}

	pm, err := OpenPageManager(dataPath, DefaultOptions())
	if err != nil {
		wal.Close()
		t.Fatalf("Failed to open PageManager: %v", err)
	}

	// Allocate a page for testing
	pageID, err := pm.AllocatePage(PageTypeData)
	if err != nil {
		wal.Close()
		pm.Close()
		t.Fatalf("Failed to allocate page: %v", err)
	}

	// Write BEGIN record
	beginRecord := NewWALRecord(0, 1, WALBegin)
	_, err = wal.Append(beginRecord)
	if err != nil {
		wal.Close()
		pm.Close()
		t.Fatalf("Failed to append BEGIN: %v", err)
	}

	// Write UPDATE record
	oldData := []byte("old data")
	newData := []byte("new data")
	updateRecord := NewWALUpdateRecord(0, 1, pageID, 0, oldData, newData)
	_, err = wal.Append(updateRecord)
	if err != nil {
		wal.Close()
		pm.Close()
		t.Fatalf("Failed to append UPDATE: %v", err)
	}

	// Write COMMIT record
	commitRecord := NewWALRecord(0, 1, WALCommit)
	_, err = wal.Append(commitRecord)
	if err != nil {
		wal.Close()
		pm.Close()
		t.Fatalf("Failed to append COMMIT: %v", err)
	}

	wal.Sync()
	wal.Close()
	pm.Close()

	// Reopen and recover
	wal, err = OpenWAL(walPath)
	if err != nil {
		t.Fatalf("Failed to reopen WAL: %v", err)
	}
	defer wal.Close()

	pm, err = OpenPageManager(dataPath, DefaultOptions())
	if err != nil {
		t.Fatalf("Failed to reopen PageManager: %v", err)
	}
	defer pm.Close()

	recovery := NewRecovery(wal, pm)
	err = recovery.Recover()
	if err != nil {
		t.Errorf("Recovery failed: %v", err)
	}

	// Verify transaction state
	activeTx := recovery.GetActiveTx()
	if len(activeTx) != 1 {
		t.Errorf("Expected 1 transaction, got %d", len(activeTx))
	}

	txInfo, exists := activeTx[1]
	if !exists {
		t.Error("Transaction 1 not found")
	} else if txInfo.State != TxStateCommitted {
		t.Errorf("Expected TxStateCommitted, got %v", txInfo.State)
	}
}

// TestRecoveryUncommittedTransaction tests recovery rolls back uncommitted transactions.
func TestRecoveryUncommittedTransaction(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "test.wal")
	dataPath := filepath.Join(tmpDir, "test.db")

	// Create WAL and write uncommitted transaction
	wal, err := OpenWAL(walPath)
	if err != nil {
		t.Fatalf("Failed to open WAL: %v", err)
	}

	pm, err := OpenPageManager(dataPath, DefaultOptions())
	if err != nil {
		wal.Close()
		t.Fatalf("Failed to open PageManager: %v", err)
	}

	// Allocate a page for testing
	pageID, err := pm.AllocatePage(PageTypeData)
	if err != nil {
		wal.Close()
		pm.Close()
		t.Fatalf("Failed to allocate page: %v", err)
	}

	// Write initial data to page
	page, err := pm.ReadPage(pageID)
	if err != nil {
		wal.Close()
		pm.Close()
		t.Fatalf("Failed to read page: %v", err)
	}
	copy(page.Data[0:8], []byte("original"))
	pm.WritePage(page)
	pm.Sync()

	// Write BEGIN record
	beginRecord := NewWALRecord(0, 1, WALBegin)
	_, err = wal.Append(beginRecord)
	if err != nil {
		wal.Close()
		pm.Close()
		t.Fatalf("Failed to append BEGIN: %v", err)
	}

	// Write UPDATE record (no COMMIT - simulating crash)
	oldData := []byte("original")
	newData := []byte("modified")
	updateRecord := NewWALUpdateRecord(0, 1, pageID, 0, oldData, newData)
	_, err = wal.Append(updateRecord)
	if err != nil {
		wal.Close()
		pm.Close()
		t.Fatalf("Failed to append UPDATE: %v", err)
	}

	// Apply the change to simulate partial write before crash
	copy(page.Data[0:8], newData)
	pm.WritePage(page)

	wal.Sync()
	wal.Close()
	pm.Close()

	// Reopen and recover
	wal, err = OpenWAL(walPath)
	if err != nil {
		t.Fatalf("Failed to reopen WAL: %v", err)
	}
	defer wal.Close()

	pm, err = OpenPageManager(dataPath, DefaultOptions())
	if err != nil {
		t.Fatalf("Failed to reopen PageManager: %v", err)
	}
	defer pm.Close()

	recovery := NewRecovery(wal, pm)
	err = recovery.Recover()
	if err != nil {
		t.Errorf("Recovery failed: %v", err)
	}

	// Verify transaction was rolled back
	activeTx := recovery.GetActiveTx()
	txInfo, exists := activeTx[1]
	if !exists {
		t.Error("Transaction 1 not found")
	} else if txInfo.State != TxStateAborted {
		t.Errorf("Expected TxStateAborted, got %v", txInfo.State)
	}

	// Verify page data was restored
	page, err = pm.ReadPage(pageID)
	if err != nil {
		t.Fatalf("Failed to read page after recovery: %v", err)
	}
	if string(page.Data[0:8]) != "original" {
		t.Errorf("Page data not restored, got %s", string(page.Data[0:8]))
	}
}

// TestRecoveryMultipleTransactions tests recovery with multiple transactions.
func TestRecoveryMultipleTransactions(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "test.wal")
	dataPath := filepath.Join(tmpDir, "test.db")

	wal, err := OpenWAL(walPath)
	if err != nil {
		t.Fatalf("Failed to open WAL: %v", err)
	}

	pm, err := OpenPageManager(dataPath, DefaultOptions())
	if err != nil {
		wal.Close()
		t.Fatalf("Failed to open PageManager: %v", err)
	}

	// Transaction 1: Committed
	beginRecord1 := NewWALRecord(0, 1, WALBegin)
	wal.Append(beginRecord1)
	commitRecord1 := NewWALRecord(0, 1, WALCommit)
	wal.Append(commitRecord1)

	// Transaction 2: Aborted
	beginRecord2 := NewWALRecord(0, 2, WALBegin)
	wal.Append(beginRecord2)
	abortRecord2 := NewWALRecord(0, 2, WALAbort)
	wal.Append(abortRecord2)

	// Transaction 3: Active (uncommitted)
	beginRecord3 := NewWALRecord(0, 3, WALBegin)
	wal.Append(beginRecord3)

	wal.Sync()
	wal.Close()
	pm.Close()

	// Reopen and recover
	wal, err = OpenWAL(walPath)
	if err != nil {
		t.Fatalf("Failed to reopen WAL: %v", err)
	}
	defer wal.Close()

	pm, err = OpenPageManager(dataPath, DefaultOptions())
	if err != nil {
		t.Fatalf("Failed to reopen PageManager: %v", err)
	}
	defer pm.Close()

	recovery := NewRecovery(wal, pm)
	err = recovery.Recover()
	if err != nil {
		t.Errorf("Recovery failed: %v", err)
	}

	activeTx := recovery.GetActiveTx()
	if len(activeTx) != 3 {
		t.Errorf("Expected 3 transactions, got %d", len(activeTx))
	}

	// Verify transaction states
	if tx1, ok := activeTx[1]; ok {
		if tx1.State != TxStateCommitted {
			t.Errorf("Tx1: expected Committed, got %v", tx1.State)
		}
	} else {
		t.Error("Transaction 1 not found")
	}

	if tx2, ok := activeTx[2]; ok {
		if tx2.State != TxStateAborted {
			t.Errorf("Tx2: expected Aborted, got %v", tx2.State)
		}
	} else {
		t.Error("Transaction 2 not found")
	}

	if tx3, ok := activeTx[3]; ok {
		if tx3.State != TxStateAborted {
			t.Errorf("Tx3: expected Aborted (rolled back), got %v", tx3.State)
		}
	} else {
		t.Error("Transaction 3 not found")
	}
}

// TestRecoveryInProgress tests that concurrent recovery is prevented.
func TestRecoveryInProgress(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "test.wal")
	dataPath := filepath.Join(tmpDir, "test.db")

	wal, err := OpenWAL(walPath)
	if err != nil {
		t.Fatalf("Failed to open WAL: %v", err)
	}
	defer wal.Close()

	pm, err := OpenPageManager(dataPath, DefaultOptions())
	if err != nil {
		t.Fatalf("Failed to open PageManager: %v", err)
	}
	defer pm.Close()

	recovery := NewRecovery(wal, pm)

	// Manually set inProgress
	recovery.mu.Lock()
	recovery.inProgress = true
	recovery.mu.Unlock()

	err = recovery.Recover()
	if err != ErrRecoveryInProgress {
		t.Errorf("Expected ErrRecoveryInProgress, got %v", err)
	}

	if !recovery.IsInProgress() {
		t.Error("IsInProgress should return true")
	}
}

// TestRecoverySetBufferPool tests setting the buffer pool.
func TestRecoverySetBufferPool(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "test.wal")
	dataPath := filepath.Join(tmpDir, "test.db")

	wal, err := OpenWAL(walPath)
	if err != nil {
		t.Fatalf("Failed to open WAL: %v", err)
	}
	defer wal.Close()

	pm, err := OpenPageManager(dataPath, DefaultOptions())
	if err != nil {
		t.Fatalf("Failed to open PageManager: %v", err)
	}
	defer pm.Close()

	recovery := NewRecovery(wal, pm)
	bp := NewBufferPool(16, PageSize)

	recovery.SetBufferPool(bp)

	if recovery.bufferPool != bp {
		t.Error("BufferPool not set correctly")
	}
}

// TestRecoveryGetters tests the getter methods.
func TestRecoveryGetters(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "test.wal")
	dataPath := filepath.Join(tmpDir, "test.db")

	wal, err := OpenWAL(walPath)
	if err != nil {
		t.Fatalf("Failed to open WAL: %v", err)
	}
	defer wal.Close()

	pm, err := OpenPageManager(dataPath, DefaultOptions())
	if err != nil {
		t.Fatalf("Failed to open PageManager: %v", err)
	}
	defer pm.Close()

	recovery := NewRecovery(wal, pm)
	recovery.Recover()

	// Test getters return copies
	activeTx := recovery.GetActiveTx()
	if activeTx == nil {
		t.Error("GetActiveTx returned nil")
	}

	dirtyPages := recovery.GetDirtyPages()
	if dirtyPages == nil {
		t.Error("GetDirtyPages returned nil")
	}

	_ = recovery.GetCheckpointLSN()
	_ = recovery.GetRedoLSN()
}

// TestCheckpointDataSerialize tests CheckpointData serialization.
func TestCheckpointDataSerialize(t *testing.T) {
	data := &CheckpointData{
		Timestamp:    time.Now(),
		LastLSN:      100,
		ActiveTxIDs:  []uint64{1, 2, 3},
		DirtyPageIDs: []PageID{10, 20, 30},
	}

	buf := data.Serialize()
	if len(buf) == 0 {
		t.Error("Serialize returned empty buffer")
	}

	// Deserialize and verify
	data2 := &CheckpointData{}
	err := data2.Deserialize(buf)
	if err != nil {
		t.Fatalf("Deserialize failed: %v", err)
	}

	if data2.LastLSN != data.LastLSN {
		t.Errorf("LastLSN mismatch: got %d, want %d", data2.LastLSN, data.LastLSN)
	}

	if len(data2.ActiveTxIDs) != len(data.ActiveTxIDs) {
		t.Errorf("ActiveTxIDs length mismatch: got %d, want %d", len(data2.ActiveTxIDs), len(data.ActiveTxIDs))
	}

	if len(data2.DirtyPageIDs) != len(data.DirtyPageIDs) {
		t.Errorf("DirtyPageIDs length mismatch: got %d, want %d", len(data2.DirtyPageIDs), len(data.DirtyPageIDs))
	}
}

// TestCheckpointDataDeserializeInvalid tests CheckpointData deserialization with invalid data.
func TestCheckpointDataDeserializeInvalid(t *testing.T) {
	data := &CheckpointData{}

	// Too short buffer
	err := data.Deserialize([]byte{1, 2, 3})
	if err != ErrInvalidCheckpoint {
		t.Errorf("Expected ErrInvalidCheckpoint, got %v", err)
	}
}

// TestNewCheckpointManager tests creating a new CheckpointManager.
func TestNewCheckpointManager(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "test.wal")
	dataPath := filepath.Join(tmpDir, "test.db")

	wal, err := OpenWAL(walPath)
	if err != nil {
		t.Fatalf("Failed to open WAL: %v", err)
	}
	defer wal.Close()

	pm, err := OpenPageManager(dataPath, DefaultOptions())
	if err != nil {
		t.Fatalf("Failed to open PageManager: %v", err)
	}
	defer pm.Close()

	cm := NewCheckpointManager(wal, pm)
	if cm == nil {
		t.Fatal("NewCheckpointManager returned nil")
	}

	if cm.wal != wal {
		t.Error("WAL not set correctly")
	}
	if cm.pageManager != pm {
		t.Error("PageManager not set correctly")
	}
	if cm.checkpointInterval != 5*time.Minute {
		t.Errorf("Default interval wrong: got %v", cm.checkpointInterval)
	}
}

// TestCheckpointManagerSetters tests CheckpointManager setter methods.
func TestCheckpointManagerSetters(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "test.wal")
	dataPath := filepath.Join(tmpDir, "test.db")

	wal, err := OpenWAL(walPath)
	if err != nil {
		t.Fatalf("Failed to open WAL: %v", err)
	}
	defer wal.Close()

	pm, err := OpenPageManager(dataPath, DefaultOptions())
	if err != nil {
		t.Fatalf("Failed to open PageManager: %v", err)
	}
	defer pm.Close()

	cm := NewCheckpointManager(wal, pm)

	// Test SetBufferPool
	bp := NewBufferPool(16, PageSize)
	cm.SetBufferPool(bp)
	if cm.bufferPool != bp {
		t.Error("BufferPool not set correctly")
	}

	// Test SetCheckpointInterval
	cm.SetCheckpointInterval(10 * time.Minute)
	if cm.GetCheckpointInterval() != 10*time.Minute {
		t.Error("CheckpointInterval not set correctly")
	}

	// Test SetActiveTxCallback
	cm.SetActiveTxCallback(func() []uint64 {
		return []uint64{1, 2}
	})
	if cm.getActiveTxIDs == nil {
		t.Error("ActiveTxCallback not set")
	}
}

// TestCheckpoint tests the checkpoint operation.
func TestCheckpoint(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "test.wal")
	dataPath := filepath.Join(tmpDir, "test.db")

	wal, err := OpenWAL(walPath)
	if err != nil {
		t.Fatalf("Failed to open WAL: %v", err)
	}
	defer wal.Close()

	pm, err := OpenPageManager(dataPath, DefaultOptions())
	if err != nil {
		t.Fatalf("Failed to open PageManager: %v", err)
	}
	defer pm.Close()

	cm := NewCheckpointManager(wal, pm)
	bp := NewBufferPool(16, PageSize)
	cm.SetBufferPool(bp)

	// Perform checkpoint
	err = cm.Checkpoint()
	if err != nil {
		t.Errorf("Checkpoint failed: %v", err)
	}

	// Verify checkpoint was recorded
	if cm.LastCheckpointLSN() == 0 {
		t.Error("LastCheckpointLSN should be non-zero after checkpoint")
	}

	if cm.LastCheckpointTime().IsZero() {
		t.Error("LastCheckpointTime should be set after checkpoint")
	}
}

// TestCheckpointInProgress tests that concurrent checkpoints are prevented.
func TestCheckpointInProgress(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "test.wal")
	dataPath := filepath.Join(tmpDir, "test.db")

	wal, err := OpenWAL(walPath)
	if err != nil {
		t.Fatalf("Failed to open WAL: %v", err)
	}
	defer wal.Close()

	pm, err := OpenPageManager(dataPath, DefaultOptions())
	if err != nil {
		t.Fatalf("Failed to open PageManager: %v", err)
	}
	defer pm.Close()

	cm := NewCheckpointManager(wal, pm)

	// Manually set inProgress
	cm.mu.Lock()
	cm.inProgress = true
	cm.mu.Unlock()

	err = cm.Checkpoint()
	if err != ErrCheckpointInProgress {
		t.Errorf("Expected ErrCheckpointInProgress, got %v", err)
	}

	if !cm.IsInProgress() {
		t.Error("IsInProgress should return true")
	}
}

// TestShouldCheckpoint tests the ShouldCheckpoint method.
func TestShouldCheckpoint(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "test.wal")
	dataPath := filepath.Join(tmpDir, "test.db")

	wal, err := OpenWAL(walPath)
	if err != nil {
		t.Fatalf("Failed to open WAL: %v", err)
	}
	defer wal.Close()

	pm, err := OpenPageManager(dataPath, DefaultOptions())
	if err != nil {
		t.Fatalf("Failed to open PageManager: %v", err)
	}
	defer pm.Close()

	cm := NewCheckpointManager(wal, pm)

	// Should checkpoint when no previous checkpoint
	if !cm.ShouldCheckpoint() {
		t.Error("ShouldCheckpoint should return true when no previous checkpoint")
	}

	// Perform checkpoint
	cm.Checkpoint()

	// Should not checkpoint immediately after
	cm.SetCheckpointInterval(1 * time.Hour)
	if cm.ShouldCheckpoint() {
		t.Error("ShouldCheckpoint should return false immediately after checkpoint")
	}
}

// TestTruncateWAL tests WAL truncation after checkpoint.
func TestTruncateWAL(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "test.wal")
	dataPath := filepath.Join(tmpDir, "test.db")

	wal, err := OpenWAL(walPath)
	if err != nil {
		t.Fatalf("Failed to open WAL: %v", err)
	}
	defer wal.Close()

	pm, err := OpenPageManager(dataPath, DefaultOptions())
	if err != nil {
		t.Fatalf("Failed to open PageManager: %v", err)
	}
	defer pm.Close()

	cm := NewCheckpointManager(wal, pm)

	// TruncateWAL should fail without checkpoint
	err = cm.TruncateWAL()
	if err != ErrNoActiveCheckpoint {
		t.Errorf("Expected ErrNoActiveCheckpoint, got %v", err)
	}

	// Perform checkpoint
	cm.Checkpoint()

	// TruncateWAL should succeed after checkpoint
	err = cm.TruncateWAL()
	if err != nil {
		t.Errorf("TruncateWAL failed: %v", err)
	}
}

// TestParseCheckpointRecord tests parsing checkpoint records.
func TestParseCheckpointRecord(t *testing.T) {
	// Create a valid checkpoint record
	data := &CheckpointData{
		Timestamp:    time.Now(),
		LastLSN:      50,
		ActiveTxIDs:  []uint64{1, 2},
		DirtyPageIDs: []PageID{10, 20},
	}

	record := NewWALRecord(1, 0, WALCheckpoint)
	record.NewData = data.Serialize()

	// Parse the record
	parsed, err := ParseCheckpointRecord(record)
	if err != nil {
		t.Fatalf("ParseCheckpointRecord failed: %v", err)
	}

	if parsed.LastLSN != data.LastLSN {
		t.Errorf("LastLSN mismatch: got %d, want %d", parsed.LastLSN, data.LastLSN)
	}
}

// TestParseCheckpointRecordInvalid tests parsing invalid checkpoint records.
func TestParseCheckpointRecordInvalid(t *testing.T) {
	// Wrong record type
	record := NewWALRecord(1, 0, WALBegin)
	_, err := ParseCheckpointRecord(record)
	if err != ErrInvalidCheckpoint {
		t.Errorf("Expected ErrInvalidCheckpoint for wrong type, got %v", err)
	}

	// Empty data
	record = NewWALRecord(1, 0, WALCheckpoint)
	_, err = ParseCheckpointRecord(record)
	if err != ErrInvalidCheckpoint {
		t.Errorf("Expected ErrInvalidCheckpoint for empty data, got %v", err)
	}
}

// TestRecoveryWithCheckpoint tests recovery using checkpoint.
func TestRecoveryWithCheckpoint(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "test.wal")
	dataPath := filepath.Join(tmpDir, "test.db")

	// Create WAL with checkpoint
	wal, err := OpenWAL(walPath)
	if err != nil {
		t.Fatalf("Failed to open WAL: %v", err)
	}

	pm, err := OpenPageManager(dataPath, DefaultOptions())
	if err != nil {
		wal.Close()
		t.Fatalf("Failed to open PageManager: %v", err)
	}

	// Write some records before checkpoint
	beginRecord := NewWALRecord(0, 1, WALBegin)
	wal.Append(beginRecord)
	commitRecord := NewWALRecord(0, 1, WALCommit)
	wal.Append(commitRecord)

	// Create checkpoint
	cm := NewCheckpointManager(wal, pm)
	cm.Checkpoint()

	// Write more records after checkpoint
	beginRecord2 := NewWALRecord(0, 2, WALBegin)
	wal.Append(beginRecord2)
	commitRecord2 := NewWALRecord(0, 2, WALCommit)
	wal.Append(commitRecord2)

	wal.Sync()
	wal.Close()
	pm.Close()

	// Reopen and recover
	wal, err = OpenWAL(walPath)
	if err != nil {
		t.Fatalf("Failed to reopen WAL: %v", err)
	}
	defer wal.Close()

	pm, err = OpenPageManager(dataPath, DefaultOptions())
	if err != nil {
		t.Fatalf("Failed to reopen PageManager: %v", err)
	}
	defer pm.Close()

	recovery := NewRecovery(wal, pm)
	err = recovery.Recover()
	if err != nil {
		t.Errorf("Recovery failed: %v", err)
	}

	// Verify checkpoint was found
	if recovery.GetCheckpointLSN() == 0 {
		t.Error("Checkpoint LSN should be non-zero")
	}
}

// TestRecoveryDirtyPages tests dirty page tracking during recovery.
func TestRecoveryDirtyPages(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "test.wal")
	dataPath := filepath.Join(tmpDir, "test.db")

	wal, err := OpenWAL(walPath)
	if err != nil {
		t.Fatalf("Failed to open WAL: %v", err)
	}

	pm, err := OpenPageManager(dataPath, DefaultOptions())
	if err != nil {
		wal.Close()
		t.Fatalf("Failed to open PageManager: %v", err)
	}

	// Allocate pages
	pageID1, _ := pm.AllocatePage(PageTypeData)
	pageID2, _ := pm.AllocatePage(PageTypeData)

	// Write transaction with updates to multiple pages
	beginRecord := NewWALRecord(0, 1, WALBegin)
	wal.Append(beginRecord)

	updateRecord1 := NewWALUpdateRecord(0, 1, pageID1, 0, []byte("old1"), []byte("new1"))
	wal.Append(updateRecord1)

	updateRecord2 := NewWALUpdateRecord(0, 1, pageID2, 0, []byte("old2"), []byte("new2"))
	wal.Append(updateRecord2)

	commitRecord := NewWALRecord(0, 1, WALCommit)
	wal.Append(commitRecord)

	wal.Sync()
	wal.Close()
	pm.Close()

	// Reopen and recover
	wal, err = OpenWAL(walPath)
	if err != nil {
		t.Fatalf("Failed to reopen WAL: %v", err)
	}
	defer wal.Close()

	pm, err = OpenPageManager(dataPath, DefaultOptions())
	if err != nil {
		t.Fatalf("Failed to reopen PageManager: %v", err)
	}
	defer pm.Close()

	recovery := NewRecovery(wal, pm)
	recovery.Recover()

	// Verify dirty pages were tracked
	dirtyPages := recovery.GetDirtyPages()
	if len(dirtyPages) != 2 {
		t.Errorf("Expected 2 dirty pages, got %d", len(dirtyPages))
	}

	if _, exists := dirtyPages[pageID1]; !exists {
		t.Error("Page 1 should be in dirty pages")
	}
	if _, exists := dirtyPages[pageID2]; !exists {
		t.Error("Page 2 should be in dirty pages")
	}
}

// TestCheckpointWithActiveTx tests checkpoint with active transaction callback.
func TestCheckpointWithActiveTx(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "test.wal")
	dataPath := filepath.Join(tmpDir, "test.db")

	wal, err := OpenWAL(walPath)
	if err != nil {
		t.Fatalf("Failed to open WAL: %v", err)
	}
	defer wal.Close()

	pm, err := OpenPageManager(dataPath, DefaultOptions())
	if err != nil {
		t.Fatalf("Failed to open PageManager: %v", err)
	}
	defer pm.Close()

	cm := NewCheckpointManager(wal, pm)

	// Set callback to return active transactions
	cm.SetActiveTxCallback(func() []uint64 {
		return []uint64{100, 200, 300}
	})

	err = cm.Checkpoint()
	if err != nil {
		t.Errorf("Checkpoint failed: %v", err)
	}
}

// TestRecoveryCleanup verifies cleanup after recovery.
func TestRecoveryCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "test.wal")
	dataPath := filepath.Join(tmpDir, "test.db")

	wal, err := OpenWAL(walPath)
	if err != nil {
		t.Fatalf("Failed to open WAL: %v", err)
	}
	defer wal.Close()

	pm, err := OpenPageManager(dataPath, DefaultOptions())
	if err != nil {
		t.Fatalf("Failed to open PageManager: %v", err)
	}
	defer pm.Close()

	recovery := NewRecovery(wal, pm)

	// First recovery
	err = recovery.Recover()
	if err != nil {
		t.Errorf("First recovery failed: %v", err)
	}

	// Second recovery should also work (state should be reset)
	err = recovery.Recover()
	if err != nil {
		t.Errorf("Second recovery failed: %v", err)
	}
}

// Ensure test files are cleaned up
func TestMain(m *testing.M) {
	code := m.Run()
	os.Exit(code)
}
