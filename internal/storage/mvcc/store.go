// Package mvcc provides Multi-Version Concurrency Control for ObaDB.
package mvcc

import (
	"sync"

	"github.com/KilimcininKorOglu/oba/internal/storage"
	"github.com/KilimcininKorOglu/oba/internal/storage/tx"
)

// VersionStore manages version chains for all entries.
// It provides methods to create, read, and delete versions with proper
// visibility rules based on transaction snapshots.
type VersionStore struct {
	// versions maps DN (Distinguished Name) to the latest version in the chain.
	versions map[string]*Version

	// pageManager handles page allocation and I/O.
	pageManager *storage.PageManager

	// cache provides LRU caching for frequently accessed entries.
	cache *EntryCache

	// diskLoader is a callback to load entries from disk on cache miss.
	diskLoader func(dn string) (*Version, storage.PageID, uint16, error)

	// mu protects concurrent access to the versions map.
	mu sync.RWMutex

	// activeWriters tracks which DNs have uncommitted writes.
	// This is used to detect write-write conflicts.
	activeWriters map[string]uint64

	// writerMu protects the activeWriters map.
	writerMu sync.RWMutex
}

// NewVersionStore creates a new VersionStore with the given PageManager.
func NewVersionStore(pm *storage.PageManager) *VersionStore {
	return NewVersionStoreWithCache(pm, DefaultCacheSize)
}

// NewVersionStoreWithCache creates a new VersionStore with a specified cache size.
func NewVersionStoreWithCache(pm *storage.PageManager, cacheSize int) *VersionStore {
	return &VersionStore{
		versions:      make(map[string]*Version),
		pageManager:   pm,
		cache:         NewEntryCache(cacheSize),
		activeWriters: make(map[string]uint64),
	}
}

// SetDiskLoader sets the callback function for loading entries from disk.
func (vs *VersionStore) SetDiskLoader(loader func(dn string) (*Version, storage.PageID, uint16, error)) {
	vs.diskLoader = loader
}

// GetVisible returns the version of the entry visible to the given snapshot.
// It traverses the version chain to find the appropriate version.
//
// Visibility rules:
// 1. Start from the latest version
// 2. For each version in the chain:
//   - If uncommitted (CommitTS == 0): visible only to the creating transaction
//   - If committed (CommitTS > 0): visible if CommitTS <= snapshot
//
// 3. Return the first visible version found
// 4. If the visible version is deleted, return ErrVersionDeleted
// 5. If no visible version exists, return ErrNoVisibleVersion
func (vs *VersionStore) GetVisible(dn string, snapshot uint64) (*Version, error) {
	return vs.GetVisibleForTx(dn, snapshot, 0)
}

// GetVisibleForTx returns the version visible to a specific transaction.
// The activeTxID parameter allows uncommitted versions to be visible to their
// creating transaction.
func (vs *VersionStore) GetVisibleForTx(dn string, snapshot uint64, activeTxID uint64) (*Version, error) {
	// First check in-memory versions (for uncommitted changes)
	vs.mu.RLock()
	latestVersion := vs.versions[dn]
	vs.mu.RUnlock()

	if latestVersion != nil {
		// Traverse the version chain to find the visible version
		current := latestVersion
		for current != nil {
			if current.IsVisibleTo(snapshot, activeTxID) {
				if current.IsDeleted() {
					return nil, ErrVersionDeleted
				}
				return current.Clone(), nil
			}
			current = current.GetPrev()
		}
	}

	// Check cache
	if vs.cache != nil {
		if cached := vs.cache.Get(dn); cached != nil && cached.Version != nil {
			if cached.Version.IsVisibleTo(snapshot, activeTxID) {
				if cached.Version.IsDeleted() {
					return nil, ErrVersionDeleted
				}
				return cached.Version.Clone(), nil
			}
		}
	}

	// Cache miss - try to load from disk
	if vs.diskLoader != nil {
		version, pageID, slotID, err := vs.diskLoader(dn)
		if err != nil {
			return nil, ErrVersionNotFound
		}
		if version != nil {
			// Add to cache
			if vs.cache != nil {
				vs.cache.Put(dn, version, pageID, slotID)
			}
			if version.IsVisibleTo(snapshot, activeTxID) {
				if version.IsDeleted() {
					return nil, ErrVersionDeleted
				}
				return version.Clone(), nil
			}
		}
	}

	return nil, ErrVersionNotFound
}

