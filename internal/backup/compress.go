// Package backup provides backup and restore functionality for ObaDB.
package backup

import (
	"encoding/binary"
	"io"
)

// Compression constants for LZ4-style compression.
const (
	// MinMatchLength is the minimum length for a match.
	MinMatchLength = 4

	// MaxMatchLength is the maximum length for a match.
	MaxMatchLength = 255 + MinMatchLength

	// MaxOffset is the maximum offset for a match (16-bit).
	MaxOffset = 65535

	// HashTableSize is the size of the hash table for compression.
	HashTableSize = 1 << 14 // 16384 entries

	// CompressBlockSize is the size of each compression block.
	CompressBlockSize = 64 * 1024 // 64KB blocks

	// LiteralRunMask is the mask for literal run length in token.
	LiteralRunMask = 0xF0

	// MatchLengthMask is the mask for match length in token.
	MatchLengthMask = 0x0F
)

// CompressWriter wraps an io.Writer and compresses data using LZ4-style compression.
type CompressWriter struct {
	w          io.Writer
	buffer     []byte
	bufferPos  int
	hashTable  []int
	written    int64
	totalInput int64
	closed     bool
}

// NewCompressWriter creates a new compression writer.
func NewCompressWriter(w io.Writer) *CompressWriter {
	return &CompressWriter{
		w:         w,
		buffer:    make([]byte, CompressBlockSize),
		bufferPos: 0,
		hashTable: make([]int, HashTableSize),
		written:   0,
		closed:    false,
	}
}

// Write writes data to the compression buffer.
// Data is compressed and written when the buffer is full.
func (cw *CompressWriter) Write(p []byte) (n int, err error) {
	if cw.closed {
		return 0, io.ErrClosedPipe
	}

	total := 0
	for len(p) > 0 {
		// Fill buffer
		space := len(cw.buffer) - cw.bufferPos
		if space == 0 {
			// Buffer full, compress and write
			if err := cw.flushBuffer(); err != nil {
				return total, err
			}
			space = len(cw.buffer)
		}

		// Copy to buffer
		toCopy := len(p)
		if toCopy > space {
			toCopy = space
		}
		copy(cw.buffer[cw.bufferPos:], p[:toCopy])
		cw.bufferPos += toCopy
		cw.totalInput += int64(toCopy)
		p = p[toCopy:]
		total += toCopy
	}

	return total, nil
}

// flushBuffer compresses and writes the buffer contents.
func (cw *CompressWriter) flushBuffer() error {
	if cw.bufferPos == 0 {
		return nil
	}

	// Compress the buffer
	compressed := cw.compress(cw.buffer[:cw.bufferPos])

	// Write block header: original size (4 bytes) + compressed size (4 bytes)
	header := make([]byte, 8)
	binary.LittleEndian.PutUint32(header[0:4], uint32(cw.bufferPos))
	binary.LittleEndian.PutUint32(header[4:8], uint32(len(compressed)))

	if _, err := cw.w.Write(header); err != nil {
		return err
	}
	cw.written += 8

	// Write compressed data
	if _, err := cw.w.Write(compressed); err != nil {
		return err
	}
	cw.written += int64(len(compressed))

	// Reset buffer
	cw.bufferPos = 0

	// Reset hash table
	for i := range cw.hashTable {
		cw.hashTable[i] = 0
	}

	return nil
}

// Close flushes any remaining data and closes the writer.
func (cw *CompressWriter) Close() error {
	if cw.closed {
		return nil
	}
	cw.closed = true

	// Flush remaining buffer
	if err := cw.flushBuffer(); err != nil {
		return err
	}

	// Write end marker (zero-length block)
	endMarker := make([]byte, 8)
	if _, err := cw.w.Write(endMarker); err != nil {
		return err
	}
	cw.written += 8

	return nil
}

// Written returns the total compressed bytes written.
func (cw *CompressWriter) Written() int64 {
	return cw.written
}

// TotalInput returns the total uncompressed bytes received.
func (cw *CompressWriter) TotalInput() int64 {
	return cw.totalInput
}

