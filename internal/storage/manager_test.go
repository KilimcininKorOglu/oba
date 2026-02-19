package storage

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// =============================================================================
// FreeList Tests
// =============================================================================

func TestNewFreeList(t *testing.T) {
	fl := NewFreeList()

	if fl.Head() != 0 {
		t.Errorf("Head() = %v, want 0", fl.Head())
	}
	if fl.Count() != 0 {
		t.Errorf("Count() = %v, want 0", fl.Count())
	}
	if !fl.IsEmpty() {
		t.Error("IsEmpty() should return true for new free list")
	}
}

func TestNewFreeListWithHead(t *testing.T) {
	fl := NewFreeListWithHead(42)

	if fl.Head() != 42 {
		t.Errorf("Head() = %v, want 42", fl.Head())
	}
}

func TestFreeListPushPop(t *testing.T) {
	fl := NewFreeList()

	// Push some pages
	fl.Push(10)
	fl.Push(20)
	fl.Push(30)

	if fl.Count() != 3 {
		t.Errorf("Count() = %v, want 3", fl.Count())
	}
	if fl.IsEmpty() {
		t.Error("IsEmpty() should return false after push")
	}

	// Pop should return in LIFO order
	id, ok := fl.Pop()
	if !ok || id != 30 {
		t.Errorf("Pop() = %v, %v, want 30, true", id, ok)
	}

	id, ok = fl.Pop()
	if !ok || id != 20 {
		t.Errorf("Pop() = %v, %v, want 20, true", id, ok)
	}

	id, ok = fl.Pop()
	if !ok || id != 10 {
		t.Errorf("Pop() = %v, %v, want 10, true", id, ok)
	}

	// Pop from empty list
	id, ok = fl.Pop()
	if ok || id != 0 {
		t.Errorf("Pop() from empty = %v, %v, want 0, false", id, ok)
	}
}

func TestFreeListContains(t *testing.T) {
	fl := NewFreeList()
	fl.Push(10)
	fl.Push(20)

	if !fl.Contains(10) {
		t.Error("Contains(10) should return true")
	}
	if !fl.Contains(20) {
		t.Error("Contains(20) should return true")
	}
	if fl.Contains(30) {
		t.Error("Contains(30) should return false")
	}
}

func TestFreeListRemove(t *testing.T) {
	fl := NewFreeList()
	fl.Push(10)
	fl.Push(20)
	fl.Push(30)

	// Remove middle element
	if !fl.Remove(20) {
		t.Error("Remove(20) should return true")
	}
	if fl.Count() != 2 {
		t.Errorf("Count() = %v, want 2", fl.Count())
	}
	if fl.Contains(20) {
		t.Error("Contains(20) should return false after remove")
	}

	// Remove non-existent element
	if fl.Remove(99) {
		t.Error("Remove(99) should return false")
	}
}

func TestFreeListClear(t *testing.T) {
	fl := NewFreeList()
	fl.Push(10)
	fl.Push(20)
	fl.SetHead(5)

	fl.Clear()

	if fl.Head() != 0 {
		t.Errorf("Head() = %v, want 0 after clear", fl.Head())
	}
	if fl.Count() != 0 {
		t.Errorf("Count() = %v, want 0 after clear", fl.Count())
	}
	if !fl.IsEmpty() {
		t.Error("IsEmpty() should return true after clear")
	}
}

func TestFreeListPeekAll(t *testing.T) {
	fl := NewFreeList()
	fl.Push(10)
	fl.Push(20)
	fl.Push(30)

	pages := fl.PeekAll()

	if len(pages) != 3 {
		t.Errorf("PeekAll() length = %v, want 3", len(pages))
	}

	// Verify it's a copy
	pages[0] = 999
	if fl.Contains(999) {
		t.Error("PeekAll should return a copy, not the original slice")
	}
}

func TestFreeListConcurrency(t *testing.T) {
	fl := NewFreeList()
	var wg sync.WaitGroup

	// Concurrent pushes
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id PageID) {
			defer wg.Done()
			fl.Push(id)
		}(PageID(i))
	}
	wg.Wait()

	if fl.Count() != 100 {
		t.Errorf("Count() = %v, want 100 after concurrent pushes", fl.Count())
	}

	// Concurrent pops
	popped := make(chan PageID, 100)
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if id, ok := fl.Pop(); ok {
				popped <- id
			}
		}()
	}
	wg.Wait()
	close(popped)

	count := 0
	for range popped {
		count++
	}

	if count != 100 {
		t.Errorf("Popped %v pages, want 100", count)
	}

	if !fl.IsEmpty() {
		t.Error("Free list should be empty after popping all pages")
	}
}

