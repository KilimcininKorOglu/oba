// Package btree provides B+ Tree implementation for attribute indexing in ObaDB.
package btree

import (
	"encoding/binary"
	"errors"

	"github.com/KilimcininKorOglu/oba/internal/storage"
)

// Serialization constants.
const (
	// BPlusNodeHeaderSize is the size of the B+ tree node header in bytes.
	// Layout:
	//   - Byte 0:      IsLeaf (uint8, 0 or 1)
	//   - Bytes 1-2:   KeyCount (uint16)
	//   - Bytes 3-10:  NextLeaf (PageID/uint64)
	//   - Bytes 11-18: PrevLeaf (PageID/uint64)
	//   - Bytes 19-20: Reserved (2 bytes for alignment)
	BPlusNodeHeaderSize = 21

	// MaxKeySize is the maximum size of a single key in bytes.
	// Keys larger than this will cause an error during serialization.
	MaxKeySize = 1024

	// KeyLengthSize is the size of the key length prefix (2 bytes for uint16).
	KeyLengthSize = 2

	// EntryRefSize is the size of an EntryRef in bytes (8 + 2 = 10).
	EntryRefSize = 10

	// PageIDSize is the size of a PageID in bytes.
	PageIDSize = 8
)

// Serialization errors.
var (
	ErrKeyTooLarge        = errors.New("key exceeds maximum size")
	ErrBufferTooSmall     = errors.New("buffer too small for serialization")
	ErrInvalidNodeData    = errors.New("invalid node data")
	ErrNodeTooLarge       = errors.New("node data exceeds page size")
	ErrCorruptedNode      = errors.New("corrupted node data")
	ErrInvalidKeyCount    = errors.New("invalid key count")
	ErrInvalidChildCount  = errors.New("invalid child count for internal node")
	ErrMismatchedKeyValue = errors.New("mismatched key and value count in leaf node")
)

// SerializedSize calculates the serialized size of a B+ tree node.
// This is useful for checking if the node will fit in a page.
func (n *BPlusNode) SerializedSize() int {
	size := BPlusNodeHeaderSize

	// Add key sizes (length prefix + key data)
	for _, key := range n.Keys {
		size += KeyLengthSize + len(key)
	}

	if n.IsLeaf {
		// Leaf nodes store entry references
		size += len(n.Values) * EntryRefSize
	} else {
		// Internal nodes store child page IDs
		size += len(n.Children) * PageIDSize
	}

	return size
}

// FitsInPage returns true if the node can be serialized within a page.
func (n *BPlusNode) FitsInPage() bool {
	// Account for page header
	availableSpace := storage.PageSize - storage.PageHeaderSize
	return n.SerializedSize() <= availableSpace
}

// Serialize writes the B+ tree node to a byte slice.
// Returns the number of bytes written.
func (n *BPlusNode) Serialize(buf []byte) (int, error) {
	requiredSize := n.SerializedSize()
	if len(buf) < requiredSize {
		return 0, ErrBufferTooSmall
	}

	// Validate keys
	for _, key := range n.Keys {
		if len(key) > MaxKeySize {
			return 0, ErrKeyTooLarge
		}
	}

	// Validate internal node structure
	if !n.IsLeaf && len(n.Children) != len(n.Keys)+1 && len(n.Keys) > 0 {
		return 0, ErrInvalidChildCount
	}

	// Validate leaf node structure
	if n.IsLeaf && len(n.Values) != len(n.Keys) {
		return 0, ErrMismatchedKeyValue
	}

	offset := 0

	// Write header
	if n.IsLeaf {
		buf[offset] = 1
	} else {
		buf[offset] = 0
	}
	offset++

	binary.LittleEndian.PutUint16(buf[offset:offset+2], uint16(len(n.Keys)))
	offset += 2

	binary.LittleEndian.PutUint64(buf[offset:offset+8], uint64(n.Next))
	offset += 8

	binary.LittleEndian.PutUint64(buf[offset:offset+8], uint64(n.Prev))
	offset += 8

	// Reserved bytes
	buf[offset] = 0
	buf[offset+1] = 0
	offset += 2

	// Write keys (length-prefixed)
	for _, key := range n.Keys {
		binary.LittleEndian.PutUint16(buf[offset:offset+2], uint16(len(key)))
		offset += 2
		copy(buf[offset:], key)
		offset += len(key)
	}

	// Write values or children
	if n.IsLeaf {
		for _, ref := range n.Values {
			binary.LittleEndian.PutUint64(buf[offset:offset+8], uint64(ref.PageID))
			offset += 8
			binary.LittleEndian.PutUint16(buf[offset:offset+2], ref.SlotID)
			offset += 2
		}
	} else {
		for _, child := range n.Children {
			binary.LittleEndian.PutUint64(buf[offset:offset+8], uint64(child))
			offset += 8
		}
	}

	return offset, nil
}

