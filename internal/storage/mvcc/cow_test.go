// Package mvcc provides Multi-Version Concurrency Control components for ObaDB.
package mvcc

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/oba-ldap/oba/internal/storage"
	"github.com/oba-ldap/oba/internal/storage/tx"
)

// testSetup creates a temporary directory and initializes all required components.
func testSetup(t *testing.T) (string, *storage.PageManager, *storage.WAL, *tx.TxManager, func()) {
	t.Helper()

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "cow_test_*")
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
	walPath := filepath.Join(tmpDir, "wal.oba")
	wal, err := storage.OpenWAL(walPath)
	if err != nil {
		pm.Close()
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to create WAL: %v", err)
	}

	// Create transaction manager
	txm := tx.NewTxManager(wal)

	cleanup := func() {
		wal.Close()
		pm.Close()
		os.RemoveAll(tmpDir)
	}

	return tmpDir, pm, wal, txm, cleanup
}

// TestShadowManager_CreateShadow tests shadow page creation.
func TestShadowManager_CreateShadow(t *testing.T) {
	_, pm, _, _, cleanup := testSetup(t)
	defer cleanup()

	sm := NewShadowManager(pm)
	defer sm.Close()

	// Allocate an original page
	originalID, err := pm.AllocatePage(storage.PageTypeData)
	if err != nil {
		t.Fatalf("failed to allocate page: %v", err)
	}

	// Write some data to the original page
	originalPage, err := pm.ReadPage(originalID)
	if err != nil {
		t.Fatalf("failed to read original page: %v", err)
	}
	copy(originalPage.Data, []byte("original data"))
	if err := pm.WritePage(originalPage); err != nil {
		t.Fatalf("failed to write original page: %v", err)
	}

	// Create a shadow
	txID := uint64(1)
	shadowID, err := sm.CreateShadow(txID, originalID)
	if err != nil {
		t.Fatalf("failed to create shadow: %v", err)
	}

	// Verify shadow was created
	if shadowID == 0 {
		t.Error("shadow ID should not be 0")
	}
	if shadowID == originalID {
		t.Error("shadow ID should be different from original ID")
	}

	// Verify shadow content matches original
	shadowPage, err := pm.ReadPage(shadowID)
	if err != nil {
		t.Fatalf("failed to read shadow page: %v", err)
	}

	if string(shadowPage.Data[:13]) != "original data" {
		t.Errorf("shadow data mismatch: got %q, want %q", shadowPage.Data[:13], "original data")
	}

	// Verify mapping exists
	if !sm.HasShadow(originalID) {
		t.Error("HasShadow should return true")
	}

	gotShadowID, err := sm.GetShadow(originalID)
	if err != nil {
		t.Fatalf("GetShadow failed: %v", err)
	}
	if gotShadowID != shadowID {
		t.Errorf("GetShadow returned wrong ID: got %d, want %d", gotShadowID, shadowID)
	}

	// Verify reverse mapping
	gotOriginalID, err := sm.GetOriginal(shadowID)
	if err != nil {
		t.Fatalf("GetOriginal failed: %v", err)
	}
	if gotOriginalID != originalID {
		t.Errorf("GetOriginal returned wrong ID: got %d, want %d", gotOriginalID, originalID)
	}

	// Verify transaction shadows
	txShadows := sm.GetTransactionShadows(txID)
	if len(txShadows) != 1 {
		t.Errorf("expected 1 transaction shadow, got %d", len(txShadows))
	}
	if txShadows[0] != shadowID {
		t.Errorf("transaction shadow mismatch: got %d, want %d", txShadows[0], shadowID)
	}
}

