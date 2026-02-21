// Package storage provides the core storage engine components for ObaDB.
package storage

import (
	"encoding/binary"
	"errors"
	"hash/crc32"
)

// File header constants.
const (
	// FileHeaderSize is the size of the file header (first page).
	FileHeaderSize = PageSize

	// MagicNumber is the magic bytes identifying an ObaDB file.
	// "OBA\x00" in bytes.
	MagicByte0 = 'O'
	MagicByte1 = 'B'
	MagicByte2 = 'A'
	MagicByte3 = 0x00

	// CurrentVersion is the current file format version.
	CurrentVersion uint32 = 1

	// FileHeaderReservedSize is the size of reserved space in the header.
	FileHeaderReservedSize = 4020
)

// Magic is the magic number for ObaDB files.
var Magic = [4]byte{MagicByte0, MagicByte1, MagicByte2, MagicByte3}

// RootPages contains pointers to root pages for different structures.
type RootPages struct {
	DNIndex  PageID // Radix tree root for DN hierarchy
	DataRoot PageID // First data page
}

// FileHeader represents the header of an ObaDB data file (first 4KB page).
// Layout:
//   - Bytes 0-3:     Magic number ("OBA\x00")
//   - Bytes 4-7:     Version (uint32)
//   - Bytes 8-11:    PageSize (uint32)
//   - Bytes 12-19:   TotalPages (uint64)
//   - Bytes 20-27:   FreeListHead (PageID/uint64)
//   - Bytes 28-35:   RootPages.DNIndex (PageID/uint64)
//   - Bytes 36-43:   RootPages.DataRoot (PageID/uint64)
//   - Bytes 44-47:   Checksum (uint32)
//   - Bytes 48-4095: Reserved
type FileHeader struct {
	Magic        [4]byte   // "OBA\x00"
	Version      uint32    // File format version
	PageSize     uint32    // Page size in bytes (4096)
	TotalPages   uint64    // Total pages allocated
	FreeListHead PageID    // First page in free list
	RootPages    RootPages // Root page pointers
	Checksum     uint32    // CRC32 of header
	Reserved     [FileHeaderReservedSize]byte
}

// Errors for file header operations.
var (
	ErrInvalidMagic       = errors.New("invalid magic number: not an ObaDB file")
	ErrUnsupportedVersion = errors.New("unsupported file format version")
	ErrHeaderChecksum     = errors.New("file header checksum mismatch")
	ErrInvalidHeaderSize  = errors.New("invalid header size")
)

// NewFileHeader creates a new FileHeader with default values.
func NewFileHeader() *FileHeader {
	return &FileHeader{
		Magic:        Magic,
		Version:      CurrentVersion,
		PageSize:     PageSize,
		TotalPages:   1, // At least the header page
		FreeListHead: 0, // No free pages initially
		RootPages: RootPages{
			DNIndex:  0,
			DataRoot: 0,
		},
		Checksum: 0,
	}
}

// Serialize writes the FileHeader to a byte slice.
// Returns a new byte slice of FileHeaderSize (4096) bytes.
func (h *FileHeader) Serialize() ([]byte, error) {
	buf := make([]byte, FileHeaderSize)
	return buf, h.SerializeTo(buf)
}

// SerializeTo writes the FileHeader to an existing byte slice.
// The slice must be at least FileHeaderSize bytes.
func (h *FileHeader) SerializeTo(buf []byte) error {
	if len(buf) < FileHeaderSize {
		return ErrInvalidHeaderSize
	}

	// Clear the buffer first
	for i := range buf {
		buf[i] = 0
	}

	// Write magic number
	copy(buf[0:4], h.Magic[:])

	// Write version
	binary.LittleEndian.PutUint32(buf[4:8], h.Version)

	// Write page size
	binary.LittleEndian.PutUint32(buf[8:12], h.PageSize)

	// Write total pages
	binary.LittleEndian.PutUint64(buf[12:20], h.TotalPages)

	// Write free list head
	binary.LittleEndian.PutUint64(buf[20:28], uint64(h.FreeListHead))

	// Write root pages
	binary.LittleEndian.PutUint64(buf[28:36], uint64(h.RootPages.DNIndex))
	binary.LittleEndian.PutUint64(buf[36:44], uint64(h.RootPages.DataRoot))

	// Calculate and write checksum (checksum field is at bytes 44-47)
	// Checksum is calculated over bytes 0-43 (before checksum field)
	checksum := h.calculateChecksumFromBuffer(buf)
	binary.LittleEndian.PutUint32(buf[44:48], checksum)

	// Copy reserved bytes
	copy(buf[48:], h.Reserved[:])

	return nil
}

