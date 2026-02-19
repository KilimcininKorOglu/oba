package storage

import (
	"bytes"
	"testing"
)

// =============================================================================
// FileHeader Tests
// =============================================================================

func TestNewFileHeader(t *testing.T) {
	header := NewFileHeader()

	if header.Magic != Magic {
		t.Errorf("Magic = %v, want %v", header.Magic, Magic)
	}
	if header.Version != CurrentVersion {
		t.Errorf("Version = %v, want %v", header.Version, CurrentVersion)
	}
	if header.PageSize != PageSize {
		t.Errorf("PageSize = %v, want %v", header.PageSize, PageSize)
	}
	if header.TotalPages != 1 {
		t.Errorf("TotalPages = %v, want 1", header.TotalPages)
	}
	if header.FreeListHead != 0 {
		t.Errorf("FreeListHead = %v, want 0", header.FreeListHead)
	}
	if header.RootPages.DNIndex != 0 {
		t.Errorf("RootPages.DNIndex = %v, want 0", header.RootPages.DNIndex)
	}
	if header.RootPages.DataRoot != 0 {
		t.Errorf("RootPages.DataRoot = %v, want 0", header.RootPages.DataRoot)
	}
}

func TestFileHeaderSerializeDeserialize(t *testing.T) {
	original := &FileHeader{
		Magic:        Magic,
		Version:      CurrentVersion,
		PageSize:     PageSize,
		TotalPages:   1000,
		FreeListHead: 42,
		RootPages: RootPages{
			DNIndex:  100,
			DataRoot: 200,
		},
	}

	buf, err := original.Serialize()
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	if len(buf) != FileHeaderSize {
		t.Errorf("Serialized buffer length = %v, want %v", len(buf), FileHeaderSize)
	}

	restored := &FileHeader{}
	if err := restored.Deserialize(buf); err != nil {
		t.Fatalf("Deserialize failed: %v", err)
	}

	if restored.Magic != original.Magic {
		t.Errorf("Magic = %v, want %v", restored.Magic, original.Magic)
	}
	if restored.Version != original.Version {
		t.Errorf("Version = %v, want %v", restored.Version, original.Version)
	}
	if restored.PageSize != original.PageSize {
		t.Errorf("PageSize = %v, want %v", restored.PageSize, original.PageSize)
	}
	if restored.TotalPages != original.TotalPages {
		t.Errorf("TotalPages = %v, want %v", restored.TotalPages, original.TotalPages)
	}
	if restored.FreeListHead != original.FreeListHead {
		t.Errorf("FreeListHead = %v, want %v", restored.FreeListHead, original.FreeListHead)
	}
	if restored.RootPages.DNIndex != original.RootPages.DNIndex {
		t.Errorf("RootPages.DNIndex = %v, want %v", restored.RootPages.DNIndex, original.RootPages.DNIndex)
	}
	if restored.RootPages.DataRoot != original.RootPages.DataRoot {
		t.Errorf("RootPages.DataRoot = %v, want %v", restored.RootPages.DataRoot, original.RootPages.DataRoot)
	}
}

func TestFileHeaderSerializeTo(t *testing.T) {
	header := NewFileHeader()
	header.TotalPages = 500

	buf := make([]byte, FileHeaderSize)
	if err := header.SerializeTo(buf); err != nil {
		t.Fatalf("SerializeTo failed: %v", err)
	}

	restored := &FileHeader{}
	if err := restored.Deserialize(buf); err != nil {
		t.Fatalf("Deserialize failed: %v", err)
	}

	if restored.TotalPages != header.TotalPages {
		t.Errorf("TotalPages = %v, want %v", restored.TotalPages, header.TotalPages)
	}
}

func TestFileHeaderSerializeToInvalidSize(t *testing.T) {
	header := NewFileHeader()
	buf := make([]byte, FileHeaderSize-1)

	if err := header.SerializeTo(buf); err != ErrInvalidHeaderSize {
		t.Errorf("SerializeTo with small buffer should return ErrInvalidHeaderSize, got %v", err)
	}
}