// Deserialize reads a B+ tree node from a byte slice.
// The pageID parameter is used to set the node's PageID field.
func (n *BPlusNode) Deserialize(buf []byte, pageID storage.PageID) error {
	if len(buf) < BPlusNodeHeaderSize {
		return ErrBufferTooSmall
	}

	offset := 0

	// Read header
	n.IsLeaf = buf[offset] == 1
	offset++

	keyCount := int(binary.LittleEndian.Uint16(buf[offset : offset+2]))
	offset += 2

	n.Next = storage.PageID(binary.LittleEndian.Uint64(buf[offset : offset+8]))
	offset += 8

	n.Prev = storage.PageID(binary.LittleEndian.Uint64(buf[offset : offset+8]))
	offset += 8

	// Skip reserved bytes
	offset += 2

	n.PageID = pageID

	// Validate key count
	if n.IsLeaf && keyCount > BPlusLeafCapacity {
		return ErrInvalidKeyCount
	}
	if !n.IsLeaf && keyCount > BPlusOrder-1 {
		return ErrInvalidKeyCount
	}

	// Read keys
	n.Keys = make([][]byte, keyCount)
	for i := 0; i < keyCount; i++ {
		if offset+KeyLengthSize > len(buf) {
			return ErrCorruptedNode
		}

		keyLen := int(binary.LittleEndian.Uint16(buf[offset : offset+2]))
		offset += 2

		if keyLen > MaxKeySize {
			return ErrKeyTooLarge
		}

		if offset+keyLen > len(buf) {
			return ErrCorruptedNode
		}

		n.Keys[i] = make([]byte, keyLen)
		copy(n.Keys[i], buf[offset:offset+keyLen])
		offset += keyLen
	}

	// Read values or children
	if n.IsLeaf {
		n.Values = make([]EntryRef, keyCount)
		n.Children = nil

		for i := 0; i < keyCount; i++ {
			if offset+EntryRefSize > len(buf) {
				return ErrCorruptedNode
			}

			n.Values[i].PageID = storage.PageID(binary.LittleEndian.Uint64(buf[offset : offset+8]))
			offset += 8
			n.Values[i].SlotID = binary.LittleEndian.Uint16(buf[offset : offset+2])
			offset += 2
		}
	} else {
		childCount := keyCount + 1
		if keyCount == 0 {
			childCount = 0
		}

		n.Children = make([]storage.PageID, childCount)
		n.Values = nil

		for i := 0; i < childCount; i++ {
			if offset+PageIDSize > len(buf) {
				return ErrCorruptedNode
			}

			n.Children[i] = storage.PageID(binary.LittleEndian.Uint64(buf[offset : offset+8]))
			offset += 8
		}
	}

	return nil
}

// SerializeToPage serializes the B+ tree node to a storage page.
// The page type is set to PageTypeAttrIndex.
func (n *BPlusNode) SerializeToPage(page *storage.Page) error {
	if !n.FitsInPage() {
		return ErrNodeTooLarge
	}

	// Clear page data
	for i := range page.Data {
		page.Data[i] = 0
	}

	// Serialize node to page data
	_, err := n.Serialize(page.Data)
	if err != nil {
		return err
	}

	// Update page header
	page.Header.PageType = storage.PageTypeAttrIndex
	page.Header.ItemCount = uint16(len(n.Keys))

	if n.IsLeaf {
		page.Header.SetLeaf()
	} else {
		page.Header.Flags &^= storage.PageFlagLeaf
	}

	page.Header.FreeSpace = uint16(storage.PageSize - storage.PageHeaderSize - n.SerializedSize())
	page.Header.SetDirty()

	return nil
}

