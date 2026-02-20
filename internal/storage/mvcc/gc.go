// Package mvcc provides Multi-Version Concurrency Control for ObaDB.
package mvcc

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/oba-ldap/oba/internal/storage"
)

// GC errors.
var (
	ErrGCAlreadyRunning = errors.New("garbage collector is already running")
	ErrGCNotRunning     = errors.New("garbage collector is not running")
	ErrGCClosed         = errors.New("garbage collector is closed")
	ErrEntryNotFound    = errors.New("entry not found")
)

// DefaultGCInterval is the default interval between GC runs.
const DefaultGCInterval = 30 * time.Second

// GCConfig holds configuration options for the GarbageCollector.
type GCConfig struct {
	// Interval is the time between automatic GC runs.
	Interval time.Duration

	// MinVersionAge is the minimum age of versions before they can be collected.
	// This provides a safety margin to avoid collecting versions that might
	// still be needed by very recent transactions.
	MinVersionAge time.Duration

	// BatchSize is the maximum number of entries to process per GC cycle.
	// 0 means no limit.
	BatchSize int
}

// DefaultGCConfig returns the default GC configuration.
func DefaultGCConfig() GCConfig {
	return GCConfig{
		Interval:      DefaultGCInterval,
		MinVersionAge: 0,
		BatchSize:     0,
	}
}

// GarbageCollector reclaims space from old versions that are no longer
// visible to any active transaction. This prevents unbounded storage
// growth from MVCC versioning.
//
// GC algorithm:
// 1. Find oldest active snapshot timestamp
// 2. For each entry with version chain:
//   - Find versions older than the oldest snapshot
//   - Remove versions that are no longer visible to any snapshot
//
// 3. Free pages containing only garbage
// 4. Update free list
type GarbageCollector struct {
	// versionStore manages version chains for all entries.
	versionStore *VersionStore

	// snapshotManager tracks active snapshots.
	snapshotManager *SnapshotManager

	// pageManager handles page allocation and I/O.
	pageManager *storage.PageManager

	// config holds GC configuration.
	config GCConfig

	// running indicates if the background GC is running.
	running int32

	// stopCh signals the background GC to stop.
	stopCh chan struct{}

	// doneCh signals that the background GC has stopped.
	doneCh chan struct{}

	// stats tracks GC statistics.
	stats GCStats

	// mu protects concurrent access.
	mu sync.RWMutex

	// closed indicates if the GC has been closed.
	closed bool
}

// GCStats holds statistics about garbage collection.
type GCStats struct {
	// TotalRuns is the total number of GC runs.
	TotalRuns uint64

	// TotalVersionsCollected is the total number of versions collected.
	TotalVersionsCollected uint64

	// TotalPagesFreed is the total number of pages freed.
	TotalPagesFreed uint64

	// TotalEntriesProcessed is the total number of entries processed.
	TotalEntriesProcessed uint64

	// LastRunTime is the timestamp of the last GC run.
	LastRunTime time.Time

	// LastRunDuration is the duration of the last GC run.
	LastRunDuration time.Duration

	// LastVersionsCollected is the number of versions collected in the last run.
	LastVersionsCollected int

	// LastPagesFreed is the number of pages freed in the last run.
	LastPagesFreed int
}

// NewGarbageCollector creates a new GarbageCollector with the given dependencies.
func NewGarbageCollector(vs *VersionStore, sm *SnapshotManager, pm *storage.PageManager) *GarbageCollector {
	return NewGarbageCollectorWithConfig(vs, sm, pm, DefaultGCConfig())
}

// NewGarbageCollectorWithConfig creates a new GarbageCollector with custom configuration.
func NewGarbageCollectorWithConfig(vs *VersionStore, sm *SnapshotManager, pm *storage.PageManager, config GCConfig) *GarbageCollector {
	if config.Interval <= 0 {
		config.Interval = DefaultGCInterval
	}

	return &GarbageCollector{
		versionStore:    vs,
		snapshotManager: sm,
		pageManager:     pm,
		config:          config,
		running:         0,
		stopCh:          nil,
		doneCh:          nil,
		stats:           GCStats{},
		closed:          false,
	}
}

// Start starts the background garbage collection process.
// GC runs periodically at the configured interval.
func (gc *GarbageCollector) Start() error {
	gc.mu.Lock()
	defer gc.mu.Unlock()

	if gc.closed {
		return ErrGCClosed
	}

	if atomic.LoadInt32(&gc.running) == 1 {
		return ErrGCAlreadyRunning
	}

	stopCh := make(chan struct{})
	doneCh := make(chan struct{})
	gc.stopCh = stopCh
	gc.doneCh = doneCh
	atomic.StoreInt32(&gc.running, 1)

	go gc.runBackground(stopCh, doneCh)

	return nil
}

