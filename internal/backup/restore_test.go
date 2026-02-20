// Package backup provides backup and restore functionality for ObaDB.
package backup

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/oba-ldap/oba/internal/storage"
)

// TestRestoreOptionsValidation tests the RestoreOptions validation for RestoreManager.
func TestRestoreOptionsValidation(t *testing.T) {
	tests := []struct {
		name    string
		opts    RestoreOptions
		wantErr bool
	}{
		{
			name: "valid options",
			opts: RestoreOptions{
				InputPath: "/tmp/backup.oba",
				Verify:    true,
				DataDir:   "/tmp/data",
			},
			wantErr: false,
		},
		{
			name: "empty input path",
			opts: RestoreOptions{
				InputPath: "",
				DataDir:   "/tmp/data",
			},
			wantErr: true,
		},
		{
			name: "valid without data dir",
			opts: RestoreOptions{
				InputPath: "/tmp/backup.oba",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.opts.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestRestoreManager tests the RestoreManager creation.
func TestRestoreManager(t *testing.T) {
	t.Run("create manager", func(t *testing.T) {
		rm := NewRestoreManager("/tmp/data")
		if rm == nil {
			t.Fatal("NewRestoreManager() returned nil")
		}
		if rm.DataDir() != "/tmp/data" {
			t.Errorf("DataDir() = %s, want /tmp/data", rm.DataDir())
		}
	})

	t.Run("set data dir", func(t *testing.T) {
		rm := NewRestoreManager("/tmp/data")
		rm.SetDataDir("/new/path")
		if rm.DataDir() != "/new/path" {
			t.Errorf("DataDir() = %s, want /new/path", rm.DataDir())
		}
	})
}

// TestFullBackupRestore tests the full backup restore functionality.
func TestFullBackupRestore(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "restore_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create source page manager with test data
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

	// Create test data
	testData := []byte("Test data for restore verification - this is important data!")
	var createdPageIDs []storage.PageID

	for i := 0; i < 5; i++ {
		pageID, err := srcPM.AllocatePage(storage.PageTypeData)
		if err != nil {
			t.Fatalf("Failed to allocate page: %v", err)
		}
		createdPageIDs = append(createdPageIDs, pageID)

		page, err := srcPM.ReadPage(pageID)
		if err != nil {
			t.Fatalf("Failed to read page: %v", err)
		}
		copy(page.Data, testData)
		if err := srcPM.WritePage(page); err != nil {
			t.Fatalf("Failed to write page: %v", err)
		}
	}

	// Create backup
	bm := NewBackupManager(srcPM)
	backupPath := filepath.Join(tmpDir, "full_backup.oba")
	backupOpts := &BackupOptions{
		OutputPath: backupPath,
		Format:     FormatNative,
		Compress:   false,
	}

	_, err = bm.Backup(backupOpts)
	if err != nil {
		t.Fatalf("Backup() error = %v", err)
	}

	// Close source
	srcPM.Close()

	// Create restore manager
	restoreDir := filepath.Join(tmpDir, "restored")
	rm := NewRestoreManager(restoreDir)

	t.Run("restore full backup", func(t *testing.T) {
		restoreOpts := &RestoreOptions{
			InputPath: backupPath,
			Verify:    false,
			DataDir:   restoreDir,
		}

		stats, err := rm.Restore(restoreOpts)
		if err != nil {
			t.Fatalf("Restore() error = %v", err)
		}

		if stats.TotalPages == 0 {
			t.Error("Restore() TotalPages = 0, want > 0")
		}

		if stats.BackupType != "full" {
			t.Errorf("BackupType = %s, want full", stats.BackupType)
		}

		if stats.Duration == 0 {
			t.Error("Duration = 0, want > 0")
		}

		// Verify restored file exists
		restoredPath := filepath.Join(restoreDir, "data.oba")
		if _, err := os.Stat(restoredPath); os.IsNotExist(err) {
			t.Error("Restored data file was not created")
		}
	})

	t.Run("restore with verification", func(t *testing.T) {
		verifyDir := filepath.Join(tmpDir, "verified")
		restoreOpts := &RestoreOptions{
			InputPath: backupPath,
			Verify:    true,
			DataDir:   verifyDir,
		}

		stats, err := rm.Restore(restoreOpts)
		if err != nil {
			t.Fatalf("Restore() with verify error = %v", err)
		}

		if stats.TotalPages == 0 {
			t.Error("Restore() TotalPages = 0, want > 0")
		}
	})

	t.Run("verify restored data is usable", func(t *testing.T) {
		// Open restored database
		restoredPath := filepath.Join(restoreDir, "data.oba")
		restoredPM, err := storage.OpenPageManager(restoredPath, storage.Options{
			PageSize:    storage.PageSize,
			CreateIfNew: false,
			ReadOnly:    true,
		})
		if err != nil {
			t.Fatalf("Failed to open restored database: %v", err)
		}
		defer restoredPM.Close()

		// Verify we can read pages
		for _, pageID := range createdPageIDs {
			page, err := restoredPM.ReadPage(pageID)
			if err != nil {
				t.Errorf("Failed to read restored page %d: %v", pageID, err)
				continue
			}

			// Verify data matches
			for i := 0; i < len(testData); i++ {
				if page.Data[i] != testData[i] {
					t.Errorf("Page %d data mismatch at byte %d", pageID, i)
					break
				}
			}
		}
	})
}

// TestCompressedBackupRestore tests restore of compressed backups.
func TestCompressedBackupRestore(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "compressed_restore_test")
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

	// Create test data
	for i := 0; i < 3; i++ {
		pageID, err := srcPM.AllocatePage(storage.PageTypeData)
		if err != nil {
			t.Fatalf("Failed to allocate page: %v", err)
		}

		page, err := srcPM.ReadPage(pageID)
		if err != nil {
			t.Fatalf("Failed to read page: %v", err)
		}
		copy(page.Data, []byte("Compressed backup test data"))
		if err := srcPM.WritePage(page); err != nil {
			t.Fatalf("Failed to write page: %v", err)
		}
	}

	// Create compressed backup
	bm := NewBackupManager(srcPM)
	backupPath := filepath.Join(tmpDir, "compressed_backup.oba")
	backupOpts := &BackupOptions{
		OutputPath: backupPath,
		Format:     FormatNative,
		Compress:   true,
	}

	_, err = bm.Backup(backupOpts)
	if err != nil {
		t.Fatalf("Backup() error = %v", err)
	}

	srcPM.Close()

	// Restore compressed backup
	restoreDir := filepath.Join(tmpDir, "restored")
	rm := NewRestoreManager(restoreDir)

	restoreOpts := &RestoreOptions{
		InputPath: backupPath,
		Verify:    true,
		DataDir:   restoreDir,
	}

	stats, err := rm.Restore(restoreOpts)
	if err != nil {
		t.Fatalf("Restore() error = %v", err)
	}

	if stats.TotalPages == 0 {
		t.Error("Restore() TotalPages = 0, want > 0")
	}

	// Verify restored file exists
	restoredPath := filepath.Join(restoreDir, "data.oba")
	if _, err := os.Stat(restoredPath); os.IsNotExist(err) {
		t.Error("Restored data file was not created")
	}
}

// TestIncrementalBackupRestore tests the incremental backup restore functionality.
func TestIncrementalBackupRestore(t *testing.T) {
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

	// Create initial data
	testData := []byte("Initial data for incremental restore test")
	var initialPageIDs []storage.PageID

	for i := 0; i < 3; i++ {
		pageID, err := srcPM.AllocatePage(storage.PageTypeData)
		if err != nil {
			t.Fatalf("Failed to allocate page: %v", err)
		}
		initialPageIDs = append(initialPageIDs, pageID)

		page, err := srcPM.ReadPage(pageID)
		if err != nil {
			t.Fatalf("Failed to read page: %v", err)
		}
		copy(page.Data, testData)
		if err := srcPM.WritePage(page); err != nil {
			t.Fatalf("Failed to write page: %v", err)
		}
	}

	// Create full backup first
	bm := NewBackupManager(srcPM)
	fullBackupPath := filepath.Join(tmpDir, "full_backup.oba")
	fullOpts := &BackupOptions{
		OutputPath: fullBackupPath,
		Format:     FormatNative,
	}

	_, err = bm.Backup(fullOpts)
	if err != nil {
		t.Fatalf("Full backup error = %v", err)
	}

	// Create incremental backup manager and record base
	metadataDir := filepath.Join(tmpDir, "metadata")
	ibm := NewIncrementalBackupManager(srcPM, wal, metadataDir)

	baseLSN := wal.CurrentLSN()
	if err := ibm.RecordFullBackup(baseLSN, fullBackupPath); err != nil {
		t.Fatalf("RecordFullBackup() error = %v", err)
	}

	// Make changes for incremental backup
	modifiedData := []byte("Modified data after full backup")
	var modifiedPageIDs []storage.PageID

	for i := 0; i < 2; i++ {
		pageID, err := srcPM.AllocatePage(storage.PageTypeData)
		if err != nil {
			t.Fatalf("Failed to allocate page: %v", err)
		}
		modifiedPageIDs = append(modifiedPageIDs, pageID)

		// Write WAL record
		record := storage.NewWALUpdateRecord(0, uint64(i+1), pageID, 0, nil, modifiedData)
		_, err = wal.Append(record)
		if err != nil {
			t.Fatalf("Failed to append WAL record: %v", err)
		}

		page, err := srcPM.ReadPage(pageID)
		if err != nil {
			t.Fatalf("Failed to read page: %v", err)
		}
		copy(page.Data, modifiedData)
		if err := srcPM.WritePage(page); err != nil {
			t.Fatalf("Failed to write page: %v", err)
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

	_, err = ibm.IncrementalBackup(incrOpts)
	if err != nil {
		t.Fatalf("IncrementalBackup() error = %v", err)
	}

	// Close source
	srcPM.Close()
	wal.Close()

	// Restore full backup first
	restoreDir := filepath.Join(tmpDir, "restored")
	rm := NewRestoreManager(restoreDir)

	fullRestoreOpts := &RestoreOptions{
		InputPath: fullBackupPath,
		Verify:    false,
		DataDir:   restoreDir,
	}

	_, err = rm.Restore(fullRestoreOpts)
	if err != nil {
		t.Fatalf("Full restore error = %v", err)
	}

	t.Run("restore incremental backup", func(t *testing.T) {
		incrRestoreOpts := &RestoreOptions{
			InputPath: incrBackupPath,
			Verify:    false,
			DataDir:   restoreDir,
		}

		stats, err := rm.Restore(incrRestoreOpts)
		if err != nil {
			t.Fatalf("Incremental restore error = %v", err)
		}

		if stats.BackupType != "incremental" {
			t.Errorf("BackupType = %s, want incremental", stats.BackupType)
		}

		if stats.TotalPages == 0 {
			t.Error("TotalPages = 0, want > 0")
		}
	})

	t.Run("restore incremental with verification", func(t *testing.T) {
		// Create another restore directory
		verifyDir := filepath.Join(tmpDir, "verified")

		// First restore full backup
		fullOpts := &RestoreOptions{
			InputPath: fullBackupPath,
			Verify:    true,
			DataDir:   verifyDir,
		}

		_, err := rm.Restore(fullOpts)
		if err != nil {
			t.Fatalf("Full restore error = %v", err)
		}

		// Then restore incremental with verification
		incrOpts := &RestoreOptions{
			InputPath: incrBackupPath,
			Verify:    true,
			DataDir:   verifyDir,
		}

		stats, err := rm.Restore(incrOpts)
		if err != nil {
			t.Fatalf("Incremental restore with verify error = %v", err)
		}

		if stats.TotalPages == 0 {
			t.Error("TotalPages = 0, want > 0")
		}
	})
}

// TestRestoreChain tests restoring a chain of backups.
func TestRestoreChain(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "chain_restore_test")
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

	// Create initial data
	for i := 0; i < 3; i++ {
		pageID, err := srcPM.AllocatePage(storage.PageTypeData)
		if err != nil {
			t.Fatalf("Failed to allocate page: %v", err)
		}

		page, err := srcPM.ReadPage(pageID)
		if err != nil {
			t.Fatalf("Failed to read page: %v", err)
		}
		copy(page.Data, []byte("Initial chain data"))
		if err := srcPM.WritePage(page); err != nil {
			t.Fatalf("Failed to write page: %v", err)
		}
	}

	// Create full backup
	bm := NewBackupManager(srcPM)
	fullBackupPath := filepath.Join(tmpDir, "full.oba")
	fullOpts := &BackupOptions{
		OutputPath: fullBackupPath,
		Format:     FormatNative,
	}

	_, err = bm.Backup(fullOpts)
	if err != nil {
		t.Fatalf("Full backup error = %v", err)
	}

	// Create incremental backup manager
	metadataDir := filepath.Join(tmpDir, "metadata")
	ibm := NewIncrementalBackupManager(srcPM, wal, metadataDir)

	baseLSN := wal.CurrentLSN()
	if err := ibm.RecordFullBackup(baseLSN, fullBackupPath); err != nil {
		t.Fatalf("RecordFullBackup() error = %v", err)
	}

	// Create multiple incremental backups
	var incrBackups []string
	for i := 1; i <= 3; i++ {
		// Make changes
		pageID, err := srcPM.AllocatePage(storage.PageTypeData)
		if err != nil {
			t.Fatalf("Failed to allocate page: %v", err)
		}

		record := storage.NewWALUpdateRecord(0, uint64(i), pageID, 0, nil, []byte("incr data"))
		_, err = wal.Append(record)
		if err != nil {
			t.Fatalf("Failed to append WAL record: %v", err)
		}

		if err := wal.Sync(); err != nil {
			t.Fatalf("Failed to sync WAL: %v", err)
		}

		// Create incremental backup
		incrPath := filepath.Join(tmpDir, "incr_"+string(rune('0'+i))+".oba")
		incrOpts := &BackupOptions{
			OutputPath: incrPath,
			Format:     FormatNative,
		}

		_, err = ibm.IncrementalBackup(incrOpts)
		if err != nil {
			t.Fatalf("IncrementalBackup() %d error = %v", i, err)
		}

		incrBackups = append(incrBackups, incrPath)
	}

	// Close source
	srcPM.Close()
	wal.Close()

	// Restore chain
	restoreDir := filepath.Join(tmpDir, "restored")
	rm := NewRestoreManager(restoreDir)

	t.Run("restore full chain", func(t *testing.T) {
		backups := append([]string{fullBackupPath}, incrBackups...)
		restoreOpts := &RestoreOptions{
			Verify:  false,
			DataDir: restoreDir,
		}

		stats, err := rm.RestoreChain(backups, restoreOpts)
		if err != nil {
			t.Fatalf("RestoreChain() error = %v", err)
		}

		if stats.BackupsApplied != len(backups) {
			t.Errorf("BackupsApplied = %d, want %d", stats.BackupsApplied, len(backups))
		}

		if stats.BackupType != "chain" {
			t.Errorf("BackupType = %s, want chain", stats.BackupType)
		}
	})

	t.Run("restore partial chain", func(t *testing.T) {
		partialDir := filepath.Join(tmpDir, "partial")
		backups := []string{fullBackupPath, incrBackups[0]}
		restoreOpts := &RestoreOptions{
			Verify:  false,
			DataDir: partialDir,
		}

		stats, err := rm.RestoreChain(backups, restoreOpts)
		if err != nil {
			t.Fatalf("RestoreChain() error = %v", err)
		}

		if stats.BackupsApplied != 2 {
			t.Errorf("BackupsApplied = %d, want 2", stats.BackupsApplied)
		}
	})

	t.Run("empty backup list", func(t *testing.T) {
		restoreOpts := &RestoreOptions{
			DataDir: restoreDir,
		}

		_, err := rm.RestoreChain([]string{}, restoreOpts)
		if err != ErrNoBackupsToRestore {
			t.Errorf("RestoreChain() error = %v, want %v", err, ErrNoBackupsToRestore)
		}
	})
}

// TestGetBackupType tests backup type detection.
func TestGetBackupType(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "backup_type_test")
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

	// Create WAL
	walPath := filepath.Join(tmpDir, "wal.oba")
	wal, err := storage.OpenWAL(walPath)
	if err != nil {
		t.Fatalf("Failed to create WAL: %v", err)
	}

	// Create full backup
	bm := NewBackupManager(pm)
	fullPath := filepath.Join(tmpDir, "full.oba")
	fullOpts := &BackupOptions{
		OutputPath: fullPath,
		Format:     FormatNative,
	}

	_, err = bm.Backup(fullOpts)
	if err != nil {
		t.Fatalf("Full backup error = %v", err)
	}

	// Create incremental backup
	metadataDir := filepath.Join(tmpDir, "metadata")
	ibm := NewIncrementalBackupManager(pm, wal, metadataDir)

	baseLSN := wal.CurrentLSN()
	if err := ibm.RecordFullBackup(baseLSN, fullPath); err != nil {
		t.Fatalf("RecordFullBackup() error = %v", err)
	}

	// Make a change
	pageID, err := pm.AllocatePage(storage.PageTypeData)
	if err != nil {
		t.Fatalf("Failed to allocate page: %v", err)
	}

	record := storage.NewWALUpdateRecord(0, 1, pageID, 0, nil, []byte("test"))
	_, err = wal.Append(record)
	if err != nil {
		t.Fatalf("Failed to append WAL record: %v", err)
	}

	if err := wal.Sync(); err != nil {
		t.Fatalf("Failed to sync WAL: %v", err)
	}

	incrPath := filepath.Join(tmpDir, "incr.oba")
	incrOpts := &BackupOptions{
		OutputPath: incrPath,
		Format:     FormatNative,
	}

	_, err = ibm.IncrementalBackup(incrOpts)
	if err != nil {
		t.Fatalf("IncrementalBackup() error = %v", err)
	}

	pm.Close()
	wal.Close()

	rm := NewRestoreManager(tmpDir)

	t.Run("detect full backup", func(t *testing.T) {
		backupType, err := rm.GetBackupType(fullPath)
		if err != nil {
			t.Fatalf("GetBackupType() error = %v", err)
		}

		if backupType != "full" {
			t.Errorf("GetBackupType() = %s, want full", backupType)
		}
	})

	t.Run("detect incremental backup", func(t *testing.T) {
		backupType, err := rm.GetBackupType(incrPath)
		if err != nil {
			t.Fatalf("GetBackupType() error = %v", err)
		}

		if backupType != "incremental" {
			t.Errorf("GetBackupType() = %s, want incremental", backupType)
		}
	})

	t.Run("unknown backup type", func(t *testing.T) {
		// Create a file with unknown magic
		unknownPath := filepath.Join(tmpDir, "unknown.oba")
		if err := os.WriteFile(unknownPath, []byte("XXXX"), 0644); err != nil {
			t.Fatalf("Failed to create unknown file: %v", err)
		}

		_, err := rm.GetBackupType(unknownPath)
		if err != ErrUnknownBackupType {
			t.Errorf("GetBackupType() error = %v, want %v", err, ErrUnknownBackupType)
		}
	})
}

// TestVerifyBackupViaRestoreManager tests backup verification through RestoreManager.
func TestVerifyBackupViaRestoreManager(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "verify_backup_test")
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

	// Allocate some pages
	for i := 0; i < 3; i++ {
		pm.AllocatePage(storage.PageTypeData)
	}

	// Create backup
	bm := NewBackupManager(pm)
	backupPath := filepath.Join(tmpDir, "backup.oba")
	opts := &BackupOptions{
		OutputPath: backupPath,
		Format:     FormatNative,
	}

	_, err = bm.Backup(opts)
	if err != nil {
		t.Fatalf("Backup() error = %v", err)
	}

	pm.Close()

	rm := NewRestoreManager(tmpDir)

	t.Run("verify valid backup", func(t *testing.T) {
		if err := rm.VerifyBackup(backupPath); err != nil {
			t.Errorf("VerifyBackup() error = %v", err)
		}
	})

	t.Run("verify corrupted backup", func(t *testing.T) {
		// Read backup file
		data, err := os.ReadFile(backupPath)
		if err != nil {
			t.Fatalf("Failed to read backup: %v", err)
		}

		// Corrupt some data
		if len(data) > BackupHeaderSize+100 {
			data[BackupHeaderSize+50] ^= 0xFF
		}

		// Write corrupted backup
		corruptedPath := filepath.Join(tmpDir, "corrupted.oba")
		if err := os.WriteFile(corruptedPath, data, 0644); err != nil {
			t.Fatalf("Failed to write corrupted backup: %v", err)
		}

		// Verify should fail
		if err := rm.VerifyBackup(corruptedPath); err == nil {
			t.Error("VerifyBackup() should fail for corrupted backup")
		}
	})
}

// TestGetBackupInfo tests getting backup information.
func TestGetBackupInfo(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "backup_info_test")
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

	// Create backup
	bm := NewBackupManager(pm)
	backupPath := filepath.Join(tmpDir, "backup.oba")
	opts := &BackupOptions{
		OutputPath: backupPath,
		Format:     FormatNative,
	}

	_, err = bm.Backup(opts)
	if err != nil {
		t.Fatalf("Backup() error = %v", err)
	}

	pm.Close()

	rm := NewRestoreManager(tmpDir)

	t.Run("get full backup info", func(t *testing.T) {
		info, err := rm.GetBackupInfo(backupPath)
		if err != nil {
			t.Fatalf("GetBackupInfo() error = %v", err)
		}

		header, ok := info.(*BackupHeader)
		if !ok {
			t.Fatal("GetBackupInfo() did not return *BackupHeader")
		}

		if header.Magic != BackupMagic {
			t.Errorf("Magic = %v, want %v", header.Magic, BackupMagic)
		}

		if header.Version != BackupVersion {
			t.Errorf("Version = %d, want %d", header.Version, BackupVersion)
		}
	})
}

// TestRestoreCorruptedBackup tests handling of corrupted backups.
func TestRestoreCorruptedBackup(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "corrupted_restore_test")
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

	// Allocate some pages
	for i := 0; i < 3; i++ {
		pm.AllocatePage(storage.PageTypeData)
	}

	// Create backup
	bm := NewBackupManager(pm)
	backupPath := filepath.Join(tmpDir, "backup.oba")
	opts := &BackupOptions{
		OutputPath: backupPath,
		Format:     FormatNative,
	}

	_, err = bm.Backup(opts)
	if err != nil {
		t.Fatalf("Backup() error = %v", err)
	}

	pm.Close()

	// Corrupt the backup
	data, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("Failed to read backup: %v", err)
	}

	if len(data) > BackupHeaderSize+100 {
		data[BackupHeaderSize+50] ^= 0xFF
	}

	corruptedPath := filepath.Join(tmpDir, "corrupted.oba")
	if err := os.WriteFile(corruptedPath, data, 0644); err != nil {
		t.Fatalf("Failed to write corrupted backup: %v", err)
	}

	rm := NewRestoreManager(tmpDir)
	restoreDir := filepath.Join(tmpDir, "restored")

	t.Run("restore corrupted with verification", func(t *testing.T) {
		restoreOpts := &RestoreOptions{
			InputPath: corruptedPath,
			Verify:    true,
			DataDir:   restoreDir,
		}

		_, err := rm.Restore(restoreOpts)
		if err == nil {
			t.Error("Restore() should fail for corrupted backup with verification")
		}
	})
}

