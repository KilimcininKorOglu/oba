// Package backup provides backup and restore functionality for ObaDB.
package backup

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/KilimcininKorOglu/oba/internal/storage"
)

// BackupManager manages backup and restore operations for ObaDB.
type BackupManager struct {
	pageManager *storage.PageManager
	engine      storage.StorageEngine
}

// NewBackupManager creates a new BackupManager with the given page manager.
func NewBackupManager(pageManager *storage.PageManager) *BackupManager {
	return &BackupManager{
		pageManager: pageManager,
	}
}

// NewBackupManagerWithEngine creates a new BackupManager with a storage engine.
// This is useful for LDIF exports that need access to entries.
func NewBackupManagerWithEngine(pageManager *storage.PageManager, engine storage.StorageEngine) *BackupManager {
	return &BackupManager{
		pageManager: pageManager,
		engine:      engine,
	}
}

// Backup performs a backup operation based on the provided options.
func (bm *BackupManager) Backup(opts *BackupOptions) (*BackupStats, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	// If DataDir is specified, use directory backup
	if opts.DataDir != "" {
		return bm.directoryBackup(opts)
	}

	switch opts.Format {
	case FormatNative:
		return bm.fullBackup(opts)
	case FormatLDIF:
		return bm.ldifBackup(opts)
	default:
		return nil, ErrUnsupportedFormat
	}
}

// fullBackup creates a full native backup of the database.
// It creates a consistent snapshot using MVCC to ensure the backup
// doesn't block ongoing operations.
func (bm *BackupManager) fullBackup(opts *BackupOptions) (*BackupStats, error) {
	if bm.pageManager == nil {
		return nil, ErrNilPageManager
	}

	startTime := time.Now()
	stats := &BackupStats{}

	// Open output file
	out, err := os.Create(opts.OutputPath)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBackupFailed, err)
	}
	defer out.Close()

	// Get page manager stats for consistent snapshot
	pmStats := bm.pageManager.Stats()
	totalPages := pmStats.TotalPages
	pageSize := bm.pageManager.PageSize()

	// Create backup header
	header := NewBackupHeader()
	header.PageSize = uint32(pageSize)
	header.TotalPages = totalPages
	header.SetCompressed(opts.Compress)
	header.SetIncremental(opts.Incremental)

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

	// Copy all pages (starting from page 1, page 0 is header)
	for pageID := storage.PageID(1); pageID < storage.PageID(totalPages); pageID++ {
		page, err := bm.pageManager.ReadPage(pageID)
		if err != nil {
			// Skip pages that can't be read (might be free pages)
			continue
		}

		// Serialize page
		pageData, err := page.Serialize()
		if err != nil {
			return nil, fmt.Errorf("%w: failed to serialize page %d: %v", ErrBackupFailed, pageID, err)
		}

		// Write page data
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
	header.EntryCount = stats.EntryCount

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

	// Get final file size
	fileInfo, err := out.Stat()
	if err == nil {
		stats.TotalBytes = fileInfo.Size()
	}

	stats.Duration = time.Since(startTime)

	return stats, nil
}

// ldifBackup creates an LDIF format backup.
func (bm *BackupManager) ldifBackup(opts *BackupOptions) (*BackupStats, error) {
	if bm.engine == nil {
		return nil, ErrNilEngine
	}

	startTime := time.Now()
	stats := &BackupStats{}

	// Open output file
	out, err := os.Create(opts.OutputPath)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBackupFailed, err)
	}
	defer out.Close()

	// Setup writer (with optional compression)
	var writer io.Writer
	var compressWriter *CompressWriter

	if opts.Compress {
		compressWriter = NewCompressWriter(out)
		writer = compressWriter
	} else {
		writer = out
	}

	// Use LDIF exporter
	exporter := NewLDIFExporter(bm.engine)
	baseDN := opts.BaseDN
	if baseDN == "" {
		baseDN = "" // Export all entries
	}

	if err := exporter.Export(writer, baseDN); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBackupFailed, err)
	}

	// Close compression writer if used
	if compressWriter != nil {
		if err := compressWriter.Close(); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrBackupFailed, err)
		}
		stats.CompressedBytes = compressWriter.Written()
	}

	// Sync to disk
	if err := out.Sync(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBackupFailed, err)
	}

	// Get final file size
	fileInfo, err := out.Stat()
	if err == nil {
		stats.TotalBytes = fileInfo.Size()
	}

	stats.Duration = time.Since(startTime)

	return stats, nil
}

