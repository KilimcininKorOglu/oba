// Package mvcc provides Multi-Version Concurrency Control for ObaDB.
package mvcc

import (
	"testing"

	"github.com/oba-ldap/oba/internal/storage"
)

// --- Visibility Tests ---

// TestIsVersionVisibleUncommitted tests visibility of uncommitted versions.
func TestIsVersionVisibleUncommitted(t *testing.T) {
	// Create an uncommitted version
	version := NewVersion(100, []byte("data"), storage.PageID(1), 0)
	// CommitTS is 0 (uncommitted)

	tests := []struct {
		name        string
		snapshotTxID uint64
		wantVisible bool
	}{
		{
			name:        "visible to creating transaction",
			snapshotTxID: 100,
			wantVisible: true,
		},
		{
			name:        "not visible to other transaction",
			snapshotTxID: 101,
			wantVisible: false,
		},
		{
			name:        "not visible to transaction 0",
			snapshotTxID: 0,
			wantVisible: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapshot := NewSnapshot(200, nil, tt.snapshotTxID)
			got := IsVersionVisible(version, snapshot)
			if got != tt.wantVisible {
				t.Errorf("IsVersionVisible() = %v, want %v", got, tt.wantVisible)
			}
		})
	}
}

// TestIsVersionVisibleCommittedBeforeSnapshot tests visibility of versions committed before snapshot.
func TestIsVersionVisibleCommittedBeforeSnapshot(t *testing.T) {
	version := NewVersion(100, []byte("data"), storage.PageID(1), 0)
	version.Commit(150) // Committed at timestamp 150

	tests := []struct {
		name            string
		snapshotTS      uint64
		activeTxIDs     []uint64
		wantVisible     bool
	}{
		{
			name:        "visible when snapshot after commit",
			snapshotTS:  200,
			activeTxIDs: nil,
			wantVisible: true,
		},
		{
			name:        "visible when snapshot equals commit",
			snapshotTS:  150,
			activeTxIDs: nil,
			wantVisible: true,
		},
		{
			name:        "not visible when snapshot before commit",
			snapshotTS:  100,
			activeTxIDs: nil,
			wantVisible: false,
		},
		{
			name:        "not visible when creating tx was active at snapshot",
			snapshotTS:  200,
			activeTxIDs: []uint64{100}, // Version's TxID was active
			wantVisible: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapshot := NewSnapshot(tt.snapshotTS, tt.activeTxIDs, 999)
			got := IsVersionVisible(version, snapshot)
			if got != tt.wantVisible {
				t.Errorf("IsVersionVisible() = %v, want %v", got, tt.wantVisible)
			}
		})
	}
}

// TestIsVersionVisibleNilInputs tests handling of nil inputs.
func TestIsVersionVisibleNilInputs(t *testing.T) {
	version := NewVersion(100, []byte("data"), storage.PageID(1), 0)
	snapshot := NewSnapshot(200, nil, 1)

	if IsVersionVisible(nil, snapshot) {
		t.Error("expected false for nil version")
	}
	if IsVersionVisible(version, nil) {
		t.Error("expected false for nil snapshot")
	}
	if IsVersionVisible(nil, nil) {
		t.Error("expected false for both nil")
	}
}

