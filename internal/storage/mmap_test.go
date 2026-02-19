package storage

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestNewMmapManager(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test_mmap.db")

	// Create a test file
	file, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Write some initial data
	initialSize := int64(PageSize * 4)
	if err := file.Truncate(initialSize); err != nil {
		file.Close()
		t.Fatalf("failed to truncate file: %v", err)
	}

	// Create mmap manager
	mm, err := NewMmapManager(file, initialSize)
	if err != nil {
		file.Close()
		t.Fatalf("failed to create mmap manager: %v", err)
	}

	// Verify properties
	if mm.Size() != initialSize {
		t.Errorf("expected size %d, got %d", initialSize, mm.Size())
	}

	if mm.PageSize() != PageSize {
		t.Errorf("expected page size %d, got %d", PageSize, mm.PageSize())
	}

	if mm.PageCount() != 4 {
		t.Errorf("expected page count 4, got %d", mm.PageCount())
	}

	if !mm.IsMapped() {
		t.Error("expected IsMapped to return true")
	}

	// Clean up
	if err := mm.Close(); err != nil {
		t.Errorf("failed to close mmap manager: %v", err)
	}

	file.Close()
}

func TestNewMmapManagerWithOptions(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test_mmap_opts.db")

	file, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	defer file.Close()

	opts := MmapOptions{
		PageSize: 8192,
		ReadOnly: false,
	}

	mm, err := NewMmapManagerWithOptions(file, int64(opts.PageSize*2), opts)
	if err != nil {
		t.Fatalf("failed to create mmap manager: %v", err)
	}
	defer mm.Close()

	if mm.PageSize() != 8192 {
		t.Errorf("expected page size 8192, got %d", mm.PageSize())
	}

	if mm.IsReadOnly() {
		t.Error("expected IsReadOnly to return false")
	}
}

func TestMmapManagerGetPage(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test_getpage.db")

	file, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	defer file.Close()

	size := int64(PageSize * 4)
	mm, err := NewMmapManager(file, size)
	if err != nil {
		t.Fatalf("failed to create mmap manager: %v", err)
	}
	defer mm.Close()

	// Test getting valid pages
	for i := PageID(0); i < 4; i++ {
		page, err := mm.GetPage(i)
		if err != nil {
			t.Errorf("failed to get page %d: %v", i, err)
			continue
		}

		if len(page) != PageSize {
			t.Errorf("page %d: expected size %d, got %d", i, PageSize, len(page))
		}
	}

	// Test getting invalid page (out of range)
	_, err = mm.GetPage(10)
	if err != ErrMmapPageOutOfRange {
		t.Errorf("expected ErrMmapPageOutOfRange, got %v", err)
	}
}

func TestMmapManagerGetPageRange(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test_getpagerange.db")

	file, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	defer file.Close()

	size := int64(PageSize * 8)
	mm, err := NewMmapManager(file, size)
	if err != nil {
		t.Fatalf("failed to create mmap manager: %v", err)
	}
	defer mm.Close()

	// Test getting a range of pages
	data, err := mm.GetPageRange(2, 3)
	if err != nil {
		t.Fatalf("failed to get page range: %v", err)
	}

	expectedSize := PageSize * 3
	if len(data) != expectedSize {
		t.Errorf("expected size %d, got %d", expectedSize, len(data))
	}

	// Test invalid range
	_, err = mm.GetPageRange(6, 5)
	if err != ErrMmapPageOutOfRange {
		t.Errorf("expected ErrMmapPageOutOfRange, got %v", err)
	}

	// Test invalid count
	_, err = mm.GetPageRange(0, 0)
	if err != ErrMmapInvalidSize {
		t.Errorf("expected ErrMmapInvalidSize, got %v", err)
	}
}

func TestMmapManagerReadWrite(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test_readwrite.db")

	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	defer file.Close()

	size := int64(PageSize * 4)
	mm, err := NewMmapManager(file, size)
	if err != nil {
		t.Fatalf("failed to create mmap manager: %v", err)
	}
	defer mm.Close()

	// Write test data
	testData := []byte("Hello, Memory-Mapped I/O!")
	offset := int64(PageSize) // Write to second page

	n, err := mm.WriteAt(testData, offset)
	if err != nil {
		t.Fatalf("failed to write data: %v", err)
	}

	if n != len(testData) {
		t.Errorf("expected to write %d bytes, wrote %d", len(testData), n)
	}

	// Read back the data
	readBuf := make([]byte, len(testData))
	n, err = mm.ReadAt(readBuf, offset)
	if err != nil {
		t.Fatalf("failed to read data: %v", err)
	}

	if n != len(testData) {
		t.Errorf("expected to read %d bytes, read %d", len(testData), n)
	}

	if !bytes.Equal(readBuf, testData) {
		t.Errorf("data mismatch: expected %q, got %q", testData, readBuf)
	}
}

