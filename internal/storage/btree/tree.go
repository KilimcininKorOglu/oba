// Package btree provides B+ Tree implementation for attribute indexing in ObaDB.
package btree

import (
	"errors"
	"sync"

	"github.com/oba-ldap/oba/internal/storage"
)

// Tree errors.
var (
	ErrTreeNotInitialized = errors.New("b+ tree not initialized")
	ErrKeyNotFound        = errors.New("key not found")
	ErrKeyExists          = errors.New("key already exists")
	ErrInvalidPageManager = errors.New("invalid page manager")
	ErrEmptyKey           = errors.New("key cannot be empty")
	ErrNodeNotFound       = errors.New("node not found")
	ErrInvalidNode        = errors.New("invalid node")
	ErrTreeEmpty          = errors.New("tree is empty")
)

// BPlusTree represents a B+ Tree for attribute indexing.
// It provides efficient lookup, insertion, and deletion of attribute values.
type BPlusTree struct {
	root        storage.PageID
	pageManager *storage.PageManager
	order       int
	mu          sync.RWMutex
}

// NewBPlusTree creates a new BPlusTree with the given PageManager and order.
// The order determines the maximum number of children per internal node.
// If order is 0, the default BPlusOrder is used.
func NewBPlusTree(pm *storage.PageManager, order int) (*BPlusTree, error) {
	if pm == nil {
		return nil, ErrInvalidPageManager
	}

	if order <= 0 {
		order = BPlusOrder
	}

	tree := &BPlusTree{
		root:        InvalidPageID,
		pageManager: pm,
		order:       order,
	}

	// Allocate a root page (initially a leaf node)
	pageID, err := pm.AllocatePage(storage.PageTypeAttrIndex)
	if err != nil {
		return nil, err
	}

	// Create an empty leaf node as the root
	rootNode := NewLeafNode(pageID)
	if err := tree.writeNode(rootNode); err != nil {
		return nil, err
	}

	tree.root = pageID

	return tree, nil
}

// NewBPlusTreeWithRoot creates a BPlusTree loading from an existing root page.
func NewBPlusTreeWithRoot(pm *storage.PageManager, rootPageID storage.PageID, order int) (*BPlusTree, error) {
	if pm == nil {
		return nil, ErrInvalidPageManager
	}

	if order <= 0 {
		order = BPlusOrder
	}

	tree := &BPlusTree{
		root:        rootPageID,
		pageManager: pm,
		order:       order,
	}

	// Verify the root page exists and is valid
	_, err := tree.readNode(rootPageID)
	if err != nil {
		return nil, err
	}

	return tree, nil
}

// Root returns the root page ID of the tree.
func (t *BPlusTree) Root() storage.PageID {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.root
}

// Order returns the order of the tree.
func (t *BPlusTree) Order() int {
	return t.order
}

// IsEmpty returns true if the tree has no keys.
func (t *BPlusTree) IsEmpty() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.root == InvalidPageID {
		return true
	}

	node, err := t.readNode(t.root)
	if err != nil {
		return true
	}

	return len(node.Keys) == 0
}

// readNode reads a node from disk.
func (t *BPlusTree) readNode(pageID storage.PageID) (*BPlusNode, error) {
	if pageID == InvalidPageID {
		return nil, ErrNodeNotFound
	}

	page, err := t.pageManager.ReadPage(pageID)
	if err != nil {
		return nil, err
	}

	node, err := NewNodeFromPage(page)
	if err != nil {
		return nil, err
	}

	return node, nil
}

// writeNode writes a node to disk.
func (t *BPlusTree) writeNode(node *BPlusNode) error {
	if node == nil {
		return ErrInvalidNode
	}

	page, err := node.CreatePage()
	if err != nil {
		return err
	}

	return t.pageManager.WritePage(page)
}

// allocateNode allocates a new page and creates a node.
func (t *BPlusTree) allocateNode(isLeaf bool) (*BPlusNode, error) {
	pageID, err := t.pageManager.AllocatePage(storage.PageTypeAttrIndex)
	if err != nil {
		return nil, err
	}

	if isLeaf {
		return NewLeafNode(pageID), nil
	}
	return NewInternalNode(pageID), nil
}

