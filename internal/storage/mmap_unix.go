//go:build unix || darwin || linux

// Package storage provides the core storage engine components for ObaDB.
package storage

import (
	"syscall"
	"unsafe"
)

// mapFile performs the actual memory mapping using syscall.Mmap.
// This implementation works on Unix-like systems (macOS, Linux).
func (m *MmapManager) mapFile() error {
	if m.data != nil {
		return ErrMmapAlreadyMapped
	}

	// Determine protection flags
	prot := syscall.PROT_READ
	if !m.readOnly {
		prot |= syscall.PROT_WRITE
	}

	// Use MAP_SHARED to ensure changes are written to the file
	flags := syscall.MAP_SHARED

	// Get file descriptor
	fd := int(m.file.Fd())

	// Perform the mmap syscall
	data, err := syscall.Mmap(fd, 0, int(m.size), prot, flags)
	if err != nil {
		return err
	}

	m.data = data
	return nil
}

// unmapFile unmaps the memory-mapped region.
func (m *MmapManager) unmapFile() error {
	if m.data == nil {
		return nil
	}

	err := syscall.Munmap(m.data)
	m.data = nil
	return err
}

// syncFile flushes changes to the underlying file using msync.
func (m *MmapManager) syncFile() error {
	if m.data == nil {
		return ErrMmapNotMapped
	}

	// MS_SYNC: synchronous sync - wait for completion
	_, _, errno := syscall.Syscall(syscall.SYS_MSYNC,
		uintptr(unsafe.Pointer(&m.data[0])),
		uintptr(len(m.data)),
		uintptr(syscall.MS_SYNC))

	if errno != 0 {
		return errno
	}

	return nil
}

// Advise provides hints to the kernel about expected access patterns.
// This can improve performance by allowing the kernel to optimize paging.
func (m *MmapManager) Advise(advice int) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		return ErrMmapClosed
	}

	if m.data == nil {
		return ErrMmapNotMapped
	}

	_, _, errno := syscall.Syscall(syscall.SYS_MADVISE,
		uintptr(unsafe.Pointer(&m.data[0])),
		uintptr(len(m.data)),
		uintptr(advice))

	if errno != 0 {
		return errno
	}

	return nil
}

// MadviseSequential hints that pages will be accessed sequentially.
func (m *MmapManager) MadviseSequential() error {
	return m.Advise(syscall.MADV_SEQUENTIAL)
}

// MadviseRandom hints that pages will be accessed randomly.
func (m *MmapManager) MadviseRandom() error {
	return m.Advise(syscall.MADV_RANDOM)
}

// MadviseWillNeed hints that pages will be needed soon.
func (m *MmapManager) MadviseWillNeed() error {
	return m.Advise(syscall.MADV_WILLNEED)
}

// MadviseDontNeed hints that pages won't be needed soon.
func (m *MmapManager) MadviseDontNeed() error {
	return m.Advise(syscall.MADV_DONTNEED)
}

// Lock locks the mapped pages in memory, preventing them from being paged out.
// This requires appropriate privileges.
func (m *MmapManager) Lock() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		return ErrMmapClosed
	}

	if m.data == nil {
		return ErrMmapNotMapped
	}

	return syscall.Mlock(m.data)
}

// Unlock unlocks the mapped pages, allowing them to be paged out.
func (m *MmapManager) Unlock() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		return ErrMmapClosed
	}

	if m.data == nil {
		return ErrMmapNotMapped
	}

	return syscall.Munlock(m.data)
}
