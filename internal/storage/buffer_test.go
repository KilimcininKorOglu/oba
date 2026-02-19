package storage

import (
	"sync"
	"sync/atomic"
	"testing"
)

// =============================================================================
// LRU Cache Tests
// =============================================================================

func TestNewLRUCache(t *testing.T) {
	cache := NewLRUCache()

	if cache == nil {
		t.Fatal("NewLRUCache returned nil")
	}
	if cache.Len() != 0 {
		t.Errorf("New cache should be empty, got %d", cache.Len())
	}
}

func TestLRUCacheAccess(t *testing.T) {
	cache := NewLRUCache()

	// Add pages
	cache.Access(1)
	cache.Access(2)
	cache.Access(3)

	if cache.Len() != 3 {
		t.Errorf("Cache should have 3 entries, got %d", cache.Len())
	}

	// Verify LRU order (1 should be LRU since it was accessed first)
	lru, ok := cache.GetLRU()
	if !ok {
		t.Fatal("GetLRU should return true")
	}
	if lru != 1 {
		t.Errorf("LRU should be 1, got %d", lru)
	}

	// Access 1 again, now 2 should be LRU
	cache.Access(1)
	lru, _ = cache.GetLRU()
	if lru != 2 {
		t.Errorf("LRU should be 2 after accessing 1, got %d", lru)
	}
}

func TestLRUCacheRemove(t *testing.T) {
	cache := NewLRUCache()

	cache.Access(1)
	cache.Access(2)
	cache.Access(3)

	cache.Remove(2)

	if cache.Len() != 2 {
		t.Errorf("Cache should have 2 entries after removal, got %d", cache.Len())
	}
	if cache.Contains(2) {
		t.Error("Cache should not contain removed page")
	}
}

func TestLRUCacheGetLRUExcluding(t *testing.T) {
	cache := NewLRUCache()

	cache.Access(1)
	cache.Access(2)
	cache.Access(3)

	// Exclude page 1 (the LRU)
	excluded := map[PageID]bool{1: true}
	lru, ok := cache.GetLRUExcluding(excluded)
	if !ok {
		t.Fatal("GetLRUExcluding should return true")
	}
	if lru != 2 {
		t.Errorf("LRU excluding 1 should be 2, got %d", lru)
	}

	// Exclude all pages
	excluded = map[PageID]bool{1: true, 2: true, 3: true}
	_, ok = cache.GetLRUExcluding(excluded)
	if ok {
		t.Error("GetLRUExcluding should return false when all pages excluded")
	}
}

func TestLRUCacheContains(t *testing.T) {
	cache := NewLRUCache()

	cache.Access(1)

	if !cache.Contains(1) {
		t.Error("Cache should contain page 1")
	}
	if cache.Contains(2) {
		t.Error("Cache should not contain page 2")
	}
}

func TestLRUCacheClear(t *testing.T) {
	cache := NewLRUCache()

	cache.Access(1)
	cache.Access(2)
	cache.Access(3)

	cache.Clear()

	if cache.Len() != 0 {
		t.Errorf("Cache should be empty after clear, got %d", cache.Len())
	}
}

func TestLRUCacheGetAll(t *testing.T) {
	cache := NewLRUCache()

	cache.Access(1)
	cache.Access(2)
	cache.Access(3)

	// GetAll returns MRU to LRU order
	all := cache.GetAll()
	if len(all) != 3 {
		t.Fatalf("GetAll should return 3 entries, got %d", len(all))
	}
	// Most recently used is 3
	if all[0] != 3 {
		t.Errorf("First entry should be 3 (MRU), got %d", all[0])
	}
	// Least recently used is 1
	if all[2] != 1 {
		t.Errorf("Last entry should be 1 (LRU), got %d", all[2])
	}
}

