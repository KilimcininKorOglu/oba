// Package ber implements ASN.1 BER (Basic Encoding Rules) encoding
// as specified in ITU-T X.690.
package ber

import (
	"errors"
)

// Errors returned by the encoder
var (
	ErrInvalidTagClass   = errors.New("ber: invalid tag class")
	ErrInvalidTagNumber  = errors.New("ber: invalid tag number")
	ErrLengthOverflow    = errors.New("ber: length value overflow")
	ErrNegativeLength    = errors.New("ber: negative length not allowed")
)

// BEREncoder encodes ASN.1 values using BER (Basic Encoding Rules).
type BEREncoder struct {
	buf []byte
}

// NewBEREncoder creates a new BER encoder with an optional initial capacity.
func NewBEREncoder(capacity int) *BEREncoder {
	if capacity <= 0 {
		capacity = 64
	}
	return &BEREncoder{
		buf: make([]byte, 0, capacity),
	}
}

// Bytes returns the encoded bytes.
func (e *BEREncoder) Bytes() []byte {
	return e.buf
}

// Reset clears the encoder buffer for reuse.
func (e *BEREncoder) Reset() {
	e.buf = e.buf[:0]
}

// Len returns the current length of encoded data.
func (e *BEREncoder) Len() int {
	return len(e.buf)
}

// WriteTag writes a BER tag byte(s) to the buffer.
// class: ClassUniversal, ClassApplication, ClassContextSpecific, or ClassPrivate
// constructed: TypePrimitive or TypeConstructed
// number: tag number (0-30 for short form, >30 for long form)
func (e *BEREncoder) WriteTag(class, constructed, number int) error {
	// Validate class
	if class != ClassUniversal && class != ClassApplication &&
		class != ClassContextSpecific && class != ClassPrivate {
		return ErrInvalidTagClass
	}

	// Validate tag number
	if number < 0 {
		return ErrInvalidTagNumber
	}

	// Short form: tag number fits in 5 bits (0-30)
	if number <= 30 {
		tag := byte(class) | byte(constructed) | byte(number)
		e.buf = append(e.buf, tag)
		return nil
	}

	// Long form: tag number > 30
	// First byte: class | constructed | 0x1F (all 5 bits set)
	firstByte := byte(class) | byte(constructed) | 0x1F
	e.buf = append(e.buf, firstByte)

	// Encode tag number in subsequent bytes using base-128
	e.writeBase128(number)
	return nil
}

// writeBase128 encodes an integer in base-128 format (high bit indicates continuation)
func (e *BEREncoder) writeBase128(value int) {
	if value == 0 {
		e.buf = append(e.buf, 0)
		return
	}

	// Calculate how many bytes we need
	var bytes []byte
	for value > 0 {
		bytes = append(bytes, byte(value&0x7F))
		value >>= 7
	}

	// Write bytes in reverse order with continuation bits
	for i := len(bytes) - 1; i >= 0; i-- {
		b := bytes[i]
		if i > 0 {
			b |= 0x80 // Set continuation bit for all but last byte
		}
		e.buf = append(e.buf, b)
	}
}

// WriteLength writes a BER length value to the buffer.
// Uses short form for lengths 0-127, long form for larger values.
func (e *BEREncoder) WriteLength(length int) error {
	if length < 0 {
		return ErrNegativeLength
	}

	// Short form: length fits in 7 bits (0-127)
	if length <= MaxShortFormLength {
		e.buf = append(e.buf, byte(length))
		return nil
	}

	// Long form: first byte indicates number of length bytes
	// Calculate number of bytes needed for length
	numBytes := 0
	temp := length
	for temp > 0 {
		numBytes++
		temp >>= 8
	}

	// Check for overflow (length byte can only indicate up to 127 subsequent bytes)
	if numBytes > 127 {
		return ErrLengthOverflow
	}

	// Write first byte: 0x80 | number of length bytes
	e.buf = append(e.buf, byte(LengthLongFormBit|numBytes))

	// Write length bytes in big-endian order
	for i := numBytes - 1; i >= 0; i-- {
		e.buf = append(e.buf, byte(length>>(i*8)))
	}

	return nil
}

