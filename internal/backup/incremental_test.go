// Package backup provides backup and restore functionality for ObaDB.
package backup

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/KilimcininKorOglu/oba/internal/storage"
)

// TestIncrementalHeader tests the IncrementalHeader serialization and deserialization.
func TestIncrementalHeader(t *testing.T) {
	t.Run("serialize and deserialize", func(t *testing.T) {
		original := NewIncrementalHeader()
		original.PageSize = 4096
		original.BaseLSN = 100
		original.CurrentLSN = 200
		original.PageCount = 50
		original.TotalBytes = 204800
		original.Checksum = 12345
		original.SetCompressed(true)

		// Serialize
		buf, err := original.Serialize()
		if err != nil {
			t.Fatalf("Serialize() error = %v", err)
		}

		if len(buf) != IncrementalHeaderSize {
			t.Errorf("Serialize() len = %d, want %d", len(buf), IncrementalHeaderSize)
		}

		// Deserialize
		restored := &IncrementalHeader{}
		if err := restored.Deserialize(buf); err != nil {
			t.Fatalf("Deserialize() error = %v", err)
		}

		// Verify fields
		if restored.Magic != original.Magic {
			t.Errorf("Magic = %v, want %v", restored.Magic, original.Magic)
		}
		if restored.Version != original.Version {
			t.Errorf("Version = %d, want %d", restored.Version, original.Version)
		}
		if restored.PageSize != original.PageSize {
			t.Errorf("PageSize = %d, want %d", restored.PageSize, original.PageSize)
		}
		if restored.BaseLSN != original.BaseLSN {
			t.Errorf("BaseLSN = %d, want %d", restored.BaseLSN, original.BaseLSN)
		}
		if restored.CurrentLSN != original.CurrentLSN {
			t.Errorf("CurrentLSN = %d, want %d", restored.CurrentLSN, original.CurrentLSN)
		}
		if restored.PageCount != original.PageCount {
			t.Errorf("PageCount = %d, want %d", restored.PageCount, original.PageCount)
		}
		if restored.TotalBytes != original.TotalBytes {
			t.Errorf("TotalBytes = %d, want %d", restored.TotalBytes, original.TotalBytes)
		}
		if restored.Checksum != original.Checksum {
			t.Errorf("Checksum = %d, want %d", restored.Checksum, original.Checksum)
		}
		if restored.IsCompressed() != original.IsCompressed() {
			t.Errorf("IsCompressed() = %v, want %v", restored.IsCompressed(), original.IsCompressed())
		}
	})

	t.Run("validate magic", func(t *testing.T) {
		header := NewIncrementalHeader()
		if err := header.Validate(); err != nil {
			t.Errorf("Validate() error = %v for valid header", err)
		}

		header.Magic = [4]byte{'X', 'X', 'X', 'X'}
		if err := header.Validate(); err != ErrInvalidIncrementalMagic {
			t.Errorf("Validate() error = %v, want %v", err, ErrInvalidIncrementalMagic)
		}
	})

	t.Run("validate version", func(t *testing.T) {
		header := NewIncrementalHeader()
		header.Version = 0
		if err := header.Validate(); err != ErrUnsupportedFormat {
			t.Errorf("Validate() error = %v, want %v", err, ErrUnsupportedFormat)
		}

		header.Version = BackupVersion + 1
		if err := header.Validate(); err != ErrUnsupportedFormat {
			t.Errorf("Validate() error = %v, want %v", err, ErrUnsupportedFormat)
		}
	})

	t.Run("compression flag", func(t *testing.T) {
		header := NewIncrementalHeader()

		if header.IsCompressed() {
			t.Error("IsCompressed() should be false initially")
		}
		header.SetCompressed(true)
		if !header.IsCompressed() {
			t.Error("IsCompressed() should be true after SetCompressed(true)")
		}
		header.SetCompressed(false)
		if header.IsCompressed() {
			t.Error("IsCompressed() should be false after SetCompressed(false)")
		}
	})
}