// CreateVersion creates a new version for the given DN.
// The new version is linked to the previous version (if any) and becomes
// the latest version in the chain.
//
// The version is initially uncommitted (CommitTS == 0) and will be committed
// when the transaction commits.
func (vs *VersionStore) CreateVersion(txn *tx.Transaction, dn string, data []byte) error {
	_, _, err := vs.CreateVersionWithLocation(txn, dn, data)
	return err
}

// CreateVersionWithLocation creates a new version and returns its storage location.
func (vs *VersionStore) CreateVersionWithLocation(txn *tx.Transaction, dn string, data []byte) (storage.PageID, uint16, error) {
	if txn == nil {
		return 0, 0, ErrNilTransaction
	}

	if !txn.IsActive() {
		return 0, 0, ErrTransactionAborted
	}

	// Check for write-write conflicts
	vs.writerMu.Lock()
	if existingTxID, exists := vs.activeWriters[dn]; exists && existingTxID != txn.ID {
		vs.writerMu.Unlock()
		return 0, 0, ErrVersionConflict
	}
	vs.activeWriters[dn] = txn.ID
	vs.writerMu.Unlock()

	// Allocate a page for the new version data
	pageID, slotID, err := vs.allocateStorage(data)
	if err != nil {
		vs.clearActiveWriter(dn, txn.ID)
		return 0, 0, err
	}

	// Create the new version
	newVersion := NewVersion(txn.ID, data, pageID, slotID)

	// Link to the previous version
	vs.mu.Lock()
	if prevVersion := vs.versions[dn]; prevVersion != nil {
		newVersion.SetPrev(prevVersion)
	}
	vs.versions[dn] = newVersion
	vs.mu.Unlock()

	// Add the page to the transaction's write set
	txn.AddToWriteSet(pageID)

	// Write-through: update cache
	if vs.cache != nil {
		vs.cache.Put(dn, newVersion, pageID, slotID)
	}

	return pageID, slotID, nil
}

// DeleteVersion marks the entry as deleted by creating a delete version.
// The delete version is linked to the previous version chain.
func (vs *VersionStore) DeleteVersion(txn *tx.Transaction, dn string) error {
	if txn == nil {
		return ErrNilTransaction
	}

	if !txn.IsActive() {
		return ErrTransactionAborted
	}

	// Check if the entry exists
	vs.mu.RLock()
	latestVersion := vs.versions[dn]
	vs.mu.RUnlock()

	if latestVersion == nil {
		return ErrVersionNotFound
	}

	// Check for write-write conflicts
	vs.writerMu.Lock()
	if existingTxID, exists := vs.activeWriters[dn]; exists && existingTxID != txn.ID {
		vs.writerMu.Unlock()
		return ErrVersionConflict
	}
	vs.activeWriters[dn] = txn.ID
	vs.writerMu.Unlock()

	// Get the storage location from the latest version
	pageID, slotID := latestVersion.GetLocation()

	// Create a delete version
	deleteVersion := NewDeletedVersion(txn.ID, pageID, slotID)

	// Link to the previous version
	vs.mu.Lock()
	deleteVersion.SetPrev(vs.versions[dn])
	vs.versions[dn] = deleteVersion
	vs.mu.Unlock()

	// Add the page to the transaction's write set
	txn.AddToWriteSet(pageID)

	// Write-through: remove from cache
	if vs.cache != nil {
		vs.cache.Delete(dn)
	}

	return nil
}