func TestLRUCacheGetAllLRUOrder(t *testing.T) {
	cache := NewLRUCache()

	cache.Access(1)
	cache.Access(2)
	cache.Access(3)

	// GetAllLRUOrder returns LRU to MRU order
	all := cache.GetAllLRUOrder()
	if len(all) != 3 {
		t.Fatalf("GetAllLRUOrder should return 3 entries, got %d", len(all))
	}
	// Least recently used is 1
	if all[0] != 1 {
		t.Errorf("First entry should be 1 (LRU), got %d", all[0])
	}
	// Most recently used is 3
	if all[2] != 3 {
		t.Errorf("Last entry should be 3 (MRU), got %d", all[2])
	}
}

func TestLRUCacheEmptyGetLRU(t *testing.T) {
	cache := NewLRUCache()

	_, ok := cache.GetLRU()
	if ok {
		t.Error("GetLRU on empty cache should return false")
	}
}

// =============================================================================
// Buffer Pool Tests
// =============================================================================

func TestNewBufferPool(t *testing.T) {
	bp := NewBufferPool(10, PageSize)

	if bp == nil {
		t.Fatal("NewBufferPool returned nil")
	}
	if bp.Capacity() != 10 {
		t.Errorf("Capacity should be 10, got %d", bp.Capacity())
	}
	if bp.Size() != 0 {
		t.Errorf("Size should be 0, got %d", bp.Size())
	}
}

func TestNewBufferPoolDefaults(t *testing.T) {
	// Test with invalid values
	bp := NewBufferPool(0, 0)

	if bp.Capacity() != 16 {
		t.Errorf("Default capacity should be 16, got %d", bp.Capacity())
	}
}

func TestBufferPoolPut(t *testing.T) {
	bp := NewBufferPool(10, PageSize)

	data := make([]byte, PageSize)
	data[0] = 0xAB

	page, err := bp.Put(1, data)
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	if page.ID() != 1 {
		t.Errorf("Page ID should be 1, got %d", page.ID())
	}
	if page.Data()[0] != 0xAB {
		t.Errorf("Page data mismatch")
	}
	if bp.Size() != 1 {
		t.Errorf("Size should be 1, got %d", bp.Size())
	}
}

func TestBufferPoolGet(t *testing.T) {
	bp := NewBufferPool(10, PageSize)

	data := make([]byte, PageSize)
	data[0] = 0xCD

	bp.Put(1, data)

	page, ok := bp.Get(1)
	if !ok {
		t.Fatal("Get should return true for existing page")
	}
	if page.Data()[0] != 0xCD {
		t.Error("Page data mismatch")
	}

	_, ok = bp.Get(999)
	if ok {
		t.Error("Get should return false for non-existing page")
	}
}

func TestBufferPoolPutUpdate(t *testing.T) {
	bp := NewBufferPool(10, PageSize)

	data1 := make([]byte, PageSize)
	data1[0] = 0x11
	bp.Put(1, data1)

	data2 := make([]byte, PageSize)
	data2[0] = 0x22
	bp.Put(1, data2)

	page, _ := bp.Get(1)
	if page.Data()[0] != 0x22 {
		t.Error("Page data should be updated")
	}
	if bp.Size() != 1 {
		t.Errorf("Size should still be 1, got %d", bp.Size())
	}
}

func TestBufferPoolPin(t *testing.T) {
	bp := NewBufferPool(10, PageSize)

	bp.Put(1, nil)

	if err := bp.Pin(1); err != nil {
		t.Fatalf("Pin failed: %v", err)
	}

	page, _ := bp.Get(1)
	if page.PinCount() != 1 {
		t.Errorf("Pin count should be 1, got %d", page.PinCount())
	}

	// Pin again
	bp.Pin(1)
	page, _ = bp.Get(1)
	if page.PinCount() != 2 {
		t.Errorf("Pin count should be 2, got %d", page.PinCount())
	}
}

func TestBufferPoolPinNotFound(t *testing.T) {
	bp := NewBufferPool(10, PageSize)

	err := bp.Pin(999)
	if err != ErrPageNotFound {
		t.Errorf("Pin non-existing page should return ErrPageNotFound, got %v", err)
	}
}