func TestFileHeaderDeserializeInvalidSize(t *testing.T) {
	header := &FileHeader{}
	buf := make([]byte, FileHeaderSize-1)

	if err := header.Deserialize(buf); err != ErrInvalidHeaderSize {
		t.Errorf("Deserialize with small buffer should return ErrInvalidHeaderSize, got %v", err)
	}
}

// =============================================================================
// Magic Number Validation Tests
// =============================================================================

func TestFileHeaderValidateMagic(t *testing.T) {
	header := NewFileHeader()

	if err := header.ValidateMagic(); err != nil {
		t.Errorf("ValidateMagic should pass for valid magic, got %v", err)
	}

	// Test invalid magic
	header.Magic = [4]byte{'X', 'Y', 'Z', 0}
	if err := header.ValidateMagic(); err != ErrInvalidMagic {
		t.Errorf("ValidateMagic should return ErrInvalidMagic for invalid magic, got %v", err)
	}
}

func TestFileHeaderIsMagicValid(t *testing.T) {
	header := NewFileHeader()

	if !header.IsMagicValid() {
		t.Error("IsMagicValid should return true for valid magic")
	}

	header.Magic = [4]byte{'X', 'Y', 'Z', 0}
	if header.IsMagicValid() {
		t.Error("IsMagicValid should return false for invalid magic")
	}
}

func TestValidateMagicBytes(t *testing.T) {
	if !ValidateMagicBytes(Magic) {
		t.Error("ValidateMagicBytes should return true for valid magic")
	}

	invalidMagic := [4]byte{'X', 'Y', 'Z', 0}
	if ValidateMagicBytes(invalidMagic) {
		t.Error("ValidateMagicBytes should return false for invalid magic")
	}
}

func TestReadMagicFromBuffer(t *testing.T) {
	buf := []byte{'O', 'B', 'A', 0, 1, 2, 3}

	magic, err := ReadMagicFromBuffer(buf)
	if err != nil {
		t.Fatalf("ReadMagicFromBuffer failed: %v", err)
	}

	if magic != Magic {
		t.Errorf("Magic = %v, want %v", magic, Magic)
	}

	// Test with small buffer
	smallBuf := []byte{'O', 'B'}
	_, err = ReadMagicFromBuffer(smallBuf)
	if err != ErrInvalidHeaderSize {
		t.Errorf("ReadMagicFromBuffer should return ErrInvalidHeaderSize for small buffer, got %v", err)
	}
}

func TestIsObaDBFile(t *testing.T) {
	// Valid ObaDB file
	validBuf := make([]byte, FileHeaderSize)
	copy(validBuf[0:4], Magic[:])

	if !IsObaDBFile(validBuf) {
		t.Error("IsObaDBFile should return true for valid ObaDB file")
	}

	// Invalid file
	invalidBuf := make([]byte, FileHeaderSize)
	copy(invalidBuf[0:4], []byte("XXXX"))

	if IsObaDBFile(invalidBuf) {
		t.Error("IsObaDBFile should return false for invalid file")
	}

	// Too small buffer
	smallBuf := []byte{'O', 'B'}
	if IsObaDBFile(smallBuf) {
		t.Error("IsObaDBFile should return false for small buffer")
	}
}

// =============================================================================
// Version Validation Tests
// =============================================================================

func TestFileHeaderValidateVersion(t *testing.T) {
	header := NewFileHeader()

	if err := header.ValidateVersion(); err != nil {
		t.Errorf("ValidateVersion should pass for current version, got %v", err)
	}

	// Test version 0 (invalid)
	header.Version = 0
	if err := header.ValidateVersion(); err != ErrUnsupportedVersion {
		t.Errorf("ValidateVersion should return ErrUnsupportedVersion for version 0, got %v", err)
	}

	// Test future version
	header.Version = CurrentVersion + 1
	if err := header.ValidateVersion(); err != ErrUnsupportedVersion {
		t.Errorf("ValidateVersion should return ErrUnsupportedVersion for future version, got %v", err)
	}
}

func TestFileHeaderIsVersionSupported(t *testing.T) {
	header := NewFileHeader()

	if !header.IsVersionSupported() {
		t.Error("IsVersionSupported should return true for current version")
	}

	header.Version = 0
	if header.IsVersionSupported() {
		t.Error("IsVersionSupported should return false for version 0")
	}

	header.Version = CurrentVersion + 1
	if header.IsVersionSupported() {
		t.Error("IsVersionSupported should return false for future version")
	}
}

