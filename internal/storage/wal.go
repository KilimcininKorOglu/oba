// Package storage provides the core storage engine components for ObaDB.
package storage

import (
	"encoding/binary"
	"errors"
	"io"
	"os"
	"sync"

	"github.com/oba-ldap/oba/internal/crypto"
)

// WAL constants.
const (
	// WALBufferSize is the default size of the WAL write buffer.
	WALBufferSize = 64 * 1024 // 64KB

	// WALRecordLengthSize is the size of the length prefix for each record.
	WALRecordLengthSize = 4
)

// WAL errors.
var (
	ErrWALClosed       = errors.New("WAL is closed")
	ErrWALCorrupted    = errors.New("WAL file is corrupted")
	ErrWALTruncateLSN  = errors.New("cannot truncate to LSN greater than current")
	ErrWALInvalidLSN   = errors.New("invalid LSN")
	ErrWALReadPastEnd  = errors.New("read past end of WAL")
	ErrWALRecordLength = errors.New("invalid WAL record length")
)

// WAL represents the Write-Ahead Log for durability and crash recovery.
// All modifications are logged to the WAL before being applied to data pages,
// ensuring atomicity and durability.
type WAL struct {
	file       *os.File
	path       string
	currentLSN uint64
	buffer     []byte
	bufferPos  int
	mu         sync.Mutex
	closed     bool

	// Index for fast LSN lookup: maps LSN to file offset
	lsnIndex map[uint64]int64

	// Encryption key (nil if encryption is disabled)
	encryptionKey *crypto.EncryptionKey
}

// OpenWAL opens or creates a WAL file at the given path.
func OpenWAL(path string) (*WAL, error) {
	return OpenWALWithEncryption(path, nil)
}

// OpenWALWithEncryption opens or creates a WAL file with optional encryption.
func OpenWALWithEncryption(path string, encryptionKey *crypto.EncryptionKey) (*WAL, error) {
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, err
	}

	wal := &WAL{
		file:          file,
		path:          path,
		currentLSN:    0,
		buffer:        make([]byte, WALBufferSize),
		bufferPos:     0,
		closed:        false,
		lsnIndex:      make(map[uint64]int64),
		encryptionKey: encryptionKey,
	}

	// Recover existing records and rebuild LSN index
	if err := wal.recover(); err != nil {
		file.Close()
		return nil, err
	}

	return wal, nil
}

// recover reads existing WAL records and rebuilds the LSN index.
func (w *WAL) recover() error {
	// Seek to beginning
	if _, err := w.file.Seek(0, io.SeekStart); err != nil {
		return err
	}

	// Get file size
	info, err := w.file.Stat()
	if err != nil {
		return err
	}

	fileSize := info.Size()
	if fileSize == 0 {
		// Empty WAL, start fresh with LSN 1
		w.currentLSN = 1
		return nil
	}

	// Read all records to rebuild index
	var offset int64 = 0
	var maxLSN uint64 = 0

	for offset < fileSize {
		// Read record length
		lengthBuf := make([]byte, WALRecordLengthSize)
		n, err := w.file.ReadAt(lengthBuf, offset)
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		if n < WALRecordLengthSize {
			// Incomplete length prefix, truncate here
			break
		}

		recordLen := binary.LittleEndian.Uint32(lengthBuf)
		if recordLen == 0 || recordLen > uint32(WALBufferSize) {
			// Invalid record length, truncate here
			break
		}

		// Read record data
		recordBuf := make([]byte, recordLen)
		n, err = w.file.ReadAt(recordBuf, offset+WALRecordLengthSize)
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		if n < int(recordLen) {
			// Incomplete record, truncate here
			break
		}

		// Decrypt if encryption is enabled
		if w.encryptionKey != nil {
			decrypted, err := w.encryptionKey.Decrypt(recordBuf)
			if err != nil {
				// Corrupted or wrong key, truncate here
				break
			}
			recordBuf = decrypted
		}

		// Deserialize and validate record
		record := &WALRecord{}
		if err := record.DeserializeAndValidate(recordBuf); err != nil {
			// Corrupted record, truncate here
			break
		}

		// Add to index
		w.lsnIndex[record.LSN] = offset

		// Track max LSN
		if record.LSN > maxLSN {
			maxLSN = record.LSN
		}

		// Move to next record
		offset += WALRecordLengthSize + int64(recordLen)
	}

	// Set current LSN to max + 1
	if len(w.lsnIndex) > 0 {
		w.currentLSN = maxLSN + 1
	} else {
		w.currentLSN = 1
	}

	// Truncate file to valid portion
	if err := w.file.Truncate(offset); err != nil {
		return err
	}

	// Seek to end for appending
	if _, err := w.file.Seek(0, io.SeekEnd); err != nil {
		return err
	}

	return nil
}

