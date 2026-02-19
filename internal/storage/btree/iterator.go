// Package btree provides B+ Tree implementation for attribute indexing in ObaDB.
package btree

import (
	"bytes"
)

// BPlusIterator provides sequential access to B+ tree entries.
// It supports forward iteration through leaf nodes and can be bounded
// by start and end keys for range queries.
type BPlusIterator struct {
	tree       *BPlusTree
	current    *BPlusNode
	position   int
	endKey     []byte
	prefix     []byte
	closed     bool
	excludeEnd bool // If true, exclude endKey from results (for < comparison)
}

// newIterator creates a new iterator starting from the given leaf node and position.
func newIterator(tree *BPlusTree, startLeaf *BPlusNode, startPos int, endKey []byte) *BPlusIterator {
	return &BPlusIterator{
		tree:       tree,
		current:    startLeaf,
		position:   startPos,
		endKey:     endKey,
		prefix:     nil,
		closed:     false,
		excludeEnd: false,
	}
}

// newLessThanIteratorInternal creates an iterator that excludes the end key.
func newLessThanIteratorInternal(tree *BPlusTree, startLeaf *BPlusNode, startPos int, endKey []byte) *BPlusIterator {
	return &BPlusIterator{
		tree:       tree,
		current:    startLeaf,
		position:   startPos,
		endKey:     endKey,
		prefix:     nil,
		closed:     false,
		excludeEnd: true,
	}
}

// newPrefixIterator creates a new iterator for prefix matching.
func newPrefixIterator(tree *BPlusTree, startLeaf *BPlusNode, startPos int, prefix []byte) *BPlusIterator {
	return &BPlusIterator{
		tree:       tree,
		current:    startLeaf,
		position:   startPos,
		endKey:     nil,
		prefix:     prefix,
		closed:     false,
		excludeEnd: false,
	}
}

// Next returns the next key-value pair from the iterator.
// Returns (key, ref, true) if there is a next entry, or (nil, EntryRef{}, false) if exhausted.
func (it *BPlusIterator) Next() (key []byte, ref EntryRef, ok bool) {
	if it.closed || it.current == nil {
		return nil, EntryRef{}, false
	}

	// Check if we've exhausted the current leaf
	for it.position >= len(it.current.Keys) {
		// Move to the next leaf
		if it.current.Next == InvalidPageID {
			return nil, EntryRef{}, false
		}

		nextLeaf, err := it.tree.readNode(it.current.Next)
		if err != nil {
			return nil, EntryRef{}, false
		}

		it.current = nextLeaf
		it.position = 0
	}

	// Get the current key
	key = it.current.Keys[it.position]

	// Check end key boundary
	if it.endKey != nil {
		cmp := compareKeys(key, it.endKey)
		if it.excludeEnd {
			// For < comparison, stop when key >= endKey
			if cmp >= 0 {
				return nil, EntryRef{}, false
			}
		} else {
			// For <= comparison, stop when key > endKey
			if cmp > 0 {
				return nil, EntryRef{}, false
			}
		}
	}

	// Check prefix boundary
	if it.prefix != nil && !bytes.HasPrefix(key, it.prefix) {
		return nil, EntryRef{}, false
	}

	ref = it.current.Values[it.position]
	it.position++

	return key, ref, true
}

// Peek returns the next key-value pair without advancing the iterator.
// Returns (key, ref, true) if there is a next entry, or (nil, EntryRef{}, false) if exhausted.
func (it *BPlusIterator) Peek() (key []byte, ref EntryRef, ok bool) {
	if it.closed || it.current == nil {
		return nil, EntryRef{}, false
	}

	// Save current state
	savedCurrent := it.current
	savedPosition := it.position

	// Get next
	key, ref, ok = it.Next()

	// Restore state
	it.current = savedCurrent
	it.position = savedPosition

	return key, ref, ok
}

// Close releases resources associated with the iterator.
// After Close is called, Next will always return false.
func (it *BPlusIterator) Close() {
	it.closed = true
	it.current = nil
}

// Valid returns true if the iterator has more entries.
func (it *BPlusIterator) Valid() bool {
	if it.closed || it.current == nil {
		return false
	}

	// Check if we have entries in current leaf
	if it.position < len(it.current.Keys) {
		key := it.current.Keys[it.position]

		// Check end key boundary
		if it.endKey != nil {
			cmp := compareKeys(key, it.endKey)
			if it.excludeEnd {
				if cmp >= 0 {
					return false
				}
			} else {
				if cmp > 0 {
					return false
				}
			}
		}

		// Check prefix boundary
		if it.prefix != nil && !bytes.HasPrefix(key, it.prefix) {
			return false
		}

		return true
	}

	// Check if there's a next leaf
	if it.current.Next == InvalidPageID {
		return false
	}

	// Peek at the next leaf
	nextLeaf, err := it.tree.readNode(it.current.Next)
	if err != nil || len(nextLeaf.Keys) == 0 {
		return false
	}

	key := nextLeaf.Keys[0]

	// Check end key boundary
	if it.endKey != nil {
		cmp := compareKeys(key, it.endKey)
		if it.excludeEnd {
			if cmp >= 0 {
				return false
			}
		} else {
			if cmp > 0 {
				return false
			}
		}
	}

	// Check prefix boundary
	if it.prefix != nil && !bytes.HasPrefix(key, it.prefix) {
		return false
	}

	return true
}