// TestShadowManager_CreateShadow_SameTransaction tests creating shadow for same page twice.
func TestShadowManager_CreateShadow_SameTransaction(t *testing.T) {
	_, pm, _, _, cleanup := testSetup(t)
	defer cleanup()

	sm := NewShadowManager(pm)
	defer sm.Close()

	// Allocate an original page
	originalID, err := pm.AllocatePage(storage.PageTypeData)
	if err != nil {
		t.Fatalf("failed to allocate page: %v", err)
	}

	// Create first shadow
	txID := uint64(1)
	shadowID1, err := sm.CreateShadow(txID, originalID)
	if err != nil {
		t.Fatalf("failed to create first shadow: %v", err)
	}

	// Create second shadow for same page and transaction - should return existing
	shadowID2, err := sm.CreateShadow(txID, originalID)
	if err != nil {
		t.Fatalf("failed to create second shadow: %v", err)
	}

	if shadowID1 != shadowID2 {
		t.Errorf("expected same shadow ID, got %d and %d", shadowID1, shadowID2)
	}
}

// TestShadowManager_FreeShadow tests freeing shadow pages.
func TestShadowManager_FreeShadow(t *testing.T) {
	_, pm, _, _, cleanup := testSetup(t)
	defer cleanup()

	sm := NewShadowManager(pm)
	defer sm.Close()

	// Allocate and create shadow
	originalID, err := pm.AllocatePage(storage.PageTypeData)
	if err != nil {
		t.Fatalf("failed to allocate page: %v", err)
	}

	txID := uint64(1)
	_, err = sm.CreateShadow(txID, originalID)
	if err != nil {
		t.Fatalf("failed to create shadow: %v", err)
	}

	// Free the shadow
	if err := sm.FreeShadow(originalID); err != nil {
		t.Fatalf("failed to free shadow: %v", err)
	}

	// Verify shadow is gone
	if sm.HasShadow(originalID) {
		t.Error("HasShadow should return false after FreeShadow")
	}

	_, err = sm.GetShadow(originalID)
	if err != ErrShadowPageNotFound {
		t.Errorf("expected ErrShadowPageNotFound, got %v", err)
	}
}

// TestShadowManager_FreeTransactionShadows tests freeing all shadows for a transaction.
func TestShadowManager_FreeTransactionShadows(t *testing.T) {
	_, pm, _, _, cleanup := testSetup(t)
	defer cleanup()

	sm := NewShadowManager(pm)
	defer sm.Close()

	txID := uint64(1)

	// Create multiple shadows
	var originalIDs []storage.PageID
	for i := 0; i < 3; i++ {
		originalID, err := pm.AllocatePage(storage.PageTypeData)
		if err != nil {
			t.Fatalf("failed to allocate page %d: %v", i, err)
		}
		originalIDs = append(originalIDs, originalID)

		_, err = sm.CreateShadow(txID, originalID)
		if err != nil {
			t.Fatalf("failed to create shadow %d: %v", i, err)
		}
	}

	// Verify all shadows exist
	if sm.ShadowCount() != 3 {
		t.Errorf("expected 3 shadows, got %d", sm.ShadowCount())
	}

	// Free all transaction shadows
	if err := sm.FreeTransactionShadows(txID); err != nil {
		t.Fatalf("failed to free transaction shadows: %v", err)
	}

	// Verify all shadows are gone
	if sm.ShadowCount() != 0 {
		t.Errorf("expected 0 shadows after free, got %d", sm.ShadowCount())
	}

	for _, originalID := range originalIDs {
		if sm.HasShadow(originalID) {
			t.Errorf("shadow for page %d should not exist", originalID)
		}
	}
}

// TestCoWManager_GetPage tests reading pages through CoW manager.
func TestCoWManager_GetPage(t *testing.T) {
	_, pm, wal, txm, cleanup := testSetup(t)
	defer cleanup()

	cow := NewCoWManager(pm, txm, wal)
	defer cow.Close()

	// Allocate and write a page
	originalID, err := pm.AllocatePage(storage.PageTypeData)
	if err != nil {
		t.Fatalf("failed to allocate page: %v", err)
	}

	originalPage, err := pm.ReadPage(originalID)
	if err != nil {
		t.Fatalf("failed to read page: %v", err)
	}
	copy(originalPage.Data, []byte("test data"))
	if err := pm.WritePage(originalPage); err != nil {
		t.Fatalf("failed to write page: %v", err)
	}

	// Begin transaction
	txn, err := txm.Begin()
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}

	// Read page through CoW manager
	page, err := cow.GetPage(txn, originalID)
	if err != nil {
		t.Fatalf("GetPage failed: %v", err)
	}

	// Verify content
	if string(page.Data[:9]) != "test data" {
		t.Errorf("page data mismatch: got %q, want %q", page.Data[:9], "test data")
	}

	// Verify page was added to read set
	readSet := txn.GetReadSet()
	if len(readSet) != 1 || readSet[0] != originalID {
		t.Errorf("read set mismatch: got %v, want [%d]", readSet, originalID)
	}
}