// TestIncrementalMagic tests the incremental backup magic number.
func TestIncrementalMagic(t *testing.T) {
	expected := [4]byte{'O', 'B', 'A', 'I'}
	if IncrementalMagic != expected {
		t.Errorf("IncrementalMagic = %v, want %v", IncrementalMagic, expected)
	}
}

// TestBackupMetadata tests the backup metadata read/write operations.
func TestBackupMetadata(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "backup_metadata_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a page manager (needed for IncrementalBackupManager)
	dataPath := filepath.Join(tmpDir, "data.oba")
	pmOpts := storage.Options{
		PageSize:     storage.PageSize,
		InitialPages: 16,
		CreateIfNew:  true,
	}

	pm, err := storage.OpenPageManager(dataPath, pmOpts)
	if err != nil {
		t.Fatalf("Failed to create page manager: %v", err)
	}
	defer pm.Close()

	// Create WAL
	walPath := filepath.Join(tmpDir, "wal.oba")
	wal, err := storage.OpenWAL(walPath)
	if err != nil {
		t.Fatalf("Failed to create WAL: %v", err)
	}
	defer wal.Close()

	// Create incremental backup manager
	ibm := NewIncrementalBackupManager(pm, wal, tmpDir)

	t.Run("write and read metadata", func(t *testing.T) {
		// Write metadata
		testLSN := uint64(12345)
		testType := "full"
		testPath := "/backup/test.oba"

		if err := ibm.writeLastBackupLSN(testLSN, testType, testPath); err != nil {
			t.Fatalf("writeLastBackupLSN() error = %v", err)
		}

		// Read metadata
		lsn, err := ibm.readLastBackupLSN()
		if err != nil {
			t.Fatalf("readLastBackupLSN() error = %v", err)
		}

		if lsn != testLSN {
			t.Errorf("readLastBackupLSN() = %d, want %d", lsn, testLSN)
		}
	})

	t.Run("get last backup info", func(t *testing.T) {
		// Write metadata
		testLSN := uint64(67890)
		testType := "incremental"
		testPath := "/backup/incr.oba"

		if err := ibm.writeLastBackupLSN(testLSN, testType, testPath); err != nil {
			t.Fatalf("writeLastBackupLSN() error = %v", err)
		}

		// Get backup info
		info, err := ibm.GetLastBackupInfo()
		if err != nil {
			t.Fatalf("GetLastBackupInfo() error = %v", err)
		}

		if info.LastBackupLSN != testLSN {
			t.Errorf("LastBackupLSN = %d, want %d", info.LastBackupLSN, testLSN)
		}
		if info.BackupType != testType {
			t.Errorf("BackupType = %s, want %s", info.BackupType, testType)
		}
		if info.BackupPath != testPath {
			t.Errorf("BackupPath = %s, want %s", info.BackupPath, testPath)
		}
		if info.LastBackupTime == 0 {
			t.Error("LastBackupTime should not be 0")
		}
	})

	t.Run("metadata not found", func(t *testing.T) {
		// Create a new manager with different metadata dir
		newMetadataDir := filepath.Join(tmpDir, "nonexistent")
		newIBM := NewIncrementalBackupManager(pm, wal, newMetadataDir)

		_, err := newIBM.readLastBackupLSN()
		if err != ErrMetadataNotFound {
			t.Errorf("readLastBackupLSN() error = %v, want %v", err, ErrMetadataNotFound)
		}
	})
}