// Collect returns all remaining entries as slices.
// This is a convenience method that exhausts the iterator.
func (it *BPlusIterator) Collect() (keys [][]byte, refs []EntryRef) {
	for {
		key, ref, ok := it.Next()
		if !ok {
			break
		}
		keys = append(keys, key)
		refs = append(refs, ref)
	}
	return keys, refs
}

// CollectRefs returns all remaining entry references.
// This is a convenience method that exhausts the iterator.
func (it *BPlusIterator) CollectRefs() []EntryRef {
	var refs []EntryRef
	for {
		_, ref, ok := it.Next()
		if !ok {
			break
		}
		refs = append(refs, ref)
	}
	return refs
}

// Count returns the number of remaining entries without collecting them.
// This exhausts the iterator.
func (it *BPlusIterator) Count() int {
	count := 0
	for {
		_, _, ok := it.Next()
		if !ok {
			break
		}
		count++
	}
	return count
}

// Skip advances the iterator by n entries.
// Returns the number of entries actually skipped.
func (it *BPlusIterator) Skip(n int) int {
	skipped := 0
	for i := 0; i < n; i++ {
		_, _, ok := it.Next()
		if !ok {
			break
		}
		skipped++
	}
	return skipped
}

// Take returns up to n entries from the iterator.
func (it *BPlusIterator) Take(n int) (keys [][]byte, refs []EntryRef) {
	for i := 0; i < n; i++ {
		key, ref, ok := it.Next()
		if !ok {
			break
		}
		keys = append(keys, key)
		refs = append(refs, ref)
	}
	return keys, refs
}

// emptyIterator returns an iterator that yields no results.
func emptyIterator() *BPlusIterator {
	return &BPlusIterator{
		tree:    nil,
		current: nil,
		closed:  true,
	}
}

// ReverseIterator provides reverse sequential access to B+ tree entries.
// It supports backward iteration through leaf nodes.
type ReverseIterator struct {
	tree     *BPlusTree
	current  *BPlusNode
	position int
	startKey []byte
	closed   bool
}

// newReverseIterator creates a new reverse iterator starting from the given leaf node and position.
func newReverseIterator(tree *BPlusTree, startLeaf *BPlusNode, startPos int, startKey []byte) *ReverseIterator {
	return &ReverseIterator{
		tree:     tree,
		current:  startLeaf,
		position: startPos,
		startKey: startKey,
		closed:   false,
	}
}

// Next returns the next key-value pair from the reverse iterator (moving backward).
// Returns (key, ref, true) if there is a next entry, or (nil, EntryRef{}, false) if exhausted.
func (it *ReverseIterator) Next() (key []byte, ref EntryRef, ok bool) {
	if it.closed || it.current == nil {
		return nil, EntryRef{}, false
	}

	// Check if we've exhausted the current leaf (going backward)
	for it.position < 0 {
		// Move to the previous leaf
		if it.current.Prev == InvalidPageID {
			return nil, EntryRef{}, false
		}

		prevLeaf, err := it.tree.readNode(it.current.Prev)
		if err != nil {
			return nil, EntryRef{}, false
		}

		it.current = prevLeaf
		it.position = len(it.current.Keys) - 1
	}

	if it.position < 0 || it.position >= len(it.current.Keys) {
		return nil, EntryRef{}, false
	}

	// Get the current key
	key = it.current.Keys[it.position]

	// Check start key boundary (for reverse iteration, we stop when key < startKey)
	if it.startKey != nil && compareKeys(key, it.startKey) < 0 {
		return nil, EntryRef{}, false
	}

	ref = it.current.Values[it.position]
	it.position--

	return key, ref, true
}

// Close releases resources associated with the reverse iterator.
func (it *ReverseIterator) Close() {
	it.closed = true
	it.current = nil
}

// Valid returns true if the reverse iterator has more entries.
func (it *ReverseIterator) Valid() bool {
	if it.closed || it.current == nil {
		return false
	}

	// Check if we have entries in current leaf
	if it.position >= 0 && it.position < len(it.current.Keys) {
		key := it.current.Keys[it.position]

		// Check start key boundary
		if it.startKey != nil && compareKeys(key, it.startKey) < 0 {
			return false
		}

		return true
	}

	// Check if there's a previous leaf
	if it.current.Prev == InvalidPageID {
		return false
	}

	// Peek at the previous leaf
	prevLeaf, err := it.tree.readNode(it.current.Prev)
	if err != nil || len(prevLeaf.Keys) == 0 {
		return false
	}

	key := prevLeaf.Keys[len(prevLeaf.Keys)-1]

	// Check start key boundary
	if it.startKey != nil && compareKeys(key, it.startKey) < 0 {
		return false
	}

	return true
}

// findRightmostLeaf finds the rightmost leaf node in the tree.
func (t *BPlusTree) findRightmostLeaf() (*BPlusNode, error) {
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
		node, err = t.readNode(node.Children[len(node.Children)-1])
		if err != nil {
			return nil, err
		}
	}

	return node, nil
}
