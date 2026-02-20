// Package mvcc implements Multi-Version Concurrency Control for the ObaDB
// storage engine.
//
// # Overview
//
// MVCC provides transaction isolation without blocking readers. It enables:
//
//   - Snapshot isolation for consistent reads
//   - Non-blocking reads during writes
//   - Copy-on-write for safe updates
//   - Garbage collection of old versions
//
// # Snapshot Isolation
//
// Each transaction sees a consistent snapshot of the database:
//
//	snapshot := mvcc.NewSnapshot(store, txID)
//
//	// Read sees data as of snapshot creation
//	entry, err := snapshot.Get(dn)
//
//	// Concurrent writes don't affect this snapshot
//
// # Copy-on-Write
//
// Modifications create new versions without modifying existing data:
//
//	cow := mvcc.NewCOW(store)
//
//	// Create new version of page
//	newPageID, err := cow.CopyPage(oldPageID)
//
//	// Modify new page (original unchanged)
//	err = cow.ModifyPage(newPageID, data)
//
// # Version Management
//
// Versions are tracked with transaction IDs:
//
//	version := &mvcc.Version{
//	    TxID:      txID,
//	    PageID:    pageID,
//	    CreatedAt: time.Now(),
//	}
//
// # Garbage Collection
//
// Old versions are cleaned up when no longer needed:
//
//	gc := mvcc.NewGC(store)
//
//	// Remove versions older than oldest active transaction
//	gc.Collect(oldestActiveTxID)
package mvcc