// =============================================================================
// PageManager Tests
// =============================================================================

func createTempDir(t *testing.T) string {
	dir, err := os.MkdirTemp("", "obadb-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	return dir
}

func TestOpenPageManagerNew(t *testing.T) {
	dir := createTempDir(t)
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "test.oba")
	pm, err := OpenPageManager(path, DefaultOptions())
	if err != nil {
		t.Fatalf("OpenPageManager failed: %v", err)
	}
	defer pm.Close()

	if pm.TotalPages() != DefaultInitialPages {
		t.Errorf("TotalPages() = %v, want %v", pm.TotalPages(), DefaultInitialPages)
	}

	if pm.PageSize() != DefaultPageSize {
		t.Errorf("PageSize() = %v, want %v", pm.PageSize(), DefaultPageSize)
	}

	if pm.Path() != path {
		t.Errorf("Path() = %v, want %v", pm.Path(), path)
	}

	if pm.IsReadOnly() {
		t.Error("IsReadOnly() should return false")
	}
}

func TestOpenPageManagerExisting(t *testing.T) {
	dir := createTempDir(t)
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "test.oba")

	// Create and close
	pm1, err := OpenPageManager(path, DefaultOptions())
	if err != nil {
		t.Fatalf("First OpenPageManager failed: %v", err)
	}

	// Allocate some pages
	id1, err := pm1.AllocatePage(PageTypeData)
	if err != nil {
		t.Fatalf("AllocatePage failed: %v", err)
	}

	if err := pm1.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Reopen
	pm2, err := OpenPageManager(path, DefaultOptions())
	if err != nil {
		t.Fatalf("Second OpenPageManager failed: %v", err)
	}
	defer pm2.Close()

	// Verify header is valid
	header := pm2.Header()
	if header.Magic != Magic {
		t.Errorf("Magic = %v, want %v", header.Magic, Magic)
	}

	// The allocated page should still exist
	page, err := pm2.ReadPage(id1)
	if err != nil {
		t.Fatalf("ReadPage failed: %v", err)
	}
	if page.Header.PageType != PageTypeData {
		t.Errorf("PageType = %v, want PageTypeData", page.Header.PageType)
	}
}

func TestOpenPageManagerNoCreate(t *testing.T) {
	dir := createTempDir(t)
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "nonexistent.oba")
	opts := DefaultOptions()
	opts.CreateIfNew = false

	_, err := OpenPageManager(path, opts)
	if !os.IsNotExist(err) {
		t.Errorf("Expected os.ErrNotExist, got %v", err)
	}
}

func TestPageManagerAllocatePage(t *testing.T) {
	dir := createTempDir(t)
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "test.oba")
	pm, err := OpenPageManager(path, DefaultOptions())
	if err != nil {
		t.Fatalf("OpenPageManager failed: %v", err)
	}
	defer pm.Close()

	initialFree := pm.FreePageCount()

	// Allocate a page
	id, err := pm.AllocatePage(PageTypeData)
	if err != nil {
		t.Fatalf("AllocatePage failed: %v", err)
	}

	if id == 0 {
		t.Error("AllocatePage returned page ID 0")
	}

	// Free count should decrease
	if pm.FreePageCount() != initialFree-1 {
		t.Errorf("FreePageCount() = %v, want %v", pm.FreePageCount(), initialFree-1)
	}

	// Read the page back
	page, err := pm.ReadPage(id)
	if err != nil {
		t.Fatalf("ReadPage failed: %v", err)
	}

	if page.Header.PageID != id {
		t.Errorf("PageID = %v, want %v", page.Header.PageID, id)
	}
	if page.Header.PageType != PageTypeData {
		t.Errorf("PageType = %v, want PageTypeData", page.Header.PageType)
	}
}

