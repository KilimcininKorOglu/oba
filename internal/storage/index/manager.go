// Package index provides the Index Manager for coordinating multiple B+ Tree indexes
// for different attributes in ObaDB.
package index

import (
	"encoding/binary"
	"errors"
	"strings"
	"sync"

	"github.com/oba-ldap/oba/internal/storage"
	"github.com/oba-ldap/oba/internal/storage/btree"
)

// Index Manager errors.
var (
	ErrIndexExists       = errors.New("index already exists")
	ErrIndexNotFound     = errors.New("index not found")
	ErrInvalidAttribute  = errors.New("invalid attribute name")
	ErrManagerClosed     = errors.New("index manager is closed")
	ErrInvalidPageManager = errors.New("invalid page manager")
	ErrMetadataCorrupted = errors.New("index metadata corrupted")
)

// Metadata page constants.
const (
	// MetadataPageType is a marker byte for index metadata pages.
	MetadataPageType byte = 0xAA

	// MaxAttributeNameLength is the maximum length of an attribute name.
	MaxAttributeNameLength = 256

	// MetadataEntrySize is the size of each index metadata entry.
	// Layout: 1 byte type marker + 1 byte index type + 8 bytes root page ID + 2 bytes attr len + attr name
	MetadataEntryHeaderSize = 12
)

// IndexManager coordinates multiple B+ Tree indexes for different attributes.
// It handles index creation, deletion, and maintenance during entry modifications.
type IndexManager struct {
	// indexes maps attribute names to their Index structures.
	indexes map[string]*Index

	// pageManager is the underlying page manager for storage.
	pageManager *storage.PageManager

	// metadataPageID is the page ID where index metadata is stored.
	metadataPageID storage.PageID

	// mu protects concurrent access to the index manager.
	mu sync.RWMutex

	// closed indicates if the manager has been closed.
	closed bool
}

// NewIndexManager creates a new IndexManager with the given PageManager.
// It initializes default indexes for commonly searched attributes.
func NewIndexManager(pm *storage.PageManager) (*IndexManager, error) {
	if pm == nil {
		return nil, ErrInvalidPageManager
	}

	im := &IndexManager{
		indexes:     make(map[string]*Index),
		pageManager: pm,
	}

	// Try to load existing metadata
	if err := im.loadMetadata(); err != nil {
		// No existing metadata, create new
		if err := im.initializeMetadata(); err != nil {
			return nil, err
		}

		// Create default indexes
		if err := im.createDefaultIndexes(); err != nil {
			return nil, err
		}
	}

	return im, nil
}

// initializeMetadata allocates a page for storing index metadata.
func (im *IndexManager) initializeMetadata() error {
	pageID, err := im.pageManager.AllocatePage(storage.PageTypeAttrIndex)
	if err != nil {
		return err
	}

	im.metadataPageID = pageID

	// Write initial empty metadata page
	return im.saveMetadata()
}

// loadMetadata loads index metadata from the first available metadata page.
// It scans pages to find the metadata page.
func (im *IndexManager) loadMetadata() error {
	// Scan for metadata page starting from page 1
	totalPages := im.pageManager.TotalPages()

	for pageID := storage.PageID(1); uint64(pageID) < totalPages; pageID++ {
		page, err := im.pageManager.ReadPage(pageID)
		if err != nil {
			continue
		}

		// Check if this is an index metadata page
		if page.Header.PageType == storage.PageTypeAttrIndex && len(page.Data) > 0 && page.Data[0] == MetadataPageType {
			im.metadataPageID = pageID
			return im.parseMetadataPage(page)
		}
	}

	return errors.New("no metadata page found")
}

