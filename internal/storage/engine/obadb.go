// Package engine provides the ObaDB storage engine implementation.
package engine

import (
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/oba-ldap/oba/internal/storage"
	"github.com/oba-ldap/oba/internal/storage/index"
	"github.com/oba-ldap/oba/internal/storage/mvcc"
	"github.com/oba-ldap/oba/internal/storage/radix"
	"github.com/oba-ldap/oba/internal/storage/tx"
)

// File names for ObaDB storage.
const (
	DataFileName        = "data.oba"
	IndexFileName       = "index.oba"
	WALFileName         = "wal.oba"
	CacheDir            = "cache"
	RadixCacheFileName  = "radix.cache"
	BTreeCacheFileName  = "btree.cache"
	EntryCacheFileName  = "entry.cache"
)

// ObaDB errors.
var (
	ErrDatabaseClosed    = errors.New("database is closed")
	ErrDatabaseReadOnly  = errors.New("database is read-only")
	ErrEntryNotFound     = errors.New("entry not found")
	ErrEntryExists       = errors.New("entry already exists")
	ErrInvalidDN         = errors.New("invalid distinguished name")
	ErrInvalidEntry      = errors.New("invalid entry")
	ErrTransactionClosed = errors.New("transaction is closed")
)

// ObaDB is the main storage engine implementation.
// It integrates all storage components into a cohesive API.
type ObaDB struct {
	// Core components
	pageManager     *storage.PageManager
	wal             *storage.WAL
	txManager       *tx.TxManager
	versionStore    *mvcc.VersionStore
	snapshotManager *mvcc.SnapshotManager
	radixTree       *radix.RadixTree
	indexManager    *index.IndexManager
	gc              *mvcc.GarbageCollector

	// Additional components
	bufferPool        *storage.BufferPool
	checkpointManager *storage.CheckpointManager

	// Configuration
	options storage.EngineOptions
	path    string

	// State
	closed   bool
	readOnly bool
	mu       sync.RWMutex
}