// TestIncrementalBackupManager tests the IncrementalBackupManager creation.
func TestIncrementalBackupManager(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "incr_backup_manager_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a page manager
	dataPath := filepath.Join(tmpDir, "data.oba")
	pmOpts := storage.Options{
		PageSize:     storage.PageSize,
		InitialPages: 16,
		CreateIfNew:  true,
	}

	pm, err := storage.OpenPageManager(dataPath, pmOpts)
	if err != nil {
		t.Fatalf("Failed to create page manager: %v", err)
	}
	defer pm.Close()

	// Create WAL
	walPath := filepath.Join(tmpDir, "wal.oba")
	wal, err := storage.OpenWAL(walPath)
	if err != nil {
		t.Fatalf("Failed to create WAL: %v", err)
	}
	defer wal.Close()

	t.Run("create manager", func(t *testing.T) {
		ibm := NewIncrementalBackupManager(pm, wal, tmpDir)
		if ibm == nil {
			t.Fatal("NewIncrementalBackupManager() returned nil")
		}
		if ibm.pageManager != pm {
			t.Error("pageManager not set correctly")
		}
		if ibm.wal != wal {
			t.Error("wal not set correctly")
		}
		if ibm.metadataDir != tmpDir {
			t.Error("metadataDir not set correctly")
		}
	})

	t.Run("nil page manager", func(t *testing.T) {
		ibm := NewIncrementalBackupManager(nil, wal, tmpDir)
		opts := &BackupOptions{
			OutputPath: filepath.Join(tmpDir, "backup.oba"),
			Format:     FormatNative,
		}

		_, err := ibm.IncrementalBackup(opts)
		if err != ErrNilPageManager {
			t.Errorf("IncrementalBackup() error = %v, want %v", err, ErrNilPageManager)
		}
	})

	t.Run("nil WAL", func(t *testing.T) {
		ibm := NewIncrementalBackupManager(pm, nil, tmpDir)
		opts := &BackupOptions{
			OutputPath: filepath.Join(tmpDir, "backup.oba"),
			Format:     FormatNative,
		}

		_, err := ibm.IncrementalBackup(opts)
		if err != ErrWALNotAvailable {
			t.Errorf("IncrementalBackup() error = %v, want %v", err, ErrWALNotAvailable)
		}
	})
}