// TestFindVisibleVersionInChain tests finding visible version in a chain.
func TestFindVisibleVersionInChain(t *testing.T) {
	// Create a version chain: v3 -> v2 -> v1
	v1 := NewVersion(10, []byte("v1"), storage.PageID(1), 0)
	v1.Commit(100)

	v2 := NewVersion(20, []byte("v2"), storage.PageID(2), 0)
	v2.Commit(200)
	v2.SetPrev(v1)

	v3 := NewVersion(30, []byte("v3"), storage.PageID(3), 0)
	v3.Commit(300)
	v3.SetPrev(v2)

	tests := []struct {
		name         string
		snapshotTS   uint64
		expectedData string
	}{
		{
			name:         "snapshot at 350 sees v3",
			snapshotTS:   350,
			expectedData: "v3",
		},
		{
			name:         "snapshot at 300 sees v3",
			snapshotTS:   300,
			expectedData: "v3",
		},
		{
			name:         "snapshot at 250 sees v2",
			snapshotTS:   250,
			expectedData: "v2",
		},
		{
			name:         "snapshot at 200 sees v2",
			snapshotTS:   200,
			expectedData: "v2",
		},
		{
			name:         "snapshot at 150 sees v1",
			snapshotTS:   150,
			expectedData: "v1",
		},
		{
			name:         "snapshot at 100 sees v1",
			snapshotTS:   100,
			expectedData: "v1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapshot := NewSnapshot(tt.snapshotTS, nil, 999)
			visible := FindVisibleVersionInChain(v3, snapshot)
			if visible == nil {
				t.Fatal("expected to find a visible version")
			}
			if string(visible.Data) != tt.expectedData {
				t.Errorf("expected data %q, got %q", tt.expectedData, visible.Data)
			}
		})
	}
}

// TestFindVisibleVersionInChainNoVisible tests when no version is visible.
func TestFindVisibleVersionInChainNoVisible(t *testing.T) {
	v1 := NewVersion(10, []byte("v1"), storage.PageID(1), 0)
	v1.Commit(100)

	// Snapshot before any version was committed
	snapshot := NewSnapshot(50, nil, 999)
	visible := FindVisibleVersionInChain(v1, snapshot)
	if visible != nil {
		t.Error("expected no visible version")
	}
}

// TestFindVisibleVersionInChainNilInputs tests nil handling.
func TestFindVisibleVersionInChainNilInputs(t *testing.T) {
	v1 := NewVersion(10, []byte("v1"), storage.PageID(1), 0)
	snapshot := NewSnapshot(200, nil, 1)

	if FindVisibleVersionInChain(nil, snapshot) != nil {
		t.Error("expected nil for nil head")
	}
	if FindVisibleVersionInChain(v1, nil) != nil {
		t.Error("expected nil for nil snapshot")
	}
}

// TestVisibilityCheckerIsVisible tests VisibilityChecker.IsVisible.
func TestVisibilityCheckerIsVisible(t *testing.T) {
	vc := NewVisibilityChecker(nil)

	version := NewVersion(100, []byte("data"), storage.PageID(1), 0)
	version.Commit(150)

	snapshot := NewSnapshot(200, nil, 999)

	if !vc.IsVisible(version, snapshot) {
		t.Error("expected version to be visible")
	}

	// Test with nil inputs
	if vc.IsVisible(nil, snapshot) {
		t.Error("expected false for nil version")
	}
	if vc.IsVisible(version, nil) {
		t.Error("expected false for nil snapshot")
	}
}

// TestVisibilityCheckerFindVisibleVersion tests VisibilityChecker.FindVisibleVersion.
func TestVisibilityCheckerFindVisibleVersion(t *testing.T) {
	vc := NewVisibilityChecker(nil)

	v1 := NewVersion(10, []byte("v1"), storage.PageID(1), 0)
	v1.Commit(100)

	v2 := NewVersion(20, []byte("v2"), storage.PageID(2), 0)
	v2.Commit(200)
	v2.SetPrev(v1)

	snapshot := NewSnapshot(150, nil, 999)
	visible := vc.FindVisibleVersion(v2, snapshot)

	if visible == nil {
		t.Fatal("expected to find visible version")
	}
	if string(visible.Data) != "v1" {
		t.Errorf("expected v1, got %q", visible.Data)
	}
}

// TestVisibilityCheckerGetVisibleData tests VisibilityChecker.GetVisibleData.
func TestVisibilityCheckerGetVisibleData(t *testing.T) {
	vc := NewVisibilityChecker(nil)

	v1 := NewVersion(10, []byte("test data"), storage.PageID(1), 0)
	v1.Commit(100)

	snapshot := NewSnapshot(200, nil, 999)

	data, err := vc.GetVisibleData(v1, snapshot)
	if err != nil {
		t.Fatalf("GetVisibleData failed: %v", err)
	}
	if string(data) != "test data" {
		t.Errorf("expected 'test data', got %q", data)
	}
}

