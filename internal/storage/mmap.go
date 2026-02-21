// Package storage provides the core storage engine components for ObaDB.
package storage

import (
	"errors"
	"os"
	"sync"
)

// MmapManager errors.
var (
	ErrMmapNotMapped      = errors.New("file is not memory mapped")
	ErrMmapAlreadyMapped  = errors.New("file is already memory mapped")
	ErrMmapInvalidSize    = errors.New("invalid mmap size")
	ErrMmapClosed         = errors.New("mmap manager is closed")
	ErrMmapReadOnly       = errors.New("mmap is read-only")
	ErrMmapPageOutOfRange = errors.New("page ID out of mmap range")
	ErrMmapRemapFailed    = errors.New("failed to remap file")
	ErrMmapSyncFailed     = errors.New("failed to sync mmap")
)

// MmapManager provides memory-mapped file I/O for zero-copy reads.
// It maps a file into memory, allowing direct access to file contents
// without explicit read/write system calls.
type MmapManager struct {
	file      *os.File
	data      []byte // mmap'd region
	size      int64  // current mapped size
	pageSize  int    // page size for alignment
	readOnly  bool   // whether mapping is read-only
	mu        sync.RWMutex
	closed    bool
	mapHandle uintptr // Windows file mapping handle (unused on Unix)
}

// MmapOptions configures the MmapManager.
type MmapOptions struct {
	PageSize int  // Page size for alignment (default: system page size)
	ReadOnly bool // Open in read-only mode
}

// DefaultMmapOptions returns the default MmapManager options.
func DefaultMmapOptions() MmapOptions {
	return MmapOptions{
		PageSize: PageSize,
		ReadOnly: false,
	}
}

// NewMmapManager creates a new MmapManager for the given file.
// The file must be opened with appropriate permissions.
// Size specifies the initial mapping size; if 0, the current file size is used.
func NewMmapManager(file *os.File, size int64) (*MmapManager, error) {
	return NewMmapManagerWithOptions(file, size, DefaultMmapOptions())
}

// NewMmapManagerWithOptions creates a new MmapManager with custom options.
func NewMmapManagerWithOptions(file *os.File, size int64, opts MmapOptions) (*MmapManager, error) {
	if file == nil {
		return nil, ErrFileNotOpen
	}

	if opts.PageSize <= 0 {
		opts.PageSize = PageSize
	}

	// Get file info to determine size if not specified
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}

	if size <= 0 {
		size = info.Size()
	}

	// Ensure size is at least one page
	if size < int64(opts.PageSize) {
		size = int64(opts.PageSize)
	}

	// Align size to page boundary
	size = alignToPageSize(size, opts.PageSize)

	// Ensure file is large enough
	if info.Size() < size && !opts.ReadOnly {
		if err := file.Truncate(size); err != nil {
			return nil, err
		}
	}

	m := &MmapManager{
		file:     file,
		pageSize: opts.PageSize,
		size:     size,
		readOnly: opts.ReadOnly,
		closed:   false,
	}

	// Perform the actual memory mapping
	if err := m.mapFile(); err != nil {
		return nil, err
	}

	return m, nil
}

// alignToPageSize aligns a size to the nearest page boundary.
func alignToPageSize(size int64, pageSize int) int64 {
	ps := int64(pageSize)
	if size%ps == 0 {
		return size
	}
	return ((size / ps) + 1) * ps
}

// Close unmaps the file and releases resources.
func (m *MmapManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return ErrMmapClosed
	}

	m.closed = true

	if m.data == nil {
		return nil
	}

	return m.unmapFile()
}

// GetPage returns a slice into the mmap'd region for the given page ID.
// This is a zero-copy operation - the returned slice points directly
// to the memory-mapped region.
func (m *MmapManager) GetPage(id PageID) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		return nil, ErrMmapClosed
	}

	if m.data == nil {
		return nil, ErrMmapNotMapped
	}

	offset := int64(id) * int64(m.pageSize)
	end := offset + int64(m.pageSize)

	if end > m.size {
		return nil, ErrMmapPageOutOfRange
	}

	return m.data[offset:end], nil
}

