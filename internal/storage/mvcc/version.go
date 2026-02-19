// Package mvcc provides Multi-Version Concurrency Control for ObaDB.
// It implements version chains that track multiple versions of each entry,
// allowing readers to access consistent snapshots while writers create new versions.
package mvcc

import (
	"encoding/binary"
	"errors"
	"sync"

	"github.com/oba-ldap/oba/internal/storage"
)

// Version errors.
var (
	ErrVersionNotFound    = errors.New("version not found")
	ErrVersionDeleted     = errors.New("version has been deleted")
	ErrNoVisibleVersion   = errors.New("no visible version for snapshot")
	ErrInvalidVersion     = errors.New("invalid version data")
	ErrVersionConflict    = errors.New("version conflict detected")
	ErrNilTransaction     = errors.New("transaction is nil")
	ErrTransactionAborted = errors.New("transaction has been aborted")
)

// VersionState represents the state of a version.
type VersionState uint8

const (
	// VersionActive indicates the version is active and visible.
	VersionActive VersionState = iota
	// VersionDeleted indicates the version has been marked as deleted.
	VersionDeleted
)

// String returns the string representation of a VersionState.
func (s VersionState) String() string {
	switch s {
	case VersionActive:
		return "Active"
	case VersionDeleted:
		return "Deleted"
	default:
		return "Unknown"
	}
}

// Version represents a single version of an entry in the version chain.
// Each modification creates a new version linked to the previous one.
// Readers traverse the chain to find the version visible to their snapshot.
type Version struct {
	// TxID is the transaction ID that created this version.
	TxID uint64

	// CommitTS is the commit timestamp (0 if uncommitted).
	// A version is visible to a snapshot if CommitTS <= snapshot and CommitTS > 0.
	CommitTS uint64

	// Data contains the entry data for this version.
	// For deleted entries, this may be nil or empty.
	Data []byte

	// Prev points to the previous version in the chain.
	// nil indicates this is the oldest version.
	Prev *Version

	// PageID is the storage location page ID.
	PageID storage.PageID

	// SlotID is the slot index within the page.
	SlotID uint16

	// State indicates whether this version is active or deleted.
	State VersionState

	// mu protects concurrent access to this version.
	mu sync.RWMutex
}

// NewVersion creates a new version with the given transaction ID and data.
func NewVersion(txID uint64, data []byte, pageID storage.PageID, slotID uint16) *Version {
	// Make a copy of the data to avoid external modifications
	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)

	return &Version{
		TxID:     txID,
		CommitTS: 0, // Uncommitted initially
		Data:     dataCopy,
		Prev:     nil,
		PageID:   pageID,
		SlotID:   slotID,
		State:    VersionActive,
	}
}

// NewDeletedVersion creates a new version that marks an entry as deleted.
func NewDeletedVersion(txID uint64, pageID storage.PageID, slotID uint16) *Version {
	return &Version{
		TxID:     txID,
		CommitTS: 0, // Uncommitted initially
		Data:     nil,
		Prev:     nil,
		PageID:   pageID,
		SlotID:   slotID,
		State:    VersionDeleted,
	}
}

// IsCommitted returns true if this version has been committed.
func (v *Version) IsCommitted() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.CommitTS > 0
}

// IsDeleted returns true if this version represents a deletion.
func (v *Version) IsDeleted() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.State == VersionDeleted
}

// IsActive returns true if this version is active (not deleted).
func (v *Version) IsActive() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.State == VersionActive
}

// Commit marks this version as committed with the given timestamp.
func (v *Version) Commit(commitTS uint64) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.CommitTS = commitTS
}

// SetPrev sets the previous version in the chain.
func (v *Version) SetPrev(prev *Version) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.Prev = prev
}

// GetPrev returns the previous version in the chain.
func (v *Version) GetPrev() *Version {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.Prev
}

// GetData returns a copy of the version data.
func (v *Version) GetData() []byte {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if v.Data == nil {
		return nil
	}

	dataCopy := make([]byte, len(v.Data))
	copy(dataCopy, v.Data)
	return dataCopy
}

