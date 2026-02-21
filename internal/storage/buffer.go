// Package storage provides the core storage engine components for ObaDB.
package storage

import (
	"errors"
	"sync"
)

// Buffer pool errors.
var (
	ErrBufferPoolFull   = errors.New("buffer pool is full and no pages can be evicted")
	ErrPageNotFound     = errors.New("page not found in buffer pool")
	ErrPagePinned       = errors.New("page is pinned and cannot be evicted")
	ErrInvalidCapacity  = errors.New("buffer pool capacity must be positive")
	ErrNegativePinCount = errors.New("pin count cannot be negative")
)

// BufferPage represents a page cached in the buffer pool.
type BufferPage struct {
	id       PageID
	data     []byte
	dirty    bool
	pinCount int
}

// ID returns the page ID.
func (bp *BufferPage) ID() PageID {
	return bp.id
}

// Data returns the page data.
func (bp *BufferPage) Data() []byte {
	return bp.data
}

// IsDirty returns true if the page has been modified.
func (bp *BufferPage) IsDirty() bool {
	return bp.dirty
}

// PinCount returns the current pin count.
func (bp *BufferPage) PinCount() int {
	return bp.pinCount
}

// BufferPool manages a pool of cached pages with LRU eviction policy.
// It provides thread-safe access to pages and ensures dirty pages are
// written before eviction.
type BufferPool struct {
	capacity   int
	pageSize   int
	pages      map[PageID]*BufferPage
	lru        *LRUCache
	dirtyPages map[PageID]bool
	mu         sync.RWMutex

	// Callback for flushing dirty pages before eviction
	flushCallback func(pageID PageID, data []byte) error
}

// NewBufferPool creates a new buffer pool with the specified capacity and page size.
func NewBufferPool(capacity int, pageSize int) *BufferPool {
	if capacity <= 0 {
		capacity = 16 // Default capacity
	}
	if pageSize <= 0 {
		pageSize = PageSize // Default page size
	}

	return &BufferPool{
		capacity:   capacity,
		pageSize:   pageSize,
		pages:      make(map[PageID]*BufferPage),
		lru:        NewLRUCache(),
		dirtyPages: make(map[PageID]bool),
	}
}

// SetFlushCallback sets the callback function for flushing dirty pages.
// This callback is invoked before a dirty page is evicted.
func (bp *BufferPool) SetFlushCallback(callback func(pageID PageID, data []byte) error) {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	bp.flushCallback = callback
}

// Get retrieves a page from the buffer pool.
// Returns the page and true if found, nil and false otherwise.
// Accessing a page marks it as recently used.
func (bp *BufferPool) Get(id PageID) (*BufferPage, bool) {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	page, exists := bp.pages[id]
	if !exists {
		return nil, false
	}

	// Mark as recently accessed
	bp.lru.Access(id)

	return page, true
}

// Put adds or updates a page in the buffer pool.
// If the pool is at capacity, it will attempt to evict a page first.
// Returns the buffer page.
func (bp *BufferPool) Put(id PageID, data []byte) (*BufferPage, error) {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	// Check if page already exists
	if page, exists := bp.pages[id]; exists {
		// Update existing page
		copy(page.data, data)
		bp.lru.Access(id)
		return page, nil
	}

	// Check if we need to evict
	if len(bp.pages) >= bp.capacity {
		if err := bp.evictOneLocked(); err != nil {
			return nil, err
		}
	}

	// Create new buffer page
	pageData := make([]byte, bp.pageSize)
	if len(data) > 0 {
		copy(pageData, data)
	}

	page := &BufferPage{
		id:       id,
		data:     pageData,
		dirty:    false,
		pinCount: 0,
	}

	bp.pages[id] = page
	bp.lru.Access(id)

	return page, nil
}

// Pin increments the pin count for a page, preventing it from being evicted.
func (bp *BufferPool) Pin(id PageID) error {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	page, exists := bp.pages[id]
	if !exists {
		return ErrPageNotFound
	}

	page.pinCount++
	return nil
}

// Unpin decrements the pin count for a page.
func (bp *BufferPool) Unpin(id PageID) error {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	page, exists := bp.pages[id]
	if !exists {
		return ErrPageNotFound
	}

	if page.pinCount <= 0 {
		return ErrNegativePinCount
	}

	page.pinCount--
	return nil
}

// MarkDirty marks a page as dirty (modified).
func (bp *BufferPool) MarkDirty(id PageID) error {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	page, exists := bp.pages[id]
	if !exists {
		return ErrPageNotFound
	}

	page.dirty = true
	bp.dirtyPages[id] = true
	return nil
}

// FlushAll writes all dirty pages using the flush callback.
func (bp *BufferPool) FlushAll() error {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	return bp.flushAllLocked()
}

// flushAllLocked writes all dirty pages. Must be called with lock held.
func (bp *BufferPool) flushAllLocked() error {
	if bp.flushCallback == nil {
		// No callback set, just clear dirty flags
		for id := range bp.dirtyPages {
			if page, exists := bp.pages[id]; exists {
				page.dirty = false
			}
		}
		bp.dirtyPages = make(map[PageID]bool)
		return nil
	}

	for id := range bp.dirtyPages {
		page, exists := bp.pages[id]
		if !exists {
			continue
		}

		if err := bp.flushCallback(id, page.data); err != nil {
			return err
		}

		page.dirty = false
	}

	bp.dirtyPages = make(map[PageID]bool)
	return nil
}