// freeNode frees a node's page.
func (t *BPlusTree) freeNode(pageID storage.PageID) error {
	return t.pageManager.FreePage(pageID)
}

// findLeaf finds the leaf node that should contain the given key.
func (t *BPlusTree) findLeaf(key []byte) (*BPlusNode, error) {
	if t.root == InvalidPageID {
		return nil, ErrTreeNotInitialized
	}

	node, err := t.readNode(t.root)
	if err != nil {
		return nil, err
	}

	for !node.IsLeaf {
		childID := node.GetChildForKey(key)
		if childID == InvalidPageID {
			return nil, ErrNodeNotFound
		}

		node, err = t.readNode(childID)
		if err != nil {
			return nil, err
		}
	}

	return node, nil
}

// findLeafWithPath finds the leaf node and returns the path from root to leaf.
func (t *BPlusTree) findLeafWithPath(key []byte) ([]*BPlusNode, error) {
	if t.root == InvalidPageID {
		return nil, ErrTreeNotInitialized
	}

	var path []*BPlusNode

	node, err := t.readNode(t.root)
	if err != nil {
		return nil, err
	}
	path = append(path, node)

	for !node.IsLeaf {
		childID := node.GetChildForKey(key)
		if childID == InvalidPageID {
			return nil, ErrNodeNotFound
		}

		node, err = t.readNode(childID)
		if err != nil {
			return nil, err
		}
		path = append(path, node)
	}

	return path, nil
}

// findLeafWithPathForPageID finds the path from root to a specific leaf page.
func (t *BPlusTree) findLeafWithPathForPageID(targetPageID storage.PageID) ([]*BPlusNode, error) {
	if t.root == InvalidPageID {
		return nil, ErrTreeNotInitialized
	}

	// Read the target leaf to get its first key
	targetLeaf, err := t.readNode(targetPageID)
	if err != nil {
		return nil, err
	}

	if !targetLeaf.IsLeaf {
		return nil, ErrInvalidNode
	}

	// If the leaf is empty, we can't find a path to it using keys
	// In this case, just return the leaf as a single-element path
	if len(targetLeaf.Keys) == 0 {
		return []*BPlusNode{targetLeaf}, nil
	}

	// Use the first key to find the path
	return t.findLeafWithPath(targetLeaf.Keys[0])
}

