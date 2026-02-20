// Package cache provides index cache persistence for fast startup.
package cache

import (
	"encoding/binary"
	"errors"
	"hash/crc32"
)

// Cache file constants.
const (
	Magic      = "OBAC"
	Version    = 1
	HeaderSize = 48
)

// Cache types.
const (
	TypeRadix uint8 = 1
	TypeBTree uint8 = 2
)

// Errors.
var (
	ErrInvalidMagic   = errors.New("invalid cache magic")
	ErrInvalidVersion = errors.New("invalid cache version")
	ErrInvalidType    = errors.New("invalid cache type")
	ErrStaleTxID      = errors.New("stale transaction ID")
	ErrCorruptData    = errors.New("corrupt cache data")
	ErrBufferTooSmall = errors.New("buffer too small")
)

// Header represents the cache file header.
// Total size: 48 bytes
//
// Layout:
//   - Bytes 0-3:   Magic ("OBAC")
//   - Bytes 4-7:   Version (uint32 LE)
//   - Byte 8:      CacheType (1=Radix, 2=BTree)
//   - Bytes 9-15:  Reserved (7 bytes)
//   - Bytes 16-23: EntryCount (uint64 LE)
//   - Bytes 24-31: LastTxID (uint64 LE)
//   - Bytes 32-35: DataCRC32 (uint32 LE)
//   - Bytes 36-43: DataLength (uint64 LE)
//   - Bytes 44-47: HeaderCRC32 (uint32 LE)
type Header struct {
	Magic       [4]byte
	Version     uint32
	CacheType   uint8
	Reserved    [7]byte
	EntryCount  uint64
	LastTxID    uint64
	DataCRC32   uint32
	DataLength  uint64
	HeaderCRC32 uint32
}

// NewHeader creates a new cache header.
func NewHeader(cacheType uint8, entryCount, lastTxID uint64, data []byte) *Header {
	h := &Header{
		Version:    Version,
		CacheType:  cacheType,
		EntryCount: entryCount,
		LastTxID:   lastTxID,
		DataCRC32:  crc32.ChecksumIEEE(data),
		DataLength: uint64(len(data)),
	}
	copy(h.Magic[:], Magic)
	return h
}

// Serialize writes the header to a byte slice.
func (h *Header) Serialize() []byte {
	buf := make([]byte, HeaderSize)

	copy(buf[0:4], h.Magic[:])
	binary.LittleEndian.PutUint32(buf[4:8], h.Version)
	buf[8] = h.CacheType
	// Reserved bytes 9-15 are zero
	binary.LittleEndian.PutUint64(buf[16:24], h.EntryCount)
	binary.LittleEndian.PutUint64(buf[24:32], h.LastTxID)
	binary.LittleEndian.PutUint32(buf[32:36], h.DataCRC32)
	binary.LittleEndian.PutUint64(buf[36:44], h.DataLength)

	// Calculate header CRC (excluding last 4 bytes)
	h.HeaderCRC32 = crc32.ChecksumIEEE(buf[:44])
	binary.LittleEndian.PutUint32(buf[44:48], h.HeaderCRC32)

	return buf
}

// Deserialize reads the header from a byte slice.
func (h *Header) Deserialize(buf []byte) error {
	if len(buf) < HeaderSize {
		return ErrBufferTooSmall
	}

	copy(h.Magic[:], buf[0:4])
	h.Version = binary.LittleEndian.Uint32(buf[4:8])
	h.CacheType = buf[8]
	copy(h.Reserved[:], buf[9:16])
	h.EntryCount = binary.LittleEndian.Uint64(buf[16:24])
	h.LastTxID = binary.LittleEndian.Uint64(buf[24:32])
	h.DataCRC32 = binary.LittleEndian.Uint32(buf[32:36])
	h.DataLength = binary.LittleEndian.Uint64(buf[36:44])
	h.HeaderCRC32 = binary.LittleEndian.Uint32(buf[44:48])

	return nil
}

// Validate validates the header.
func (h *Header) Validate(expectedType uint8, expectedTxID uint64) error {
	if string(h.Magic[:]) != Magic {
		return ErrInvalidMagic
	}

	if h.Version != Version {
		return ErrInvalidVersion
	}

	if h.CacheType != expectedType {
		return ErrInvalidType
	}

	if h.LastTxID != expectedTxID {
		return ErrStaleTxID
	}

	return nil
}

// ValidateHeaderCRC validates the header CRC.
func (h *Header) ValidateHeaderCRC(buf []byte) error {
	if len(buf) < HeaderSize {
		return ErrBufferTooSmall
	}

	expectedCRC := crc32.ChecksumIEEE(buf[:44])
	if h.HeaderCRC32 != expectedCRC {
		return ErrCorruptData
	}

	return nil
}

// ValidateDataCRC validates the data CRC.
func (h *Header) ValidateDataCRC(data []byte) error {
	if uint64(len(data)) != h.DataLength {
		return ErrCorruptData
	}

	if crc32.ChecksumIEEE(data) != h.DataCRC32 {
		return ErrCorruptData
	}

	return nil
}