// Open opens or creates an ObaDB database at the given path.
func Open(path string, opts storage.EngineOptions) (*ObaDB, error) {
	// Validate and apply defaults
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	// Create directory if needed
	if opts.CreateIfNotExists {
		if err := os.MkdirAll(path, 0755); err != nil {
			return nil, err
		}
	}

	db := &ObaDB{
		options:  opts,
		path:     path,
		closed:   false,
		readOnly: opts.ReadOnly,
	}

	// Initialize components
	if err := db.initComponents(); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

// initComponents initializes all storage components.
func (db *ObaDB) initComponents() error {
	var err error

	// 1. Open page manager for data file
	dataPath := filepath.Join(db.path, DataFileName)
	pmOpts := storage.Options{
		PageSize:     db.options.PageSize,
		InitialPages: db.options.InitialPages,
		CreateIfNew:  db.options.CreateIfNotExists,
		ReadOnly:     db.options.ReadOnly,
		SyncOnWrite:  db.options.SyncOnWrite,
	}

	db.pageManager, err = storage.OpenPageManager(dataPath, pmOpts)
	if err != nil {
		return err
	}

	// 2. Open WAL
	if !db.options.ReadOnly {
		walPath := filepath.Join(db.path, WALFileName)
		db.wal, err = storage.OpenWAL(walPath)
		if err != nil {
			return err
		}
	}

	// 3. Create buffer pool
	db.bufferPool = storage.NewBufferPool(db.options.BufferPoolSize, db.options.PageSize)
	db.bufferPool.SetFlushCallback(func(pageID storage.PageID, data []byte) error {
		page := &storage.Page{
			Header: storage.PageHeader{PageID: pageID},
			Data:   data,
		}
		return db.pageManager.WritePage(page)
	})

	// 4. Create transaction manager
	if db.wal != nil {
		db.txManager = tx.NewTxManager(db.wal)
	}

	// 5. Create version store
	db.versionStore = mvcc.NewVersionStore(db.pageManager)

	// 6. Create snapshot manager
	db.snapshotManager = mvcc.NewSnapshotManager(db.txManager)

	// 7. Initialize or load radix tree
	if err := db.initRadixTree(); err != nil {
		return err
	}

	// 8. Open index manager
	indexPath := filepath.Join(db.path, IndexFileName)
	indexPMOpts := storage.Options{
		PageSize:     db.options.PageSize,
		InitialPages: db.options.InitialPages,
		CreateIfNew:  db.options.CreateIfNotExists,
		ReadOnly:     db.options.ReadOnly,
		SyncOnWrite:  db.options.SyncOnWrite,
	}

	indexPM, err := storage.OpenPageManager(indexPath, indexPMOpts)
	if err != nil {
		return err
	}

	db.indexManager, err = index.NewIndexManager(indexPM)
	if err != nil {
		return err
	}

	// 9. Create checkpoint manager
	if db.wal != nil {
		db.checkpointManager = storage.NewCheckpointManager(db.wal, db.pageManager)
		db.checkpointManager.SetBufferPool(db.bufferPool)
		db.checkpointManager.SetCheckpointInterval(db.options.CheckpointInterval)

		if db.txManager != nil {
			db.checkpointManager.SetActiveTxCallback(func() []uint64 {
				activeTxs := db.txManager.GetActiveTransactions()
				ids := make([]uint64, len(activeTxs))
				for i, t := range activeTxs {
					ids[i] = t.ID
				}
				return ids
			})
		}
	}

	// 10. Create garbage collector
	if db.options.GCEnabled && !db.options.ReadOnly {
		gcConfig := mvcc.GCConfig{
			Interval: db.options.GCInterval,
		}
		db.gc = mvcc.NewGarbageCollectorWithConfig(
			db.versionStore,
			db.snapshotManager,
			db.pageManager,
			gcConfig,
		)
		if err := db.gc.Start(); err != nil {
			return err
		}
	}

	// 11. Set up disk loader for lazy loading
	db.setupDiskLoader()

	// 12. Try to load entry cache for fast startup
	entryCachePath := filepath.Join(db.path, CacheDir, EntryCacheFileName)
	txID := db.getLastTxID()

	if err := db.versionStore.LoadCache(entryCachePath, txID); err == nil {
		// Cache loaded successfully, skip preload
		return nil
	}

	// 13. Fallback: Preload hot entries asynchronously
	go db.preloadHotEntriesAsync()

	return nil
}

// initRadixTree initializes or loads the radix tree.
func (db *ObaDB) initRadixTree() error {
	// Check if we have a stored root page ID in the header
	header := db.pageManager.Header()

	if header.RootPages.DNIndex != 0 {
		// Try to load from cache first
		cachePath := filepath.Join(db.path, CacheDir, RadixCacheFileName)
		txID := db.getLastTxID()

		// Create tree with root page
		var err error
		db.radixTree, err = radix.NewRadixTreeWithRoot(db.pageManager, header.RootPages.DNIndex)
		if err != nil {
			return err
		}

		// Try to load cache (faster startup)
		if cacheErr := db.radixTree.LoadCache(cachePath, txID); cacheErr == nil {
			// Cache loaded successfully
			return nil
		}
		// Cache miss or stale - tree already loaded from pages
		return nil
	}

	// Create new tree
	var err error
	db.radixTree, err = radix.NewRadixTree(db.pageManager)
	if err != nil {
		return err
	}

	// Save root page ID to header
	header.RootPages.DNIndex = db.radixTree.RootPageID()
	if err := db.pageManager.UpdateHeader(header); err != nil {
		return err
	}

	return nil
}

// setupDiskLoader configures the version store to load entries from disk on cache miss.
func (db *ObaDB) setupDiskLoader() {
	if db.versionStore == nil || db.radixTree == nil || db.pageManager == nil {
		return
	}

	db.versionStore.SetDiskLoader(func(dn string) (*mvcc.Version, storage.PageID, uint16, error) {
		pageID, slotID, found := db.radixTree.Lookup(dn)
		if !found || pageID == 0 {
			return nil, 0, 0, ErrEntryNotFound
		}

		page, err := db.pageManager.ReadPage(pageID)
		if err != nil {
			return nil, 0, 0, err
		}

		if len(page.Data) < 4 {
			return nil, 0, 0, ErrInvalidEntry
		}

		dataLen := int(page.Data[0]) | int(page.Data[1])<<8 | int(page.Data[2])<<16 | int(page.Data[3])<<24
		if dataLen <= 0 || dataLen+4 > len(page.Data) {
			return nil, 0, 0, ErrInvalidEntry
		}

		data := make([]byte, dataLen)
		copy(data, page.Data[4:4+dataLen])

		version := mvcc.NewCommittedVersion(data, pageID, slotID)
		return version, pageID, slotID, nil
	})
}

// preloadHotEntries preloads frequently accessed entries into the cache at startup.
func (db *ObaDB) preloadHotEntries() error {
	if db.radixTree == nil || db.versionStore == nil || db.pageManager == nil {
		return nil
	}

	const maxPreload = 1000

	// Collect entries to preload
	type entryInfo struct {
		dn     string
		pageID storage.PageID
		slotID uint16
	}
	entries := make([]entryInfo, 0, maxPreload)

	db.radixTree.IterateSubtree("", func(dn string, pageID storage.PageID, slotID uint16) bool {
		if pageID == 0 || len(entries) >= maxPreload {
			return len(entries) < maxPreload
		}
		entries = append(entries, entryInfo{dn, pageID, slotID})
		return true
	})

	if len(entries) == 0 {
		return nil
	}

	// Collect unique page IDs
	pageIDSet := make(map[storage.PageID]bool)
	for _, e := range entries {
		pageIDSet[e.pageID] = true
	}

	pageIDs := make([]storage.PageID, 0, len(pageIDSet))
	for id := range pageIDSet {
		pageIDs = append(pageIDs, id)
	}

	// Batch read all pages
	pages, err := db.pageManager.ReadPages(pageIDs)
	if err != nil {
		return err
	}

	// Build page map for quick lookup
	pageMap := make(map[storage.PageID]*storage.Page)
	for i, id := range pageIDs {
		if pages[i] != nil {
			pageMap[id] = pages[i]
		}
	}

	// Load entries from cached pages
	for _, e := range entries {
		page, ok := pageMap[e.pageID]
		if !ok || page == nil {
			continue
		}

		if len(page.Data) < 4 {
			continue
		}

		dataLen := int(page.Data[0]) | int(page.Data[1])<<8 | int(page.Data[2])<<16 | int(page.Data[3])<<24
		if dataLen <= 0 || dataLen+4 > len(page.Data) {
			continue
		}

		data := make([]byte, dataLen)
		copy(data, page.Data[4:4+dataLen])

		db.versionStore.LoadCommittedVersion(e.dn, data, e.pageID, e.slotID)
	}

	return nil
}

// preloadHotEntriesAsync preloads hot entries in the background.
func (db *ObaDB) preloadHotEntriesAsync() {
	db.mu.RLock()
	if db.closed {
		db.mu.RUnlock()
		return
	}
	db.mu.RUnlock()

	db.preloadHotEntries()
}

// Close closes the database and releases all resources.
func (db *ObaDB) Close() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
		return nil
	}

	db.closed = true

	var errs []error

	// Stop garbage collector
	if db.gc != nil {
		if err := db.gc.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	// Flush buffer pool
	if db.bufferPool != nil {
		if err := db.bufferPool.FlushAll(); err != nil {
			errs = append(errs, err)
		}
	}

	// Persist radix tree
	if db.radixTree != nil {
		if err := db.radixTree.Persist(); err != nil {
			errs = append(errs, err)
		}
	}

	// Save caches before closing
	db.saveCachesInternal()

	// Close index manager
	if db.indexManager != nil {
		if err := db.indexManager.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	// Close WAL
	if db.wal != nil {
		if err := db.wal.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	// Close page manager
	if db.pageManager != nil {
		if err := db.pageManager.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errs[0]
	}

	return nil
}

// Begin starts a new transaction.
func (db *ObaDB) Begin() (interface{}, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.closed {
		return nil, ErrDatabaseClosed
	}

	if db.txManager == nil {
		return nil, ErrDatabaseReadOnly
	}

	return db.txManager.Begin()
}

// Commit commits the transaction.
func (db *ObaDB) Commit(txnIface interface{}) error {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.closed {
		return ErrDatabaseClosed
	}

	if db.txManager == nil {
		return ErrDatabaseReadOnly
	}

	txn, ok := txnIface.(*tx.Transaction)
	if !ok || txn == nil {
		return tx.ErrNilTransaction
	}

	// Get commit timestamp
	commitTS := db.snapshotManager.AdvanceTimestamp()

	// Commit versions in version store
	db.versionStore.CommitVersion(txn, commitTS)

	// Commit transaction
	return db.txManager.Commit(txn)
}

// Rollback aborts the transaction.
func (db *ObaDB) Rollback(txnIface interface{}) error {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.closed {
		return ErrDatabaseClosed
	}

	if db.txManager == nil {
		return ErrDatabaseReadOnly
	}

	txn, ok := txnIface.(*tx.Transaction)
	if !ok || txn == nil {
		return tx.ErrNilTransaction
	}

	// Rollback versions in version store
	db.versionStore.RollbackVersion(txn)

	// Rollback transaction
	return db.txManager.Rollback(txn)
}

// Get retrieves an entry by its DN.
func (db *ObaDB) Get(txnIface interface{}, dn string) (*storage.Entry, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.closed {
		return nil, ErrDatabaseClosed
	}

	if dn == "" {
		return nil, ErrInvalidDN
	}

	// Normalize DN
	dn = normalizeDN(dn)

	// Get snapshot timestamp
	var snapshot uint64
	var activeTxID uint64
	if txn, ok := txnIface.(*tx.Transaction); ok && txn != nil {
		snapshot = txn.Snapshot
		activeTxID = txn.ID
	} else {
		snapshot = db.snapshotManager.CurrentTimestamp()
	}

	// Get version from version store
	version, err := db.versionStore.GetVisibleForTx(dn, snapshot, activeTxID)
	if err != nil {
		if err == mvcc.ErrVersionNotFound || err == mvcc.ErrNoVisibleVersion {
			return nil, ErrEntryNotFound
		}
		if err == mvcc.ErrVersionDeleted {
			return nil, ErrEntryNotFound
		}
		return nil, err
	}

	// Deserialize entry from version data
	entry, err := deserializeEntry(dn, version.GetData())
	if err != nil {
		return nil, err
	}

	return entry, nil
}

// Put stores an entry.
func (db *ObaDB) Put(txnIface interface{}, entry *storage.Entry) error {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.closed {
		return ErrDatabaseClosed
	}

	if db.readOnly {
		return ErrDatabaseReadOnly
	}

	txn, ok := txnIface.(*tx.Transaction)
	if !ok || txn == nil {
		return tx.ErrNilTransaction
	}

	if entry == nil || entry.DN == "" {
		return ErrInvalidEntry
	}

	// Normalize DN
	dn := normalizeDN(entry.DN)
	entry.DN = dn

	// Serialize entry
	data, err := serializeEntry(entry)
	if err != nil {
		return err
	}

	// Check if entry exists (for index update)
	var oldEntry *storage.Entry
	existingVersion, err := db.versionStore.GetVisibleForTx(dn, txn.Snapshot, txn.ID)
	if err == nil && existingVersion != nil {
		oldEntry, _ = deserializeEntry(dn, existingVersion.GetData())
	}

	// Create version in version store and get the storage location
	pageID, slotID, err := db.versionStore.CreateVersionWithLocation(txn, dn, data)
	if err != nil {
		return err
	}

	// Update radix tree if this is a new entry
	if oldEntry == nil {
		if err := db.radixTree.Insert(dn, pageID, slotID); err != nil {
			if err != radix.ErrEntryExists {
				return err
			}
		}
	}

	// Update indexes
	if err := db.updateIndexes(oldEntry, entry); err != nil {
		return err
	}

	return nil
}

// Delete removes an entry.
func (db *ObaDB) Delete(txnIface interface{}, dn string) error {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.closed {
		return ErrDatabaseClosed
	}

	if db.readOnly {
		return ErrDatabaseReadOnly
	}

	txn, ok := txnIface.(*tx.Transaction)
	if !ok || txn == nil {
		return tx.ErrNilTransaction
	}

	if dn == "" {
		return ErrInvalidDN
	}

	// Normalize DN
	dn = normalizeDN(dn)

	// Check if entry exists
	existingVersion, err := db.versionStore.GetVisibleForTx(dn, txn.Snapshot, txn.ID)
	if err != nil {
		if err == mvcc.ErrVersionNotFound || err == mvcc.ErrNoVisibleVersion {
			return ErrEntryNotFound
		}
		return err
	}

	// Get old entry for index update
	var oldEntry *storage.Entry
	if existingVersion != nil {
		oldEntry, _ = deserializeEntry(dn, existingVersion.GetData())
	}

	// Delete version in version store
	if err := db.versionStore.DeleteVersion(txn, dn); err != nil {
		return err
	}

	// Update indexes (remove old entry)
	if oldEntry != nil {
		if err := db.updateIndexes(oldEntry, nil); err != nil {
			return err
		}
	}

	return nil
}

// HasChildren returns true if the entry at the given DN has child entries.
func (db *ObaDB) HasChildren(txnIface interface{}, dn string) (bool, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.closed {
		return false, ErrDatabaseClosed
	}

	if dn == "" {
		return false, ErrInvalidDN
	}

	// Normalize DN
	dn = normalizeDN(dn)

	// Use the radix tree to check for children
	return db.radixTree.HasChildren(dn)
}

// SearchByDN searches for entries by DN with the given scope.
func (db *ObaDB) SearchByDN(txnIface interface{}, baseDN string, scope storage.Scope) storage.Iterator {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.closed {
		return &errorIterator{err: ErrDatabaseClosed}
	}

	// Normalize base DN
	baseDN = normalizeDN(baseDN)

	// Get snapshot info
	var snapshot uint64
	var activeTxID uint64
	if txn, ok := txnIface.(*tx.Transaction); ok && txn != nil {
		snapshot = txn.Snapshot
		activeTxID = txn.ID
	} else {
		snapshot = db.snapshotManager.CurrentTimestamp()
	}

	// Convert storage.Scope to radix.Scope
	radixScope := radix.Scope(scope)

	// Create radix tree iterator
	radixIter, err := db.radixTree.Iterator(baseDN, radixScope)
	if err != nil {
		return &errorIterator{err: err}
	}

	return &dnIterator{
		db:         db,
		radixIter:  radixIter,
		snapshot:   snapshot,
		activeTxID: activeTxID,
	}
}

// SearchByFilter searches for entries matching the given filter.
func (db *ObaDB) SearchByFilter(txnIface interface{}, baseDN string, f interface{}) storage.Iterator {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.closed {
		return &errorIterator{err: ErrDatabaseClosed}
	}

	// Normalize base DN
	baseDN = normalizeDN(baseDN)

	// Get snapshot info
	var snapshot uint64
	var activeTxID uint64
	if txn, ok := txnIface.(*tx.Transaction); ok && txn != nil {
		snapshot = txn.Snapshot
		activeTxID = txn.ID
	} else {
		snapshot = db.snapshotManager.CurrentTimestamp()
	}

	// Create radix tree iterator for subtree scope
	radixIter, err := db.radixTree.Iterator(baseDN, radix.ScopeSubtree)
	if err != nil {
		return &errorIterator{err: err}
	}

	// Create filter matcher function
	var filterMatcher storage.FilterMatcher
	if f != nil {
		if matcher, ok := f.(storage.FilterMatcher); ok {
			filterMatcher = matcher
		}
	}

	return &filterIterator{
		db:            db,
		radixIter:     radixIter,
		filterMatcher: filterMatcher,
		snapshot:      snapshot,
		activeTxID:    activeTxID,
	}
}

// CreateIndex creates a new index for the given attribute.
func (db *ObaDB) CreateIndex(attribute string, indexType storage.IndexType) error {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.closed {
		return ErrDatabaseClosed
	}

	if db.readOnly {
		return ErrDatabaseReadOnly
	}

	return db.indexManager.CreateIndex(attribute, index.IndexType(indexType))
}

// DropIndex removes an index for the given attribute.
func (db *ObaDB) DropIndex(attribute string) error {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.closed {
		return ErrDatabaseClosed
	}

	if db.readOnly {
		return ErrDatabaseReadOnly
	}

	return db.indexManager.DropIndex(attribute)
}

// Checkpoint performs a checkpoint operation.
func (db *ObaDB) Checkpoint() error {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.closed {
		return ErrDatabaseClosed
	}

	if db.readOnly {
		return ErrDatabaseReadOnly
	}

	if db.checkpointManager == nil {
		return nil
	}

	// Persist radix tree first
	if err := db.radixTree.Persist(); err != nil {
		return err
	}

	// Sync index manager
	if err := db.indexManager.Sync(); err != nil {
		return err
	}

	// Save caches for faster startup
	db.saveCaches()

	return db.checkpointManager.Checkpoint()
}

// Compact compacts the database to reclaim space.
func (db *ObaDB) Compact() error {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.closed {
		return ErrDatabaseClosed
	}

	if db.readOnly {
		return ErrDatabaseReadOnly
	}

	// Trigger garbage collection
	if db.gc != nil {
		_, err := db.gc.TriggerCollect()
		if err != nil {
			return err
		}
	}

	// Truncate WAL after checkpoint
	if db.checkpointManager != nil {
		if err := db.Checkpoint(); err != nil {
			return err
		}
		return db.checkpointManager.TruncateWAL()
	}

	return nil
}

// Stats returns statistics about the storage engine.
func (db *ObaDB) Stats() *storage.EngineStats {
	db.mu.RLock()
	defer db.mu.RUnlock()

	stats := &storage.EngineStats{}

	if db.closed {
		return stats
	}

	// Page manager stats
	if db.pageManager != nil {
		pmStats := db.pageManager.Stats()
		stats.TotalPages = pmStats.TotalPages
		stats.FreePages = pmStats.FreePages
		stats.UsedPages = pmStats.UsedPages
	}

	// Entry count from radix tree
	if db.radixTree != nil {
		stats.EntryCount = uint64(db.radixTree.EntryCount())
	}

	// Index count
	if db.indexManager != nil {
		stats.IndexCount = db.indexManager.IndexCount()
	}

	// Active transactions
	if db.txManager != nil {
		stats.ActiveTransactions = db.txManager.ActiveCount()
	}

	// Buffer pool stats
	if db.bufferPool != nil {
		bpStats := db.bufferPool.Stats()
		stats.BufferPoolSize = bpStats.Size
		stats.DirtyPages = bpStats.DirtyPages
	}

	// Checkpoint LSN
	if db.checkpointManager != nil {
		stats.LastCheckpointLSN = db.checkpointManager.LastCheckpointLSN()
	}

	return stats
}

// updateIndexes updates indexes when an entry is modified.
func (db *ObaDB) updateIndexes(oldEntry, newEntry *storage.Entry) error {
	if db.indexManager == nil {
		return nil
	}

	var oldIndexEntry, newIndexEntry *index.Entry

	if oldEntry != nil {
		oldIndexEntry = &index.Entry{
			DN:         oldEntry.DN,
			Attributes: oldEntry.Attributes,
		}
	}

	if newEntry != nil {
		newIndexEntry = &index.Entry{
			DN:         newEntry.DN,
			Attributes: newEntry.Attributes,
		}
	}

	return db.indexManager.UpdateIndexes(oldIndexEntry, newIndexEntry)
}

// normalizeDN normalizes a DN for consistent storage and lookup.
func normalizeDN(dn string) string {
	return strings.TrimSpace(strings.ToLower(dn))
}

// serializeEntry serializes an entry to bytes.
func serializeEntry(entry *storage.Entry) ([]byte, error) {
	if entry == nil {
		return nil, ErrInvalidEntry
	}

	size := 4 + len(entry.DN) + 4
	for name, values := range entry.Attributes {
		size += 2 + len(name) + 4
		for _, v := range values {
			size += 4 + len(v)
		}
	}

	buf := make([]byte, size)
	offset := 0

	binary.LittleEndian.PutUint32(buf[offset:], uint32(len(entry.DN)))
	offset += 4
	copy(buf[offset:], entry.DN)
	offset += len(entry.DN)

	binary.LittleEndian.PutUint32(buf[offset:], uint32(len(entry.Attributes)))
	offset += 4

	for name, values := range entry.Attributes {
		binary.LittleEndian.PutUint16(buf[offset:], uint16(len(name)))
		offset += 2
		copy(buf[offset:], name)
		offset += len(name)

		binary.LittleEndian.PutUint32(buf[offset:], uint32(len(values)))
		offset += 4

		for _, v := range values {
			binary.LittleEndian.PutUint32(buf[offset:], uint32(len(v)))
			offset += 4
			copy(buf[offset:], v)
			offset += len(v)
		}
	}

	return buf, nil
}

// deserializeEntry deserializes an entry from bytes.
func deserializeEntry(dn string, data []byte) (*storage.Entry, error) {
	if len(data) < 8 {
		return nil, ErrInvalidEntry
	}

	entry := &storage.Entry{
		DN:         dn,
		Attributes: make(map[string][][]byte),
	}

	offset := 0
	dnLen := binary.LittleEndian.Uint32(data[offset:])
	offset += 4 + int(dnLen)

	if offset+4 > len(data) {
		return entry, nil
	}

	attrCount := binary.LittleEndian.Uint32(data[offset:])
	offset += 4

	for i := uint32(0); i < attrCount && offset < len(data); i++ {
		if offset+2 > len(data) {
			break
		}

		nameLen := binary.LittleEndian.Uint16(data[offset:])
		offset += 2

		if offset+int(nameLen) > len(data) {
			break
		}

		name := string(data[offset : offset+int(nameLen)])
		offset += int(nameLen)

		if offset+4 > len(data) {
			break
		}

		valueCount := binary.LittleEndian.Uint32(data[offset:])
		offset += 4

		values := make([][]byte, 0, valueCount)

		for j := uint32(0); j < valueCount && offset < len(data); j++ {
			if offset+4 > len(data) {
				break
			}

			valueLen := binary.LittleEndian.Uint32(data[offset:])
			offset += 4

			if offset+int(valueLen) > len(data) {
				break
			}

			value := make([]byte, valueLen)
			copy(value, data[offset:offset+int(valueLen)])
			offset += int(valueLen)

			values = append(values, value)
		}

		entry.Attributes[name] = values
	}

	return entry, nil
}

// errorIterator is an iterator that returns an error.
type errorIterator struct {
	err error
}

func (it *errorIterator) Next() bool            { return false }
func (it *errorIterator) Entry() *storage.Entry { return nil }
func (it *errorIterator) Error() error          { return it.err }
func (it *errorIterator) Close()                {}

// dnIterator iterates over entries by DN.
type dnIterator struct {
	db         *ObaDB
	radixIter  *radix.RadixIterator
	snapshot   uint64
	activeTxID uint64
	current    *storage.Entry
	err        error
}

func (it *dnIterator) Next() bool {
	for {
		dn, _, _, ok := it.radixIter.Next()
		if !ok {
			return false
		}

		version, err := it.db.versionStore.GetVisibleForTx(dn, it.snapshot, it.activeTxID)
		if err != nil {
			continue
		}

		entry, err := deserializeEntry(dn, version.GetData())
		if err != nil {
			it.err = err
			return false
		}

		it.current = entry
		return true
	}
}

func (it *dnIterator) Entry() *storage.Entry { return it.current }
func (it *dnIterator) Error() error          { return it.err }
func (it *dnIterator) Close()                { it.radixIter.Close() }

// filterIterator iterates over entries matching a filter.
type filterIterator struct {
	db            *ObaDB
	radixIter     *radix.RadixIterator
	filterMatcher storage.FilterMatcher
	snapshot      uint64
	activeTxID    uint64
	current       *storage.Entry
	err           error
}

func (it *filterIterator) Next() bool {
	for {
		dn, _, _, ok := it.radixIter.Next()
		if !ok {
			return false
		}

		version, err := it.db.versionStore.GetVisibleForTx(dn, it.snapshot, it.activeTxID)
		if err != nil {
			continue
		}

		entry, err := deserializeEntry(dn, version.GetData())
		if err != nil {
			it.err = err
			return false
		}

		if it.filterMatcher != nil {
			if !it.filterMatcher.Match(entry) {
				continue
			}
		}

		it.current = entry
		return true
	}
}

func (it *filterIterator) Entry() *storage.Entry { return it.current }
func (it *filterIterator) Error() error          { return it.err }
func (it *filterIterator) Close()                { it.radixIter.Close() }

// getLastTxID returns the last transaction ID for cache validation.
func (db *ObaDB) getLastTxID() uint64 {
	if db.wal != nil {
		return db.wal.CurrentLSN()
	}
	return 0
}

// saveCaches saves index caches for faster startup (acquires lock).
func (db *ObaDB) saveCaches() {
	db.saveCachesInternal()
}

// saveCachesInternal saves index caches without acquiring lock.
func (db *ObaDB) saveCachesInternal() {
	cacheDir := filepath.Join(db.path, CacheDir)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return
	}

	txID := db.getLastTxID()

	// Save radix tree cache
	radixCachePath := filepath.Join(cacheDir, RadixCacheFileName)
	if db.radixTree != nil {
		db.radixTree.SaveCache(radixCachePath, txID)
	}

	// Save index cache
	btreeCachePath := filepath.Join(cacheDir, BTreeCacheFileName)
	if db.indexManager != nil {
		db.indexManager.SaveCache(btreeCachePath, txID)
	}

	// Save entry cache
	entryCachePath := filepath.Join(cacheDir, EntryCacheFileName)
	if db.versionStore != nil {
		db.versionStore.SaveCache(entryCachePath, txID)
	}
}

// Ensure ObaDB implements StorageEngine interface.
var _ storage.StorageEngine = (*ObaDB)(nil)