// Stop stops the background garbage collection process.
// It waits for the current GC cycle to complete before returning.
func (gc *GarbageCollector) Stop() error {
	gc.mu.Lock()
	if gc.closed {
		gc.mu.Unlock()
		return ErrGCClosed
	}

	// Use CAS to ensure only one goroutine can stop the GC
	if !atomic.CompareAndSwapInt32(&gc.running, 1, 0) {
		gc.mu.Unlock()
		return ErrGCNotRunning
	}

	stopCh := gc.stopCh
	doneCh := gc.doneCh
	gc.stopCh = nil
	gc.doneCh = nil
	gc.mu.Unlock()

	// Signal stop
	close(stopCh)

	// Wait for background goroutine to finish
	<-doneCh

	return nil
}

// runBackground runs the GC loop in the background.
func (gc *GarbageCollector) runBackground(stopCh <-chan struct{}, doneCh chan<- struct{}) {
	defer close(doneCh)

	ticker := time.NewTicker(gc.config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			// Run GC cycle
			_, _ = gc.Collect()
		}
	}
}

// Collect performs a garbage collection cycle.
// It identifies and removes old versions that are no longer visible
// to any active snapshot.
// Returns the number of pages freed and any error encountered.
func (gc *GarbageCollector) Collect() (int, error) {
	gc.mu.RLock()
	if gc.closed {
		gc.mu.RUnlock()
		return 0, ErrGCClosed
	}
	gc.mu.RUnlock()

	startTime := time.Now()

	// Get the oldest active snapshot timestamp
	oldestSnapshot := gc.getOldestVisibleTimestamp()

	// If there are no active snapshots, use the current timestamp
	// This means all committed versions except the latest can be collected
	if oldestSnapshot == 0 && gc.snapshotManager != nil {
		oldestSnapshot = gc.snapshotManager.CurrentTimestamp()
	}

	// Collect garbage from the version store
	versionsCollected := 0
	pagesFreed := 0
	entriesProcessed := 0

	if gc.versionStore != nil {
		versionsCollected = gc.versionStore.GarbageCollect(oldestSnapshot)
		entriesProcessed = gc.versionStore.EntryCount()
	}

	// Collect pages that are no longer referenced
	pagesFreed = gc.collectUnreferencedPages(oldestSnapshot)

	// Update statistics
	duration := time.Since(startTime)
	gc.updateStats(versionsCollected, pagesFreed, entriesProcessed, duration)

	return pagesFreed, nil
}

// CollectEntry performs garbage collection on a specific entry.
// This is useful for targeted cleanup after deleting an entry.
func (gc *GarbageCollector) CollectEntry(dn string) error {
	gc.mu.RLock()
	if gc.closed {
		gc.mu.RUnlock()
		return ErrGCClosed
	}
	gc.mu.RUnlock()

	if gc.versionStore == nil {
		return nil
	}

	// Check if the entry exists
	if !gc.versionStore.HasEntry(dn) {
		return ErrEntryNotFound
	}

	// Get the oldest active snapshot timestamp
	oldestSnapshot := gc.getOldestVisibleTimestamp()
	if oldestSnapshot == 0 && gc.snapshotManager != nil {
		oldestSnapshot = gc.snapshotManager.CurrentTimestamp()
	}

	// Collect garbage for this specific entry
	gc.collectEntryVersions(dn, oldestSnapshot)

	return nil
}

// collectEntryVersions collects old versions for a specific entry.
func (gc *GarbageCollector) collectEntryVersions(dn string, oldestSnapshot uint64) int {
	if gc.versionStore == nil {
		return 0
	}

	// Get the version chain for this entry
	chain := gc.versionStore.GetVersionChain(dn)
	if len(chain) <= 1 {
		// No old versions to collect
		return 0
	}

	// Find versions that can be collected
	// A version can be collected if:
	// 1. It is committed (CommitTS > 0)
	// 2. Its CommitTS < oldestSnapshot
	// 3. There is a newer committed version visible to the oldest snapshot

	collected := 0
	pagesToFree := make([]storage.PageID, 0)

	// Find the first version visible to the oldest snapshot
	var visibleVersion *Version
	for _, v := range chain {
		if v.IsCommitted() && v.GetCommitTS() <= oldestSnapshot {
			visibleVersion = v
			break
		}
	}

	if visibleVersion == nil {
		return 0
	}

	// Collect all versions older than the visible version
	foundVisible := false
	for _, v := range chain {
		if v == visibleVersion {
			foundVisible = true
			continue
		}
		if foundVisible && v.IsCommitted() {
			// This version is older than the visible version and can be collected
			pageID, _ := v.GetLocation()
			if pageID != 0 {
				pagesToFree = append(pagesToFree, pageID)
			}
			collected++
		}
	}

	// Free the pages
	for _, pageID := range pagesToFree {
		gc.freePage(pageID)
	}

	return collected
}

