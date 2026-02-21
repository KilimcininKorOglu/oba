// Package storage provides the core storage engine components for ObaDB.
package storage

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
)

// Default options for PageManager.
const (
	DefaultPageSize     = PageSize
	DefaultInitialPages = 16
	DefaultGrowthFactor = 2
	MinGrowthPages      = 8
)

// Errors for PageManager operations.
var (
	ErrFileNotOpen      = errors.New("file not open")
	ErrInvalidPageID    = errors.New("invalid page ID")
	ErrPageOutOfRange   = errors.New("page ID out of range")
	ErrNoFreePages      = errors.New("no free pages available")
	ErrPageAlreadyFree  = errors.New("page is already free")
	ErrCannotFreeHeader = errors.New("cannot free header page")
	ErrFileClosed       = errors.New("page manager is closed")
	ErrFileExists       = errors.New("file already exists")
	ErrFileCorrupted    = errors.New("file is corrupted")
)

// Options configures the PageManager.
type Options struct {
	PageSize     int  // Page size in bytes (default: 4096)
	InitialPages int  // Initial number of pages to allocate
	CreateIfNew  bool // Create file if it doesn't exist
	ReadOnly     bool // Open in read-only mode
	SyncOnWrite  bool // Sync to disk after each write
}

// DefaultOptions returns the default PageManager options.
func DefaultOptions() Options {
	return Options{
		PageSize:     DefaultPageSize,
		InitialPages: DefaultInitialPages,
		CreateIfNew:  true,
		ReadOnly:     false,
		SyncOnWrite:  false,
	}
}

// PageManager handles page allocation, deallocation, and I/O operations.
type PageManager struct {
	file        *os.File
	header      *FileHeader
	pageSize    int
	totalPages  uint64
	freeList    *FreeList
	mu          sync.RWMutex
	path        string
	readOnly    bool
	syncOnWrite bool
	closed      bool
}

// OpenPageManager opens or creates a page manager for the given file path.
func OpenPageManager(path string, opts Options) (*PageManager, error) {
	if opts.PageSize == 0 {
		opts.PageSize = DefaultPageSize
	}
	if opts.InitialPages == 0 {
		opts.InitialPages = DefaultInitialPages
	}

	pm := &PageManager{
		pageSize:    opts.PageSize,
		freeList:    NewFreeList(),
		path:        path,
		readOnly:    opts.ReadOnly,
		syncOnWrite: opts.SyncOnWrite,
	}

	// Check if file exists
	_, err := os.Stat(path)
	fileExists := err == nil

	if !fileExists && !opts.CreateIfNew {
		return nil, os.ErrNotExist
	}

	// Open or create the file
	var flags int
	if opts.ReadOnly {
		flags = os.O_RDONLY
	} else {
		flags = os.O_RDWR
		if !fileExists {
			flags |= os.O_CREATE
		}
	}

	pm.file, err = os.OpenFile(path, flags, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	if fileExists {
		// Load existing file
		if err := pm.loadExisting(); err != nil {
			pm.file.Close()
			return nil, err
		}
	} else {
		// Initialize new file
		if err := pm.initializeNew(opts.InitialPages); err != nil {
			pm.file.Close()
			os.Remove(path)
			return nil, err
		}
	}

	return pm, nil
}

// loadExisting loads an existing database file.
func (pm *PageManager) loadExisting() error {
	// Read the header page
	headerBuf := make([]byte, FileHeaderSize)
	if _, err := pm.file.ReadAt(headerBuf, 0); err != nil {
		return fmt.Errorf("failed to read header: %w", err)
	}

	pm.header = &FileHeader{}
	if err := pm.header.DeserializeAndValidate(headerBuf); err != nil {
		return fmt.Errorf("invalid header: %w", err)
	}

	pm.totalPages = pm.header.TotalPages
	pm.pageSize = int(pm.header.PageSize)

	// Load free list
	if err := pm.loadFreeList(); err != nil {
		return fmt.Errorf("failed to load free list: %w", err)
	}

	return nil
}

// loadFreeList loads the free list from disk.
func (pm *PageManager) loadFreeList() error {
	pm.freeList = NewFreeList()

	if pm.header.FreeListHead == 0 {
		return nil
	}

	pm.freeList.SetHead(pm.header.FreeListHead)

	// Read all free list pages
	var pages []*Page
	currentPageID := pm.header.FreeListHead

	for currentPageID != 0 {
		page, err := pm.readPageInternal(currentPageID)
		if err != nil {
			return err
		}
		pages = append(pages, page)
		currentPageID = GetNextPageID(page)
	}

	return pm.freeList.LoadFromPages(pages)
}

// initializeNew initializes a new database file.
func (pm *PageManager) initializeNew(initialPages int) error {
	if initialPages < 1 {
		initialPages = 1
	}

	// Create header
	pm.header = NewFileHeader()
	pm.header.PageSize = uint32(pm.pageSize)
	pm.header.TotalPages = uint64(initialPages)
	pm.totalPages = uint64(initialPages)

	// Write header
	headerBuf, err := pm.header.Serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize header: %w", err)
	}

	if _, err := pm.file.WriteAt(headerBuf, 0); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// Initialize remaining pages as free (except page 0 which is the header)
	for i := 1; i < initialPages; i++ {
		pm.freeList.Push(PageID(i))
	}

	// Extend file to full size
	fileSize := int64(initialPages) * int64(pm.pageSize)
	if err := pm.file.Truncate(fileSize); err != nil {
		return fmt.Errorf("failed to extend file: %w", err)
	}

	if err := pm.file.Sync(); err != nil {
		return fmt.Errorf("failed to sync file: %w", err)
	}

	return nil
}

