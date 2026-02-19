// Package storage provides the core storage engine components for ObaDB.
package storage

import (
	"encoding/binary"
	"errors"
	"hash/crc32"
)

// WAL record constants.
const (
	// WALRecordHeaderSize is the fixed size of the WAL record header.
	// Layout:
	//   - Bytes 0-7:   LSN (uint64)
	//   - Bytes 8-15:  TxID (uint64)
	//   - Byte 16:     Type (uint8)
	//   - Bytes 17-24: PageID (uint64)
	//   - Bytes 25-26: Offset (uint16)
	//   - Bytes 27-28: OldDataLen (uint16)
	//   - Bytes 29-30: NewDataLen (uint16)
	//   - Bytes 31-34: Checksum (uint32)
	WALRecordHeaderSize = 35

	// MaxWALDataSize is the maximum size for old/new data in a WAL record.
	MaxWALDataSize = 65535
)

// WALType represents the type of a WAL record.
type WALType uint8

const (
	// WALBegin marks the beginning of a transaction.
	WALBegin WALType = iota
	// WALCommit marks the successful completion of a transaction.
	WALCommit
	// WALAbort marks the rollback of a transaction.
	WALAbort
	// WALUpdate records a page modification.
	WALUpdate
	// WALCheckpoint marks a checkpoint in the WAL.
	WALCheckpoint
)

// String returns the string representation of a WALType.
func (t WALType) String() string {
	switch t {
	case WALBegin:
		return "Begin"
	case WALCommit:
		return "Commit"
	case WALAbort:
		return "Abort"
	case WALUpdate:
		return "Update"
	case WALCheckpoint:
		return "Checkpoint"
	default:
		return "Unknown"
	}
}

// WALRecord represents a single record in the Write-Ahead Log.
// All modifications are logged to the WAL before being applied to data pages,
// ensuring atomicity and durability.
type WALRecord struct {
	LSN      uint64  // Log Sequence Number (monotonically increasing)
	TxID     uint64  // Transaction ID
	Type     WALType // Begin, Commit, Abort, Update, Checkpoint
	PageID   PageID  // Affected page (for Update records)
	Offset   uint16  // Offset within page
	OldData  []byte  // Before image (for undo)
	NewData  []byte  // After image (for redo)
	Checksum uint32  // CRC32 of record
}

// Errors for WAL record operations.
var (
	ErrWALRecordTooSmall    = errors.New("WAL record buffer too small")
	ErrWALRecordChecksum    = errors.New("WAL record checksum mismatch")
	ErrWALDataTooLarge      = errors.New("WAL record data exceeds maximum size")
	ErrWALInvalidRecordType = errors.New("invalid WAL record type")
)

// NewWALRecord creates a new WAL record with the given parameters.
func NewWALRecord(lsn, txID uint64, recordType WALType) *WALRecord {
	return &WALRecord{
		LSN:      lsn,
		TxID:     txID,
		Type:     recordType,
		PageID:   0,
		Offset:   0,
		OldData:  nil,
		NewData:  nil,
		Checksum: 0,
	}
}

// NewWALUpdateRecord creates a new WAL update record with page modification data.
func NewWALUpdateRecord(lsn, txID uint64, pageID PageID, offset uint16, oldData, newData []byte) *WALRecord {
	return &WALRecord{
		LSN:      lsn,
		TxID:     txID,
		Type:     WALUpdate,
		PageID:   pageID,
		Offset:   offset,
		OldData:  oldData,
		NewData:  newData,
		Checksum: 0,
	}
}

// Size returns the total serialized size of the WAL record.
func (r *WALRecord) Size() int {
	return WALRecordHeaderSize + len(r.OldData) + len(r.NewData)
}

// Serialize writes the WAL record to a byte slice.
// Returns a new byte slice containing the serialized record.
func (r *WALRecord) Serialize() ([]byte, error) {
	size := r.Size()
	buf := make([]byte, size)
	if err := r.SerializeTo(buf); err != nil {
		return nil, err
	}
	return buf, nil
}

// SerializeTo writes the WAL record to an existing byte slice.
// The slice must be at least Size() bytes.
func (r *WALRecord) SerializeTo(buf []byte) error {
	size := r.Size()
	if len(buf) < size {
		return ErrWALRecordTooSmall
	}

	if len(r.OldData) > MaxWALDataSize || len(r.NewData) > MaxWALDataSize {
		return ErrWALDataTooLarge
	}

	// Write header fields
	binary.LittleEndian.PutUint64(buf[0:8], r.LSN)
	binary.LittleEndian.PutUint64(buf[8:16], r.TxID)
	buf[16] = byte(r.Type)
	binary.LittleEndian.PutUint64(buf[17:25], uint64(r.PageID))
	binary.LittleEndian.PutUint16(buf[25:27], r.Offset)
	binary.LittleEndian.PutUint16(buf[27:29], uint16(len(r.OldData)))
	binary.LittleEndian.PutUint16(buf[29:31], uint16(len(r.NewData)))

	// Write data
	offset := WALRecordHeaderSize
	if len(r.OldData) > 0 {
		copy(buf[offset:], r.OldData)
		offset += len(r.OldData)
	}
	if len(r.NewData) > 0 {
		copy(buf[offset:], r.NewData)
	}

	// Calculate and write checksum (over all bytes except checksum field)
	r.Checksum = r.calculateChecksumFromBuffer(buf[:size])
	binary.LittleEndian.PutUint32(buf[31:35], r.Checksum)

	return nil
}

