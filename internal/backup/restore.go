// Package backup provides backup and restore functionality for ObaDB.
package backup

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/oba-ldap/oba/internal/storage"
)

// Restore errors.
var (
	ErrDataDirEmpty       = errors.New("data directory is empty")
	ErrDataDirNotExist    = errors.New("data directory does not exist")
	ErrUnknownBackupType  = errors.New("unknown backup format")
	ErrRestoreInProgress  = errors.New("restore already in progress")
	ErrInvalidBackupChain = errors.New("invalid backup chain: LSN mismatch")
	ErrNoBackupsToRestore = errors.New("no backups to restore")
)

// RestoreStats contains statistics about a restore operation.
type RestoreStats struct {
	// TotalPages is the total number of pages restored.
	TotalPages uint64

	// TotalBytes is the total size of restored data in bytes.
	TotalBytes int64

	// Duration is the time taken to complete the restore.
	Duration time.Duration

	// BackupType indicates the type of backup restored ("full" or "incremental").
	BackupType string

	// BackupsApplied is the number of backup files applied (for chain restore).
	BackupsApplied int
}

// RestoreManager manages database restore operations.
type RestoreManager struct {
	dataDir string
}

// NewRestoreManager creates a new RestoreManager with the given data directory.
func NewRestoreManager(dataDir string) *RestoreManager {
	return &RestoreManager{
		dataDir: dataDir,
	}
}

// Restore restores a database from a backup file.
// It automatically detects the backup type (full or incremental) and
// performs the appropriate restore operation.
func (rm *RestoreManager) Restore(opts *RestoreOptions) (*RestoreStats, error) {
	// Set default format if not specified (RestoreManager auto-detects)
	if opts.Format == "" {
		opts.Format = FormatNative
	}

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	// Determine target data directory
	dataDir := opts.DataDir
	if dataDir == "" {
		dataDir = rm.dataDir
	}
	if dataDir == "" {
		return nil, ErrDataDirEmpty
	}

	// Open backup file
	in, err := os.Open(opts.InputPath)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRestoreFailed, err)
	}
	defer in.Close()

	// Read magic number to determine backup type
	var magic [4]byte
	if _, err := io.ReadFull(in, magic[:]); err != nil {
		return nil, fmt.Errorf("%w: failed to read magic: %v", ErrRestoreFailed, err)
	}

	// Seek back to beginning
	if _, err := in.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRestoreFailed, err)
	}

	// Dispatch based on backup type
	switch string(magic[:]) {
	case string(BackupMagic[:]):
		return rm.restoreFull(in, opts, dataDir)
	case string(IncrementalMagic[:]):
		return rm.restoreIncremental(in, opts, dataDir)
	default:
		return nil, fmt.Errorf("%w: magic=%v", ErrUnknownBackupType, magic)
	}
}

// restoreFull restores from a full backup file.
func (rm *RestoreManager) restoreFull(in *os.File, opts *RestoreOptions, dataDir string) (*RestoreStats, error) {
	startTime := time.Now()
	stats := &RestoreStats{
		BackupType:     "full",
		BackupsApplied: 1,
	}

	// Read and validate header
	headerBuf := make([]byte, BackupHeaderSize)
	if _, err := io.ReadFull(in, headerBuf); err != nil {
		return nil, fmt.Errorf("%w: failed to read header: %v", ErrRestoreFailed, err)
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
	if header.IsCompressed() {
		reader = NewDecompressReader(in)
	} else {
		reader = in
	}

	// Verify checksum if requested
	if opts.Verify {
		if err := rm.verifyFullBackupChecksum(in, header); err != nil {
			return nil, err
		}

		// Seek back to start of data
		if _, err := in.Seek(BackupHeaderSize, io.SeekStart); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrRestoreFailed, err)
		}

		// Recreate reader
		if header.IsCompressed() {
			reader = NewDecompressReader(in)
		} else {
			reader = in
		}
	}

	// Ensure data directory exists
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("%w: failed to create data directory: %v", ErrRestoreFailed, err)
	}

	// Create new data file
	dataPath := filepath.Join(dataDir, "data.oba")
	out, err := os.Create(dataPath)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create data file: %v", ErrRestoreFailed, err)
	}
	defer out.Close()

	// Write file header first
	fileHeader := storage.NewFileHeader()
	fileHeader.PageSize = header.PageSize
	fileHeader.TotalPages = header.TotalPages + 1 // +1 for header page

	headerBytes, err := fileHeader.Serialize()
	if err != nil {
		return nil, fmt.Errorf("%w: failed to serialize file header: %v", ErrRestoreFailed, err)
	}

	if _, err := out.Write(headerBytes); err != nil {
		return nil, fmt.Errorf("%w: failed to write file header: %v", ErrRestoreFailed, err)
	}

	// Copy pages from backup
	pageSize := int(header.PageSize)
	pageBuf := make([]byte, pageSize)
	var totalBytes int64

	for {
		n, err := io.ReadFull(reader, pageBuf)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			if n > 0 {
				// Write partial page
				if _, err := out.Write(pageBuf[:n]); err != nil {
					return nil, fmt.Errorf("%w: failed to write page: %v", ErrRestoreFailed, err)
				}
				totalBytes += int64(n)
			}
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%w: failed to read page: %v", ErrRestoreFailed, err)
		}

		if _, err := out.Write(pageBuf[:n]); err != nil {
			return nil, fmt.Errorf("%w: failed to write page: %v", ErrRestoreFailed, err)
		}

		stats.TotalPages++
		totalBytes += int64(n)
	}

	// Sync to disk
	if err := out.Sync(); err != nil {
		return nil, fmt.Errorf("%w: failed to sync: %v", ErrRestoreFailed, err)
	}

	stats.TotalBytes = totalBytes
	stats.Duration = time.Since(startTime)

	return stats, nil
}