// Restore restores a database from a backup file.
func (bm *BackupManager) Restore(opts *RestoreOptions) (*BackupStats, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	switch opts.Format {
	case FormatNative:
		return bm.fullRestore(opts)
	case FormatLDIF:
		return bm.ldifRestore(opts)
	default:
		return nil, ErrUnsupportedFormat
	}
}

// fullRestore restores from a native backup file.
func (bm *BackupManager) fullRestore(opts *RestoreOptions) (*BackupStats, error) {
	if bm.pageManager == nil {
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
	headerBuf := make([]byte, BackupHeaderSize)
	if _, err := io.ReadFull(in, headerBuf); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRestoreFailed, err)
	}

	header := &BackupHeader{}
	if err := header.Deserialize(headerBuf); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRestoreFailed, err)
	}

	if err := header.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRestoreFailed, err)
	}

	// Setup reader (with optional decompression)
	var reader io.Reader
	var decompressReader *DecompressReader

	if header.IsCompressed() {
		decompressReader = NewDecompressReader(in)
		reader = decompressReader
	} else {
		reader = in
	}

	// Verify checksum if requested
	if opts.Verify {
		checksumWriter := newChecksumWriter(io.Discard)
		pageSize := int(header.PageSize)
		pageBuf := make([]byte, pageSize)

		for {
			n, err := io.ReadFull(reader, pageBuf)
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("%w: %v", ErrRestoreFailed, err)
			}
			checksumWriter.Write(pageBuf[:n])
		}

		if checksumWriter.Checksum() != header.Checksum {
			return nil, ErrChecksumMismatch
		}

		// Seek back to start of data
		if _, err := in.Seek(BackupHeaderSize, io.SeekStart); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrRestoreFailed, err)
		}

		// Recreate reader
		if header.IsCompressed() {
			decompressReader = NewDecompressReader(in)
			reader = decompressReader
		} else {
			reader = in
		}
	}

	// Restore pages
	pageSize := int(header.PageSize)
	pageBuf := make([]byte, pageSize)
	pageID := storage.PageID(1) // Start from page 1

	for {
		n, err := io.ReadFull(reader, pageBuf)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrRestoreFailed, err)
		}

		if n < pageSize {
			break
		}

		// Deserialize page
		page := &storage.Page{}
		if err := page.Deserialize(pageBuf); err != nil {
			return nil, fmt.Errorf("%w: failed to deserialize page %d: %v", ErrRestoreFailed, pageID, err)
		}

		// Write page
		if err := bm.pageManager.WritePage(page); err != nil {
			return nil, fmt.Errorf("%w: failed to write page %d: %v", ErrRestoreFailed, pageID, err)
		}

		stats.TotalPages++
		pageID++
	}

	// Sync to disk
	if err := bm.pageManager.Sync(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRestoreFailed, err)
	}

	stats.Duration = time.Since(startTime)
	stats.EntryCount = header.EntryCount

	return stats, nil
}

// ldifRestore restores from an LDIF backup file.
func (bm *BackupManager) ldifRestore(opts *RestoreOptions) (*BackupStats, error) {
	if bm.engine == nil {
		return nil, ErrNilEngine
	}

	startTime := time.Now()
	stats := &BackupStats{}

	// Open input file
	in, err := os.Open(opts.InputPath)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRestoreFailed, err)
	}
	defer in.Close()

	// Setup reader (check for compression by trying to read header)
	var reader io.Reader

	// Try to detect compression by reading first 8 bytes
	headerBuf := make([]byte, 8)
	n, err := in.Read(headerBuf)
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("%w: %v", ErrRestoreFailed, err)
	}

	// Seek back to start
	if _, err := in.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRestoreFailed, err)
	}

	// Check if it looks like compressed data (has block header)
	if n >= 8 {
		// Check if first bytes look like LDIF (starts with "dn:" or "#")
		if headerBuf[0] == 'd' || headerBuf[0] == '#' || headerBuf[0] == '\n' {
			reader = in
		} else {
			// Assume compressed
			reader = NewDecompressReader(in)
		}
	} else {
		reader = in
	}

	// Use LDIF importer
	importer := NewLDIFImporter(bm.engine)
	if err := importer.ImportBatch(reader); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRestoreFailed, err)
	}

	stats.Duration = time.Since(startTime)

	return stats, nil
}

