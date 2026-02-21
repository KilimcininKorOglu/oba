// Package storage provides the core storage engine components for ObaDB.
package storage

import (
	"errors"
	"sort"
	"sync"
)

// Recovery errors.
var (
	ErrRecoveryFailed     = errors.New("recovery failed")
	ErrNoWAL              = errors.New("WAL is required for recovery")
	ErrNoPageManager      = errors.New("page manager is required for recovery")
	ErrInvalidCheckpoint  = errors.New("invalid checkpoint record")
	ErrRecoveryInProgress = errors.New("recovery is already in progress")
)

// TxState represents the state of a transaction during recovery.
type TxState int

const (
	// TxStateActive indicates the transaction is active (not yet committed or aborted).
	TxStateActive TxState = iota
	// TxStateCommitted indicates the transaction has been committed.
	TxStateCommitted
	// TxStateAborted indicates the transaction has been aborted.
	TxStateAborted
)

// String returns the string representation of a TxState.
func (s TxState) String() string {
	switch s {
	case TxStateActive:
		return "Active"
	case TxStateCommitted:
		return "Committed"
	case TxStateAborted:
		return "Aborted"
	default:
		return "Unknown"
	}
}

// RecoveryTxInfo holds information about a transaction during recovery.
type RecoveryTxInfo struct {
	TxID        uint64
	State       TxState
	FirstLSN    uint64 // First LSN of this transaction
	LastLSN     uint64 // Last LSN of this transaction
	UndoNextLSN uint64 // Next LSN to undo (for rollback)
}

// Recovery implements crash recovery using the WAL.
// It follows a simplified ARIES protocol with three phases:
// 1. Analysis: Scan WAL to find active transactions and dirty pages
// 2. Redo: Replay all logged changes from last checkpoint
// 3. Undo: Roll back uncommitted transactions
type Recovery struct {
	wal         *WAL
	pageManager *PageManager
	bufferPool  *BufferPool

	// activeTx tracks transactions found during analysis
	activeTx map[uint64]*RecoveryTxInfo

	// dirtyPages maps PageID to the first LSN that dirtied it
	dirtyPages map[PageID]uint64

	// checkpointLSN is the LSN of the last checkpoint
	checkpointLSN uint64

	// redoLSN is the starting point for redo phase
	redoLSN uint64

	// allRecords stores all WAL records for processing
	allRecords []*WALRecord

	// mu protects recovery state
	mu sync.Mutex

	// inProgress indicates if recovery is currently running
	inProgress bool
}

// NewRecovery creates a new Recovery instance.
func NewRecovery(wal *WAL, pm *PageManager) *Recovery {
	return &Recovery{
		wal:         wal,
		pageManager: pm,
		activeTx:    make(map[uint64]*RecoveryTxInfo),
		dirtyPages:  make(map[PageID]uint64),
	}
}

// SetBufferPool sets the buffer pool for recovery operations.
func (r *Recovery) SetBufferPool(bp *BufferPool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bufferPool = bp
}

// Recover performs crash recovery by executing the three phases.
// Returns nil if recovery succeeds, or an error if it fails.
func (r *Recovery) Recover() error {
	r.mu.Lock()
	if r.inProgress {
		r.mu.Unlock()
		return ErrRecoveryInProgress
	}
	r.inProgress = true
	r.mu.Unlock()

	defer func() {
		r.mu.Lock()
		r.inProgress = false
		r.mu.Unlock()
	}()

	if r.wal == nil {
		return ErrNoWAL
	}
	if r.pageManager == nil {
		return ErrNoPageManager
	}

	// Reset state
	r.activeTx = make(map[uint64]*RecoveryTxInfo)
	r.dirtyPages = make(map[PageID]uint64)
	r.allRecords = nil
	r.checkpointLSN = 0
	r.redoLSN = 0

	// Phase 1: Analysis
	if err := r.analysis(); err != nil {
		return err
	}

	// Phase 2: Redo
	if err := r.redo(); err != nil {
		return err
	}

	// Phase 3: Undo
	if err := r.undo(); err != nil {
		return err
	}

	return nil
}

