// Package storage provides the core storage engine components for ObaDB.
package storage

import (
	"encoding/binary"
	"sync"
)

// FreeListEntrySize is the size of each entry in the free list (8 bytes for PageID).
const FreeListEntrySize = 8

// MaxFreeListEntriesPerPage is the maximum number of free page entries per page.
// Calculated as: (PageSize - PageHeaderSize - 8 bytes for next pointer) / 8 bytes per entry
const MaxFreeListEntriesPerPage = (PageSize - PageHeaderSize - 8) / FreeListEntrySize

// FreeList manages free pages in the database.
// It uses a linked list of pages, where each page contains an array of free page IDs.
// Layout of a free list page:
//   - Bytes 0-15:    PageHeader
//   - Bytes 16-23:   NextPage (PageID of next free list page, 0 if none)
//   - Bytes 24-...:  Array of free PageIDs
type FreeList struct {
	head      PageID       // First page in the free list chain
	count     uint64       // Total number of free pages
	freePages []PageID     // In-memory cache of free pages
	mu        sync.RWMutex // Protects the free list
}

// NewFreeList creates a new empty FreeList.
func NewFreeList() *FreeList {
	return &FreeList{
		head:      0,
		count:     0,
		freePages: make([]PageID, 0),
	}
}

// NewFreeListWithHead creates a FreeList with an existing head page.
func NewFreeListWithHead(head PageID) *FreeList {
	return &FreeList{
		head:      head,
		count:     0,
		freePages: make([]PageID, 0),
	}
}

// Head returns the head page ID of the free list.
func (fl *FreeList) Head() PageID {
	fl.mu.RLock()
	defer fl.mu.RUnlock()
	return fl.head
}

// SetHead sets the head page ID of the free list.
func (fl *FreeList) SetHead(head PageID) {
	fl.mu.Lock()
	defer fl.mu.Unlock()
	fl.head = head
}

// Count returns the total number of free pages.
func (fl *FreeList) Count() uint64 {
	fl.mu.RLock()
	defer fl.mu.RUnlock()
	return fl.count
}

// IsEmpty returns true if there are no free pages.
func (fl *FreeList) IsEmpty() bool {
	fl.mu.RLock()
	defer fl.mu.RUnlock()
	return fl.count == 0 && len(fl.freePages) == 0
}

// Push adds a page ID to the free list.
func (fl *FreeList) Push(pageID PageID) {
	fl.mu.Lock()
	defer fl.mu.Unlock()
	fl.freePages = append(fl.freePages, pageID)
	fl.count++
}

// Pop removes and returns a page ID from the free list.
// Returns 0 and false if the free list is empty.
func (fl *FreeList) Pop() (PageID, bool) {
	fl.mu.Lock()
	defer fl.mu.Unlock()

	if len(fl.freePages) == 0 {
		return 0, false
	}

	// Pop from the end (LIFO for better locality)
	idx := len(fl.freePages) - 1
	pageID := fl.freePages[idx]
	fl.freePages = fl.freePages[:idx]
	fl.count--

	return pageID, true
}

// PeekAll returns a copy of all free page IDs in memory.
func (fl *FreeList) PeekAll() []PageID {
	fl.mu.RLock()
	defer fl.mu.RUnlock()

	result := make([]PageID, len(fl.freePages))
	copy(result, fl.freePages)
	return result
}

// LoadFromPages loads free page IDs from a chain of free list pages.
// This is called during database open to restore the free list state.
func (fl *FreeList) LoadFromPages(pages []*Page) error {
	fl.mu.Lock()
	defer fl.mu.Unlock()

	fl.freePages = make([]PageID, 0)
	fl.count = 0

	for _, page := range pages {
		if page == nil {
			continue
		}

		// Read the number of entries from ItemCount
		numEntries := int(page.Header.ItemCount)

		// Read each free page ID (starting after the next pointer at offset 8)
		for i := 0; i < numEntries && i < MaxFreeListEntriesPerPage; i++ {
			offset := 8 + i*FreeListEntrySize
			if offset+FreeListEntrySize > len(page.Data) {
				break
			}
			pageID := PageID(binary.LittleEndian.Uint64(page.Data[offset : offset+FreeListEntrySize]))
			if pageID != 0 {
				fl.freePages = append(fl.freePages, pageID)
				fl.count++
			}
		}
	}

	return nil
}

// SerializeToPage writes the free list entries to a page.
// Returns the next page ID if there are more entries than fit in one page.
func (fl *FreeList) SerializeToPage(page *Page, startIdx int) (nextIdx int, hasMore bool) {
	fl.mu.RLock()
	defer fl.mu.RUnlock()

	if page == nil || startIdx >= len(fl.freePages) {
		return startIdx, false
	}

	// Clear the data area
	for i := range page.Data {
		page.Data[i] = 0
	}

	// Write entries starting from startIdx
	entriesWritten := 0
	for i := startIdx; i < len(fl.freePages) && entriesWritten < MaxFreeListEntriesPerPage; i++ {
		offset := 8 + entriesWritten*FreeListEntrySize
		binary.LittleEndian.PutUint64(page.Data[offset:offset+FreeListEntrySize], uint64(fl.freePages[i]))
		entriesWritten++
	}

	page.Header.ItemCount = uint16(entriesWritten)
	page.Header.PageType = PageTypeFree

	nextIdx = startIdx + entriesWritten
	hasMore = nextIdx < len(fl.freePages)

	return nextIdx, hasMore
}

// GetNextPageID reads the next page pointer from a free list page.
func GetNextPageID(page *Page) PageID {
	if page == nil || len(page.Data) < 8 {
		return 0
	}
	return PageID(binary.LittleEndian.Uint64(page.Data[0:8]))
}

// SetNextPageID writes the next page pointer to a free list page.
func SetNextPageID(page *Page, nextPageID PageID) {
	if page == nil || len(page.Data) < 8 {
		return
	}
	binary.LittleEndian.PutUint64(page.Data[0:8], uint64(nextPageID))
}

// Clear removes all entries from the free list.
func (fl *FreeList) Clear() {
	fl.mu.Lock()
	defer fl.mu.Unlock()
	fl.freePages = make([]PageID, 0)
	fl.count = 0
	fl.head = 0
}

// Contains checks if a page ID is in the free list.
func (fl *FreeList) Contains(pageID PageID) bool {
	fl.mu.RLock()
	defer fl.mu.RUnlock()

	for _, id := range fl.freePages {
		if id == pageID {
			return true
		}
	}
	return false
}

// Remove removes a specific page ID from the free list.
// Returns true if the page was found and removed.
func (fl *FreeList) Remove(pageID PageID) bool {
	fl.mu.Lock()
	defer fl.mu.Unlock()

	for i, id := range fl.freePages {
		if id == pageID {
			// Remove by swapping with last element
			fl.freePages[i] = fl.freePages[len(fl.freePages)-1]
			fl.freePages = fl.freePages[:len(fl.freePages)-1]
			fl.count--
			return true
		}
	}
	return false
}
