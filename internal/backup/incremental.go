// Package backup provides backup and restore functionality for ObaDB.
package backup

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/KilimcininKorOglu/oba/internal/storage"
)

// Incremental backup constants.
const (
	// IncrementalMagicByte0-3 form the magic number for incremental backups.
	IncrementalMagicByte0 = 'O'
	IncrementalMagicByte1 = 'B'
	IncrementalMagicByte2 = 'A'
	IncrementalMagicByte3 = 'I'

	// IncrementalHeaderSize is the size of the incremental backup header in bytes.
	IncrementalHeaderSize = 80

	// MetadataFileName is the name of the backup metadata file.
	MetadataFileName = "backup_metadata.oba"
)

// IncrementalMagic is the magic number for incremental backup files.
var IncrementalMagic = [4]byte{IncrementalMagicByte0, IncrementalMagicByte1, IncrementalMagicByte2, IncrementalMagicByte3}

// Incremental backup errors.
var (
	ErrNoBaseBackup            = errors.New("no base backup found, run full backup first")
	ErrInvalidIncrementalMagic = errors.New("invalid incremental backup magic number")
	ErrIncrementalCorrupted    = errors.New("incremental backup is corrupted")
	ErrWALNotAvailable         = errors.New("WAL is not available")
	ErrMetadataNotFound        = errors.New("backup metadata not found")
	ErrInvalidMetadata         = errors.New("invalid backup metadata")
)

// IncrementalHeader represents the header of an incremental backup file.
// Layout (80 bytes):
//   - Bytes 0-3:   Magic number ("OBAI")
//   - Bytes 4-7:   Version (uint32)
//   - Bytes 8-15:  Timestamp (int64, Unix timestamp)
//   - Bytes 16-19: Flags (uint32)
//   - Bytes 20-23: PageSize (uint32)
//   - Bytes 24-31: BaseLSN (uint64) - LSN of the base backup
//   - Bytes 32-39: CurrentLSN (uint64) - Current LSN at backup time
//   - Bytes 40-47: PageCount (uint64) - Number of modified pages
//   - Bytes 48-55: TotalBytes (uint64) - Total size of page data
//   - Bytes 56-59: Checksum (uint32, CRC32 of all page data)
//   - Bytes 60-79: Reserved
type IncrementalHeader struct {
	Magic      [4]byte
	Version    uint32
	Timestamp  int64
	Flags      uint32
	PageSize   uint32
	BaseLSN    uint64
	CurrentLSN uint64
	PageCount  uint64
	TotalBytes uint64
	Checksum   uint32
	Reserved   [20]byte
}

// NewIncrementalHeader creates a new incremental backup header with default values.
func NewIncrementalHeader() *IncrementalHeader {
	return &IncrementalHeader{
		Magic:     IncrementalMagic,
		Version:   BackupVersion,
		Timestamp: time.Now().Unix(),
		Flags:     0,
	}
}

// IsCompressed returns true if the backup is compressed.
func (h *IncrementalHeader) IsCompressed() bool {
	return h.Flags&BackupFlagCompressed != 0
}

// SetCompressed sets the compressed flag.
func (h *IncrementalHeader) SetCompressed(compressed bool) {
	if compressed {
		h.Flags |= BackupFlagCompressed
	} else {
		h.Flags &^= BackupFlagCompressed
	}
}

// Serialize writes the incremental header to a byte slice.
func (h *IncrementalHeader) Serialize() ([]byte, error) {
	buf := make([]byte, IncrementalHeaderSize)
	return buf, h.SerializeTo(buf)
}