// DeserializeFromPage deserializes a B+ tree node from a storage page.
func (n *BPlusNode) DeserializeFromPage(page *storage.Page) error {
	if page.Header.PageType != storage.PageTypeAttrIndex {
		return ErrInvalidNodeData
	}

	return n.Deserialize(page.Data, page.Header.PageID)
}

// NewNodeFromPage creates a new B+ tree node from a storage page.
func NewNodeFromPage(page *storage.Page) (*BPlusNode, error) {
	node := &BPlusNode{}
	if err := node.DeserializeFromPage(page); err != nil {
		return nil, err
	}
	return node, nil
}

// CreatePage creates a new storage page for this node.
func (n *BPlusNode) CreatePage() (*storage.Page, error) {
	page := storage.NewPage(n.PageID, storage.PageTypeAttrIndex)
	if err := n.SerializeToPage(page); err != nil {
		return nil, err
	}
	return page, nil
}

// EncodeKey encodes a variable-length key with a length prefix.
// Returns the encoded key as a byte slice.
func EncodeKey(key []byte) ([]byte, error) {
	if len(key) > MaxKeySize {
		return nil, ErrKeyTooLarge
	}

	encoded := make([]byte, KeyLengthSize+len(key))
	binary.LittleEndian.PutUint16(encoded[0:2], uint16(len(key)))
	copy(encoded[2:], key)

	return encoded, nil
}

// DecodeKey decodes a length-prefixed key from a byte slice.
// Returns the key and the number of bytes consumed.
func DecodeKey(buf []byte) ([]byte, int, error) {
	if len(buf) < KeyLengthSize {
		return nil, 0, ErrBufferTooSmall
	}

	keyLen := int(binary.LittleEndian.Uint16(buf[0:2]))
	if keyLen > MaxKeySize {
		return nil, 0, ErrKeyTooLarge
	}

	totalLen := KeyLengthSize + keyLen
	if len(buf) < totalLen {
		return nil, 0, ErrBufferTooSmall
	}

	key := make([]byte, keyLen)
	copy(key, buf[KeyLengthSize:totalLen])

	return key, totalLen, nil
}

// EncodeEntryRef encodes an EntryRef to a byte slice.
func EncodeEntryRef(ref EntryRef) []byte {
	buf := make([]byte, EntryRefSize)
	binary.LittleEndian.PutUint64(buf[0:8], uint64(ref.PageID))
	binary.LittleEndian.PutUint16(buf[8:10], ref.SlotID)
	return buf
}

// DecodeEntryRef decodes an EntryRef from a byte slice.
func DecodeEntryRef(buf []byte) (EntryRef, error) {
	if len(buf) < EntryRefSize {
		return EntryRef{}, ErrBufferTooSmall
	}

	return EntryRef{
		PageID: storage.PageID(binary.LittleEndian.Uint64(buf[0:8])),
		SlotID: binary.LittleEndian.Uint16(buf[8:10]),
	}, nil
}

// CalculateMaxKeysForSize calculates the maximum number of keys that can fit
// in a given buffer size, assuming average key size.
func CalculateMaxKeysForSize(bufSize int, avgKeySize int, isLeaf bool) int {
	availableSpace := bufSize - BPlusNodeHeaderSize

	if isLeaf {
		// Each entry needs: key length (2) + key data + entry ref (10)
		entrySize := KeyLengthSize + avgKeySize + EntryRefSize
		return availableSpace / entrySize
	}

	// Internal node: key length (2) + key data + child pointer (8)
	// Plus one extra child pointer
	entrySize := KeyLengthSize + avgKeySize + PageIDSize
	maxKeys := (availableSpace - PageIDSize) / entrySize
	return maxKeys
}
