// Package btree provides B+ Tree implementation for attribute indexing in ObaDB.
package btree

import (
	"bytes"
)

// Range returns an iterator over all entries with keys in the range [startKey, endKey].
// If startKey is nil, iteration starts from the beginning.
// If endKey is nil, iteration continues to the end.
// The iterator must be closed after use.
func (t *BPlusTree) Range(startKey, endKey []byte) *BPlusIterator {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.root == InvalidPageID {
		return emptyIterator()
	}

	// Find the starting leaf
	var leaf *BPlusNode
	var startIdx int
	var err error

	if startKey == nil {
		// Start from the leftmost leaf
		leaf, err = t.findLeftmostLeaf()
		if err != nil {
			return emptyIterator()
		}
		startIdx = 0
	} else {
		leaf, err = t.findLeaf(startKey)
		if err != nil {
			return emptyIterator()
		}
		startIdx, _ = leaf.FindKeyIndex(startKey)
	}

	return newIterator(t, leaf, startIdx, endKey)
}

// Prefix returns an iterator over all entries with keys that have the given prefix.
// The iterator must be closed after use.
func (t *BPlusTree) Prefix(prefix []byte) *BPlusIterator {
	if len(prefix) == 0 {
		// Empty prefix matches everything
		return t.All()
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.root == InvalidPageID {
		return emptyIterator()
	}

	// Find the leaf that should contain the first key with this prefix
	leaf, err := t.findLeaf(prefix)
	if err != nil {
		return emptyIterator()
	}

	// Find the first key that could have this prefix
	startIdx, _ := leaf.FindKeyIndex(prefix)

	// Check if we need to look at the previous leaf
	// (in case the prefix is less than all keys in this leaf)
	if startIdx == 0 && leaf.Prev != InvalidPageID {
		prevLeaf, err := t.readNode(leaf.Prev)
		if err == nil && len(prevLeaf.Keys) > 0 {
			lastKey := prevLeaf.Keys[len(prevLeaf.Keys)-1]
			if bytes.HasPrefix(lastKey, prefix) {
				// There might be matching keys in the previous leaf
				// Go back to find the first one
				leaf = prevLeaf
				for leaf.Prev != InvalidPageID {
					prevLeaf, err := t.readNode(leaf.Prev)
					if err != nil {
						break
					}
					if len(prevLeaf.Keys) == 0 {
						break
					}
					lastKey := prevLeaf.Keys[len(prevLeaf.Keys)-1]
					if !bytes.HasPrefix(lastKey, prefix) {
						break
					}
					leaf = prevLeaf
				}
				// Find the first matching key in this leaf
				startIdx = 0
				for i, key := range leaf.Keys {
					if bytes.HasPrefix(key, prefix) {
						startIdx = i
						break
					}
				}
			}
		}
	}

	// Verify that the starting position has a matching prefix
	if startIdx < len(leaf.Keys) && !bytes.HasPrefix(leaf.Keys[startIdx], prefix) {
		// No keys with this prefix exist
		return emptyIterator()
	}

	return newPrefixIterator(t, leaf, startIdx, prefix)
}

// All returns an iterator over all entries in the tree.
// The iterator must be closed after use.
func (t *BPlusTree) All() *BPlusIterator {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.root == InvalidPageID {
		return emptyIterator()
	}

	leaf, err := t.findLeftmostLeaf()
	if err != nil {
		return emptyIterator()
	}

	return newIterator(t, leaf, 0, nil)
}

// GreaterThan returns an iterator over all entries with keys greater than the given key.
// The iterator must be closed after use.
func (t *BPlusTree) GreaterThan(key []byte) *BPlusIterator {
	if len(key) == 0 {
		return t.All()
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.root == InvalidPageID {
		return emptyIterator()
	}

	leaf, err := t.findLeaf(key)
	if err != nil {
		return emptyIterator()
	}

	// Find the first key greater than the given key
	idx, found := leaf.FindKeyIndex(key)
	if found {
		// Skip all entries with the exact key
		for idx < len(leaf.Keys) && compareKeys(leaf.Keys[idx], key) == 0 {
			idx++
		}
	}

	// If we've exhausted this leaf, move to the next
	if idx >= len(leaf.Keys) {
		if leaf.Next == InvalidPageID {
			return emptyIterator()
		}
		nextLeaf, err := t.readNode(leaf.Next)
		if err != nil {
			return emptyIterator()
		}
		leaf = nextLeaf
		idx = 0
	}

	return newIterator(t, leaf, idx, nil)
}

// GreaterThanOrEqual returns an iterator over all entries with keys >= the given key.
// The iterator must be closed after use.
func (t *BPlusTree) GreaterThanOrEqual(key []byte) *BPlusIterator {
	if len(key) == 0 {
		return t.All()
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.root == InvalidPageID {
		return emptyIterator()
	}

	leaf, err := t.findLeaf(key)
	if err != nil {
		return emptyIterator()
	}

	idx, _ := leaf.FindKeyIndex(key)

	return newIterator(t, leaf, idx, nil)
}

// LessThan returns an iterator over all entries with keys less than the given key.
// The iterator must be closed after use.
func (t *BPlusTree) LessThan(key []byte) *BPlusIterator {
	if len(key) == 0 {
		return emptyIterator()
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.root == InvalidPageID {
		return emptyIterator()
	}

	leaf, err := t.findLeftmostLeaf()
	if err != nil {
		return emptyIterator()
	}

	// Use the less-than iterator that excludes the end key
	return newLessThanIteratorInternal(t, leaf, 0, key)
}

// LessThanOrEqual returns an iterator over all entries with keys <= the given key.
// The iterator must be closed after use.
func (t *BPlusTree) LessThanOrEqual(key []byte) *BPlusIterator {
	if len(key) == 0 {
		return emptyIterator()
	}

	return t.Range(nil, key)
}

// SearchPrefix finds all entry references for keys that start with the given prefix.
// This is useful for LDAP substring filters like (cn=admin*).
func (t *BPlusTree) SearchPrefix(prefix []byte) ([]EntryRef, error) {
	if len(prefix) == 0 {
		// Empty prefix matches everything
		return t.SearchRange(nil, nil)
	}

	iter := t.Prefix(prefix)
	defer iter.Close()

	return iter.CollectRefs(), nil
}

// SearchGreaterThan finds all entry references for keys greater than the given key.
// This is useful for LDAP filters like (uidNumber>=1000).
func (t *BPlusTree) SearchGreaterThan(key []byte) ([]EntryRef, error) {
	if len(key) == 0 {
		return nil, ErrEmptyKey
	}

	iter := t.GreaterThan(key)
	defer iter.Close()

	return iter.CollectRefs(), nil
}

// SearchGreaterThanOrEqual finds all entry references for keys >= the given key.
func (t *BPlusTree) SearchGreaterThanOrEqual(key []byte) ([]EntryRef, error) {
	if len(key) == 0 {
		return nil, ErrEmptyKey
	}

	iter := t.GreaterThanOrEqual(key)
	defer iter.Close()

	return iter.CollectRefs(), nil
}

// SearchLessThan finds all entry references for keys less than the given key.
func (t *BPlusTree) SearchLessThan(key []byte) ([]EntryRef, error) {
	if len(key) == 0 {
		return nil, ErrEmptyKey
	}

	iter := t.LessThan(key)
	defer iter.Close()

	return iter.CollectRefs(), nil
}

// SearchLessThanOrEqual finds all entry references for keys <= the given key.
func (t *BPlusTree) SearchLessThanOrEqual(key []byte) ([]EntryRef, error) {
	if len(key) == 0 {
		return nil, ErrEmptyKey
	}

	iter := t.LessThanOrEqual(key)
	defer iter.Close()

	return iter.CollectRefs(), nil
}

// Contains checks if a key exists in the tree.
func (t *BPlusTree) Contains(key []byte) (bool, error) {
	if len(key) == 0 {
		return false, ErrEmptyKey
	}

	refs, err := t.Search(key)
	if err != nil {
		return false, err
	}

	return len(refs) > 0, nil
}

// First returns the first (smallest) key and its entry reference in the tree.
// Returns ErrTreeEmpty if the tree is empty.
func (t *BPlusTree) First() ([]byte, EntryRef, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.root == InvalidPageID {
		return nil, EntryRef{}, ErrTreeEmpty
	}

	leaf, err := t.findLeftmostLeaf()
	if err != nil {
		return nil, EntryRef{}, err
	}

	if len(leaf.Keys) == 0 {
		return nil, EntryRef{}, ErrTreeEmpty
	}

	return leaf.Keys[0], leaf.Values[0], nil
}

// Last returns the last (largest) key and its entry reference in the tree.
// Returns ErrTreeEmpty if the tree is empty.
func (t *BPlusTree) Last() ([]byte, EntryRef, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.root == InvalidPageID {
		return nil, EntryRef{}, ErrTreeEmpty
	}

	leaf, err := t.findRightmostLeaf()
	if err != nil {
		return nil, EntryRef{}, err
	}

	if len(leaf.Keys) == 0 {
		return nil, EntryRef{}, ErrTreeEmpty
	}

	lastIdx := len(leaf.Keys) - 1
	return leaf.Keys[lastIdx], leaf.Values[lastIdx], nil
}

// RangeReverse returns a reverse iterator over all entries with keys in the range [startKey, endKey].
// Iteration proceeds from endKey toward startKey.
// If endKey is nil, iteration starts from the end.
// If startKey is nil, iteration continues to the beginning.
func (t *BPlusTree) RangeReverse(startKey, endKey []byte) *ReverseIterator {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.root == InvalidPageID {
		return &ReverseIterator{closed: true}
	}

	// Find the ending leaf (where we start reverse iteration)
	var leaf *BPlusNode
	var startIdx int
	var err error

	if endKey == nil {
		// Start from the rightmost leaf
		leaf, err = t.findRightmostLeaf()
		if err != nil {
			return &ReverseIterator{closed: true}
		}
		startIdx = len(leaf.Keys) - 1
	} else {
		leaf, err = t.findLeaf(endKey)
		if err != nil {
			return &ReverseIterator{closed: true}
		}
		idx, found := leaf.FindKeyIndex(endKey)
		if found {
			// Find the last occurrence of the key
			for idx < len(leaf.Keys)-1 && compareKeys(leaf.Keys[idx+1], endKey) == 0 {
				idx++
			}
			startIdx = idx
		} else {
			// Key not found, start from the position before where it would be
			startIdx = idx - 1
		}
	}

	return newReverseIterator(t, leaf, startIdx, startKey)
}
