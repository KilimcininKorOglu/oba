// Package mvcc provides Multi-Version Concurrency Control for ObaDB.
package mvcc

// VisibilityChecker provides methods to determine version visibility
// based on snapshot isolation rules.
type VisibilityChecker struct {
	// snapshotManager provides access to snapshot information.
	snapshotManager *SnapshotManager
}

// NewVisibilityChecker creates a new VisibilityChecker with the given SnapshotManager.
func NewVisibilityChecker(sm *SnapshotManager) *VisibilityChecker {
	return &VisibilityChecker{
		snapshotManager: sm,
	}
}

// IsVisible determines if a version is visible to the given snapshot.
// This implements the core snapshot isolation visibility rules:
//
// 1. Uncommitted version from another transaction: NOT visible
//   - If CommitTS == 0 and TxID != snapshot.TxID, the version is not visible
//
// 2. Committed after snapshot: NOT visible
//   - If CommitTS > snapshot.Timestamp, the version is not visible
//
// 3. Committed by transaction that was active at snapshot time: NOT visible
//   - If the version's TxID is in snapshot.ActiveTxIDs, the version is not visible
//   - This prevents seeing changes from transactions that started before the snapshot
//     but committed after
//
// 4. Otherwise: VISIBLE
//   - The version was committed before the snapshot was taken and by a transaction
//     that was not active at snapshot time
func (vc *VisibilityChecker) IsVisible(version *Version, snapshot *Snapshot) bool {
	if version == nil || snapshot == nil {
		return false
	}

	return IsVersionVisible(version, snapshot)
}

// IsVersionVisible is a standalone function that determines version visibility.
// This can be used without a VisibilityChecker instance.
func IsVersionVisible(version *Version, snapshot *Snapshot) bool {
	if version == nil || snapshot == nil {
		return false
	}

	version.mu.RLock()
	txID := version.TxID
	commitTS := version.CommitTS
	version.mu.RUnlock()

	// Rule 1: Uncommitted version from another transaction is not visible
	if commitTS == 0 {
		// Uncommitted - only visible to the creating transaction
		return txID == snapshot.TxID
	}

	// Rule 2: Committed after snapshot is not visible
	if commitTS > snapshot.Timestamp {
		return false
	}

	// Rule 3: Committed by transaction that was active at snapshot time is not visible
	// This handles the case where a transaction started before our snapshot but
	// committed after - we should not see its changes
	if snapshot.WasActiveAtSnapshot(txID) {
		return false
	}

	// Rule 4: Version is visible
	return true
}

// FindVisibleVersion traverses a version chain and returns the first visible version.
// Returns nil if no visible version is found.
func (vc *VisibilityChecker) FindVisibleVersion(head *Version, snapshot *Snapshot) *Version {
	return FindVisibleVersionInChain(head, snapshot)
}

// FindVisibleVersionInChain is a standalone function that finds the visible version
// in a version chain. This can be used without a VisibilityChecker instance.
func FindVisibleVersionInChain(head *Version, snapshot *Snapshot) *Version {
	if head == nil || snapshot == nil {
		return nil
	}

	current := head
	for current != nil {
		if IsVersionVisible(current, snapshot) {
			return current
		}
		current = current.GetPrev()
	}

	return nil
}

// GetVisibleData returns the data from the visible version in the chain.
// Returns nil if no visible version is found or if the visible version is deleted.
func (vc *VisibilityChecker) GetVisibleData(head *Version, snapshot *Snapshot) ([]byte, error) {
	visible := vc.FindVisibleVersion(head, snapshot)
	if visible == nil {
		return nil, ErrNoVisibleVersion
	}

	if visible.IsDeleted() {
		return nil, ErrVersionDeleted
	}

	return visible.GetData(), nil
}

// CanSeeVersion is a convenience method that checks if a specific version
// is visible to a snapshot.
func (vc *VisibilityChecker) CanSeeVersion(version *Version, snapshot *Snapshot) bool {
	return vc.IsVisible(version, snapshot)
}

// VisibilityResult contains the result of a visibility check with additional context.
type VisibilityResult struct {
	// Visible indicates whether the version is visible.
	Visible bool

	// Reason provides a human-readable explanation for the visibility decision.
	Reason string

	// Version is the version that was checked (may be nil).
	Version *Version
}

// CheckVisibilityWithReason performs a visibility check and returns detailed results.
// This is useful for debugging and understanding visibility decisions.
func CheckVisibilityWithReason(version *Version, snapshot *Snapshot) VisibilityResult {
	if version == nil {
		return VisibilityResult{
			Visible: false,
			Reason:  "version is nil",
			Version: nil,
		}
	}

	if snapshot == nil {
		return VisibilityResult{
			Visible: false,
			Reason:  "snapshot is nil",
			Version: version,
		}
	}

	version.mu.RLock()
	txID := version.TxID
	commitTS := version.CommitTS
	version.mu.RUnlock()

	// Rule 1: Uncommitted version
	if commitTS == 0 {
		if txID == snapshot.TxID {
			return VisibilityResult{
				Visible: true,
				Reason:  "uncommitted version visible to creating transaction",
				Version: version,
			}
		}
		return VisibilityResult{
			Visible: false,
			Reason:  "uncommitted version from another transaction",
			Version: version,
		}
	}

	// Rule 2: Committed after snapshot
	if commitTS > snapshot.Timestamp {
		return VisibilityResult{
			Visible: false,
			Reason:  "committed after snapshot timestamp",
			Version: version,
		}
	}

	// Rule 3: Committed by active transaction
	if snapshot.WasActiveAtSnapshot(txID) {
		return VisibilityResult{
			Visible: false,
			Reason:  "committed by transaction that was active at snapshot time",
			Version: version,
		}
	}

	// Rule 4: Visible
	return VisibilityResult{
		Visible: true,
		Reason:  "committed before snapshot by completed transaction",
		Version: version,
	}
}

// VisibilityStats tracks visibility check statistics for monitoring.
type VisibilityStats struct {
	TotalChecks          uint64
	VisibleCount         uint64
	InvisibleUncommitted uint64
	InvisibleFuture      uint64
	InvisibleActiveTx    uint64
}

// TrackedVisibilityChecker wraps VisibilityChecker with statistics tracking.
type TrackedVisibilityChecker struct {
	*VisibilityChecker
	stats VisibilityStats
}

// NewTrackedVisibilityChecker creates a new TrackedVisibilityChecker.
func NewTrackedVisibilityChecker(sm *SnapshotManager) *TrackedVisibilityChecker {
	return &TrackedVisibilityChecker{
		VisibilityChecker: NewVisibilityChecker(sm),
		stats:             VisibilityStats{},
	}
}

// IsVisible checks visibility and tracks statistics.
func (tvc *TrackedVisibilityChecker) IsVisible(version *Version, snapshot *Snapshot) bool {
	tvc.stats.TotalChecks++

	result := CheckVisibilityWithReason(version, snapshot)

	if result.Visible {
		tvc.stats.VisibleCount++
	} else {
		switch result.Reason {
		case "uncommitted version from another transaction":
			tvc.stats.InvisibleUncommitted++
		case "committed after snapshot timestamp":
			tvc.stats.InvisibleFuture++
		case "committed by transaction that was active at snapshot time":
			tvc.stats.InvisibleActiveTx++
		}
	}

	return result.Visible
}

// GetStats returns the current visibility statistics.
func (tvc *TrackedVisibilityChecker) GetStats() VisibilityStats {
	return tvc.stats
}

// ResetStats resets the visibility statistics.
func (tvc *TrackedVisibilityChecker) ResetStats() {
	tvc.stats = VisibilityStats{}
}