func TestBufferPoolUnpin(t *testing.T) {
	bp := NewBufferPool(10, PageSize)

	bp.Put(1, nil)
	bp.Pin(1)
	bp.Pin(1)

	if err := bp.Unpin(1); err != nil {
		t.Fatalf("Unpin failed: %v", err)
	}

	page, _ := bp.Get(1)
	if page.PinCount() != 1 {
		t.Errorf("Pin count should be 1 after unpin, got %d", page.PinCount())
	}
}

func TestBufferPoolUnpinNotFound(t *testing.T) {
	bp := NewBufferPool(10, PageSize)

	err := bp.Unpin(999)
	if err != ErrPageNotFound {
		t.Errorf("Unpin non-existing page should return ErrPageNotFound, got %v", err)
	}
}

func TestBufferPoolUnpinNegative(t *testing.T) {
	bp := NewBufferPool(10, PageSize)

	bp.Put(1, nil)

	err := bp.Unpin(1)
	if err != ErrNegativePinCount {
		t.Errorf("Unpin with zero pin count should return ErrNegativePinCount, got %v", err)
	}
}

func TestBufferPoolMarkDirty(t *testing.T) {
	bp := NewBufferPool(10, PageSize)

	bp.Put(1, nil)

	if err := bp.MarkDirty(1); err != nil {
		t.Fatalf("MarkDirty failed: %v", err)
	}

	page, _ := bp.Get(1)
	if !page.IsDirty() {
		t.Error("Page should be dirty")
	}
	if bp.DirtyPageCount() != 1 {
		t.Errorf("Dirty page count should be 1, got %d", bp.DirtyPageCount())
	}
}

func TestBufferPoolMarkDirtyNotFound(t *testing.T) {
	bp := NewBufferPool(10, PageSize)

	err := bp.MarkDirty(999)
	if err != ErrPageNotFound {
		t.Errorf("MarkDirty non-existing page should return ErrPageNotFound, got %v", err)
	}
}

// =============================================================================
// Eviction Tests
// =============================================================================

func TestBufferPoolEvictLRU(t *testing.T) {
	bp := NewBufferPool(3, PageSize)

	// Add 3 pages
	bp.Put(1, nil)
	bp.Put(2, nil)
	bp.Put(3, nil)

	// Evict should remove page 1 (LRU)
	pageID, _, ok := bp.Evict()
	if !ok {
		t.Fatal("Evict should succeed")
	}
	if pageID != 1 {
		t.Errorf("Evicted page should be 1 (LRU), got %d", pageID)
	}
	if bp.Size() != 2 {
		t.Errorf("Size should be 2 after eviction, got %d", bp.Size())
	}
	if bp.Contains(1) {
		t.Error("Buffer pool should not contain evicted page")
	}
}

func TestBufferPoolEvictSkipsPinned(t *testing.T) {
	bp := NewBufferPool(3, PageSize)

	// Add 3 pages
	bp.Put(1, nil)
	bp.Put(2, nil)
	bp.Put(3, nil)

	// Pin page 1 (the LRU)
	bp.Pin(1)

	// Evict should skip page 1 and remove page 2
	pageID, _, ok := bp.Evict()
	if !ok {
		t.Fatal("Evict should succeed")
	}
	if pageID != 2 {
		t.Errorf("Evicted page should be 2 (next LRU after pinned), got %d", pageID)
	}
	if !bp.Contains(1) {
		t.Error("Pinned page 1 should still be in buffer pool")
	}
}

func TestBufferPoolEvictAllPinned(t *testing.T) {
	bp := NewBufferPool(3, PageSize)

	// Add and pin all pages
	bp.Put(1, nil)
	bp.Put(2, nil)
	bp.Put(3, nil)
	bp.Pin(1)
	bp.Pin(2)
	bp.Pin(3)

	// Evict should fail
	_, _, ok := bp.Evict()
	if ok {
		t.Error("Evict should fail when all pages are pinned")
	}
}

