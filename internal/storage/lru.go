// Package storage provides the core storage engine components for ObaDB.
package storage

import "container/list"

// LRUCache implements a Least Recently Used (LRU) cache for page eviction.
// It maintains the order of page access to identify cold pages for eviction.
type LRUCache struct {
	list    *list.List               // Doubly linked list for LRU ordering
	entries map[PageID]*list.Element // Map for O(1) lookup
}

// lruEntry represents an entry in the LRU cache.
type lruEntry struct {
	pageID PageID
}

// NewLRUCache creates a new LRU cache.
func NewLRUCache() *LRUCache {
	return &LRUCache{
		list:    list.New(),
		entries: make(map[PageID]*list.Element),
	}
}

// Access marks a page as recently accessed, moving it to the front of the list.
// If the page is not in the cache, it is added.
func (c *LRUCache) Access(pageID PageID) {
	if elem, exists := c.entries[pageID]; exists {
		// Move to front (most recently used)
		c.list.MoveToFront(elem)
		return
	}

	// Add new entry at front
	entry := &lruEntry{pageID: pageID}
	elem := c.list.PushFront(entry)
	c.entries[pageID] = elem
}

// Remove removes a page from the LRU cache.
func (c *LRUCache) Remove(pageID PageID) {
	if elem, exists := c.entries[pageID]; exists {
		c.list.Remove(elem)
		delete(c.entries, pageID)
	}
}

// GetLRU returns the least recently used page ID.
// Returns the page ID and true if the cache is not empty, otherwise 0 and false.
func (c *LRUCache) GetLRU() (PageID, bool) {
	if c.list.Len() == 0 {
		return 0, false
	}

	// Back of the list is the least recently used
	elem := c.list.Back()
	if elem == nil {
		return 0, false
	}

	entry := elem.Value.(*lruEntry)
	return entry.pageID, true
}

// GetLRUExcluding returns the least recently used page ID that is not in the excluded set.
// This is useful for finding eviction candidates while skipping pinned pages.
func (c *LRUCache) GetLRUExcluding(excluded map[PageID]bool) (PageID, bool) {
	// Iterate from back (LRU) to front (MRU)
	for elem := c.list.Back(); elem != nil; elem = elem.Prev() {
		entry := elem.Value.(*lruEntry)
		if !excluded[entry.pageID] {
			return entry.pageID, true
		}
	}
	return 0, false
}

// Contains checks if a page is in the LRU cache.
func (c *LRUCache) Contains(pageID PageID) bool {
	_, exists := c.entries[pageID]
	return exists
}

// Len returns the number of entries in the LRU cache.
func (c *LRUCache) Len() int {
	return c.list.Len()
}

// Clear removes all entries from the LRU cache.
func (c *LRUCache) Clear() {
	c.list.Init()
	c.entries = make(map[PageID]*list.Element)
}

// GetAll returns all page IDs in the cache, ordered from most to least recently used.
func (c *LRUCache) GetAll() []PageID {
	result := make([]PageID, 0, c.list.Len())
	for elem := c.list.Front(); elem != nil; elem = elem.Next() {
		entry := elem.Value.(*lruEntry)
		result = append(result, entry.pageID)
	}
	return result
}

// GetAllLRUOrder returns all page IDs in the cache, ordered from least to most recently used.
func (c *LRUCache) GetAllLRUOrder() []PageID {
	result := make([]PageID, 0, c.list.Len())
	for elem := c.list.Back(); elem != nil; elem = elem.Prev() {
		entry := elem.Value.(*lruEntry)
		result = append(result, entry.pageID)
	}
	return result
}