// TestIncrementalBackup tests the full incremental backup workflow.
func TestIncrementalBackup(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "incr_backup_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a page manager with test data
	dataPath := filepath.Join(tmpDir, "data.oba")
	pmOpts := storage.Options{
		PageSize:     storage.PageSize,
		InitialPages: 16,
		CreateIfNew:  true,
	}

	pm, err := storage.OpenPageManager(dataPath, pmOpts)
	if err != nil {
		t.Fatalf("Failed to create page manager: %v", err)
	}
	defer pm.Close()

	// Create WAL
	walPath := filepath.Join(tmpDir, "wal.oba")
	wal, err := storage.OpenWAL(walPath)
	if err != nil {
		t.Fatalf("Failed to create WAL: %v", err)
	}
	defer wal.Close()

	// Create incremental backup manager
	metadataDir := filepath.Join(tmpDir, "metadata")
	ibm := NewIncrementalBackupManager(pm, wal, metadataDir)

	t.Run("no base backup", func(t *testing.T) {
		backupPath := filepath.Join(tmpDir, "incr_no_base.oba")
		opts := &BackupOptions{
			OutputPath: backupPath,
			Format:     FormatNative,
		}

		_, err := ibm.IncrementalBackup(opts)
		if err == nil {
			t.Error("IncrementalBackup() should fail without base backup")
		}
	})

	t.Run("incremental backup after base", func(t *testing.T) {
		// Record a base backup (simulating a full backup was done)
		baseLSN := wal.CurrentLSN()
		if err := ibm.RecordFullBackup(baseLSN, "/backup/full.oba"); err != nil {
			t.Fatalf("RecordFullBackup() error = %v", err)
		}

		// Allocate some pages and write WAL records
		for i := 0; i < 3; i++ {
			pageID, err := pm.AllocatePage(storage.PageTypeData)
			if err != nil {
				t.Fatalf("Failed to allocate page: %v", err)
			}

			// Write a WAL record for this page
			record := storage.NewWALUpdateRecord(0, 1, pageID, 0, nil, []byte("test data"))
			_, err = wal.Append(record)
			if err != nil {
				t.Fatalf("Failed to append WAL record: %v", err)
			}

			// Write some data to the page
			page, err := pm.ReadPage(pageID)
			if err != nil {
				t.Fatalf("Failed to read page: %v", err)
			}
			copy(page.Data, []byte("Test data for incremental backup"))
			if err := pm.WritePage(page); err != nil {
				t.Fatalf("Failed to write page: %v", err)
			}
		}

		// Sync WAL
		if err := wal.Sync(); err != nil {
			t.Fatalf("Failed to sync WAL: %v", err)
		}

		// Perform incremental backup
		backupPath := filepath.Join(tmpDir, "incr_backup.oba")
		opts := &BackupOptions{
			OutputPath: backupPath,
			Format:     FormatNative,
			Compress:   false,
		}

		stats, err := ibm.IncrementalBackup(opts)
		if err != nil {
			t.Fatalf("IncrementalBackup() error = %v", err)
		}

		if stats.TotalPages == 0 {
			t.Error("IncrementalBackup() TotalPages = 0, want > 0")
		}

		if stats.Duration == 0 {
			t.Error("IncrementalBackup() Duration = 0, want > 0")
		}

		// Verify backup file exists
		if _, err := os.Stat(backupPath); os.IsNotExist(err) {
			t.Error("Incremental backup file was not created")
		}

		// Verify backup header
		header, err := ibm.GetIncrementalBackupInfo(backupPath)
		if err != nil {
			t.Fatalf("GetIncrementalBackupInfo() error = %v", err)
		}

		if header.Magic != IncrementalMagic {
			t.Errorf("Header magic = %v, want %v", header.Magic, IncrementalMagic)
		}

		if header.BaseLSN != baseLSN {
			t.Errorf("Header BaseLSN = %d, want %d", header.BaseLSN, baseLSN)
		}

		if header.PageCount == 0 {
			t.Error("Header PageCount = 0, want > 0")
		}
	})

	t.Run("incremental backup with compression", func(t *testing.T) {
		// Record a new base backup
		baseLSN := wal.CurrentLSN()
		if err := ibm.RecordFullBackup(baseLSN, "/backup/full2.oba"); err != nil {
			t.Fatalf("RecordFullBackup() error = %v", err)
		}

		// Allocate a page and write WAL record
		pageID, err := pm.AllocatePage(storage.PageTypeData)
		if err != nil {
			t.Fatalf("Failed to allocate page: %v", err)
		}

		record := storage.NewWALUpdateRecord(0, 1, pageID, 0, nil, []byte("compressed test"))
		_, err = wal.Append(record)
		if err != nil {
			t.Fatalf("Failed to append WAL record: %v", err)
		}

		// Sync WAL
		if err := wal.Sync(); err != nil {
			t.Fatalf("Failed to sync WAL: %v", err)
		}

		// Perform compressed incremental backup
		backupPath := filepath.Join(tmpDir, "incr_compressed.oba")
		opts := &BackupOptions{
			OutputPath: backupPath,
			Format:     FormatNative,
			Compress:   true,
		}

		stats, err := ibm.IncrementalBackup(opts)
		if err != nil {
			t.Fatalf("IncrementalBackup() error = %v", err)
		}

		// Verify backup header shows compression
		header, err := ibm.GetIncrementalBackupInfo(backupPath)
		if err != nil {
			t.Fatalf("GetIncrementalBackupInfo() error = %v", err)
		}

		if !header.IsCompressed() {
			t.Error("Header should indicate compression")
		}

		// Compressed bytes should be tracked
		if stats.CompressedBytes == 0 && stats.TotalPages > 0 {
			t.Log("Warning: CompressedBytes = 0 with pages backed up")
		}
	})

	t.Run("no changes since last backup", func(t *testing.T) {
		// Record a base backup at current LSN
		currentLSN := wal.CurrentLSN()
		if err := ibm.RecordFullBackup(currentLSN, "/backup/full3.oba"); err != nil {
			t.Fatalf("RecordFullBackup() error = %v", err)
		}

		// Don't make any changes

		// Perform incremental backup
		backupPath := filepath.Join(tmpDir, "incr_no_changes.oba")
		opts := &BackupOptions{
			OutputPath: backupPath,
			Format:     FormatNative,
		}

		stats, err := ibm.IncrementalBackup(opts)
		if err != nil {
			t.Fatalf("IncrementalBackup() error = %v", err)
		}

		// Should have 0 pages since no changes
		if stats.TotalPages != 0 {
			t.Errorf("IncrementalBackup() TotalPages = %d, want 0 (no changes)", stats.TotalPages)
		}
	})
}

