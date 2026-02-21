package radix

import (
	"encoding/binary"
	"errors"

	"github.com/KilimcininKorOglu/oba/internal/storage"
)

// Serialization constants.
const (
	// SerializedNodeHeaderSize is the fixed size of a serialized node header.
	// Layout:
	//   - Bytes 0-1:   KeyOffset (uint16) - absolute offset from varDataStart
	//   - Bytes 2-3:   KeyLength (uint16)
	//   - Byte 4:      ChildCount (uint8)
	//   - Byte 5:      Flags (HasEntry in bit 0)
	//   - Bytes 6-13:  PageID (uint64)
	//   - Bytes 14-15: SlotID (uint16)
	//   - Bytes 16-19: SubtreeCount (uint32)
	//   - Bytes 20-21: ParentIndex (uint16, 0xFFFF = no parent)
	SerializedNodeHeaderSize = 22

	// ChildEntrySize is the size of each child entry in serialized form.
	// Layout:
	//   - Byte 0:      FirstByte (uint8)
	//   - Bytes 1-2:   ChildIndex (uint16)
	ChildEntrySize = 3

	// MaxNodesPerPage is the maximum number of nodes that can fit in a page.
	// This is a conservative estimate based on minimum node size.
	MaxNodesPerPage = (storage.PageSize - storage.PageHeaderSize - 4) / SerializedNodeHeaderSize

	// NoParentIndex indicates a node has no parent in the serialized form.
	NoParentIndex = uint16(0xFFFF)
)

// Serialization errors.
var (
	ErrBufferTooSmall     = errors.New("buffer too small for serialization")
	ErrInvalidNodeData    = errors.New("invalid node data")
	ErrTooManyNodes       = errors.New("too many nodes for page")
	ErrTooManyChildren    = errors.New("too many children for node")
	ErrKeyTooLong         = errors.New("key too long")
	ErrInvalidNodeIndex   = errors.New("invalid node index")
	ErrInvalidParentIndex = errors.New("invalid parent index")
)

// SerializedNode represents a node in serialized form.
type SerializedNode struct {
	KeyOffset    uint16         // Offset to key in variable data area
	KeyLength    uint16         // Length of key
	ChildCount   uint8          // Number of children
	HasEntry     bool           // Whether this node has an entry
	PageID       storage.PageID // Entry page ID
	SlotID       uint16         // Entry slot ID
	SubtreeCount uint32         // Subtree entry count
	ParentIndex  uint16         // Index of parent node (NoParentIndex if root)
}

// NodePage represents a page containing serialized radix tree nodes.
type NodePage struct {
	Header    storage.PageHeader
	NodeCount uint16
	Nodes     []SerializedNode
	// Variable length data (keys, child entries) follows
}

// NodeSerializer handles serialization and deserialization of radix tree nodes.
type NodeSerializer struct {
	// nodeIndex maps nodes to their indices during serialization
	nodeIndex map[*Node]uint16
	// indexNode maps indices to nodes during deserialization
	indexNode map[uint16]*Node
}

// NewNodeSerializer creates a new NodeSerializer.
func NewNodeSerializer() *NodeSerializer {
	return &NodeSerializer{
		nodeIndex: make(map[*Node]uint16),
		indexNode: make(map[uint16]*Node),
	}
}

// Reset clears the serializer state.
func (s *NodeSerializer) Reset() {
	s.nodeIndex = make(map[*Node]uint16)
	s.indexNode = make(map[uint16]*Node)
}