func TestMmapManagerZeroCopyRead(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test_zerocopy.db")

	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	defer file.Close()

	size := int64(PageSize * 2)
	mm, err := NewMmapManager(file, size)
	if err != nil {
		t.Fatalf("failed to create mmap manager: %v", err)
	}
	defer mm.Close()

	// Write data directly to the mmap'd region
	page, err := mm.GetPage(0)
	if err != nil {
		t.Fatalf("failed to get page: %v", err)
	}

	testData := []byte("Zero-copy test data")
	copy(page, testData)

	// Sync to ensure data is written
	if err := mm.Sync(); err != nil {
		t.Fatalf("failed to sync: %v", err)
	}

	// Read back using GetPage (zero-copy)
	page2, err := mm.GetPage(0)
	if err != nil {
		t.Fatalf("failed to get page again: %v", err)
	}

	// Verify the data
	if !bytes.Equal(page2[:len(testData)], testData) {
		t.Errorf("zero-copy read failed: expected %q, got %q", testData, page2[:len(testData)])
	}

	// Verify that page and page2 point to the same memory
	// (this is the essence of zero-copy)
	page[0] = 'X'
	if page2[0] != 'X' {
		t.Error("pages do not share the same memory - not zero-copy")
	}
}

func TestMmapManagerRemap(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test_remap.db")

	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	defer file.Close()

	initialSize := int64(PageSize * 2)
	mm, err := NewMmapManager(file, initialSize)
	if err != nil {
		t.Fatalf("failed to create mmap manager: %v", err)
	}
	defer mm.Close()

	// Write data to first page
	testData := []byte("Data before remap")
	page, err := mm.GetPage(0)
	if err != nil {
		t.Fatalf("failed to get page: %v", err)
	}
	copy(page, testData)

	// Sync before remap
	if err := mm.Sync(); err != nil {
		t.Fatalf("failed to sync: %v", err)
	}

	// Grow the mapping
	newSize := int64(PageSize * 8)
	if err := mm.Remap(newSize); err != nil {
		t.Fatalf("failed to remap: %v", err)
	}

	// Verify new size
	if mm.Size() != newSize {
		t.Errorf("expected size %d after remap, got %d", newSize, mm.Size())
	}

	if mm.PageCount() != 8 {
		t.Errorf("expected page count 8 after remap, got %d", mm.PageCount())
	}

	// Verify data is preserved
	page, err = mm.GetPage(0)
	if err != nil {
		t.Fatalf("failed to get page after remap: %v", err)
	}

	if !bytes.Equal(page[:len(testData)], testData) {
		t.Errorf("data not preserved after remap: expected %q, got %q", testData, page[:len(testData)])
	}

	// Verify we can access new pages
	_, err = mm.GetPage(7)
	if err != nil {
		t.Errorf("failed to access new page after remap: %v", err)
	}
}

func TestMmapManagerSync(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test_sync.db")

	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	defer file.Close()

	size := int64(PageSize * 2)
	mm, err := NewMmapManager(file, size)
	if err != nil {
		t.Fatalf("failed to create mmap manager: %v", err)
	}
	defer mm.Close()

	// Write data
	testData := []byte("Sync test data")
	page, err := mm.GetPage(0)
	if err != nil {
		t.Fatalf("failed to get page: %v", err)
	}
	copy(page, testData)

	// Sync
	if err := mm.Sync(); err != nil {
		t.Fatalf("failed to sync: %v", err)
	}

	// Close and reopen to verify persistence
	if err := mm.Close(); err != nil {
		t.Fatalf("failed to close: %v", err)
	}

	// Reopen file
	file2, err := os.OpenFile(filePath, os.O_RDWR, 0644)
	if err != nil {
		t.Fatalf("failed to reopen file: %v", err)
	}
	defer file2.Close()

	mm2, err := NewMmapManager(file2, size)
	if err != nil {
		t.Fatalf("failed to create second mmap manager: %v", err)
	}
	defer mm2.Close()

	// Verify data persisted
	page2, err := mm2.GetPage(0)
	if err != nil {
		t.Fatalf("failed to get page from second manager: %v", err)
	}

	if !bytes.Equal(page2[:len(testData)], testData) {
		t.Errorf("data not persisted: expected %q, got %q", testData, page2[:len(testData)])
	}
}

