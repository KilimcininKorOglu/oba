// Package radix implements a radix tree (compressed trie) for DN hierarchy
// traversal in the ObaDB storage engine.
//
// # Overview
//
// The radix tree is optimized for LDAP's hierarchical namespace. It provides:
//
//   - O(k) lookup where k is the number of DN components
//   - Efficient subtree iteration for scope=subtree searches
//   - Parent pointers for scope=onelevel searches
//   - Path compression for memory efficiency
//
// # DN Structure
//
// DNs are stored as paths in the tree:
//
//	Root
//	 └─ dc=example
//	     └─ dc=com
//	         ├─ ou=users
//	         │   ├─ uid=alice  → Page 42, Slot 3
//	         │   └─ uid=bob    → Page 42, Slot 7
//	         └─ ou=groups
//	             └─ cn=admins  → Page 58, Slot 1
//
// # Usage
//
// Create and use a radix tree:
//
//	tree := radix.NewTree()
//
//	// Insert DN with page location
//	tree.Insert("uid=alice,ou=users,dc=example,dc=com", pageID, slotID)
//
//	// Lookup DN
//	pageID, slotID, found := tree.Lookup("uid=alice,ou=users,dc=example,dc=com")
//
//	// Iterate subtree
//	iter := tree.Subtree("ou=users,dc=example,dc=com")
//	for iter.Next() {
//	    dn := iter.DN()
//	    pageID, slotID := iter.Location()
//	}
//
// # DN Parsing
//
// The package includes utilities for parsing and normalizing DNs:
//
//	components := radix.ParseDN("uid=alice,ou=users,dc=example,dc=com")
//	// Returns: ["uid=alice", "ou=users", "dc=example", "dc=com"]
package radix