// Close closes the page manager and flushes all data to disk.
func (pm *PageManager) Close() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.closed {
		return ErrFileClosed
	}

	pm.closed = true

	if pm.file == nil {
		return nil
	}

	// Save free list to disk
	if !pm.readOnly {
		if err := pm.saveFreeListLocked(); err != nil {
			pm.file.Close()
			return fmt.Errorf("failed to save free list: %w", err)
		}

		// Update and save header
		if err := pm.saveHeaderLocked(); err != nil {
			pm.file.Close()
			return fmt.Errorf("failed to save header: %w", err)
		}

		if err := pm.file.Sync(); err != nil {
			pm.file.Close()
			return fmt.Errorf("failed to sync file: %w", err)
		}
	}

	return pm.file.Close()
}

// saveFreeListLocked saves the free list to disk. Must be called with lock held.
func (pm *PageManager) saveFreeListLocked() error {
	freePages := pm.freeList.PeekAll()
	if len(freePages) == 0 {
		pm.header.FreeListHead = 0
		return nil
	}

	// We need to allocate pages for the free list itself
	// For simplicity, we'll use the last pages in the file
	numPagesNeeded := (len(freePages) + MaxFreeListEntriesPerPage - 1) / MaxFreeListEntriesPerPage

	// Grow file if needed to store free list
	currentFilePages := pm.totalPages
	freeListStartPage := currentFilePages

	// Extend file for free list pages
	newTotalPages := currentFilePages + uint64(numPagesNeeded)
	fileSize := int64(newTotalPages) * int64(pm.pageSize)
	if err := pm.file.Truncate(fileSize); err != nil {
		return err
	}

	pm.totalPages = newTotalPages
	pm.header.TotalPages = newTotalPages

	// Write free list pages
	var prevPageID PageID = 0
	startIdx := 0

	for i := numPagesNeeded - 1; i >= 0; i-- {
		pageID := PageID(freeListStartPage + uint64(i))
		page := NewPage(pageID, PageTypeFree)

		// Calculate which entries go on this page
		entriesPerPage := MaxFreeListEntriesPerPage
		pageStartIdx := i * entriesPerPage
		pageEndIdx := pageStartIdx + entriesPerPage
		if pageEndIdx > len(freePages) {
			pageEndIdx = len(freePages)
		}

		// Write entries
		entriesWritten := 0
		for j := pageStartIdx; j < pageEndIdx; j++ {
			offset := 8 + entriesWritten*FreeListEntrySize
			binary.LittleEndian.PutUint64(page.Data[offset:offset+FreeListEntrySize], uint64(freePages[j]))
			entriesWritten++
		}

		page.Header.ItemCount = uint16(entriesWritten)
		SetNextPageID(page, prevPageID)

		if err := pm.writePageInternal(page); err != nil {
			return err
		}

		prevPageID = pageID
		startIdx = pageEndIdx
	}

	pm.header.FreeListHead = prevPageID
	pm.freeList.SetHead(prevPageID)

	_ = startIdx // Silence unused variable

	return nil
}