// TestVisibilityCheckerGetVisibleDataDeleted tests GetVisibleData with deleted version.
func TestVisibilityCheckerGetVisibleDataDeleted(t *testing.T) {
	vc := NewVisibilityChecker(nil)

	v1 := NewDeletedVersion(10, storage.PageID(1), 0)
	v1.Commit(100)

	snapshot := NewSnapshot(200, nil, 999)

	_, err := vc.GetVisibleData(v1, snapshot)
	if err != ErrVersionDeleted {
		t.Errorf("expected ErrVersionDeleted, got %v", err)
	}
}

// TestVisibilityCheckerGetVisibleDataNoVisible tests GetVisibleData with no visible version.
func TestVisibilityCheckerGetVisibleDataNoVisible(t *testing.T) {
	vc := NewVisibilityChecker(nil)

	v1 := NewVersion(10, []byte("data"), storage.PageID(1), 0)
	v1.Commit(100)

	snapshot := NewSnapshot(50, nil, 999) // Before commit

	_, err := vc.GetVisibleData(v1, snapshot)
	if err != ErrNoVisibleVersion {
		t.Errorf("expected ErrNoVisibleVersion, got %v", err)
	}
}

// TestCheckVisibilityWithReason tests detailed visibility checking.
func TestCheckVisibilityWithReason(t *testing.T) {
	tests := []struct {
		name           string
		setupVersion   func() *Version
		snapshotTS     uint64
		snapshotTxID   uint64
		activeTxIDs    []uint64
		wantVisible    bool
		wantReasonContains string
	}{
		{
			name: "nil version",
			setupVersion: func() *Version {
				return nil
			},
			snapshotTS:         200,
			snapshotTxID:       999,
			wantVisible:        false,
			wantReasonContains: "version is nil",
		},
		{
			name: "uncommitted visible to creator",
			setupVersion: func() *Version {
				return NewVersion(100, []byte("data"), storage.PageID(1), 0)
			},
			snapshotTS:         200,
			snapshotTxID:       100,
			wantVisible:        true,
			wantReasonContains: "uncommitted version visible to creating transaction",
		},
		{
			name: "uncommitted not visible to others",
			setupVersion: func() *Version {
				return NewVersion(100, []byte("data"), storage.PageID(1), 0)
			},
			snapshotTS:         200,
			snapshotTxID:       101,
			wantVisible:        false,
			wantReasonContains: "uncommitted version from another transaction",
		},
		{
			name: "committed after snapshot",
			setupVersion: func() *Version {
				v := NewVersion(100, []byte("data"), storage.PageID(1), 0)
				v.Commit(300)
				return v
			},
			snapshotTS:         200,
			snapshotTxID:       999,
			wantVisible:        false,
			wantReasonContains: "committed after snapshot timestamp",
		},
		{
			name: "committed by active transaction",
			setupVersion: func() *Version {
				v := NewVersion(100, []byte("data"), storage.PageID(1), 0)
				v.Commit(150)
				return v
			},
			snapshotTS:         200,
			snapshotTxID:       999,
			activeTxIDs:        []uint64{100},
			wantVisible:        false,
			wantReasonContains: "committed by transaction that was active at snapshot time",
		},
		{
			name: "committed before snapshot by completed transaction",
			setupVersion: func() *Version {
				v := NewVersion(100, []byte("data"), storage.PageID(1), 0)
				v.Commit(150)
				return v
			},
			snapshotTS:         200,
			snapshotTxID:       999,
			wantVisible:        true,
			wantReasonContains: "committed before snapshot by completed transaction",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version := tt.setupVersion()
			var snapshot *Snapshot
			if tt.name != "nil version" {
				snapshot = NewSnapshot(tt.snapshotTS, tt.activeTxIDs, tt.snapshotTxID)
			}

			result := CheckVisibilityWithReason(version, snapshot)

			if result.Visible != tt.wantVisible {
				t.Errorf("Visible = %v, want %v", result.Visible, tt.wantVisible)
			}
			if result.Reason != tt.wantReasonContains {
				t.Errorf("Reason = %q, want %q", result.Reason, tt.wantReasonContains)
			}
		})
	}
}