// Search finds all entry references for the given key.
// Returns an empty slice if the key is not found.
func (t *BPlusTree) Search(key []byte) ([]EntryRef, error) {
	if len(key) == 0 {
		return nil, ErrEmptyKey
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	leaf, err := t.findLeaf(key)
	if err != nil {
		if err == ErrTreeNotInitialized {
			return nil, nil
		}
		return nil, err
	}

	// First, go back to find the first occurrence of the key
	// (in case duplicates span multiple leaves)
	for leaf.Prev != InvalidPageID {
		prevLeaf, err := t.readNode(leaf.Prev)
		if err != nil {
			break
		}
		// Check if the previous leaf has this key
		if len(prevLeaf.Keys) > 0 && compareKeys(prevLeaf.Keys[len(prevLeaf.Keys)-1], key) == 0 {
			leaf = prevLeaf
		} else {
			break
		}
	}

	// Find the first occurrence of the key in this leaf
	idx, found := leaf.FindKeyIndex(key)
	if !found {
		// Key might be in the next leaf if we went too far back
		if leaf.Next != InvalidPageID {
			nextLeaf, err := t.readNode(leaf.Next)
			if err == nil {
				idx, found = nextLeaf.FindKeyIndex(key)
				if found {
					leaf = nextLeaf
				}
			}
		}
		if !found {
			return nil, nil
		}
	}

	// Collect all matching entries (there might be duplicates)
	var refs []EntryRef
	
	// Collect from current position to end of leaf
	for i := idx; i < len(leaf.Keys); i++ {
		if compareKeys(leaf.Keys[i], key) == 0 {
			refs = append(refs, leaf.Values[i])
		} else {
			return refs, nil
		}
	}

	// Check next leaves for more duplicates
	for leaf.Next != InvalidPageID {
		nextLeaf, err := t.readNode(leaf.Next)
		if err != nil {
			break
		}

		for i := 0; i < len(nextLeaf.Keys); i++ {
			if compareKeys(nextLeaf.Keys[i], key) == 0 {
				refs = append(refs, nextLeaf.Values[i])
			} else {
				return refs, nil
			}
		}
		leaf = nextLeaf
	}

	return refs, nil
}

// SearchRange finds all entry references for keys in the range [startKey, endKey].
// If startKey is nil, starts from the beginning.
// If endKey is nil, goes to the end.
func (t *BPlusTree) SearchRange(startKey, endKey []byte) ([]EntryRef, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.root == InvalidPageID {
		return nil, nil
	}

	var refs []EntryRef

	// Find the starting leaf
	var leaf *BPlusNode
	var startIdx int
	var err error

	if startKey == nil {
		// Start from the leftmost leaf
		leaf, err = t.findLeftmostLeaf()
		if err != nil {
			return nil, err
		}
		startIdx = 0
	} else {
		leaf, err = t.findLeaf(startKey)
		if err != nil {
			return nil, err
		}
		startIdx, _ = leaf.FindKeyIndex(startKey)
	}

	// Iterate through leaves collecting matching entries
	for leaf != nil {
		for i := startIdx; i < len(leaf.Keys); i++ {
			// Check if we've passed the end key
			if endKey != nil && compareKeys(leaf.Keys[i], endKey) > 0 {
				return refs, nil
			}
			refs = append(refs, leaf.Values[i])
		}

		// Move to next leaf
		if leaf.Next == InvalidPageID {
			break
		}
		leaf, err = t.readNode(leaf.Next)
		if err != nil {
			return refs, err
		}
		startIdx = 0
	}

	return refs, nil
}

// findLeftmostLeaf finds the leftmost leaf node in the tree.
func (t *BPlusTree) findLeftmostLeaf() (*BPlusNode, error) {
	if t.root == InvalidPageID {
		return nil, ErrTreeNotInitialized
	}

	node, err := t.readNode(t.root)
	if err != nil {
		return nil, err
	}

	for !node.IsLeaf {
		if len(node.Children) == 0 {
			return nil, ErrInvalidNode
		}
		node, err = t.readNode(node.Children[0])
		if err != nil {
			return nil, err
		}
	}

	return node, nil
}

// TreeStats holds statistics about the B+ tree.
type TreeStats struct {
	Height       int
	InternalNodes int
	LeafNodes    int
	TotalKeys    int
	TotalEntries int
}

// Stats returns statistics about the tree.
func (t *BPlusTree) Stats() (TreeStats, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	stats := TreeStats{}
	if t.root == InvalidPageID {
		return stats, nil
	}

	// Calculate height
	node, err := t.readNode(t.root)
	if err != nil {
		return stats, err
	}

	height := 1
	for !node.IsLeaf {
		height++
		if len(node.Children) == 0 {
			break
		}
		node, err = t.readNode(node.Children[0])
		if err != nil {
			return stats, err
		}
	}
	stats.Height = height

	// Count nodes and keys by traversing leaves
	leaf, err := t.findLeftmostLeaf()
	if err != nil {
		return stats, err
	}

	for leaf != nil {
		stats.LeafNodes++
		stats.TotalKeys += len(leaf.Keys)
		stats.TotalEntries += len(leaf.Values)

		if leaf.Next == InvalidPageID {
			break
		}
		leaf, err = t.readNode(leaf.Next)
		if err != nil {
			return stats, err
		}
	}

	// Estimate internal nodes (this is approximate)
	if stats.Height > 1 {
		stats.InternalNodes = (stats.LeafNodes + t.order - 2) / (t.order - 1)
	}

	return stats, nil
}
