// Package storage provides the core storage engine components for ObaDB.
package storage

import (
	"encoding/binary"
	"errors"
	"sync"
	"time"
)

// Checkpoint errors.
var (
	ErrCheckpointFailed    = errors.New("checkpoint failed")
	ErrCheckpointInProgress = errors.New("checkpoint is already in progress")
	ErrNoActiveCheckpoint  = errors.New("no active checkpoint")
)

// CheckpointData contains the data stored in a checkpoint record.
// This includes information about active transactions and dirty pages
// at the time of the checkpoint.
type CheckpointData struct {
	// Timestamp is when the checkpoint was created.
	Timestamp time.Time

	// ActiveTxIDs contains the IDs of transactions that were active.
	ActiveTxIDs []uint64

	// DirtyPageIDs contains the IDs of pages that were dirty.
	DirtyPageIDs []PageID

	// LastLSN is the LSN at the time of checkpoint.
	LastLSN uint64
}

// Serialize converts the checkpoint data to bytes for storage in WAL.
func (cd *CheckpointData) Serialize() []byte {
	// Calculate size:
	// - 8 bytes for timestamp (Unix nano)
	// - 8 bytes for LastLSN
	// - 4 bytes for number of active transactions
	// - 8 bytes per active transaction ID
	// - 4 bytes for number of dirty pages
	// - 8 bytes per dirty page ID
	size := 8 + 8 + 4 + len(cd.ActiveTxIDs)*8 + 4 + len(cd.DirtyPageIDs)*8
	buf := make([]byte, size)

	offset := 0

	// Write timestamp
	binary.LittleEndian.PutUint64(buf[offset:], uint64(cd.Timestamp.UnixNano()))
	offset += 8

	// Write LastLSN
	binary.LittleEndian.PutUint64(buf[offset:], cd.LastLSN)
	offset += 8

	// Write active transaction count and IDs
	binary.LittleEndian.PutUint32(buf[offset:], uint32(len(cd.ActiveTxIDs)))
	offset += 4
	for _, txID := range cd.ActiveTxIDs {
		binary.LittleEndian.PutUint64(buf[offset:], txID)
		offset += 8
	}

	// Write dirty page count and IDs
	binary.LittleEndian.PutUint32(buf[offset:], uint32(len(cd.DirtyPageIDs)))
	offset += 4
	for _, pageID := range cd.DirtyPageIDs {
		binary.LittleEndian.PutUint64(buf[offset:], uint64(pageID))
		offset += 8
	}

	return buf
}

// Deserialize reads checkpoint data from bytes.
func (cd *CheckpointData) Deserialize(buf []byte) error {
	if len(buf) < 24 { // Minimum: timestamp + LastLSN + 2 counts
		return ErrInvalidCheckpoint
	}

	offset := 0

	// Read timestamp
	timestamp := binary.LittleEndian.Uint64(buf[offset:])
	cd.Timestamp = time.Unix(0, int64(timestamp))
	offset += 8

	// Read LastLSN
	cd.LastLSN = binary.LittleEndian.Uint64(buf[offset:])
	offset += 8

	// Read active transaction count and IDs
	if offset+4 > len(buf) {
		return ErrInvalidCheckpoint
	}
	txCount := binary.LittleEndian.Uint32(buf[offset:])
	offset += 4

	if offset+int(txCount)*8 > len(buf) {
		return ErrInvalidCheckpoint
	}
	cd.ActiveTxIDs = make([]uint64, txCount)
	for i := uint32(0); i < txCount; i++ {
		cd.ActiveTxIDs[i] = binary.LittleEndian.Uint64(buf[offset:])
		offset += 8
	}

	// Read dirty page count and IDs
	if offset+4 > len(buf) {
		return ErrInvalidCheckpoint
	}
	pageCount := binary.LittleEndian.Uint32(buf[offset:])
	offset += 4

	if offset+int(pageCount)*8 > len(buf) {
		return ErrInvalidCheckpoint
	}
	cd.DirtyPageIDs = make([]PageID, pageCount)
	for i := uint32(0); i < pageCount; i++ {
		cd.DirtyPageIDs[i] = PageID(binary.LittleEndian.Uint64(buf[offset:]))
		offset += 8
	}

	return nil
}

// CheckpointManager manages checkpoint operations.
// Checkpoints reduce recovery time by recording the database state
// at specific points, allowing recovery to start from the checkpoint
// instead of the beginning of the WAL.
type CheckpointManager struct {
	wal         *WAL
	pageManager *PageManager
	bufferPool  *BufferPool

	// lastCheckpointLSN is the LSN of the last successful checkpoint.
	lastCheckpointLSN uint64

	// lastCheckpointTime is when the last checkpoint was taken.
	lastCheckpointTime time.Time

	// checkpointInterval is the minimum time between automatic checkpoints.
	checkpointInterval time.Duration

	// mu protects checkpoint state.
	mu sync.Mutex

	// inProgress indicates if a checkpoint is currently running.
	inProgress bool

	// getActiveTxIDs is a callback to get active transaction IDs.
	getActiveTxIDs func() []uint64
}

