package storage

import (
	"bytes"
	"testing"
)

// =============================================================================
// PageType Tests
// =============================================================================

func TestPageTypeString(t *testing.T) {
	tests := []struct {
		pageType PageType
		expected string
	}{
		{PageTypeFree, "Free"},
		{PageTypeData, "Data"},
		{PageTypeDNIndex, "DNIndex"},
		{PageTypeAttrIndex, "AttrIndex"},
		{PageTypeOverflow, "Overflow"},
		{PageTypeWAL, "WAL"},
		{PageType(255), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.pageType.String(); got != tt.expected {
				t.Errorf("PageType.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// =============================================================================
// PageHeader Tests
// =============================================================================

func TestNewPageHeader(t *testing.T) {
	header := NewPageHeader(42, PageTypeData)

	if header.PageID != 42 {
		t.Errorf("PageID = %v, want 42", header.PageID)
	}
	if header.PageType != PageTypeData {
		t.Errorf("PageType = %v, want PageTypeData", header.PageType)
	}
	if header.Flags != 0 {
		t.Errorf("Flags = %v, want 0", header.Flags)
	}
	if header.ItemCount != 0 {
		t.Errorf("ItemCount = %v, want 0", header.ItemCount)
	}
	if header.FreeSpace != PageSize-PageHeaderSize {
		t.Errorf("FreeSpace = %v, want %v", header.FreeSpace, PageSize-PageHeaderSize)
	}
	if header.Checksum != 0 {
		t.Errorf("Checksum = %v, want 0", header.Checksum)
	}
}

func TestPageHeaderSerializeDeserialize(t *testing.T) {
	original := &PageHeader{
		PageID:    12345,
		PageType:  PageTypeDNIndex,
		Flags:     PageFlagDirty | PageFlagPinned,
		ItemCount: 100,
		FreeSpace: 2048,
		Checksum:  0xABCD,
	}

	buf := make([]byte, PageHeaderSize)
	if err := original.Serialize(buf); err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	restored := &PageHeader{}
	if err := restored.Deserialize(buf); err != nil {
		t.Fatalf("Deserialize failed: %v", err)
	}

	if restored.PageID != original.PageID {
		t.Errorf("PageID = %v, want %v", restored.PageID, original.PageID)
	}
	if restored.PageType != original.PageType {
		t.Errorf("PageType = %v, want %v", restored.PageType, original.PageType)
	}
	if restored.Flags != original.Flags {
		t.Errorf("Flags = %v, want %v", restored.Flags, original.Flags)
	}
	if restored.ItemCount != original.ItemCount {
		t.Errorf("ItemCount = %v, want %v", restored.ItemCount, original.ItemCount)
	}
	if restored.FreeSpace != original.FreeSpace {
		t.Errorf("FreeSpace = %v, want %v", restored.FreeSpace, original.FreeSpace)
	}
	if restored.Checksum != original.Checksum {
		t.Errorf("Checksum = %v, want %v", restored.Checksum, original.Checksum)
	}
}

func TestPageHeaderSerializeInvalidSize(t *testing.T) {
	header := NewPageHeader(1, PageTypeData)
	buf := make([]byte, PageHeaderSize-1)

	if err := header.Serialize(buf); err != ErrInvalidPageSize {
		t.Errorf("Serialize with small buffer should return ErrInvalidPageSize, got %v", err)
	}
}

func TestPageHeaderDeserializeInvalidSize(t *testing.T) {
	header := &PageHeader{}
	buf := make([]byte, PageHeaderSize-1)

	if err := header.Deserialize(buf); err != ErrInvalidPageSize {
		t.Errorf("Deserialize with small buffer should return ErrInvalidPageSize, got %v", err)
	}
}

func TestPageHeaderFlags(t *testing.T) {
	header := NewPageHeader(1, PageTypeData)

	// Test dirty flag
	if header.IsDirty() {
		t.Error("New header should not be dirty")
	}
	header.SetDirty()
	if !header.IsDirty() {
		t.Error("Header should be dirty after SetDirty")
	}
	header.ClearDirty()
	if header.IsDirty() {
		t.Error("Header should not be dirty after ClearDirty")
	}

	// Test pinned flag
	if header.IsPinned() {
		t.Error("New header should not be pinned")
	}
	header.SetPinned()
	if !header.IsPinned() {
		t.Error("Header should be pinned after SetPinned")
	}
	header.ClearPinned()
	if header.IsPinned() {
		t.Error("Header should not be pinned after ClearPinned")
	}

	// Test leaf flag
	if header.IsLeaf() {
		t.Error("New header should not be leaf")
	}
	header.SetLeaf()
	if !header.IsLeaf() {
		t.Error("Header should be leaf after SetLeaf")
	}

	// Test multiple flags
	header.SetDirty()
	header.SetPinned()
	if !header.IsDirty() || !header.IsPinned() || !header.IsLeaf() {
		t.Error("Multiple flags should be set independently")
	}
}

// =============================================================================
// Page Tests
// =============================================================================

func TestNewPage(t *testing.T) {
	page := NewPage(42, PageTypeData)

	if page.Header.PageID != 42 {
		t.Errorf("PageID = %v, want 42", page.Header.PageID)
	}
	if page.Header.PageType != PageTypeData {
		t.Errorf("PageType = %v, want PageTypeData", page.Header.PageType)
	}
	if len(page.Data) != PageSize-PageHeaderSize {
		t.Errorf("Data length = %v, want %v", len(page.Data), PageSize-PageHeaderSize)
	}
}

func TestPageSerializeDeserialize(t *testing.T) {
	original := NewPage(12345, PageTypeDNIndex)
	original.Header.ItemCount = 50
	original.Header.FreeSpace = 1024

	// Write some test data
	testData := []byte("Hello, ObaDB!")
	copy(original.Data, testData)

	// Serialize
	buf, err := original.Serialize()
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	if len(buf) != PageSize {
		t.Errorf("Serialized buffer length = %v, want %v", len(buf), PageSize)
	}

	// Deserialize
	restored := &Page{}
	if err := restored.Deserialize(buf); err != nil {
		t.Fatalf("Deserialize failed: %v", err)
	}

	if restored.Header.PageID != original.Header.PageID {
		t.Errorf("PageID = %v, want %v", restored.Header.PageID, original.Header.PageID)
	}
	if restored.Header.PageType != original.Header.PageType {
		t.Errorf("PageType = %v, want %v", restored.Header.PageType, original.Header.PageType)
	}
	if restored.Header.ItemCount != original.Header.ItemCount {
		t.Errorf("ItemCount = %v, want %v", restored.Header.ItemCount, original.Header.ItemCount)
	}

	// Verify data
	if !bytes.Equal(restored.Data[:len(testData)], testData) {
		t.Errorf("Data mismatch: got %v, want %v", restored.Data[:len(testData)], testData)
	}
}

func TestPageSerializeTo(t *testing.T) {
	page := NewPage(1, PageTypeData)
	copy(page.Data, []byte("test data"))

	buf := make([]byte, PageSize)
	if err := page.SerializeTo(buf); err != nil {
		t.Fatalf("SerializeTo failed: %v", err)
	}

	// Verify by deserializing
	restored := &Page{}
	if err := restored.Deserialize(buf); err != nil {
		t.Fatalf("Deserialize failed: %v", err)
	}

	if restored.Header.PageID != page.Header.PageID {
		t.Errorf("PageID mismatch")
	}
}

func TestPageSerializeToInvalidSize(t *testing.T) {
	page := NewPage(1, PageTypeData)
	buf := make([]byte, PageSize-1)

	if err := page.SerializeTo(buf); err != ErrInvalidPageSize {
		t.Errorf("SerializeTo with small buffer should return ErrInvalidPageSize, got %v", err)
	}
}

func TestPageDeserializeInvalidSize(t *testing.T) {
	page := &Page{}
	buf := make([]byte, PageSize-1)

	if err := page.Deserialize(buf); err != ErrInvalidPageSize {
		t.Errorf("Deserialize with small buffer should return ErrInvalidPageSize, got %v", err)
	}
}

func TestPageChecksum(t *testing.T) {
	page := NewPage(1, PageTypeData)
	copy(page.Data, []byte("test data for checksum"))

	// Calculate checksum
	checksum1 := page.CalculateChecksum()

	// Checksum should be consistent
	checksum2 := page.CalculateChecksum()
	if checksum1 != checksum2 {
		t.Errorf("Checksum should be consistent: %v != %v", checksum1, checksum2)
	}

	// Modify data and checksum should change
	page.Data[0] = 'X'
	checksum3 := page.CalculateChecksum()
	if checksum1 == checksum3 {
		t.Error("Checksum should change when data changes")
	}
}

func TestPageValidateChecksum(t *testing.T) {
	page := NewPage(1, PageTypeData)
	copy(page.Data, []byte("test data"))

	// Set correct checksum
	page.Header.Checksum = page.CalculateChecksum()

	if !page.ValidateChecksum() {
		t.Error("ValidateChecksum should return true for correct checksum")
	}

	// Corrupt the checksum
	page.Header.Checksum = 0xFFFF

	if page.ValidateChecksum() {
		t.Error("ValidateChecksum should return false for incorrect checksum")
	}
}

func TestPageDeserializeAndValidate(t *testing.T) {
	// Create and serialize a valid page
	original := NewPage(1, PageTypeData)
	copy(original.Data, []byte("valid data"))

	buf, err := original.Serialize()
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	// Deserialize and validate
	restored := &Page{}
	if err := restored.DeserializeAndValidate(buf); err != nil {
		t.Fatalf("DeserializeAndValidate failed: %v", err)
	}

	// Corrupt the data and try again
	buf[PageHeaderSize] = 0xFF // Modify first data byte

	corrupted := &Page{}
	if err := corrupted.DeserializeAndValidate(buf); err != ErrInvalidChecksum {
		t.Errorf("DeserializeAndValidate should return ErrInvalidChecksum for corrupted data, got %v", err)
	}
}

func TestPageUsableSpace(t *testing.T) {
	page := NewPage(1, PageTypeData)

	expected := PageSize - PageHeaderSize
	if got := page.UsableSpace(); got != expected {
		t.Errorf("UsableSpace() = %v, want %v", got, expected)
	}
}

func TestPageReset(t *testing.T) {
	page := NewPage(1, PageTypeData)
	page.Header.Flags = PageFlagDirty | PageFlagPinned
	page.Header.ItemCount = 100
	page.Header.FreeSpace = 500
	page.Header.Checksum = 0xABCD
	copy(page.Data, []byte("some data"))

	page.Reset(PageTypeFree)

	if page.Header.PageType != PageTypeFree {
		t.Errorf("PageType = %v, want PageTypeFree", page.Header.PageType)
	}
	if page.Header.Flags != 0 {
		t.Errorf("Flags = %v, want 0", page.Header.Flags)
	}
	if page.Header.ItemCount != 0 {
		t.Errorf("ItemCount = %v, want 0", page.Header.ItemCount)
	}
	if page.Header.FreeSpace != PageSize-PageHeaderSize {
		t.Errorf("FreeSpace = %v, want %v", page.Header.FreeSpace, PageSize-PageHeaderSize)
	}
	if page.Header.Checksum != 0 {
		t.Errorf("Checksum = %v, want 0", page.Header.Checksum)
	}

	// Verify data is cleared
	for i, b := range page.Data {
		if b != 0 {
			t.Errorf("Data[%d] = %v, want 0", i, b)
			break
		}
	}
}

func TestPageSerializeUpdatesChecksum(t *testing.T) {
	page := NewPage(1, PageTypeData)
	copy(page.Data, []byte("test data"))

	// Checksum should be 0 initially
	if page.Header.Checksum != 0 {
		t.Errorf("Initial checksum should be 0, got %v", page.Header.Checksum)
	}

	// Serialize should update checksum
	buf, err := page.Serialize()
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	// Checksum in header should be updated
	if page.Header.Checksum == 0 {
		t.Error("Checksum should be updated after Serialize")
	}

	// Deserialize and verify checksum matches
	restored := &Page{}
	if err := restored.Deserialize(buf); err != nil {
		t.Fatalf("Deserialize failed: %v", err)
	}

	if !restored.ValidateChecksum() {
		t.Error("Restored page should have valid checksum")
	}
}

// =============================================================================
// Constants Tests
// =============================================================================

func TestConstants(t *testing.T) {
	if PageSize != 4096 {
		t.Errorf("PageSize = %v, want 4096", PageSize)
	}
	if PageHeaderSize != 16 {
		t.Errorf("PageHeaderSize = %v, want 16", PageHeaderSize)
	}
}

// =============================================================================
// Edge Cases
// =============================================================================

func TestPageWithMaxPageID(t *testing.T) {
	maxID := PageID(^uint64(0))
	page := NewPage(maxID, PageTypeData)

	buf, err := page.Serialize()
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	restored := &Page{}
	if err := restored.Deserialize(buf); err != nil {
		t.Fatalf("Deserialize failed: %v", err)
	}

	if restored.Header.PageID != maxID {
		t.Errorf("PageID = %v, want %v", restored.Header.PageID, maxID)
	}
}

func TestPageWithAllPageTypes(t *testing.T) {
	pageTypes := []PageType{
		PageTypeFree,
		PageTypeData,
		PageTypeDNIndex,
		PageTypeAttrIndex,
		PageTypeOverflow,
		PageTypeWAL,
	}

	for _, pt := range pageTypes {
		t.Run(pt.String(), func(t *testing.T) {
			page := NewPage(1, pt)

			buf, err := page.Serialize()
			if err != nil {
				t.Fatalf("Serialize failed: %v", err)
			}

			restored := &Page{}
			if err := restored.Deserialize(buf); err != nil {
				t.Fatalf("Deserialize failed: %v", err)
			}

			if restored.Header.PageType != pt {
				t.Errorf("PageType = %v, want %v", restored.Header.PageType, pt)
			}
		})
	}
}

func TestPageWithFullData(t *testing.T) {
	page := NewPage(1, PageTypeData)

	// Fill entire data area
	for i := range page.Data {
		page.Data[i] = byte(i % 256)
	}

	buf, err := page.Serialize()
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	restored := &Page{}
	if err := restored.DeserializeAndValidate(buf); err != nil {
		t.Fatalf("DeserializeAndValidate failed: %v", err)
	}

	if !bytes.Equal(restored.Data, page.Data) {
		t.Error("Data mismatch after serialize/deserialize")
	}
}
