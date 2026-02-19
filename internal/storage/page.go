// Package storage provides the core storage engine components for ObaDB.
package storage

import (
	"encoding/binary"
	"errors"
	"hash/crc32"
)

// PageSize is the default page size in bytes.
const PageSize = 4096

// PageHeaderSize is the size of the page header in bytes.
const PageHeaderSize = 16

// PageType represents the type of a page in the database.
type PageType uint8

const (
	// PageTypeFree indicates a free/unused page.
	PageTypeFree PageType = iota
	// PageTypeData indicates a data page containing entries.
	PageTypeData
	// PageTypeDNIndex indicates a DN radix tree index page.
	PageTypeDNIndex
	// PageTypeAttrIndex indicates an attribute B+ tree index page.
	PageTypeAttrIndex
	// PageTypeOverflow indicates an overflow page for large entries.
	PageTypeOverflow
	// PageTypeWAL indicates a write-ahead log page.
	PageTypeWAL
)

// String returns the string representation of a PageType.
func (pt PageType) String() string {
	switch pt {
	case PageTypeFree:
		return "Free"
	case PageTypeData:
		return "Data"
	case PageTypeDNIndex:
		return "DNIndex"
	case PageTypeAttrIndex:
		return "AttrIndex"
	case PageTypeOverflow:
		return "Overflow"
	case PageTypeWAL:
		return "WAL"
	default:
		return "Unknown"
	}
}

// PageFlag represents flags for a page.
type PageFlag uint8

const (
	// PageFlagDirty indicates the page has been modified.
	PageFlagDirty PageFlag = 1 << iota
	// PageFlagPinned indicates the page is pinned in memory.
	PageFlagPinned
	// PageFlagLeaf indicates the page is a leaf node (for tree structures).
	PageFlagLeaf
)

// PageID represents a unique identifier for a page.
type PageID uint64

// PageHeader represents the header of each page (first 16 bytes).
// Layout:
//   - Bytes 0-7:   PageID (uint64)
//   - Byte 8:      PageType (uint8)
//   - Byte 9:      Flags (uint8)
//   - Bytes 10-11: ItemCount (uint16)
//   - Bytes 12-13: FreeSpace (uint16)
//   - Bytes 14-15: Checksum (uint16)
type PageHeader struct {
	PageID    PageID   // This page's ID
	PageType  PageType // Data, Index, Free, Overflow
	Flags     PageFlag // Dirty, Pinned, etc.
	ItemCount uint16   // Number of items in page
	FreeSpace uint16   // Bytes of free space
	Checksum  uint16   // CRC16 of page content
}

// Errors for page operations.
var (
	ErrInvalidPageSize     = errors.New("invalid page size")
	ErrInvalidChecksum     = errors.New("page checksum mismatch")
	ErrInvalidPageType     = errors.New("invalid page type")
	ErrInsufficientSpace   = errors.New("insufficient space in page")
	ErrPageHeaderCorrupted = errors.New("page header corrupted")
)

// NewPageHeader creates a new PageHeader with the given parameters.
func NewPageHeader(pageID PageID, pageType PageType) *PageHeader {
	return &PageHeader{
		PageID:    pageID,
		PageType:  pageType,
		Flags:     0,
		ItemCount: 0,
		FreeSpace: PageSize - PageHeaderSize,
		Checksum:  0,
	}
}

// Serialize writes the PageHeader to a byte slice.
// The slice must be at least PageHeaderSize bytes.
func (h *PageHeader) Serialize(buf []byte) error {
	if len(buf) < PageHeaderSize {
		return ErrInvalidPageSize
	}

	binary.LittleEndian.PutUint64(buf[0:8], uint64(h.PageID))
	buf[8] = byte(h.PageType)
	buf[9] = byte(h.Flags)
	binary.LittleEndian.PutUint16(buf[10:12], h.ItemCount)
	binary.LittleEndian.PutUint16(buf[12:14], h.FreeSpace)
	binary.LittleEndian.PutUint16(buf[14:16], h.Checksum)

	return nil
}

// Deserialize reads the PageHeader from a byte slice.
// The slice must be at least PageHeaderSize bytes.
func (h *PageHeader) Deserialize(buf []byte) error {
	if len(buf) < PageHeaderSize {
		return ErrInvalidPageSize
	}

	h.PageID = PageID(binary.LittleEndian.Uint64(buf[0:8]))
	h.PageType = PageType(buf[8])
	h.Flags = PageFlag(buf[9])
	h.ItemCount = binary.LittleEndian.Uint16(buf[10:12])
	h.FreeSpace = binary.LittleEndian.Uint16(buf[12:14])
	h.Checksum = binary.LittleEndian.Uint16(buf[14:16])

	return nil
}

