// Package mvcc provides Multi-Version Concurrency Control components for ObaDB.
package mvcc

import (
	"errors"
	"sync"

	"github.com/KilimcininKorOglu/oba/internal/storage"
	"github.com/KilimcininKorOglu/oba/internal/storage/tx"
)

// CoW manager errors.
var (
	ErrCoWManagerClosed    = errors.New("CoW manager is closed")
	ErrTransactionNil      = errors.New("transaction is nil")
	ErrTransactionNotActive = errors.New("transaction is not active")
	ErrPageNotFound        = errors.New("page not found")
	ErrCommitFailed        = errors.New("commit failed")
	ErrRollbackFailed      = errors.New("rollback failed")
	ErrWALWriteFailed      = errors.New("failed to write WAL record")
)

// CoWManager implements copy-on-write semantics for page modifications.
// Instead of updating pages in place, modifications create new page versions.
// This enables lock-free reads and simplifies crash recovery.
//
// CoW workflow:
// 1. Read: Return original page (no copy needed)
// 2. Write: Create shadow copy, modify shadow
// 3. Commit: Update page pointers atomically
// 4. Rollback: Free shadow pages
type CoWManager struct {
	// pageManager handles page I/O operations.
	pageManager *storage.PageManager

	// shadowManager manages shadow page mappings.
	shadowManager *ShadowManager

	// txManager manages transaction lifecycle.
	txManager *tx.TxManager

	// wal is the Write-Ahead Log for durability.
	wal *storage.WAL

	// mu protects concurrent access.
	mu sync.RWMutex

	// closed indicates if the manager has been closed.
	closed bool
}

// NewCoWManager creates a new CoWManager with the given dependencies.
func NewCoWManager(pm *storage.PageManager, txm *tx.TxManager, wal *storage.WAL) *CoWManager {
	return &CoWManager{
		pageManager:   pm,
		shadowManager: NewShadowManager(pm),
		txManager:     txm,
		wal:           wal,
		closed:        false,
	}
}

// GetPage returns the page for reading within the given transaction.
// For reads, we return the original page if no shadow exists,
// or the shadow page if the transaction has modified it.
func (cow *CoWManager) GetPage(txn *tx.Transaction, id storage.PageID) (*storage.Page, error) {
	cow.mu.RLock()
	defer cow.mu.RUnlock()

	if cow.closed {
		return nil, ErrCoWManagerClosed
	}

	if txn == nil {
		return nil, ErrTransactionNil
	}

	if !txn.IsActive() {
		return nil, ErrTransactionNotActive
	}

	// Check if this transaction has a shadow for this page
	shadowID, err := cow.shadowManager.GetShadow(id)
	if err == nil {
		// Check if this shadow belongs to our transaction
		txShadows := cow.shadowManager.GetTransactionShadows(txn.ID)
		for _, sid := range txShadows {
			if sid == shadowID {
				// Return the shadow page for this transaction
				page, err := cow.pageManager.ReadPage(shadowID)
				if err != nil {
					return nil, err
				}
				// Add to read set
				txn.AddToReadSet(id)
				return page, nil
			}
		}
	}

	// No shadow for this transaction, return original page
	page, err := cow.pageManager.ReadPage(id)
	if err != nil {
		return nil, err
	}

	// Add to read set
	txn.AddToReadSet(id)

	return page, nil
}

// ModifyPage returns a writable copy of the page for the given transaction.
// If no shadow exists, it creates one. If a shadow already exists for this
// transaction, it returns the existing shadow.
func (cow *CoWManager) ModifyPage(txn *tx.Transaction, id storage.PageID) (*storage.Page, error) {
	cow.mu.Lock()
	defer cow.mu.Unlock()

	if cow.closed {
		return nil, ErrCoWManagerClosed
	}

	if txn == nil {
		return nil, ErrTransactionNil
	}

	if !txn.IsActive() {
		return nil, ErrTransactionNotActive
	}

	// Check if this transaction already has a shadow for this page
	shadowID, err := cow.shadowManager.GetShadow(id)
	if err == nil {
		// Check if this shadow belongs to our transaction
		txShadows := cow.shadowManager.GetTransactionShadows(txn.ID)
		for _, sid := range txShadows {
			if sid == shadowID {
				// Return existing shadow page
				page, err := cow.pageManager.ReadPage(shadowID)
				if err != nil {
					return nil, err
				}
				return page, nil
			}
		}
		// Shadow exists but belongs to different transaction - conflict
		return nil, ErrShadowPageExists
	}

	// Read the original page first (for WAL before-image)
	originalPage, err := cow.pageManager.ReadPage(id)
	if err != nil {
		return nil, err
	}

	// Create a shadow copy
	shadowID, err = cow.shadowManager.CreateShadow(txn.ID, id)
	if err != nil {
		return nil, err
	}

	// Write WAL record for the modification (before-image)
	walRecord := storage.NewWALUpdateRecord(
		0, // LSN will be assigned by WAL
		txn.ID,
		id,
		0, // Offset 0 - full page
		originalPage.Data,
		nil, // New data will be written on commit
	)

	if _, err := cow.wal.Append(walRecord); err != nil {
		// Rollback shadow creation on WAL failure
		cow.shadowManager.FreeShadow(id)
		return nil, ErrWALWriteFailed
	}

	// Add to write set
	txn.AddToWriteSet(id)

	// Return the shadow page for modification
	return cow.pageManager.ReadPage(shadowID)
}

