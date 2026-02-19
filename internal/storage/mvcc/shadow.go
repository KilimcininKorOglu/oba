// Package mvcc provides Multi-Version Concurrency Control components for ObaDB.
package mvcc

import (
	"errors"
	"sync"

	"github.com/oba-ldap/oba/internal/storage"
)

// Shadow page management errors.
var (
	ErrShadowPageNotFound   = errors.New("shadow page not found")
	ErrShadowPageExists     = errors.New("shadow page already exists for this original")
	ErrInvalidShadowMapping = errors.New("invalid shadow page mapping")
	ErrShadowManagerClosed  = errors.New("shadow manager is closed")
)

// ShadowMapping represents a mapping from an original page to its shadow copy.
type ShadowMapping struct {
	OriginalID storage.PageID // The original page ID
	ShadowID   storage.PageID // The shadow (copy) page ID
	TxID       uint64         // Transaction that created this shadow
}

// ShadowManager manages shadow pages for copy-on-write operations.
// It tracks which original pages have shadow copies and provides
// methods to create, lookup, and cleanup shadow pages.
type ShadowManager struct {
	// pageManager is used to allocate and free shadow pages.
	pageManager *storage.PageManager

	// shadowPages maps original page IDs to their shadow page IDs.
	// Key: original PageID, Value: shadow PageID
	shadowPages map[storage.PageID]storage.PageID

	// txShadows maps transaction IDs to their shadow page sets.
	// This allows efficient cleanup on rollback.
	txShadows map[uint64][]storage.PageID

	// reverseMappings maps shadow page IDs back to original page IDs.
	// This is useful for commit operations.
	reverseMappings map[storage.PageID]storage.PageID

	// mu protects all maps.
	mu sync.RWMutex

	// closed indicates if the manager has been closed.
	closed bool
}

// NewShadowManager creates a new ShadowManager with the given PageManager.
func NewShadowManager(pm *storage.PageManager) *ShadowManager {
	return &ShadowManager{
		pageManager:     pm,
		shadowPages:     make(map[storage.PageID]storage.PageID),
		txShadows:       make(map[uint64][]storage.PageID),
		reverseMappings: make(map[storage.PageID]storage.PageID),
		closed:          false,
	}
}

// CreateShadow creates a shadow copy of the given page for the specified transaction.
// It allocates a new page, copies the original page's content, and records the mapping.
// Returns the shadow page ID.
func (sm *ShadowManager) CreateShadow(txID uint64, originalID storage.PageID) (storage.PageID, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.closed {
		return 0, ErrShadowManagerClosed
	}

	// Check if shadow already exists for this original in this transaction
	if existingShadow, exists := sm.shadowPages[originalID]; exists {
		// Return existing shadow if it belongs to the same transaction
		for _, shadowID := range sm.txShadows[txID] {
			if shadowID == existingShadow {
				return existingShadow, nil
			}
		}
		// Shadow exists but belongs to different transaction
		return 0, ErrShadowPageExists
	}

	// Read the original page
	originalPage, err := sm.pageManager.ReadPage(originalID)
	if err != nil {
		return 0, err
	}

	// Allocate a new page for the shadow
	shadowID, err := sm.pageManager.AllocatePage(originalPage.Header.PageType)
	if err != nil {
		return 0, err
	}

	// Create shadow page with copied content
	shadowPage := storage.NewPage(shadowID, originalPage.Header.PageType)
	shadowPage.Header.Flags = originalPage.Header.Flags
	shadowPage.Header.ItemCount = originalPage.Header.ItemCount
	shadowPage.Header.FreeSpace = originalPage.Header.FreeSpace
	copy(shadowPage.Data, originalPage.Data)

	// Write the shadow page
	if err := sm.pageManager.WritePage(shadowPage); err != nil {
		// Try to free the allocated page on error
		sm.pageManager.FreePage(shadowID)
		return 0, err
	}

	// Record the mapping
	sm.shadowPages[originalID] = shadowID
	sm.reverseMappings[shadowID] = originalID
	sm.txShadows[txID] = append(sm.txShadows[txID], shadowID)

	return shadowID, nil
}

// GetShadow returns the shadow page ID for the given original page ID.
// Returns ErrShadowPageNotFound if no shadow exists.
func (sm *ShadowManager) GetShadow(originalID storage.PageID) (storage.PageID, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if sm.closed {
		return 0, ErrShadowManagerClosed
	}

	shadowID, exists := sm.shadowPages[originalID]
	if !exists {
		return 0, ErrShadowPageNotFound
	}

	return shadowID, nil
}

// HasShadow checks if a shadow page exists for the given original page ID.
func (sm *ShadowManager) HasShadow(originalID storage.PageID) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if sm.closed {
		return false
	}

	_, exists := sm.shadowPages[originalID]
	return exists
}

