// Package btree provides B+ Tree implementation for attribute indexing in ObaDB.
package btree

import (
	"github.com/oba-ldap/oba/internal/storage"
)

// B+ Tree constants.
const (
	// BPlusOrder is the maximum number of children per internal node.
	// This determines the branching factor of the tree.
	BPlusOrder = 128

	// BPlusLeafCapacity is the maximum number of entries per leaf node.
	BPlusLeafCapacity = 256

	// MinInternalKeys is the minimum number of keys in an internal node (except root).
	MinInternalKeys = (BPlusOrder - 1) / 2

	// MinLeafKeys is the minimum number of keys in a leaf node (except root).
	MinLeafKeys = BPlusLeafCapacity / 2

	// InvalidPageID represents an invalid or null page reference.
	InvalidPageID storage.PageID = 0
)

// EntryRef represents a reference to an entry stored in a data page.
// It contains the page ID and slot ID where the entry is located.
type EntryRef struct {
	PageID storage.PageID // Page containing the entry
	SlotID uint16         // Slot index within the page
}

// BPlusNode represents a node in the B+ Tree.
// It can be either an internal node (containing keys and child pointers)
// or a leaf node (containing keys and entry references).
type BPlusNode struct {
	// IsLeaf indicates whether this is a leaf node.
	// Leaf nodes store actual entry references, while internal nodes
	// store only keys for navigation.
	IsLeaf bool

	// Keys contains the attribute values used for indexing.
	// For internal nodes: Keys[i] is the separator between Children[i] and Children[i+1].
	// For leaf nodes: Keys[i] corresponds to Values[i].
	Keys [][]byte

	// Children contains child page IDs (only used in internal nodes).
	// len(Children) = len(Keys) + 1 for internal nodes.
	Children []storage.PageID

	// Values contains entry references (only used in leaf nodes).
	// len(Values) = len(Keys) for leaf nodes.
	Values []EntryRef

	// Next is the page ID of the next leaf node (for range scans).
	// Only valid for leaf nodes. Set to InvalidPageID if this is the last leaf.
	Next storage.PageID

	// Prev is the page ID of the previous leaf node.
	// Only valid for leaf nodes. Set to InvalidPageID if this is the first leaf.
	Prev storage.PageID

	// PageID is the page ID where this node is stored.
	PageID storage.PageID
}

// NewInternalNode creates a new internal (non-leaf) B+ Tree node.
func NewInternalNode(pageID storage.PageID) *BPlusNode {
	return &BPlusNode{
		IsLeaf:   false,
		Keys:     make([][]byte, 0, BPlusOrder-1),
		Children: make([]storage.PageID, 0, BPlusOrder),
		Values:   nil,
		Next:     InvalidPageID,
		Prev:     InvalidPageID,
		PageID:   pageID,
	}
}

// NewLeafNode creates a new leaf B+ Tree node.
func NewLeafNode(pageID storage.PageID) *BPlusNode {
	return &BPlusNode{
		IsLeaf:   true,
		Keys:     make([][]byte, 0, BPlusLeafCapacity),
		Children: nil,
		Values:   make([]EntryRef, 0, BPlusLeafCapacity),
		Next:     InvalidPageID,
		Prev:     InvalidPageID,
		PageID:   pageID,
	}
}

// KeyCount returns the number of keys in the node.
func (n *BPlusNode) KeyCount() int {
	return len(n.Keys)
}

// IsFull returns true if the node cannot accept more keys.
func (n *BPlusNode) IsFull() bool {
	if n.IsLeaf {
		return len(n.Keys) >= BPlusLeafCapacity
	}
	return len(n.Keys) >= BPlusOrder-1
}

// IsUnderflow returns true if the node has fewer than minimum keys.
// Root nodes are exempt from underflow checks.
func (n *BPlusNode) IsUnderflow() bool {
	if n.IsLeaf {
		return len(n.Keys) < MinLeafKeys
	}
	return len(n.Keys) < MinInternalKeys
}

// CanBorrow returns true if the node has more than minimum keys
// and can lend a key to a sibling.
func (n *BPlusNode) CanBorrow() bool {
	if n.IsLeaf {
		return len(n.Keys) > MinLeafKeys
	}
	return len(n.Keys) > MinInternalKeys
}