// SerializeTo writes the incremental header to an existing byte slice.
func (h *IncrementalHeader) SerializeTo(buf []byte) error {
	if len(buf) < IncrementalHeaderSize {
		return ErrInvalidBackup
	}

	// Clear buffer
	for i := range buf[:IncrementalHeaderSize] {
		buf[i] = 0
	}

	// Write magic
	copy(buf[0:4], h.Magic[:])

	// Write version
	binary.LittleEndian.PutUint32(buf[4:8], h.Version)

	// Write timestamp
	binary.LittleEndian.PutUint64(buf[8:16], uint64(h.Timestamp))

	// Write flags
	binary.LittleEndian.PutUint32(buf[16:20], h.Flags)

	// Write page size
	binary.LittleEndian.PutUint32(buf[20:24], h.PageSize)

	// Write base LSN
	binary.LittleEndian.PutUint64(buf[24:32], h.BaseLSN)

	// Write current LSN
	binary.LittleEndian.PutUint64(buf[32:40], h.CurrentLSN)

	// Write page count
	binary.LittleEndian.PutUint64(buf[40:48], h.PageCount)

	// Write total bytes
	binary.LittleEndian.PutUint64(buf[48:56], h.TotalBytes)

	// Write checksum
	binary.LittleEndian.PutUint32(buf[56:60], h.Checksum)

	// Reserved bytes are already zeroed

	return nil
}

// Deserialize reads the incremental header from a byte slice.
func (h *IncrementalHeader) Deserialize(buf []byte) error {
	if len(buf) < IncrementalHeaderSize {
		return ErrInvalidBackup
	}

	// Read magic
	copy(h.Magic[:], buf[0:4])

	// Read version
	h.Version = binary.LittleEndian.Uint32(buf[4:8])

	// Read timestamp
	h.Timestamp = int64(binary.LittleEndian.Uint64(buf[8:16]))

	// Read flags
	h.Flags = binary.LittleEndian.Uint32(buf[16:20])

	// Read page size
	h.PageSize = binary.LittleEndian.Uint32(buf[20:24])

	// Read base LSN
	h.BaseLSN = binary.LittleEndian.Uint64(buf[24:32])

	// Read current LSN
	h.CurrentLSN = binary.LittleEndian.Uint64(buf[32:40])

	// Read page count
	h.PageCount = binary.LittleEndian.Uint64(buf[40:48])

	// Read total bytes
	h.TotalBytes = binary.LittleEndian.Uint64(buf[48:56])

	// Read checksum
	h.Checksum = binary.LittleEndian.Uint32(buf[56:60])

	// Read reserved
	copy(h.Reserved[:], buf[60:80])

	return nil
}

// Validate validates the incremental header.
func (h *IncrementalHeader) Validate() error {
	if h.Magic != IncrementalMagic {
		return ErrInvalidIncrementalMagic
	}

	if h.Version == 0 || h.Version > BackupVersion {
		return ErrUnsupportedFormat
	}

	return nil
}

// BackupMetadata stores information about the last backup for incremental backups.
type BackupMetadata struct {
	LastBackupLSN  uint64
	LastBackupTime int64
	BackupType     string // "full" or "incremental"
	BackupPath     string
}

// IncrementalBackupManager extends BackupManager with incremental backup support.
type IncrementalBackupManager struct {
	*BackupManager
	wal         *storage.WAL
	metadataDir string
}

// NewIncrementalBackupManager creates a new IncrementalBackupManager.
func NewIncrementalBackupManager(pageManager *storage.PageManager, wal *storage.WAL, metadataDir string) *IncrementalBackupManager {
	return &IncrementalBackupManager{
		BackupManager: NewBackupManager(pageManager),
		wal:           wal,
		metadataDir:   metadataDir,
	}
}

