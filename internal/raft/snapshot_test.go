package raft

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestSnapshotStore(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSnapshotStore(dir)
	if err != nil {
		t.Fatalf("NewSnapshotStore failed: %v", err)
	}

	// Initially no snapshot
	snap, err := store.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if snap != nil {
		t.Error("Expected no snapshot initially")
	}

	// Save a snapshot
	testSnap := &Snapshot{
		LastIncludedIndex: 100,
		LastIncludedTerm:  5,
		Data:              []byte("test snapshot data"),
	}

	if err := store.Save(testSnap); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Load it back
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.LastIncludedIndex != testSnap.LastIncludedIndex {
		t.Errorf("LastIncludedIndex mismatch")
	}
	if loaded.LastIncludedTerm != testSnap.LastIncludedTerm {
		t.Errorf("LastIncludedTerm mismatch")
	}
	if !bytes.Equal(loaded.Data, testSnap.Data) {
		t.Errorf("Data mismatch")
	}
}

func TestSnapshotStoreMeta(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewSnapshotStore(dir)

	// No meta initially
	meta, err := store.GetMeta()
	if err != nil {
		t.Fatalf("GetMeta failed: %v", err)
	}
	if meta != nil {
		t.Error("Expected no meta initially")
	}

	// Save snapshot
	snap := &Snapshot{
		LastIncludedIndex: 50,
		LastIncludedTerm:  3,
		Data:              []byte("data"),
	}
	store.Save(snap)

	// Check meta
	meta, err = store.GetMeta()
	if err != nil {
		t.Fatalf("GetMeta failed: %v", err)
	}
	if meta.LastIncludedIndex != 50 {
		t.Errorf("LastIncludedIndex mismatch")
	}
	if meta.LastIncludedTerm != 3 {
		t.Errorf("LastIncludedTerm mismatch")
	}
	if meta.Size != 4 {
		t.Errorf("Size mismatch: got %d", meta.Size)
	}
}

func TestSnapshotStoreDelete(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewSnapshotStore(dir)

	snap := &Snapshot{
		LastIncludedIndex: 100,
		LastIncludedTerm:  5,
		Data:              []byte("data"),
	}
	store.Save(snap)

	// File should exist
	filename := filepath.Join(dir, "snapshot-100-5.snap")
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		t.Error("Snapshot file should exist")
	}

	// Delete
	if err := store.Delete(100, 5); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// File should not exist
	if _, err := os.Stat(filename); !os.IsNotExist(err) {
		t.Error("Snapshot file should be deleted")
	}
}

func TestSnapshotStoreEmptyData(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewSnapshotStore(dir)

	snap := &Snapshot{
		LastIncludedIndex: 10,
		LastIncludedTerm:  1,
		Data:              nil,
	}

	if err := store.Save(snap); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(loaded.Data) != 0 {
		t.Error("Data should be empty")
	}
}

func TestSnapshotter(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewSnapshotStore(dir)
	sm := NewMockStateMachine()
	sm.snapshot = []byte("state machine data")

	snapshotter := NewSnapshotter(store, sm, nil)

	// Create snapshot
	snap, err := snapshotter.CreateSnapshot(100, 5)
	if err != nil {
		t.Fatalf("CreateSnapshot failed: %v", err)
	}

	if snap.LastIncludedIndex != 100 {
		t.Errorf("LastIncludedIndex mismatch")
	}
	if !bytes.Equal(snap.Data, []byte("state machine data")) {
		t.Errorf("Data mismatch")
	}

	// Load latest
	loaded, err := snapshotter.LoadLatest()
	if err != nil {
		t.Fatalf("LoadLatest failed: %v", err)
	}
	if loaded.LastIncludedIndex != 100 {
		t.Errorf("Loaded snapshot mismatch")
	}
}

func TestSnapshotterRestore(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewSnapshotStore(dir)
	sm := NewMockStateMachine()

	snapshotter := NewSnapshotter(store, sm, nil)

	snap := &Snapshot{
		LastIncludedIndex: 50,
		LastIncludedTerm:  3,
		Data:              []byte("restored data"),
	}

	if err := snapshotter.RestoreSnapshot(snap); err != nil {
		t.Fatalf("RestoreSnapshot failed: %v", err)
	}

	if !bytes.Equal(sm.snapshot, []byte("restored data")) {
		t.Errorf("State machine not restored correctly")
	}
}

func TestSnapshotterShouldSnapshot(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewSnapshotStore(dir)
	sm := NewMockStateMachine()

	policy := &SnapshotPolicy{
		LogSizeThreshold: 100,
	}
	snapshotter := NewSnapshotter(store, sm, policy)

	if snapshotter.ShouldSnapshot(50) {
		t.Error("Should not snapshot at 50 entries")
	}
	if !snapshotter.ShouldSnapshot(100) {
		t.Error("Should snapshot at 100 entries")
	}
	if !snapshotter.ShouldSnapshot(150) {
		t.Error("Should snapshot at 150 entries")
	}
}

func TestSnapshotterDisabledPolicy(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewSnapshotStore(dir)
	sm := NewMockStateMachine()

	policy := &SnapshotPolicy{
		LogSizeThreshold: 0, // Disabled
	}
	snapshotter := NewSnapshotter(store, sm, policy)

	if snapshotter.ShouldSnapshot(1000000) {
		t.Error("Should never snapshot when disabled")
	}
}

func TestItoa(t *testing.T) {
	tests := []struct {
		input uint64
		want  string
	}{
		{0, "0"},
		{1, "1"},
		{10, "10"},
		{100, "100"},
		{12345, "12345"},
		{18446744073709551615, "18446744073709551615"},
	}

	for _, tt := range tests {
		got := itoa(tt.input)
		if got != tt.want {
			t.Errorf("itoa(%d) = %s, want %s", tt.input, got, tt.want)
		}
	}
}