// Append writes a WAL record and returns its LSN.
// The record's LSN field will be set to the assigned LSN.
func (w *WAL) Append(record *WALRecord) (uint64, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return 0, ErrWALClosed
	}

	// Assign LSN
	record.LSN = w.currentLSN

	// Serialize record
	recordBuf, err := record.Serialize()
	if err != nil {
		return 0, err
	}

	// Encrypt if encryption is enabled
	if w.encryptionKey != nil {
		recordBuf, err = w.encryptionKey.Encrypt(recordBuf)
		if err != nil {
			return 0, err
		}
	}

	recordLen := uint32(len(recordBuf))

	// Check if we need to flush buffer
	totalSize := WALRecordLengthSize + int(recordLen)
	if w.bufferPos+totalSize > len(w.buffer) {
		if err := w.flushBuffer(); err != nil {
			return 0, err
		}
	}

	// Get current file position for index
	filePos, err := w.file.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}
	indexOffset := filePos + int64(w.bufferPos)

	// Write length prefix to buffer
	binary.LittleEndian.PutUint32(w.buffer[w.bufferPos:], recordLen)
	w.bufferPos += WALRecordLengthSize

	// Write record to buffer
	copy(w.buffer[w.bufferPos:], recordBuf)
	w.bufferPos += int(recordLen)

	// Add to index
	w.lsnIndex[record.LSN] = indexOffset

	// Increment LSN
	lsn := w.currentLSN
	w.currentLSN++

	return lsn, nil
}

// flushBuffer writes the buffer contents to the file.
func (w *WAL) flushBuffer() error {
	if w.bufferPos == 0 {
		return nil
	}

	_, err := w.file.Write(w.buffer[:w.bufferPos])
	if err != nil {
		return err
	}

	w.bufferPos = 0
	return nil
}

// Sync ensures all WAL records are durably written to disk.
func (w *WAL) Sync() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return ErrWALClosed
	}

	// Flush buffer first
	if err := w.flushBuffer(); err != nil {
		return err
	}

	// Sync to disk
	return w.file.Sync()
}

// Truncate removes all WAL records with LSN less than or equal to the given LSN.
// This is typically called after a checkpoint to reclaim space.
func (w *WAL) Truncate(lsn uint64) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return ErrWALClosed
	}

	// Flush buffer first
	if err := w.flushBuffer(); err != nil {
		return err
	}

	// Find the offset of the first record after the truncation point
	var truncateOffset int64 = -1
	var minLSNAfter uint64 = 0

	for recordLSN, offset := range w.lsnIndex {
		if recordLSN > lsn {
			if truncateOffset == -1 || offset < truncateOffset {
				truncateOffset = offset
				minLSNAfter = recordLSN
			}
		}
	}

	// If no records after truncation point, truncate entire file
	if truncateOffset == -1 {
		// Remove all entries from index
		w.lsnIndex = make(map[uint64]int64)

		// Truncate file to zero
		if err := w.file.Truncate(0); err != nil {
			return err
		}

		// Seek to beginning
		if _, err := w.file.Seek(0, io.SeekStart); err != nil {
			return err
		}

		return nil
	}

	// Read remaining records
	remainingData, err := w.readFromOffset(truncateOffset)
	if err != nil {
		return err
	}

	// Truncate file
	if err := w.file.Truncate(0); err != nil {
		return err
	}

	// Seek to beginning
	if _, err := w.file.Seek(0, io.SeekStart); err != nil {
		return err
	}

	// Write remaining data
	if len(remainingData) > 0 {
		if _, err := w.file.Write(remainingData); err != nil {
			return err
		}
	}

	// Rebuild index with new offsets
	newIndex := make(map[uint64]int64)
	var offset int64 = 0

	for recordLSN := range w.lsnIndex {
		if recordLSN > lsn {
			// Calculate new offset based on order
			newIndex[recordLSN] = offset
			// We need to recalculate offsets properly
		}
	}

	// Re-read to rebuild index properly
	w.lsnIndex = make(map[uint64]int64)
	if _, err := w.file.Seek(0, io.SeekStart); err != nil {
		return err
	}

	// Rebuild index by reading records
	info, err := w.file.Stat()
	if err != nil {
		return err
	}

	fileSize := info.Size()
	offset = 0

	for offset < fileSize {
		lengthBuf := make([]byte, WALRecordLengthSize)
		n, err := w.file.ReadAt(lengthBuf, offset)
		if err != nil || n < WALRecordLengthSize {
			break
		}

		recordLen := binary.LittleEndian.Uint32(lengthBuf)
		if recordLen == 0 {
			break
		}

		recordBuf := make([]byte, recordLen)
		n, err = w.file.ReadAt(recordBuf, offset+WALRecordLengthSize)
		if err != nil || n < int(recordLen) {
			break
		}

		// Decrypt if encryption is enabled
		if w.encryptionKey != nil {
			decrypted, err := w.encryptionKey.Decrypt(recordBuf)
			if err != nil {
				break
			}
			recordBuf = decrypted
		}

		record := &WALRecord{}
		if err := record.Deserialize(recordBuf); err != nil {
			break
		}

		w.lsnIndex[record.LSN] = offset
		offset += WALRecordLengthSize + int64(recordLen)
	}

	// Seek to end
	if _, err := w.file.Seek(0, io.SeekEnd); err != nil {
		return err
	}

	// Ensure minLSNAfter is used to avoid unused variable error
	_ = minLSNAfter

	return nil
}