// saveHeaderLocked saves the header to disk. Must be called with lock held.
func (pm *PageManager) saveHeaderLocked() error {
	pm.header.TotalPages = pm.totalPages
	headerBuf, err := pm.header.Serialize()
	if err != nil {
		return err
	}

	_, err = pm.file.WriteAt(headerBuf, 0)
	return err
}

// AllocatePage allocates a new page of the specified type.
func (pm *PageManager) AllocatePage(pageType PageType) (PageID, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.closed {
		return 0, ErrFileClosed
	}

	if pm.readOnly {
		return 0, errors.New("cannot allocate page in read-only mode")
	}

	// Try to get a page from the free list
	if pageID, ok := pm.freeList.Pop(); ok {
		// Initialize the page
		page := NewPage(pageID, pageType)
		if err := pm.writePageInternal(page); err != nil {
			// Put the page back on the free list
			pm.freeList.Push(pageID)
			return 0, err
		}
		return pageID, nil
	}

	// No free pages, grow the file
	newPageID := PageID(pm.totalPages)
	if err := pm.growFileLocked(1); err != nil {
		return 0, err
	}

	// Initialize the new page
	page := NewPage(newPageID, pageType)
	if err := pm.writePageInternal(page); err != nil {
		return 0, err
	}

	return newPageID, nil
}

// growFileLocked grows the file by the specified number of pages.
// Must be called with lock held.
func (pm *PageManager) growFileLocked(numPages int) error {
	if numPages < MinGrowthPages {
		numPages = MinGrowthPages
	}

	newTotalPages := pm.totalPages + uint64(numPages)
	fileSize := int64(newTotalPages) * int64(pm.pageSize)

	if err := pm.file.Truncate(fileSize); err != nil {
		return fmt.Errorf("failed to grow file: %w", err)
	}

	// Add new pages to free list (except the first one which will be used)
	oldTotal := pm.totalPages
	pm.totalPages = newTotalPages
	pm.header.TotalPages = newTotalPages

	// Add pages after the first new one to free list
	for i := oldTotal + 1; i < newTotalPages; i++ {
		pm.freeList.Push(PageID(i))
	}

	return nil
}

// FreePage marks a page as free for reuse.
func (pm *PageManager) FreePage(id PageID) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.closed {
		return ErrFileClosed
	}

	if pm.readOnly {
		return errors.New("cannot free page in read-only mode")
	}

	// Cannot free page 0 (header)
	if id == 0 {
		return ErrCannotFreeHeader
	}

	// Check if page is in valid range
	if uint64(id) >= pm.totalPages {
		return ErrPageOutOfRange
	}

	// Check if already free
	if pm.freeList.Contains(id) {
		return ErrPageAlreadyFree
	}

	// Clear the page
	page := NewPage(id, PageTypeFree)
	if err := pm.writePageInternal(page); err != nil {
		return err
	}

	// Add to free list
	pm.freeList.Push(id)

	return nil
}

// ReadPage reads a page from disk.
func (pm *PageManager) ReadPage(id PageID) (*Page, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if pm.closed {
		return nil, ErrFileClosed
	}

	return pm.readPageInternal(id)
}

// readPageInternal reads a page without locking.
func (pm *PageManager) readPageInternal(id PageID) (*Page, error) {
	if id == 0 {
		return nil, ErrInvalidPageID
	}

	if uint64(id) >= pm.totalPages {
		return nil, ErrPageOutOfRange
	}

	offset := int64(id) * int64(pm.pageSize)
	buf := make([]byte, pm.pageSize)

	n, err := pm.file.ReadAt(buf, offset)
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("failed to read page %d: %w", id, err)
	}

	if n < pm.pageSize {
		return nil, fmt.Errorf("incomplete page read: got %d bytes, expected %d", n, pm.pageSize)
	}

	page := &Page{}
	if err := page.Deserialize(buf); err != nil {
		return nil, fmt.Errorf("failed to deserialize page %d: %w", id, err)
	}

	return page, nil
}