// TestIncrementalRestore tests the incremental restore functionality.
func TestIncrementalRestore(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "incr_restore_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create source page manager
	srcDataPath := filepath.Join(tmpDir, "src_data.oba")
	pmOpts := storage.Options{
		PageSize:     storage.PageSize,
		InitialPages: 16,
		CreateIfNew:  true,
	}

	srcPM, err := storage.OpenPageManager(srcDataPath, pmOpts)
	if err != nil {
		t.Fatalf("Failed to create source page manager: %v", err)
	}

	// Create WAL
	walPath := filepath.Join(tmpDir, "wal.oba")
	wal, err := storage.OpenWAL(walPath)
	if err != nil {
		t.Fatalf("Failed to create WAL: %v", err)
	}

	// Create incremental backup manager
	metadataDir := filepath.Join(tmpDir, "metadata")
	ibm := NewIncrementalBackupManager(srcPM, wal, metadataDir)

	// Record base backup
	baseLSN := wal.CurrentLSN()
	if err := ibm.RecordFullBackup(baseLSN, "/backup/full.oba"); err != nil {
		t.Fatalf("RecordFullBackup() error = %v", err)
	}

	// Create test data
	testData := []byte("Test data for restore verification")
	var modifiedPageIDs []storage.PageID

	for i := 0; i < 3; i++ {
		pageID, err := srcPM.AllocatePage(storage.PageTypeData)
		if err != nil {
			t.Fatalf("Failed to allocate page: %v", err)
		}
		modifiedPageIDs = append(modifiedPageIDs, pageID)

		// Write WAL record
		record := storage.NewWALUpdateRecord(0, 1, pageID, 0, nil, testData)
		_, err = wal.Append(record)
		if err != nil {
			t.Fatalf("Failed to append WAL record: %v", err)
		}

		// Write data to page
		page, err := srcPM.ReadPage(pageID)
		if err != nil {
			t.Fatalf("Failed to read page: %v", err)
		}
		copy(page.Data, testData)
		if err := srcPM.WritePage(page); err != nil {
			t.Fatalf("Failed to write page: %v", err)
		}
	}

	// Sync WAL
	if err := wal.Sync(); err != nil {
		t.Fatalf("Failed to sync WAL: %v", err)
	}

	// Create incremental backup
	backupPath := filepath.Join(tmpDir, "incr_backup.oba")
	opts := &BackupOptions{
		OutputPath: backupPath,
		Format:     FormatNative,
	}

	_, err = ibm.IncrementalBackup(opts)
	if err != nil {
		t.Fatalf("IncrementalBackup() error = %v", err)
	}

	// Close source
	srcPM.Close()
	wal.Close()

	// Create destination page manager
	dstDataPath := filepath.Join(tmpDir, "dst_data.oba")
	dstPM, err := storage.OpenPageManager(dstDataPath, pmOpts)
	if err != nil {
		t.Fatalf("Failed to create destination page manager: %v", err)
	}
	defer dstPM.Close()

	// Allocate pages in destination to match source
	for range modifiedPageIDs {
		_, err := dstPM.AllocatePage(storage.PageTypeData)
		if err != nil {
			t.Fatalf("Failed to allocate page in destination: %v", err)
		}
	}

	// Create restore manager
	restoreIBM := NewIncrementalBackupManager(dstPM, nil, metadataDir)

	t.Run("restore incremental backup", func(t *testing.T) {
		restoreOpts := &RestoreOptions{
			InputPath: backupPath,
			Format:    FormatNative,
			Verify:    false,
		}

		stats, err := restoreIBM.IncrementalRestore(restoreOpts)
		if err != nil {
			t.Fatalf("IncrementalRestore() error = %v", err)
		}

		if stats.TotalPages == 0 {
			t.Error("IncrementalRestore() TotalPages = 0, want > 0")
		}
	})

	t.Run("restore with verification", func(t *testing.T) {
		restoreOpts := &RestoreOptions{
			InputPath: backupPath,
			Format:    FormatNative,
			Verify:    true,
		}

		stats, err := restoreIBM.IncrementalRestore(restoreOpts)
		if err != nil {
			t.Fatalf("IncrementalRestore() with verify error = %v", err)
		}

		if stats.TotalPages == 0 {
			t.Error("IncrementalRestore() TotalPages = 0, want > 0")
		}
	})
}