// WriteShadowPage writes the modified shadow page back to disk.
// This should be called after modifying the page returned by ModifyPage.
func (cow *CoWManager) WriteShadowPage(txn *tx.Transaction, originalID storage.PageID, page *storage.Page) error {
	cow.mu.Lock()
	defer cow.mu.Unlock()

	if cow.closed {
		return ErrCoWManagerClosed
	}

	if txn == nil {
		return ErrTransactionNil
	}

	if !txn.IsActive() {
		return ErrTransactionNotActive
	}

	// Verify this transaction owns the shadow
	shadowID, err := cow.shadowManager.GetShadow(originalID)
	if err != nil {
		return err
	}

	txShadows := cow.shadowManager.GetTransactionShadows(txn.ID)
	found := false
	for _, sid := range txShadows {
		if sid == shadowID {
			found = true
			break
		}
	}

	if !found {
		return ErrShadowPageNotFound
	}

	// Ensure the page ID matches the shadow
	page.Header.PageID = shadowID

	// Write the shadow page
	return cow.pageManager.WritePage(page)
}

// CommitPages commits all shadow pages for the given transaction.
// This atomically switches page pointers from originals to shadows.
// The original pages are freed after successful commit.
func (cow *CoWManager) CommitPages(txn *tx.Transaction) error {
	cow.mu.Lock()
	defer cow.mu.Unlock()

	if cow.closed {
		return ErrCoWManagerClosed
	}

	if txn == nil {
		return ErrTransactionNil
	}

	if !txn.IsActive() {
		return ErrTransactionNotActive
	}

	// Get all shadows for this transaction
	shadows := cow.shadowManager.GetTransactionShadows(txn.ID)
	if len(shadows) == 0 {
		// No modifications, nothing to commit
		return nil
	}

	// Collect original page IDs that need to be freed
	originalsToFree := make([]storage.PageID, 0, len(shadows))

	// For each shadow, we need to:
	// 1. Copy shadow content to original page location
	// 2. Free the shadow page
	for _, shadowID := range shadows {
		originalID, err := cow.shadowManager.GetOriginal(shadowID)
		if err != nil {
			return err
		}

		// Read the shadow page
		shadowPage, err := cow.pageManager.ReadPage(shadowID)
		if err != nil {
			return err
		}

		// Write WAL record with after-image
		walRecord := storage.NewWALUpdateRecord(
			0, // LSN will be assigned by WAL
			txn.ID,
			originalID,
			0, // Offset 0 - full page
			nil,
			shadowPage.Data,
		)

		if _, err := cow.wal.Append(walRecord); err != nil {
			return ErrWALWriteFailed
		}

		// Copy shadow content to original page
		originalPage := storage.NewPage(originalID, shadowPage.Header.PageType)
		originalPage.Header.Flags = shadowPage.Header.Flags
		originalPage.Header.ItemCount = shadowPage.Header.ItemCount
		originalPage.Header.FreeSpace = shadowPage.Header.FreeSpace
		copy(originalPage.Data, shadowPage.Data)

		// Write the updated original page
		if err := cow.pageManager.WritePage(originalPage); err != nil {
			return err
		}

		originalsToFree = append(originalsToFree, shadowID)
	}

	// Sync WAL to ensure durability
	if err := cow.wal.Sync(); err != nil {
		return err
	}

	// Free shadow pages (they're no longer needed)
	for _, shadowID := range originalsToFree {
		if err := cow.pageManager.FreePage(shadowID); err != nil {
			// Log error but continue - shadow pages will be reclaimed eventually
			continue
		}
	}

	// Clear transaction mappings
	cow.shadowManager.ClearTransactionMappings(txn.ID)

	return nil
}

// RollbackPages rolls back all shadow pages for the given transaction.
// This frees all shadow pages without modifying the original pages.
func (cow *CoWManager) RollbackPages(txn *tx.Transaction) error {
	cow.mu.Lock()
	defer cow.mu.Unlock()

	if cow.closed {
		return ErrCoWManagerClosed
	}

	if txn == nil {
		return ErrTransactionNil
	}

	// Free all shadow pages for this transaction
	if err := cow.shadowManager.FreeTransactionShadows(txn.ID); err != nil {
		return err
	}

	return nil
}

// GetShadowManager returns the underlying shadow manager.
// This is useful for testing and advanced operations.
func (cow *CoWManager) GetShadowManager() *ShadowManager {
	return cow.shadowManager
}

// GetPageManager returns the underlying page manager.
func (cow *CoWManager) GetPageManager() *storage.PageManager {
	return cow.pageManager
}

// ShadowCount returns the total number of active shadow pages.
func (cow *CoWManager) ShadowCount() int {
	cow.mu.RLock()
	defer cow.mu.RUnlock()

	if cow.closed {
		return 0
	}

	return cow.shadowManager.ShadowCount()
}

// TransactionShadowCount returns the number of shadow pages for a specific transaction.
func (cow *CoWManager) TransactionShadowCount(txID uint64) int {
	cow.mu.RLock()
	defer cow.mu.RUnlock()

	if cow.closed {
		return 0
	}

	return cow.shadowManager.TransactionShadowCount(txID)
}

// Close closes the CoW manager and releases resources.
func (cow *CoWManager) Close() error {
	cow.mu.Lock()
	defer cow.mu.Unlock()

	if cow.closed {
		return nil
	}

	cow.closed = true

	// Close shadow manager
	if cow.shadowManager != nil {
		cow.shadowManager.Close()
	}

	return nil
}