// ReadPages reads multiple pages from disk in a single operation.
// This is more efficient than calling ReadPage multiple times.
func (pm *PageManager) ReadPages(ids []PageID) ([]*Page, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if pm.closed {
		return nil, ErrFileClosed
	}

	if len(ids) == 0 {
		return []*Page{}, nil
	}

	pages := make([]*Page, len(ids))

	// Read all pages
	for i, id := range ids {
		if id == 0 {
			continue // Skip invalid IDs
		}

		page, err := pm.readPageInternal(id)
		if err != nil {
			continue // Skip failed reads
		}
		pages[i] = page
	}

	return pages, nil
}

// WritePage writes a page to disk.
func (pm *PageManager) WritePage(page *Page) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.closed {
		return ErrFileClosed
	}

	if pm.readOnly {
		return errors.New("cannot write page in read-only mode")
	}

	return pm.writePageInternal(page)
}

// writePageInternal writes a page without locking.
func (pm *PageManager) writePageInternal(page *Page) error {
	if page.Header.PageID == 0 {
		return ErrInvalidPageID
	}

	if uint64(page.Header.PageID) >= pm.totalPages {
		return ErrPageOutOfRange
	}

	offset := int64(page.Header.PageID) * int64(pm.pageSize)

	buf, err := page.Serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize page: %w", err)
	}

	if _, err := pm.file.WriteAt(buf, offset); err != nil {
		return fmt.Errorf("failed to write page %d: %w", page.Header.PageID, err)
	}

	if pm.syncOnWrite {
		if err := pm.file.Sync(); err != nil {
			return fmt.Errorf("failed to sync after write: %w", err)
		}
	}

	return nil
}

// Sync flushes all pending writes to disk.
func (pm *PageManager) Sync() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.closed {
		return ErrFileClosed
	}

	if pm.file == nil {
		return ErrFileNotOpen
	}

	// Save header with current state
	if !pm.readOnly {
		if err := pm.saveHeaderLocked(); err != nil {
			return err
		}
	}

	return pm.file.Sync()
}

// TotalPages returns the total number of pages in the file.
func (pm *PageManager) TotalPages() uint64 {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.totalPages
}

// FreePageCount returns the number of free pages.
func (pm *PageManager) FreePageCount() uint64 {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.freeList.Count()
}

// PageSize returns the page size in bytes.
func (pm *PageManager) PageSize() int {
	return pm.pageSize
}

// Path returns the file path.
func (pm *PageManager) Path() string {
	return pm.path
}

// IsReadOnly returns true if the page manager is in read-only mode.
func (pm *PageManager) IsReadOnly() bool {
	return pm.readOnly
}

// Header returns a copy of the file header.
func (pm *PageManager) Header() FileHeader {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	if pm.header == nil {
		return FileHeader{}
	}
	return *pm.header
}

// UpdateHeader updates the file header with the given values and saves to disk.
func (pm *PageManager) UpdateHeader(header FileHeader) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.closed {
		return ErrFileClosed
	}

	// Update header fields (preserve some internal fields)
	pm.header.RootPages = header.RootPages

	return pm.saveHeaderLocked()
}

// Stats returns statistics about the page manager.
type Stats struct {
	TotalPages    uint64
	FreePages     uint64
	UsedPages     uint64
	PageSize      int
	FileSizeBytes int64
}

// Stats returns current statistics.
func (pm *PageManager) Stats() Stats {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	freeCount := pm.freeList.Count()
	return Stats{
		TotalPages:    pm.totalPages,
		FreePages:     freeCount,
		UsedPages:     pm.totalPages - freeCount - 1, // -1 for header
		PageSize:      pm.pageSize,
		FileSizeBytes: int64(pm.totalPages) * int64(pm.pageSize),
	}
}
