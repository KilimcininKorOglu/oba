// Package ber implements ASN.1 BER (Basic Encoding Rules) encoding
// as specified in ITU-T X.690.
package ber

// BERDecoder decodes ASN.1 values using BER (Basic Encoding Rules).
type BERDecoder struct {
	data   []byte
	offset int
}

// NewBERDecoder creates a new BER decoder for the given data.
func NewBERDecoder(data []byte) *BERDecoder {
	return &BERDecoder{
		data:   data,
		offset: 0,
	}
}

// Offset returns the current read position in the data.
func (d *BERDecoder) Offset() int {
	return d.offset
}

// Remaining returns the number of bytes remaining to be read.
func (d *BERDecoder) Remaining() int {
	return len(d.data) - d.offset
}

// Reset resets the decoder to the beginning of the data.
func (d *BERDecoder) Reset() {
	d.offset = 0
}

// SetOffset sets the current read position.
func (d *BERDecoder) SetOffset(offset int) {
	d.offset = offset
}

// SetData sets new data for the decoder and resets the offset.
func (d *BERDecoder) SetData(data []byte) {
	d.data = data
	d.offset = 0
}

// ReadTag reads a BER tag from the current position.
// Returns the tag class, constructed flag, and tag number.
func (d *BERDecoder) ReadTag() (class, constructed, number int, err error) {
	startOffset := d.offset

	if d.offset >= len(d.data) {
		return 0, 0, 0, NewDecodeError(startOffset, "cannot read tag", ErrUnexpectedEOF)
	}

	firstByte := d.data[d.offset]
	d.offset++

	// Extract class (bits 7-8)
	class = int(firstByte & 0xC0)

	// Extract constructed flag (bit 6)
	constructed = int(firstByte & 0x20)

	// Extract tag number (bits 1-5)
	number = int(firstByte & 0x1F)

	// Check for long form tag (all 5 bits set = 0x1F)
	if number == 0x1F {
		// Long form: read subsequent bytes
		number, err = d.readBase128()
		if err != nil {
			return 0, 0, 0, NewDecodeError(startOffset, "cannot read long form tag number", err)
		}
	}

	return class, constructed, number, nil
}

// readBase128 reads a base-128 encoded integer (used for long form tags).
func (d *BERDecoder) readBase128() (int, error) {
	result := 0
	for {
		if d.offset >= len(d.data) {
			return 0, ErrUnexpectedEOF
		}

		b := d.data[d.offset]
		d.offset++

		// Check for overflow before shifting
		if result > (1 << 24) {
			// Prevent overflow for very large tag numbers
			return 0, NewDecodeError(d.offset-1, "tag number overflow", nil)
		}

		result = (result << 7) | int(b&0x7F)

		// If high bit is not set, this is the last byte
		if b&0x80 == 0 {
			break
		}
	}
	return result, nil
}

// ReadLength reads a BER length value from the current position.
func (d *BERDecoder) ReadLength() (int, error) {
	startOffset := d.offset

	if d.offset >= len(d.data) {
		return 0, NewDecodeError(startOffset, "cannot read length", ErrUnexpectedEOF)
	}

	firstByte := d.data[d.offset]
	d.offset++

	// Short form: bit 8 is 0, bits 1-7 contain the length
	if firstByte&LengthLongFormBit == 0 {
		return int(firstByte), nil
	}

	// Long form: bit 8 is 1, bits 1-7 contain the number of subsequent length bytes
	numBytes := int(firstByte & 0x7F)

	// Check for indefinite length (0x80)
	if numBytes == 0 {
		return 0, NewDecodeError(startOffset, "indefinite length encoding", ErrIndefiniteLength)
	}

	// Check if we have enough data
	if d.offset+numBytes > len(d.data) {
		return 0, NewDecodeError(startOffset, "truncated length encoding", ErrUnexpectedEOF)
	}

	// Read the length value
	length := 0
	for i := 0; i < numBytes; i++ {
		// Check for overflow
		if length > (1 << 24) {
			return 0, NewDecodeError(startOffset, "length value overflow", ErrInvalidLength)
		}
		length = (length << 8) | int(d.data[d.offset])
		d.offset++
	}

	return length, nil
}