func TestPageManagerFreePage(t *testing.T) {
	dir := createTempDir(t)
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "test.oba")
	pm, err := OpenPageManager(path, DefaultOptions())
	if err != nil {
		t.Fatalf("OpenPageManager failed: %v", err)
	}
	defer pm.Close()

	// Allocate a page
	id, err := pm.AllocatePage(PageTypeData)
	if err != nil {
		t.Fatalf("AllocatePage failed: %v", err)
	}

	freeCountBefore := pm.FreePageCount()

	// Free the page
	if err := pm.FreePage(id); err != nil {
		t.Fatalf("FreePage failed: %v", err)
	}

	// Free count should increase
	if pm.FreePageCount() != freeCountBefore+1 {
		t.Errorf("FreePageCount() = %v, want %v", pm.FreePageCount(), freeCountBefore+1)
	}

	// Read the freed page - should be PageTypeFree
	page, err := pm.ReadPage(id)
	if err != nil {
		t.Fatalf("ReadPage failed: %v", err)
	}
	if page.Header.PageType != PageTypeFree {
		t.Errorf("PageType = %v, want PageTypeFree", page.Header.PageType)
	}
}

func TestPageManagerFreePageReuse(t *testing.T) {
	dir := createTempDir(t)
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "test.oba")
	pm, err := OpenPageManager(path, DefaultOptions())
	if err != nil {
		t.Fatalf("OpenPageManager failed: %v", err)
	}
	defer pm.Close()

	// Allocate and free a page
	id1, err := pm.AllocatePage(PageTypeData)
	if err != nil {
		t.Fatalf("AllocatePage failed: %v", err)
	}

	if err := pm.FreePage(id1); err != nil {
		t.Fatalf("FreePage failed: %v", err)
	}

	// Allocate again - should reuse the freed page
	id2, err := pm.AllocatePage(PageTypeData)
	if err != nil {
		t.Fatalf("AllocatePage failed: %v", err)
	}

	if id2 != id1 {
		t.Errorf("Expected freed page %v to be reused, got %v", id1, id2)
	}
}

func TestPageManagerFreePageErrors(t *testing.T) {
	dir := createTempDir(t)
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "test.oba")
	pm, err := OpenPageManager(path, DefaultOptions())
	if err != nil {
		t.Fatalf("OpenPageManager failed: %v", err)
	}
	defer pm.Close()

	// Cannot free page 0 (header)
	if err := pm.FreePage(0); err != ErrCannotFreeHeader {
		t.Errorf("FreePage(0) = %v, want ErrCannotFreeHeader", err)
	}

	// Cannot free out of range page
	if err := pm.FreePage(PageID(pm.TotalPages() + 100)); err != ErrPageOutOfRange {
		t.Errorf("FreePage(out of range) = %v, want ErrPageOutOfRange", err)
	}

	// Allocate and free a page
	id, _ := pm.AllocatePage(PageTypeData)
	pm.FreePage(id)

	// Cannot free already free page
	if err := pm.FreePage(id); err != ErrPageAlreadyFree {
		t.Errorf("FreePage(already free) = %v, want ErrPageAlreadyFree", err)
	}
}

func TestPageManagerReadWritePage(t *testing.T) {
	dir := createTempDir(t)
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "test.oba")
	pm, err := OpenPageManager(path, DefaultOptions())
	if err != nil {
		t.Fatalf("OpenPageManager failed: %v", err)
	}
	defer pm.Close()

	// Allocate a page
	id, err := pm.AllocatePage(PageTypeData)
	if err != nil {
		t.Fatalf("AllocatePage failed: %v", err)
	}

	// Read the page
	page, err := pm.ReadPage(id)
	if err != nil {
		t.Fatalf("ReadPage failed: %v", err)
	}

	// Modify the page
	testData := []byte("Hello, ObaDB!")
	copy(page.Data, testData)
	page.Header.ItemCount = 1

	// Write the page
	if err := pm.WritePage(page); err != nil {
		t.Fatalf("WritePage failed: %v", err)
	}

	// Read again and verify
	page2, err := pm.ReadPage(id)
	if err != nil {
		t.Fatalf("ReadPage failed: %v", err)
	}

	if page2.Header.ItemCount != 1 {
		t.Errorf("ItemCount = %v, want 1", page2.Header.ItemCount)
	}

	for i, b := range testData {
		if page2.Data[i] != b {
			t.Errorf("Data[%d] = %v, want %v", i, page2.Data[i], b)
		}
	}
}