func TestMmapManagerClose(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test_close.db")

	file, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	defer file.Close()

	mm, err := NewMmapManager(file, int64(PageSize))
	if err != nil {
		t.Fatalf("failed to create mmap manager: %v", err)
	}

	// Close the manager
	if err := mm.Close(); err != nil {
		t.Fatalf("failed to close: %v", err)
	}

	// Verify operations fail after close
	_, err = mm.GetPage(0)
	if err != ErrMmapClosed {
		t.Errorf("expected ErrMmapClosed, got %v", err)
	}

	err = mm.Sync()
	if err != ErrMmapClosed {
		t.Errorf("expected ErrMmapClosed from Sync, got %v", err)
	}

	err = mm.Remap(int64(PageSize * 2))
	if err != ErrMmapClosed {
		t.Errorf("expected ErrMmapClosed from Remap, got %v", err)
	}

	// Double close should return error
	err = mm.Close()
	if err != ErrMmapClosed {
		t.Errorf("expected ErrMmapClosed from double close, got %v", err)
	}
}

func TestMmapManagerReadOnly(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test_readonly.db")

	// Create and write initial data
	file, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	size := int64(PageSize * 2)
	if err := file.Truncate(size); err != nil {
		file.Close()
		t.Fatalf("failed to truncate: %v", err)
	}

	// Write some data
	testData := []byte("Read-only test data")
	if _, err := file.WriteAt(testData, 0); err != nil {
		file.Close()
		t.Fatalf("failed to write initial data: %v", err)
	}
	file.Close()

	// Open in read-only mode
	file, err = os.Open(filePath)
	if err != nil {
		t.Fatalf("failed to open file: %v", err)
	}
	defer file.Close()

	opts := MmapOptions{
		PageSize: PageSize,
		ReadOnly: true,
	}

	mm, err := NewMmapManagerWithOptions(file, size, opts)
	if err != nil {
		t.Fatalf("failed to create read-only mmap manager: %v", err)
	}
	defer mm.Close()

	if !mm.IsReadOnly() {
		t.Error("expected IsReadOnly to return true")
	}

	// Reading should work
	page, err := mm.GetPage(0)
	if err != nil {
		t.Fatalf("failed to get page: %v", err)
	}

	if !bytes.Equal(page[:len(testData)], testData) {
		t.Errorf("read data mismatch: expected %q, got %q", testData, page[:len(testData)])
	}

	// Writing should fail
	_, err = mm.WriteAt([]byte("test"), 0)
	if err != ErrMmapReadOnly {
		t.Errorf("expected ErrMmapReadOnly from WriteAt, got %v", err)
	}

	// Remap should fail
	err = mm.Remap(int64(PageSize * 4))
	if err != ErrMmapReadOnly {
		t.Errorf("expected ErrMmapReadOnly from Remap, got %v", err)
	}
}

func TestMmapManagerGetData(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test_getdata.db")

	file, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	defer file.Close()

	size := int64(PageSize * 4)
	mm, err := NewMmapManager(file, size)
	if err != nil {
		t.Fatalf("failed to create mmap manager: %v", err)
	}
	defer mm.Close()

	data, err := mm.GetData()
	if err != nil {
		t.Fatalf("failed to get data: %v", err)
	}

	if int64(len(data)) != size {
		t.Errorf("expected data length %d, got %d", size, len(data))
	}
}

func TestAlignToPageSize(t *testing.T) {
	tests := []struct {
		size     int64
		pageSize int
		expected int64
	}{
		{0, 4096, 0},
		{1, 4096, 4096},
		{4096, 4096, 4096},
		{4097, 4096, 8192},
		{8192, 4096, 8192},
		{10000, 4096, 12288},
	}

	for _, tt := range tests {
		result := alignToPageSize(tt.size, tt.pageSize)
		if result != tt.expected {
			t.Errorf("alignToPageSize(%d, %d) = %d, expected %d",
				tt.size, tt.pageSize, result, tt.expected)
		}
	}
}

// Benchmarks for zero-copy reads

func BenchmarkMmapGetPage(b *testing.B) {
	tmpDir := b.TempDir()
	filePath := filepath.Join(tmpDir, "bench_mmap.db")

	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		b.Fatalf("failed to create test file: %v", err)
	}
	defer file.Close()

	size := int64(PageSize * 1000)
	mm, err := NewMmapManager(file, size)
	if err != nil {
		b.Fatalf("failed to create mmap manager: %v", err)
	}
	defer mm.Close()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		pageID := PageID(i % 1000)
		_, err := mm.GetPage(pageID)
		if err != nil {
			b.Fatalf("failed to get page: %v", err)
		}
	}
}

