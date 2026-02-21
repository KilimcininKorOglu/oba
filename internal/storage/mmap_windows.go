//go:build windows

// Package storage provides the core storage engine components for ObaDB.
package storage

import (
	"syscall"
	"unsafe"
)

var (
	modkernel32           = syscall.NewLazyDLL("kernel32.dll")
	procCreateFileMapping = modkernel32.NewProc("CreateFileMappingW")
	procMapViewOfFile     = modkernel32.NewProc("MapViewOfFile")
	procUnmapViewOfFile   = modkernel32.NewProc("UnmapViewOfFile")
	procFlushViewOfFile   = modkernel32.NewProc("FlushViewOfFile")
	procVirtualLock       = modkernel32.NewProc("VirtualLock")
	procVirtualUnlock     = modkernel32.NewProc("VirtualUnlock")
)

const (
	pageReadonly  = 0x02
	pageReadWrite = 0x04
	fileMapRead   = 0x04
	fileMapWrite  = 0x02
)

// mapFile performs the actual memory mapping using Windows API.
func (m *MmapManager) mapFile() error {
	if m.data != nil {
		return ErrMmapAlreadyMapped
	}

	// Determine protection flags
	prot := uint32(pageReadonly)
	access := uint32(fileMapRead)
	if !m.readOnly {
		prot = pageReadWrite
		access = fileMapWrite | fileMapRead
	}

	// Get file handle
	handle := syscall.Handle(m.file.Fd())

	// Create file mapping
	sizeLow := uint32(m.size)
	sizeHigh := uint32(m.size >> 32)

	mapHandle, _, err := procCreateFileMapping.Call(
		uintptr(handle),
		0,
		uintptr(prot),
		uintptr(sizeHigh),
		uintptr(sizeLow),
		0,
	)
	if mapHandle == 0 {
		return err
	}

	// Map view of file
	addr, _, err := procMapViewOfFile.Call(
		mapHandle,
		uintptr(access),
		0,
		0,
		uintptr(m.size),
	)
	if addr == 0 {
		syscall.CloseHandle(syscall.Handle(mapHandle))
		return err
	}

	// Store the mapping handle for later cleanup
	m.mapHandle = mapHandle

	// Create slice from mapped memory
	m.data = unsafe.Slice((*byte)(unsafe.Pointer(addr)), m.size)

	return nil
}

// unmapFile unmaps the memory-mapped region.
func (m *MmapManager) unmapFile() error {
	if m.data == nil {
		return nil
	}

	addr := uintptr(unsafe.Pointer(&m.data[0]))

	ret, _, err := procUnmapViewOfFile.Call(addr)
	if ret == 0 {
		return err
	}

	// Close the mapping handle
	if m.mapHandle != 0 {
		syscall.CloseHandle(syscall.Handle(m.mapHandle))
		m.mapHandle = 0
	}

	m.data = nil
	return nil
}

// syncFile flushes changes to the underlying file.
func (m *MmapManager) syncFile() error {
	if m.data == nil {
		return ErrMmapNotMapped
	}

	addr := uintptr(unsafe.Pointer(&m.data[0]))

	ret, _, err := procFlushViewOfFile.Call(addr, uintptr(len(m.data)))
	if ret == 0 {
		return err
	}

	return nil
}

// Advise is a no-op on Windows as madvise is not available.
func (m *MmapManager) Advise(advice int) error {
	return nil
}

// MadviseSequential is a no-op on Windows.
func (m *MmapManager) MadviseSequential() error {
	return nil
}

// MadviseRandom is a no-op on Windows.
func (m *MmapManager) MadviseRandom() error {
	return nil
}

// MadviseWillNeed is a no-op on Windows.
func (m *MmapManager) MadviseWillNeed() error {
	return nil
}

// MadviseDontNeed is a no-op on Windows.
func (m *MmapManager) MadviseDontNeed() error {
	return nil
}

// Lock locks the mapped pages in memory.
func (m *MmapManager) Lock() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		return ErrMmapClosed
	}

	if m.data == nil {
		return ErrMmapNotMapped
	}

	addr := uintptr(unsafe.Pointer(&m.data[0]))
	ret, _, err := procVirtualLock.Call(addr, uintptr(len(m.data)))
	if ret == 0 {
		return err
	}

	return nil
}

// Unlock unlocks the mapped pages.
func (m *MmapManager) Unlock() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		return ErrMmapClosed
	}

	if m.data == nil {
		return ErrMmapNotMapped
	}

	addr := uintptr(unsafe.Pointer(&m.data[0]))
	ret, _, err := procVirtualUnlock.Call(addr, uintptr(len(m.data)))
	if ret == 0 {
		return err
	}

	return nil
}