// VerifyBackup verifies the integrity of a backup file.
func (bm *BackupManager) VerifyBackup(path string) error {
	// Open input file
	in, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidBackup, err)
	}
	defer in.Close()

	// Read header
	headerBuf := make([]byte, BackupHeaderSize)
	if _, err := io.ReadFull(in, headerBuf); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidBackup, err)
	}

	header := &BackupHeader{}
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
	pageBuf := make([]byte, pageSize)

	for {
		n, err := io.ReadFull(reader, pageBuf)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
		if err != nil {
			return fmt.Errorf("%w: %v", ErrBackupCorrupted, err)
		}
		checksumWriter.Write(pageBuf[:n])
	}

	if checksumWriter.Checksum() != header.Checksum {
		return ErrChecksumMismatch
	}

	return nil
}

// GetBackupInfo returns information about a backup file.
func (bm *BackupManager) GetBackupInfo(path string) (*BackupHeader, error) {
	// Open input file
	in, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidBackup, err)
	}
	defer in.Close()

	// Read header
	headerBuf := make([]byte, BackupHeaderSize)
	if _, err := io.ReadFull(in, headerBuf); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidBackup, err)
	}

	header := &BackupHeader{}
	if err := header.Deserialize(headerBuf); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidBackup, err)
	}

	if err := header.Validate(); err != nil {
		return nil, err
	}

	return header, nil
}

// directoryBackup creates a backup of all storage files in the data directory.
// This includes data.oba, index.oba, and wal.oba files.
func (bm *BackupManager) directoryBackup(opts *BackupOptions) (*BackupStats, error) {
	startTime := time.Now()
	stats := &BackupStats{}

	// Open output file
	out, err := os.Create(opts.OutputPath)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBackupFailed, err)
	}
	defer out.Close()

	// Create backup header
	header := NewBackupHeader()
	header.SetCompressed(opts.Compress)
	header.SetMultiFile(true)

	// Write placeholder header
	headerBuf, err := header.Serialize()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBackupFailed, err)
	}
	if _, err := out.Write(headerBuf); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBackupFailed, err)
	}

	// Setup writer
	var writer io.Writer
	var compressWriter *CompressWriter
	checksumWriter := newChecksumWriter(out)

	if opts.Compress {
		compressWriter = NewCompressWriter(checksumWriter)
		writer = compressWriter
	} else {
		writer = checksumWriter
	}

	// Count files to backup
	var fileCount uint32
	for _, fileName := range StorageFiles {
		filePath := filepath.Join(opts.DataDir, fileName)
		if _, err := os.Stat(filePath); err == nil {
			fileCount++
		}
	}

	// Write file count
	if err := binary.Write(writer, binary.LittleEndian, fileCount); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBackupFailed, err)
	}

	// Backup each storage file
	for _, fileName := range StorageFiles {
		filePath := filepath.Join(opts.DataDir, fileName)

		// Check if file exists
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("%w: %v", ErrBackupFailed, err)
		}

		// Write file name length and name
		nameBytes := []byte(fileName)
		if err := binary.Write(writer, binary.LittleEndian, uint16(len(nameBytes))); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrBackupFailed, err)
		}
		if _, err := writer.Write(nameBytes); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrBackupFailed, err)
		}

		// Write file size
		fileSize := fileInfo.Size()
		if err := binary.Write(writer, binary.LittleEndian, fileSize); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrBackupFailed, err)
		}

		// Copy file content
		f, err := os.Open(filePath)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrBackupFailed, err)
		}

		written, err := io.Copy(writer, f)
		f.Close()
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrBackupFailed, err)
		}

		stats.TotalBytes += written
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
	header.TotalPages = uint64(fileCount)

	// Seek back and write final header
	if _, err := out.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBackupFailed, err)
	}
	headerBuf, _ = header.Serialize()
	if _, err := out.Write(headerBuf); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBackupFailed, err)
	}

	stats.Duration = time.Since(startTime)
	return stats, nil
}
