package radix

import (
	"github.com/KilimcininKorOglu/oba/internal/storage"
)

// Scope represents the LDAP search scope.
type Scope int

const (
	// ScopeBase returns only the base entry itself.
	ScopeBase Scope = iota
	// ScopeOneLevel returns only the immediate children of the base entry.
	ScopeOneLevel
	// ScopeSubtree returns the base entry and all its descendants.
	ScopeSubtree
)

// String returns the string representation of the scope.
func (s Scope) String() string {
	switch s {
	case ScopeBase:
		return "base"
	case ScopeOneLevel:
		return "onelevel"
	case ScopeSubtree:
		return "subtree"
	default:
		return "unknown"
	}
}

// RadixIterator provides iteration over entries in a RadixTree based on LDAP search scope.
type RadixIterator struct {
	tree   *RadixTree
	baseDN string
	scope  Scope

	// Internal state
	baseNode       *Node
	baseComponents []string
	started        bool
	done           bool

	// For subtree traversal
	stack []iteratorFrame

	// For onelevel traversal
	childKeys   []string
	childIndex  int
	baseVisited bool
}

// iteratorFrame holds state for depth-first subtree traversal.
type iteratorFrame struct {
	node       *Node
	components []string
	childKeys  []string
	childIndex int
	visited    bool
}

// Iterator creates a new RadixIterator for the given base DN and scope.
// Returns an error if the base DN is invalid.
func (t *RadixTree) Iterator(baseDN string, scope Scope) (*RadixIterator, error) {
	// Parse and validate the base DN
	components, err := ParseDN(baseDN)
	if err != nil {
		return nil, err
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	// Find the base node
	baseNode := t.findNode(components)

	return &RadixIterator{
		tree:           t,
		baseDN:         baseDN,
		scope:          scope,
		baseNode:       baseNode,
		baseComponents: components,
		started:        false,
		done:           false,
	}, nil
}

// Next returns the next entry in the iteration.
// Returns (dn, pageID, slotID, true) if an entry is found, or ("", 0, 0, false) if iteration is complete.
func (it *RadixIterator) Next() (dn string, pageID storage.PageID, slotID uint16, ok bool) {
	if it.done {
		return "", 0, 0, false
	}

	// If base node doesn't exist, iteration is done
	if it.baseNode == nil {
		it.done = true
		return "", 0, 0, false
	}

	switch it.scope {
	case ScopeBase:
		return it.nextBase()
	case ScopeOneLevel:
		return it.nextOneLevel()
	case ScopeSubtree:
		return it.nextSubtree()
	default:
		it.done = true
		return "", 0, 0, false
	}
}

// nextBase returns the base entry only.
func (it *RadixIterator) nextBase() (string, storage.PageID, uint16, bool) {
	if it.started {
		it.done = true
		return "", 0, 0, false
	}

	it.started = true
	it.done = true

	if it.baseNode.HasEntry {
		return it.baseDN, it.baseNode.PageID, it.baseNode.SlotID, true
	}

	return "", 0, 0, false
}

// nextOneLevel returns immediate children of the base entry.
func (it *RadixIterator) nextOneLevel() (string, storage.PageID, uint16, bool) {
	it.tree.mu.RLock()
	defer it.tree.mu.RUnlock()

	// Initialize child keys on first call
	if !it.started {
		it.started = true
		it.childKeys = it.baseNode.GetChildKeys()
		it.childIndex = 0
	}

	// Iterate through children
	for it.childIndex < len(it.childKeys) {
		key := it.childKeys[it.childIndex]
		it.childIndex++

		child := it.baseNode.GetChild(key)
		if child != nil && child.HasEntry {
			// Build the child DN
			childComponents := append(it.baseComponents, child.Key)
			childDN := JoinDNReverse(childComponents)
			return childDN, child.PageID, child.SlotID, true
		}
	}

	it.done = true
	return "", 0, 0, false
}

// nextSubtree returns all entries in the subtree (including base).
func (it *RadixIterator) nextSubtree() (string, storage.PageID, uint16, bool) {
	it.tree.mu.RLock()
	defer it.tree.mu.RUnlock()

	// Initialize stack on first call
	if !it.started {
		it.started = true
		it.stack = []iteratorFrame{{
			node:       it.baseNode,
			components: it.baseComponents,
			childKeys:  it.baseNode.GetChildKeys(),
			childIndex: 0,
			visited:    false,
		}}
	}

	for len(it.stack) > 0 {
		// Get current frame
		frameIdx := len(it.stack) - 1
		frame := &it.stack[frameIdx]

		// If not visited yet, check if this node has an entry
		if !frame.visited {
			frame.visited = true
			if frame.node.HasEntry {
				dn := JoinDNReverse(frame.components)
				return dn, frame.node.PageID, frame.node.SlotID, true
			}
		}

		// Try to descend to next child
		for frame.childIndex < len(frame.childKeys) {
			key := frame.childKeys[frame.childIndex]
			frame.childIndex++

			child := frame.node.GetChild(key)
			if child != nil {
				// Push child frame onto stack
				childComponents := make([]string, len(frame.components)+1)
				copy(childComponents, frame.components)
				childComponents[len(frame.components)] = child.Key

				it.stack = append(it.stack, iteratorFrame{
					node:       child,
					components: childComponents,
					childKeys:  child.GetChildKeys(),
					childIndex: 0,
					visited:    false,
				})
				break
			}
		}

		// If we didn't push a new frame, pop current frame
		if len(it.stack) > 0 && len(it.stack) == frameIdx+1 && frame.childIndex >= len(frame.childKeys) {
			it.stack = it.stack[:frameIdx]
		}
	}

	it.done = true
	return "", 0, 0, false
}

// Close releases any resources held by the iterator.
func (it *RadixIterator) Close() {
	it.done = true
	it.stack = nil
	it.childKeys = nil
}

// Reset resets the iterator to the beginning.
func (it *RadixIterator) Reset() {
	it.started = false
	it.done = false
	it.stack = nil
	it.childKeys = nil
	it.childIndex = 0
	it.baseVisited = false
}

// Scope returns the scope of this iterator.
func (it *RadixIterator) Scope() Scope {
	return it.scope
}

// BaseDN returns the base DN of this iterator.
func (it *RadixIterator) BaseDN() string {
	return it.baseDN
}

// IsDone returns true if the iterator has finished.
func (it *RadixIterator) IsDone() bool {
	return it.done
}

// Collect returns all entries as a slice.
// This is a convenience method that iterates through all entries.
func (it *RadixIterator) Collect() []IteratorEntry {
	var entries []IteratorEntry

	for {
		dn, pageID, slotID, ok := it.Next()
		if !ok {
			break
		}
		entries = append(entries, IteratorEntry{
			DN:     dn,
			PageID: pageID,
			SlotID: slotID,
		})
	}

	return entries
}

// IteratorEntry represents an entry returned by the iterator.
type IteratorEntry struct {
	DN     string
	PageID storage.PageID
	SlotID uint16
}