// TestCleanDataDir tests the CleanDataDir functionality.
func TestCleanDataDir(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "clean_data_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	rm := NewRestoreManager(tmpDir)

	t.Run("clean existing directory", func(t *testing.T) {
		// Create some .oba files
		for _, name := range []string{"data.oba", "index.oba", "wal.oba"} {
			path := filepath.Join(tmpDir, name)
			if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
				t.Fatalf("Failed to create file: %v", err)
			}
		}

		// Also create a non-.oba file that should not be deleted
		otherPath := filepath.Join(tmpDir, "other.txt")
		if err := os.WriteFile(otherPath, []byte("keep"), 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}

		if err := rm.CleanDataDir(); err != nil {
			t.Fatalf("CleanDataDir() error = %v", err)
		}

		// Verify .oba files are deleted
		for _, name := range []string{"data.oba", "index.oba", "wal.oba"} {
			path := filepath.Join(tmpDir, name)
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Errorf("File %s should be deleted", name)
			}
		}

		// Verify non-.oba file is kept
		if _, err := os.Stat(otherPath); os.IsNotExist(err) {
			t.Error("Non-.oba file should not be deleted")
		}
	})

	t.Run("clean non-existent directory", func(t *testing.T) {
		nonExistentRM := NewRestoreManager("/non/existent/path")
		if err := nonExistentRM.CleanDataDir(); err != nil {
			t.Errorf("CleanDataDir() should not error for non-existent directory: %v", err)
		}
	})

	t.Run("clean empty data dir", func(t *testing.T) {
		emptyRM := NewRestoreManager("")
		if err := emptyRM.CleanDataDir(); err != ErrDataDirEmpty {
			t.Errorf("CleanDataDir() error = %v, want %v", err, ErrDataDirEmpty)
		}
	})
}