// ReadBoolean reads a BER-encoded boolean value.
func (d *BERDecoder) ReadBoolean() (bool, error) {
	startOffset := d.offset

	// Read and verify tag
	class, constructed, number, err := d.ReadTag()
	if err != nil {
		return false, err
	}

	if class != ClassUniversal || constructed != TypePrimitive || number != TagBoolean {
		return false, &TagMismatchError{
			Offset:            startOffset,
			ExpectedClass:     ClassUniversal,
			ExpectedNumber:    TagBoolean,
			ActualClass:       class,
			ActualNumber:      number,
			ActualConstructed: constructed,
		}
	}

	// Read length
	length, err := d.ReadLength()
	if err != nil {
		return false, err
	}

	// Boolean must have length 1
	if length != 1 {
		return false, NewDecodeError(startOffset, "boolean must have length 1", ErrInvalidBoolean)
	}

	// Check if we have enough data
	if d.offset >= len(d.data) {
		return false, NewDecodeError(d.offset, "cannot read boolean value", ErrUnexpectedEOF)
	}

	value := d.data[d.offset]
	d.offset++

	// Per X.690, FALSE is 0x00, TRUE is any non-zero value
	return value != 0x00, nil
}

// ReadInteger reads a BER-encoded integer value.
func (d *BERDecoder) ReadInteger() (int64, error) {
	startOffset := d.offset

	// Read and verify tag
	class, constructed, number, err := d.ReadTag()
	if err != nil {
		return 0, err
	}

	if class != ClassUniversal || constructed != TypePrimitive || number != TagInteger {
		return 0, &TagMismatchError{
			Offset:            startOffset,
			ExpectedClass:     ClassUniversal,
			ExpectedNumber:    TagInteger,
			ActualClass:       class,
			ActualNumber:      number,
			ActualConstructed: constructed,
		}
	}

	// Read length
	length, err := d.ReadLength()
	if err != nil {
		return 0, err
	}

	// Integer must have at least 1 byte
	if length == 0 {
		return 0, NewDecodeError(startOffset, "integer must have at least 1 byte", ErrInvalidInteger)
	}

	// Check for overflow (int64 is 8 bytes)
	if length > 8 {
		return 0, NewDecodeError(startOffset, "integer too large for int64", ErrInvalidInteger)
	}

	// Check if we have enough data
	if d.offset+length > len(d.data) {
		return 0, NewDecodeError(d.offset, "truncated integer value", ErrUnexpectedEOF)
	}

	return d.decodeInteger(length), nil
}

// decodeInteger decodes a two's complement integer from the current position.
func (d *BERDecoder) decodeInteger(length int) int64 {
	// Read first byte to determine sign
	firstByte := d.data[d.offset]
	d.offset++

	var result int64

	// If high bit is set, the number is negative (two's complement)
	if firstByte&0x80 != 0 {
		// Start with all bits set for sign extension
		result = -1
	}

	// Shift in the first byte
	result = (result << 8) | int64(firstByte)

	// Read remaining bytes
	for i := 1; i < length; i++ {
		result = (result << 8) | int64(d.data[d.offset])
		d.offset++
	}

	return result
}

// ReadOctetString reads a BER-encoded octet string.
func (d *BERDecoder) ReadOctetString() ([]byte, error) {
	startOffset := d.offset

	// Read and verify tag
	class, constructed, number, err := d.ReadTag()
	if err != nil {
		return nil, err
	}

	if class != ClassUniversal || number != TagOctetString {
		return nil, &TagMismatchError{
			Offset:            startOffset,
			ExpectedClass:     ClassUniversal,
			ExpectedNumber:    TagOctetString,
			ActualClass:       class,
			ActualNumber:      number,
			ActualConstructed: constructed,
		}
	}

	// Note: Octet strings can be primitive or constructed in BER
	// For simplicity, we only support primitive encoding here
	if constructed != TypePrimitive {
		return nil, NewDecodeError(startOffset, "constructed octet string not supported", nil)
	}

	// Read length
	length, err := d.ReadLength()
	if err != nil {
		return nil, err
	}

	// Check if we have enough data
	if d.offset+length > len(d.data) {
		return nil, NewDecodeError(d.offset, "truncated octet string value", ErrUnexpectedEOF)
	}

	// Read the value
	value := make([]byte, length)
	copy(value, d.data[d.offset:d.offset+length])
	d.offset += length

	return value, nil
}