// analysis scans the WAL to identify active transactions and dirty pages.
// It determines the starting point for redo and builds the transaction table.
func (r *Recovery) analysis() error {
	// Read all WAL records starting from LSN 1
	iter := r.wal.Iterator(1)
	r.allRecords = make([]*WALRecord, 0)

	for iter.Next() {
		record, err := iter.Record()
		if err != nil {
			// End of valid records
			break
		}
		r.allRecords = append(r.allRecords, record)
	}

	// Process records to build transaction table and dirty page table
	for _, record := range r.allRecords {
		switch record.Type {
		case WALBegin:
			// New transaction started
			r.activeTx[record.TxID] = &RecoveryTxInfo{
				TxID:     record.TxID,
				State:    TxStateActive,
				FirstLSN: record.LSN,
				LastLSN:  record.LSN,
			}

		case WALCommit:
			// Transaction committed
			if txInfo, exists := r.activeTx[record.TxID]; exists {
				txInfo.State = TxStateCommitted
				txInfo.LastLSN = record.LSN
			}

		case WALAbort:
			// Transaction aborted
			if txInfo, exists := r.activeTx[record.TxID]; exists {
				txInfo.State = TxStateAborted
				txInfo.LastLSN = record.LSN
			}

		case WALUpdate:
			// Page modification
			if txInfo, exists := r.activeTx[record.TxID]; exists {
				txInfo.LastLSN = record.LSN
				txInfo.UndoNextLSN = record.LSN
			}

			// Track dirty pages - record the first LSN that dirtied each page
			if _, exists := r.dirtyPages[record.PageID]; !exists {
				r.dirtyPages[record.PageID] = record.LSN
			}

		case WALCheckpoint:
			// Found a checkpoint - update checkpoint LSN
			r.checkpointLSN = record.LSN
		}
	}

	// Determine redo LSN (minimum of checkpoint LSN and minimum dirty page LSN)
	r.redoLSN = r.checkpointLSN
	if r.redoLSN == 0 && len(r.allRecords) > 0 {
		r.redoLSN = r.allRecords[0].LSN
	}

	// Find minimum dirty page LSN
	for _, lsn := range r.dirtyPages {
		if r.redoLSN == 0 || lsn < r.redoLSN {
			r.redoLSN = lsn
		}
	}

	return nil
}

// redo replays all logged changes from the redo LSN.
// This ensures all committed changes are applied to the database.
func (r *Recovery) redo() error {
	// Find records starting from redoLSN
	for _, record := range r.allRecords {
		if record.LSN < r.redoLSN {
			continue
		}

		// Only redo update records
		if record.Type != WALUpdate {
			continue
		}

		// Check if this transaction exists in our records
		_, exists := r.activeTx[record.TxID]
		if !exists {
			continue
		}

		// Redo the update by applying the new data
		if err := r.redoUpdate(record); err != nil {
			// Log error but continue - page might not exist yet
			continue
		}
	}

	// Sync changes to disk
	if err := r.pageManager.Sync(); err != nil {
		return err
	}

	return nil
}

// redoUpdate applies a single update record during redo phase.
func (r *Recovery) redoUpdate(record *WALRecord) error {
	if record.PageID == 0 {
		return nil // Skip invalid page IDs
	}

	// Read the page
	page, err := r.pageManager.ReadPage(record.PageID)
	if err != nil {
		// Page doesn't exist, skip
		return nil
	}

	// Apply the new data at the specified offset
	if record.NewData != nil && len(record.NewData) > 0 {
		offset := int(record.Offset)
		if offset+len(record.NewData) <= len(page.Data) {
			copy(page.Data[offset:], record.NewData)
			page.Header.SetDirty()

			// Write the page back
			if err := r.pageManager.WritePage(page); err != nil {
				return err
			}

			// Update buffer pool if available
			if r.bufferPool != nil {
				if _, err := r.bufferPool.Put(record.PageID, page.Data); err != nil {
					// Non-fatal, continue
				}
			}
		}
	}

	return nil
}

