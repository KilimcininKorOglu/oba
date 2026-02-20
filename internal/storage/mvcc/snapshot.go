// Package mvcc provides Multi-Version Concurrency Control for ObaDB.
package mvcc

import (
	"errors"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/oba-ldap/oba/internal/storage/tx"
)

// Snapshot errors.
var (
	ErrSnapshotNotFound   = errors.New("snapshot not found")
	ErrSnapshotReleased   = errors.New("snapshot has been released")
	ErrInvalidSnapshot    = errors.New("invalid snapshot")
	ErrNilTxManager       = errors.New("transaction manager is nil")
	ErrSnapshotInUse      = errors.New("snapshot is still in use")
)

// Snapshot represents a consistent view of the database at a specific point in time.
// It captures the state of active transactions at the time of creation to determine
// version visibility.
type Snapshot struct {
	// Timestamp is the logical timestamp when this snapshot was created.
	// Versions with CommitTS <= Timestamp are potentially visible.
	Timestamp uint64

	// ActiveTxIDs contains the IDs of transactions that were active when
	// this snapshot was created. Versions created by these transactions
	// are not visible to this snapshot, even if they commit later.
	ActiveTxIDs []uint64

	// TxID is the transaction ID that owns this snapshot.
	// Used to allow a transaction to see its own uncommitted changes.
	TxID uint64

	// refCount tracks how many references exist to this snapshot.
	// The snapshot can only be released when refCount reaches 0.
	refCount int32

	// released indicates whether this snapshot has been released.
	released bool

	// mu protects concurrent access to the snapshot.
	mu sync.RWMutex
}

// NewSnapshot creates a new snapshot with the given timestamp and active transaction IDs.
func NewSnapshot(timestamp uint64, activeTxIDs []uint64, txID uint64) *Snapshot {
	// Make a copy of activeTxIDs to avoid external modifications
	activeIDs := make([]uint64, len(activeTxIDs))
	copy(activeIDs, activeTxIDs)

	// Sort for efficient binary search
	sort.Slice(activeIDs, func(i, j int) bool {
		return activeIDs[i] < activeIDs[j]
	})

	return &Snapshot{
		Timestamp:   timestamp,
		ActiveTxIDs: activeIDs,
		TxID:        txID,
		refCount:    1, // Start with 1 reference
		released:    false,
	}
}

// GetTimestamp returns the snapshot timestamp.
func (s *Snapshot) GetTimestamp() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Timestamp
}

// GetTxID returns the transaction ID that owns this snapshot.
func (s *Snapshot) GetTxID() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.TxID
}

// GetActiveTxIDs returns a copy of the active transaction IDs.
func (s *Snapshot) GetActiveTxIDs() []uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]uint64, len(s.ActiveTxIDs))
	copy(result, s.ActiveTxIDs)
	return result
}

// IsReleased returns true if the snapshot has been released.
func (s *Snapshot) IsReleased() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.released
}

// WasActiveAtSnapshot returns true if the given transaction ID was active
// when this snapshot was created.
func (s *Snapshot) WasActiveAtSnapshot(txID uint64) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Binary search for efficiency
	idx := sort.Search(len(s.ActiveTxIDs), func(i int) bool {
		return s.ActiveTxIDs[i] >= txID
	})

	return idx < len(s.ActiveTxIDs) && s.ActiveTxIDs[idx] == txID
}

// AddRef increments the reference count.
func (s *Snapshot) AddRef() {
	atomic.AddInt32(&s.refCount, 1)
}

// Release decrements the reference count and marks as released if count reaches 0.
// Returns true if the snapshot was actually released (refCount reached 0).
func (s *Snapshot) Release() bool {
	newCount := atomic.AddInt32(&s.refCount, -1)
	if newCount <= 0 {
		s.mu.Lock()
		s.released = true
		s.mu.Unlock()
		return true
	}
	return false
}

// RefCount returns the current reference count.
func (s *Snapshot) RefCount() int32 {
	return atomic.LoadInt32(&s.refCount)
}

// Clone creates a copy of the snapshot (for inspection purposes).
func (s *Snapshot) Clone() *Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	activeIDs := make([]uint64, len(s.ActiveTxIDs))
	copy(activeIDs, s.ActiveTxIDs)

	return &Snapshot{
		Timestamp:   s.Timestamp,
		ActiveTxIDs: activeIDs,
		TxID:        s.TxID,
		refCount:    1,
		released:    false,
	}
}

// SnapshotManager manages snapshots for MVCC.
// It tracks the current timestamp, active snapshots, and provides
// methods to create and release snapshots.
type SnapshotManager struct {
	// currentTS is the current logical timestamp (atomic).
	// Incremented for each new snapshot or commit.
	currentTS uint64

	// snapshots maps snapshot timestamps to active snapshots.
	snapshots map[uint64]*Snapshot

	// txManager provides access to active transactions.
	txManager *tx.TxManager

	// mu protects concurrent access to the snapshots map.
	mu sync.RWMutex
}

// NewSnapshotManager creates a new SnapshotManager with the given TxManager.
func NewSnapshotManager(txManager *tx.TxManager) *SnapshotManager {
	return &SnapshotManager{
		currentTS: 0,
		snapshots: make(map[uint64]*Snapshot),
		txManager: txManager,
	}
}