// compress compresses data using LZ4-style algorithm.
func (cw *CompressWriter) compress(src []byte) []byte {
	if len(src) == 0 {
		return nil
	}

	// Worst case: no compression, need space for literals
	dst := make([]byte, 0, len(src)+len(src)/255+16)

	srcPos := 0
	literalStart := 0

	for srcPos < len(src)-MinMatchLength {
		// Calculate hash for current position
		hash := cw.hash4(src[srcPos:])
		matchPos := cw.hashTable[hash]
		cw.hashTable[hash] = srcPos

		// Check for match
		if matchPos > 0 && srcPos-matchPos <= MaxOffset {
			matchLen := cw.findMatchLength(src, matchPos, srcPos)
			if matchLen >= MinMatchLength {
				// Encode literals before match
				dst = cw.encodeLiteralsAndMatch(dst, src[literalStart:srcPos], srcPos-matchPos, matchLen)

				// Advance past match
				srcPos += matchLen
				literalStart = srcPos
				continue
			}
		}

		srcPos++
	}

	// Encode remaining literals
	if literalStart < len(src) {
		dst = cw.encodeLiterals(dst, src[literalStart:])
	}

	return dst
}

// hash4 calculates a hash for 4 bytes.
func (cw *CompressWriter) hash4(data []byte) int {
	if len(data) < 4 {
		return 0
	}
	v := binary.LittleEndian.Uint32(data)
	return int((v*2654435761)>>18) & (HashTableSize - 1)
}

// findMatchLength finds the length of a match.
func (cw *CompressWriter) findMatchLength(src []byte, matchPos, srcPos int) int {
	maxLen := len(src) - srcPos
	if maxLen > MaxMatchLength {
		maxLen = MaxMatchLength
	}

	length := 0
	for length < maxLen && src[matchPos+length] == src[srcPos+length] {
		length++
	}
	return length
}

// encodeLiteralsAndMatch encodes literals followed by a match.
func (cw *CompressWriter) encodeLiteralsAndMatch(dst, literals []byte, offset, matchLen int) []byte {
	literalLen := len(literals)
	matchLenEncoded := matchLen - MinMatchLength

	// Token byte: high 4 bits = literal length, low 4 bits = match length
	token := byte(0)
	if literalLen >= 15 {
		token = 0xF0
	} else {
		token = byte(literalLen << 4)
	}
	if matchLenEncoded >= 15 {
		token |= 0x0F
	} else {
		token |= byte(matchLenEncoded)
	}
	dst = append(dst, token)

	// Extended literal length
	if literalLen >= 15 {
		remaining := literalLen - 15
		for remaining >= 255 {
			dst = append(dst, 255)
			remaining -= 255
		}
		dst = append(dst, byte(remaining))
	}

	// Literals
	dst = append(dst, literals...)

	// Offset (little-endian 16-bit)
	dst = append(dst, byte(offset), byte(offset>>8))

	// Extended match length
	if matchLenEncoded >= 15 {
		remaining := matchLenEncoded - 15
		for remaining >= 255 {
			dst = append(dst, 255)
			remaining -= 255
		}
		dst = append(dst, byte(remaining))
	}

	return dst
}

// encodeLiterals encodes only literals (no match).
func (cw *CompressWriter) encodeLiterals(dst, literals []byte) []byte {
	literalLen := len(literals)
	if literalLen == 0 {
		return dst
	}

	// Token byte: high 4 bits = literal length, low 4 bits = 0 (no match)
	token := byte(0)
	if literalLen >= 15 {
		token = 0xF0
	} else {
		token = byte(literalLen << 4)
	}
	dst = append(dst, token)

	// Extended literal length
	if literalLen >= 15 {
		remaining := literalLen - 15
		for remaining >= 255 {
			dst = append(dst, 255)
			remaining -= 255
		}
		dst = append(dst, byte(remaining))
	}

	// Literals
	dst = append(dst, literals...)

	return dst
}