// ReadEnumerated reads a BER-encoded enumerated value.
func (d *BERDecoder) ReadEnumerated() (int64, error) {
	startOffset := d.offset

	// Read and verify tag
	class, constructed, number, err := d.ReadTag()
	if err != nil {
		return 0, err
	}

	if class != ClassUniversal || constructed != TypePrimitive || number != TagEnumerated {
		return 0, &TagMismatchError{
			Offset:            startOffset,
			ExpectedClass:     ClassUniversal,
			ExpectedNumber:    TagEnumerated,
			ActualClass:       class,
			ActualNumber:      number,
			ActualConstructed: constructed,
		}
	}

	// Read length
	length, err := d.ReadLength()
	if err != nil {
		return 0, err
	}

	// Enumerated must have at least 1 byte
	if length == 0 {
		return 0, NewDecodeError(startOffset, "enumerated must have at least 1 byte", ErrInvalidInteger)
	}

	// Check for overflow (int64 is 8 bytes)
	if length > 8 {
		return 0, NewDecodeError(startOffset, "enumerated too large for int64", ErrInvalidInteger)
	}

	// Check if we have enough data
	if d.offset+length > len(d.data) {
		return 0, NewDecodeError(d.offset, "truncated enumerated value", ErrUnexpectedEOF)
	}

	return d.decodeInteger(length), nil
}

// ReadNull reads a BER-encoded null value.
func (d *BERDecoder) ReadNull() error {
	startOffset := d.offset

	// Read and verify tag
	class, constructed, number, err := d.ReadTag()
	if err != nil {
		return err
	}

	if class != ClassUniversal || constructed != TypePrimitive || number != TagNull {
		return &TagMismatchError{
			Offset:            startOffset,
			ExpectedClass:     ClassUniversal,
			ExpectedNumber:    TagNull,
			ActualClass:       class,
			ActualNumber:      number,
			ActualConstructed: constructed,
		}
	}

	// Read length
	length, err := d.ReadLength()
	if err != nil {
		return err
	}

	// Null must have length 0
	if length != 0 {
		return NewDecodeError(startOffset, "null must have length 0", ErrInvalidNull)
	}

	return nil
}

// PeekTag reads a tag without advancing the offset.
func (d *BERDecoder) PeekTag() (class, constructed, number int, err error) {
	savedOffset := d.offset
	class, constructed, number, err = d.ReadTag()
	d.offset = savedOffset
	return
}

// Skip skips the current TLV (Tag-Length-Value) element.
func (d *BERDecoder) Skip() error {
	startOffset := d.offset

	// Read tag
	_, _, _, err := d.ReadTag()
	if err != nil {
		return err
	}

	// Read length
	length, err := d.ReadLength()
	if err != nil {
		return err
	}

	// Check if we have enough data
	if d.offset+length > len(d.data) {
		return NewDecodeError(startOffset, "truncated value", ErrUnexpectedEOF)
	}

	// Skip the value
	d.offset += length
	return nil
}

// ReadRawValue reads the raw bytes of the current TLV element (including tag and length).
func (d *BERDecoder) ReadRawValue() ([]byte, error) {
	startOffset := d.offset

	// Read tag
	_, _, _, err := d.ReadTag()
	if err != nil {
		return nil, err
	}

	// Read length
	length, err := d.ReadLength()
	if err != nil {
		return nil, err
	}

	// Check if we have enough data
	if d.offset+length > len(d.data) {
		return nil, NewDecodeError(startOffset, "truncated value", ErrUnexpectedEOF)
	}

	// Calculate total length including tag and length bytes
	endOffset := d.offset + length
	result := make([]byte, endOffset-startOffset)
	copy(result, d.data[startOffset:endOffset])

	// Advance past the value
	d.offset = endOffset

	return result, nil
}