// parseMetadataPage parses index metadata from a page.
func (im *IndexManager) parseMetadataPage(page *storage.Page) error {
	data := page.Data
	if len(data) < 1 || data[0] != MetadataPageType {
		return ErrMetadataCorrupted
	}

	offset := 1 // Skip type marker

	// Read number of indexes
	if offset+2 > len(data) {
		return ErrMetadataCorrupted
	}
	numIndexes := binary.LittleEndian.Uint16(data[offset:])
	offset += 2

	// Read each index metadata
	for i := uint16(0); i < numIndexes; i++ {
		if offset+MetadataEntryHeaderSize > len(data) {
			return ErrMetadataCorrupted
		}

		// Read index type
		indexType := IndexType(data[offset])
		offset++

		// Read root page ID
		rootPageID := storage.PageID(binary.LittleEndian.Uint64(data[offset:]))
		offset += 8

		// Read attribute name length
		attrLen := binary.LittleEndian.Uint16(data[offset:])
		offset += 2

		// Validate attribute length
		if attrLen > MaxAttributeNameLength || offset+int(attrLen) > len(data) {
			return ErrMetadataCorrupted
		}

		// Read attribute name
		attribute := string(data[offset : offset+int(attrLen)])
		offset += int(attrLen)

		// Load the B+ Tree for this index
		tree, err := btree.NewBPlusTreeWithRoot(im.pageManager, rootPageID, 0)
		if err != nil {
			return err
		}

		im.indexes[attribute] = &Index{
			Attribute:  attribute,
			Type:       indexType,
			Tree:       tree,
			RootPageID: rootPageID,
		}
	}

	return nil
}

// saveMetadata saves index metadata to the metadata page.
func (im *IndexManager) saveMetadata() error {
	page, err := im.pageManager.ReadPage(im.metadataPageID)
	if err != nil {
		// Create new page if it doesn't exist
		page = storage.NewPage(im.metadataPageID, storage.PageTypeAttrIndex)
	}

	// Clear page data
	for i := range page.Data {
		page.Data[i] = 0
	}

	offset := 0

	// Write type marker
	page.Data[offset] = MetadataPageType
	offset++

	// Write number of indexes
	binary.LittleEndian.PutUint16(page.Data[offset:], uint16(len(im.indexes)))
	offset += 2

	// Write each index metadata
	for attr, idx := range im.indexes {
		// Write index type
		page.Data[offset] = byte(idx.Type)
		offset++

		// Write root page ID
		binary.LittleEndian.PutUint64(page.Data[offset:], uint64(idx.RootPageID))
		offset += 8

		// Write attribute name length
		attrBytes := []byte(attr)
		binary.LittleEndian.PutUint16(page.Data[offset:], uint16(len(attrBytes)))
		offset += 2

		// Write attribute name
		copy(page.Data[offset:], attrBytes)
		offset += len(attrBytes)
	}

	page.Header.ItemCount = uint16(len(im.indexes))

	return im.pageManager.WritePage(page)
}

// createDefaultIndexes creates indexes for commonly searched attributes.
func (im *IndexManager) createDefaultIndexes() error {
	for _, attr := range DefaultIndexedAttributes() {
		if err := im.createIndexInternal(attr, IndexEquality); err != nil {
			return err
		}
	}
	return nil
}

// CreateIndex creates a new index for the given attribute.
// Returns ErrIndexExists if an index already exists for this attribute.
func (im *IndexManager) CreateIndex(attr string, indexType IndexType) error {
	im.mu.Lock()
	defer im.mu.Unlock()

	if im.closed {
		return ErrManagerClosed
	}

	return im.createIndexInternal(attr, indexType)
}

// createIndexInternal creates an index without locking (caller must hold lock).
func (im *IndexManager) createIndexInternal(attr string, indexType IndexType) error {
	// Normalize attribute name to lowercase
	attr = strings.ToLower(strings.TrimSpace(attr))

	if attr == "" {
		return ErrInvalidAttribute
	}

	if len(attr) > MaxAttributeNameLength {
		return ErrInvalidAttribute
	}

	// Check if index already exists
	if _, exists := im.indexes[attr]; exists {
		return ErrIndexExists
	}

	// Create a new B+ Tree for this index
	tree, err := btree.NewBPlusTree(im.pageManager, 0)
	if err != nil {
		return err
	}

	im.indexes[attr] = &Index{
		Attribute:  attr,
		Type:       indexType,
		Tree:       tree,
		RootPageID: tree.Root(),
	}

	// Persist metadata
	return im.saveMetadata()
}

// DropIndex removes an index for the given attribute.
// It cleans up all pages used by the index.
func (im *IndexManager) DropIndex(attr string) error {
	im.mu.Lock()
	defer im.mu.Unlock()

	if im.closed {
		return ErrManagerClosed
	}

	// Normalize attribute name
	attr = strings.ToLower(strings.TrimSpace(attr))

	idx, exists := im.indexes[attr]
	if !exists {
		return ErrIndexNotFound
	}

	// Clean up all pages used by the B+ Tree
	if err := im.cleanupTreePages(idx.Tree); err != nil {
		return err
	}

	// Remove from indexes map
	delete(im.indexes, attr)

	// Persist metadata
	return im.saveMetadata()
}

