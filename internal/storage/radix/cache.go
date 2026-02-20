package radix

import (
	"bytes"
	"encoding/binary"
	"errors"

	"github.com/oba-ldap/oba/internal/storage"
	"github.com/oba-ldap/oba/internal/storage/cache"
)

// Cache errors.
var (
	ErrCacheCorrupt = errors.New("cache data corrupt")
)

// CacheEntry represents a serialized node entry in the cache.
type CacheEntry struct {
	Key          string
	HasEntry     bool
	PageID       storage.PageID
	SlotID       uint16
	SubtreeCount uint32
	ParentIdx    int32 // -1 for root
	ChildCount   int
}

// SaveCache saves the radix tree to a cache file.
func (t *RadixTree) SaveCache(path string, txID uint64) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.root == nil {
		return nil
	}

	// Collect all nodes in BFS order
	nodes := t.collectAllNodes()
	if len(nodes) == 0 {
		return nil
	}

	// Serialize nodes
	data, err := t.serializeNodes(nodes)
	if err != nil {
		return err
	}

	// Write cache file
	return cache.WriteFile(path, cache.TypeRadix, data, uint64(len(nodes)), txID)
}

// LoadCache loads the radix tree from a cache file.
func (t *RadixTree) LoadCache(path string, expectedTxID uint64) error {
	data, header, err := cache.ReadFile(path, cache.TypeRadix, expectedTxID)
	if err != nil {
		return err
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	// Deserialize nodes
	root, err := t.deserializeNodes(data, int(header.EntryCount))
	if err != nil {
		return err
	}

	t.root = root
	t.clearDirty()
	return nil
}

// collectAllNodes collects all nodes in BFS order.
func (t *RadixTree) collectAllNodes() []*Node {
	if t.root == nil {
		return nil
	}

	nodes := make([]*Node, 0)
	queue := []*Node{t.root}

	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		nodes = append(nodes, node)

		// Add children to queue
		children := node.GetChildren()
		queue = append(queue, children...)
	}

	return nodes
}

// serializeNodes serializes nodes to bytes.
// Format per node:
//   - KeyLen (uint16)
//   - Key ([]byte)
//   - Flags (uint8) - bit 0: HasEntry
//   - PageID (uint64)
//   - SlotID (uint16)
//   - SubtreeCount (uint32)
//   - ParentIdx (int32) - index in array, -1 for root
//   - ChildCount (uint16)
//   - ChildIndices ([]int32) - indices of children in array
func (t *RadixTree) serializeNodes(nodes []*Node) ([]byte, error) {
	// Build node index map
	nodeIndex := make(map[*Node]int32)
	for i, node := range nodes {
		nodeIndex[node] = int32(i)
	}

	var buf bytes.Buffer

	// Write node count
	if err := binary.Write(&buf, binary.LittleEndian, uint32(len(nodes))); err != nil {
		return nil, err
	}

	for _, node := range nodes {
		// Key length and key
		keyBytes := []byte(node.Key)
		if err := binary.Write(&buf, binary.LittleEndian, uint16(len(keyBytes))); err != nil {
			return nil, err
		}
		if _, err := buf.Write(keyBytes); err != nil {
			return nil, err
		}

		// Flags
		flags := uint8(0)
		if node.HasEntry {
			flags |= 0x01
		}
		if err := buf.WriteByte(flags); err != nil {
			return nil, err
		}

		// PageID
		if err := binary.Write(&buf, binary.LittleEndian, uint64(node.PageID)); err != nil {
			return nil, err
		}

		// SlotID
		if err := binary.Write(&buf, binary.LittleEndian, node.SlotID); err != nil {
			return nil, err
		}

		// SubtreeCount
		if err := binary.Write(&buf, binary.LittleEndian, node.SubtreeCount); err != nil {
			return nil, err
		}

		// ParentIdx
		parentIdx := int32(-1)
		if node.Parent != nil {
			if idx, ok := nodeIndex[node.Parent]; ok {
				parentIdx = idx
			}
		}
		if err := binary.Write(&buf, binary.LittleEndian, parentIdx); err != nil {
			return nil, err
		}

		// Children
		children := node.GetChildren()
		if err := binary.Write(&buf, binary.LittleEndian, uint16(len(children))); err != nil {
			return nil, err
		}

		for _, child := range children {
			childIdx := int32(-1)
			if idx, ok := nodeIndex[child]; ok {
				childIdx = idx
			}
			if err := binary.Write(&buf, binary.LittleEndian, childIdx); err != nil {
				return nil, err
			}
		}
	}

	return buf.Bytes(), nil
}