func TestPageManagerReadPageErrors(t *testing.T) {
	dir := createTempDir(t)
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "test.oba")
	pm, err := OpenPageManager(path, DefaultOptions())
	if err != nil {
		t.Fatalf("OpenPageManager failed: %v", err)
	}
	defer pm.Close()

	// Cannot read page 0
	if _, err := pm.ReadPage(0); err != ErrInvalidPageID {
		t.Errorf("ReadPage(0) error = %v, want ErrInvalidPageID", err)
	}

	// Cannot read out of range page
	if _, err := pm.ReadPage(PageID(pm.TotalPages() + 100)); err != ErrPageOutOfRange {
		t.Errorf("ReadPage(out of range) error = %v, want ErrPageOutOfRange", err)
	}
}

func TestPageManagerSync(t *testing.T) {
	dir := createTempDir(t)
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "test.oba")
	pm, err := OpenPageManager(path, DefaultOptions())
	if err != nil {
		t.Fatalf("OpenPageManager failed: %v", err)
	}
	defer pm.Close()

	// Allocate and write a page
	id, _ := pm.AllocatePage(PageTypeData)
	page, _ := pm.ReadPage(id)
	copy(page.Data, []byte("test data"))
	pm.WritePage(page)

	// Sync should not error
	if err := pm.Sync(); err != nil {
		t.Fatalf("Sync failed: %v", err)
	}
}

func TestPageManagerFileGrowth(t *testing.T) {
	dir := createTempDir(t)
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "test.oba")
	opts := DefaultOptions()
	opts.InitialPages = 4 // Small initial size

	pm, err := OpenPageManager(path, opts)
	if err != nil {
		t.Fatalf("OpenPageManager failed: %v", err)
	}
	defer pm.Close()

	initialTotal := pm.TotalPages()

	// Allocate all free pages
	for pm.FreePageCount() > 0 {
		_, err := pm.AllocatePage(PageTypeData)
		if err != nil {
			t.Fatalf("AllocatePage failed: %v", err)
		}
	}

	// Allocate one more - should trigger growth
	_, err = pm.AllocatePage(PageTypeData)
	if err != nil {
		t.Fatalf("AllocatePage after exhaustion failed: %v", err)
	}

	if pm.TotalPages() <= initialTotal {
		t.Errorf("TotalPages() = %v, should be > %v after growth", pm.TotalPages(), initialTotal)
	}
}

func TestPageManagerStats(t *testing.T) {
	dir := createTempDir(t)
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "test.oba")
	pm, err := OpenPageManager(path, DefaultOptions())
	if err != nil {
		t.Fatalf("OpenPageManager failed: %v", err)
	}
	defer pm.Close()

	stats := pm.Stats()

	if stats.TotalPages != pm.TotalPages() {
		t.Errorf("Stats.TotalPages = %v, want %v", stats.TotalPages, pm.TotalPages())
	}
	if stats.FreePages != pm.FreePageCount() {
		t.Errorf("Stats.FreePages = %v, want %v", stats.FreePages, pm.FreePageCount())
	}
	if stats.PageSize != pm.PageSize() {
		t.Errorf("Stats.PageSize = %v, want %v", stats.PageSize, pm.PageSize())
	}
	if stats.FileSizeBytes != int64(stats.TotalPages)*int64(stats.PageSize) {
		t.Errorf("Stats.FileSizeBytes = %v, want %v", stats.FileSizeBytes, int64(stats.TotalPages)*int64(stats.PageSize))
	}
}

func TestPageManagerReadOnly(t *testing.T) {
	dir := createTempDir(t)
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "test.oba")

	// Create the file first
	pm1, err := OpenPageManager(path, DefaultOptions())
	if err != nil {
		t.Fatalf("OpenPageManager failed: %v", err)
	}
	pm1.Close()

	// Open in read-only mode
	opts := DefaultOptions()
	opts.ReadOnly = true
	pm2, err := OpenPageManager(path, opts)
	if err != nil {
		t.Fatalf("OpenPageManager read-only failed: %v", err)
	}
	defer pm2.Close()

	if !pm2.IsReadOnly() {
		t.Error("IsReadOnly() should return true")
	}

	// Should not be able to allocate
	_, err = pm2.AllocatePage(PageTypeData)
	if err == nil {
		t.Error("AllocatePage should fail in read-only mode")
	}
}