// Deserialize reads the FileHeader from a byte slice.
// The slice must be at least FileHeaderSize bytes.
func (h *FileHeader) Deserialize(buf []byte) error {
	if len(buf) < FileHeaderSize {
		return ErrInvalidHeaderSize
	}

	// Read magic number
	copy(h.Magic[:], buf[0:4])

	// Read version
	h.Version = binary.LittleEndian.Uint32(buf[4:8])

	// Read page size
	h.PageSize = binary.LittleEndian.Uint32(buf[8:12])

	// Read total pages
	h.TotalPages = binary.LittleEndian.Uint64(buf[12:20])

	// Read free list head
	h.FreeListHead = PageID(binary.LittleEndian.Uint64(buf[20:28]))

	// Read root pages
	h.RootPages.DNIndex = PageID(binary.LittleEndian.Uint64(buf[28:36]))
	h.RootPages.DataRoot = PageID(binary.LittleEndian.Uint64(buf[36:44]))

	// Read checksum
	h.Checksum = binary.LittleEndian.Uint32(buf[44:48])

	// Copy reserved bytes
	copy(h.Reserved[:], buf[48:])

	return nil
}

// ValidateMagic checks if the magic number is valid.
func (h *FileHeader) ValidateMagic() error {
	if h.Magic != Magic {
		return ErrInvalidMagic
	}
	return nil
}

// ValidateVersion checks if the file format version is supported.
func (h *FileHeader) ValidateVersion() error {
	if h.Version > CurrentVersion || h.Version == 0 {
		return ErrUnsupportedVersion
	}
	return nil
}

// calculateChecksumFromBuffer computes CRC32 checksum from the serialized buffer.
// Checksum is calculated over bytes 0-43 (header fields before checksum).
func (h *FileHeader) calculateChecksumFromBuffer(buf []byte) uint32 {
	return crc32.ChecksumIEEE(buf[0:44])
}

// CalculateChecksum computes the CRC32 checksum of the header fields.
// This serializes the header to a temporary buffer to calculate the checksum.
func (h *FileHeader) CalculateChecksum() uint32 {
	buf := make([]byte, 44)

	// Write fields to buffer for checksum calculation
	copy(buf[0:4], h.Magic[:])
	binary.LittleEndian.PutUint32(buf[4:8], h.Version)
	binary.LittleEndian.PutUint32(buf[8:12], h.PageSize)
	binary.LittleEndian.PutUint64(buf[12:20], h.TotalPages)
	binary.LittleEndian.PutUint64(buf[20:28], uint64(h.FreeListHead))
	binary.LittleEndian.PutUint64(buf[28:36], uint64(h.RootPages.DNIndex))
	binary.LittleEndian.PutUint64(buf[36:44], uint64(h.RootPages.DataRoot))

	return crc32.ChecksumIEEE(buf)
}

// ValidateChecksum verifies the header checksum matches the stored value.
func (h *FileHeader) ValidateChecksum() bool {
	return h.Checksum == h.CalculateChecksum()
}

// Validate performs all validation checks on the header.
func (h *FileHeader) Validate() error {
	if err := h.ValidateMagic(); err != nil {
		return err
	}

	if err := h.ValidateVersion(); err != nil {
		return err
	}

	if !h.ValidateChecksum() {
		return ErrHeaderChecksum
	}

	return nil
}

// DeserializeAndValidate reads the header and performs all validation checks.
func (h *FileHeader) DeserializeAndValidate(buf []byte) error {
	if err := h.Deserialize(buf); err != nil {
		return err
	}

	return h.Validate()
}

// UpdateChecksum recalculates and updates the checksum field.
func (h *FileHeader) UpdateChecksum() {
	h.Checksum = h.CalculateChecksum()
}

// IsMagicValid is a convenience method to check magic number validity.
func (h *FileHeader) IsMagicValid() bool {
	return h.Magic == Magic
}

// IsVersionSupported is a convenience method to check version support.
func (h *FileHeader) IsVersionSupported() bool {
	return h.Version > 0 && h.Version <= CurrentVersion
}

// ValidateMagicBytes checks if the given bytes represent a valid ObaDB magic number.
// This is a standalone function for quick file type detection.
func ValidateMagicBytes(magic [4]byte) bool {
	return magic == Magic
}

// ReadMagicFromBuffer extracts the magic number from a buffer.
// Returns an error if the buffer is too small.
func ReadMagicFromBuffer(buf []byte) ([4]byte, error) {
	var magic [4]byte
	if len(buf) < 4 {
		return magic, ErrInvalidHeaderSize
	}
	copy(magic[:], buf[0:4])
	return magic, nil
}

// IsObaDBFile checks if the given buffer starts with the ObaDB magic number.
// This is useful for quick file type detection without full deserialization.
func IsObaDBFile(buf []byte) bool {
	magic, err := ReadMagicFromBuffer(buf)
	if err != nil {
		return false
	}
	return ValidateMagicBytes(magic)
}