// deserializeNodes deserializes nodes from bytes.
func (t *RadixTree) deserializeNodes(data []byte, expectedCount int) (*Node, error) {
	if len(data) < 4 {
		return nil, ErrCacheCorrupt
	}

	buf := bytes.NewReader(data)

	// Read node count
	var nodeCount uint32
	if err := binary.Read(buf, binary.LittleEndian, &nodeCount); err != nil {
		return nil, err
	}

	if int(nodeCount) != expectedCount {
		return nil, ErrCacheCorrupt
	}

	if nodeCount == 0 {
		return NewRootNode(), nil
	}

	// First pass: create all nodes
	nodes := make([]*Node, nodeCount)
	parentIndices := make([]int32, nodeCount)
	childIndices := make([][]int32, nodeCount)

	for i := uint32(0); i < nodeCount; i++ {
		// Read key
		var keyLen uint16
		if err := binary.Read(buf, binary.LittleEndian, &keyLen); err != nil {
			return nil, err
		}

		keyBytes := make([]byte, keyLen)
		if _, err := buf.Read(keyBytes); err != nil {
			return nil, err
		}

		// Read flags
		flags, err := buf.ReadByte()
		if err != nil {
			return nil, err
		}

		// Read PageID
		var pageID uint64
		if err := binary.Read(buf, binary.LittleEndian, &pageID); err != nil {
			return nil, err
		}

		// Read SlotID
		var slotID uint16
		if err := binary.Read(buf, binary.LittleEndian, &slotID); err != nil {
			return nil, err
		}

		// Read SubtreeCount
		var subtreeCount uint32
		if err := binary.Read(buf, binary.LittleEndian, &subtreeCount); err != nil {
			return nil, err
		}

		// Read ParentIdx
		var parentIdx int32
		if err := binary.Read(buf, binary.LittleEndian, &parentIdx); err != nil {
			return nil, err
		}

		// Read child count and indices
		var childCount uint16
		if err := binary.Read(buf, binary.LittleEndian, &childCount); err != nil {
			return nil, err
		}

		childIdxs := make([]int32, childCount)
		for j := uint16(0); j < childCount; j++ {
			if err := binary.Read(buf, binary.LittleEndian, &childIdxs[j]); err != nil {
				return nil, err
			}
		}

		// Create node
		nodes[i] = &Node{
			Key:           string(keyBytes),
			Children:      make(map[byte]*Node),
			ChildrenByKey: make(map[string]*Node),
			HasEntry:      flags&0x01 != 0,
			PageID:        storage.PageID(pageID),
			SlotID:        slotID,
			SubtreeCount:  subtreeCount,
		}

		parentIndices[i] = parentIdx
		childIndices[i] = childIdxs
	}

	// Second pass: restore relationships
	for i := uint32(0); i < nodeCount; i++ {
		node := nodes[i]

		// Set parent
		if parentIndices[i] >= 0 && int(parentIndices[i]) < len(nodes) {
			node.Parent = nodes[parentIndices[i]]
		}

		// Set children
		for _, childIdx := range childIndices[i] {
			if childIdx >= 0 && int(childIdx) < len(nodes) {
				child := nodes[childIdx]
				if len(child.Key) > 0 {
					node.Children[child.Key[0]] = child
				}
				node.ChildrenByKey[child.Key] = child
			}
		}
	}

	// Root is the first node
	return nodes[0], nil
}
