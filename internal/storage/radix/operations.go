package radix

import (
	"github.com/oba-ldap/oba/internal/storage"
)

// Insert adds a new DN to the tree with the given page and slot location.
// If the DN already exists, it returns ErrEntryExists.
//
// Algorithm:
// 1. Parse DN into components (reverse order: root first)
// 2. Traverse tree, creating nodes as needed
// 3. Mark final node as having entry
// 4. Update subtree counts along path
// 5. Persist modified nodes to pages
func (t *RadixTree) Insert(dn string, pageID storage.PageID, slotID uint16) error {
	// Parse DN into components (reverse order for tree traversal)
	components, err := ParseDN(dn)
	if err != nil {
		return err
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	// Traverse to the node, creating intermediate nodes as needed
	node, _ := t.traverseToNode(components, true)
	if node == nil {
		return ErrTreeNotInitialized
	}

	// Check if entry already exists
	if node.HasEntry {
		return ErrEntryExists
	}

	// Set the entry
	node.SetEntry(pageID, slotID)

	// Propagate subtree count change up to root
	// Note: SetEntry already increments the node's own count,
	// but we need to propagate to ancestors
	if node.Parent != nil {
		node.Parent.PropagateSubtreeCountChange(1)
	}

	// Mark tree as dirty and persist
	t.markDirty()
	return t.persistRoot()
}

// Delete removes a DN from the tree.
// If the DN doesn't exist, it returns ErrEntryNotFound.
//
// Algorithm:
// 1. Find node for DN
// 2. Clear entry reference
// 3. If node has no children and no entry, remove it
// 4. Propagate removal up to parent if needed
// 5. Update subtree counts
func (t *RadixTree) Delete(dn string) error {
	// Parse DN into components
	components, err := ParseDN(dn)
	if err != nil {
		return err
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	// Find the node
	node := t.findNode(components)
	if node == nil || !node.HasEntry {
		return ErrEntryNotFound
	}

	// Clear the entry
	node.ClearEntry()

	// Propagate subtree count change up to root
	if node.Parent != nil {
		node.Parent.PropagateSubtreeCountChange(-1)
	}

	// Clean up empty nodes (nodes with no entry and no children)
	t.cleanupEmptyNodes(node)

	// Mark tree as dirty and persist
	t.markDirty()
	return t.persistRoot()
}

// cleanupEmptyNodes removes empty nodes (no entry, no children) from the tree.
// It propagates up the tree, removing nodes that become empty.
func (t *RadixTree) cleanupEmptyNodes(node *Node) {
	current := node

	for current != nil && current != t.root {
		// If node has entry or children, stop cleanup
		if current.HasEntry || len(current.Children) > 0 {
			break
		}

		parent := current.Parent
		if parent != nil {
			parent.RemoveChild(current.Key)
		}

		current = parent
	}
}

// Lookup finds an entry by DN and returns its page and slot location.
// Returns (pageID, slotID, true) if found, or (0, 0, false) if not found.
func (t *RadixTree) Lookup(dn string) (storage.PageID, uint16, bool) {
	// Parse DN into components
	components, err := ParseDN(dn)
	if err != nil {
		return 0, 0, false
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	// Find the node
	node := t.findNode(components)
	if node == nil || !node.HasEntry {
		return 0, 0, false
	}

	return node.PageID, node.SlotID, true
}

// Exists checks if a DN exists in the tree.
func (t *RadixTree) Exists(dn string) bool {
	_, _, found := t.Lookup(dn)
	return found
}

// Update updates the page and slot location for an existing DN.
// Returns ErrEntryNotFound if the DN doesn't exist.
func (t *RadixTree) Update(dn string, pageID storage.PageID, slotID uint16) error {
	// Parse DN into components
	components, err := ParseDN(dn)
	if err != nil {
		return err
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	// Find the node
	node := t.findNode(components)
	if node == nil || !node.HasEntry {
		return ErrEntryNotFound
	}

	// Update the entry location
	node.PageID = pageID
	node.SlotID = slotID

	// Mark tree as dirty and persist
	t.markDirty()
	return t.persistRoot()
}

// GetSubtreeCount returns the number of entries in the subtree rooted at the given DN.
// Returns 0 if the DN doesn't exist.
func (t *RadixTree) GetSubtreeCount(dn string) (uint32, error) {
	// Parse DN into components
	components, err := ParseDN(dn)
	if err != nil {
		return 0, err
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	// Find the node
	node := t.findNode(components)
	if node == nil {
		return 0, nil
	}

	return node.SubtreeCount, nil
}

// GetChildren returns the DNs of all direct children of the given DN.
func (t *RadixTree) GetChildren(dn string) ([]string, error) {
	// Parse DN into components
	components, err := ParseDN(dn)
	if err != nil {
		return nil, err
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	// Find the node
	node := t.findNode(components)
	if node == nil {
		return nil, nil
	}

	// Collect child DNs - use ChildrenByKey if available
	var children []string
	if node.ChildrenByKey != nil && len(node.ChildrenByKey) > 0 {
		for _, child := range node.ChildrenByKey {
			if child.HasEntry {
				// Build the child DN
				childComponents := append(components, child.Key)
				childDN := JoinDNReverse(childComponents)
				children = append(children, childDN)
			}
		}
	} else {
		for _, child := range node.Children {
			if child.HasEntry {
				// Build the child DN
				childComponents := append(components, child.Key)
				childDN := JoinDNReverse(childComponents)
				children = append(children, childDN)
			}
		}
	}

	return children, nil
}

// GetAllChildren returns the DNs of all children (direct and indirect) of the given DN.
func (t *RadixTree) GetAllChildren(dn string) ([]string, error) {
	// Parse DN into components
	components, err := ParseDN(dn)
	if err != nil {
		return nil, err
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	// Find the node
	node := t.findNode(components)
	if node == nil {
		return nil, nil
	}

	// Collect all descendant DNs
	var descendants []string
	t.collectDescendants(node, components, &descendants)

	return descendants, nil
}

// collectDescendants recursively collects all descendant DNs.
func (t *RadixTree) collectDescendants(node *Node, pathComponents []string, descendants *[]string) {
	// Use ChildrenByKey if available
	if node.ChildrenByKey != nil && len(node.ChildrenByKey) > 0 {
		for _, child := range node.ChildrenByKey {
			childPath := append(pathComponents, child.Key)
			if child.HasEntry {
				childDN := JoinDNReverse(childPath)
				*descendants = append(*descendants, childDN)
			}
			t.collectDescendants(child, childPath, descendants)
		}
	} else {
		for _, child := range node.Children {
			childPath := append(pathComponents, child.Key)
			if child.HasEntry {
				childDN := JoinDNReverse(childPath)
				*descendants = append(*descendants, childDN)
			}
			t.collectDescendants(child, childPath, descendants)
		}
	}
}

// EntryIterator is a function that receives entry information during iteration.
// Return false to stop iteration.
type EntryIterator func(dn string, pageID storage.PageID, slotID uint16) bool

// IterateSubtree iterates over all entries in the subtree rooted at the given DN.
// The iterator function is called for each entry.
// If baseDN is empty, iterates over all entries in the tree.
func (t *RadixTree) IterateSubtree(baseDN string, iterator EntryIterator) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var startNode *Node
	var pathComponents []string

	if baseDN == "" {
		startNode = t.root
		pathComponents = nil
	} else {
		components, err := ParseDN(baseDN)
		if err != nil {
			return err
		}
		startNode = t.findNode(components)
		if startNode == nil {
			return nil
		}
		pathComponents = components
	}

	t.iterateNode(startNode, pathComponents, iterator)
	return nil
}

// iterateNode recursively iterates over nodes.
func (t *RadixTree) iterateNode(node *Node, pathComponents []string, iterator EntryIterator) bool {
	if node == nil {
		return true
	}

	// Visit this node if it has an entry
	if node.HasEntry {
		dn := JoinDNReverse(pathComponents)
		if !iterator(dn, node.PageID, node.SlotID) {
			return false
		}
	}

	// Visit children - use ChildrenByKey if available
	if node.ChildrenByKey != nil && len(node.ChildrenByKey) > 0 {
		for _, child := range node.ChildrenByKey {
			childPath := append(pathComponents, child.Key)
			if !t.iterateNode(child, childPath, iterator) {
				return false
			}
		}
	} else {
		for _, child := range node.Children {
			childPath := append(pathComponents, child.Key)
			if !t.iterateNode(child, childPath, iterator) {
				return false
			}
		}
	}

	return true
}

// IterateOneLevelChildren iterates over direct children of the given DN.
func (t *RadixTree) IterateOneLevelChildren(baseDN string, iterator EntryIterator) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var startNode *Node
	var pathComponents []string

	if baseDN == "" {
		startNode = t.root
		pathComponents = nil
	} else {
		components, err := ParseDN(baseDN)
		if err != nil {
			return err
		}
		startNode = t.findNode(components)
		if startNode == nil {
			return nil
		}
		pathComponents = components
	}

	// Iterate only direct children - use ChildrenByKey if available
	if startNode.ChildrenByKey != nil && len(startNode.ChildrenByKey) > 0 {
		for _, child := range startNode.ChildrenByKey {
			if child.HasEntry {
				childPath := append(pathComponents, child.Key)
				childDN := JoinDNReverse(childPath)
				if !iterator(childDN, child.PageID, child.SlotID) {
					return nil
				}
			}
		}
	} else {
		for _, child := range startNode.Children {
			if child.HasEntry {
				childPath := append(pathComponents, child.Key)
				childDN := JoinDNReverse(childPath)
				if !iterator(childDN, child.PageID, child.SlotID) {
					return nil
				}
			}
		}
	}

	return nil
}

// HasChildren returns true if the given DN has any children.
func (t *RadixTree) HasChildren(dn string) (bool, error) {
	components, err := ParseDN(dn)
	if err != nil {
		return false, err
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	node := t.findNode(components)
	if node == nil {
		return false, nil
	}

	// Check ChildrenByKey first
	if node.ChildrenByKey != nil && len(node.ChildrenByKey) > 0 {
		return true, nil
	}
	return len(node.Children) > 0, nil
}

// GetParent returns the DN of the parent entry.
// Returns empty string if the entry has no parent or doesn't exist.
func (t *RadixTree) GetParent(dn string) (string, error) {
	components, err := ParseDN(dn)
	if err != nil {
		return "", err
	}

	if len(components) <= 1 {
		return "", nil
	}

	// Parent components are all but the last
	parentComponents := components[:len(components)-1]
	return JoinDNReverse(parentComponents), nil
}