// GetOriginal returns the original page ID for the given shadow page ID.
// Returns ErrShadowPageNotFound if the shadow is not found.
func (sm *ShadowManager) GetOriginal(shadowID storage.PageID) (storage.PageID, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if sm.closed {
		return 0, ErrShadowManagerClosed
	}

	originalID, exists := sm.reverseMappings[shadowID]
	if !exists {
		return 0, ErrShadowPageNotFound
	}

	return originalID, nil
}

// GetTransactionShadows returns all shadow page IDs created by the given transaction.
func (sm *ShadowManager) GetTransactionShadows(txID uint64) []storage.PageID {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if sm.closed {
		return nil
	}

	shadows := sm.txShadows[txID]
	if shadows == nil {
		return nil
	}

	// Return a copy to prevent external modification
	result := make([]storage.PageID, len(shadows))
	copy(result, shadows)
	return result
}

// GetAllMappings returns a copy of all current shadow mappings.
// Key: original PageID, Value: shadow PageID
func (sm *ShadowManager) GetAllMappings() map[storage.PageID]storage.PageID {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if sm.closed {
		return nil
	}

	result := make(map[storage.PageID]storage.PageID, len(sm.shadowPages))
	for k, v := range sm.shadowPages {
		result[k] = v
	}
	return result
}

// RemoveShadow removes the shadow mapping for the given original page ID.
// This does NOT free the shadow page - use FreeShadow for that.
func (sm *ShadowManager) RemoveShadow(originalID storage.PageID) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.closed {
		return ErrShadowManagerClosed
	}

	shadowID, exists := sm.shadowPages[originalID]
	if !exists {
		return ErrShadowPageNotFound
	}

	delete(sm.shadowPages, originalID)
	delete(sm.reverseMappings, shadowID)

	return nil
}

// FreeShadow removes the shadow mapping and frees the shadow page.
func (sm *ShadowManager) FreeShadow(originalID storage.PageID) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.closed {
		return ErrShadowManagerClosed
	}

	shadowID, exists := sm.shadowPages[originalID]
	if !exists {
		return ErrShadowPageNotFound
	}

	// Free the shadow page
	if err := sm.pageManager.FreePage(shadowID); err != nil {
		return err
	}

	// Remove mappings
	delete(sm.shadowPages, originalID)
	delete(sm.reverseMappings, shadowID)

	return nil
}

// FreeTransactionShadows frees all shadow pages created by the given transaction.
// This is typically called during rollback.
func (sm *ShadowManager) FreeTransactionShadows(txID uint64) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.closed {
		return ErrShadowManagerClosed
	}

	shadows := sm.txShadows[txID]
	if shadows == nil {
		return nil
	}

	var lastErr error
	for _, shadowID := range shadows {
		// Find the original page ID
		originalID, exists := sm.reverseMappings[shadowID]
		if !exists {
			continue
		}

		// Free the shadow page
		if err := sm.pageManager.FreePage(shadowID); err != nil {
			lastErr = err
			continue
		}

		// Remove mappings
		delete(sm.shadowPages, originalID)
		delete(sm.reverseMappings, shadowID)
	}

	// Remove transaction shadow list
	delete(sm.txShadows, txID)

	return lastErr
}

// ClearTransactionMappings removes the transaction's shadow list without freeing pages.
// This is typically called after commit when shadows become the new originals.
func (sm *ShadowManager) ClearTransactionMappings(txID uint64) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.closed {
		return
	}

	shadows := sm.txShadows[txID]
	if shadows == nil {
		return
	}

	// Remove all mappings for this transaction's shadows
	for _, shadowID := range shadows {
		originalID, exists := sm.reverseMappings[shadowID]
		if exists {
			delete(sm.shadowPages, originalID)
			delete(sm.reverseMappings, shadowID)
		}
	}

	delete(sm.txShadows, txID)
}

// ShadowCount returns the total number of active shadow pages.
func (sm *ShadowManager) ShadowCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if sm.closed {
		return 0
	}

	return len(sm.shadowPages)
}

// TransactionShadowCount returns the number of shadow pages for a specific transaction.
func (sm *ShadowManager) TransactionShadowCount(txID uint64) int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if sm.closed {
		return 0
	}

	return len(sm.txShadows[txID])
}

// Close closes the shadow manager.
// Note: This does NOT free any shadow pages - they should be cleaned up
// by the CoW manager before closing.
func (sm *ShadowManager) Close() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.closed {
		return nil
	}

	sm.closed = true
	sm.shadowPages = nil
	sm.txShadows = nil
	sm.reverseMappings = nil

	return nil
}