// SetDirty sets the dirty flag on the page header.
func (h *PageHeader) SetDirty() {
	h.Flags |= PageFlagDirty
}

// ClearDirty clears the dirty flag on the page header.
func (h *PageHeader) ClearDirty() {
	h.Flags &^= PageFlagDirty
}

// IsDirty returns true if the page is marked as dirty.
func (h *PageHeader) IsDirty() bool {
	return h.Flags&PageFlagDirty != 0
}

// SetPinned sets the pinned flag on the page header.
func (h *PageHeader) SetPinned() {
	h.Flags |= PageFlagPinned
}

// ClearPinned clears the pinned flag on the page header.
func (h *PageHeader) ClearPinned() {
	h.Flags &^= PageFlagPinned
}

// IsPinned returns true if the page is pinned in memory.
func (h *PageHeader) IsPinned() bool {
	return h.Flags&PageFlagPinned != 0
}

// IsLeaf returns true if the page is a leaf node.
func (h *PageHeader) IsLeaf() bool {
	return h.Flags&PageFlagLeaf != 0
}

// SetLeaf sets the leaf flag on the page header.
func (h *PageHeader) SetLeaf() {
	h.Flags |= PageFlagLeaf
}

// Page represents a complete page in the database.
type Page struct {
	Header PageHeader
	Data   []byte // Page data excluding header
}

// NewPage creates a new page with the given ID and type.
func NewPage(pageID PageID, pageType PageType) *Page {
	return &Page{
		Header: PageHeader{
			PageID:    pageID,
			PageType:  pageType,
			Flags:     0,
			ItemCount: 0,
			FreeSpace: PageSize - PageHeaderSize,
			Checksum:  0,
		},
		Data: make([]byte, PageSize-PageHeaderSize),
	}
}

// Serialize writes the entire page to a byte slice.
// Returns a new byte slice of PageSize bytes.
func (p *Page) Serialize() ([]byte, error) {
	buf := make([]byte, PageSize)

	// Calculate checksum before serializing header
	p.Header.Checksum = p.CalculateChecksum()

	if err := p.Header.Serialize(buf[:PageHeaderSize]); err != nil {
		return nil, err
	}

	copy(buf[PageHeaderSize:], p.Data)

	return buf, nil
}

// SerializeTo writes the entire page to an existing byte slice.
// The slice must be at least PageSize bytes.
func (p *Page) SerializeTo(buf []byte) error {
	if len(buf) < PageSize {
		return ErrInvalidPageSize
	}

	// Calculate checksum before serializing header
	p.Header.Checksum = p.CalculateChecksum()

	if err := p.Header.Serialize(buf[:PageHeaderSize]); err != nil {
		return err
	}

	copy(buf[PageHeaderSize:], p.Data)

	return nil
}

// Deserialize reads the entire page from a byte slice.
// The slice must be at least PageSize bytes.
func (p *Page) Deserialize(buf []byte) error {
	if len(buf) < PageSize {
		return ErrInvalidPageSize
	}

	if err := p.Header.Deserialize(buf[:PageHeaderSize]); err != nil {
		return err
	}

	if p.Data == nil || len(p.Data) < PageSize-PageHeaderSize {
		p.Data = make([]byte, PageSize-PageHeaderSize)
	}

	copy(p.Data, buf[PageHeaderSize:PageSize])

	return nil
}

// CalculateChecksum computes the CRC16 checksum of the page data.
// Uses CRC32 internally and truncates to 16 bits for the page header.
func (p *Page) CalculateChecksum() uint16 {
	// Use CRC32 and truncate to 16 bits
	crc := crc32.ChecksumIEEE(p.Data)
	return uint16(crc & 0xFFFF)
}

// ValidateChecksum verifies the page checksum matches the stored value.
func (p *Page) ValidateChecksum() bool {
	return p.Header.Checksum == p.CalculateChecksum()
}

// DeserializeAndValidate reads the page and validates its checksum.
func (p *Page) DeserializeAndValidate(buf []byte) error {
	if err := p.Deserialize(buf); err != nil {
		return err
	}

	if !p.ValidateChecksum() {
		return ErrInvalidChecksum
	}

	return nil
}

// UsableSpace returns the amount of usable space in the page data area.
func (p *Page) UsableSpace() int {
	return PageSize - PageHeaderSize
}

// Reset clears the page data and resets the header.
func (p *Page) Reset(pageType PageType) {
	p.Header.PageType = pageType
	p.Header.Flags = 0
	p.Header.ItemCount = 0
	p.Header.FreeSpace = PageSize - PageHeaderSize
	p.Header.Checksum = 0

	// Clear data
	for i := range p.Data {
		p.Data[i] = 0
	}
}