// FlushPage writes a specific dirty page using the flush callback.
func (bp *BufferPool) FlushPage(id PageID) error {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	page, exists := bp.pages[id]
	if !exists {
		return ErrPageNotFound
	}

	if !page.dirty {
		return nil // Not dirty, nothing to flush
	}

	if bp.flushCallback != nil {
		if err := bp.flushCallback(id, page.data); err != nil {
			return err
		}
	}

	page.dirty = false
	delete(bp.dirtyPages, id)
	return nil
}

// Evict attempts to evict the least recently used unpinned page.
// Returns the evicted page ID, its data, and true if successful.
// If the page was dirty, it will be flushed before eviction.
func (bp *BufferPool) Evict() (PageID, []byte, bool) {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	pageID, data, ok := bp.evictLocked()
	return pageID, data, ok
}

// evictLocked performs eviction without locking. Must be called with lock held.
func (bp *BufferPool) evictLocked() (PageID, []byte, bool) {
	// Build set of pinned pages
	pinnedPages := make(map[PageID]bool)
	for id, page := range bp.pages {
		if page.pinCount > 0 {
			pinnedPages[id] = true
		}
	}

	// Find LRU page that is not pinned
	pageID, found := bp.lru.GetLRUExcluding(pinnedPages)
	if !found {
		return 0, nil, false
	}

	page := bp.pages[pageID]
	if page == nil {
		return 0, nil, false
	}

	// Flush if dirty
	if page.dirty && bp.flushCallback != nil {
		if err := bp.flushCallback(pageID, page.data); err != nil {
			// Eviction failed due to flush error
			return 0, nil, false
		}
	}

	// Copy data before removing
	data := make([]byte, len(page.data))
	copy(data, page.data)

	// Remove from pool
	delete(bp.pages, pageID)
	delete(bp.dirtyPages, pageID)
	bp.lru.Remove(pageID)

	return pageID, data, true
}

// evictOneLocked evicts one page to make room. Must be called with lock held.
func (bp *BufferPool) evictOneLocked() error {
	_, _, ok := bp.evictLocked()
	if !ok {
		return ErrBufferPoolFull
	}
	return nil
}

// Remove removes a page from the buffer pool.
// If the page is dirty, it will be flushed first.
// If the page is pinned, an error is returned.
func (bp *BufferPool) Remove(id PageID) error {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	page, exists := bp.pages[id]
	if !exists {
		return ErrPageNotFound
	}

	if page.pinCount > 0 {
		return ErrPagePinned
	}

	// Flush if dirty
	if page.dirty && bp.flushCallback != nil {
		if err := bp.flushCallback(id, page.data); err != nil {
			return err
		}
	}

	delete(bp.pages, id)
	delete(bp.dirtyPages, id)
	bp.lru.Remove(id)

	return nil
}

// Contains checks if a page is in the buffer pool.
func (bp *BufferPool) Contains(id PageID) bool {
	bp.mu.RLock()
	defer bp.mu.RUnlock()
	_, exists := bp.pages[id]
	return exists
}

// Size returns the number of pages currently in the buffer pool.
func (bp *BufferPool) Size() int {
	bp.mu.RLock()
	defer bp.mu.RUnlock()
	return len(bp.pages)
}

// Capacity returns the maximum capacity of the buffer pool.
func (bp *BufferPool) Capacity() int {
	return bp.capacity
}

// DirtyPageCount returns the number of dirty pages in the buffer pool.
func (bp *BufferPool) DirtyPageCount() int {
	bp.mu.RLock()
	defer bp.mu.RUnlock()
	return len(bp.dirtyPages)
}

// Clear removes all pages from the buffer pool.
// Dirty pages are flushed first if a callback is set.
func (bp *BufferPool) Clear() error {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	// Flush all dirty pages first
	if err := bp.flushAllLocked(); err != nil {
		return err
	}

	bp.pages = make(map[PageID]*BufferPage)
	bp.dirtyPages = make(map[PageID]bool)
	bp.lru.Clear()

	return nil
}

// BufferPoolStats contains statistics about the buffer pool.
type BufferPoolStats struct {
	Capacity    int
	Size        int
	DirtyPages  int
	PinnedPages int
}

// Stats returns current statistics about the buffer pool.
func (bp *BufferPool) Stats() BufferPoolStats {
	bp.mu.RLock()
	defer bp.mu.RUnlock()

	pinnedCount := 0
	for _, page := range bp.pages {
		if page.pinCount > 0 {
			pinnedCount++
		}
	}

	return BufferPoolStats{
		Capacity:    bp.capacity,
		Size:        len(bp.pages),
		DirtyPages:  len(bp.dirtyPages),
		PinnedPages: pinnedCount,
	}
}

// GetAllPageIDs returns all page IDs currently in the buffer pool.
func (bp *BufferPool) GetAllPageIDs() []PageID {
	bp.mu.RLock()
	defer bp.mu.RUnlock()

	ids := make([]PageID, 0, len(bp.pages))
	for id := range bp.pages {
		ids = append(ids, id)
	}
	return ids
}

// GetDirtyPageIDs returns all dirty page IDs.
func (bp *BufferPool) GetDirtyPageIDs() []PageID {
	bp.mu.RLock()
	defer bp.mu.RUnlock()

	ids := make([]PageID, 0, len(bp.dirtyPages))
	for id := range bp.dirtyPages {
		ids = append(ids, id)
	}
	return ids
}