// CommitVersion commits all versions created by the given transaction.
// This sets the CommitTS on all uncommitted versions created by the transaction.
func (vs *VersionStore) CommitVersion(txn *tx.Transaction, commitTS uint64) {
	if txn == nil {
		return
	}

	vs.mu.RLock()
	defer vs.mu.RUnlock()

	// Iterate through all versions and commit those belonging to this transaction
	for dn, version := range vs.versions {
		current := version
		for current != nil {
			if current.GetTxID() == txn.ID && !current.IsCommitted() {
				current.Commit(commitTS)
			}
			current = current.GetPrev()
		}

		// Clear the active writer for this DN
		vs.clearActiveWriter(dn, txn.ID)
	}
}

// RollbackVersion removes all uncommitted versions created by the given transaction.
// This restores the version chain to its state before the transaction started.
func (vs *VersionStore) RollbackVersion(txn *tx.Transaction) {
	if txn == nil {
		return
	}

	vs.mu.Lock()
	defer vs.mu.Unlock()

	// Iterate through all versions and remove uncommitted ones from this transaction
	for dn, version := range vs.versions {
		// Find the first version not created by this transaction
		current := version
		for current != nil && current.GetTxID() == txn.ID && !current.IsCommitted() {
			current = current.GetPrev()
		}

		if current == nil {
			// All versions were from this transaction, remove the entry
			delete(vs.versions, dn)
			// Also remove from cache
			if vs.cache != nil {
				vs.cache.Delete(dn)
			}
		} else if current != version {
			// Some versions were removed, update the latest
			vs.versions[dn] = current
		}

		// Clear the active writer for this DN
		vs.clearActiveWriter(dn, txn.ID)
	}
}

// allocateStorage allocates storage for version data.
// Returns the PageID and SlotID where the data is stored.
func (vs *VersionStore) allocateStorage(data []byte) (storage.PageID, uint16, error) {
	if vs.pageManager == nil {
		// If no page manager, use dummy values (for testing without persistence)
		return 0, 0, nil
	}

	// Allocate a data page for the version
	pageID, err := vs.pageManager.AllocatePage(storage.PageTypeData)
	if err != nil {
		return 0, 0, err
	}

	// For simplicity, we use slot 0 for each version
	// In a real implementation, we would use a slotted page format
	slotID := uint16(0)

	// Write the data to the page
	page, err := vs.pageManager.ReadPage(pageID)
	if err != nil {
		return 0, 0, err
	}

	// Store the data length and data in the page
	if len(data)+4 <= len(page.Data) {
		// Write data length (4 bytes) + data
		page.Data[0] = byte(len(data))
		page.Data[1] = byte(len(data) >> 8)
		page.Data[2] = byte(len(data) >> 16)
		page.Data[3] = byte(len(data) >> 24)
		copy(page.Data[4:], data)
		page.Header.ItemCount = 1

		if err := vs.pageManager.WritePage(page); err != nil {
			return 0, 0, err
		}
	}

	return pageID, slotID, nil
}

// clearActiveWriter removes the active writer for a DN if it matches the given txID.
func (vs *VersionStore) clearActiveWriter(dn string, txID uint64) {
	vs.writerMu.Lock()
	defer vs.writerMu.Unlock()

	if existingTxID, exists := vs.activeWriters[dn]; exists && existingTxID == txID {
		delete(vs.activeWriters, dn)
	}
}

// GetLatestVersion returns the latest version for a DN (regardless of visibility).
// This is useful for debugging and testing.
func (vs *VersionStore) GetLatestVersion(dn string) *Version {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	return vs.versions[dn]
}

// LoadCommittedVersion loads a committed version from disk into the version store.
// This is used during database initialization to restore persisted data.
func (vs *VersionStore) LoadCommittedVersion(dn string, data []byte, pageID storage.PageID, slotID uint16) {
	version := NewVersion(0, data, pageID, slotID)
	version.CommitTS = 1 // Mark as committed with timestamp 1

	vs.mu.Lock()
	vs.versions[dn] = version
	vs.mu.Unlock()
}