// TestCoWManager_ModifyPage tests modifying pages through CoW manager.
func TestCoWManager_ModifyPage(t *testing.T) {
	_, pm, wal, txm, cleanup := testSetup(t)
	defer cleanup()

	cow := NewCoWManager(pm, txm, wal)
	defer cow.Close()

	// Allocate and write a page
	originalID, err := pm.AllocatePage(storage.PageTypeData)
	if err != nil {
		t.Fatalf("failed to allocate page: %v", err)
	}

	originalPage, err := pm.ReadPage(originalID)
	if err != nil {
		t.Fatalf("failed to read page: %v", err)
	}
	copy(originalPage.Data, []byte("original"))
	if err := pm.WritePage(originalPage); err != nil {
		t.Fatalf("failed to write page: %v", err)
	}

	// Begin transaction
	txn, err := txm.Begin()
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}

	// Modify page through CoW manager
	shadowPage, err := cow.ModifyPage(txn, originalID)
	if err != nil {
		t.Fatalf("ModifyPage failed: %v", err)
	}

	// Verify shadow was created
	if cow.ShadowCount() != 1 {
		t.Errorf("expected 1 shadow, got %d", cow.ShadowCount())
	}

	// Verify shadow content matches original
	if string(shadowPage.Data[:8]) != "original" {
		t.Errorf("shadow data mismatch: got %q, want %q", shadowPage.Data[:8], "original")
	}

	// Modify the shadow
	copy(shadowPage.Data, []byte("modified"))
	if err := cow.WriteShadowPage(txn, originalID, shadowPage); err != nil {
		t.Fatalf("WriteShadowPage failed: %v", err)
	}

	// Verify original page is unchanged
	originalPage, err = pm.ReadPage(originalID)
	if err != nil {
		t.Fatalf("failed to read original page: %v", err)
	}
	if string(originalPage.Data[:8]) != "original" {
		t.Errorf("original page should be unchanged: got %q, want %q", originalPage.Data[:8], "original")
	}

	// Verify page was added to write set
	writeSet := txn.GetWriteSet()
	if len(writeSet) != 1 || writeSet[0] != originalID {
		t.Errorf("write set mismatch: got %v, want [%d]", writeSet, originalID)
	}
}

// TestCoWManager_ModifyPage_SameTransaction tests modifying same page twice.
func TestCoWManager_ModifyPage_SameTransaction(t *testing.T) {
	_, pm, wal, txm, cleanup := testSetup(t)
	defer cleanup()

	cow := NewCoWManager(pm, txm, wal)
	defer cow.Close()

	// Allocate a page
	originalID, err := pm.AllocatePage(storage.PageTypeData)
	if err != nil {
		t.Fatalf("failed to allocate page: %v", err)
	}

	// Begin transaction
	txn, err := txm.Begin()
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}

	// First modify
	shadowPage1, err := cow.ModifyPage(txn, originalID)
	if err != nil {
		t.Fatalf("first ModifyPage failed: %v", err)
	}
	shadowID1 := shadowPage1.Header.PageID

	// Second modify - should return same shadow
	shadowPage2, err := cow.ModifyPage(txn, originalID)
	if err != nil {
		t.Fatalf("second ModifyPage failed: %v", err)
	}
	shadowID2 := shadowPage2.Header.PageID

	if shadowID1 != shadowID2 {
		t.Errorf("expected same shadow ID, got %d and %d", shadowID1, shadowID2)
	}

	// Should still have only 1 shadow
	if cow.ShadowCount() != 1 {
		t.Errorf("expected 1 shadow, got %d", cow.ShadowCount())
	}
}