func TestBufferPoolEvictFlushesDirty(t *testing.T) {
	bp := NewBufferPool(3, PageSize)

	var flushedPageID PageID
	var flushedData []byte
	bp.SetFlushCallback(func(pageID PageID, data []byte) error {
		flushedPageID = pageID
		flushedData = make([]byte, len(data))
		copy(flushedData, data)
		return nil
	})

	// Add pages
	data := make([]byte, PageSize)
	data[0] = 0xFF
	bp.Put(1, data)
	bp.Put(2, nil)
	bp.Put(3, nil)

	// Mark page 1 as dirty
	bp.MarkDirty(1)

	// Evict should flush page 1 before evicting
	pageID, _, ok := bp.Evict()
	if !ok {
		t.Fatal("Evict should succeed")
	}
	if pageID != 1 {
		t.Errorf("Evicted page should be 1, got %d", pageID)
	}
	if flushedPageID != 1 {
		t.Errorf("Flushed page should be 1, got %d", flushedPageID)
	}
	if flushedData[0] != 0xFF {
		t.Error("Flushed data mismatch")
	}
}

func TestBufferPoolEvictOnPut(t *testing.T) {
	bp := NewBufferPool(3, PageSize)

	// Fill the pool
	bp.Put(1, nil)
	bp.Put(2, nil)
	bp.Put(3, nil)

	// Adding a new page should trigger eviction
	_, err := bp.Put(4, nil)
	if err != nil {
		t.Fatalf("Put should succeed with eviction: %v", err)
	}

	if bp.Size() != 3 {
		t.Errorf("Size should still be 3, got %d", bp.Size())
	}
	if bp.Contains(1) {
		t.Error("Page 1 should have been evicted")
	}
	if !bp.Contains(4) {
		t.Error("Page 4 should be in buffer pool")
	}
}

func TestBufferPoolEvictOnPutAllPinned(t *testing.T) {
	bp := NewBufferPool(3, PageSize)

	// Fill and pin all pages
	bp.Put(1, nil)
	bp.Put(2, nil)
	bp.Put(3, nil)
	bp.Pin(1)
	bp.Pin(2)
	bp.Pin(3)

	// Adding a new page should fail
	_, err := bp.Put(4, nil)
	if err != ErrBufferPoolFull {
		t.Errorf("Put should fail with ErrBufferPoolFull, got %v", err)
	}
}

// =============================================================================
// Flush Tests
// =============================================================================

func TestBufferPoolFlushAll(t *testing.T) {
	bp := NewBufferPool(10, PageSize)

	flushedPages := make(map[PageID]bool)
	bp.SetFlushCallback(func(pageID PageID, data []byte) error {
		flushedPages[pageID] = true
		return nil
	})

	// Add and mark pages as dirty
	bp.Put(1, nil)
	bp.Put(2, nil)
	bp.Put(3, nil)
	bp.MarkDirty(1)
	bp.MarkDirty(3)

	if err := bp.FlushAll(); err != nil {
		t.Fatalf("FlushAll failed: %v", err)
	}

	if !flushedPages[1] {
		t.Error("Page 1 should have been flushed")
	}
	if flushedPages[2] {
		t.Error("Page 2 should not have been flushed (not dirty)")
	}
	if !flushedPages[3] {
		t.Error("Page 3 should have been flushed")
	}

	if bp.DirtyPageCount() != 0 {
		t.Errorf("Dirty page count should be 0 after flush, got %d", bp.DirtyPageCount())
	}
}

func TestBufferPoolFlushPage(t *testing.T) {
	bp := NewBufferPool(10, PageSize)

	var flushedPageID PageID
	bp.SetFlushCallback(func(pageID PageID, data []byte) error {
		flushedPageID = pageID
		return nil
	})

	bp.Put(1, nil)
	bp.MarkDirty(1)

	if err := bp.FlushPage(1); err != nil {
		t.Fatalf("FlushPage failed: %v", err)
	}

	if flushedPageID != 1 {
		t.Errorf("Flushed page should be 1, got %d", flushedPageID)
	}

	page, _ := bp.Get(1)
	if page.IsDirty() {
		t.Error("Page should not be dirty after flush")
	}
}

