// Package engine implements the ObaDB storage engine that combines all
// storage components into a unified interface.
//
// # Overview
//
// The engine package provides the main StorageEngine implementation:
//
//   - Transaction management with ACID guarantees
//   - Entry CRUD operations
//   - Search with DN and filter-based queries
//   - Index management
//   - Maintenance operations (checkpoint, compact)
//
// # Creating an Engine
//
// Create a new storage engine:
//
//	opts := &engine.Options{
//	    DataDir:            "/var/lib/oba",
//	    WALDir:             "/var/lib/oba/wal",
//	    PageSize:           4096,
//	    BufferPoolSize:     256 * 1024 * 1024, // 256MB
//	    CheckpointInterval: 5 * time.Minute,
//	}
//
//	eng, err := engine.New(opts)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer eng.Close()
//
// # Basic Operations
//
// Perform CRUD operations within transactions:
//
//	tx, _ := eng.Begin()
//	defer eng.Rollback(tx)
//
//	// Create entry
//	entry := storage.NewEntry("uid=alice,ou=users,dc=example,dc=com")
//	entry.SetStringAttribute("cn", "Alice Smith")
//	eng.Put(tx, entry)
//
//	// Read entry
//	entry, err := eng.Get(tx, "uid=alice,ou=users,dc=example,dc=com")
//
//	// Update entry
//	entry.SetStringAttribute("mail", "alice@example.com")
//	eng.Put(tx, entry)
//
//	// Delete entry
//	eng.Delete(tx, "uid=alice,ou=users,dc=example,dc=com")
//
//	eng.Commit(tx)
//
// # Search Operations
//
// Search by DN scope or filter:
//
//	// Subtree search
//	iter := eng.SearchByDN(tx, "ou=users,dc=example,dc=com", storage.ScopeSubtree)
//	defer iter.Close()
//
//	for iter.Next() {
//	    entry := iter.Entry()
//	    // Process entry
//	}
//
// # Maintenance
//
// Perform maintenance operations:
//
//	// Checkpoint WAL to data file
//	eng.Checkpoint()
//
//	// Compact to reclaim space
//	eng.Compact()
//
//	// Get statistics
//	stats := eng.Stats()
package engine
