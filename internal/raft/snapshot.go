package raft

import (
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// Snapshot represents a point-in-time snapshot of the state machine.
type Snapshot struct {
	// Metadata
	LastIncludedIndex uint64
	LastIncludedTerm  uint64

	// Data
	Data []byte
}

// SnapshotMeta contains snapshot metadata without the data.
type SnapshotMeta struct {
	LastIncludedIndex uint64
	LastIncludedTerm  uint64
	Size              int64
}

// SnapshotStore manages snapshot persistence.
type SnapshotStore struct {
	dir string
	mu  sync.RWMutex
}

// NewSnapshotStore creates a new snapshot store.
func NewSnapshotStore(dir string) (*SnapshotStore, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	return &SnapshotStore{dir: dir}, nil
}

// snapshotFilename returns the filename for a snapshot.
func (s *SnapshotStore) snapshotFilename(index, term uint64) string {
	return filepath.Join(s.dir, "snapshot-"+itoa(index)+"-"+itoa(term)+".snap")
}

// metaFilename returns the filename for snapshot metadata.
func (s *SnapshotStore) metaFilename() string {
	return filepath.Join(s.dir, "snapshot.meta")
}

// Save saves a snapshot to disk.
func (s *SnapshotStore) Save(snap *Snapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	filename := s.snapshotFilename(snap.LastIncludedIndex, snap.LastIncludedTerm)

	// Write snapshot data
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	// Write header: [index:8][term:8][dataLen:8]
	header := make([]byte, 24)
	binary.LittleEndian.PutUint64(header[0:8], snap.LastIncludedIndex)
	binary.LittleEndian.PutUint64(header[8:16], snap.LastIncludedTerm)
	binary.LittleEndian.PutUint64(header[16:24], uint64(len(snap.Data)))

	if _, err := f.Write(header); err != nil {
		return err
	}
	if _, err := f.Write(snap.Data); err != nil {
		return err
	}

	// Sync to disk
	if err := f.Sync(); err != nil {
		return err
	}

	// Update metadata file
	return s.saveMeta(&SnapshotMeta{
		LastIncludedIndex: snap.LastIncludedIndex,
		LastIncludedTerm:  snap.LastIncludedTerm,
		Size:              int64(len(snap.Data)),
	})
}

func (s *SnapshotStore) saveMeta(meta *SnapshotMeta) error {
	f, err := os.Create(s.metaFilename())
	if err != nil {
		return err
	}
	defer f.Close()

	data := make([]byte, 24)
	binary.LittleEndian.PutUint64(data[0:8], meta.LastIncludedIndex)
	binary.LittleEndian.PutUint64(data[8:16], meta.LastIncludedTerm)
	binary.LittleEndian.PutUint64(data[16:24], uint64(meta.Size))

	if _, err := f.Write(data); err != nil {
		return err
	}
	return f.Sync()
}

// Load loads the latest snapshot from disk.
func (s *SnapshotStore) Load() (*Snapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	meta, err := s.loadMeta()
	if err != nil {
		return nil, err
	}
	if meta == nil {
		return nil, nil // No snapshot
	}

	filename := s.snapshotFilename(meta.LastIncludedIndex, meta.LastIncludedTerm)
	return s.loadFromFile(filename)
}

func (s *SnapshotStore) loadMeta() (*SnapshotMeta, error) {
	f, err := os.Open(s.metaFilename())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	data := make([]byte, 24)
	if _, err := io.ReadFull(f, data); err != nil {
		return nil, err
	}

	return &SnapshotMeta{
		LastIncludedIndex: binary.LittleEndian.Uint64(data[0:8]),
		LastIncludedTerm:  binary.LittleEndian.Uint64(data[8:16]),
		Size:              int64(binary.LittleEndian.Uint64(data[16:24])),
	}, nil
}

func (s *SnapshotStore) loadFromFile(filename string) (*Snapshot, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Read header
	header := make([]byte, 24)
	if _, err := io.ReadFull(f, header); err != nil {
		return nil, err
	}

	snap := &Snapshot{
		LastIncludedIndex: binary.LittleEndian.Uint64(header[0:8]),
		LastIncludedTerm:  binary.LittleEndian.Uint64(header[8:16]),
	}

	dataLen := binary.LittleEndian.Uint64(header[16:24])
	snap.Data = make([]byte, dataLen)
	if _, err := io.ReadFull(f, snap.Data); err != nil {
		return nil, err
	}

	return snap, nil
}

// GetMeta returns the metadata of the latest snapshot.
func (s *SnapshotStore) GetMeta() (*SnapshotMeta, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.loadMeta()
}

// Delete removes old snapshots, keeping only the latest.
func (s *SnapshotStore) Delete(index, term uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	filename := s.snapshotFilename(index, term)
	return os.Remove(filename)
}

// itoa converts uint64 to string without fmt package.
func itoa(n uint64) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// SnapshotPolicy defines when to take snapshots.
type SnapshotPolicy struct {
	// LogSizeThreshold triggers snapshot when log exceeds this size
	LogSizeThreshold uint64
	// Interval between automatic snapshots (0 = disabled)
	Interval uint64
}

// DefaultSnapshotPolicy returns default snapshot policy.
func DefaultSnapshotPolicy() *SnapshotPolicy {
	return &SnapshotPolicy{
		LogSizeThreshold: 10000, // Snapshot after 10000 log entries
		Interval:         0,     // Disabled by default
	}
}

// Snapshotter handles snapshot creation and restoration.
type Snapshotter struct {
	store        *SnapshotStore
	stateMachine StateMachine
	policy       *SnapshotPolicy
	mu           sync.Mutex
}

// NewSnapshotter creates a new snapshotter.
func NewSnapshotter(store *SnapshotStore, sm StateMachine, policy *SnapshotPolicy) *Snapshotter {
	if policy == nil {
		policy = DefaultSnapshotPolicy()
	}
	return &Snapshotter{
		store:        store,
		stateMachine: sm,
		policy:       policy,
	}
}

// ShouldSnapshot returns true if a snapshot should be taken.
func (s *Snapshotter) ShouldSnapshot(logSize uint64) bool {
	if s.policy.LogSizeThreshold == 0 {
		return false
	}
	return logSize >= s.policy.LogSizeThreshold
}

// CreateSnapshot creates a new snapshot from the state machine.
func (s *Snapshotter) CreateSnapshot(lastIndex, lastTerm uint64) (*Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stateMachine == nil {
		return nil, errors.New("no state machine")
	}

	data, err := s.stateMachine.Snapshot()
	if err != nil {
		return nil, err
	}

	snap := &Snapshot{
		LastIncludedIndex: lastIndex,
		LastIncludedTerm:  lastTerm,
		Data:              data,
	}

	if err := s.store.Save(snap); err != nil {
		return nil, err
	}

	return snap, nil
}

// RestoreSnapshot restores the state machine from a snapshot.
func (s *Snapshotter) RestoreSnapshot(snap *Snapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stateMachine == nil {
		return errors.New("no state machine")
	}

	return s.stateMachine.Restore(snap.Data)
}

// LoadLatest loads and returns the latest snapshot.
func (s *Snapshotter) LoadLatest() (*Snapshot, error) {
	return s.store.Load()
}