// TestVerifyIncrementalBackup tests the backup verification functionality.
func TestVerifyIncrementalBackup(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "verify_incr_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create page manager
	dataPath := filepath.Join(tmpDir, "data.oba")
	pmOpts := storage.Options{
		PageSize:     storage.PageSize,
		InitialPages: 16,
		CreateIfNew:  true,
	}

	pm, err := storage.OpenPageManager(dataPath, pmOpts)
	if err != nil {
		t.Fatalf("Failed to create page manager: %v", err)
	}
	defer pm.Close()

	// Create WAL
	walPath := filepath.Join(tmpDir, "wal.oba")
	wal, err := storage.OpenWAL(walPath)
	if err != nil {
		t.Fatalf("Failed to create WAL: %v", err)
	}
	defer wal.Close()

	// Create incremental backup manager
	metadataDir := filepath.Join(tmpDir, "metadata")
	ibm := NewIncrementalBackupManager(pm, wal, metadataDir)

	// Record base backup
	baseLSN := wal.CurrentLSN()
	if err := ibm.RecordFullBackup(baseLSN, "/backup/full.oba"); err != nil {
		t.Fatalf("RecordFullBackup() error = %v", err)
	}

	// Create some changes
	pageID, err := pm.AllocatePage(storage.PageTypeData)
	if err != nil {
		t.Fatalf("Failed to allocate page: %v", err)
	}

	record := storage.NewWALUpdateRecord(0, 1, pageID, 0, nil, []byte("verify test"))
	_, err = wal.Append(record)
	if err != nil {
		t.Fatalf("Failed to append WAL record: %v", err)
	}

	if err := wal.Sync(); err != nil {
		t.Fatalf("Failed to sync WAL: %v", err)
	}

	// Create backup
	backupPath := filepath.Join(tmpDir, "verify_backup.oba")
	opts := &BackupOptions{
		OutputPath: backupPath,
		Format:     FormatNative,
	}

	_, err = ibm.IncrementalBackup(opts)
	if err != nil {
		t.Fatalf("IncrementalBackup() error = %v", err)
	}

	t.Run("verify valid backup", func(t *testing.T) {
		if err := ibm.VerifyIncrementalBackup(backupPath); err != nil {
			t.Errorf("VerifyIncrementalBackup() error = %v", err)
		}
	})

	t.Run("verify corrupted backup", func(t *testing.T) {
		// Read backup file
		data, err := os.ReadFile(backupPath)
		if err != nil {
			t.Fatalf("Failed to read backup: %v", err)
		}

		// Corrupt some data (after header)
		if len(data) > IncrementalHeaderSize+100 {
			data[IncrementalHeaderSize+50] ^= 0xFF
		}

		// Write corrupted backup
		corruptedPath := filepath.Join(tmpDir, "corrupted.oba")
		if err := os.WriteFile(corruptedPath, data, 0644); err != nil {
			t.Fatalf("Failed to write corrupted backup: %v", err)
		}

		// Verify should fail
		if err := ibm.VerifyIncrementalBackup(corruptedPath); err == nil {
			t.Error("VerifyIncrementalBackup() should fail for corrupted backup")
		}
	})
}