// cleanupTreePages frees all pages used by a B+ Tree.
func (im *IndexManager) cleanupTreePages(tree *btree.BPlusTree) error {
	if tree == nil || tree.Root() == btree.InvalidPageID {
		return nil
	}

	// Collect all page IDs used by the tree
	pageIDs, err := im.collectTreePages(tree.Root())
	if err != nil {
		return err
	}

	// Free all pages
	for _, pageID := range pageIDs {
		if err := im.pageManager.FreePage(pageID); err != nil {
			// Continue freeing other pages even if one fails
			continue
		}
	}

	return nil
}

// collectTreePages collects all page IDs used by a B+ Tree starting from the given root.
func (im *IndexManager) collectTreePages(rootPageID storage.PageID) ([]storage.PageID, error) {
	var pageIDs []storage.PageID
	visited := make(map[storage.PageID]bool)

	var collect func(pageID storage.PageID) error
	collect = func(pageID storage.PageID) error {
		if pageID == btree.InvalidPageID || visited[pageID] {
			return nil
		}

		visited[pageID] = true
		pageIDs = append(pageIDs, pageID)

		page, err := im.pageManager.ReadPage(pageID)
		if err != nil {
			return err
		}

		// Parse the node to find children
		node := &btree.BPlusNode{}
		if err := node.DeserializeFromPage(page); err != nil {
			return nil // Skip invalid nodes
		}

		// Collect children for internal nodes
		if !node.IsLeaf {
			for _, childID := range node.Children {
				if err := collect(childID); err != nil {
					return err
				}
			}
		} else {
			// For leaf nodes, follow the next pointer
			if node.Next != btree.InvalidPageID && !visited[node.Next] {
				if err := collect(node.Next); err != nil {
					return err
				}
			}
		}

		return nil
	}

	if err := collect(rootPageID); err != nil {
		return nil, err
	}

	return pageIDs, nil
}

// GetIndex returns the index for the given attribute.
// Returns (nil, false) if no index exists for this attribute.
func (im *IndexManager) GetIndex(attr string) (*Index, bool) {
	im.mu.RLock()
	defer im.mu.RUnlock()

	if im.closed {
		return nil, false
	}

	// Normalize attribute name
	attr = strings.ToLower(strings.TrimSpace(attr))

	idx, exists := im.indexes[attr]
	return idx, exists
}

// UpdateIndexes updates all indexes when an entry is modified.
// It removes old values and adds new values atomically.
func (im *IndexManager) UpdateIndexes(oldEntry, newEntry *Entry) error {
	im.mu.Lock()
	defer im.mu.Unlock()

	if im.closed {
		return ErrManagerClosed
	}

	// Handle deletion (oldEntry != nil, newEntry == nil)
	if newEntry == nil && oldEntry != nil {
		return im.removeFromIndexes(oldEntry)
	}

	// Handle insertion (oldEntry == nil, newEntry != nil)
	if oldEntry == nil && newEntry != nil {
		return im.addToIndexes(newEntry)
	}

	// Handle update (both non-nil)
	if oldEntry != nil && newEntry != nil {
		// Remove old values first
		if err := im.removeFromIndexes(oldEntry); err != nil {
			return err
		}
		// Add new values
		return im.addToIndexes(newEntry)
	}

	// Both nil - nothing to do
	return nil
}