func TestBufferPoolFlushPageNotDirty(t *testing.T) {
	bp := NewBufferPool(10, PageSize)

	flushed := false
	bp.SetFlushCallback(func(pageID PageID, data []byte) error {
		flushed = true
		return nil
	})

	bp.Put(1, nil)

	if err := bp.FlushPage(1); err != nil {
		t.Fatalf("FlushPage failed: %v", err)
	}

	if flushed {
		t.Error("Non-dirty page should not be flushed")
	}
}

func TestBufferPoolFlushPageNotFound(t *testing.T) {
	bp := NewBufferPool(10, PageSize)

	err := bp.FlushPage(999)
	if err != ErrPageNotFound {
		t.Errorf("FlushPage non-existing page should return ErrPageNotFound, got %v", err)
	}
}

// =============================================================================
// Remove Tests
// =============================================================================

func TestBufferPoolRemove(t *testing.T) {
	bp := NewBufferPool(10, PageSize)

	bp.Put(1, nil)

	if err := bp.Remove(1); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	if bp.Contains(1) {
		t.Error("Page should be removed")
	}
	if bp.Size() != 0 {
		t.Errorf("Size should be 0, got %d", bp.Size())
	}
}

func TestBufferPoolRemovePinned(t *testing.T) {
	bp := NewBufferPool(10, PageSize)

	bp.Put(1, nil)
	bp.Pin(1)

	err := bp.Remove(1)
	if err != ErrPagePinned {
		t.Errorf("Remove pinned page should return ErrPagePinned, got %v", err)
	}
}

func TestBufferPoolRemoveNotFound(t *testing.T) {
	bp := NewBufferPool(10, PageSize)

	err := bp.Remove(999)
	if err != ErrPageNotFound {
		t.Errorf("Remove non-existing page should return ErrPageNotFound, got %v", err)
	}
}

func TestBufferPoolRemoveFlushesDirty(t *testing.T) {
	bp := NewBufferPool(10, PageSize)

	var flushedPageID PageID
	bp.SetFlushCallback(func(pageID PageID, data []byte) error {
		flushedPageID = pageID
		return nil
	})

	bp.Put(1, nil)
	bp.MarkDirty(1)

	if err := bp.Remove(1); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	if flushedPageID != 1 {
		t.Errorf("Dirty page should be flushed before removal, got %d", flushedPageID)
	}
}

// =============================================================================
// Clear Tests
// =============================================================================

func TestBufferPoolClear(t *testing.T) {
	bp := NewBufferPool(10, PageSize)

	bp.Put(1, nil)
	bp.Put(2, nil)
	bp.Put(3, nil)
	bp.MarkDirty(1)
	bp.MarkDirty(2)

	flushedCount := 0
	bp.SetFlushCallback(func(pageID PageID, data []byte) error {
		flushedCount++
		return nil
	})

	if err := bp.Clear(); err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	if bp.Size() != 0 {
		t.Errorf("Size should be 0 after clear, got %d", bp.Size())
	}
	if bp.DirtyPageCount() != 0 {
		t.Errorf("Dirty page count should be 0 after clear, got %d", bp.DirtyPageCount())
	}
	if flushedCount != 2 {
		t.Errorf("Should have flushed 2 dirty pages, got %d", flushedCount)
	}
}

// =============================================================================
// Stats Tests
// =============================================================================

func TestBufferPoolStats(t *testing.T) {
	bp := NewBufferPool(10, PageSize)

	bp.Put(1, nil)
	bp.Put(2, nil)
	bp.Put(3, nil)
	bp.Pin(1)
	bp.Pin(2)
	bp.MarkDirty(2)
	bp.MarkDirty(3)

	stats := bp.Stats()

	if stats.Capacity != 10 {
		t.Errorf("Capacity should be 10, got %d", stats.Capacity)
	}
	if stats.Size != 3 {
		t.Errorf("Size should be 3, got %d", stats.Size)
	}
	if stats.PinnedPages != 2 {
		t.Errorf("PinnedPages should be 2, got %d", stats.PinnedPages)
	}
	if stats.DirtyPages != 2 {
		t.Errorf("DirtyPages should be 2, got %d", stats.DirtyPages)
	}
}

