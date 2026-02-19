// Package radix provides a radix tree implementation optimized for DN (Distinguished Name)
// components in LDAP directory structures.
package radix

import (
	"github.com/oba-ldap/oba/internal/storage"
)

// Node represents a node in the radix tree for DN hierarchy traversal.
// Each node represents a DN component (e.g., "dc=example", "ou=users") and contains
// pointers to child nodes and optionally a reference to the entry's data page.
type Node struct {
	// Key is the DN component (e.g., "dc=example", "ou=users").
	// For path-compressed nodes, this may contain multiple components.
	Key string

	// Children maps the first byte of child keys to child nodes for fast lookup.
	// This provides O(1) access to the correct child branch.
	Children map[byte]*Node

	// HasEntry indicates whether this node represents a complete DN with an entry.
	HasEntry bool

	// PageID is the page where the entry data is stored (valid only if HasEntry is true).
	PageID storage.PageID

	// SlotID is the slot within the page where the entry is stored.
	SlotID uint16

	// Parent is a pointer to the parent node for onelevel scope traversal.
	Parent *Node

	// SubtreeCount is the number of entries in this node's subtree (including itself).
	// Used for query optimization and statistics.
	SubtreeCount uint32
}

// NewNode creates a new radix tree node with the given key.
func NewNode(key string) *Node {
	return &Node{
		Key:          key,
		Children:     make(map[byte]*Node),
		HasEntry:     false,
		PageID:       0,
		SlotID:       0,
		Parent:       nil,
		SubtreeCount: 0,
	}
}

// NewRootNode creates a new root node for the radix tree.
// The root node has an empty key and no parent.
func NewRootNode() *Node {
	return &Node{
		Key:          "",
		Children:     make(map[byte]*Node),
		HasEntry:     false,
		PageID:       0,
		SlotID:       0,
		Parent:       nil,
		SubtreeCount: 0,
	}
}

// NewEntryNode creates a new node that represents an entry.
func NewEntryNode(key string, pageID storage.PageID, slotID uint16) *Node {
	return &Node{
		Key:          key,
		Children:     make(map[byte]*Node),
		HasEntry:     true,
		PageID:       pageID,
		SlotID:       slotID,
		Parent:       nil,
		SubtreeCount: 1,
	}
}

// SetEntry marks this node as having an entry and sets the location.
func (n *Node) SetEntry(pageID storage.PageID, slotID uint16) {
	if !n.HasEntry {
		n.SubtreeCount++
	}
	n.HasEntry = true
	n.PageID = pageID
	n.SlotID = slotID
}

// ClearEntry removes the entry from this node.
func (n *Node) ClearEntry() {
	if n.HasEntry {
		n.SubtreeCount--
	}
	n.HasEntry = false
	n.PageID = 0
	n.SlotID = 0
}

// AddChild adds a child node to this node.
// The child's parent pointer is automatically set.
func (n *Node) AddChild(child *Node) {
	if len(child.Key) == 0 {
		return
	}
	firstByte := child.Key[0]
	child.Parent = n
	n.Children[firstByte] = child
	n.updateSubtreeCount()
}

// RemoveChild removes a child node from this node.
func (n *Node) RemoveChild(key string) *Node {
	if len(key) == 0 {
		return nil
	}
	firstByte := key[0]
	child, exists := n.Children[firstByte]
	if !exists || child.Key != key {
		return nil
	}
	delete(n.Children, firstByte)
	child.Parent = nil
	n.updateSubtreeCount()
	return child
}

// GetChild returns the child node with the given key, or nil if not found.
func (n *Node) GetChild(key string) *Node {
	if len(key) == 0 {
		return nil
	}
	firstByte := key[0]
	child, exists := n.Children[firstByte]
	if !exists || child.Key != key {
		return nil
	}
	return child
}

// FindChildByPrefix finds a child whose key starts with the given prefix.
// Returns the child and the remaining suffix, or nil if no match.
func (n *Node) FindChildByPrefix(prefix string) (*Node, string) {
	if len(prefix) == 0 {
		return nil, ""
	}
	firstByte := prefix[0]
	child, exists := n.Children[firstByte]
	if !exists {
		return nil, ""
	}

	// Check if child key is a prefix of the search prefix
	childKey := child.Key
	if len(childKey) <= len(prefix) && prefix[:len(childKey)] == childKey {
		return child, prefix[len(childKey):]
	}

	// Check if search prefix is a prefix of child key (partial match)
	if len(prefix) < len(childKey) && childKey[:len(prefix)] == prefix {
		return child, ""
	}

	return nil, ""
}

// IsLeaf returns true if this node has no children.
func (n *Node) IsLeaf() bool {
	return len(n.Children) == 0
}

// IsRoot returns true if this node has no parent.
func (n *Node) IsRoot() bool {
	return n.Parent == nil
}

// ChildCount returns the number of children.
func (n *Node) ChildCount() int {
	return len(n.Children)
}