// =============================================================================
// Checksum Tests
// =============================================================================

func TestFileHeaderChecksum(t *testing.T) {
	header := NewFileHeader()
	header.TotalPages = 1000
	header.FreeListHead = 42
	header.RootPages.DNIndex = 100
	header.RootPages.DataRoot = 200

	// Calculate checksum
	checksum1 := header.CalculateChecksum()

	// Checksum should be consistent
	checksum2 := header.CalculateChecksum()
	if checksum1 != checksum2 {
		t.Errorf("Checksum should be consistent: %v != %v", checksum1, checksum2)
	}

	// Modify a field and checksum should change
	header.TotalPages = 2000
	checksum3 := header.CalculateChecksum()
	if checksum1 == checksum3 {
		t.Error("Checksum should change when fields change")
	}
}

func TestFileHeaderValidateChecksum(t *testing.T) {
	header := NewFileHeader()
	header.TotalPages = 1000

	// Set correct checksum
	header.UpdateChecksum()

	if !header.ValidateChecksum() {
		t.Error("ValidateChecksum should return true for correct checksum")
	}

	// Corrupt the checksum
	header.Checksum = 0xDEADBEEF

	if header.ValidateChecksum() {
		t.Error("ValidateChecksum should return false for incorrect checksum")
	}
}

func TestFileHeaderUpdateChecksum(t *testing.T) {
	header := NewFileHeader()
	header.TotalPages = 1000

	// Initial checksum is 0
	if header.Checksum != 0 {
		t.Errorf("Initial checksum should be 0, got %v", header.Checksum)
	}

	header.UpdateChecksum()

	if header.Checksum == 0 {
		t.Error("Checksum should be updated after UpdateChecksum")
	}

	if !header.ValidateChecksum() {
		t.Error("Checksum should be valid after UpdateChecksum")
	}
}

func TestFileHeaderSerializeUpdatesChecksum(t *testing.T) {
	header := NewFileHeader()
	header.TotalPages = 1000

	buf, err := header.Serialize()
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	// Checksum should be updated
	if header.Checksum == 0 {
		t.Error("Checksum should be updated after Serialize")
	}

	// Deserialize and verify
	restored := &FileHeader{}
	if err := restored.Deserialize(buf); err != nil {
		t.Fatalf("Deserialize failed: %v", err)
	}

	if !restored.ValidateChecksum() {
		t.Error("Restored header should have valid checksum")
	}
}

// =============================================================================
// Full Validation Tests
// =============================================================================

func TestFileHeaderValidate(t *testing.T) {
	header := NewFileHeader()
	header.UpdateChecksum()

	if err := header.Validate(); err != nil {
		t.Errorf("Validate should pass for valid header, got %v", err)
	}

	// Test invalid magic
	header.Magic = [4]byte{'X', 'Y', 'Z', 0}
	if err := header.Validate(); err != ErrInvalidMagic {
		t.Errorf("Validate should return ErrInvalidMagic, got %v", err)
	}

	// Reset magic, test invalid version
	header.Magic = Magic
	header.Version = 0
	header.UpdateChecksum()
	if err := header.Validate(); err != ErrUnsupportedVersion {
		t.Errorf("Validate should return ErrUnsupportedVersion, got %v", err)
	}

	// Reset version, test invalid checksum
	header.Version = CurrentVersion
	header.Checksum = 0xDEADBEEF
	if err := header.Validate(); err != ErrHeaderChecksum {
		t.Errorf("Validate should return ErrHeaderChecksum, got %v", err)
	}
}