// NewCheckpointManager creates a new CheckpointManager.
func NewCheckpointManager(wal *WAL, pm *PageManager) *CheckpointManager {
	return &CheckpointManager{
		wal:                wal,
		pageManager:        pm,
		checkpointInterval: 5 * time.Minute, // Default interval
	}
}

// SetBufferPool sets the buffer pool for checkpoint operations.
func (cm *CheckpointManager) SetBufferPool(bp *BufferPool) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.bufferPool = bp
}

// SetCheckpointInterval sets the minimum interval between checkpoints.
func (cm *CheckpointManager) SetCheckpointInterval(interval time.Duration) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.checkpointInterval = interval
}

// SetActiveTxCallback sets the callback to get active transaction IDs.
func (cm *CheckpointManager) SetActiveTxCallback(callback func() []uint64) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.getActiveTxIDs = callback
}

// Checkpoint performs a checkpoint operation.
// This involves:
// 1. Flushing all dirty pages to disk
// 2. Writing a checkpoint record to the WAL
// 3. Optionally truncating the WAL to reclaim space
func (cm *CheckpointManager) Checkpoint() error {
	cm.mu.Lock()
	if cm.inProgress {
		cm.mu.Unlock()
		return ErrCheckpointInProgress
	}
	cm.inProgress = true
	cm.mu.Unlock()

	defer func() {
		cm.mu.Lock()
		cm.inProgress = false
		cm.mu.Unlock()
	}()

	// Flush all dirty pages from buffer pool
	if cm.bufferPool != nil {
		if err := cm.bufferPool.FlushAll(); err != nil {
			return err
		}
	}

	// Sync page manager to ensure all pages are on disk
	if err := cm.pageManager.Sync(); err != nil {
		return err
	}

	// Collect checkpoint data
	checkpointData := &CheckpointData{
		Timestamp: time.Now(),
		LastLSN:   cm.wal.CurrentLSN() - 1, // Current LSN is next to be assigned
	}

	// Get active transaction IDs if callback is set
	if cm.getActiveTxIDs != nil {
		checkpointData.ActiveTxIDs = cm.getActiveTxIDs()
	}

	// Get dirty page IDs from buffer pool
	if cm.bufferPool != nil {
		checkpointData.DirtyPageIDs = cm.bufferPool.GetDirtyPageIDs()
	}

	// Create checkpoint WAL record
	checkpointRecord := NewWALRecord(0, 0, WALCheckpoint)
	checkpointRecord.NewData = checkpointData.Serialize()

	// Write checkpoint record to WAL
	lsn, err := cm.wal.Append(checkpointRecord)
	if err != nil {
		return err
	}

	// Sync WAL to ensure checkpoint is durable
	if err := cm.wal.Sync(); err != nil {
		return err
	}

	// Update checkpoint state
	cm.mu.Lock()
	cm.lastCheckpointLSN = lsn
	cm.lastCheckpointTime = checkpointData.Timestamp
	cm.mu.Unlock()

	return nil
}

// TruncateWAL truncates the WAL up to the last checkpoint.
// This reclaims space by removing WAL records that are no longer needed.
func (cm *CheckpointManager) TruncateWAL() error {
	cm.mu.Lock()
	lastLSN := cm.lastCheckpointLSN
	cm.mu.Unlock()

	if lastLSN == 0 {
		return ErrNoActiveCheckpoint
	}

	return cm.wal.Truncate(lastLSN)
}

// ShouldCheckpoint returns true if a checkpoint should be taken.
// This is based on the time since the last checkpoint.
func (cm *CheckpointManager) ShouldCheckpoint() bool {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.lastCheckpointTime.IsZero() {
		return true
	}

	return time.Since(cm.lastCheckpointTime) >= cm.checkpointInterval
}

// LastCheckpointLSN returns the LSN of the last checkpoint.
func (cm *CheckpointManager) LastCheckpointLSN() uint64 {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return cm.lastCheckpointLSN
}

// LastCheckpointTime returns the time of the last checkpoint.
func (cm *CheckpointManager) LastCheckpointTime() time.Time {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return cm.lastCheckpointTime
}

// IsInProgress returns true if a checkpoint is currently running.
func (cm *CheckpointManager) IsInProgress() bool {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return cm.inProgress
}

// GetCheckpointInterval returns the current checkpoint interval.
func (cm *CheckpointManager) GetCheckpointInterval() time.Duration {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return cm.checkpointInterval
}

// ParseCheckpointRecord extracts checkpoint data from a WAL record.
func ParseCheckpointRecord(record *WALRecord) (*CheckpointData, error) {
	if record.Type != WALCheckpoint {
		return nil, ErrInvalidCheckpoint
	}

	if record.NewData == nil || len(record.NewData) == 0 {
		return nil, ErrInvalidCheckpoint
	}

	data := &CheckpointData{}
	if err := data.Deserialize(record.NewData); err != nil {
		return nil, err
	}

	return data, nil
}