// InsertKeyAt inserts a key at the specified index.
// For leaf nodes, also inserts the corresponding value.
// For internal nodes, also inserts the corresponding child pointer.
func (n *BPlusNode) InsertKeyAt(index int, key []byte, value *EntryRef, child storage.PageID) {
	// Make a copy of the key to avoid external modifications
	keyCopy := make([]byte, len(key))
	copy(keyCopy, key)

	// Expand keys slice
	n.Keys = append(n.Keys, nil)
	copy(n.Keys[index+1:], n.Keys[index:])
	n.Keys[index] = keyCopy

	if n.IsLeaf {
		if value != nil {
			n.Values = append(n.Values, EntryRef{})
			copy(n.Values[index+1:], n.Values[index:])
			n.Values[index] = *value
		}
	} else {
		// For internal nodes, child goes to the right of the key
		n.Children = append(n.Children, InvalidPageID)
		copy(n.Children[index+2:], n.Children[index+1:])
		n.Children[index+1] = child
	}
}

// RemoveKeyAt removes the key at the specified index.
// For leaf nodes, also removes the corresponding value.
// For internal nodes, also removes the corresponding child pointer.
func (n *BPlusNode) RemoveKeyAt(index int) ([]byte, *EntryRef, storage.PageID) {
	if index < 0 || index >= len(n.Keys) {
		return nil, nil, InvalidPageID
	}

	key := n.Keys[index]
	n.Keys = append(n.Keys[:index], n.Keys[index+1:]...)

	var value *EntryRef
	var child storage.PageID = InvalidPageID

	if n.IsLeaf {
		if index < len(n.Values) {
			v := n.Values[index]
			value = &v
			n.Values = append(n.Values[:index], n.Values[index+1:]...)
		}
	} else {
		// For internal nodes, remove the child to the right of the key
		if index+1 < len(n.Children) {
			child = n.Children[index+1]
			n.Children = append(n.Children[:index+1], n.Children[index+2:]...)
		}
	}

	return key, value, child
}

// FindKeyIndex returns the index where the key should be inserted
// or the index of the key if it exists.
// Returns (index, found) where found is true if the exact key exists.
func (n *BPlusNode) FindKeyIndex(key []byte) (int, bool) {
	low, high := 0, len(n.Keys)

	for low < high {
		mid := (low + high) / 2
		cmp := compareKeys(n.Keys[mid], key)
		if cmp < 0 {
			low = mid + 1
		} else if cmp > 0 {
			high = mid
		} else {
			return mid, true
		}
	}

	return low, false
}

// GetChildForKey returns the child page ID that should contain the given key.
// Only valid for internal nodes.
func (n *BPlusNode) GetChildForKey(key []byte) storage.PageID {
	if n.IsLeaf || len(n.Children) == 0 {
		return InvalidPageID
	}

	index, _ := n.FindKeyIndex(key)
	if index < len(n.Children) {
		return n.Children[index]
	}
	return n.Children[len(n.Children)-1]
}

// SetLink sets the next and previous leaf pointers.
// Only valid for leaf nodes.
func (n *BPlusNode) SetLink(prev, next storage.PageID) {
	n.Prev = prev
	n.Next = next
}

// GetFirstKey returns the first key in the node, or nil if empty.
func (n *BPlusNode) GetFirstKey() []byte {
	if len(n.Keys) == 0 {
		return nil
	}
	return n.Keys[0]
}

// GetLastKey returns the last key in the node, or nil if empty.
func (n *BPlusNode) GetLastKey() []byte {
	if len(n.Keys) == 0 {
		return nil
	}
	return n.Keys[len(n.Keys)-1]
}

// compareKeys compares two byte slice keys lexicographically.
// Returns -1 if a < b, 0 if a == b, 1 if a > b.
func compareKeys(a, b []byte) int {
	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}

	for i := 0; i < minLen; i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}

	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}
	return 0
}

// CompareKeys is the exported version of compareKeys for external use.
func CompareKeys(a, b []byte) int {
	return compareKeys(a, b)
}