// GetTxID returns the transaction ID that created this version.
func (v *Version) GetTxID() uint64 {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.TxID
}

// GetCommitTS returns the commit timestamp.
func (v *Version) GetCommitTS() uint64 {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.CommitTS
}

// GetState returns the version state.
func (v *Version) GetState() VersionState {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.State
}

// GetLocation returns the storage location (PageID, SlotID).
func (v *Version) GetLocation() (storage.PageID, uint16) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.PageID, v.SlotID
}

// IsVisibleTo determines if this version is visible to the given snapshot.
// Visibility rules:
// 1. If the version is uncommitted (CommitTS == 0):
//   - Visible only to the transaction that created it (txID == snapshot)
//
// 2. If the version is committed (CommitTS > 0):
//   - Visible if CommitTS <= snapshot
//
// 3. Deleted versions are visible (to indicate the entry was deleted)
//    but GetVisible will return an error for deleted entries.
func (v *Version) IsVisibleTo(snapshot uint64, activeTxID uint64) bool {
	v.mu.RLock()
	defer v.mu.RUnlock()

	// Uncommitted version: visible only to the creating transaction
	if v.CommitTS == 0 {
		return v.TxID == activeTxID
	}

	// Committed version: visible if committed before or at snapshot
	return v.CommitTS <= snapshot
}

// Clone creates a deep copy of the version (without the Prev pointer).
func (v *Version) Clone() *Version {
	v.mu.RLock()
	defer v.mu.RUnlock()

	var dataCopy []byte
	if v.Data != nil {
		dataCopy = make([]byte, len(v.Data))
		copy(dataCopy, v.Data)
	}

	return &Version{
		TxID:     v.TxID,
		CommitTS: v.CommitTS,
		Data:     dataCopy,
		Prev:     nil, // Don't copy the chain
		PageID:   v.PageID,
		SlotID:   v.SlotID,
		State:    v.State,
	}
}

// VersionHeader is the serialized header for a version.
// Layout (32 bytes):
//   - Bytes 0-7:   TxID (uint64)
//   - Bytes 8-15:  CommitTS (uint64)
//   - Bytes 16-23: PageID (uint64)
//   - Bytes 24-25: SlotID (uint16)
//   - Byte 26:     State (uint8)
//   - Bytes 27-31: Reserved
const VersionHeaderSize = 32

// Serialize serializes the version header to bytes.
func (v *Version) SerializeHeader() []byte {
	v.mu.RLock()
	defer v.mu.RUnlock()

	buf := make([]byte, VersionHeaderSize)
	binary.LittleEndian.PutUint64(buf[0:8], v.TxID)
	binary.LittleEndian.PutUint64(buf[8:16], v.CommitTS)
	binary.LittleEndian.PutUint64(buf[16:24], uint64(v.PageID))
	binary.LittleEndian.PutUint16(buf[24:26], v.SlotID)
	buf[26] = byte(v.State)
	// Bytes 27-31 reserved

	return buf
}

// DeserializeHeader deserializes a version header from bytes.
func DeserializeVersionHeader(buf []byte) (*Version, error) {
	if len(buf) < VersionHeaderSize {
		return nil, ErrInvalidVersion
	}

	return &Version{
		TxID:     binary.LittleEndian.Uint64(buf[0:8]),
		CommitTS: binary.LittleEndian.Uint64(buf[8:16]),
		PageID:   storage.PageID(binary.LittleEndian.Uint64(buf[16:24])),
		SlotID:   binary.LittleEndian.Uint16(buf[24:26]),
		State:    VersionState(buf[26]),
		Data:     nil,
		Prev:     nil,
	}, nil
}

// ChainLength returns the length of the version chain starting from this version.
func (v *Version) ChainLength() int {
	count := 0
	current := v
	for current != nil {
		count++
		current = current.GetPrev()
	}
	return count
}