func BenchmarkMmapReadAt(b *testing.B) {
	tmpDir := b.TempDir()
	filePath := filepath.Join(tmpDir, "bench_mmap_read.db")

	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		b.Fatalf("failed to create test file: %v", err)
	}
	defer file.Close()

	size := int64(PageSize * 1000)
	mm, err := NewMmapManager(file, size)
	if err != nil {
		b.Fatalf("failed to create mmap manager: %v", err)
	}
	defer mm.Close()

	buf := make([]byte, PageSize)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		offset := int64((i % 1000) * PageSize)
		_, err := mm.ReadAt(buf, offset)
		if err != nil {
			b.Fatalf("failed to read: %v", err)
		}
	}
}

func BenchmarkMmapWriteAt(b *testing.B) {
	tmpDir := b.TempDir()
	filePath := filepath.Join(tmpDir, "bench_mmap_write.db")

	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		b.Fatalf("failed to create test file: %v", err)
	}
	defer file.Close()

	size := int64(PageSize * 1000)
	mm, err := NewMmapManager(file, size)
	if err != nil {
		b.Fatalf("failed to create mmap manager: %v", err)
	}
	defer mm.Close()

	buf := make([]byte, PageSize)
	for i := range buf {
		buf[i] = byte(i % 256)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		offset := int64((i % 1000) * PageSize)
		_, err := mm.WriteAt(buf, offset)
		if err != nil {
			b.Fatalf("failed to write: %v", err)
		}
	}
}

func BenchmarkMmapZeroCopyVsRegularRead(b *testing.B) {
	tmpDir := b.TempDir()

	// Setup mmap file
	mmapPath := filepath.Join(tmpDir, "bench_zerocopy.db")
	mmapFile, err := os.OpenFile(mmapPath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		b.Fatalf("failed to create mmap file: %v", err)
	}
	defer mmapFile.Close()

	size := int64(PageSize * 100)
	mm, err := NewMmapManager(mmapFile, size)
	if err != nil {
		b.Fatalf("failed to create mmap manager: %v", err)
	}
	defer mm.Close()

	// Setup regular file
	regularPath := filepath.Join(tmpDir, "bench_regular.db")
	regularFile, err := os.OpenFile(regularPath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		b.Fatalf("failed to create regular file: %v", err)
	}
	defer regularFile.Close()

	if err := regularFile.Truncate(size); err != nil {
		b.Fatalf("failed to truncate regular file: %v", err)
	}

	b.Run("ZeroCopy", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			pageID := PageID(i % 100)
			_, err := mm.GetPage(pageID)
			if err != nil {
				b.Fatalf("failed to get page: %v", err)
			}
		}
	})

	b.Run("RegularRead", func(b *testing.B) {
		buf := make([]byte, PageSize)
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			offset := int64((i % 100) * PageSize)
			_, err := regularFile.ReadAt(buf, offset)
			if err != nil {
				b.Fatalf("failed to read: %v", err)
			}
		}
	})
}

func BenchmarkMmapSequentialRead(b *testing.B) {
	tmpDir := b.TempDir()
	filePath := filepath.Join(tmpDir, "bench_seq.db")

	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		b.Fatalf("failed to create test file: %v", err)
	}
	defer file.Close()

	numPages := 1000
	size := int64(PageSize * numPages)
	mm, err := NewMmapManager(file, size)
	if err != nil {
		b.Fatalf("failed to create mmap manager: %v", err)
	}
	defer mm.Close()

	// Hint sequential access
	_ = mm.MadviseSequential()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		for j := 0; j < numPages; j++ {
			_, err := mm.GetPage(PageID(j))
			if err != nil {
				b.Fatalf("failed to get page: %v", err)
			}
		}
	}
}

func BenchmarkMmapRandomRead(b *testing.B) {
	tmpDir := b.TempDir()
	filePath := filepath.Join(tmpDir, "bench_random.db")

	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		b.Fatalf("failed to create test file: %v", err)
	}
	defer file.Close()

	numPages := 1000
	size := int64(PageSize * numPages)
	mm, err := NewMmapManager(file, size)
	if err != nil {
		b.Fatalf("failed to create mmap manager: %v", err)
	}
	defer mm.Close()

	// Hint random access
	_ = mm.MadviseRandom()

	// Generate pseudo-random page access pattern
	pattern := make([]PageID, numPages)
	for i := range pattern {
		pattern[i] = PageID((i * 37) % numPages) // Simple LCG-like pattern
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		for _, pageID := range pattern {
			_, err := mm.GetPage(pageID)
			if err != nil {
				b.Fatalf("failed to get page: %v", err)
			}
		}
	}
}
