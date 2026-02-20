// Package btree implements a B+ tree data structure for attribute indexing
// in the ObaDB storage engine.
//
// # Overview
//
// B+ trees are used for efficient attribute-based searches in LDAP. They provide:
//
//   - O(log n) lookup, insertion, and deletion
//   - Efficient range scans via leaf node linking
//   - Page-aligned nodes for disk storage
//
// # Node Structure
//
// B+ tree nodes are stored in 4KB pages:
//
//   - Internal nodes: Keys and child page pointers
//   - Leaf nodes: Keys, values, and sibling pointers
//
// # Usage
//
// Create and use a B+ tree:
//
//	tree := btree.NewTree(pageManager, rootPageID)
//
//	// Insert key-value pair
//	err := tree.Insert([]byte("uid=alice"), pageID)
//
//	// Search for key
//	value, found := tree.Search([]byte("uid=alice"))
//
//	// Range scan
//	iter := tree.Range([]byte("uid=a"), []byte("uid=z"))
//	for iter.Next() {
//	    key, value := iter.KeyValue()
//	}
//
// # Serialization
//
// Nodes are serialized to/from byte slices for disk storage:
//
//	data := node.Serialize()
//	node, err := btree.DeserializeNode(data)
package btree