// =============================================================================
// Helper Method Tests
// =============================================================================

func TestBufferPoolGetAllPageIDs(t *testing.T) {
	bp := NewBufferPool(10, PageSize)

	bp.Put(1, nil)
	bp.Put(2, nil)
	bp.Put(3, nil)

	ids := bp.GetAllPageIDs()
	if len(ids) != 3 {
		t.Errorf("Should have 3 page IDs, got %d", len(ids))
	}

	// Check all IDs are present
	idMap := make(map[PageID]bool)
	for _, id := range ids {
		idMap[id] = true
	}
	if !idMap[1] || !idMap[2] || !idMap[3] {
		t.Error("Missing page IDs")
	}
}

func TestBufferPoolGetDirtyPageIDs(t *testing.T) {
	bp := NewBufferPool(10, PageSize)

	bp.Put(1, nil)
	bp.Put(2, nil)
	bp.Put(3, nil)
	bp.MarkDirty(1)
	bp.MarkDirty(3)

	ids := bp.GetDirtyPageIDs()
	if len(ids) != 2 {
		t.Errorf("Should have 2 dirty page IDs, got %d", len(ids))
	}

	idMap := make(map[PageID]bool)
	for _, id := range ids {
		idMap[id] = true
	}
	if !idMap[1] || !idMap[3] {
		t.Error("Missing dirty page IDs")
	}
	if idMap[2] {
		t.Error("Page 2 should not be in dirty list")
	}
}

// =============================================================================
// Concurrency Tests
// =============================================================================

func TestBufferPoolConcurrentAccess(t *testing.T) {
	bp := NewBufferPool(100, PageSize)

	var wg sync.WaitGroup
	numGoroutines := 10
	numOperations := 100

	// Concurrent Put operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				pageID := PageID(goroutineID*numOperations + j)
				data := make([]byte, PageSize)
				data[0] = byte(goroutineID)
				bp.Put(pageID, data)
			}
		}(i)
	}

	wg.Wait()

	// Verify some pages exist
	if bp.Size() == 0 {
		t.Error("Buffer pool should have pages after concurrent puts")
	}
}

func TestBufferPoolConcurrentGetPut(t *testing.T) {
	bp := NewBufferPool(50, PageSize)

	// Pre-populate
	for i := 0; i < 50; i++ {
		bp.Put(PageID(i), nil)
	}

	var wg sync.WaitGroup
	numGoroutines := 10

	// Concurrent Get and Put
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				pageID := PageID(j % 50)
				if j%2 == 0 {
					bp.Get(pageID)
				} else {
					bp.Put(pageID, nil)
				}
			}
		}(i)
	}

	wg.Wait()
}

func TestBufferPoolConcurrentPinUnpin(t *testing.T) {
	bp := NewBufferPool(10, PageSize)

	// Pre-populate
	for i := 1; i <= 10; i++ {
		bp.Put(PageID(i), nil)
	}

	var wg sync.WaitGroup
	numGoroutines := 5

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				pageID := PageID((j % 10) + 1)
				bp.Pin(pageID)
				bp.Unpin(pageID)
			}
		}()
	}

	wg.Wait()

	// All pages should have pin count 0
	for i := 1; i <= 10; i++ {
		page, ok := bp.Get(PageID(i))
		if ok && page.PinCount() != 0 {
			t.Errorf("Page %d should have pin count 0, got %d", i, page.PinCount())
		}
	}
}

func TestBufferPoolConcurrentEviction(t *testing.T) {
	bp := NewBufferPool(10, PageSize)

	var flushCount int64
	bp.SetFlushCallback(func(pageID PageID, data []byte) error {
		atomic.AddInt64(&flushCount, 1)
		return nil
	})

	var wg sync.WaitGroup
	numGoroutines := 5

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				pageID := PageID(goroutineID*100 + j)
				bp.Put(pageID, nil)
				bp.MarkDirty(pageID)
			}
		}(i)
	}

	wg.Wait()

	// Buffer pool should have at most capacity pages
	if bp.Size() > bp.Capacity() {
		t.Errorf("Buffer pool size %d exceeds capacity %d", bp.Size(), bp.Capacity())
	}
}