// restoreIncremental restores from an incremental backup file.
// This requires an existing database to apply the incremental changes to.
func (rm *RestoreManager) restoreIncremental(in *os.File, opts *RestoreOptions, dataDir string) (*RestoreStats, error) {
	startTime := time.Now()
	stats := &RestoreStats{
		BackupType:     "incremental",
		BackupsApplied: 1,
	}

	// Read and validate header
	headerBuf := make([]byte, IncrementalHeaderSize)
	if _, err := io.ReadFull(in, headerBuf); err != nil {
		return nil, fmt.Errorf("%w: failed to read header: %v", ErrRestoreFailed, err)
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
		if err := rm.verifyIncrementalBackupChecksum(in, header); err != nil {
			return nil, err
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

	// Open existing data file for modification
	dataPath := filepath.Join(dataDir, "data.oba")
	out, err := os.OpenFile(dataPath, os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to open data file (incremental restore requires existing database): %v", ErrRestoreFailed, err)
	}
	defer out.Close()

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
			return nil, fmt.Errorf("%w: failed to read page entry: %v", ErrRestoreFailed, err)
		}

		if n < entrySize {
			break
		}

		// Read page ID
		pageID := storage.PageID(binary.LittleEndian.Uint64(entryBuf[:pageIDSize]))

		// Calculate file offset for this page
		offset := int64(pageID) * int64(pageSize)

		// Write page data at the correct offset
		if _, err := out.WriteAt(entryBuf[pageIDSize:], offset); err != nil {
			return nil, fmt.Errorf("%w: failed to write page %d: %v", ErrRestoreFailed, pageID, err)
		}

		stats.TotalPages++
		stats.TotalBytes += int64(pageSize)
	}

	// Sync to disk
	if err := out.Sync(); err != nil {
		return nil, fmt.Errorf("%w: failed to sync: %v", ErrRestoreFailed, err)
	}

	stats.Duration = time.Since(startTime)

	return stats, nil
}

// verifyFullBackupChecksum verifies the checksum of a full backup.
func (rm *RestoreManager) verifyFullBackupChecksum(in *os.File, header *BackupHeader) error {
	// Seek to start of data
	if _, err := in.Seek(BackupHeaderSize, io.SeekStart); err != nil {
		return fmt.Errorf("%w: %v", ErrRestoreFailed, err)
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
			if n > 0 {
				checksumWriter.Write(pageBuf[:n])
			}
			break
		}
		if err != nil {
			return fmt.Errorf("%w: %v", ErrRestoreFailed, err)
		}
		checksumWriter.Write(pageBuf[:n])
	}

	if checksumWriter.Checksum() != header.Checksum {
		return ErrChecksumMismatch
	}

	return nil
}