// SerializeNode serializes a single node to bytes.
// Returns the serialized header and variable data (key + children).
func (s *NodeSerializer) SerializeNode(node *Node, parentIndex uint16) ([]byte, []byte, error) {
	if node == nil {
		return nil, nil, ErrInvalidNodeData
	}

	if len(node.Key) > 65535 {
		return nil, nil, ErrKeyTooLong
	}

	// Get child count from ChildrenByKey if available
	childCount := len(node.Children)
	if node.ChildrenByKey != nil && len(node.ChildrenByKey) > 0 {
		childCount = len(node.ChildrenByKey)
	}

	if childCount > 255 {
		return nil, nil, ErrTooManyChildren
	}

	// Serialize header
	header := make([]byte, SerializedNodeHeaderSize)
	// KeyOffset will be set by the caller
	binary.LittleEndian.PutUint16(header[0:2], 0)                     // KeyOffset (placeholder)
	binary.LittleEndian.PutUint16(header[2:4], uint16(len(node.Key))) // KeyLength
	header[4] = uint8(childCount)                                     // ChildCount
	flags := uint8(0)
	if node.HasEntry {
		flags |= 0x01
	}
	header[5] = flags                                               // Flags
	binary.LittleEndian.PutUint64(header[6:14], uint64(node.PageID)) // PageID
	binary.LittleEndian.PutUint16(header[14:16], node.SlotID)       // SlotID
	binary.LittleEndian.PutUint32(header[16:20], node.SubtreeCount) // SubtreeCount
	binary.LittleEndian.PutUint16(header[20:22], parentIndex)       // ParentIndex

	// Serialize variable data (key + child entries)
	varDataSize := len(node.Key) + childCount*ChildEntrySize
	varData := make([]byte, varDataSize)

	// Copy key
	copy(varData[0:len(node.Key)], node.Key)

	// Serialize child entries - use ChildrenByKey if available
	offset := len(node.Key)
	if node.ChildrenByKey != nil && len(node.ChildrenByKey) > 0 {
		for _, child := range node.ChildrenByKey {
			childIndex, ok := s.nodeIndex[child]
			if !ok {
				childIndex = 0xFFFF // Will be updated later
			}
			firstByte := byte(0)
			if len(child.Key) > 0 {
				firstByte = child.Key[0]
			}
			varData[offset] = firstByte
			binary.LittleEndian.PutUint16(varData[offset+1:offset+3], childIndex)
			offset += ChildEntrySize
		}
	} else {
		for firstByte, child := range node.Children {
			childIndex, ok := s.nodeIndex[child]
			if !ok {
				childIndex = 0xFFFF // Will be updated later
			}
			varData[offset] = firstByte
			binary.LittleEndian.PutUint16(varData[offset+1:offset+3], childIndex)
			offset += ChildEntrySize
		}
	}

	return header, varData, nil
}

// DeserializeNode deserializes a node from bytes.
func (s *NodeSerializer) DeserializeNode(header []byte, varData []byte, keyOffset uint16) (*Node, error) {
	if len(header) < SerializedNodeHeaderSize {
		return nil, ErrBufferTooSmall
	}

	keyLength := binary.LittleEndian.Uint16(header[2:4])
	flags := header[5]
	pageID := storage.PageID(binary.LittleEndian.Uint64(header[6:14]))
	slotID := binary.LittleEndian.Uint16(header[14:16])
	subtreeCount := binary.LittleEndian.Uint32(header[16:20])

	// Calculate required variable data size
	requiredSize := int(keyOffset) + int(keyLength)
	if len(varData) < requiredSize {
		return nil, ErrBufferTooSmall
	}

	// Extract key
	key := string(varData[keyOffset : keyOffset+keyLength])

	// Create node
	node := &Node{
		Key:           key,
		Children:      make(map[byte]*Node),
		ChildrenByKey: make(map[string]*Node),
		HasEntry:      flags&0x01 != 0,
		PageID:        pageID,
		SlotID:        slotID,
		Parent:        nil,
		SubtreeCount:  subtreeCount,
	}

	return node, nil
}