func TestPageManagerConcurrentAccess(t *testing.T) {
	dir := createTempDir(t)
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "test.oba")
	opts := DefaultOptions()
	opts.InitialPages = 100

	pm, err := OpenPageManager(path, opts)
	if err != nil {
		t.Fatalf("OpenPageManager failed: %v", err)
	}
	defer pm.Close()

	var wg sync.WaitGroup
	errors := make(chan error, 100)

	// Concurrent allocations
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id, err := pm.AllocatePage(PageTypeData)
			if err != nil {
				errors <- err
				return
			}

			// Write to the page
			page, err := pm.ReadPage(id)
			if err != nil {
				errors <- err
				return
			}
			copy(page.Data, []byte("concurrent test"))
			if err := pm.WritePage(page); err != nil {
				errors <- err
			}
		}()
	}

	// Concurrent reads
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = pm.Stats()
			_ = pm.TotalPages()
			_ = pm.FreePageCount()
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("Concurrent operation error: %v", err)
	}
}

func TestPageManagerCloseErrors(t *testing.T) {
	dir := createTempDir(t)
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "test.oba")
	pm, err := OpenPageManager(path, DefaultOptions())
	if err != nil {
		t.Fatalf("OpenPageManager failed: %v", err)
	}

	// First close should succeed
	if err := pm.Close(); err != nil {
		t.Fatalf("First Close failed: %v", err)
	}

	// Second close should return error
	if err := pm.Close(); err != ErrFileClosed {
		t.Errorf("Second Close = %v, want ErrFileClosed", err)
	}

	// Operations on closed manager should fail
	if _, err := pm.AllocatePage(PageTypeData); err != ErrFileClosed {
		t.Errorf("AllocatePage on closed = %v, want ErrFileClosed", err)
	}

	if _, err := pm.ReadPage(1); err != ErrFileClosed {
		t.Errorf("ReadPage on closed = %v, want ErrFileClosed", err)
	}

	if err := pm.FreePage(1); err != ErrFileClosed {
		t.Errorf("FreePage on closed = %v, want ErrFileClosed", err)
	}

	if err := pm.Sync(); err != ErrFileClosed {
		t.Errorf("Sync on closed = %v, want ErrFileClosed", err)
	}
}

func TestPageManagerSyncOnWrite(t *testing.T) {
	dir := createTempDir(t)
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "test.oba")
	opts := DefaultOptions()
	opts.SyncOnWrite = true

	pm, err := OpenPageManager(path, opts)
	if err != nil {
		t.Fatalf("OpenPageManager failed: %v", err)
	}
	defer pm.Close()

	// Allocate and write - should sync after each write
	id, err := pm.AllocatePage(PageTypeData)
	if err != nil {
		t.Fatalf("AllocatePage failed: %v", err)
	}

	page, _ := pm.ReadPage(id)
	copy(page.Data, []byte("sync on write test"))
	if err := pm.WritePage(page); err != nil {
		t.Fatalf("WritePage failed: %v", err)
	}
}

func TestPageManagerMultiplePageTypes(t *testing.T) {
	dir := createTempDir(t)
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "test.oba")
	pm, err := OpenPageManager(path, DefaultOptions())
	if err != nil {
		t.Fatalf("OpenPageManager failed: %v", err)
	}
	defer pm.Close()

	pageTypes := []PageType{
		PageTypeData,
		PageTypeDNIndex,
		PageTypeAttrIndex,
		PageTypeOverflow,
	}

	for _, pt := range pageTypes {
		t.Run(pt.String(), func(t *testing.T) {
			id, err := pm.AllocatePage(pt)
			if err != nil {
				t.Fatalf("AllocatePage(%v) failed: %v", pt, err)
			}

			page, err := pm.ReadPage(id)
			if err != nil {
				t.Fatalf("ReadPage failed: %v", err)
			}

			if page.Header.PageType != pt {
				t.Errorf("PageType = %v, want %v", page.Header.PageType, pt)
			}
		})
	}
}

func TestPageManagerPersistence(t *testing.T) {
	dir := createTempDir(t)
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "test.oba")
	testData := []byte("persistence test data")

	// Create, write, close
	pm1, err := OpenPageManager(path, DefaultOptions())
	if err != nil {
		t.Fatalf("OpenPageManager failed: %v", err)
	}

	id, _ := pm1.AllocatePage(PageTypeData)
	page, _ := pm1.ReadPage(id)
	copy(page.Data, testData)
	page.Header.ItemCount = 42
	pm1.WritePage(page)
	pm1.Close()

	// Reopen and verify
	pm2, err := OpenPageManager(path, DefaultOptions())
	if err != nil {
		t.Fatalf("OpenPageManager reopen failed: %v", err)
	}
	defer pm2.Close()

	page2, err := pm2.ReadPage(id)
	if err != nil {
		t.Fatalf("ReadPage failed: %v", err)
	}

	if page2.Header.ItemCount != 42 {
		t.Errorf("ItemCount = %v, want 42", page2.Header.ItemCount)
	}

	for i, b := range testData {
		if page2.Data[i] != b {
			t.Errorf("Data[%d] = %v, want %v", i, page2.Data[i], b)
			break
		}
	}
}