// undo rolls back all uncommitted transactions.
// This ensures atomicity by undoing changes from transactions that didn't commit.
func (r *Recovery) undo() error {
	// Collect all active (uncommitted) transactions
	var undoTxs []*RecoveryTxInfo
	for _, txInfo := range r.activeTx {
		if txInfo.State == TxStateActive {
			undoTxs = append(undoTxs, txInfo)
		}
	}

	if len(undoTxs) == 0 {
		return nil // Nothing to undo
	}

	// Collect all update records from uncommitted transactions
	// Process them in reverse LSN order
	var undoRecords []*WALRecord
	for _, record := range r.allRecords {
		if record.Type != WALUpdate {
			continue
		}

		txInfo, exists := r.activeTx[record.TxID]
		if !exists || txInfo.State != TxStateActive {
			continue
		}

		undoRecords = append(undoRecords, record)
	}

	// Sort by LSN in descending order (undo in reverse order)
	sort.Slice(undoRecords, func(i, j int) bool {
		return undoRecords[i].LSN > undoRecords[j].LSN
	})

	// Undo each record
	for _, record := range undoRecords {
		if err := r.undoUpdate(record); err != nil {
			// Log error but continue
			continue
		}
	}

	// Write abort records for uncommitted transactions
	for _, txInfo := range undoTxs {
		abortRecord := NewWALRecord(0, txInfo.TxID, WALAbort)
		if _, err := r.wal.Append(abortRecord); err != nil {
			return err
		}
		txInfo.State = TxStateAborted
	}

	// Sync WAL and pages
	if err := r.wal.Sync(); err != nil {
		return err
	}
	if err := r.pageManager.Sync(); err != nil {
		return err
	}

	return nil
}

// undoUpdate reverts a single update record during undo phase.
func (r *Recovery) undoUpdate(record *WALRecord) error {
	if record.PageID == 0 {
		return nil // Skip invalid page IDs
	}

	// Read the page
	page, err := r.pageManager.ReadPage(record.PageID)
	if err != nil {
		// Page doesn't exist, skip
		return nil
	}

	// Apply the old data at the specified offset (undo the change)
	if record.OldData != nil && len(record.OldData) > 0 {
		offset := int(record.Offset)
		if offset+len(record.OldData) <= len(page.Data) {
			copy(page.Data[offset:], record.OldData)
			page.Header.SetDirty()

			// Write the page back
			if err := r.pageManager.WritePage(page); err != nil {
				return err
			}

			// Update buffer pool if available
			if r.bufferPool != nil {
				if _, err := r.bufferPool.Put(record.PageID, page.Data); err != nil {
					// Non-fatal, continue
				}
			}
		}
	}

	return nil
}

// GetActiveTx returns a copy of the active transaction map.
func (r *Recovery) GetActiveTx() map[uint64]*RecoveryTxInfo {
	r.mu.Lock()
	defer r.mu.Unlock()

	result := make(map[uint64]*RecoveryTxInfo)
	for k, v := range r.activeTx {
		result[k] = &RecoveryTxInfo{
			TxID:        v.TxID,
			State:       v.State,
			FirstLSN:    v.FirstLSN,
			LastLSN:     v.LastLSN,
			UndoNextLSN: v.UndoNextLSN,
		}
	}
	return result
}

// GetDirtyPages returns a copy of the dirty pages map.
func (r *Recovery) GetDirtyPages() map[PageID]uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()

	result := make(map[PageID]uint64)
	for k, v := range r.dirtyPages {
		result[k] = v
	}
	return result
}

// GetCheckpointLSN returns the checkpoint LSN found during analysis.
func (r *Recovery) GetCheckpointLSN() uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.checkpointLSN
}

// GetRedoLSN returns the redo starting LSN.
func (r *Recovery) GetRedoLSN() uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.redoLSN
}

// IsInProgress returns true if recovery is currently running.
func (r *Recovery) IsInProgress() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.inProgress
}