// GetVersionChain returns all versions in the chain for a DN.
// This is useful for debugging and testing.
func (vs *VersionStore) GetVersionChain(dn string) []*Version {
	vs.mu.RLock()
	latestVersion := vs.versions[dn]
	vs.mu.RUnlock()

	if latestVersion == nil {
		return nil
	}

	var chain []*Version
	current := latestVersion
	for current != nil {
		chain = append(chain, current.Clone())
		current = current.GetPrev()
	}

	return chain
}

// EntryCount returns the number of entries in the version store.
func (vs *VersionStore) EntryCount() int {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	return len(vs.versions)
}

// HasEntry returns true if an entry exists for the given DN.
func (vs *VersionStore) HasEntry(dn string) bool {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	_, exists := vs.versions[dn]
	return exists
}

// Clear removes all versions from the store.
// This is useful for testing.
func (vs *VersionStore) Clear() {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	vs.writerMu.Lock()
	defer vs.writerMu.Unlock()

	vs.versions = make(map[string]*Version)
	vs.activeWriters = make(map[string]uint64)
}

// GarbageCollect removes old versions that are no longer visible to any active snapshot.
// The oldestActiveSnapshot parameter is the oldest snapshot timestamp that is still active.
// Versions with CommitTS < oldestActiveSnapshot and that have newer committed versions
// can be safely removed.
func (vs *VersionStore) GarbageCollect(oldestActiveSnapshot uint64) int {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	removedCount := 0

	for dn, latestVersion := range vs.versions {
		// Find the first committed version visible to the oldest snapshot
		var visibleVersion *Version
		current := latestVersion

		for current != nil {
			if current.IsCommitted() && current.GetCommitTS() <= oldestActiveSnapshot {
				visibleVersion = current
				break
			}
			current = current.GetPrev()
		}

		// If we found a visible version, we can remove all versions after it
		if visibleVersion != nil {
			prev := visibleVersion.GetPrev()
			if prev != nil {
				// Count versions to be removed
				toRemove := prev
				for toRemove != nil {
					removedCount++
					toRemove = toRemove.GetPrev()
				}

				// Truncate the chain
				visibleVersion.SetPrev(nil)
			}
		}

		// If the latest version is deleted and committed, and there are no
		// active snapshots that could see the non-deleted version, remove the entry
		if latestVersion.IsDeleted() && latestVersion.IsCommitted() {
			if latestVersion.GetCommitTS() < oldestActiveSnapshot {
				delete(vs.versions, dn)
				removedCount++
			}
		}
	}

	return removedCount
}

// Stats returns statistics about the version store.
type VersionStoreStats struct {
	EntryCount       int
	TotalVersions    int
	ActiveWriters    int
	AverageChainLen  float64
	MaxChainLen      int
	DeletedEntries   int
	UncommittedCount int
}

// Stats returns current statistics about the version store.
func (vs *VersionStore) Stats() VersionStoreStats {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	vs.writerMu.RLock()
	defer vs.writerMu.RUnlock()

	stats := VersionStoreStats{
		EntryCount:    len(vs.versions),
		ActiveWriters: len(vs.activeWriters),
	}

	totalChainLen := 0
	for _, version := range vs.versions {
		chainLen := version.ChainLength()
		totalChainLen += chainLen
		stats.TotalVersions += chainLen

		if chainLen > stats.MaxChainLen {
			stats.MaxChainLen = chainLen
		}

		if version.IsDeleted() {
			stats.DeletedEntries++
		}

		// Count uncommitted versions
		current := version
		for current != nil {
			if !current.IsCommitted() {
				stats.UncommittedCount++
			}
			current = current.GetPrev()
		}
	}

	if stats.EntryCount > 0 {
		stats.AverageChainLen = float64(totalChainLen) / float64(stats.EntryCount)
	}

	return stats
}