// WriteBoolean writes a BER-encoded boolean value.
// Per X.690, FALSE is encoded as 0x00, TRUE as any non-zero value (we use 0xFF).
func (e *BEREncoder) WriteBoolean(v bool) error {
	if err := e.WriteTag(ClassUniversal, TypePrimitive, TagBoolean); err != nil {
		return err
	}
	if err := e.WriteLength(1); err != nil {
		return err
	}
	if v {
		e.buf = append(e.buf, 0xFF)
	} else {
		e.buf = append(e.buf, 0x00)
	}
	return nil
}

// WriteInteger writes a BER-encoded integer value.
// Uses the minimum number of octets with two's complement representation.
func (e *BEREncoder) WriteInteger(v int64) error {
	if err := e.WriteTag(ClassUniversal, TypePrimitive, TagInteger); err != nil {
		return err
	}

	// Encode the integer value
	encoded := encodeInteger(v)

	if err := e.WriteLength(len(encoded)); err != nil {
		return err
	}
	e.buf = append(e.buf, encoded...)
	return nil
}

// encodeInteger encodes an int64 as a minimal two's complement byte slice.
func encodeInteger(v int64) []byte {
	// Special case for zero
	if v == 0 {
		return []byte{0x00}
	}

	// Convert to unsigned for bit manipulation
	var bytes []byte
	uv := uint64(v)

	// For negative numbers, we need to handle two's complement
	if v < 0 {
		// Find the minimum number of bytes needed
		// A negative number needs enough bytes to preserve the sign bit
		for i := 7; i >= 0; i-- {
			b := byte(uv >> (i * 8))
			if len(bytes) > 0 || b != 0xFF || (i > 0 && (uv>>((i-1)*8))&0x80 == 0) {
				bytes = append(bytes, b)
			}
		}
		// Ensure at least one byte
		if len(bytes) == 0 {
			bytes = []byte{0xFF}
		}
		// Ensure sign bit is set (for negative numbers)
		if bytes[0]&0x80 == 0 {
			bytes = append([]byte{0xFF}, bytes...)
		}
	} else {
		// Positive number: find first non-zero byte
		for i := 7; i >= 0; i-- {
			b := byte(uv >> (i * 8))
			if len(bytes) > 0 || b != 0 {
				bytes = append(bytes, b)
			}
		}
		// Ensure sign bit is clear (for positive numbers)
		// If high bit is set, prepend 0x00
		if len(bytes) > 0 && bytes[0]&0x80 != 0 {
			bytes = append([]byte{0x00}, bytes...)
		}
	}

	return bytes
}

// WriteOctetString writes a BER-encoded octet string.
func (e *BEREncoder) WriteOctetString(v []byte) error {
	if err := e.WriteTag(ClassUniversal, TypePrimitive, TagOctetString); err != nil {
		return err
	}
	if err := e.WriteLength(len(v)); err != nil {
		return err
	}
	e.buf = append(e.buf, v...)
	return nil
}

// WriteEnumerated writes a BER-encoded enumerated value.
// Enumerated values are encoded identically to integers.
func (e *BEREncoder) WriteEnumerated(v int64) error {
	if err := e.WriteTag(ClassUniversal, TypePrimitive, TagEnumerated); err != nil {
		return err
	}

	encoded := encodeInteger(v)

	if err := e.WriteLength(len(encoded)); err != nil {
		return err
	}
	e.buf = append(e.buf, encoded...)
	return nil
}

// WriteNull writes a BER-encoded null value.
func (e *BEREncoder) WriteNull() error {
	if err := e.WriteTag(ClassUniversal, TypePrimitive, TagNull); err != nil {
		return err
	}
	return e.WriteLength(0)
}

// WriteRaw writes raw bytes directly to the buffer.
// Useful for pre-encoded data or custom encoding.
func (e *BEREncoder) WriteRaw(data []byte) {
	e.buf = append(e.buf, data...)
}

// WriteTaggedValue writes a context-specific tagged value.
// This is commonly used in LDAP for protocol-specific fields.
func (e *BEREncoder) WriteTaggedValue(tagNumber int, constructed bool, value []byte) error {
	constructedFlag := TypePrimitive
	if constructed {
		constructedFlag = TypeConstructed
	}

	if err := e.WriteTag(ClassContextSpecific, constructedFlag, tagNumber); err != nil {
		return err
	}
	if err := e.WriteLength(len(value)); err != nil {
		return err
	}
	e.buf = append(e.buf, value...)
	return nil
}
