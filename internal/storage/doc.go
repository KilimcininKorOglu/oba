// Package storage provides the core storage engine components for ObaDB,
// a purpose-built embedded database optimized for LDAP workloads.
//
// # Overview
//
// ObaDB is a custom storage engine designed specifically for LDAP directory
// services. It provides:
//
//   - ACID transactions with MVCC (Multi-Version Concurrency Control)
//   - Write-Ahead Logging (WAL) for crash recovery
//   - Radix tree indexing for DN hierarchy traversal
//   - B+ tree indexing for attribute-based searches
//   - Memory-mapped I/O for efficient reads
//   - Buffer pool with LRU eviction
//
// # Storage Engine Interface
//
// The StorageEngine interface defines all database operations:
//
//	type StorageEngine interface {
//	    // Transaction management
//	    Begin() (interface{}, error)
//	    Commit(tx interface{}) error
//	    Rollback(tx interface{}) error
//
//	    // Entry operations
//	    Get(tx interface{}, dn string) (*Entry, error)
//	    Put(tx interface{}, entry *Entry) error
//	    Delete(tx interface{}, dn string) error
//
//	    // Search operations
//	    SearchByDN(tx interface{}, baseDN string, scope Scope) Iterator
//	    SearchByFilter(tx interface{}, baseDN string, f interface{}) Iterator
//
//	    // Index management
//	    CreateIndex(attribute string, indexType IndexType) error
//	    DropIndex(attribute string) error
//
//	    // Maintenance
//	    Checkpoint() error
//	    Compact() error
//	    Stats() *EngineStats
//	    Close() error
//	}
//
// # Search Scopes
//
// LDAP search scopes control which entries are examined:
//
//   - ScopeBase: Only the base entry itself
//   - ScopeOneLevel: Immediate children of the base entry
//   - ScopeSubtree: Base entry and all descendants
//
// # Index Types
//
// Indexes accelerate attribute-based searches:
//
//   - IndexEquality: For equality searches like (uid=alice)
//   - IndexPresence: For presence searches like (mail=*)
//   - IndexSubstring: For substring searches like (cn=*admin*)
//
// # Entry Structure
//
// Entries store DN and multi-valued attributes:
//
//	entry := storage.NewEntry("uid=alice,ou=users,dc=example,dc=com")
//	entry.SetAttribute("objectClass", [][]byte{[]byte("person"), []byte("top")})
//	entry.SetAttribute("cn", [][]byte{[]byte("Alice Smith")})
//
// # Transaction Usage
//
// All operations should be performed within transactions:
//
//	tx, err := engine.Begin()
//	if err != nil {
//	    return err
//	}
//	defer engine.Rollback(tx) // Rollback if not committed
//
//	entry, err := engine.Get(tx, "uid=alice,ou=users,dc=example,dc=com")
//	if err != nil {
//	    return err
//	}
//
//	// Modify entry...
//
//	if err := engine.Put(tx, entry); err != nil {
//	    return err
//	}
//
//	return engine.Commit(tx)
//
// # Iterator Usage
//
// Search operations return iterators for efficient result streaming:
//
//	iter := engine.SearchByDN(tx, "ou=users,dc=example,dc=com", storage.ScopeSubtree)
//	defer iter.Close()
//
//	for iter.Next() {
//	    entry := iter.Entry()
//	    // Process entry
//	}
//
//	if err := iter.Error(); err != nil {
//	    return err
//	}
package storage