// IncrementalBackup performs an incremental backup that only captures changes since the last backup.
func (ibm *IncrementalBackupManager) IncrementalBackup(opts *BackupOptions) (*BackupStats, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	if ibm.pageManager == nil {
		return nil, ErrNilPageManager
	}

	if ibm.wal == nil {
		return nil, ErrWALNotAvailable
	}

	startTime := time.Now()
	stats := &BackupStats{}

	// Read last backup LSN from metadata
	lastLSN, err := ibm.readLastBackupLSN()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNoBaseBackup, err)
	}

	// Get current LSN
	currentLSN := ibm.wal.CurrentLSN()

	// If current LSN is same as last backup, nothing to backup
	if currentLSN <= lastLSN {
		// No changes since last backup
		stats.Duration = time.Since(startTime)
		return stats, nil
	}

	// Find modified pages from WAL
	modifiedPages := make(map[storage.PageID]bool)
	iter := ibm.wal.Iterator(lastLSN)
	for iter.Next() {
		record, err := iter.Record()
		if err != nil {
			break
		}
		if record.Type == storage.WALUpdate {
			modifiedPages[record.PageID] = true
		}
	}

	// Open output file
	out, err := os.Create(opts.OutputPath)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBackupFailed, err)
	}
	defer out.Close()

	// Get page size
	pageSize := ibm.pageManager.PageSize()

	// Create incremental backup header
	header := NewIncrementalHeader()
	header.PageSize = uint32(pageSize)
	header.BaseLSN = lastLSN
	header.CurrentLSN = currentLSN
	header.PageCount = uint64(len(modifiedPages))
	header.SetCompressed(opts.Compress)

	// Write placeholder header (will update checksum at end)
	headerBuf, err := header.Serialize()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBackupFailed, err)
	}
	if _, err := out.Write(headerBuf); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBackupFailed, err)
	}

	// Setup writer (with optional compression)
	var writer io.Writer
	var compressWriter *CompressWriter
	var checksumWriter *checksumWriter

	if opts.Compress {
		compressWriter = NewCompressWriter(out)
		checksumWriter = newChecksumWriter(compressWriter)
		writer = checksumWriter
	} else {
		checksumWriter = newChecksumWriter(out)
		writer = checksumWriter
	}

	// Write only modified pages
	// Each page entry: PageID (8 bytes) + Page Data (pageSize bytes)
	pageIDSize := 8
	for pageID := range modifiedPages {
		page, err := ibm.pageManager.ReadPage(pageID)
		if err != nil {
			// Skip pages that can't be read (might be freed)
			continue
		}

		// Write page ID
		pageIDBuf := make([]byte, pageIDSize)
		binary.LittleEndian.PutUint64(pageIDBuf, uint64(pageID))
		if _, err := writer.Write(pageIDBuf); err != nil {
			return nil, fmt.Errorf("%w: failed to write page ID %d: %v", ErrBackupFailed, pageID, err)
		}

		// Serialize and write page data
		pageData, err := page.Serialize()
		if err != nil {
			return nil, fmt.Errorf("%w: failed to serialize page %d: %v", ErrBackupFailed, pageID, err)
		}

		if _, err := writer.Write(pageData); err != nil {
			return nil, fmt.Errorf("%w: failed to write page %d: %v", ErrBackupFailed, pageID, err)
		}

		stats.TotalPages++
	}

	// Close compression writer if used
	if compressWriter != nil {
		if err := compressWriter.Close(); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrBackupFailed, err)
		}
		stats.CompressedBytes = compressWriter.Written()
	}

	// Update header with checksum
	header.Checksum = checksumWriter.Checksum()
	header.TotalBytes = uint64(checksumWriter.Written())

	// Seek back to beginning and rewrite header
	if _, err := out.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBackupFailed, err)
	}

	headerBuf, err = header.Serialize()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBackupFailed, err)
	}
	if _, err := out.Write(headerBuf); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBackupFailed, err)
	}

	// Sync to disk
	if err := out.Sync(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBackupFailed, err)
	}

	// Update last backup LSN
	if err := ibm.writeLastBackupLSN(currentLSN, "incremental", opts.OutputPath); err != nil {
		return nil, fmt.Errorf("%w: failed to update metadata: %v", ErrBackupFailed, err)
	}

	// Get final file size
	fileInfo, err := out.Stat()
	if err == nil {
		stats.TotalBytes = fileInfo.Size()
	}

	stats.Duration = time.Since(startTime)

	return stats, nil
}

