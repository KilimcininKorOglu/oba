package radix

import (
	"errors"
	"sync"

	"github.com/oba-ldap/oba/internal/storage"
)

// Tree errors.
var (
	ErrTreeNotInitialized = errors.New("radix tree not initialized")
	ErrNodeNotFound       = errors.New("node not found")
	ErrEntryNotFound      = errors.New("entry not found")
	ErrEntryExists        = errors.New("entry already exists")
	ErrInvalidPageManager = errors.New("invalid page manager")
)

// RadixTree represents a radix tree for DN hierarchy traversal.
// It provides efficient lookup, insertion, and deletion of LDAP entries
// based on their Distinguished Names.
type RadixTree struct {
	root        *Node
	pageManager *storage.PageManager
	rootPageID  storage.PageID
	mu          sync.RWMutex
	dirty       bool
}

// NewRadixTree creates a new RadixTree with the given PageManager.
// If rootPageID is 0, a new root page will be allocated.
func NewRadixTree(pm *storage.PageManager) (*RadixTree, error) {
	if pm == nil {
		return nil, ErrInvalidPageManager
	}

	tree := &RadixTree{
		root:        NewRootNode(),
		pageManager: pm,
		rootPageID:  0,
		dirty:       false,
	}

	// Allocate a root page
	pageID, err := pm.AllocatePage(storage.PageTypeDNIndex)
	if err != nil {
		return nil, err
	}
	tree.rootPageID = pageID

	// Persist the initial empty tree
	if err := tree.persistRoot(); err != nil {
		return nil, err
	}

	return tree, nil
}

// NewRadixTreeWithRoot creates a RadixTree loading from an existing root page.
func NewRadixTreeWithRoot(pm *storage.PageManager, rootPageID storage.PageID) (*RadixTree, error) {
	if pm == nil {
		return nil, ErrInvalidPageManager
	}

	tree := &RadixTree{
		pageManager: pm,
		rootPageID:  rootPageID,
		dirty:       false,
	}

	// Load the root from disk
	if err := tree.loadRoot(); err != nil {
		return nil, err
	}

	return tree, nil
}

// loadRoot loads the root node from the root page.
func (t *RadixTree) loadRoot() error {
	page, err := t.pageManager.ReadPage(t.rootPageID)
	if err != nil {
		return err
	}

	// Serialize the page to get the full buffer including header
	buf, err := page.Serialize()
	if err != nil {
		return err
	}

	root, err := DeserializeFromPage(buf)
	if err != nil {
		return err
	}

	if root == nil {
		t.root = NewRootNode()
	} else {
		t.root = root
	}

	return nil
}

// persistRoot persists the root node to the root page.
func (t *RadixTree) persistRoot() error {
	buf, err := SerializeToPage(t.root, t.rootPageID)
	if err != nil {
		return err
	}

	page := &storage.Page{
		Header: storage.PageHeader{
			PageID:   t.rootPageID,
			PageType: storage.PageTypeDNIndex,
		},
		Data: buf[storage.PageHeaderSize:],
	}

	// Copy the header from serialized buffer
	if err := page.Header.Deserialize(buf[:storage.PageHeaderSize]); err != nil {
		return err
	}

	return t.pageManager.WritePage(page)
}

// Root returns the root node of the tree.
func (t *RadixTree) Root() *Node {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.root
}

// RootPageID returns the page ID of the root page.
func (t *RadixTree) RootPageID() storage.PageID {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.rootPageID
}

// IsDirty returns true if the tree has been modified since last persist.
func (t *RadixTree) IsDirty() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.dirty
}

// markDirty marks the tree as dirty.
func (t *RadixTree) markDirty() {
	t.dirty = true
}

// clearDirty clears the dirty flag.
func (t *RadixTree) clearDirty() {
	t.dirty = false
}

// Persist persists the tree to disk if it has been modified.
func (t *RadixTree) Persist() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.dirty {
		return nil
	}

	if err := t.persistRoot(); err != nil {
		return err
	}

	t.clearDirty()
	return nil
}

// EntryCount returns the total number of entries in the tree.
func (t *RadixTree) EntryCount() uint32 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.root == nil {
		return 0
	}
	return t.root.SubtreeCount
}

// traverseToNode traverses the tree to find or create the node for the given DN components.
// If create is true, intermediate nodes are created as needed.
// Returns the node and whether it was found (or created).
func (t *RadixTree) traverseToNode(components []string, create bool) (*Node, bool) {
	if t.root == nil {
		return nil, false
	}

	current := t.root

	for _, comp := range components {
		child := current.GetChild(comp)
		if child == nil {
			if !create {
				return nil, false
			}
			// Create intermediate node
			child = NewNode(comp)
			current.AddChild(child)
		}
		current = child
	}

	return current, true
}

// findNode finds the node for the given DN components.
// Returns nil if not found.
func (t *RadixTree) findNode(components []string) *Node {
	node, found := t.traverseToNode(components, false)
	if !found {
		return nil
	}
	return node
}

// TreeStats holds statistics about the radix tree.
type TreeStats struct {
	TotalNodes   int
	EntryNodes   int
	MaxDepth     int
	TotalEntries uint32
}

// Stats returns statistics about the tree.
func (t *RadixTree) Stats() TreeStats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	stats := TreeStats{}
	if t.root == nil {
		return stats
	}

	t.collectStats(t.root, 0, &stats)
	stats.TotalEntries = t.root.SubtreeCount
	return stats
}

// collectStats recursively collects statistics.
func (t *RadixTree) collectStats(node *Node, depth int, stats *TreeStats) {
	if node == nil {
		return
	}

	stats.TotalNodes++
	if node.HasEntry {
		stats.EntryNodes++
	}
	if depth > stats.MaxDepth {
		stats.MaxDepth = depth
	}

	// Use ChildrenByKey if available
	if node.ChildrenByKey != nil && len(node.ChildrenByKey) > 0 {
		for _, child := range node.ChildrenByKey {
			t.collectStats(child, depth+1, stats)
		}
	} else {
		for _, child := range node.Children {
			t.collectStats(child, depth+1, stats)
		}
	}
}

// Clear removes all entries from the tree.
func (t *RadixTree) Clear() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.root = NewRootNode()
	t.markDirty()

	return t.persistRoot()
}
