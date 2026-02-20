// Package backup provides backup and restore functionality for ObaDB.
package backup

import (
	"encoding/binary"
	"errors"
	"hash/crc32"
	"io"
	"time"
)

// Backup format constants.
const (
	// BackupMagic is the magic number for ObaDB backup files.
	BackupMagicByte0 = 'O'
	BackupMagicByte1 = 'B'
	BackupMagicByte2 = 'A'
	BackupMagicByte3 = 'B'

	// BackupVersion is the current backup format version.
	BackupVersion uint32 = 1

	// BackupHeaderSize is the size of the backup header in bytes.
	BackupHeaderSize = 64
)

// BackupMagic is the magic number for ObaDB backup files.
var BackupMagic = [4]byte{BackupMagicByte0, BackupMagicByte1, BackupMagicByte2, BackupMagicByte3}

// Backup errors.
var (
	ErrNilEngine         = errors.New("storage engine is nil")
	ErrNilPageManager    = errors.New("page manager is nil")
	ErrBackupFailed      = errors.New("backup failed")
	ErrRestoreFailed     = errors.New("restore failed")
	ErrInvalidBackup     = errors.New("invalid backup file")
	ErrInvalidMagic      = errors.New("invalid backup magic number")
	ErrUnsupportedFormat = errors.New("unsupported backup format")
	ErrChecksumMismatch  = errors.New("backup checksum mismatch")
	ErrOutputPathEmpty   = errors.New("output path is empty")
	ErrInputPathEmpty    = errors.New("input path is empty")
	ErrBackupCorrupted   = errors.New("backup file is corrupted")
	ErrImportFailed      = errors.New("import failed")
	ErrExportFailed      = errors.New("export failed")
)

// BackupFormat represents the backup file format.
type BackupFormat string

const (
	// FormatNative is the native binary backup format.
	FormatNative BackupFormat = "native"
	// FormatLDIF is the LDIF text backup format.
	FormatLDIF BackupFormat = "ldif"
)

// BackupOptions configures the backup operation.
type BackupOptions struct {
	// OutputPath is the path to the backup file.
	OutputPath string

	// Compress enables compression for the backup.
	Compress bool

	// Incremental enables incremental backup (only changes since last backup).
	Incremental bool

	// Format specifies the backup format ("native" or "ldif").
	Format BackupFormat

	// BaseDN is the base DN for LDIF export (optional, defaults to root).
	BaseDN string
}

// Validate validates the backup options.
func (o *BackupOptions) Validate() error {
	if o.OutputPath == "" {
		return ErrOutputPathEmpty
	}

	if o.Format == "" {
		o.Format = FormatNative
	}

	if o.Format != FormatNative && o.Format != FormatLDIF {
		return ErrUnsupportedFormat
	}

	return nil
}

// RestoreOptions configures the restore operation.
type RestoreOptions struct {
	// InputPath is the path to the backup file.
	InputPath string

	// Verify enables checksum verification before restore.
	Verify bool

	// Format specifies the backup format ("native" or "ldif").
	Format BackupFormat

	// DataDir is the target directory for restored data.
	// Used by RestoreManager for specifying the restore destination.
	DataDir string
}

// Validate validates the restore options.
func (o *RestoreOptions) Validate() error {
	if o.InputPath == "" {
		return ErrInputPathEmpty
	}

	if o.Format == "" {
		o.Format = FormatNative
	}

	if o.Format != FormatNative && o.Format != FormatLDIF {
		return ErrUnsupportedFormat
	}

	return nil
}

// BackupHeader represents the header of a native backup file.
// Layout (64 bytes):
//   - Bytes 0-3:   Magic number ("OBAB")
//   - Bytes 4-7:   Version (uint32)
//   - Bytes 8-15:  Timestamp (int64, Unix timestamp)
//   - Bytes 16-19: Flags (uint32)
//   - Bytes 20-23: PageSize (uint32)
//   - Bytes 24-31: TotalPages (uint64)
//   - Bytes 32-39: EntryCount (uint64)
//   - Bytes 40-43: Checksum (uint32, CRC32 of all page data)
//   - Bytes 44-63: Reserved
type BackupHeader struct {
	Magic      [4]byte
	Version    uint32
	Timestamp  int64
	Flags      uint32
	PageSize   uint32
	TotalPages uint64
	EntryCount uint64
	Checksum   uint32
	Reserved   [20]byte
}

// Backup flags.
const (
	// BackupFlagCompressed indicates the backup is compressed.
	BackupFlagCompressed uint32 = 1 << iota
	// BackupFlagIncremental indicates the backup is incremental.
	BackupFlagIncremental
)