// IncrementalRestore restores from an incremental backup file.
func (ibm *IncrementalBackupManager) IncrementalRestore(opts *RestoreOptions) (*BackupStats, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	if ibm.pageManager == nil {
		return nil, ErrNilPageManager
	}

	startTime := time.Now()
	stats := &BackupStats{}

	// Open input file
	in, err := os.Open(opts.InputPath)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRestoreFailed, err)
	}
	defer in.Close()

	// Read and validate header
	headerBuf := make([]byte, IncrementalHeaderSize)
	if _, err := io.ReadFull(in, headerBuf); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRestoreFailed, err)
	}

	header := &IncrementalHeader{}
	if err := header.Deserialize(headerBuf); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRestoreFailed, err)
	}

	if err := header.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRestoreFailed, err)
	}

	// Setup reader (with optional decompression)
	var reader io.Reader
	if header.IsCompressed() {
		reader = NewDecompressReader(in)
	} else {
		reader = in
	}

	// Verify checksum if requested
	if opts.Verify {
		checksumWriter := newChecksumWriter(io.Discard)
		pageSize := int(header.PageSize)
		pageIDSize := 8
		entrySize := pageIDSize + pageSize

		for i := uint64(0); i < header.PageCount; i++ {
			entryBuf := make([]byte, entrySize)
			n, err := io.ReadFull(reader, entryBuf)
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("%w: %v", ErrRestoreFailed, err)
			}
			checksumWriter.Write(entryBuf[:n])
		}

		if checksumWriter.Checksum() != header.Checksum {
			return nil, ErrChecksumMismatch
		}

		// Seek back to start of data
		if _, err := in.Seek(IncrementalHeaderSize, io.SeekStart); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrRestoreFailed, err)
		}

		// Recreate reader
		if header.IsCompressed() {
			reader = NewDecompressReader(in)
		} else {
			reader = in
		}
	}

	// Restore pages
	pageSize := int(header.PageSize)
	pageIDSize := 8
	entrySize := pageIDSize + pageSize

	for i := uint64(0); i < header.PageCount; i++ {
		entryBuf := make([]byte, entrySize)
		n, err := io.ReadFull(reader, entryBuf)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrRestoreFailed, err)
		}

		if n < entrySize {
			break
		}

		// Read page ID
		pageID := storage.PageID(binary.LittleEndian.Uint64(entryBuf[:pageIDSize]))

		// Deserialize page
		page := &storage.Page{}
		if err := page.Deserialize(entryBuf[pageIDSize:]); err != nil {
			return nil, fmt.Errorf("%w: failed to deserialize page %d: %v", ErrRestoreFailed, pageID, err)
		}

		// Ensure page ID matches
		page.Header.PageID = pageID

		// Write page
		if err := ibm.pageManager.WritePage(page); err != nil {
			return nil, fmt.Errorf("%w: failed to write page %d: %v", ErrRestoreFailed, pageID, err)
		}

		stats.TotalPages++
	}

	// Sync to disk
	if err := ibm.pageManager.Sync(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRestoreFailed, err)
	}

	stats.Duration = time.Since(startTime)

	return stats, nil
}

// readLastBackupLSN reads the last backup LSN from metadata file.
func (ibm *IncrementalBackupManager) readLastBackupLSN() (uint64, error) {
	metadataPath := filepath.Join(ibm.metadataDir, MetadataFileName)

	data, err := os.ReadFile(metadataPath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, ErrMetadataNotFound
		}
		return 0, err
	}

	if len(data) < 8 {
		return 0, ErrInvalidMetadata
	}

	return binary.LittleEndian.Uint64(data[:8]), nil
}

// writeLastBackupLSN writes the last backup LSN to metadata file.
func (ibm *IncrementalBackupManager) writeLastBackupLSN(lsn uint64, backupType, backupPath string) error {
	// Ensure metadata directory exists
	if err := os.MkdirAll(ibm.metadataDir, 0755); err != nil {
		return err
	}

	metadataPath := filepath.Join(ibm.metadataDir, MetadataFileName)

	// Create metadata buffer
	// Layout: LSN (8 bytes) + Timestamp (8 bytes) + Type length (2 bytes) + Type + Path length (2 bytes) + Path
	typeBytes := []byte(backupType)
	pathBytes := []byte(backupPath)
	size := 8 + 8 + 2 + len(typeBytes) + 2 + len(pathBytes)
	data := make([]byte, size)

	offset := 0
	binary.LittleEndian.PutUint64(data[offset:], lsn)
	offset += 8

	binary.LittleEndian.PutUint64(data[offset:], uint64(time.Now().Unix()))
	offset += 8

	binary.LittleEndian.PutUint16(data[offset:], uint16(len(typeBytes)))
	offset += 2
	copy(data[offset:], typeBytes)
	offset += len(typeBytes)

	binary.LittleEndian.PutUint16(data[offset:], uint16(len(pathBytes)))
	offset += 2
	copy(data[offset:], pathBytes)

	return os.WriteFile(metadataPath, data, 0644)
}