// Deserialize reads the WAL record from a byte slice.
// The slice must be at least WALRecordHeaderSize bytes.
func (r *WALRecord) Deserialize(buf []byte) error {
	if len(buf) < WALRecordHeaderSize {
		return ErrWALRecordTooSmall
	}

	// Read header fields
	r.LSN = binary.LittleEndian.Uint64(buf[0:8])
	r.TxID = binary.LittleEndian.Uint64(buf[8:16])
	r.Type = WALType(buf[16])
	r.PageID = PageID(binary.LittleEndian.Uint64(buf[17:25]))
	r.Offset = binary.LittleEndian.Uint16(buf[25:27])
	oldDataLen := binary.LittleEndian.Uint16(buf[27:29])
	newDataLen := binary.LittleEndian.Uint16(buf[29:31])
	r.Checksum = binary.LittleEndian.Uint32(buf[31:35])

	// Calculate total size needed
	totalSize := WALRecordHeaderSize + int(oldDataLen) + int(newDataLen)
	if len(buf) < totalSize {
		return ErrWALRecordTooSmall
	}

	// Read data
	offset := WALRecordHeaderSize
	if oldDataLen > 0 {
		r.OldData = make([]byte, oldDataLen)
		copy(r.OldData, buf[offset:offset+int(oldDataLen)])
		offset += int(oldDataLen)
	} else {
		r.OldData = nil
	}

	if newDataLen > 0 {
		r.NewData = make([]byte, newDataLen)
		copy(r.NewData, buf[offset:offset+int(newDataLen)])
	} else {
		r.NewData = nil
	}

	return nil
}

// calculateChecksumFromBuffer computes CRC32 checksum from the serialized buffer.
// Checksum is calculated over all bytes except the checksum field (bytes 31-34).
func (r *WALRecord) calculateChecksumFromBuffer(buf []byte) uint32 {
	// Create a copy without the checksum field for calculation
	checksumBuf := make([]byte, len(buf))
	copy(checksumBuf, buf)
	// Zero out the checksum field
	checksumBuf[31] = 0
	checksumBuf[32] = 0
	checksumBuf[33] = 0
	checksumBuf[34] = 0
	return crc32.ChecksumIEEE(checksumBuf)
}

// CalculateChecksum computes the CRC32 checksum of the record.
// This serializes the record to a temporary buffer to calculate the checksum.
func (r *WALRecord) CalculateChecksum() uint32 {
	_, err := r.Serialize()
	if err != nil {
		return 0
	}
	// The checksum was already calculated during serialization
	return r.Checksum
}

// ValidateChecksum verifies the record checksum matches the stored value.
func (r *WALRecord) ValidateChecksum() bool {
	buf, err := r.Serialize()
	if err != nil {
		return false
	}
	// Recalculate checksum from buffer
	expected := r.calculateChecksumFromBuffer(buf)
	return r.Checksum == expected
}

// DeserializeAndValidate reads the record and validates its checksum.
func (r *WALRecord) DeserializeAndValidate(buf []byte) error {
	if err := r.Deserialize(buf); err != nil {
		return err
	}

	// Validate checksum
	expected := r.calculateChecksumFromBuffer(buf[:r.Size()])
	if r.Checksum != expected {
		return ErrWALRecordChecksum
	}

	return nil
}

// IsTransactionControl returns true if this is a transaction control record.
func (r *WALRecord) IsTransactionControl() bool {
	return r.Type == WALBegin || r.Type == WALCommit || r.Type == WALAbort
}

// IsDataModification returns true if this record modifies data.
func (r *WALRecord) IsDataModification() bool {
	return r.Type == WALUpdate
}

// Clone creates a deep copy of the WAL record.
func (r *WALRecord) Clone() *WALRecord {
	clone := &WALRecord{
		LSN:      r.LSN,
		TxID:     r.TxID,
		Type:     r.Type,
		PageID:   r.PageID,
		Offset:   r.Offset,
		Checksum: r.Checksum,
	}

	if r.OldData != nil {
		clone.OldData = make([]byte, len(r.OldData))
		copy(clone.OldData, r.OldData)
	}

	if r.NewData != nil {
		clone.NewData = make([]byte, len(r.NewData))
		copy(clone.NewData, r.NewData)
	}

	return clone
}