// TestIncrementalBackupSizeSmallerThanFull tests that incremental backups are smaller.
func TestIncrementalBackupSizeSmallerThanFull(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "size_comparison_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create page manager with many pages
	dataPath := filepath.Join(tmpDir, "data.oba")
	pmOpts := storage.Options{
		PageSize:     storage.PageSize,
		InitialPages: 32,
		CreateIfNew:  true,
	}

	pm, err := storage.OpenPageManager(dataPath, pmOpts)
	if err != nil {
		t.Fatalf("Failed to create page manager: %v", err)
	}
	defer pm.Close()

	// Create WAL
	walPath := filepath.Join(tmpDir, "wal.oba")
	wal, err := storage.OpenWAL(walPath)
	if err != nil {
		t.Fatalf("Failed to create WAL: %v", err)
	}
	defer wal.Close()

	// Allocate many pages (simulating existing data)
	for i := 0; i < 20; i++ {
		pageID, err := pm.AllocatePage(storage.PageTypeData)
		if err != nil {
			t.Fatalf("Failed to allocate page: %v", err)
		}

		page, err := pm.ReadPage(pageID)
		if err != nil {
			t.Fatalf("Failed to read page: %v", err)
		}
		copy(page.Data, []byte("Existing data that should not be in incremental backup"))
		if err := pm.WritePage(page); err != nil {
			t.Fatalf("Failed to write page: %v", err)
		}
	}

	// Create full backup
	fullBM := NewBackupManager(pm)
	fullBackupPath := filepath.Join(tmpDir, "full_backup.oba")
	fullOpts := &BackupOptions{
		OutputPath: fullBackupPath,
		Format:     FormatNative,
	}

	fullStats, err := fullBM.Backup(fullOpts)
	if err != nil {
		t.Fatalf("Full backup error = %v", err)
	}

	// Create incremental backup manager
	metadataDir := filepath.Join(tmpDir, "metadata")
	ibm := NewIncrementalBackupManager(pm, wal, metadataDir)

	// Record base backup at current LSN
	baseLSN := wal.CurrentLSN()
	if err := ibm.RecordFullBackup(baseLSN, fullBackupPath); err != nil {
		t.Fatalf("RecordFullBackup() error = %v", err)
	}

	// Make only a few changes
	for i := 0; i < 3; i++ {
		pageID, err := pm.AllocatePage(storage.PageTypeData)
		if err != nil {
			t.Fatalf("Failed to allocate page: %v", err)
		}

		record := storage.NewWALUpdateRecord(0, 1, pageID, 0, nil, []byte("new data"))
		_, err = wal.Append(record)
		if err != nil {
			t.Fatalf("Failed to append WAL record: %v", err)
		}
	}

	if err := wal.Sync(); err != nil {
		t.Fatalf("Failed to sync WAL: %v", err)
	}

	// Create incremental backup
	incrBackupPath := filepath.Join(tmpDir, "incr_backup.oba")
	incrOpts := &BackupOptions{
		OutputPath: incrBackupPath,
		Format:     FormatNative,
	}

	incrStats, err := ibm.IncrementalBackup(incrOpts)
	if err != nil {
		t.Fatalf("Incremental backup error = %v", err)
	}

	// Get file sizes
	fullInfo, err := os.Stat(fullBackupPath)
	if err != nil {
		t.Fatalf("Failed to stat full backup: %v", err)
	}

	incrInfo, err := os.Stat(incrBackupPath)
	if err != nil {
		t.Fatalf("Failed to stat incremental backup: %v", err)
	}

	t.Logf("Full backup: %d pages, %d bytes", fullStats.TotalPages, fullInfo.Size())
	t.Logf("Incremental backup: %d pages, %d bytes", incrStats.TotalPages, incrInfo.Size())

	// Incremental backup should be smaller
	if incrInfo.Size() >= fullInfo.Size() {
		t.Errorf("Incremental backup (%d bytes) should be smaller than full backup (%d bytes)",
			incrInfo.Size(), fullInfo.Size())
	}

	// Incremental should have fewer pages
	if incrStats.TotalPages >= fullStats.TotalPages {
		t.Errorf("Incremental backup (%d pages) should have fewer pages than full backup (%d pages)",
			incrStats.TotalPages, fullStats.TotalPages)
	}
}