// TestCoWManager_CommitPages tests committing shadow pages.
func TestCoWManager_CommitPages(t *testing.T) {
	_, pm, wal, txm, cleanup := testSetup(t)
	defer cleanup()

	cow := NewCoWManager(pm, txm, wal)
	defer cow.Close()

	// Allocate and write a page
	originalID, err := pm.AllocatePage(storage.PageTypeData)
	if err != nil {
		t.Fatalf("failed to allocate page: %v", err)
	}

	originalPage, err := pm.ReadPage(originalID)
	if err != nil {
		t.Fatalf("failed to read page: %v", err)
	}
	copy(originalPage.Data, []byte("original"))
	if err := pm.WritePage(originalPage); err != nil {
		t.Fatalf("failed to write page: %v", err)
	}

	// Begin transaction
	txn, err := txm.Begin()
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}

	// Modify page
	shadowPage, err := cow.ModifyPage(txn, originalID)
	if err != nil {
		t.Fatalf("ModifyPage failed: %v", err)
	}

	// Modify the shadow
	copy(shadowPage.Data, []byte("modified"))
	if err := cow.WriteShadowPage(txn, originalID, shadowPage); err != nil {
		t.Fatalf("WriteShadowPage failed: %v", err)
	}

	// Commit
	if err := cow.CommitPages(txn); err != nil {
		t.Fatalf("CommitPages failed: %v", err)
	}

	// Verify original page now has modified content
	originalPage, err = pm.ReadPage(originalID)
	if err != nil {
		t.Fatalf("failed to read original page after commit: %v", err)
	}
	if string(originalPage.Data[:8]) != "modified" {
		t.Errorf("original page should have modified content: got %q, want %q", originalPage.Data[:8], "modified")
	}

	// Verify shadow was cleaned up
	if cow.ShadowCount() != 0 {
		t.Errorf("expected 0 shadows after commit, got %d", cow.ShadowCount())
	}
}

// TestCoWManager_RollbackPages tests rolling back shadow pages.
func TestCoWManager_RollbackPages(t *testing.T) {
	_, pm, wal, txm, cleanup := testSetup(t)
	defer cleanup()

	cow := NewCoWManager(pm, txm, wal)
	defer cow.Close()

	// Allocate and write a page
	originalID, err := pm.AllocatePage(storage.PageTypeData)
	if err != nil {
		t.Fatalf("failed to allocate page: %v", err)
	}

	originalPage, err := pm.ReadPage(originalID)
	if err != nil {
		t.Fatalf("failed to read page: %v", err)
	}
	copy(originalPage.Data, []byte("original"))
	if err := pm.WritePage(originalPage); err != nil {
		t.Fatalf("failed to write page: %v", err)
	}

	// Begin transaction
	txn, err := txm.Begin()
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}

	// Modify page
	shadowPage, err := cow.ModifyPage(txn, originalID)
	if err != nil {
		t.Fatalf("ModifyPage failed: %v", err)
	}

	// Modify the shadow
	copy(shadowPage.Data, []byte("modified"))
	if err := cow.WriteShadowPage(txn, originalID, shadowPage); err != nil {
		t.Fatalf("WriteShadowPage failed: %v", err)
	}

	// Verify shadow exists
	if cow.ShadowCount() != 1 {
		t.Errorf("expected 1 shadow before rollback, got %d", cow.ShadowCount())
	}

	// Rollback
	if err := cow.RollbackPages(txn); err != nil {
		t.Fatalf("RollbackPages failed: %v", err)
	}

	// Verify original page is unchanged
	originalPage, err = pm.ReadPage(originalID)
	if err != nil {
		t.Fatalf("failed to read original page after rollback: %v", err)
	}
	if string(originalPage.Data[:8]) != "original" {
		t.Errorf("original page should be unchanged after rollback: got %q, want %q", originalPage.Data[:8], "original")
	}

	// Verify shadow was freed
	if cow.ShadowCount() != 0 {
		t.Errorf("expected 0 shadows after rollback, got %d", cow.ShadowCount())
	}
}