// TestCheckVisibilityWithReasonNilSnapshot tests nil snapshot handling.
func TestCheckVisibilityWithReasonNilSnapshot(t *testing.T) {
	version := NewVersion(100, []byte("data"), storage.PageID(1), 0)
	result := CheckVisibilityWithReason(version, nil)

	if result.Visible {
		t.Error("expected not visible for nil snapshot")
	}
	if result.Reason != "snapshot is nil" {
		t.Errorf("expected 'snapshot is nil', got %q", result.Reason)
	}
}

// TestTrackedVisibilityChecker tests visibility tracking with statistics.
func TestTrackedVisibilityChecker(t *testing.T) {
	tvc := NewTrackedVisibilityChecker(nil)

	// Create versions with different visibility scenarios
	uncommittedOther := NewVersion(100, []byte("data"), storage.PageID(1), 0)
	committedFuture := NewVersion(200, []byte("data"), storage.PageID(2), 0)
	committedFuture.Commit(300)
	committedVisible := NewVersion(300, []byte("data"), storage.PageID(3), 0)
	committedVisible.Commit(100)

	snapshot := NewSnapshot(200, []uint64{400}, 999)

	// Check uncommitted from other transaction
	tvc.IsVisible(uncommittedOther, snapshot)

	// Check committed in future
	tvc.IsVisible(committedFuture, snapshot)

	// Check visible version
	tvc.IsVisible(committedVisible, snapshot)

	// Create version committed by active tx
	committedByActive := NewVersion(400, []byte("data"), storage.PageID(4), 0)
	committedByActive.Commit(150)
	tvc.IsVisible(committedByActive, snapshot)

	stats := tvc.GetStats()

	if stats.TotalChecks != 4 {
		t.Errorf("expected TotalChecks 4, got %d", stats.TotalChecks)
	}
	if stats.VisibleCount != 1 {
		t.Errorf("expected VisibleCount 1, got %d", stats.VisibleCount)
	}
	if stats.InvisibleUncommitted != 1 {
		t.Errorf("expected InvisibleUncommitted 1, got %d", stats.InvisibleUncommitted)
	}
	if stats.InvisibleFuture != 1 {
		t.Errorf("expected InvisibleFuture 1, got %d", stats.InvisibleFuture)
	}
	if stats.InvisibleActiveTx != 1 {
		t.Errorf("expected InvisibleActiveTx 1, got %d", stats.InvisibleActiveTx)
	}

	// Test reset
	tvc.ResetStats()
	stats = tvc.GetStats()
	if stats.TotalChecks != 0 {
		t.Error("expected TotalChecks 0 after reset")
	}
}

// TestVisibilityWithDeletedVersions tests visibility with deleted versions.
func TestVisibilityWithDeletedVersions(t *testing.T) {
	// Create a chain: delete -> v1
	v1 := NewVersion(10, []byte("v1"), storage.PageID(1), 0)
	v1.Commit(100)

	deleteVersion := NewDeletedVersion(20, storage.PageID(1), 0)
	deleteVersion.Commit(200)
	deleteVersion.SetPrev(v1)

	// Snapshot after delete should see the delete
	snapshot := NewSnapshot(250, nil, 999)
	visible := FindVisibleVersionInChain(deleteVersion, snapshot)
	if visible == nil {
		t.Fatal("expected to find visible version")
	}
	if !visible.IsDeleted() {
		t.Error("expected visible version to be deleted")
	}

	// Snapshot before delete should see v1
	snapshotBefore := NewSnapshot(150, nil, 999)
	visibleBefore := FindVisibleVersionInChain(deleteVersion, snapshotBefore)
	if visibleBefore == nil {
		t.Fatal("expected to find visible version before delete")
	}
	if visibleBefore.IsDeleted() {
		t.Error("expected visible version to not be deleted")
	}
	if string(visibleBefore.Data) != "v1" {
		t.Errorf("expected v1, got %q", visibleBefore.Data)
	}
}