// =============================================================================
// FreeList Serialization Tests
// =============================================================================

func TestFreeListSerializeToPage(t *testing.T) {
	fl := NewFreeList()

	// Add some pages
	for i := 1; i <= 10; i++ {
		fl.Push(PageID(i))
	}

	page := NewPage(100, PageTypeFree)
	nextIdx, hasMore := fl.SerializeToPage(page, 0)

	if hasMore {
		t.Error("Should not have more entries for 10 pages")
	}
	if nextIdx != 10 {
		t.Errorf("nextIdx = %v, want 10", nextIdx)
	}
	if page.Header.ItemCount != 10 {
		t.Errorf("ItemCount = %v, want 10", page.Header.ItemCount)
	}
}

func TestFreeListNextPageID(t *testing.T) {
	page := NewPage(1, PageTypeFree)

	SetNextPageID(page, 42)
	if got := GetNextPageID(page); got != 42 {
		t.Errorf("GetNextPageID() = %v, want 42", got)
	}

	SetNextPageID(page, 0)
	if got := GetNextPageID(page); got != 0 {
		t.Errorf("GetNextPageID() = %v, want 0", got)
	}
}

func TestGetNextPageIDNilPage(t *testing.T) {
	if got := GetNextPageID(nil); got != 0 {
		t.Errorf("GetNextPageID(nil) = %v, want 0", got)
	}
}

func TestSetNextPageIDNilPage(t *testing.T) {
	// Should not panic
	SetNextPageID(nil, 42)
}

// =============================================================================
// Default Options Tests
// =============================================================================

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()

	if opts.PageSize != DefaultPageSize {
		t.Errorf("PageSize = %v, want %v", opts.PageSize, DefaultPageSize)
	}
	if opts.InitialPages != DefaultInitialPages {
		t.Errorf("InitialPages = %v, want %v", opts.InitialPages, DefaultInitialPages)
	}
	if !opts.CreateIfNew {
		t.Error("CreateIfNew should be true by default")
	}
	if opts.ReadOnly {
		t.Error("ReadOnly should be false by default")
	}
	if opts.SyncOnWrite {
		t.Error("SyncOnWrite should be false by default")
	}
}

// =============================================================================
// Edge Cases
// =============================================================================

func TestPageManagerAllocateAllPages(t *testing.T) {
	dir := createTempDir(t)
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "test.oba")
	opts := DefaultOptions()
	opts.InitialPages = 5

	pm, err := OpenPageManager(path, opts)
	if err != nil {
		t.Fatalf("OpenPageManager failed: %v", err)
	}
	defer pm.Close()

	// Allocate all available pages
	allocated := make([]PageID, 0)
	for {
		id, err := pm.AllocatePage(PageTypeData)
		if err != nil {
			break
		}
		allocated = append(allocated, id)
		if len(allocated) > 100 {
			// Safety limit
			break
		}
	}

	// Should have allocated at least the initial free pages
	if len(allocated) < 4 { // 5 initial - 1 header = 4 free
		t.Errorf("Allocated %v pages, expected at least 4", len(allocated))
	}
}

func TestPageManagerWritePageErrors(t *testing.T) {
	dir := createTempDir(t)
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "test.oba")
	pm, err := OpenPageManager(path, DefaultOptions())
	if err != nil {
		t.Fatalf("OpenPageManager failed: %v", err)
	}
	defer pm.Close()

	// Cannot write page 0
	page := NewPage(0, PageTypeData)
	if err := pm.WritePage(page); err != ErrInvalidPageID {
		t.Errorf("WritePage(page 0) = %v, want ErrInvalidPageID", err)
	}

	// Cannot write out of range page
	page = NewPage(PageID(pm.TotalPages()+100), PageTypeData)
	if err := pm.WritePage(page); err != ErrPageOutOfRange {
		t.Errorf("WritePage(out of range) = %v, want ErrPageOutOfRange", err)
	}
}