// TestRestoreStats tests the RestoreStats structure.
func TestRestoreStats(t *testing.T) {
	stats := &RestoreStats{
		TotalPages:     100,
		TotalBytes:     409600,
		Duration:       time.Second * 5,
		BackupType:     "full",
		BackupsApplied: 1,
	}

	if stats.TotalPages != 100 {
		t.Errorf("TotalPages = %d, want 100", stats.TotalPages)
	}

	if stats.TotalBytes != 409600 {
		t.Errorf("TotalBytes = %d, want 409600", stats.TotalBytes)
	}

	if stats.Duration != time.Second*5 {
		t.Errorf("Duration = %v, want 5s", stats.Duration)
	}

	if stats.BackupType != "full" {
		t.Errorf("BackupType = %s, want full", stats.BackupType)
	}

	if stats.BackupsApplied != 1 {
		t.Errorf("BackupsApplied = %d, want 1", stats.BackupsApplied)
	}
}

// TestRestoreEmptyDataDir tests restore with empty data directory.
func TestRestoreEmptyDataDir(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "empty_datadir_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a backup file
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

	bm := NewBackupManager(pm)
	backupPath := filepath.Join(tmpDir, "backup.oba")
	opts := &BackupOptions{
		OutputPath: backupPath,
		Format:     FormatNative,
	}

	_, err = bm.Backup(opts)
	if err != nil {
		t.Fatalf("Backup() error = %v", err)
	}

	pm.Close()

	// Create restore manager with empty data dir
	rm := NewRestoreManager("")

	restoreOpts := &RestoreOptions{
		InputPath: backupPath,
		DataDir:   "", // Empty
	}

	_, err = rm.Restore(restoreOpts)
	if err != ErrDataDirEmpty {
		t.Errorf("Restore() error = %v, want %v", err, ErrDataDirEmpty)
	}
}