// updateSubtreeCount recalculates the subtree count based on children.
func (n *Node) updateSubtreeCount() {
	count := uint32(0)
	if n.HasEntry {
		count = 1
	}
	for _, child := range n.Children {
		count += child.SubtreeCount
	}
	n.SubtreeCount = count
}

// RecalculateSubtreeCount recursively recalculates subtree counts.
func (n *Node) RecalculateSubtreeCount() uint32 {
	count := uint32(0)
	if n.HasEntry {
		count = 1
	}
	for _, child := range n.Children {
		count += child.RecalculateSubtreeCount()
	}
	n.SubtreeCount = count
	return count
}

// PropagateSubtreeCountChange propagates a subtree count change up to the root.
func (n *Node) PropagateSubtreeCountChange(delta int32) {
	current := n
	for current != nil {
		if delta > 0 {
			current.SubtreeCount += uint32(delta)
		} else if delta < 0 && current.SubtreeCount >= uint32(-delta) {
			current.SubtreeCount -= uint32(-delta)
		}
		current = current.Parent
	}
}

// GetChildren returns all child nodes as a slice.
func (n *Node) GetChildren() []*Node {
	children := make([]*Node, 0, len(n.Children))
	for _, child := range n.Children {
		children = append(children, child)
	}
	return children
}

// GetChildKeys returns all child keys as a slice.
func (n *Node) GetChildKeys() []string {
	keys := make([]string, 0, len(n.Children))
	for _, child := range n.Children {
		keys = append(keys, child.Key)
	}
	return keys
}

// Depth returns the depth of this node in the tree (root is depth 0).
func (n *Node) Depth() int {
	depth := 0
	current := n.Parent
	for current != nil {
		depth++
		current = current.Parent
	}
	return depth
}

// Path returns the path from root to this node as a slice of keys.
func (n *Node) Path() []string {
	path := make([]string, 0)
	current := n
	for current != nil && current.Key != "" {
		path = append([]string{current.Key}, path...)
		current = current.Parent
	}
	return path
}

// CanCompress returns true if this node can be path-compressed with its only child.
// A node can be compressed if it has exactly one child and doesn't have an entry.
func (n *Node) CanCompress() bool {
	return len(n.Children) == 1 && !n.HasEntry && n.Key != ""
}

// Compress performs path compression by merging this node with its only child.
// Returns the merged node, or nil if compression is not possible.
func (n *Node) Compress() *Node {
	if !n.CanCompress() {
		return nil
	}

	// Get the only child
	var child *Node
	for _, c := range n.Children {
		child = c
		break
	}

	// Create a new merged node
	merged := &Node{
		Key:          n.Key + "/" + child.Key, // Combine keys with separator
		Children:     child.Children,
		HasEntry:     child.HasEntry,
		PageID:       child.PageID,
		SlotID:       child.SlotID,
		Parent:       n.Parent,
		SubtreeCount: child.SubtreeCount,
	}

	// Update parent pointers of grandchildren
	for _, grandchild := range merged.Children {
		grandchild.Parent = merged
	}

	return merged
}

// Split splits a compressed node at the given position.
// Returns the new parent and child nodes.
func (n *Node) Split(pos int) (*Node, *Node) {
	if pos <= 0 || pos >= len(n.Key) {
		return nil, nil
	}

	parent := &Node{
		Key:          n.Key[:pos],
		Children:     make(map[byte]*Node),
		HasEntry:     false,
		PageID:       0,
		SlotID:       0,
		Parent:       n.Parent,
		SubtreeCount: n.SubtreeCount,
	}

	child := &Node{
		Key:          n.Key[pos:],
		Children:     n.Children,
		HasEntry:     n.HasEntry,
		PageID:       n.PageID,
		SlotID:       n.SlotID,
		Parent:       parent,
		SubtreeCount: n.SubtreeCount,
	}

	// Update parent pointers of grandchildren
	for _, grandchild := range child.Children {
		grandchild.Parent = child
	}

	// Add child to parent
	if len(child.Key) > 0 {
		parent.Children[child.Key[0]] = child
	}

	return parent, child
}

// Clone creates a shallow copy of the node (children are not cloned).
func (n *Node) Clone() *Node {
	clone := &Node{
		Key:          n.Key,
		Children:     make(map[byte]*Node, len(n.Children)),
		HasEntry:     n.HasEntry,
		PageID:       n.PageID,
		SlotID:       n.SlotID,
		Parent:       nil, // Parent is not cloned
		SubtreeCount: n.SubtreeCount,
	}
	for k, v := range n.Children {
		clone.Children[k] = v
	}
	return clone
}

// DeepClone creates a deep copy of the node and all its descendants.
func (n *Node) DeepClone() *Node {
	clone := &Node{
		Key:          n.Key,
		Children:     make(map[byte]*Node, len(n.Children)),
		HasEntry:     n.HasEntry,
		PageID:       n.PageID,
		SlotID:       n.SlotID,
		Parent:       nil,
		SubtreeCount: n.SubtreeCount,
	}
	for k, child := range n.Children {
		childClone := child.DeepClone()
		childClone.Parent = clone
		clone.Children[k] = childClone
	}
	return clone
}
