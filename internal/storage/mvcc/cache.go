package mvcc

import (
	"container/list"
	"sync"

	"github.com/oba-ldap/oba/internal/storage"
)

const DefaultCacheSize = 10000

type EntryCache struct {
	entries  map[string]*CachedEntry
	lru      *list.List
	lruIndex map[string]*list.Element
	maxSize  int
	mu       sync.RWMutex

	hits   uint64
	misses uint64
}

type CachedEntry struct {
	DN      string
	Version *Version
	PageID  storage.PageID
	SlotID  uint16
}

func NewEntryCache(maxSize int) *EntryCache {
	if maxSize <= 0 {
		maxSize = DefaultCacheSize
	}
	return &EntryCache{
		entries:  make(map[string]*CachedEntry),
		lru:      list.New(),
		lruIndex: make(map[string]*list.Element),
		maxSize:  maxSize,
	}
}

func (c *EntryCache) Get(dn string) *CachedEntry {
	c.mu.RLock()
	entry, exists := c.entries[dn]
	c.mu.RUnlock()

	if !exists {
		c.mu.Lock()
		c.misses++
		c.mu.Unlock()
		return nil
	}

	c.mu.Lock()
	c.hits++
	if elem, ok := c.lruIndex[dn]; ok {
		c.lru.MoveToFront(elem)
	}
	c.mu.Unlock()

	return entry
}

func (c *EntryCache) Put(dn string, version *Version, pageID storage.PageID, slotID uint16) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if existing, exists := c.entries[dn]; exists {
		existing.Version = version
		existing.PageID = pageID
		existing.SlotID = slotID
		if elem, ok := c.lruIndex[dn]; ok {
			c.lru.MoveToFront(elem)
		}
		return
	}

	if len(c.entries) >= c.maxSize {
		c.evictLRU()
	}

	entry := &CachedEntry{
		DN:      dn,
		Version: version,
		PageID:  pageID,
		SlotID:  slotID,
	}
	c.entries[dn] = entry
	elem := c.lru.PushFront(dn)
	c.lruIndex[dn] = elem
}

func (c *EntryCache) Delete(dn string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, exists := c.lruIndex[dn]; exists {
		c.lru.Remove(elem)
		delete(c.lruIndex, dn)
	}
	delete(c.entries, dn)
}

func (c *EntryCache) evictLRU() {
	elem := c.lru.Back()
	if elem == nil {
		return
	}

	dn := elem.Value.(string)
	c.lru.Remove(elem)
	delete(c.lruIndex, dn)
	delete(c.entries, dn)
}

func (c *EntryCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

func (c *EntryCache) Stats() (hits, misses uint64) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.hits, c.misses
}

func (c *EntryCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*CachedEntry)
	c.lru.Init()
	c.lruIndex = make(map[string]*list.Element)
	c.hits = 0
	c.misses = 0
}