// TestCoWManager_MultiplePages tests modifying multiple pages in one transaction.
func TestCoWManager_MultiplePages(t *testing.T) {
	_, pm, wal, txm, cleanup := testSetup(t)
	defer cleanup()

	cow := NewCoWManager(pm, txm, wal)
	defer cow.Close()

	// Allocate multiple pages
	var originalIDs []storage.PageID
	for i := 0; i < 3; i++ {
		originalID, err := pm.AllocatePage(storage.PageTypeData)
		if err != nil {
			t.Fatalf("failed to allocate page %d: %v", i, err)
		}
		originalIDs = append(originalIDs, originalID)

		// Write initial content
		page, err := pm.ReadPage(originalID)
		if err != nil {
			t.Fatalf("failed to read page %d: %v", i, err)
		}
		copy(page.Data, []byte("original"))
		if err := pm.WritePage(page); err != nil {
			t.Fatalf("failed to write page %d: %v", i, err)
		}
	}

	// Begin transaction
	txn, err := txm.Begin()
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}

	// Modify all pages
	for i, originalID := range originalIDs {
		shadowPage, err := cow.ModifyPage(txn, originalID)
		if err != nil {
			t.Fatalf("ModifyPage %d failed: %v", i, err)
		}
		copy(shadowPage.Data, []byte("modified"))
		if err := cow.WriteShadowPage(txn, originalID, shadowPage); err != nil {
			t.Fatalf("WriteShadowPage %d failed: %v", i, err)
		}
	}

	// Verify all shadows exist
	if cow.ShadowCount() != 3 {
		t.Errorf("expected 3 shadows, got %d", cow.ShadowCount())
	}

	// Commit
	if err := cow.CommitPages(txn); err != nil {
		t.Fatalf("CommitPages failed: %v", err)
	}

	// Verify all pages have modified content
	for i, originalID := range originalIDs {
		page, err := pm.ReadPage(originalID)
		if err != nil {
			t.Fatalf("failed to read page %d after commit: %v", i, err)
		}
		if string(page.Data[:8]) != "modified" {
			t.Errorf("page %d should have modified content: got %q, want %q", i, page.Data[:8], "modified")
		}
	}

	// Verify all shadows were cleaned up
	if cow.ShadowCount() != 0 {
		t.Errorf("expected 0 shadows after commit, got %d", cow.ShadowCount())
	}
}

// TestCoWManager_ReadAfterModify tests reading a page after modifying it.
func TestCoWManager_ReadAfterModify(t *testing.T) {
	_, pm, wal, txm, cleanup := testSetup(t)
	defer cleanup()

	cow := NewCoWManager(pm, txm, wal)
	defer cow.Close()

	// Allocate and write a page
	originalID, err := pm.AllocatePage(storage.PageTypeData)
	if err != nil {
		t.Fatalf("failed to allocate page: %v", err)
	}

	originalPage, err := pm.ReadPage(originalID)
	if err != nil {
		t.Fatalf("failed to read page: %v", err)
	}
	copy(originalPage.Data, []byte("original"))
	if err := pm.WritePage(originalPage); err != nil {
		t.Fatalf("failed to write page: %v", err)
	}

	// Begin transaction
	txn, err := txm.Begin()
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}

	// Modify page
	shadowPage, err := cow.ModifyPage(txn, originalID)
	if err != nil {
		t.Fatalf("ModifyPage failed: %v", err)
	}
	copy(shadowPage.Data, []byte("modified"))
	if err := cow.WriteShadowPage(txn, originalID, shadowPage); err != nil {
		t.Fatalf("WriteShadowPage failed: %v", err)
	}

	// Read page through same transaction - should see modified content
	readPage, err := cow.GetPage(txn, originalID)
	if err != nil {
		t.Fatalf("GetPage failed: %v", err)
	}

	if string(readPage.Data[:8]) != "modified" {
		t.Errorf("read should see modified content: got %q, want %q", readPage.Data[:8], "modified")
	}
}

