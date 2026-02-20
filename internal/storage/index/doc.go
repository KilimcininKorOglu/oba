// Package index implements attribute indexing for the ObaDB storage engine.
//
// # Overview
//
// The index package provides fast attribute-based lookups using B+ trees:
//
//   - Equality indexes for exact matches: (uid=alice)
//   - Presence indexes for existence checks: (mail=*)
//   - Substring indexes for pattern matching: (cn=*admin*)
//
// # Index Manager
//
// The IndexManager coordinates all indexes:
//
//	manager := index.NewManager(pageManager)
//
//	// Create index
//	err := manager.CreateIndex("uid", index.TypeEquality)
//
//	// Use index for search
//	iter := manager.Search("uid", []byte("alice"))
//	for iter.Next() {
//	    pageID := iter.PageID()
//	}
//
// # Index Types
//
// Three index types are supported:
//
//	index.TypeEquality   // For (attr=value) filters
//	index.TypePresence   // For (attr=*) filters
//	index.TypeSubstring  // For (attr=*value*) filters
//
// # Substring Indexing
//
// Substring indexes use n-gram tokenization:
//
//	// "alice" is tokenized to: ["ali", "lic", "ice"]
//	// Search for "*lic*" finds entries containing "lic"
//
// # Index Maintenance
//
// Indexes are updated automatically on entry changes:
//
//	// On entry add
//	manager.IndexEntry(entry)
//
//	// On entry delete
//	manager.UnindexEntry(entry)
//
//	// On entry modify
//	manager.ReindexEntry(oldEntry, newEntry)
package index