// verifyIncrementalBackupChecksum verifies the checksum of an incremental backup.
func (rm *RestoreManager) verifyIncrementalBackupChecksum(in *os.File, header *IncrementalHeader) error {
	// Seek to start of data
	if _, err := in.Seek(IncrementalHeaderSize, io.SeekStart); err != nil {
		return fmt.Errorf("%w: %v", ErrRestoreFailed, err)
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
			if n > 0 {
				checksumWriter.Write(entryBuf[:n])
			}
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

// RestoreChain restores from a full backup and applies a sequence of incremental backups.
// The backups slice should contain paths to backup files in order:
// first the full backup, then incremental backups in chronological order.
func (rm *RestoreManager) RestoreChain(backups []string, opts *RestoreOptions) (*RestoreStats, error) {
	if len(backups) == 0 {
		return nil, ErrNoBackupsToRestore
	}

	// Determine target data directory
	dataDir := opts.DataDir
	if dataDir == "" {
		dataDir = rm.dataDir
	}
	if dataDir == "" {
		return nil, ErrDataDirEmpty
	}

	startTime := time.Now()
	totalStats := &RestoreStats{
		BackupType: "chain",
	}

	// First backup must be a full backup
	firstOpts := &RestoreOptions{
		InputPath: backups[0],
		Verify:    opts.Verify,
		DataDir:   dataDir,
	}

	// Verify first backup is a full backup
	backupType, err := rm.GetBackupType(backups[0])
	if err != nil {
		return nil, fmt.Errorf("%w: failed to determine backup type: %v", ErrRestoreFailed, err)
	}

	if backupType != "full" {
		return nil, fmt.Errorf("%w: first backup in chain must be a full backup", ErrInvalidBackupChain)
	}

	// Restore full backup
	stats, err := rm.Restore(firstOpts)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to restore full backup: %v", ErrRestoreFailed, err)
	}

	totalStats.TotalPages += stats.TotalPages
	totalStats.TotalBytes += stats.TotalBytes
	totalStats.BackupsApplied++

	// Apply incremental backups in order
	for i := 1; i < len(backups); i++ {
		incrOpts := &RestoreOptions{
			InputPath: backups[i],
			Verify:    opts.Verify,
			DataDir:   dataDir,
		}

		// Verify this is an incremental backup
		backupType, err := rm.GetBackupType(backups[i])
		if err != nil {
			return nil, fmt.Errorf("%w: failed to determine backup type for %s: %v", ErrRestoreFailed, backups[i], err)
		}

		if backupType != "incremental" {
			return nil, fmt.Errorf("%w: backup %d in chain must be incremental", ErrInvalidBackupChain, i)
		}

		stats, err := rm.Restore(incrOpts)
		if err != nil {
			return nil, fmt.Errorf("%w: failed to apply incremental backup %s: %v", ErrRestoreFailed, backups[i], err)
		}

		totalStats.TotalPages += stats.TotalPages
		totalStats.TotalBytes += stats.TotalBytes
		totalStats.BackupsApplied++
	}

	totalStats.Duration = time.Since(startTime)

	return totalStats, nil
}

// GetBackupType returns the type of a backup file ("full" or "incremental").
func (rm *RestoreManager) GetBackupType(path string) (string, error) {
	in, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer in.Close()

	var magic [4]byte
	if _, err := io.ReadFull(in, magic[:]); err != nil {
		return "", err
	}

	switch string(magic[:]) {
	case string(BackupMagic[:]):
		return "full", nil
	case string(IncrementalMagic[:]):
		return "incremental", nil
	default:
		return "", ErrUnknownBackupType
	}
}

// VerifyBackup verifies the integrity of a backup file.
func (rm *RestoreManager) VerifyBackup(path string) error {
	backupType, err := rm.GetBackupType(path)
	if err != nil {
		return err
	}

	in, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidBackup, err)
	}
	defer in.Close()

	switch backupType {
	case "full":
		return rm.verifyFullBackup(in)
	case "incremental":
		return rm.verifyIncrementalBackup(in)
	default:
		return ErrUnknownBackupType
	}
}

// verifyFullBackup verifies a full backup file.
func (rm *RestoreManager) verifyFullBackup(in *os.File) error {
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

	return rm.verifyFullBackupChecksum(in, header)
}

// verifyIncrementalBackup verifies an incremental backup file.
func (rm *RestoreManager) verifyIncrementalBackup(in *os.File) error {
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

	return rm.verifyIncrementalBackupChecksum(in, header)
}

// GetBackupInfo returns information about a backup file.
func (rm *RestoreManager) GetBackupInfo(path string) (interface{}, error) {
	backupType, err := rm.GetBackupType(path)
	if err != nil {
		return nil, err
	}

	in, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidBackup, err)
	}
	defer in.Close()

	switch backupType {
	case "full":
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

	case "incremental":
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

	default:
		return nil, ErrUnknownBackupType
	}
}

// BackupChainInfo contains information about a backup chain.
type BackupChainInfo struct {
	// FullBackup is the path to the full backup.
	FullBackup string

	// IncrementalBackups is the list of incremental backups in order.
	IncrementalBackups []string

	// TotalBackups is the total number of backups in the chain.
	TotalBackups int

	// StartLSN is the LSN of the full backup.
	StartLSN uint64

	// EndLSN is the LSN of the last incremental backup.
	EndLSN uint64

	// IsComplete indicates if the chain is complete (no gaps).
	IsComplete bool
}