// DecompressReader wraps an io.Reader and decompresses LZ4-style compressed data.
type DecompressReader struct {
	r         io.Reader
	buffer    []byte
	bufferPos int
	bufferLen int
	totalRead int64
	eof       bool
}

// NewDecompressReader creates a new decompression reader.
func NewDecompressReader(r io.Reader) *DecompressReader {
	return &DecompressReader{
		r:         r,
		buffer:    nil,
		bufferPos: 0,
		bufferLen: 0,
		totalRead: 0,
		eof:       false,
	}
}

// Read reads decompressed data.
func (dr *DecompressReader) Read(p []byte) (n int, err error) {
	if dr.eof && dr.bufferPos >= dr.bufferLen {
		return 0, io.EOF
	}

	total := 0
	for len(p) > 0 {
		// If buffer is empty, read and decompress next block
		if dr.bufferPos >= dr.bufferLen {
			if err := dr.readBlock(); err != nil {
				if err == io.EOF {
					dr.eof = true
					if total > 0 {
						return total, nil
					}
					return 0, io.EOF
				}
				return total, err
			}
		}

		// Copy from buffer
		available := dr.bufferLen - dr.bufferPos
		toCopy := len(p)
		if toCopy > available {
			toCopy = available
		}
		copy(p, dr.buffer[dr.bufferPos:dr.bufferPos+toCopy])
		dr.bufferPos += toCopy
		p = p[toCopy:]
		total += toCopy
	}

	return total, nil
}

// readBlock reads and decompresses the next block.
func (dr *DecompressReader) readBlock() error {
	// Read block header
	header := make([]byte, 8)
	if _, err := io.ReadFull(dr.r, header); err != nil {
		return err
	}

	originalSize := binary.LittleEndian.Uint32(header[0:4])
	compressedSize := binary.LittleEndian.Uint32(header[4:8])

	// End marker
	if originalSize == 0 && compressedSize == 0 {
		return io.EOF
	}

	// Read compressed data
	compressed := make([]byte, compressedSize)
	if _, err := io.ReadFull(dr.r, compressed); err != nil {
		return err
	}

	// Decompress
	dr.buffer = dr.decompress(compressed, int(originalSize))
	dr.bufferPos = 0
	dr.bufferLen = len(dr.buffer)
	dr.totalRead += int64(dr.bufferLen)

	return nil
}

// decompress decompresses LZ4-style compressed data.
func (dr *DecompressReader) decompress(src []byte, originalSize int) []byte {
	if len(src) == 0 {
		return nil
	}

	dst := make([]byte, 0, originalSize)
	srcPos := 0

	for srcPos < len(src) {
		// Read token
		token := src[srcPos]
		srcPos++

		// Literal length
		literalLen := int(token >> 4)
		if literalLen == 15 {
			for srcPos < len(src) {
				extra := int(src[srcPos])
				srcPos++
				literalLen += extra
				if extra != 255 {
					break
				}
			}
		}

		// Copy literals
		if literalLen > 0 {
			if srcPos+literalLen > len(src) {
				break
			}
			dst = append(dst, src[srcPos:srcPos+literalLen]...)
			srcPos += literalLen
		}

		// Check if we have a match
		if srcPos >= len(src) {
			break
		}

		// Read offset
		if srcPos+2 > len(src) {
			break
		}
		offset := int(src[srcPos]) | int(src[srcPos+1])<<8
		srcPos += 2

		// Match length
		matchLen := int(token & 0x0F)
		if matchLen == 15 {
			for srcPos < len(src) {
				extra := int(src[srcPos])
				srcPos++
				matchLen += extra
				if extra != 255 {
					break
				}
			}
		}
		matchLen += MinMatchLength

		// Copy match
		if offset > 0 && offset <= len(dst) {
			matchStart := len(dst) - offset
			for i := 0; i < matchLen; i++ {
				dst = append(dst, dst[matchStart+i])
			}
		}
	}

	return dst
}

// TotalRead returns the total decompressed bytes read.
func (dr *DecompressReader) TotalRead() int64 {
	return dr.totalRead
}