// GetLastBackupInfo returns information about the last backup.
func (ibm *IncrementalBackupManager) GetLastBackupInfo() (*BackupMetadata, error) {
	metadataPath := filepath.Join(ibm.metadataDir, MetadataFileName)

	data, err := os.ReadFile(metadataPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrMetadataNotFound
		}
		return nil, err
	}

	if len(data) < 18 { // Minimum: 8 + 8 + 2
		return nil, ErrInvalidMetadata
	}

	metadata := &BackupMetadata{}
	offset := 0

	metadata.LastBackupLSN = binary.LittleEndian.Uint64(data[offset:])
	offset += 8

	metadata.LastBackupTime = int64(binary.LittleEndian.Uint64(data[offset:]))
	offset += 8

	typeLen := binary.LittleEndian.Uint16(data[offset:])
	offset += 2

	if offset+int(typeLen) > len(data) {
		return nil, ErrInvalidMetadata
	}
	metadata.BackupType = string(data[offset : offset+int(typeLen)])
	offset += int(typeLen)

	if offset+2 > len(data) {
		return metadata, nil
	}

	pathLen := binary.LittleEndian.Uint16(data[offset:])
	offset += 2

	if offset+int(pathLen) > len(data) {
		return metadata, nil
	}
	metadata.BackupPath = string(data[offset : offset+int(pathLen)])

	return metadata, nil
}

// RecordFullBackup records a full backup in the metadata for incremental backup chain.
func (ibm *IncrementalBackupManager) RecordFullBackup(lsn uint64, backupPath string) error {
	return ibm.writeLastBackupLSN(lsn, "full", backupPath)
}

// VerifyIncrementalBackup verifies the integrity of an incremental backup file.
func (ibm *IncrementalBackupManager) VerifyIncrementalBackup(path string) error {
	// Open input file
	in, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidBackup, err)
	}
	defer in.Close()

	// Read header
	headerBuf := make([]byte, IncrementalHeaderSize)
	if _, err := io.ReadFull(in, headerBuf); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidBackup, err)
	}

	header := &IncrementalHeader{}
	if err := header.Deserialize(headerBuf); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidBackup, err)
	}

	if err := header.Validate(); err != nil {
		return err
	}

	// Setup reader
	var reader io.Reader
	if header.IsCompressed() {
		reader = NewDecompressReader(in)
	} else {
		reader = in
	}

	// Calculate checksum
	checksumWriter := newChecksumWriter(io.Discard)
	pageSize := int(header.PageSize)
	pageIDSize := 8
	entrySize := pageIDSize + pageSize

	for i := uint64(0); i < header.PageCount; i++ {
		entryBuf := make([]byte, entrySize)
		n, err := io.ReadFull(reader, entryBuf)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
		if err != nil {
			return fmt.Errorf("%w: %v", ErrIncrementalCorrupted, err)
		}
		checksumWriter.Write(entryBuf[:n])
	}

	if checksumWriter.Checksum() != header.Checksum {
		return ErrChecksumMismatch
	}

	return nil
}

// GetIncrementalBackupInfo returns information about an incremental backup file.
func (ibm *IncrementalBackupManager) GetIncrementalBackupInfo(path string) (*IncrementalHeader, error) {
	// Open input file
	in, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidBackup, err)
	}
	defer in.Close()

	// Read header
	headerBuf := make([]byte, IncrementalHeaderSize)
	if _, err := io.ReadFull(in, headerBuf); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidBackup, err)
	}

	header := &IncrementalHeader{}
	if err := header.Deserialize(headerBuf); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidBackup, err)
	}

	if err := header.Validate(); err != nil {
		return nil, err
	}

	return header, nil
}