// TestCoWManager_OriginalUnchangedDuringTransaction tests that original pages
// are never modified during a transaction.
func TestCoWManager_OriginalUnchangedDuringTransaction(t *testing.T) {
	_, pm, wal, txm, cleanup := testSetup(t)
	defer cleanup()

	cow := NewCoWManager(pm, txm, wal)
	defer cow.Close()

	// Allocate and write a page
	originalID, err := pm.AllocatePage(storage.PageTypeData)
	if err != nil {
		t.Fatalf("failed to allocate page: %v", err)
	}

	originalPage, err := pm.ReadPage(originalID)
	if err != nil {
		t.Fatalf("failed to read page: %v", err)
	}
	copy(originalPage.Data, []byte("original"))
	if err := pm.WritePage(originalPage); err != nil {
		t.Fatalf("failed to write page: %v", err)
	}

	// Begin transaction
	txn, err := txm.Begin()
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}

	// Modify page multiple times
	for i := 0; i < 5; i++ {
		shadowPage, err := cow.ModifyPage(txn, originalID)
		if err != nil {
			t.Fatalf("ModifyPage %d failed: %v", i, err)
		}
		copy(shadowPage.Data, []byte("modified"))
		if err := cow.WriteShadowPage(txn, originalID, shadowPage); err != nil {
			t.Fatalf("WriteShadowPage %d failed: %v", i, err)
		}

		// Verify original is still unchanged (read directly from page manager)
		directPage, err := pm.ReadPage(originalID)
		if err != nil {
			t.Fatalf("failed to read original page directly: %v", err)
		}
		if string(directPage.Data[:8]) != "original" {
			t.Errorf("original page should be unchanged during transaction: got %q, want %q", directPage.Data[:8], "original")
		}
	}
}

// TestCoWManager_ErrorCases tests various error conditions.
func TestCoWManager_ErrorCases(t *testing.T) {
	_, pm, wal, txm, cleanup := testSetup(t)
	defer cleanup()

	cow := NewCoWManager(pm, txm, wal)

	// Test with nil transaction
	_, err := cow.GetPage(nil, 1)
	if err != ErrTransactionNil {
		t.Errorf("expected ErrTransactionNil, got %v", err)
	}

	_, err = cow.ModifyPage(nil, 1)
	if err != ErrTransactionNil {
		t.Errorf("expected ErrTransactionNil, got %v", err)
	}

	// Test with closed manager
	cow.Close()

	txn, err := txm.Begin()
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}

	_, err = cow.GetPage(txn, 1)
	if err != ErrCoWManagerClosed {
		t.Errorf("expected ErrCoWManagerClosed, got %v", err)
	}

	_, err = cow.ModifyPage(txn, 1)
	if err != ErrCoWManagerClosed {
		t.Errorf("expected ErrCoWManagerClosed, got %v", err)
	}
}

// TestShadowManager_ErrorCases tests shadow manager error conditions.
func TestShadowManager_ErrorCases(t *testing.T) {
	_, pm, _, _, cleanup := testSetup(t)
	defer cleanup()

	sm := NewShadowManager(pm)

	// Test GetShadow for non-existent page
	_, err := sm.GetShadow(999)
	if err != ErrShadowPageNotFound {
		t.Errorf("expected ErrShadowPageNotFound, got %v", err)
	}

	// Test GetOriginal for non-existent shadow
	_, err = sm.GetOriginal(999)
	if err != ErrShadowPageNotFound {
		t.Errorf("expected ErrShadowPageNotFound, got %v", err)
	}

	// Test FreeShadow for non-existent page
	err = sm.FreeShadow(999)
	if err != ErrShadowPageNotFound {
		t.Errorf("expected ErrShadowPageNotFound, got %v", err)
	}

	// Test with closed manager
	sm.Close()

	_, err = sm.CreateShadow(1, 1)
	if err != ErrShadowManagerClosed {
		t.Errorf("expected ErrShadowManagerClosed, got %v", err)
	}

	_, err = sm.GetShadow(1)
	if err != ErrShadowManagerClosed {
		t.Errorf("expected ErrShadowManagerClosed, got %v", err)
	}
}