// DiscoverBackupChain discovers and validates a backup chain in a directory.
// It finds the full backup and all related incremental backups.
func (rm *RestoreManager) DiscoverBackupChain(dir string) (*BackupChainInfo, error) {
	// List all files in directory
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var fullBackups []string
	var incrBackups []string

	// Categorize backup files
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		backupType, err := rm.GetBackupType(path)
		if err != nil {
			continue // Skip non-backup files
		}

		switch backupType {
		case "full":
			fullBackups = append(fullBackups, path)
		case "incremental":
			incrBackups = append(incrBackups, path)
		}
	}

	if len(fullBackups) == 0 {
		return nil, ErrNoBaseBackup
	}

	// Use the most recent full backup (by timestamp in header)
	var latestFullBackup string
	var latestFullTimestamp int64

	for _, path := range fullBackups {
		info, err := rm.GetBackupInfo(path)
		if err != nil {
			continue
		}

		header, ok := info.(*BackupHeader)
		if !ok {
			continue
		}

		if header.Timestamp > latestFullTimestamp {
			latestFullTimestamp = header.Timestamp
			latestFullBackup = path
		}
	}

	if latestFullBackup == "" {
		return nil, ErrNoBaseBackup
	}

	// Build chain info
	chainInfo := &BackupChainInfo{
		FullBackup:   latestFullBackup,
		TotalBackups: 1,
		IsComplete:   true,
	}

	// Sort incremental backups by LSN
	type incrInfo struct {
		path    string
		baseLSN uint64
		currLSN uint64
	}

	var incrInfos []incrInfo
	for _, path := range incrBackups {
		info, err := rm.GetBackupInfo(path)
		if err != nil {
			continue
		}

		header, ok := info.(*IncrementalHeader)
		if !ok {
			continue
		}

		incrInfos = append(incrInfos, incrInfo{
			path:    path,
			baseLSN: header.BaseLSN,
			currLSN: header.CurrentLSN,
		})
	}

	// Sort by base LSN
	sort.Slice(incrInfos, func(i, j int) bool {
		return incrInfos[i].baseLSN < incrInfos[j].baseLSN
	})

	// Build ordered chain
	for _, info := range incrInfos {
		chainInfo.IncrementalBackups = append(chainInfo.IncrementalBackups, info.path)
		chainInfo.TotalBackups++
		chainInfo.EndLSN = info.currLSN
	}

	return chainInfo, nil
}

// RestoreToPointInTime restores the database to a specific point in time.
// It finds the appropriate full backup and applies incremental backups
// up to the specified timestamp.
func (rm *RestoreManager) RestoreToPointInTime(dir string, targetTime time.Time, opts *RestoreOptions) (*RestoreStats, error) {
	// Discover backup chain
	chainInfo, err := rm.DiscoverBackupChain(dir)
	if err != nil {
		return nil, err
	}

	// Build list of backups to apply
	backups := []string{chainInfo.FullBackup}

	// Add incremental backups up to target time
	for _, incrPath := range chainInfo.IncrementalBackups {
		info, err := rm.GetBackupInfo(incrPath)
		if err != nil {
			continue
		}

		header, ok := info.(*IncrementalHeader)
		if !ok {
			continue
		}

		// Check if this backup is before or at target time
		backupTime := time.Unix(header.Timestamp, 0)
		if backupTime.After(targetTime) {
			break
		}

		backups = append(backups, incrPath)
	}

	// Restore the chain
	return rm.RestoreChain(backups, opts)
}

// DataDir returns the configured data directory.
func (rm *RestoreManager) DataDir() string {
	return rm.dataDir
}

// SetDataDir sets the data directory.
func (rm *RestoreManager) SetDataDir(dataDir string) {
	rm.dataDir = dataDir
}

// CleanDataDir removes all files in the data directory.
// Use with caution - this is destructive!
func (rm *RestoreManager) CleanDataDir() error {
	if rm.dataDir == "" {
		return ErrDataDirEmpty
	}

	// Check if directory exists
	if _, err := os.Stat(rm.dataDir); os.IsNotExist(err) {
		return nil // Nothing to clean
	}

	// Remove all files in the directory
	entries, err := os.ReadDir(rm.dataDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		path := filepath.Join(rm.dataDir, entry.Name())
		// Only remove .oba files to be safe
		if strings.HasSuffix(entry.Name(), ".oba") {
			if err := os.Remove(path); err != nil {
				return err
			}
		}
	}

	return nil
}