// getOldestVisibleTimestamp returns the oldest timestamp that is still visible
// to any active snapshot.
func (gc *GarbageCollector) getOldestVisibleTimestamp() uint64 {
	if gc.snapshotManager == nil {
		return 0
	}

	return gc.snapshotManager.GetOldestActiveSnapshot()
}

// collectUnreferencedPages identifies and frees pages that are no longer
// referenced by any version.
func (gc *GarbageCollector) collectUnreferencedPages(oldestSnapshot uint64) int {
	// This is a simplified implementation.
	// In a full implementation, we would:
	// 1. Build a set of all pages referenced by current versions
	// 2. Compare against all allocated pages
	// 3. Free pages that are not referenced

	// For now, we rely on the version store's GarbageCollect method
	// to handle page cleanup as part of version cleanup.
	return 0
}

// freePage frees a page and adds it to the free list.
func (gc *GarbageCollector) freePage(pageID storage.PageID) error {
	if gc.pageManager == nil {
		return nil
	}

	return gc.pageManager.FreePage(pageID)
}

// updateStats updates the GC statistics.
func (gc *GarbageCollector) updateStats(versionsCollected, pagesFreed, entriesProcessed int, duration time.Duration) {
	gc.mu.Lock()
	defer gc.mu.Unlock()

	gc.stats.TotalRuns++
	gc.stats.TotalVersionsCollected += uint64(versionsCollected)
	gc.stats.TotalPagesFreed += uint64(pagesFreed)
	gc.stats.TotalEntriesProcessed += uint64(entriesProcessed)
	gc.stats.LastRunTime = time.Now()
	gc.stats.LastRunDuration = duration
	gc.stats.LastVersionsCollected = versionsCollected
	gc.stats.LastPagesFreed = pagesFreed
}

// Stats returns the current GC statistics.
func (gc *GarbageCollector) Stats() GCStats {
	gc.mu.RLock()
	defer gc.mu.RUnlock()
	return gc.stats
}

// IsRunning returns true if the background GC is running.
func (gc *GarbageCollector) IsRunning() bool {
	return atomic.LoadInt32(&gc.running) == 1
}

// GetConfig returns the current GC configuration.
func (gc *GarbageCollector) GetConfig() GCConfig {
	gc.mu.RLock()
	defer gc.mu.RUnlock()
	return gc.config
}

// SetInterval updates the GC interval.
// This takes effect on the next GC cycle.
func (gc *GarbageCollector) SetInterval(interval time.Duration) {
	gc.mu.Lock()
	defer gc.mu.Unlock()
	gc.config.Interval = interval
}

// Close stops the GC and releases resources.
func (gc *GarbageCollector) Close() error {
	gc.mu.Lock()
	if gc.closed {
		gc.mu.Unlock()
		return nil
	}
	gc.closed = true

	// Check if running and stop if needed using CAS
	if atomic.CompareAndSwapInt32(&gc.running, 1, 0) {
		stopCh := gc.stopCh
		doneCh := gc.doneCh
		gc.stopCh = nil
		gc.doneCh = nil
		gc.mu.Unlock()

		// Signal stop
		close(stopCh)

		// Wait for background goroutine to finish
		<-doneCh

		return nil
	}

	gc.mu.Unlock()
	return nil
}

// TriggerCollect triggers an immediate GC cycle without waiting for the interval.
// This is useful for testing or when immediate cleanup is needed.
func (gc *GarbageCollector) TriggerCollect() (int, error) {
	return gc.Collect()
}

// GetVersionStore returns the version store (for testing).
func (gc *GarbageCollector) GetVersionStore() *VersionStore {
	return gc.versionStore
}

// GetSnapshotManager returns the snapshot manager (for testing).
func (gc *GarbageCollector) GetSnapshotManager() *SnapshotManager {
	return gc.snapshotManager
}

// GetPageManager returns the page manager (for testing).
func (gc *GarbageCollector) GetPageManager() *storage.PageManager {
	return gc.pageManager
}