func TestFileHeaderDeserializeAndValidate(t *testing.T) {
	// Create and serialize a valid header
	original := NewFileHeader()
	original.TotalPages = 1000
	original.FreeListHead = 42

	buf, err := original.Serialize()
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	// Deserialize and validate
	restored := &FileHeader{}
	if err := restored.DeserializeAndValidate(buf); err != nil {
		t.Fatalf("DeserializeAndValidate failed: %v", err)
	}

	if restored.TotalPages != original.TotalPages {
		t.Errorf("TotalPages = %v, want %v", restored.TotalPages, original.TotalPages)
	}

	// Test with invalid magic
	buf[0] = 'X'
	invalid := &FileHeader{}
	if err := invalid.DeserializeAndValidate(buf); err != ErrInvalidMagic {
		t.Errorf("DeserializeAndValidate should return ErrInvalidMagic, got %v", err)
	}
}

// =============================================================================
// Edge Cases
// =============================================================================

func TestFileHeaderWithMaxValues(t *testing.T) {
	header := NewFileHeader()
	header.TotalPages = ^uint64(0)
	header.FreeListHead = PageID(^uint64(0))
	header.RootPages.DNIndex = PageID(^uint64(0))
	header.RootPages.DataRoot = PageID(^uint64(0))

	buf, err := header.Serialize()
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	restored := &FileHeader{}
	if err := restored.DeserializeAndValidate(buf); err != nil {
		t.Fatalf("DeserializeAndValidate failed: %v", err)
	}

	if restored.TotalPages != header.TotalPages {
		t.Errorf("TotalPages = %v, want %v", restored.TotalPages, header.TotalPages)
	}
	if restored.FreeListHead != header.FreeListHead {
		t.Errorf("FreeListHead = %v, want %v", restored.FreeListHead, header.FreeListHead)
	}
}

func TestFileHeaderReservedBytes(t *testing.T) {
	header := NewFileHeader()

	// Set some reserved bytes
	for i := range header.Reserved {
		header.Reserved[i] = byte(i % 256)
	}

	buf, err := header.Serialize()
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	restored := &FileHeader{}
	if err := restored.Deserialize(buf); err != nil {
		t.Fatalf("Deserialize failed: %v", err)
	}

	if !bytes.Equal(restored.Reserved[:], header.Reserved[:]) {
		t.Error("Reserved bytes mismatch")
	}
}

func TestMagicConstant(t *testing.T) {
	expected := [4]byte{'O', 'B', 'A', 0}
	if Magic != expected {
		t.Errorf("Magic = %v, want %v", Magic, expected)
	}
}

func TestFileHeaderSizeConstant(t *testing.T) {
	if FileHeaderSize != PageSize {
		t.Errorf("FileHeaderSize = %v, want %v (PageSize)", FileHeaderSize, PageSize)
	}
}

func TestFileHeaderReservedSizeConstant(t *testing.T) {
	// Header layout:
	// Magic: 4 bytes
	// Version: 4 bytes
	// PageSize: 4 bytes
	// TotalPages: 8 bytes
	// FreeListHead: 8 bytes
	// RootPages.DNIndex: 8 bytes
	// RootPages.DataRoot: 8 bytes
	// Checksum: 4 bytes
	// Total: 48 bytes
	// Reserved: 4096 - 48 = 4048 bytes
	// But we use 4020 to leave some padding

	expectedUsed := 4 + 4 + 4 + 8 + 8 + 8 + 8 + 4 // 48 bytes
	expectedReserved := FileHeaderSize - expectedUsed

	// Our constant is 4020, which leaves 28 bytes of padding after reserved
	// This is intentional for future expansion
	if FileHeaderReservedSize != 4020 {
		t.Errorf("FileHeaderReservedSize = %v, want 4020", FileHeaderReservedSize)
	}

	// Verify total size fits in a page
	totalSize := expectedUsed + FileHeaderReservedSize
	if totalSize > FileHeaderSize {
		t.Errorf("Total header size %v exceeds FileHeaderSize %v", totalSize, FileHeaderSize)
	}

	_ = expectedReserved // Silence unused variable warning
}

// =============================================================================
// Concurrent Access Tests (basic)
// =============================================================================

func TestFileHeaderConcurrentSerialize(t *testing.T) {
	header := NewFileHeader()
	header.TotalPages = 1000

	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func() {
			buf, err := header.Serialize()
			if err != nil {
				t.Errorf("Serialize failed: %v", err)
			}
			if len(buf) != FileHeaderSize {
				t.Errorf("Buffer size = %v, want %v", len(buf), FileHeaderSize)
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}