// ReadTaggedValue reads a context-specific tagged value.
// Returns the tag number and the raw value bytes.
func (d *BERDecoder) ReadTaggedValue() (tagNumber int, constructed bool, value []byte, err error) {
	startOffset := d.offset

	// Read tag
	class, constructedFlag, number, err := d.ReadTag()
	if err != nil {
		return 0, false, nil, err
	}

	// Verify it's context-specific
	if class != ClassContextSpecific {
		return 0, false, nil, &TagMismatchError{
			Offset:            startOffset,
			ExpectedClass:     ClassContextSpecific,
			ExpectedNumber:    -1, // Any number
			ActualClass:       class,
			ActualNumber:      number,
			ActualConstructed: constructedFlag,
		}
	}

	// Read length
	length, err := d.ReadLength()
	if err != nil {
		return 0, false, nil, err
	}

	// Check if we have enough data
	if d.offset+length > len(d.data) {
		return 0, false, nil, NewDecodeError(d.offset, "truncated tagged value", ErrUnexpectedEOF)
	}

	// Read the value
	value = make([]byte, length)
	copy(value, d.data[d.offset:d.offset+length])
	d.offset += length

	return number, constructedFlag == TypeConstructed, value, nil
}

// ReadIntegerWithTag reads an integer value with a specific context tag.
func (d *BERDecoder) ReadIntegerWithTag(expectedTag int) (int64, error) {
	startOffset := d.offset

	// Read tag
	class, constructed, number, err := d.ReadTag()
	if err != nil {
		return 0, err
	}

	if class != ClassContextSpecific || constructed != TypePrimitive || number != expectedTag {
		return 0, &TagMismatchError{
			Offset:            startOffset,
			ExpectedClass:     ClassContextSpecific,
			ExpectedNumber:    expectedTag,
			ActualClass:       class,
			ActualNumber:      number,
			ActualConstructed: constructed,
		}
	}

	// Read length
	length, err := d.ReadLength()
	if err != nil {
		return 0, err
	}

	// Integer must have at least 1 byte
	if length == 0 {
		return 0, NewDecodeError(startOffset, "integer must have at least 1 byte", ErrInvalidInteger)
	}

	// Check for overflow
	if length > 8 {
		return 0, NewDecodeError(startOffset, "integer too large for int64", ErrInvalidInteger)
	}

	// Check if we have enough data
	if d.offset+length > len(d.data) {
		return 0, NewDecodeError(d.offset, "truncated integer value", ErrUnexpectedEOF)
	}

	return d.decodeInteger(length), nil
}

// ExpectSequence reads and validates a SEQUENCE tag, returning the content length.
// The caller should read exactly 'length' bytes of content after this call.
func (d *BERDecoder) ExpectSequence() (length int, err error) {
	startOffset := d.offset

	// Read tag
	class, constructed, number, err := d.ReadTag()
	if err != nil {
		return 0, err
	}

	// Verify it's a SEQUENCE (Universal, Constructed, 0x10)
	if class != ClassUniversal || constructed != TypeConstructed || number != TagSequence {
		return 0, &TagMismatchError{
			Offset:            startOffset,
			ExpectedClass:     ClassUniversal,
			ExpectedNumber:    TagSequence,
			ActualClass:       class,
			ActualNumber:      number,
			ActualConstructed: constructed,
		}
	}

	// Read length
	length, err = d.ReadLength()
	if err != nil {
		return 0, err
	}

	// Verify we have enough data
	if d.offset+length > len(d.data) {
		return 0, NewDecodeError(startOffset, "truncated sequence content", ErrUnexpectedEOF)
	}

	return length, nil
}

// ExpectSet reads and validates a SET tag, returning the content length.
// The caller should read exactly 'length' bytes of content after this call.
func (d *BERDecoder) ExpectSet() (length int, err error) {
	startOffset := d.offset

	// Read tag
	class, constructed, number, err := d.ReadTag()
	if err != nil {
		return 0, err
	}

	// Verify it's a SET (Universal, Constructed, 0x11)
	if class != ClassUniversal || constructed != TypeConstructed || number != TagSet {
		return 0, &TagMismatchError{
			Offset:            startOffset,
			ExpectedClass:     ClassUniversal,
			ExpectedNumber:    TagSet,
			ActualClass:       class,
			ActualNumber:      number,
			ActualConstructed: constructed,
		}
	}

	// Read length
	length, err = d.ReadLength()
	if err != nil {
		return 0, err
	}

	// Verify we have enough data
	if d.offset+length > len(d.data) {
		return 0, NewDecodeError(startOffset, "truncated set content", ErrUnexpectedEOF)
	}

	return length, nil
}