// GetPageRange returns a slice covering multiple consecutive pages.
func (m *MmapManager) GetPageRange(startID PageID, count int) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		return nil, ErrMmapClosed
	}

	if m.data == nil {
		return nil, ErrMmapNotMapped
	}

	if count <= 0 {
		return nil, ErrMmapInvalidSize
	}

	offset := int64(startID) * int64(m.pageSize)
	end := offset + int64(count)*int64(m.pageSize)

	if end > m.size {
		return nil, ErrMmapPageOutOfRange
	}

	return m.data[offset:end], nil
}

// GetData returns the entire mmap'd region.
// Use with caution - this exposes the entire mapped memory.
func (m *MmapManager) GetData() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		return nil, ErrMmapClosed
	}

	if m.data == nil {
		return nil, ErrMmapNotMapped
	}

	return m.data, nil
}

// Remap grows or shrinks the mmap region to the new size.
// This is used when the underlying file grows.
func (m *MmapManager) Remap(newSize int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return ErrMmapClosed
	}

	if m.readOnly {
		return ErrMmapReadOnly
	}

	if newSize <= 0 {
		return ErrMmapInvalidSize
	}

	// Align to page boundary
	newSize = alignToPageSize(newSize, m.pageSize)

	if newSize == m.size {
		return nil
	}

	// Unmap current region
	if m.data != nil {
		if err := m.unmapFile(); err != nil {
			return err
		}
	}

	// Extend file if needed
	info, err := m.file.Stat()
	if err != nil {
		return err
	}

	if info.Size() < newSize {
		if err := m.file.Truncate(newSize); err != nil {
			return err
		}
	}

	// Update size and remap
	m.size = newSize
	return m.mapFile()
}

// Sync flushes changes to the underlying file (msync).
func (m *MmapManager) Sync() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		return ErrMmapClosed
	}

	if m.data == nil {
		return ErrMmapNotMapped
	}

	return m.syncFile()
}

// Size returns the current mapped size in bytes.
func (m *MmapManager) Size() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.size
}

// PageSize returns the page size used for alignment.
func (m *MmapManager) PageSize() int {
	return m.pageSize
}

// PageCount returns the number of pages in the mapped region.
func (m *MmapManager) PageCount() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.size / int64(m.pageSize)
}

// IsReadOnly returns true if the mapping is read-only.
func (m *MmapManager) IsReadOnly() bool {
	return m.readOnly
}

// IsMapped returns true if the file is currently mapped.
func (m *MmapManager) IsMapped() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.data != nil && !m.closed
}

// File returns the underlying file.
func (m *MmapManager) File() *os.File {
	return m.file
}

// ReadAt reads data from the mmap'd region at the given offset.
// This is provided for compatibility but GetPage is preferred for page access.
func (m *MmapManager) ReadAt(p []byte, off int64) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		return 0, ErrMmapClosed
	}

	if m.data == nil {
		return 0, ErrMmapNotMapped
	}

	if off < 0 || off >= m.size {
		return 0, ErrMmapPageOutOfRange
	}

	n := copy(p, m.data[off:])
	return n, nil
}

// WriteAt writes data to the mmap'd region at the given offset.
// Changes are written directly to the mapped memory.
func (m *MmapManager) WriteAt(p []byte, off int64) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		return 0, ErrMmapClosed
	}

	if m.readOnly {
		return 0, ErrMmapReadOnly
	}

	if m.data == nil {
		return 0, ErrMmapNotMapped
	}

	if off < 0 || off >= m.size {
		return 0, ErrMmapPageOutOfRange
	}

	n := copy(m.data[off:], p)
	return n, nil
}
