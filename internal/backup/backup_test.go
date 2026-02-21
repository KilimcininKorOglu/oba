// Package backup provides backup and restore functionality for ObaDB.
package backup

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/KilimcininKorOglu/oba/internal/storage"
)

// TestBackupOptions tests the BackupOptions validation.
func TestBackupOptions(t *testing.T) {
	tests := []struct {
		name    string
		opts    BackupOptions
		wantErr bool
	}{
		{
			name: "valid native backup",
			opts: BackupOptions{
				OutputPath: "/tmp/backup.oba",
				Format:     FormatNative,
			},
			wantErr: false,
		},
		{
			name: "valid ldif backup",
			opts: BackupOptions{
				OutputPath: "/tmp/backup.ldif",
				Format:     FormatLDIF,
			},
			wantErr: false,
		},
		{
			name: "empty output path",
			opts: BackupOptions{
				OutputPath: "",
				Format:     FormatNative,
			},
			wantErr: true,
		},
		{
			name: "invalid format",
			opts: BackupOptions{
				OutputPath: "/tmp/backup.oba",
				Format:     "invalid",
			},
			wantErr: true,
		},
		{
			name: "default format",
			opts: BackupOptions{
				OutputPath: "/tmp/backup.oba",
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

// TestRestoreOptions tests the RestoreOptions validation.
func TestRestoreOptions(t *testing.T) {
	tests := []struct {
		name    string
		opts    RestoreOptions
		wantErr bool
	}{
		{
			name: "valid native restore",
			opts: RestoreOptions{
				InputPath: "/tmp/backup.oba",
				Format:    FormatNative,
			},
			wantErr: false,
		},
		{
			name: "empty input path",
			opts: RestoreOptions{
				InputPath: "",
				Format:    FormatNative,
			},
			wantErr: true,
		},
		{
			name: "invalid format",
			opts: RestoreOptions{
				InputPath: "/tmp/backup.oba",
				Format:    "invalid",
			},
			wantErr: true,
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

// TestBackupHeader tests the BackupHeader serialization and deserialization.
func TestBackupHeader(t *testing.T) {
	t.Run("serialize and deserialize", func(t *testing.T) {
		original := NewBackupHeader()
		original.PageSize = 4096
		original.TotalPages = 100
		original.EntryCount = 50
		original.Checksum = 12345
		original.SetCompressed(true)
		original.SetIncremental(false)

		// Serialize
		buf, err := original.Serialize()
		if err != nil {
			t.Fatalf("Serialize() error = %v", err)
		}

		if len(buf) != BackupHeaderSize {
			t.Errorf("Serialize() len = %d, want %d", len(buf), BackupHeaderSize)
		}

		// Deserialize
		restored := &BackupHeader{}
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
		if restored.TotalPages != original.TotalPages {
			t.Errorf("TotalPages = %d, want %d", restored.TotalPages, original.TotalPages)
		}
		if restored.EntryCount != original.EntryCount {
			t.Errorf("EntryCount = %d, want %d", restored.EntryCount, original.EntryCount)
		}
		if restored.Checksum != original.Checksum {
			t.Errorf("Checksum = %d, want %d", restored.Checksum, original.Checksum)
		}
		if restored.IsCompressed() != original.IsCompressed() {
			t.Errorf("IsCompressed() = %v, want %v", restored.IsCompressed(), original.IsCompressed())
		}
	})

	t.Run("validate magic", func(t *testing.T) {
		header := NewBackupHeader()
		if err := header.Validate(); err != nil {
			t.Errorf("Validate() error = %v for valid header", err)
		}

		header.Magic = [4]byte{'X', 'X', 'X', 'X'}
		if err := header.Validate(); err != ErrInvalidMagic {
			t.Errorf("Validate() error = %v, want %v", err, ErrInvalidMagic)
		}
	})

	t.Run("validate version", func(t *testing.T) {
		header := NewBackupHeader()
		header.Version = 0
		if err := header.Validate(); err != ErrUnsupportedFormat {
			t.Errorf("Validate() error = %v, want %v", err, ErrUnsupportedFormat)
		}

		header.Version = BackupVersion + 1
		if err := header.Validate(); err != ErrUnsupportedFormat {
			t.Errorf("Validate() error = %v, want %v", err, ErrUnsupportedFormat)
		}
	})

	t.Run("flags", func(t *testing.T) {
		header := NewBackupHeader()

		// Test compressed flag
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

		// Test incremental flag
		if header.IsIncremental() {
			t.Error("IsIncremental() should be false initially")
		}
		header.SetIncremental(true)
		if !header.IsIncremental() {
			t.Error("IsIncremental() should be true after SetIncremental(true)")
		}
	})
}

// TestCompression tests the LZ4-style compression.
func TestCompression(t *testing.T) {
	t.Run("compress and decompress", func(t *testing.T) {
		// Create test data with some repetition for compression
		original := make([]byte, 10000)
		for i := range original {
			original[i] = byte(i % 256)
		}
		// Add some repetitive patterns
		for i := 0; i < 1000; i++ {
			copy(original[i*10:], []byte("AAAAAAAAAA"))
		}

		// Compress
		var compressed bytes.Buffer
		cw := NewCompressWriter(&compressed)
		n, err := cw.Write(original)
		if err != nil {
			t.Fatalf("CompressWriter.Write() error = %v", err)
		}
		if n != len(original) {
			t.Errorf("CompressWriter.Write() n = %d, want %d", n, len(original))
		}
		if err := cw.Close(); err != nil {
			t.Fatalf("CompressWriter.Close() error = %v", err)
		}

		// Decompress
		dr := NewDecompressReader(&compressed)
		decompressed := make([]byte, len(original))
		totalRead := 0
		for totalRead < len(original) {
			n, err := dr.Read(decompressed[totalRead:])
			if err != nil {
				t.Fatalf("DecompressReader.Read() error = %v", err)
			}
			totalRead += n
		}

		// Verify
		if !bytes.Equal(original, decompressed) {
			t.Error("Decompressed data does not match original")
		}
	})

	t.Run("compression ratio", func(t *testing.T) {
		// Create highly compressible data
		original := bytes.Repeat([]byte("ABCD"), 10000)

		var compressed bytes.Buffer
		cw := NewCompressWriter(&compressed)
		cw.Write(original)
		cw.Close()

		// Compressed size should be smaller
		if compressed.Len() >= len(original) {
			t.Logf("Warning: Compressed size (%d) >= original size (%d)", compressed.Len(), len(original))
		}
	})

	t.Run("empty data", func(t *testing.T) {
		var compressed bytes.Buffer
		cw := NewCompressWriter(&compressed)
		if err := cw.Close(); err != nil {
			t.Fatalf("CompressWriter.Close() error = %v", err)
		}

		dr := NewDecompressReader(&compressed)
		buf := make([]byte, 100)
		n, err := dr.Read(buf)
		if n != 0 {
			t.Errorf("Read() n = %d, want 0", n)
		}
		if err == nil {
			t.Error("Read() should return EOF for empty data")
		}
	})

	t.Run("small data", func(t *testing.T) {
		original := []byte("Hello, World!")

		var compressed bytes.Buffer
		cw := NewCompressWriter(&compressed)
		cw.Write(original)
		cw.Close()

		dr := NewDecompressReader(&compressed)
		decompressed := make([]byte, len(original))
		n, err := dr.Read(decompressed)
		if err != nil {
			t.Fatalf("DecompressReader.Read() error = %v", err)
		}
		if n != len(original) {
			t.Errorf("Read() n = %d, want %d", n, len(original))
		}
		if !bytes.Equal(original, decompressed[:n]) {
			t.Error("Decompressed data does not match original")
		}
	})
}

// TestBackupStats tests the BackupStats methods.
func TestBackupStats(t *testing.T) {
	t.Run("compression ratio", func(t *testing.T) {
		stats := &BackupStats{
			TotalBytes:      1000,
			CompressedBytes: 500,
		}
		ratio := stats.CompressionRatio()
		if ratio != 0.5 {
			t.Errorf("CompressionRatio() = %f, want 0.5", ratio)
		}
	})

	t.Run("compression ratio zero", func(t *testing.T) {
		stats := &BackupStats{
			TotalBytes:      0,
			CompressedBytes: 0,
		}
		ratio := stats.CompressionRatio()
		if ratio != 0 {
			t.Errorf("CompressionRatio() = %f, want 0", ratio)
		}
	})
}

// TestChecksumWriter tests the checksum writer.
func TestChecksumWriter(t *testing.T) {
	var buf bytes.Buffer
	cw := newChecksumWriter(&buf)

	data := []byte("Hello, World!")
	n, err := cw.Write(data)
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if n != len(data) {
		t.Errorf("Write() n = %d, want %d", n, len(data))
	}

	if cw.Written() != int64(len(data)) {
		t.Errorf("Written() = %d, want %d", cw.Written(), len(data))
	}

	expectedChecksum := calculateChecksum(data)
	if cw.Checksum() != expectedChecksum {
		t.Errorf("Checksum() = %d, want %d", cw.Checksum(), expectedChecksum)
	}
}

// TestFullBackup tests the full backup functionality.
func TestFullBackup(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "backup_test")
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

	// Allocate some pages with data
	for i := 0; i < 5; i++ {
		pageID, err := pm.AllocatePage(storage.PageTypeData)
		if err != nil {
			t.Fatalf("Failed to allocate page: %v", err)
		}

		page, err := pm.ReadPage(pageID)
		if err != nil {
			t.Fatalf("Failed to read page: %v", err)
		}

		// Write some test data
		copy(page.Data, []byte("Test data for page"))
		if err := pm.WritePage(page); err != nil {
			t.Fatalf("Failed to write page: %v", err)
		}
	}

	// Create backup manager
	bm := NewBackupManager(pm)

	t.Run("native backup without compression", func(t *testing.T) {
		backupPath := filepath.Join(tmpDir, "backup.oba")
		opts := &BackupOptions{
			OutputPath: backupPath,
			Format:     FormatNative,
			Compress:   false,
		}

		stats, err := bm.Backup(opts)
		if err != nil {
			t.Fatalf("Backup() error = %v", err)
		}

		if stats.TotalPages == 0 {
			t.Error("Backup() TotalPages = 0, want > 0")
		}

		if stats.Duration == 0 {
			t.Error("Backup() Duration = 0, want > 0")
		}

		// Verify backup file exists
		if _, err := os.Stat(backupPath); os.IsNotExist(err) {
			t.Error("Backup file was not created")
		}

		// Verify backup header
		header, err := bm.GetBackupInfo(backupPath)
		if err != nil {
			t.Fatalf("GetBackupInfo() error = %v", err)
		}

		if header.Magic != BackupMagic {
			t.Errorf("Header magic = %v, want %v", header.Magic, BackupMagic)
		}

		if header.IsCompressed() {
			t.Error("Header should not be compressed")
		}
	})

	t.Run("native backup with compression", func(t *testing.T) {
		backupPath := filepath.Join(tmpDir, "backup_compressed.oba")
		opts := &BackupOptions{
			OutputPath: backupPath,
			Format:     FormatNative,
			Compress:   true,
		}

		stats, err := bm.Backup(opts)
		if err != nil {
			t.Fatalf("Backup() error = %v", err)
		}

		if stats.TotalPages == 0 {
			t.Error("Backup() TotalPages = 0, want > 0")
		}

		// Verify backup header
		header, err := bm.GetBackupInfo(backupPath)
		if err != nil {
			t.Fatalf("GetBackupInfo() error = %v", err)
		}

		if !header.IsCompressed() {
			t.Error("Header should be compressed")
		}
	})

	// Close page manager
	if err := pm.Close(); err != nil {
		t.Fatalf("Failed to close page manager: %v", err)
	}
}

// TestVerifyBackup tests backup verification.
func TestVerifyBackup(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "backup_verify_test")
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
		Compress:   false,
	}

	if _, err := bm.Backup(opts); err != nil {
		t.Fatalf("Backup() error = %v", err)
	}

	// Verify backup
	if err := bm.VerifyBackup(backupPath); err != nil {
		t.Errorf("VerifyBackup() error = %v", err)
	}

	// Close page manager
	pm.Close()

	// Test with corrupted backup
	t.Run("corrupted backup", func(t *testing.T) {
		// Read backup file
		data, err := os.ReadFile(backupPath)
		if err != nil {
			t.Fatalf("Failed to read backup: %v", err)
		}

		// Corrupt some data (after header)
		if len(data) > BackupHeaderSize+100 {
			data[BackupHeaderSize+50] ^= 0xFF
		}

		// Write corrupted backup
		corruptedPath := filepath.Join(tmpDir, "corrupted.oba")
		if err := os.WriteFile(corruptedPath, data, 0644); err != nil {
			t.Fatalf("Failed to write corrupted backup: %v", err)
		}

		// Verify should fail
		if err := bm.VerifyBackup(corruptedPath); err == nil {
			t.Error("VerifyBackup() should fail for corrupted backup")
		}
	})
}

// TestBackupManagerNilPageManager tests error handling for nil page manager.
func TestBackupManagerNilPageManager(t *testing.T) {
	bm := NewBackupManager(nil)

	opts := &BackupOptions{
		OutputPath: "/tmp/backup.oba",
		Format:     FormatNative,
	}

	_, err := bm.Backup(opts)
	if err != ErrNilPageManager {
		t.Errorf("Backup() error = %v, want %v", err, ErrNilPageManager)
	}
}

// TestBackupTimestamp tests that backup timestamp is set correctly.
func TestBackupTimestamp(t *testing.T) {
	before := time.Now().Unix()
	header := NewBackupHeader()
	after := time.Now().Unix()

	if header.Timestamp < before || header.Timestamp > after {
		t.Errorf("Timestamp = %d, want between %d and %d", header.Timestamp, before, after)
	}
}

// TestBackupFormat tests backup format constants.
func TestBackupFormat(t *testing.T) {
	if FormatNative != "native" {
		t.Errorf("FormatNative = %s, want native", FormatNative)
	}
	if FormatLDIF != "ldif" {
		t.Errorf("FormatLDIF = %s, want ldif", FormatLDIF)
	}
}

// TestBackupMagic tests the backup magic number.
func TestBackupMagic(t *testing.T) {
	expected := [4]byte{'O', 'B', 'A', 'B'}
	if BackupMagic != expected {
		t.Errorf("BackupMagic = %v, want %v", BackupMagic, expected)
	}
}