// ExpectContextTag reads and validates a context-specific tag with the given number.
// Returns the content length. The caller should read exactly 'length' bytes after this call.
func (d *BERDecoder) ExpectContextTag(num int) (length int, err error) {
	startOffset := d.offset

	// Read tag
	class, constructed, number, err := d.ReadTag()
	if err != nil {
		return 0, err
	}

	// Verify it's context-specific with the expected number
	if class != ClassContextSpecific || number != num {
		return 0, &TagMismatchError{
			Offset:            startOffset,
			ExpectedClass:     ClassContextSpecific,
			ExpectedNumber:    num,
			ActualClass:       class,
			ActualNumber:      number,
			ActualConstructed: constructed,
		}
	}

	// Read length
	length, err = d.ReadLength()
	if err != nil {
		return 0, err
	}

	// Verify we have enough data
	if d.offset+length > len(d.data) {
		return 0, NewDecodeError(startOffset, "truncated context-tagged content", ErrUnexpectedEOF)
	}

	return length, nil
}

// ExpectApplicationTag reads and validates an application-specific tag with the given number.
// Returns the content length. The caller should read exactly 'length' bytes after this call.
func (d *BERDecoder) ExpectApplicationTag(num int) (length int, err error) {
	startOffset := d.offset

	// Read tag
	class, constructed, number, err := d.ReadTag()
	if err != nil {
		return 0, err
	}

	// Verify it's application-specific with the expected number
	if class != ClassApplication || number != num {
		return 0, &TagMismatchError{
			Offset:            startOffset,
			ExpectedClass:     ClassApplication,
			ExpectedNumber:    num,
			ActualClass:       class,
			ActualNumber:      number,
			ActualConstructed: constructed,
		}
	}

	// Read length
	length, err = d.ReadLength()
	if err != nil {
		return 0, err
	}

	// Verify we have enough data
	if d.offset+length > len(d.data) {
		return 0, NewDecodeError(startOffset, "truncated application-tagged content", ErrUnexpectedEOF)
	}

	return length, nil
}

// IsContextTag checks if the next tag is a context-specific tag with the given number
// without consuming it. Returns true if it matches, false otherwise.
func (d *BERDecoder) IsContextTag(num int) bool {
	class, _, number, err := d.PeekTag()
	if err != nil {
		return false
	}
	return class == ClassContextSpecific && number == num
}

// IsApplicationTag checks if the next tag is an application-specific tag with the given number
// without consuming it. Returns true if it matches, false otherwise.
func (d *BERDecoder) IsApplicationTag(num int) bool {
	class, _, number, err := d.PeekTag()
	if err != nil {
		return false
	}
	return class == ClassApplication && number == num
}

// ReadSequenceContents reads the contents of a SEQUENCE into a sub-decoder.
// This is useful for parsing nested structures.
func (d *BERDecoder) ReadSequenceContents() (*BERDecoder, error) {
	length, err := d.ExpectSequence()
	if err != nil {
		return nil, err
	}

	// Create a sub-decoder for the sequence contents
	contents := d.data[d.offset : d.offset+length]
	d.offset += length

	return NewBERDecoder(contents), nil
}

// ReadSetContents reads the contents of a SET into a sub-decoder.
func (d *BERDecoder) ReadSetContents() (*BERDecoder, error) {
	length, err := d.ExpectSet()
	if err != nil {
		return nil, err
	}

	// Create a sub-decoder for the set contents
	contents := d.data[d.offset : d.offset+length]
	d.offset += length

	return NewBERDecoder(contents), nil
}

// ReadContextTagContents reads the contents of a context-specific tag into a sub-decoder.
func (d *BERDecoder) ReadContextTagContents(num int) (*BERDecoder, error) {
	length, err := d.ExpectContextTag(num)
	if err != nil {
		return nil, err
	}

	// Create a sub-decoder for the contents
	contents := d.data[d.offset : d.offset+length]
	d.offset += length

	return NewBERDecoder(contents), nil
}

// ReadApplicationTagContents reads the contents of an application-specific tag into a sub-decoder.
func (d *BERDecoder) ReadApplicationTagContents(num int) (*BERDecoder, error) {
	length, err := d.ExpectApplicationTag(num)
	if err != nil {
		return nil, err
	}

	// Create a sub-decoder for the contents
	contents := d.data[d.offset : d.offset+length]
	d.offset += length

	return NewBERDecoder(contents), nil
}