// CreateSnapshot creates a new snapshot for the given transaction.
// The snapshot captures the current timestamp and the list of active transactions.
func (sm *SnapshotManager) CreateSnapshot(txn *tx.Transaction) (*Snapshot, error) {
	if txn == nil {
		return nil, ErrNilTransaction
	}

	// Get the current timestamp and increment it atomically
	timestamp := atomic.AddUint64(&sm.currentTS, 1)

	// Get the list of active transaction IDs
	var activeTxIDs []uint64
	if sm.txManager != nil {
		activeTxs := sm.txManager.GetActiveTransactions()
		activeTxIDs = make([]uint64, 0, len(activeTxs))
		for _, activeTx := range activeTxs {
			// Don't include the current transaction in the active list
			if activeTx.ID != txn.ID {
				activeTxIDs = append(activeTxIDs, activeTx.ID)
			}
		}
	}

	// Create the snapshot
	snapshot := NewSnapshot(timestamp, activeTxIDs, txn.ID)

	// Register the snapshot
	sm.mu.Lock()
	sm.snapshots[timestamp] = snapshot
	sm.mu.Unlock()

	return snapshot, nil
}

// ReleaseSnapshot releases a snapshot, allowing garbage collection of
// old versions that are no longer visible to any active snapshot.
func (sm *SnapshotManager) ReleaseSnapshot(snapshot *Snapshot) error {
	if snapshot == nil {
		return ErrInvalidSnapshot
	}

	if snapshot.IsReleased() {
		return ErrSnapshotReleased
	}

	// Decrement reference count
	if snapshot.Release() {
		// Remove from active snapshots if fully released
		sm.mu.Lock()
		delete(sm.snapshots, snapshot.Timestamp)
		sm.mu.Unlock()
	}

	return nil
}

// GetSnapshot returns the snapshot with the given timestamp.
func (sm *SnapshotManager) GetSnapshot(timestamp uint64) *Snapshot {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.snapshots[timestamp]
}

// GetOldestActiveSnapshot returns the timestamp of the oldest active snapshot.
// This is used to determine which old versions can be garbage collected.
// Returns 0 if there are no active snapshots.
func (sm *SnapshotManager) GetOldestActiveSnapshot() uint64 {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if len(sm.snapshots) == 0 {
		return 0
	}

	var oldest uint64 = ^uint64(0) // Max uint64
	for ts, snapshot := range sm.snapshots {
		if !snapshot.IsReleased() && ts < oldest {
			oldest = ts
		}
	}

	if oldest == ^uint64(0) {
		return 0
	}

	return oldest
}

// ActiveSnapshotCount returns the number of active (non-released) snapshots.
func (sm *SnapshotManager) ActiveSnapshotCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	count := 0
	for _, snapshot := range sm.snapshots {
		if !snapshot.IsReleased() {
			count++
		}
	}
	return count
}

// CurrentTimestamp returns the current logical timestamp.
func (sm *SnapshotManager) CurrentTimestamp() uint64 {
	return atomic.LoadUint64(&sm.currentTS)
}

// AdvanceTimestamp advances the current timestamp and returns the new value.
// This is typically called when a transaction commits.
func (sm *SnapshotManager) AdvanceTimestamp() uint64 {
	return atomic.AddUint64(&sm.currentTS, 1)
}

// SetTimestamp sets the current timestamp (used for recovery).
func (sm *SnapshotManager) SetTimestamp(ts uint64) {
	atomic.StoreUint64(&sm.currentTS, ts)
}

// CleanupReleasedSnapshots removes all released snapshots from the map.
// Returns the number of snapshots removed.
func (sm *SnapshotManager) CleanupReleasedSnapshots() int {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	removed := 0
	for ts, snapshot := range sm.snapshots {
		if snapshot.IsReleased() {
			delete(sm.snapshots, ts)
			removed++
		}
	}
	return removed
}

// GetAllActiveSnapshots returns a list of all active (non-released) snapshots.
func (sm *SnapshotManager) GetAllActiveSnapshots() []*Snapshot {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	result := make([]*Snapshot, 0, len(sm.snapshots))
	for _, snapshot := range sm.snapshots {
		if !snapshot.IsReleased() {
			result = append(result, snapshot.Clone())
		}
	}
	return result
}

// Stats returns statistics about the snapshot manager.
type SnapshotManagerStats struct {
	CurrentTimestamp      uint64
	TotalSnapshots        int
	ActiveSnapshots       int
	ReleasedSnapshots     int
	OldestActiveTimestamp uint64
}

// Stats returns current statistics about the snapshot manager.
func (sm *SnapshotManager) Stats() SnapshotManagerStats {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	stats := SnapshotManagerStats{
		CurrentTimestamp: atomic.LoadUint64(&sm.currentTS),
		TotalSnapshots:   len(sm.snapshots),
	}

	var oldest uint64 = ^uint64(0)
	for ts, snapshot := range sm.snapshots {
		if snapshot.IsReleased() {
			stats.ReleasedSnapshots++
		} else {
			stats.ActiveSnapshots++
			if ts < oldest {
				oldest = ts
			}
		}
	}

	if oldest != ^uint64(0) {
		stats.OldestActiveTimestamp = oldest
	}

	return stats
}