// SerializeNodes serializes multiple nodes to a page buffer.
// The nodes should be provided in a breadth-first order for optimal layout.
func (s *NodeSerializer) SerializeNodes(nodes []*Node, pageID storage.PageID) ([]byte, error) {
	if len(nodes) > MaxNodesPerPage {
		return nil, ErrTooManyNodes
	}

	s.Reset()

	// Build node index
	for i, node := range nodes {
		s.nodeIndex[node] = uint16(i)
	}

	// Calculate total size needed
	totalHeaderSize := storage.PageHeaderSize + 2 // Page header + node count
	totalVarDataSize := 0

	for _, node := range nodes {
		totalHeaderSize += SerializedNodeHeaderSize
		// Use ChildrenByKey count if available
		childCount := len(node.Children)
		if node.ChildrenByKey != nil && len(node.ChildrenByKey) > 0 {
			childCount = len(node.ChildrenByKey)
		}
		totalVarDataSize += len(node.Key) + childCount*ChildEntrySize
	}

	totalSize := totalHeaderSize + totalVarDataSize
	if totalSize > storage.PageSize {
		return nil, ErrTooManyNodes
	}

	// Create page buffer
	buf := make([]byte, storage.PageSize)

	// Write page header
	pageHeader := storage.NewPageHeader(pageID, storage.PageTypeDNIndex)
	pageHeader.ItemCount = uint16(len(nodes))
	pageHeader.FreeSpace = uint16(storage.PageSize - totalSize)
	if err := pageHeader.Serialize(buf[:storage.PageHeaderSize]); err != nil {
		return nil, err
	}

	// Write node count
	binary.LittleEndian.PutUint16(buf[storage.PageHeaderSize:storage.PageHeaderSize+2], uint16(len(nodes)))

	// Calculate where variable data starts
	headerStart := storage.PageHeaderSize + 2
	varDataStart := headerStart + len(nodes)*SerializedNodeHeaderSize

	// Current position in variable data area (relative to varDataStart)
	varDataPos := 0

	for i, node := range nodes {
		// Determine parent index
		parentIndex := NoParentIndex
		if node.Parent != nil {
			if idx, ok := s.nodeIndex[node.Parent]; ok {
				parentIndex = idx
			}
		}

		// Serialize node
		header, varData, err := s.SerializeNode(node, parentIndex)
		if err != nil {
			return nil, err
		}

		// Update key offset in header (relative to varDataStart)
		binary.LittleEndian.PutUint16(header[0:2], uint16(varDataPos))

		// Write header
		headerOffset := headerStart + i*SerializedNodeHeaderSize
		copy(buf[headerOffset:headerOffset+SerializedNodeHeaderSize], header)

		// Write variable data
		copy(buf[varDataStart+varDataPos:varDataStart+varDataPos+len(varData)], varData)
		varDataPos += len(varData)
	}

	return buf, nil
}

// DeserializeNodes deserializes nodes from a page buffer.
// Returns the nodes with parent-child relationships restored.
func (s *NodeSerializer) DeserializeNodes(buf []byte) ([]*Node, error) {
	if len(buf) < storage.PageHeaderSize+2 {
		return nil, ErrBufferTooSmall
	}

	s.Reset()

	// Read page header
	var pageHeader storage.PageHeader
	if err := pageHeader.Deserialize(buf[:storage.PageHeaderSize]); err != nil {
		return nil, err
	}

	// Read node count
	nodeCount := binary.LittleEndian.Uint16(buf[storage.PageHeaderSize : storage.PageHeaderSize+2])

	if nodeCount == 0 {
		return []*Node{}, nil
	}

	// Calculate offsets
	headerStart := storage.PageHeaderSize + 2
	varDataStart := headerStart + int(nodeCount)*SerializedNodeHeaderSize

	// First pass: create all nodes and collect metadata
	nodes := make([]*Node, nodeCount)
	parentIndices := make([]uint16, nodeCount)
	childEntries := make([][]struct {
		firstByte  byte
		childIndex uint16
	}, nodeCount)

	for i := uint16(0); i < nodeCount; i++ {
		headerOffset := headerStart + int(i)*SerializedNodeHeaderSize
		header := buf[headerOffset : headerOffset+SerializedNodeHeaderSize]

		keyOffset := binary.LittleEndian.Uint16(header[0:2])
		keyLength := binary.LittleEndian.Uint16(header[2:4])
		childCount := header[4]
		flags := header[5]
		pageID := storage.PageID(binary.LittleEndian.Uint64(header[6:14]))
		slotID := binary.LittleEndian.Uint16(header[14:16])
		subtreeCount := binary.LittleEndian.Uint32(header[16:20])
		parentIndex := binary.LittleEndian.Uint16(header[20:22])

		// Extract key from variable data (keyOffset is relative to varDataStart)
		actualKeyOffset := varDataStart + int(keyOffset)
		if actualKeyOffset+int(keyLength) > len(buf) {
			return nil, ErrBufferTooSmall
		}
		key := string(buf[actualKeyOffset : actualKeyOffset+int(keyLength)])

		// Create node
		nodes[i] = &Node{
			Key:          key,
			Children:     make(map[byte]*Node, childCount),
			HasEntry:     flags&0x01 != 0,
			PageID:       pageID,
			SlotID:       slotID,
			Parent:       nil,
			SubtreeCount: subtreeCount,
		}
		s.indexNode[i] = nodes[i]
		parentIndices[i] = parentIndex

		// Read child entries (immediately after the key)
		childEntriesOffset := actualKeyOffset + int(keyLength)
		childEntries[i] = make([]struct {
			firstByte  byte
			childIndex uint16
		}, childCount)

		for j := uint8(0); j < childCount; j++ {
			entryOffset := childEntriesOffset + int(j)*ChildEntrySize
			if entryOffset+ChildEntrySize > len(buf) {
				return nil, ErrBufferTooSmall
			}
			childEntries[i][j].firstByte = buf[entryOffset]
			childEntries[i][j].childIndex = binary.LittleEndian.Uint16(buf[entryOffset+1 : entryOffset+3])
		}
	}

	// Second pass: restore relationships
	for i := uint16(0); i < nodeCount; i++ {
		// Initialize ChildrenByKey
		if nodes[i].ChildrenByKey == nil {
			nodes[i].ChildrenByKey = make(map[string]*Node)
		}

		// Set parent
		if parentIndices[i] != NoParentIndex && parentIndices[i] < nodeCount {
			nodes[i].Parent = nodes[parentIndices[i]]
		}

		// Set children
		for _, entry := range childEntries[i] {
			if entry.childIndex < nodeCount {
				child := nodes[entry.childIndex]
				nodes[i].Children[entry.firstByte] = child
				nodes[i].ChildrenByKey[child.Key] = child
			}
		}
	}

	return nodes, nil
}

