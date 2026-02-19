// Package btree provides B+ Tree implementation for attribute indexing in ObaDB.
package btree

import (
	"github.com/oba-ldap/oba/internal/storage"
)

// Insert adds a key-value pair to the B+ tree.
// If the key already exists, the new entry reference is added (allowing duplicates).
//
// Algorithm:
// 1. Find the leaf node for the key
// 2. Insert the key-value pair in sorted order
// 3. If the leaf overflows, split it into two leaves
// 4. Propagate the split up to the parent
// 5. If the root splits, create a new root
func (t *BPlusTree) Insert(key []byte, ref EntryRef) error {
	if len(key) == 0 {
		return ErrEmptyKey
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	// Find the path from root to the leaf
	path, err := t.findLeafWithPath(key)
	if err != nil {
		return err
	}

	leaf := path[len(path)-1]

	// Find the insertion position
	idx, _ := leaf.FindKeyIndex(key)

	// Insert the key-value pair
	leaf.InsertKeyAt(idx, key, &ref, InvalidPageID)

	// Check if the leaf needs to be split (either by capacity or page size)
	if leaf.IsFull() || !leaf.FitsInPage() {
		return t.splitLeafAndPropagate(path)
	}

	// Write the modified leaf
	return t.writeNode(leaf)
}

// splitLeafAndPropagate splits a full leaf node and propagates the split up the tree.
func (t *BPlusTree) splitLeafAndPropagate(path []*BPlusNode) error {
	leaf := path[len(path)-1]

	// Split the leaf
	newLeaf, promotedKey, err := t.splitLeaf(leaf)
	if err != nil {
		return err
	}

	// Write both leaves
	if err := t.writeNode(leaf); err != nil {
		return err
	}
	if err := t.writeNode(newLeaf); err != nil {
		return err
	}

	// Propagate the split up the tree
	return t.insertIntoParent(path[:len(path)-1], leaf.PageID, promotedKey, newLeaf.PageID)
}

// splitLeaf splits a full leaf node into two nodes.
// Returns the new right node and the key to promote to the parent.
func (t *BPlusTree) splitLeaf(leaf *BPlusNode) (*BPlusNode, []byte, error) {
	// Allocate a new leaf node
	newLeaf, err := t.allocateNode(true)
	if err != nil {
		return nil, nil, err
	}

	// Calculate the split point (keep more keys in the left node)
	splitPoint := (len(leaf.Keys) + 1) / 2

	// Move keys and values to the new leaf
	newLeaf.Keys = make([][]byte, len(leaf.Keys)-splitPoint)
	newLeaf.Values = make([]EntryRef, len(leaf.Values)-splitPoint)

	for i := splitPoint; i < len(leaf.Keys); i++ {
		newLeaf.Keys[i-splitPoint] = leaf.Keys[i]
		newLeaf.Values[i-splitPoint] = leaf.Values[i]
	}

	// Truncate the original leaf
	leaf.Keys = leaf.Keys[:splitPoint]
	leaf.Values = leaf.Values[:splitPoint]

	// Update leaf links
	newLeaf.Next = leaf.Next
	newLeaf.Prev = leaf.PageID
	leaf.Next = newLeaf.PageID

	// Update the next leaf's prev pointer if it exists
	if newLeaf.Next != InvalidPageID {
		nextLeaf, err := t.readNode(newLeaf.Next)
		if err == nil {
			nextLeaf.Prev = newLeaf.PageID
			t.writeNode(nextLeaf)
		}
	}

	// The promoted key is the first key of the new leaf
	promotedKey := make([]byte, len(newLeaf.Keys[0]))
	copy(promotedKey, newLeaf.Keys[0])

	return newLeaf, promotedKey, nil
}

// insertIntoParent inserts a key and child pointer into the parent node.
// If the parent is full, it splits and propagates up.
func (t *BPlusTree) insertIntoParent(path []*BPlusNode, leftChild storage.PageID, key []byte, rightChild storage.PageID) error {
	// If path is empty, we need to create a new root
	if len(path) == 0 {
		return t.createNewRoot(leftChild, key, rightChild)
	}

	parent := path[len(path)-1]

	// Find the position to insert the key
	idx, _ := parent.FindKeyIndex(key)

	// Insert the key and right child
	parent.InsertKeyAt(idx, key, nil, rightChild)

	// Check if the parent needs to be split (either by capacity or page size)
	if parent.IsFull() || !parent.FitsInPage() {
		return t.splitInternalAndPropagate(path)
	}

	// Write the modified parent
	return t.writeNode(parent)
}

// createNewRoot creates a new root node with two children.
func (t *BPlusTree) createNewRoot(leftChild storage.PageID, key []byte, rightChild storage.PageID) error {
	newRoot, err := t.allocateNode(false)
	if err != nil {
		return err
	}

	// Set up the new root
	newRoot.Keys = [][]byte{key}
	newRoot.Children = []storage.PageID{leftChild, rightChild}

	// Write the new root
	if err := t.writeNode(newRoot); err != nil {
		return err
	}

	// Update the tree's root
	t.root = newRoot.PageID

	return nil
}

// splitInternalAndPropagate splits a full internal node and propagates the split up the tree.
func (t *BPlusTree) splitInternalAndPropagate(path []*BPlusNode) error {
	internal := path[len(path)-1]

	// Split the internal node
	newInternal, promotedKey, err := t.splitInternal(internal)
	if err != nil {
		return err
	}

	// Write both internal nodes
	if err := t.writeNode(internal); err != nil {
		return err
	}
	if err := t.writeNode(newInternal); err != nil {
		return err
	}

	// Propagate the split up the tree
	return t.insertIntoParent(path[:len(path)-1], internal.PageID, promotedKey, newInternal.PageID)
}

// splitInternal splits a full internal node into two nodes.
// Returns the new right node and the key to promote to the parent.
func (t *BPlusTree) splitInternal(internal *BPlusNode) (*BPlusNode, []byte, error) {
	// Allocate a new internal node
	newInternal, err := t.allocateNode(false)
	if err != nil {
		return nil, nil, err
	}

	// Calculate the split point
	// The middle key will be promoted to the parent
	splitPoint := len(internal.Keys) / 2

	// The promoted key is the middle key
	promotedKey := make([]byte, len(internal.Keys[splitPoint]))
	copy(promotedKey, internal.Keys[splitPoint])

	// Move keys after the split point to the new node
	newInternal.Keys = make([][]byte, len(internal.Keys)-splitPoint-1)
	for i := splitPoint + 1; i < len(internal.Keys); i++ {
		newInternal.Keys[i-splitPoint-1] = internal.Keys[i]
	}

	// Move children after the split point to the new node
	newInternal.Children = make([]storage.PageID, len(internal.Children)-splitPoint-1)
	for i := splitPoint + 1; i < len(internal.Children); i++ {
		newInternal.Children[i-splitPoint-1] = internal.Children[i]
	}

	// Truncate the original internal node
	internal.Keys = internal.Keys[:splitPoint]
	internal.Children = internal.Children[:splitPoint+1]

	return newInternal, promotedKey, nil
}

// InsertUnique inserts a key-value pair only if the key doesn't already exist.
// Returns ErrKeyExists if the key is already present.
func (t *BPlusTree) InsertUnique(key []byte, ref EntryRef) error {
	if len(key) == 0 {
		return ErrEmptyKey
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	// Find the path from root to the leaf
	path, err := t.findLeafWithPath(key)
	if err != nil {
		return err
	}

	leaf := path[len(path)-1]

	// Check if the key already exists
	idx, found := leaf.FindKeyIndex(key)
	if found {
		return ErrKeyExists
	}

	// Insert the key-value pair
	leaf.InsertKeyAt(idx, key, &ref, InvalidPageID)

	// Check if the leaf needs to be split (either by capacity or page size)
	if leaf.IsFull() || !leaf.FitsInPage() {
		return t.splitLeafAndPropagate(path)
	}

	// Write the modified leaf
	return t.writeNode(leaf)
}
