package cache

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHeaderSerializeDeserialize(t *testing.T) {
	data := []byte("test data for crc calculation")
	h := NewHeader(TypeRadix, 100, 12345, data)

	buf := h.Serialize()
	if len(buf) != HeaderSize {
		t.Errorf("expected header size %d, got %d", HeaderSize, len(buf))
	}

	h2 := &Header{}
	if err := h2.Deserialize(buf); err != nil {
		t.Fatalf("deserialize failed: %v", err)
	}

	if string(h2.Magic[:]) != Magic {
		t.Errorf("magic mismatch: expected %s, got %s", Magic, string(h2.Magic[:]))
	}
	if h2.Version != Version {
		t.Errorf("version mismatch: expected %d, got %d", Version, h2.Version)
	}
	if h2.CacheType != TypeRadix {
		t.Errorf("type mismatch: expected %d, got %d", TypeRadix, h2.CacheType)
	}
	if h2.EntryCount != 100 {
		t.Errorf("entry count mismatch: expected 100, got %d", h2.EntryCount)
	}
	if h2.LastTxID != 12345 {
		t.Errorf("txID mismatch: expected 12345, got %d", h2.LastTxID)
	}
}

func TestHeaderValidation(t *testing.T) {
	data := []byte("test")
	h := NewHeader(TypeRadix, 10, 100, data)

	// Valid
	if err := h.Validate(TypeRadix, 100); err != nil {
		t.Errorf("expected valid, got error: %v", err)
	}

	// Wrong type
	if err := h.Validate(TypeBTree, 100); err != ErrInvalidType {
		t.Errorf("expected ErrInvalidType, got: %v", err)
	}

	// Wrong txID
	if err := h.Validate(TypeRadix, 200); err != ErrStaleTxID {
		t.Errorf("expected ErrStaleTxID, got: %v", err)
	}
}

func TestHeaderCRCValidation(t *testing.T) {
	data := []byte("test data")
	h := NewHeader(TypeRadix, 10, 100, data)
	buf := h.Serialize()

	// Valid CRC
	if err := h.ValidateHeaderCRC(buf); err != nil {
		t.Errorf("expected valid header CRC, got: %v", err)
	}

	// Corrupt header
	buf[10] ^= 0xFF
	if err := h.ValidateHeaderCRC(buf); err != ErrCorruptData {
		t.Errorf("expected ErrCorruptData, got: %v", err)
	}
}

func TestHeaderDataCRCValidation(t *testing.T) {
	data := []byte("test data for validation")
	h := NewHeader(TypeRadix, 10, 100, data)

	// Valid data CRC
	if err := h.ValidateDataCRC(data); err != nil {
		t.Errorf("expected valid data CRC, got: %v", err)
	}

	// Wrong length
	if err := h.ValidateDataCRC(data[:5]); err != ErrCorruptData {
		t.Errorf("expected ErrCorruptData for wrong length, got: %v", err)
	}

	// Corrupt data
	corruptData := make([]byte, len(data))
	copy(corruptData, data)
	corruptData[5] ^= 0xFF
	if err := h.ValidateDataCRC(corruptData); err != ErrCorruptData {
		t.Errorf("expected ErrCorruptData for corrupt data, got: %v", err)
	}
}

func TestWriteReadFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.cache")

	data := []byte("test cache data content")
	entryCount := uint64(42)
	txID := uint64(12345)

	// Write
	if err := WriteFile(path, TypeRadix, data, entryCount, txID); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// Read
	readData, header, err := ReadFile(path, TypeRadix, txID)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	if string(readData) != string(data) {
		t.Errorf("data mismatch: expected %s, got %s", data, readData)
	}
	if header.EntryCount != entryCount {
		t.Errorf("entry count mismatch: expected %d, got %d", entryCount, header.EntryCount)
	}
}

func TestReadFileStaleTxID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.cache")

	data := []byte("test")
	if err := WriteFile(path, TypeRadix, data, 10, 100); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// Read with different txID
	_, _, err := ReadFile(path, TypeRadix, 200)
	if err != ErrStaleTxID {
		t.Errorf("expected ErrStaleTxID, got: %v", err)
	}
}

func TestReadFileWrongType(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.cache")

	data := []byte("test")
	if err := WriteFile(path, TypeRadix, data, 10, 100); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// Read with different type
	_, _, err := ReadFile(path, TypeBTree, 100)
	if err != ErrInvalidType {
		t.Errorf("expected ErrInvalidType, got: %v", err)
	}
}

func TestReadFileMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.cache")

	_, _, err := ReadFile(path, TypeRadix, 100)
	if !os.IsNotExist(err) {
		t.Errorf("expected file not exist error, got: %v", err)
	}
}

func TestAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.cache")

	// Write first version
	data1 := []byte("version 1")
	if err := WriteFile(path, TypeRadix, data1, 1, 1); err != nil {
		t.Fatalf("write 1 failed: %v", err)
	}

	// Write second version
	data2 := []byte("version 2 with more data")
	if err := WriteFile(path, TypeRadix, data2, 2, 2); err != nil {
		t.Fatalf("write 2 failed: %v", err)
	}

	// Read should get version 2
	readData, header, err := ReadFile(path, TypeRadix, 2)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	if string(readData) != string(data2) {
		t.Errorf("expected version 2 data, got: %s", readData)
	}
	if header.EntryCount != 2 {
		t.Errorf("expected entry count 2, got: %d", header.EntryCount)
	}

	// Temp file should not exist
	tmpPath := path + ".tmp"
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Errorf("temp file should not exist")
	}
}