// CalculateSerializedSize calculates the size needed to serialize a node.
func CalculateSerializedSize(node *Node) int {
	childCount := len(node.Children)
	if node.ChildrenByKey != nil && len(node.ChildrenByKey) > 0 {
		childCount = len(node.ChildrenByKey)
	}
	return SerializedNodeHeaderSize + len(node.Key) + childCount*ChildEntrySize
}

// CalculateTotalSerializedSize calculates the total size needed to serialize multiple nodes.
func CalculateTotalSerializedSize(nodes []*Node) int {
	total := storage.PageHeaderSize + 2 // Page header + node count
	for _, node := range nodes {
		total += CalculateSerializedSize(node)
	}
	return total
}

// CanFitInPage checks if the given nodes can fit in a single page.
func CanFitInPage(nodes []*Node) bool {
	return CalculateTotalSerializedSize(nodes) <= storage.PageSize
}

// CollectSubtree collects all nodes in a subtree in breadth-first order.
func CollectSubtree(root *Node) []*Node {
	if root == nil {
		return nil
	}

	nodes := make([]*Node, 0)
	queue := []*Node{root}

	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		nodes = append(nodes, node)

		// Use ChildrenByKey if available for complete traversal
		if node.ChildrenByKey != nil && len(node.ChildrenByKey) > 0 {
			for _, child := range node.ChildrenByKey {
				queue = append(queue, child)
			}
		} else {
			for _, child := range node.Children {
				queue = append(queue, child)
			}
		}
	}

	return nodes
}

// SerializeToPage serializes a subtree starting from root to a page.
// Returns the serialized page buffer.
func SerializeToPage(root *Node, pageID storage.PageID) ([]byte, error) {
	nodes := CollectSubtree(root)
	serializer := NewNodeSerializer()
	return serializer.SerializeNodes(nodes, pageID)
}

// DeserializeFromPage deserializes nodes from a page buffer.
// Returns the root node (first node in the list).
func DeserializeFromPage(buf []byte) (*Node, error) {
	serializer := NewNodeSerializer()
	nodes, err := serializer.DeserializeNodes(buf)
	if err != nil {
		return nil, err
	}
	if len(nodes) == 0 {
		return nil, nil
	}
	return nodes[0], nil
}