// addToIndexes adds an entry's attribute values to all relevant indexes.
func (im *IndexManager) addToIndexes(entry *Entry) error {
	if entry == nil {
		return nil
	}

	ref := entry.EntryRef()

	for attr, idx := range im.indexes {
		values := entry.GetAttribute(attr)
		if len(values) == 0 {
			continue
		}

		for _, value := range values {
			if len(value) == 0 {
				continue
			}

			// For equality indexes, use the value as the key
			if idx.Type == IndexEquality {
				if err := idx.Tree.Insert(value, ref); err != nil {
					return err
				}
			}

			// For presence indexes, use a marker
			if idx.Type == IndexPresence {
				if err := idx.Tree.Insert(PresenceMarker, ref); err != nil {
					return err
				}
				break // Only need one entry for presence
			}

			// For substring indexes, create multiple entries for substrings
			if idx.Type == IndexSubstring {
				substrings := generateSubstrings(value)
				for _, substr := range substrings {
					if err := idx.Tree.Insert(substr, ref); err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}

// removeFromIndexes removes an entry's attribute values from all relevant indexes.
func (im *IndexManager) removeFromIndexes(entry *Entry) error {
	if entry == nil {
		return nil
	}

	ref := entry.EntryRef()

	for attr, idx := range im.indexes {
		values := entry.GetAttribute(attr)
		if len(values) == 0 {
			continue
		}

		for _, value := range values {
			if len(value) == 0 {
				continue
			}

			// For equality indexes, delete the value
			if idx.Type == IndexEquality {
				// Ignore not found errors during deletion
				_ = idx.Tree.Delete(value, ref)
			}

			// For presence indexes, delete the marker
			if idx.Type == IndexPresence {
				_ = idx.Tree.Delete(PresenceMarker, ref)
				break
			}

			// For substring indexes, delete all substrings
			if idx.Type == IndexSubstring {
				substrings := generateSubstrings(value)
				for _, substr := range substrings {
					_ = idx.Tree.Delete(substr, ref)
				}
			}
		}
	}

	return nil
}

// generateSubstrings generates all substrings of a value for substring indexing.
// This is used for substring searches like (cn=*admin*).
func generateSubstrings(value []byte) [][]byte {
	if len(value) == 0 {
		return nil
	}

	var substrings [][]byte
	minLen := 3 // Minimum substring length

	// Generate all substrings of length >= minLen
	for start := 0; start < len(value); start++ {
		for end := start + minLen; end <= len(value); end++ {
			substr := make([]byte, end-start)
			copy(substr, value[start:end])
			substrings = append(substrings, substr)
		}
	}

	return substrings
}

// ListIndexes returns a list of all indexed attributes.
func (im *IndexManager) ListIndexes() []string {
	im.mu.RLock()
	defer im.mu.RUnlock()

	attrs := make([]string, 0, len(im.indexes))
	for attr := range im.indexes {
		attrs = append(attrs, attr)
	}
	return attrs
}

// IndexCount returns the number of indexes.
func (im *IndexManager) IndexCount() int {
	im.mu.RLock()
	defer im.mu.RUnlock()
	return len(im.indexes)
}

// Close closes the index manager and persists all metadata.
func (im *IndexManager) Close() error {
	im.mu.Lock()
	defer im.mu.Unlock()

	if im.closed {
		return ErrManagerClosed
	}

	im.closed = true

	// Save final metadata
	return im.saveMetadata()
}

// Search searches for entries matching the given attribute value.
// Returns entry references that match.
func (im *IndexManager) Search(attr string, value []byte) ([]btree.EntryRef, error) {
	im.mu.RLock()
	defer im.mu.RUnlock()

	if im.closed {
		return nil, ErrManagerClosed
	}

	attr = strings.ToLower(strings.TrimSpace(attr))

	idx, exists := im.indexes[attr]
	if !exists {
		return nil, ErrIndexNotFound
	}

	return idx.Tree.Search(value)
}

// SearchPresence searches for entries that have the given attribute.
func (im *IndexManager) SearchPresence(attr string) ([]btree.EntryRef, error) {
	im.mu.RLock()
	defer im.mu.RUnlock()

	if im.closed {
		return nil, ErrManagerClosed
	}

	attr = strings.ToLower(strings.TrimSpace(attr))

	idx, exists := im.indexes[attr]
	if !exists {
		return nil, ErrIndexNotFound
	}

	// For presence searches, we search for the presence marker
	return idx.Tree.Search(PresenceMarker)
}

// SearchRange searches for entries with attribute values in the given range.
func (im *IndexManager) SearchRange(attr string, startValue, endValue []byte) ([]btree.EntryRef, error) {
	im.mu.RLock()
	defer im.mu.RUnlock()

	if im.closed {
		return nil, ErrManagerClosed
	}

	attr = strings.ToLower(strings.TrimSpace(attr))

	idx, exists := im.indexes[attr]
	if !exists {
		return nil, ErrIndexNotFound
	}

	return idx.Tree.SearchRange(startValue, endValue)
}

// Sync persists all index metadata to disk.
func (im *IndexManager) Sync() error {
	im.mu.Lock()
	defer im.mu.Unlock()

	if im.closed {
		return ErrManagerClosed
	}

	return im.saveMetadata()
}