// =============================================================================
// LRU Order Tests
// =============================================================================

func TestLRUEvictionOrder(t *testing.T) {
	bp := NewBufferPool(5, PageSize)

	// Add pages in order 1, 2, 3, 4, 5
	for i := 1; i <= 5; i++ {
		bp.Put(PageID(i), nil)
	}

	// Access pages in order 3, 1, 5 (making 2, 4 the coldest)
	bp.Get(3)
	bp.Get(1)
	bp.Get(5)

	// Evict should remove page 2 (coldest)
	pageID, _, ok := bp.Evict()
	if !ok {
		t.Fatal("Evict should succeed")
	}
	if pageID != 2 {
		t.Errorf("First evicted page should be 2 (coldest), got %d", pageID)
	}

	// Next evict should remove page 4
	pageID, _, ok = bp.Evict()
	if !ok {
		t.Fatal("Evict should succeed")
	}
	if pageID != 4 {
		t.Errorf("Second evicted page should be 4, got %d", pageID)
	}
}

func TestLRUAccessUpdatesOrder(t *testing.T) {
	cache := NewLRUCache()

	// Add pages
	cache.Access(1)
	cache.Access(2)
	cache.Access(3)

	// LRU should be 1
	lru, _ := cache.GetLRU()
	if lru != 1 {
		t.Errorf("LRU should be 1, got %d", lru)
	}

	// Access 1, now LRU should be 2
	cache.Access(1)
	lru, _ = cache.GetLRU()
	if lru != 2 {
		t.Errorf("LRU should be 2 after accessing 1, got %d", lru)
	}

	// Access 2, now LRU should be 3
	cache.Access(2)
	lru, _ = cache.GetLRU()
	if lru != 3 {
		t.Errorf("LRU should be 3 after accessing 2, got %d", lru)
	}
}

// =============================================================================
// Edge Cases
// =============================================================================

func TestBufferPoolEmptyEvict(t *testing.T) {
	bp := NewBufferPool(10, PageSize)

	_, _, ok := bp.Evict()
	if ok {
		t.Error("Evict on empty buffer pool should return false")
	}
}

func TestBufferPoolSinglePage(t *testing.T) {
	bp := NewBufferPool(1, PageSize)

	bp.Put(1, nil)

	// Adding another page should evict the first
	bp.Put(2, nil)

	if bp.Contains(1) {
		t.Error("Page 1 should have been evicted")
	}
	if !bp.Contains(2) {
		t.Error("Page 2 should be in buffer pool")
	}
}

func TestBufferPoolDataIntegrity(t *testing.T) {
	bp := NewBufferPool(10, PageSize)

	// Put page with specific data
	data := make([]byte, PageSize)
	for i := range data {
		data[i] = byte(i % 256)
	}
	bp.Put(1, data)

	// Get and verify data
	page, ok := bp.Get(1)
	if !ok {
		t.Fatal("Page should exist")
	}

	for i := 0; i < PageSize; i++ {
		if page.Data()[i] != byte(i%256) {
			t.Errorf("Data mismatch at index %d: got %d, want %d", i, page.Data()[i], byte(i%256))
			break
		}
	}
}

func TestBufferPoolEvictReturnsData(t *testing.T) {
	bp := NewBufferPool(3, PageSize)

	// Put page with specific data
	data := make([]byte, PageSize)
	data[0] = 0xAB
	data[1] = 0xCD
	bp.Put(1, data)
	bp.Put(2, nil)
	bp.Put(3, nil)

	// Evict should return the data
	pageID, evictedData, ok := bp.Evict()
	if !ok {
		t.Fatal("Evict should succeed")
	}
	if pageID != 1 {
		t.Errorf("Evicted page should be 1, got %d", pageID)
	}
	if evictedData[0] != 0xAB || evictedData[1] != 0xCD {
		t.Error("Evicted data mismatch")
	}
}