// TestIncrementalBackupChain tests multiple incremental backups in a chain.
func TestIncrementalBackupChain(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "chain_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create page manager
	dataPath := filepath.Join(tmpDir, "data.oba")
	pmOpts := storage.Options{
		PageSize:     storage.PageSize,
		InitialPages: 16,
		CreateIfNew:  true,
	}

	pm, err := storage.OpenPageManager(dataPath, pmOpts)
	if err != nil {
		t.Fatalf("Failed to create page manager: %v", err)
	}
	defer pm.Close()

	// Create WAL
	walPath := filepath.Join(tmpDir, "wal.oba")
	wal, err := storage.OpenWAL(walPath)
	if err != nil {
		t.Fatalf("Failed to create WAL: %v", err)
	}
	defer wal.Close()

	// Create incremental backup manager
	metadataDir := filepath.Join(tmpDir, "metadata")
	ibm := NewIncrementalBackupManager(pm, wal, metadataDir)

	// Record initial base backup
	baseLSN := wal.CurrentLSN()
	if err := ibm.RecordFullBackup(baseLSN, "/backup/full.oba"); err != nil {
		t.Fatalf("RecordFullBackup() error = %v", err)
	}

	// Create chain of incremental backups
	for i := 1; i <= 3; i++ {
		// Make some changes
		pageID, err := pm.AllocatePage(storage.PageTypeData)
		if err != nil {
			t.Fatalf("Failed to allocate page: %v", err)
		}

		record := storage.NewWALUpdateRecord(0, uint64(i), pageID, 0, nil, []byte("chain data"))
		_, err = wal.Append(record)
		if err != nil {
			t.Fatalf("Failed to append WAL record: %v", err)
		}

		if err := wal.Sync(); err != nil {
			t.Fatalf("Failed to sync WAL: %v", err)
		}

		// Create incremental backup
		backupPath := filepath.Join(tmpDir, "incr_"+string(rune('0'+i))+".oba")
		opts := &BackupOptions{
			OutputPath: backupPath,
			Format:     FormatNative,
		}

		stats, err := ibm.IncrementalBackup(opts)
		if err != nil {
			t.Fatalf("IncrementalBackup() %d error = %v", i, err)
		}

		if stats.TotalPages == 0 {
			t.Errorf("IncrementalBackup() %d TotalPages = 0, want > 0", i)
		}

		// Verify the backup info shows correct chain
		info, err := ibm.GetLastBackupInfo()
		if err != nil {
			t.Fatalf("GetLastBackupInfo() error = %v", err)
		}

		if info.BackupType != "incremental" {
			t.Errorf("BackupType = %s, want incremental", info.BackupType)
		}

		if info.BackupPath != backupPath {
			t.Errorf("BackupPath = %s, want %s", info.BackupPath, backupPath)
		}

		// Get header to verify LSN chain
		header, err := ibm.GetIncrementalBackupInfo(backupPath)
		if err != nil {
			t.Fatalf("GetIncrementalBackupInfo() error = %v", err)
		}

		if header.CurrentLSN <= header.BaseLSN {
			t.Errorf("CurrentLSN (%d) should be > BaseLSN (%d)", header.CurrentLSN, header.BaseLSN)
		}
	}
}