// TestSnapshotIsolationScenario tests a complete snapshot isolation scenario.
func TestSnapshotIsolationScenario(t *testing.T) {
	// Scenario:
	// 1. Tx1 starts at time 100
	// 2. Tx2 starts at time 110
	// 3. Tx1 creates version V1 and commits at time 120
	// 4. Tx2 should NOT see V1 (because Tx1 was active when Tx2 started)
	// 5. Tx3 starts at time 130
	// 6. Tx3 SHOULD see V1 (because Tx1 was not active when Tx3 started)

	// Create V1 committed by Tx1
	v1 := NewVersion(1, []byte("V1 data"), storage.PageID(1), 0)
	v1.Commit(120)

	// Tx2's snapshot: started at 110, Tx1 (ID=1) was active
	snapshotTx2 := NewSnapshot(110, []uint64{1}, 2)

	// Tx2 should NOT see V1
	if IsVersionVisible(v1, snapshotTx2) {
		t.Error("Tx2 should NOT see V1 (Tx1 was active at snapshot time)")
	}

	// Tx3's snapshot: started at 130, no active transactions
	snapshotTx3 := NewSnapshot(130, nil, 3)

	// Tx3 SHOULD see V1
	if !IsVersionVisible(v1, snapshotTx3) {
		t.Error("Tx3 SHOULD see V1 (Tx1 was not active at snapshot time)")
	}
}

// TestPhantomReadPrevention tests that phantom reads are prevented.
func TestPhantomReadPrevention(t *testing.T) {
	// Scenario:
	// 1. Tx1 starts and takes a snapshot
	// 2. Tx2 inserts a new row and commits
	// 3. Tx1 should NOT see the new row (phantom read prevention)

	// Tx1's snapshot at time 100
	snapshotTx1 := NewSnapshot(100, nil, 1)

	// Tx2 creates a new version and commits at time 150
	newVersion := NewVersion(2, []byte("new data"), storage.PageID(1), 0)
	newVersion.Commit(150)

	// Tx1 should NOT see the new version (committed after snapshot)
	if IsVersionVisible(newVersion, snapshotTx1) {
		t.Error("Tx1 should NOT see version committed after its snapshot (phantom read)")
	}
}

// TestNonRepeatableReadPrevention tests that non-repeatable reads are prevented.
func TestNonRepeatableReadPrevention(t *testing.T) {
	// Scenario:
	// 1. V1 exists (committed at time 50)
	// 2. Tx1 starts and takes a snapshot at time 100
	// 3. Tx2 updates the row (creates V2) and commits at time 150
	// 4. Tx1 should still see V1, not V2 (non-repeatable read prevention)

	v1 := NewVersion(10, []byte("original"), storage.PageID(1), 0)
	v1.Commit(50)

	v2 := NewVersion(20, []byte("updated"), storage.PageID(1), 0)
	v2.Commit(150)
	v2.SetPrev(v1)

	// Tx1's snapshot at time 100
	snapshotTx1 := NewSnapshot(100, nil, 1)

	// Tx1 should see V1, not V2
	visible := FindVisibleVersionInChain(v2, snapshotTx1)
	if visible == nil {
		t.Fatal("expected to find visible version")
	}
	if string(visible.Data) != "original" {
		t.Errorf("expected 'original', got %q (non-repeatable read)", visible.Data)
	}
}

// TestDirtyReadPrevention tests that dirty reads are prevented.
func TestDirtyReadPrevention(t *testing.T) {
	// Scenario:
	// 1. Tx1 creates an uncommitted version
	// 2. Tx2 should NOT see the uncommitted version (dirty read prevention)

	uncommittedVersion := NewVersion(1, []byte("uncommitted data"), storage.PageID(1), 0)
	// CommitTS is 0 (uncommitted)

	// Tx2's snapshot
	snapshotTx2 := NewSnapshot(200, nil, 2)

	// Tx2 should NOT see the uncommitted version
	if IsVersionVisible(uncommittedVersion, snapshotTx2) {
		t.Error("Tx2 should NOT see uncommitted version (dirty read)")
	}
}