// TestDatabaseUsableAfterRestore tests that the database is usable after restore.
func TestDatabaseUsableAfterRestore(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "usable_after_restore_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create source database with data
	srcDataPath := filepath.Join(tmpDir, "src_data.oba")
	pmOpts := storage.Options{
		PageSize:     storage.PageSize,
		InitialPages: 32,
		CreateIfNew:  true,
	}

	srcPM, err := storage.OpenPageManager(srcDataPath, pmOpts)
	if err != nil {
		t.Fatalf("Failed to create source page manager: %v", err)
	}

	// Create multiple pages with different data
	type pageData struct {
		id   storage.PageID
		data []byte
	}
	var pages []pageData

	for i := 0; i < 10; i++ {
		pageID, err := srcPM.AllocatePage(storage.PageTypeData)
		if err != nil {
			t.Fatalf("Failed to allocate page: %v", err)
		}

		page, err := srcPM.ReadPage(pageID)
		if err != nil {
			t.Fatalf("Failed to read page: %v", err)
		}

		data := []byte("Page data " + string(rune('A'+i)) + " - unique content for verification")
		copy(page.Data, data)
		if err := srcPM.WritePage(page); err != nil {
			t.Fatalf("Failed to write page: %v", err)
		}

		pages = append(pages, pageData{id: pageID, data: data})
	}

	// Create backup
	bm := NewBackupManager(srcPM)
	backupPath := filepath.Join(tmpDir, "backup.oba")
	backupOpts := &BackupOptions{
		OutputPath: backupPath,
		Format:     FormatNative,
	}

	_, err = bm.Backup(backupOpts)
	if err != nil {
		t.Fatalf("Backup() error = %v", err)
	}

	srcPM.Close()

	// Restore to new location
	restoreDir := filepath.Join(tmpDir, "restored")
	rm := NewRestoreManager(restoreDir)

	restoreOpts := &RestoreOptions{
		InputPath: backupPath,
		Verify:    true,
		DataDir:   restoreDir,
	}

	_, err = rm.Restore(restoreOpts)
	if err != nil {
		t.Fatalf("Restore() error = %v", err)
	}

	// Open restored database and verify all data
	restoredPath := filepath.Join(restoreDir, "data.oba")
	restoredPM, err := storage.OpenPageManager(restoredPath, storage.Options{
		PageSize:    storage.PageSize,
		CreateIfNew: false,
	})
	if err != nil {
		t.Fatalf("Failed to open restored database: %v", err)
	}
	defer restoredPM.Close()

	// Verify all pages
	for _, pd := range pages {
		page, err := restoredPM.ReadPage(pd.id)
		if err != nil {
			t.Errorf("Failed to read page %d: %v", pd.id, err)
			continue
		}

		// Verify data matches
		for i := 0; i < len(pd.data); i++ {
			if page.Data[i] != pd.data[i] {
				t.Errorf("Page %d data mismatch at byte %d: got %d, want %d",
					pd.id, i, page.Data[i], pd.data[i])
				break
			}
		}
	}

	// Verify we can allocate new pages (database is writable)
	newPageID, err := restoredPM.AllocatePage(storage.PageTypeData)
	if err != nil {
		t.Errorf("Failed to allocate new page in restored database: %v", err)
	}

	// Verify we can write to the new page
	newPage, err := restoredPM.ReadPage(newPageID)
	if err != nil {
		t.Errorf("Failed to read new page: %v", err)
	}

	copy(newPage.Data, []byte("New data after restore"))
	if err := restoredPM.WritePage(newPage); err != nil {
		t.Errorf("Failed to write new page: %v", err)
	}

	// Sync and verify
	if err := restoredPM.Sync(); err != nil {
		t.Errorf("Failed to sync restored database: %v", err)
	}
}
