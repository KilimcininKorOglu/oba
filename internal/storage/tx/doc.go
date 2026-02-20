// Package tx implements transaction management for the ObaDB storage engine.
//
// # Overview
//
// The tx package provides ACID transaction support:
//
//   - Atomicity: All-or-nothing operations via WAL
//   - Consistency: Schema validation before commit
//   - Isolation: MVCC-based snapshot isolation
//   - Durability: WAL fsync on commit
//
// # Transaction Lifecycle
//
// Transactions follow a standard lifecycle:
//
//	// Begin transaction
//	tx, err := manager.Begin()
//	if err != nil {
//	    return err
//	}
//
//	// Perform operations
//	err = tx.Put(entry)
//	if err != nil {
//	    manager.Rollback(tx)
//	    return err
//	}
//
//	// Commit or rollback
//	err = manager.Commit(tx)
//
// # Transaction States
//
// Transactions progress through states:
//
//   - Active: Transaction is in progress
//   - Committed: Changes are durable
//   - Aborted: Changes are rolled back
//
// # Write Sets
//
// Transactions track modified pages:
//
//	tx.WriteSet  // Pages modified by this transaction
//	tx.ReadSet   // Pages read by this transaction
//
// # Conflict Detection
//
// Write-write conflicts are detected at commit time:
//
//	if manager.HasConflict(tx) {
//	    manager.Rollback(tx)
//	    return ErrConflict
//	}
package tx
