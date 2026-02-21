// Package btree provides B+ Tree implementation for attribute indexing in ObaDB.
package btree

import (
	"github.com/KilimcininKorOglu/oba/internal/storage"
)

// Delete removes a key-value pair from the B+ tree.
// If the key has multiple values, only the matching entry reference is removed.
// If the key is not found, returns ErrKeyNotFound.
//
// Algorithm:
// 1. Find the leaf node containing the key
// 2. Remove the key-value pair
// 3. If the leaf underflows (< 50% full):
//    a. Try to borrow from a sibling
//    b. If borrowing fails, merge with a sibling
// 4. Propagate changes up to the parent
func (t *BPlusTree) Delete(key []byte, ref EntryRef) error {
	if len(key) == 0 {
		return ErrEmptyKey
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	// Find the leaf that should contain the key
	path, err := t.findLeafWithPath(key)
	if err != nil {
		return err
	}

	leaf := path[len(path)-1]

	// Go back to find the first occurrence of the key (in case duplicates span leaves)
	for leaf.Prev != InvalidPageID {
		prevLeaf, err := t.readNode(leaf.Prev)
		if err != nil {
			break
		}
		if len(prevLeaf.Keys) > 0 && compareKeys(prevLeaf.Keys[len(prevLeaf.Keys)-1], key) == 0 {
			// Need to rebuild path for the previous leaf
			path, err = t.findLeafWithPathForPageID(prevLeaf.PageID)
			if err != nil {
				break
			}
			leaf = prevLeaf
		} else {
			break
		}
	}

	// Search for the exact entry across leaves
	for {
		idx, found := leaf.FindKeyIndex(key)
		if found {
			// Search for the exact entry in this leaf
			for i := idx; i < len(leaf.Keys); i++ {
				if compareKeys(leaf.Keys[i], key) != 0 {
					break
				}
				if leaf.Values[i].PageID == ref.PageID && leaf.Values[i].SlotID == ref.SlotID {
					// Found the entry, delete it
					leaf.RemoveKeyAt(i)

					// Check if this is the root
					if len(path) == 1 {
						return t.writeNode(leaf)
					}

					// Check if the leaf underflows
					if leaf.IsUnderflow() {
						return t.handleLeafUnderflow(path)
					}

					return t.writeNode(leaf)
				}
			}
		}

		// Check next leaf
		if leaf.Next == InvalidPageID {
			break
		}
		nextLeaf, err := t.readNode(leaf.Next)
		if err != nil {
			break
		}
		if len(nextLeaf.Keys) == 0 || compareKeys(nextLeaf.Keys[0], key) != 0 {
			break
		}
		// Rebuild path for the next leaf
		path, err = t.findLeafWithPathForPageID(nextLeaf.PageID)
		if err != nil {
			break
		}
		leaf = nextLeaf
	}

	return ErrKeyNotFound
}

// DeleteKey removes all entries with the given key from the B+ tree.
// Returns ErrKeyNotFound if the key doesn't exist.
func (t *BPlusTree) DeleteKey(key []byte) error {
	if len(key) == 0 {
		return ErrEmptyKey
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	// Find the leaf that should contain the key
	path, err := t.findLeafWithPath(key)
	if err != nil {
		return err
	}

	leaf := path[len(path)-1]

	// Go back to find the first occurrence of the key (in case duplicates span leaves)
	for leaf.Prev != InvalidPageID {
		prevLeaf, err := t.readNode(leaf.Prev)
		if err != nil {
			break
		}
		if len(prevLeaf.Keys) > 0 && compareKeys(prevLeaf.Keys[len(prevLeaf.Keys)-1], key) == 0 {
			path, err = t.findLeafWithPathForPageID(prevLeaf.PageID)
			if err != nil {
				break
			}
			leaf = prevLeaf
		} else {
			break
		}
	}

	// Find the key in the leaf
	idx, found := leaf.FindKeyIndex(key)
	if !found {
		return ErrKeyNotFound
	}

	// Remove all entries with this key from this leaf and subsequent leaves
	deleteCount := 0
	for {
		// Remove all matching keys in current leaf
		for idx < len(leaf.Keys) && compareKeys(leaf.Keys[idx], key) == 0 {
			leaf.RemoveKeyAt(idx)
			deleteCount++
		}

		// Write the modified leaf
		if err := t.writeNode(leaf); err != nil {
			return err
		}

		// Check if we need to handle underflow
		if len(path) > 1 && leaf.IsUnderflow() {
			if err := t.handleLeafUnderflow(path); err != nil {
				return err
			}
		}

		// Check next leaf for more duplicates
		if leaf.Next == InvalidPageID {
			break
		}
		nextLeaf, err := t.readNode(leaf.Next)
		if err != nil {
			break
		}
		if len(nextLeaf.Keys) == 0 || compareKeys(nextLeaf.Keys[0], key) != 0 {
			break
		}

		// Move to next leaf
		path, err = t.findLeafWithPathForPageID(nextLeaf.PageID)
		if err != nil {
			break
		}
		leaf = nextLeaf
		idx = 0
	}

	if deleteCount == 0 {
		return ErrKeyNotFound
	}

	return nil
}

// handleLeafUnderflow handles underflow in a leaf node.
func (t *BPlusTree) handleLeafUnderflow(path []*BPlusNode) error {
	leaf := path[len(path)-1]
	parent := path[len(path)-2]

	// Find the index of the leaf in the parent's children
	leafIdx := t.findChildIndex(parent, leaf.PageID)
	if leafIdx == -1 {
		return ErrInvalidNode
	}

	// Try to borrow from left sibling
	if leafIdx > 0 {
		leftSibling, err := t.readNode(parent.Children[leafIdx-1])
		if err == nil && leftSibling.CanBorrow() {
			return t.borrowFromLeftLeaf(path, leftSibling, leafIdx)
		}
	}

	// Try to borrow from right sibling
	if leafIdx < len(parent.Children)-1 {
		rightSibling, err := t.readNode(parent.Children[leafIdx+1])
		if err == nil && rightSibling.CanBorrow() {
			return t.borrowFromRightLeaf(path, rightSibling, leafIdx)
		}
	}

	// Cannot borrow, must merge
	if leafIdx > 0 {
		// Merge with left sibling
		leftSibling, err := t.readNode(parent.Children[leafIdx-1])
		if err != nil {
			return err
		}
		return t.mergeLeaves(path, leftSibling, leaf, leafIdx-1)
	}

	// Merge with right sibling
	rightSibling, err := t.readNode(parent.Children[leafIdx+1])
	if err != nil {
		return err
	}
	return t.mergeLeaves(path, leaf, rightSibling, leafIdx)
}

// findChildIndex finds the index of a child in the parent's children array.
func (t *BPlusTree) findChildIndex(parent *BPlusNode, childID storage.PageID) int {
	for i, id := range parent.Children {
		if id == childID {
			return i
		}
	}
	return -1
}

// borrowFromLeftLeaf borrows a key from the left sibling leaf.
func (t *BPlusTree) borrowFromLeftLeaf(path []*BPlusNode, leftSibling *BPlusNode, leafIdx int) error {
	leaf := path[len(path)-1]
	parent := path[len(path)-2]

	// Move the last key-value from left sibling to the beginning of leaf
	lastIdx := len(leftSibling.Keys) - 1
	key := leftSibling.Keys[lastIdx]
	value := leftSibling.Values[lastIdx]

	// Remove from left sibling
	leftSibling.RemoveKeyAt(lastIdx)

	// Insert at the beginning of leaf
	leaf.InsertKeyAt(0, key, &value, InvalidPageID)

	// Update the parent's separator key
	parent.Keys[leafIdx-1] = make([]byte, len(leaf.Keys[0]))
	copy(parent.Keys[leafIdx-1], leaf.Keys[0])

	// Write all modified nodes
	if err := t.writeNode(leftSibling); err != nil {
		return err
	}
	if err := t.writeNode(leaf); err != nil {
		return err
	}
	return t.writeNode(parent)
}

// borrowFromRightLeaf borrows a key from the right sibling leaf.
func (t *BPlusTree) borrowFromRightLeaf(path []*BPlusNode, rightSibling *BPlusNode, leafIdx int) error {
	leaf := path[len(path)-1]
	parent := path[len(path)-2]

	// Move the first key-value from right sibling to the end of leaf
	key := rightSibling.Keys[0]
	value := rightSibling.Values[0]

	// Remove from right sibling
	rightSibling.RemoveKeyAt(0)

	// Insert at the end of leaf
	leaf.InsertKeyAt(len(leaf.Keys), key, &value, InvalidPageID)

	// Update the parent's separator key
	parent.Keys[leafIdx] = make([]byte, len(rightSibling.Keys[0]))
	copy(parent.Keys[leafIdx], rightSibling.Keys[0])

	// Write all modified nodes
	if err := t.writeNode(rightSibling); err != nil {
		return err
	}
	if err := t.writeNode(leaf); err != nil {
		return err
	}
	return t.writeNode(parent)
}

// mergeLeaves merges two leaf nodes into one.
func (t *BPlusTree) mergeLeaves(path []*BPlusNode, left, right *BPlusNode, keyIdx int) error {
	// Move all keys and values from right to left
	for i := 0; i < len(right.Keys); i++ {
		left.Keys = append(left.Keys, right.Keys[i])
		left.Values = append(left.Values, right.Values[i])
	}

	// Update leaf links
	left.Next = right.Next
	if right.Next != InvalidPageID {
		nextLeaf, err := t.readNode(right.Next)
		if err == nil {
			nextLeaf.Prev = left.PageID
			t.writeNode(nextLeaf)
		}
	}

	// Write the merged left node
	if err := t.writeNode(left); err != nil {
		return err
	}

	// Free the right node
	if err := t.freeNode(right.PageID); err != nil {
		return err
	}

	// Remove the key and child from parent
	return t.deleteFromParent(path[:len(path)-1], keyIdx)
}

// deleteFromParent removes a key and child from the parent node.
func (t *BPlusTree) deleteFromParent(path []*BPlusNode, keyIdx int) error {
	if len(path) == 0 {
		return nil
	}

	parent := path[len(path)-1]

	// Remove the key and the right child
	parent.Keys = append(parent.Keys[:keyIdx], parent.Keys[keyIdx+1:]...)
	parent.Children = append(parent.Children[:keyIdx+1], parent.Children[keyIdx+2:]...)

	// Check if this is the root
	if len(path) == 1 {
		// If root has no keys but has one child, make that child the new root
		if len(parent.Keys) == 0 && len(parent.Children) == 1 {
			t.root = parent.Children[0]
			return t.freeNode(parent.PageID)
		}
		return t.writeNode(parent)
	}

	// Check if the parent underflows
	if parent.IsUnderflow() {
		return t.handleInternalUnderflow(path)
	}

	return t.writeNode(parent)
}

// handleInternalUnderflow handles underflow in an internal node.
func (t *BPlusTree) handleInternalUnderflow(path []*BPlusNode) error {
	internal := path[len(path)-1]
	parent := path[len(path)-2]

	// Find the index of the internal node in the parent's children
	internalIdx := t.findChildIndex(parent, internal.PageID)
	if internalIdx == -1 {
		return ErrInvalidNode
	}

	// Try to borrow from left sibling
	if internalIdx > 0 {
		leftSibling, err := t.readNode(parent.Children[internalIdx-1])
		if err == nil && leftSibling.CanBorrow() {
			return t.borrowFromLeftInternal(path, leftSibling, internalIdx)
		}
	}

	// Try to borrow from right sibling
	if internalIdx < len(parent.Children)-1 {
		rightSibling, err := t.readNode(parent.Children[internalIdx+1])
		if err == nil && rightSibling.CanBorrow() {
			return t.borrowFromRightInternal(path, rightSibling, internalIdx)
		}
	}

	// Cannot borrow, must merge
	if internalIdx > 0 {
		// Merge with left sibling
		leftSibling, err := t.readNode(parent.Children[internalIdx-1])
		if err != nil {
			return err
		}
		return t.mergeInternals(path, leftSibling, internal, internalIdx-1)
	}

	// Merge with right sibling
	rightSibling, err := t.readNode(parent.Children[internalIdx+1])
	if err != nil {
		return err
	}
	return t.mergeInternals(path, internal, rightSibling, internalIdx)
}

// borrowFromLeftInternal borrows a key from the left sibling internal node.
func (t *BPlusTree) borrowFromLeftInternal(path []*BPlusNode, leftSibling *BPlusNode, internalIdx int) error {
	internal := path[len(path)-1]
	parent := path[len(path)-2]

	// Move the parent's separator key down to internal
	separatorKey := parent.Keys[internalIdx-1]

	// Move the last child from left sibling
	lastChildIdx := len(leftSibling.Children) - 1
	lastChild := leftSibling.Children[lastChildIdx]

	// Move the last key from left sibling up to parent
	lastKeyIdx := len(leftSibling.Keys) - 1
	parent.Keys[internalIdx-1] = leftSibling.Keys[lastKeyIdx]

	// Insert separator key and child at the beginning of internal
	internal.Keys = append([][]byte{separatorKey}, internal.Keys...)
	internal.Children = append([]storage.PageID{lastChild}, internal.Children...)

	// Remove from left sibling
	leftSibling.Keys = leftSibling.Keys[:lastKeyIdx]
	leftSibling.Children = leftSibling.Children[:lastChildIdx]

	// Write all modified nodes
	if err := t.writeNode(leftSibling); err != nil {
		return err
	}
	if err := t.writeNode(internal); err != nil {
		return err
	}
	return t.writeNode(parent)
}

// borrowFromRightInternal borrows a key from the right sibling internal node.
func (t *BPlusTree) borrowFromRightInternal(path []*BPlusNode, rightSibling *BPlusNode, internalIdx int) error {
	internal := path[len(path)-1]
	parent := path[len(path)-2]

	// Move the parent's separator key down to internal
	separatorKey := parent.Keys[internalIdx]

	// Move the first child from right sibling
	firstChild := rightSibling.Children[0]

	// Move the first key from right sibling up to parent
	parent.Keys[internalIdx] = rightSibling.Keys[0]

	// Append separator key and child to internal
	internal.Keys = append(internal.Keys, separatorKey)
	internal.Children = append(internal.Children, firstChild)

	// Remove from right sibling
	rightSibling.Keys = rightSibling.Keys[1:]
	rightSibling.Children = rightSibling.Children[1:]

	// Write all modified nodes
	if err := t.writeNode(rightSibling); err != nil {
		return err
	}
	if err := t.writeNode(internal); err != nil {
		return err
	}
	return t.writeNode(parent)
}

// mergeInternals merges two internal nodes into one.
func (t *BPlusTree) mergeInternals(path []*BPlusNode, left, right *BPlusNode, keyIdx int) error {
	parent := path[len(path)-2]

	// Pull down the separator key from parent
	separatorKey := parent.Keys[keyIdx]
	left.Keys = append(left.Keys, separatorKey)

	// Move all keys and children from right to left
	left.Keys = append(left.Keys, right.Keys...)
	left.Children = append(left.Children, right.Children...)

	// Write the merged left node
	if err := t.writeNode(left); err != nil {
		return err
	}

	// Free the right node
	if err := t.freeNode(right.PageID); err != nil {
		return err
	}

	// Remove the key and child from parent
	return t.deleteFromParent(path[:len(path)-1], keyIdx)
}
