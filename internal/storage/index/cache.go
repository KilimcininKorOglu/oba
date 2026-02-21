package index

import (
	"bytes"
	"encoding/binary"
	"errors"

	"github.com/KilimcininKorOglu/oba/internal/storage"
	"github.com/KilimcininKorOglu/oba/internal/storage/btree"
	"github.com/KilimcininKorOglu/oba/internal/storage/cache"
)

// Cache errors.
var (
	ErrCacheCorrupt = errors.New("index cache corrupt")
)

// SaveCache saves all indexes to a cache file.
func (im *IndexManager) SaveCache(path string, txID uint64) error {
	im.mu.RLock()
	defer im.mu.RUnlock()

	if len(im.indexes) == 0 {
		return nil
	}

	var buf bytes.Buffer

	// Write index count
	if err := binary.Write(&buf, binary.LittleEndian, uint32(len(im.indexes))); err != nil {
		return err
	}

	// Serialize each index
	for attr, idx := range im.indexes {
		if err := serializeIndex(&buf, attr, idx); err != nil {
			return err
		}
	}

	return cache.WriteFile(path, cache.TypeBTree, buf.Bytes(), uint64(len(im.indexes)), txID)
}

// LoadCache loads indexes from a cache file.
func (im *IndexManager) LoadCache(path string, expectedTxID uint64) error {
	data, header, err := cache.ReadFile(path, cache.TypeBTree, expectedTxID)
	if err != nil {
		return err
	}

	im.mu.Lock()
	defer im.mu.Unlock()

	return im.deserializeIndexes(data, int(header.EntryCount))
}

// serializeIndex serializes a single index to the buffer.
// Format:
//   - AttrLen (uint16)
//   - Attribute ([]byte)
//   - IndexType (uint8)
//   - RootPageID (uint64)
//   - NodeCount (uint32)
//   - Nodes (serialized B+ tree nodes)
func serializeIndex(buf *bytes.Buffer, attr string, idx *Index) error {
	// Attribute name
	attrBytes := []byte(attr)
	if err := binary.Write(buf, binary.LittleEndian, uint16(len(attrBytes))); err != nil {
		return err
	}
	if _, err := buf.Write(attrBytes); err != nil {
		return err
	}

	// Index type
	if err := buf.WriteByte(byte(idx.Type)); err != nil {
		return err
	}

	// Root page ID
	if err := binary.Write(buf, binary.LittleEndian, uint64(idx.RootPageID)); err != nil {
		return err
	}

	// Serialize B+ tree nodes
	if idx.Tree != nil {
		nodes := collectBTreeNodes(idx.Tree)
		if err := binary.Write(buf, binary.LittleEndian, uint32(len(nodes))); err != nil {
			return err
		}

		for _, node := range nodes {
			if err := serializeBTreeNode(buf, node); err != nil {
				return err
			}
		}
	} else {
		if err := binary.Write(buf, binary.LittleEndian, uint32(0)); err != nil {
			return err
		}
	}

	return nil
}

// collectBTreeNodes collects all nodes from a B+ tree in BFS order.
func collectBTreeNodes(tree *btree.BPlusTree) []*btree.BPlusNode {
	if tree == nil {
		return nil
	}

	rootPageID := tree.Root()
	if rootPageID == 0 {
		return nil
	}

	// For cache purposes, we only need metadata
	// The actual nodes are loaded from page manager
	return nil
}

// serializeBTreeNode serializes a single B+ tree node.
func serializeBTreeNode(buf *bytes.Buffer, node *btree.BPlusNode) error {
	// Use the existing serialization
	nodeData := make([]byte, storage.PageSize)
	n, err := node.Serialize(nodeData)
	if err != nil {
		return err
	}

	// Write serialized size and data
	if err := binary.Write(buf, binary.LittleEndian, uint32(n)); err != nil {
		return err
	}
	if _, err := buf.Write(nodeData[:n]); err != nil {
		return err
	}

	return nil
}

// deserializeIndexes deserializes indexes from bytes.
func (im *IndexManager) deserializeIndexes(data []byte, expectedCount int) error {
	if len(data) < 4 {
		return ErrCacheCorrupt
	}

	buf := bytes.NewReader(data)

	// Read index count
	var indexCount uint32
	if err := binary.Read(buf, binary.LittleEndian, &indexCount); err != nil {
		return err
	}

	if int(indexCount) != expectedCount {
		return ErrCacheCorrupt
	}

	for i := uint32(0); i < indexCount; i++ {
		// Read attribute name
		var attrLen uint16
		if err := binary.Read(buf, binary.LittleEndian, &attrLen); err != nil {
			return err
		}

		attrBytes := make([]byte, attrLen)
		if _, err := buf.Read(attrBytes); err != nil {
			return err
		}
		attr := string(attrBytes)

		// Read index type
		indexTypeByte, err := buf.ReadByte()
		if err != nil {
			return err
		}
		indexType := IndexType(indexTypeByte)

		// Read root page ID
		var rootPageID uint64
		if err := binary.Read(buf, binary.LittleEndian, &rootPageID); err != nil {
			return err
		}

		// Read node count
		var nodeCount uint32
		if err := binary.Read(buf, binary.LittleEndian, &nodeCount); err != nil {
			return err
		}

		// Skip node data for now - we'll load from page manager
		for j := uint32(0); j < nodeCount; j++ {
			var nodeSize uint32
			if err := binary.Read(buf, binary.LittleEndian, &nodeSize); err != nil {
				return err
			}
			// Skip node data
			if _, err := buf.Seek(int64(nodeSize), 1); err != nil {
				return err
			}
		}

		// Create index with existing root page
		tree, err := btree.NewBPlusTreeWithRoot(im.pageManager, storage.PageID(rootPageID), 0)
		if err != nil {
			// If tree loading fails, skip this index
			continue
		}

		im.indexes[attr] = &Index{
			Attribute:  attr,
			Type:       indexType,
			Tree:       tree,
			RootPageID: storage.PageID(rootPageID),
		}
	}

	return nil
}