// readFromOffset reads all data from the given offset to end of file.
func (w *WAL) readFromOffset(offset int64) ([]byte, error) {
	info, err := w.file.Stat()
	if err != nil {
		return nil, err
	}

	size := info.Size() - offset
	if size <= 0 {
		return nil, nil
	}

	data := make([]byte, size)
	_, err = w.file.ReadAt(data, offset)
	if err != nil && err != io.EOF {
		return nil, err
	}

	return data, nil
}

// Iterator returns a WAL iterator starting from the given LSN.
func (w *WAL) Iterator(startLSN uint64) *WALIterator {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Flush buffer to ensure all records are readable
	w.flushBuffer()

	return &WALIterator{
		wal:        w,
		currentLSN: startLSN,
		offset:     -1, // Will be set on first Next() call
	}
}

// CurrentLSN returns the next LSN that will be assigned.
func (w *WAL) CurrentLSN() uint64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.currentLSN
}

// Close closes the WAL file.
func (w *WAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil
	}

	// Flush buffer
	if err := w.flushBuffer(); err != nil {
		return err
	}

	// Sync to disk
	if err := w.file.Sync(); err != nil {
		return err
	}

	w.closed = true
	return w.file.Close()
}

// WALIterator iterates over WAL records in order.
type WALIterator struct {
	wal        *WAL
	currentLSN uint64
	offset     int64
	err        error
}

// Next advances to the next record and returns true if successful.
func (it *WALIterator) Next() bool {
	if it.err != nil {
		return false
	}

	it.wal.mu.Lock()
	defer it.wal.mu.Unlock()

	// Find the offset for current LSN
	offset, exists := it.wal.lsnIndex[it.currentLSN]
	if !exists {
		// Try to find next available LSN
		var nextLSN uint64 = 0
		var nextOffset int64 = -1

		for lsn, off := range it.wal.lsnIndex {
			if lsn >= it.currentLSN {
				if nextLSN == 0 || lsn < nextLSN {
					nextLSN = lsn
					nextOffset = off
				}
			}
		}

		if nextLSN == 0 {
			return false
		}

		it.currentLSN = nextLSN
		offset = nextOffset
	}

	it.offset = offset
	return true
}

// Record returns the current WAL record.
func (it *WALIterator) Record() (*WALRecord, error) {
	if it.offset < 0 {
		return nil, ErrWALInvalidLSN
	}

	it.wal.mu.Lock()
	defer it.wal.mu.Unlock()

	// Read record length
	lengthBuf := make([]byte, WALRecordLengthSize)
	n, err := it.wal.file.ReadAt(lengthBuf, it.offset)
	if err != nil {
		return nil, err
	}
	if n < WALRecordLengthSize {
		return nil, ErrWALRecordLength
	}

	recordLen := binary.LittleEndian.Uint32(lengthBuf)
	if recordLen == 0 {
		return nil, ErrWALRecordLength
	}

	// Read record data
	recordBuf := make([]byte, recordLen)
	n, err = it.wal.file.ReadAt(recordBuf, it.offset+WALRecordLengthSize)
	if err != nil {
		return nil, err
	}
	if n < int(recordLen) {
		return nil, ErrWALRecordLength
	}

	// Decrypt if encryption is enabled
	if it.wal.encryptionKey != nil {
		decrypted, err := it.wal.encryptionKey.Decrypt(recordBuf)
		if err != nil {
			return nil, err
		}
		recordBuf = decrypted
	}

	// Deserialize record
	record := &WALRecord{}
	if err := record.DeserializeAndValidate(recordBuf); err != nil {
		return nil, err
	}

	// Advance to next LSN for subsequent Next() call
	it.currentLSN = record.LSN + 1

	return record, nil
}

// Error returns any error encountered during iteration.
func (it *WALIterator) Error() error {
	return it.err
}

// LSN returns the current LSN being iterated.
func (it *WALIterator) LSN() uint64 {
	return it.currentLSN
}