// NewBackupHeader creates a new backup header with default values.
func NewBackupHeader() *BackupHeader {
	return &BackupHeader{
		Magic:     BackupMagic,
		Version:   BackupVersion,
		Timestamp: time.Now().Unix(),
		Flags:     0,
	}
}

// IsCompressed returns true if the backup is compressed.
func (h *BackupHeader) IsCompressed() bool {
	return h.Flags&BackupFlagCompressed != 0
}

// SetCompressed sets the compressed flag.
func (h *BackupHeader) SetCompressed(compressed bool) {
	if compressed {
		h.Flags |= BackupFlagCompressed
	} else {
		h.Flags &^= BackupFlagCompressed
	}
}

// IsIncremental returns true if the backup is incremental.
func (h *BackupHeader) IsIncremental() bool {
	return h.Flags&BackupFlagIncremental != 0
}

// SetIncremental sets the incremental flag.
func (h *BackupHeader) SetIncremental(incremental bool) {
	if incremental {
		h.Flags |= BackupFlagIncremental
	} else {
		h.Flags &^= BackupFlagIncremental
	}
}

// Serialize writes the backup header to a byte slice.
func (h *BackupHeader) Serialize() ([]byte, error) {
	buf := make([]byte, BackupHeaderSize)
	return buf, h.SerializeTo(buf)
}

// SerializeTo writes the backup header to an existing byte slice.
func (h *BackupHeader) SerializeTo(buf []byte) error {
	if len(buf) < BackupHeaderSize {
		return ErrInvalidBackup
	}

	// Clear buffer
	for i := range buf[:BackupHeaderSize] {
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

	// Write total pages
	binary.LittleEndian.PutUint64(buf[24:32], h.TotalPages)

	// Write entry count
	binary.LittleEndian.PutUint64(buf[32:40], h.EntryCount)

	// Write checksum
	binary.LittleEndian.PutUint32(buf[40:44], h.Checksum)

	// Reserved bytes are already zeroed

	return nil
}

// Deserialize reads the backup header from a byte slice.
func (h *BackupHeader) Deserialize(buf []byte) error {
	if len(buf) < BackupHeaderSize {
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

	// Read total pages
	h.TotalPages = binary.LittleEndian.Uint64(buf[24:32])

	// Read entry count
	h.EntryCount = binary.LittleEndian.Uint64(buf[32:40])

	// Read checksum
	h.Checksum = binary.LittleEndian.Uint32(buf[40:44])

	// Read reserved
	copy(h.Reserved[:], buf[44:64])

	return nil
}

// Validate validates the backup header.
func (h *BackupHeader) Validate() error {
	if h.Magic != BackupMagic {
		return ErrInvalidMagic
	}

	if h.Version == 0 || h.Version > BackupVersion {
		return ErrUnsupportedFormat
	}

	return nil
}

// BackupStats contains statistics about a backup operation.
type BackupStats struct {
	// TotalPages is the total number of pages backed up.
	TotalPages uint64

	// TotalBytes is the total size of the backup in bytes.
	TotalBytes int64

	// CompressedBytes is the compressed size (if compression enabled).
	CompressedBytes int64

	// Duration is the time taken to complete the backup.
	Duration time.Duration

	// EntryCount is the number of entries backed up.
	EntryCount uint64
}

// CompressionRatio returns the compression ratio (0-1).
// Returns 0 if compression is not enabled or no data was written.
func (s *BackupStats) CompressionRatio() float64 {
	if s.TotalBytes == 0 || s.CompressedBytes == 0 {
		return 0
	}
	return 1.0 - float64(s.CompressedBytes)/float64(s.TotalBytes)
}

// calculateChecksum calculates CRC32 checksum of data.
func calculateChecksum(data []byte) uint32 {
	return crc32.ChecksumIEEE(data)
}

// checksumWriter wraps an io.Writer and calculates checksum of written data.
type checksumWriter struct {
	w        io.Writer
	checksum uint32
	written  int64
}

// newChecksumWriter creates a new checksum writer.
func newChecksumWriter(w io.Writer) *checksumWriter {
	return &checksumWriter{
		w:        w,
		checksum: 0,
		written:  0,
	}
}

// Write writes data and updates the checksum.
func (cw *checksumWriter) Write(p []byte) (n int, err error) {
	n, err = cw.w.Write(p)
	if n > 0 {
		cw.checksum = crc32.Update(cw.checksum, crc32.IEEETable, p[:n])
		cw.written += int64(n)
	}
	return n, err
}

// Checksum returns the current checksum.
func (cw *checksumWriter) Checksum() uint32 {
	return cw.checksum
}

// Written returns the total bytes written.
func (cw *checksumWriter) Written() int64 {
	return cw.written
}
