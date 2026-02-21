package ber

import (
	"bytes"
	"errors"
	"testing"
)

func TestNewBERDecoder(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03}
	dec := NewBERDecoder(data)

	if dec == nil {
		t.Fatal("expected non-nil decoder")
	}
	if dec.Offset() != 0 {
		t.Errorf("expected offset 0, got %d", dec.Offset())
	}
	if dec.Remaining() != 3 {
		t.Errorf("expected remaining 3, got %d", dec.Remaining())
	}
}

func TestBERDecoder_Reset(t *testing.T) {
	data := []byte{0x01, 0x01, 0x00}
	dec := NewBERDecoder(data)

	// Read something to advance offset
	dec.ReadBoolean()
	if dec.Offset() == 0 {
		t.Fatal("expected non-zero offset after read")
	}

	dec.Reset()
	if dec.Offset() != 0 {
		t.Errorf("expected offset 0 after reset, got %d", dec.Offset())
	}
}

func TestBERDecoder_SetData(t *testing.T) {
	dec := NewBERDecoder([]byte{0x01})
	dec.SetData([]byte{0x02, 0x03, 0x04})

	if dec.Remaining() != 3 {
		t.Errorf("expected remaining 3, got %d", dec.Remaining())
	}
	if dec.Offset() != 0 {
		t.Errorf("expected offset 0, got %d", dec.Offset())
	}
}

func TestBERDecoder_ReadTag(t *testing.T) {
	tests := []struct {
		name       string
		data       []byte
		wantClass  int
		wantConstr int
		wantNumber int
		wantErr    bool
	}{
		// Universal class tags
		{
			name:       "universal primitive boolean",
			data:       []byte{0x01},
			wantClass:  ClassUniversal,
			wantConstr: TypePrimitive,
			wantNumber: TagBoolean,
		},
		{
			name:       "universal primitive integer",
			data:       []byte{0x02},
			wantClass:  ClassUniversal,
			wantConstr: TypePrimitive,
			wantNumber: TagInteger,
		},
		{
			name:       "universal primitive octet string",
			data:       []byte{0x04},
			wantClass:  ClassUniversal,
			wantConstr: TypePrimitive,
			wantNumber: TagOctetString,
		},
		{
			name:       "universal primitive null",
			data:       []byte{0x05},
			wantClass:  ClassUniversal,
			wantConstr: TypePrimitive,
			wantNumber: TagNull,
		},
		{
			name:       "universal primitive enumerated",
			data:       []byte{0x0A},
			wantClass:  ClassUniversal,
			wantConstr: TypePrimitive,
			wantNumber: TagEnumerated,
		},
		{
			name:       "universal constructed sequence",
			data:       []byte{0x30},
			wantClass:  ClassUniversal,
			wantConstr: TypeConstructed,
			wantNumber: TagSequence,
		},
		{
			name:       "universal constructed set",
			data:       []byte{0x31},
			wantClass:  ClassUniversal,
			wantConstr: TypeConstructed,
			wantNumber: TagSet,
		},

		// Application class tags
		{
			name:       "application primitive tag 0",
			data:       []byte{0x40},
			wantClass:  ClassApplication,
			wantConstr: TypePrimitive,
			wantNumber: 0,
		},
		{
			name:       "application constructed tag 1",
			data:       []byte{0x61},
			wantClass:  ClassApplication,
			wantConstr: TypeConstructed,
			wantNumber: 1,
		},

		// Context-specific class tags
		{
			name:       "context-specific primitive tag 0",
			data:       []byte{0x80},
			wantClass:  ClassContextSpecific,
			wantConstr: TypePrimitive,
			wantNumber: 0,
		},
		{
			name:       "context-specific constructed tag 3",
			data:       []byte{0xA3},
			wantClass:  ClassContextSpecific,
			wantConstr: TypeConstructed,
			wantNumber: 3,
		},

		// Private class tags
		{
			name:       "private primitive tag 5",
			data:       []byte{0xC5},
			wantClass:  ClassPrivate,
			wantConstr: TypePrimitive,
			wantNumber: 5,
		},

		// Long form tags
		{
			name:       "universal primitive tag 31 (long form)",
			data:       []byte{0x1F, 0x1F},
			wantClass:  ClassUniversal,
			wantConstr: TypePrimitive,
			wantNumber: 31,
		},
		{
			name:       "universal primitive tag 127 (long form)",
			data:       []byte{0x1F, 0x7F},
			wantClass:  ClassUniversal,
			wantConstr: TypePrimitive,
			wantNumber: 127,
		},
		{
			name:       "universal primitive tag 128 (long form, 2 bytes)",
			data:       []byte{0x1F, 0x81, 0x00},
			wantClass:  ClassUniversal,
			wantConstr: TypePrimitive,
			wantNumber: 128,
		},
		{
			name:       "context-specific tag 256 (long form)",
			data:       []byte{0x9F, 0x82, 0x00},
			wantClass:  ClassContextSpecific,
			wantConstr: TypePrimitive,
			wantNumber: 256,
		},

		// Edge cases
		{
			name:       "tag number 30 (max short form)",
			data:       []byte{0x1E},
			wantClass:  ClassUniversal,
			wantConstr: TypePrimitive,
			wantNumber: 30,
		},
		{
			name:       "tag number 0",
			data:       []byte{0x00},
			wantClass:  ClassUniversal,
			wantConstr: TypePrimitive,
			wantNumber: 0,
		},

		// Error cases
		{
			name:    "empty data",
			data:    []byte{},
			wantErr: true,
		},
		{
			name:    "truncated long form tag",
			data:    []byte{0x1F, 0x81}, // Missing continuation byte
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dec := NewBERDecoder(tt.data)
			class, constructed, number, err := dec.ReadTag()

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if class != tt.wantClass {
				t.Errorf("class: expected %d, got %d", tt.wantClass, class)
			}
			if constructed != tt.wantConstr {
				t.Errorf("constructed: expected %d, got %d", tt.wantConstr, constructed)
			}
			if number != tt.wantNumber {
				t.Errorf("number: expected %d, got %d", tt.wantNumber, number)
			}
		})
	}
}

func TestBERDecoder_ReadLength(t *testing.T) {
	tests := []struct {
		name       string
		data       []byte
		wantLength int
		wantErr    bool
		errType    error
	}{
		// Short form (0-127)
		{
			name:       "length 0",
			data:       []byte{0x00},
			wantLength: 0,
		},
		{
			name:       "length 1",
			data:       []byte{0x01},
			wantLength: 1,
		},
		{
			name:       "length 127 (max short form)",
			data:       []byte{0x7F},
			wantLength: 127,
		},

		// Long form (>127)
		{
			name:       "length 128 (min long form)",
			data:       []byte{0x81, 0x80},
			wantLength: 128,
		},
		{
			name:       "length 255",
			data:       []byte{0x81, 0xFF},
			wantLength: 255,
		},
		{
			name:       "length 256",
			data:       []byte{0x82, 0x01, 0x00},
			wantLength: 256,
		},
		{
			name:       "length 65535",
			data:       []byte{0x82, 0xFF, 0xFF},
			wantLength: 65535,
		},
		{
			name:       "length 65536",
			data:       []byte{0x83, 0x01, 0x00, 0x00},
			wantLength: 65536,
		},
		{
			name:       "length 16777215",
			data:       []byte{0x83, 0xFF, 0xFF, 0xFF},
			wantLength: 16777215,
		},
		{
			name:       "length 16777216",
			data:       []byte{0x84, 0x01, 0x00, 0x00, 0x00},
			wantLength: 16777216,
		},

		// Error cases
		{
			name:    "empty data",
			data:    []byte{},
			wantErr: true,
			errType: ErrUnexpectedEOF,
		},
		{
			name:    "indefinite length",
			data:    []byte{0x80},
			wantErr: true,
			errType: ErrIndefiniteLength,
		},
		{
			name:    "truncated long form",
			data:    []byte{0x82, 0x01}, // Missing second byte
			wantErr: true,
			errType: ErrUnexpectedEOF,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dec := NewBERDecoder(tt.data)
			length, err := dec.ReadLength()

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				if tt.errType != nil && !errors.Is(err, tt.errType) {
					t.Errorf("expected error type %v, got %v", tt.errType, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if length != tt.wantLength {
				t.Errorf("expected length %d, got %d", tt.wantLength, length)
			}
		})
	}
}

func TestBERDecoder_ReadBoolean(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		want    bool
		wantErr bool
		errType error
	}{
		{
			name: "false",
			data: []byte{0x01, 0x01, 0x00},
			want: false,
		},
		{
			name: "true (0xFF)",
			data: []byte{0x01, 0x01, 0xFF},
			want: true,
		},
		{
			name: "true (0x01)",
			data: []byte{0x01, 0x01, 0x01},
			want: true,
		},
		{
			name: "true (any non-zero)",
			data: []byte{0x01, 0x01, 0x42},
			want: true,
		},

		// Error cases
		{
			name:    "empty data",
			data:    []byte{},
			wantErr: true,
			errType: ErrUnexpectedEOF,
		},
		{
			name:    "wrong tag",
			data:    []byte{0x02, 0x01, 0x00}, // Integer tag
			wantErr: true,
			errType: ErrTagMismatch,
		},
		{
			name:    "invalid length (0)",
			data:    []byte{0x01, 0x00},
			wantErr: true,
			errType: ErrInvalidBoolean,
		},
		{
			name:    "invalid length (2)",
			data:    []byte{0x01, 0x02, 0x00, 0x00},
			wantErr: true,
			errType: ErrInvalidBoolean,
		},
		{
			name:    "truncated value",
			data:    []byte{0x01, 0x01}, // Missing value byte
			wantErr: true,
			errType: ErrUnexpectedEOF,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dec := NewBERDecoder(tt.data)
			got, err := dec.ReadBoolean()

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				if tt.errType != nil && !errors.Is(err, tt.errType) {
					t.Errorf("expected error type %v, got %v", tt.errType, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tt.want {
				t.Errorf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestBERDecoder_ReadInteger(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		want    int64
		wantErr bool
		errType error
	}{
		// Zero
		{
			name: "zero",
			data: []byte{0x02, 0x01, 0x00},
			want: 0,
		},

		// Positive values
		{
			name: "positive 1",
			data: []byte{0x02, 0x01, 0x01},
			want: 1,
		},
		{
			name: "positive 127",
			data: []byte{0x02, 0x01, 0x7F},
			want: 127,
		},
		{
			name: "positive 128 (with padding)",
			data: []byte{0x02, 0x02, 0x00, 0x80},
			want: 128,
		},
		{
			name: "positive 255",
			data: []byte{0x02, 0x02, 0x00, 0xFF},
			want: 255,
		},
		{
			name: "positive 256",
			data: []byte{0x02, 0x02, 0x01, 0x00},
			want: 256,
		},
		{
			name: "positive 32767",
			data: []byte{0x02, 0x02, 0x7F, 0xFF},
			want: 32767,
		},
		{
			name: "positive 32768 (with padding)",
			data: []byte{0x02, 0x03, 0x00, 0x80, 0x00},
			want: 32768,
		},
		{
			name: "positive 65535",
			data: []byte{0x02, 0x03, 0x00, 0xFF, 0xFF},
			want: 65535,
		},

		// Negative values (two's complement)
		{
			name: "negative -1",
			data: []byte{0x02, 0x01, 0xFF},
			want: -1,
		},
		{
			name: "negative -128",
			data: []byte{0x02, 0x01, 0x80},
			want: -128,
		},
		{
			name: "negative -129",
			data: []byte{0x02, 0x02, 0xFF, 0x7F},
			want: -129,
		},
		{
			name: "negative -256",
			data: []byte{0x02, 0x02, 0xFF, 0x00},
			want: -256,
		},
		{
			name: "negative -32768",
			data: []byte{0x02, 0x02, 0x80, 0x00},
			want: -32768,
		},
		{
			name: "negative -32769",
			data: []byte{0x02, 0x03, 0xFF, 0x7F, 0xFF},
			want: -32769,
		},

		// Large values
		{
			name: "max int32",
			data: []byte{0x02, 0x04, 0x7F, 0xFF, 0xFF, 0xFF},
			want: 2147483647,
		},
		{
			name: "min int32",
			data: []byte{0x02, 0x04, 0x80, 0x00, 0x00, 0x00},
			want: -2147483648,
		},
		{
			name: "max int64",
			data: []byte{0x02, 0x08, 0x7F, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			want: 9223372036854775807,
		},
		{
			name: "min int64",
			data: []byte{0x02, 0x08, 0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			want: -9223372036854775808,
		},

		// Error cases
		{
			name:    "empty data",
			data:    []byte{},
			wantErr: true,
			errType: ErrUnexpectedEOF,
		},
		{
			name:    "wrong tag",
			data:    []byte{0x01, 0x01, 0x00}, // Boolean tag
			wantErr: true,
			errType: ErrTagMismatch,
		},
		{
			name:    "zero length",
			data:    []byte{0x02, 0x00},
			wantErr: true,
			errType: ErrInvalidInteger,
		},
		{
			name:    "truncated value",
			data:    []byte{0x02, 0x02, 0x01}, // Missing second byte
			wantErr: true,
			errType: ErrUnexpectedEOF,
		},
		{
			name:    "too large (9 bytes)",
			data:    []byte{0x02, 0x09, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			wantErr: true,
			errType: ErrInvalidInteger,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dec := NewBERDecoder(tt.data)
			got, err := dec.ReadInteger()

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				if tt.errType != nil && !errors.Is(err, tt.errType) {
					t.Errorf("expected error type %v, got %v", tt.errType, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tt.want {
				t.Errorf("expected %d, got %d", tt.want, got)
			}
		})
	}
}

func TestBERDecoder_ReadOctetString(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		want    []byte
		wantErr bool
		errType error
	}{
		{
			name: "empty string",
			data: []byte{0x04, 0x00},
			want: []byte{},
		},
		{
			name: "single byte",
			data: []byte{0x04, 0x01, 0x41},
			want: []byte{0x41},
		},
		{
			name: "hello",
			data: []byte{0x04, 0x05, 'h', 'e', 'l', 'l', 'o'},
			want: []byte("hello"),
		},
		{
			name: "binary data",
			data: []byte{0x04, 0x04, 0x00, 0xFF, 0x80, 0x7F},
			want: []byte{0x00, 0xFF, 0x80, 0x7F},
		},

		// Error cases
		{
			name:    "empty data",
			data:    []byte{},
			wantErr: true,
			errType: ErrUnexpectedEOF,
		},
		{
			name:    "wrong tag",
			data:    []byte{0x02, 0x01, 0x00}, // Integer tag
			wantErr: true,
			errType: ErrTagMismatch,
		},
		{
			name:    "truncated value",
			data:    []byte{0x04, 0x05, 'h', 'e', 'l'}, // Missing 2 bytes
			wantErr: true,
			errType: ErrUnexpectedEOF,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dec := NewBERDecoder(tt.data)
			got, err := dec.ReadOctetString()

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				if tt.errType != nil && !errors.Is(err, tt.errType) {
					t.Errorf("expected error type %v, got %v", tt.errType, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !bytes.Equal(got, tt.want) {
				t.Errorf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestBERDecoder_ReadOctetString_LongForm(t *testing.T) {
	// Test with a string longer than 127 bytes (requires long form length)
	value := make([]byte, 200)
	for i := range value {
		value[i] = byte(i % 256)
	}

	// Build the encoded data: tag (0x04) + length (0x81 0xC8) + value
	data := make([]byte, 0, 3+200)
	data = append(data, 0x04, 0x81, 0xC8)
	data = append(data, value...)

	dec := NewBERDecoder(data)
	got, err := dec.ReadOctetString()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !bytes.Equal(got, value) {
		t.Error("value mismatch")
	}
}

func TestBERDecoder_ReadEnumerated(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		want    int64
		wantErr bool
		errType error
	}{
		{
			name: "zero",
			data: []byte{0x0A, 0x01, 0x00},
			want: 0,
		},
		{
			name: "positive 1",
			data: []byte{0x0A, 0x01, 0x01},
			want: 1,
		},
		{
			name: "positive 255",
			data: []byte{0x0A, 0x02, 0x00, 0xFF},
			want: 255,
		},
		{
			name: "negative -1",
			data: []byte{0x0A, 0x01, 0xFF},
			want: -1,
		},
		{
			name: "large positive",
			data: []byte{0x0A, 0x03, 0x01, 0x00, 0x00},
			want: 65536,
		},
		{
			name: "large negative",
			data: []byte{0x0A, 0x03, 0xFF, 0x00, 0x00},
			want: -65536,
		},

		// Error cases
		{
			name:    "empty data",
			data:    []byte{},
			wantErr: true,
			errType: ErrUnexpectedEOF,
		},
		{
			name:    "wrong tag",
			data:    []byte{0x02, 0x01, 0x00}, // Integer tag
			wantErr: true,
			errType: ErrTagMismatch,
		},
		{
			name:    "zero length",
			data:    []byte{0x0A, 0x00},
			wantErr: true,
			errType: ErrInvalidInteger,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dec := NewBERDecoder(tt.data)
			got, err := dec.ReadEnumerated()

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				if tt.errType != nil && !errors.Is(err, tt.errType) {
					t.Errorf("expected error type %v, got %v", tt.errType, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tt.want {
				t.Errorf("expected %d, got %d", tt.want, got)
			}
		})
	}
}

func TestBERDecoder_ReadNull(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		wantErr bool
		errType error
	}{
		{
			name: "valid null",
			data: []byte{0x05, 0x00},
		},

		// Error cases
		{
			name:    "empty data",
			data:    []byte{},
			wantErr: true,
			errType: ErrUnexpectedEOF,
		},
		{
			name:    "wrong tag",
			data:    []byte{0x02, 0x00}, // Integer tag
			wantErr: true,
			errType: ErrTagMismatch,
		},
		{
			name:    "non-zero length",
			data:    []byte{0x05, 0x01, 0x00},
			wantErr: true,
			errType: ErrInvalidNull,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dec := NewBERDecoder(tt.data)
			err := dec.ReadNull()

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				if tt.errType != nil && !errors.Is(err, tt.errType) {
					t.Errorf("expected error type %v, got %v", tt.errType, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestBERDecoder_PeekTag(t *testing.T) {
	data := []byte{0x02, 0x01, 0x42}
	dec := NewBERDecoder(data)

	// Peek should not advance offset
	class, constructed, number, err := dec.PeekTag()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if class != ClassUniversal || constructed != TypePrimitive || number != TagInteger {
		t.Errorf("unexpected tag: class=%d constructed=%d number=%d", class, constructed, number)
	}

	if dec.Offset() != 0 {
		t.Errorf("expected offset 0 after peek, got %d", dec.Offset())
	}

	// Should be able to read the same tag again
	class2, constructed2, number2, err := dec.ReadTag()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if class != class2 || constructed != constructed2 || number != number2 {
		t.Error("peek and read returned different values")
	}
}

func TestBERDecoder_Skip(t *testing.T) {
	// Multiple TLV elements
	data := []byte{
		0x02, 0x01, 0x42, // Integer 66
		0x01, 0x01, 0xFF, // Boolean true
		0x04, 0x03, 'a', 'b', 'c', // Octet string "abc"
	}

	dec := NewBERDecoder(data)

	// Skip first element
	err := dec.Skip()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should now be at boolean
	val, err := dec.ReadBoolean()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != true {
		t.Errorf("expected true, got %v", val)
	}

	// Skip octet string
	err = dec.Skip()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be at end
	if dec.Remaining() != 0 {
		t.Errorf("expected 0 remaining, got %d", dec.Remaining())
	}
}

func TestBERDecoder_Skip_Truncated(t *testing.T) {
	// Truncated data
	data := []byte{0x04, 0x05, 'a', 'b'} // Claims 5 bytes but only has 2

	dec := NewBERDecoder(data)
	err := dec.Skip()
	if err == nil {
		t.Error("expected error for truncated data")
	}
	if !errors.Is(err, ErrUnexpectedEOF) {
		t.Errorf("expected ErrUnexpectedEOF, got %v", err)
	}
}

func TestBERDecoder_ReadRawValue(t *testing.T) {
	data := []byte{0x02, 0x02, 0x01, 0x00} // Integer 256

	dec := NewBERDecoder(data)
	raw, err := dec.ReadRawValue()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !bytes.Equal(raw, data) {
		t.Errorf("expected %v, got %v", data, raw)
	}

	if dec.Remaining() != 0 {
		t.Errorf("expected 0 remaining, got %d", dec.Remaining())
	}
}

func TestBERDecoder_ReadTaggedValue(t *testing.T) {
	tests := []struct {
		name       string
		data       []byte
		wantTag    int
		wantConstr bool
		wantValue  []byte
		wantErr    bool
	}{
		{
			name:       "context tag 0 primitive",
			data:       []byte{0x80, 0x02, 0x01, 0x02},
			wantTag:    0,
			wantConstr: false,
			wantValue:  []byte{0x01, 0x02},
		},
		{
			name:       "context tag 3 constructed",
			data:       []byte{0xA3, 0x03, 0x04, 0x01, 0x41},
			wantTag:    3,
			wantConstr: true,
			wantValue:  []byte{0x04, 0x01, 0x41},
		},
		{
			name:       "context tag 0 empty",
			data:       []byte{0x80, 0x00},
			wantTag:    0,
			wantConstr: false,
			wantValue:  []byte{},
		},

		// Error cases
		{
			name:    "not context-specific",
			data:    []byte{0x02, 0x01, 0x00}, // Universal integer
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dec := NewBERDecoder(tt.data)
			tag, constructed, value, err := dec.ReadTaggedValue()

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tag != tt.wantTag {
				t.Errorf("tag: expected %d, got %d", tt.wantTag, tag)
			}
			if constructed != tt.wantConstr {
				t.Errorf("constructed: expected %v, got %v", tt.wantConstr, constructed)
			}
			if !bytes.Equal(value, tt.wantValue) {
				t.Errorf("value: expected %v, got %v", tt.wantValue, value)
			}
		})
	}
}

func TestBERDecoder_ReadIntegerWithTag(t *testing.T) {
	tests := []struct {
		name        string
		data        []byte
		expectedTag int
		want        int64
		wantErr     bool
	}{
		{
			name:        "context tag 0",
			data:        []byte{0x80, 0x01, 0x42},
			expectedTag: 0,
			want:        66,
		},
		{
			name:        "context tag 5",
			data:        []byte{0x85, 0x02, 0x01, 0x00},
			expectedTag: 5,
			want:        256,
		},

		// Error cases
		{
			name:        "wrong tag number",
			data:        []byte{0x80, 0x01, 0x42},
			expectedTag: 1, // Expecting tag 1 but got tag 0
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dec := NewBERDecoder(tt.data)
			got, err := dec.ReadIntegerWithTag(tt.expectedTag)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tt.want {
				t.Errorf("expected %d, got %d", tt.want, got)
			}
		})
	}
}

// Round-trip tests: encode then decode should produce identical output

func TestRoundTrip_Boolean(t *testing.T) {
	tests := []bool{true, false}

	for _, v := range tests {
		enc := NewBEREncoder(64)
		err := enc.WriteBoolean(v)
		if err != nil {
			t.Fatalf("encode error: %v", err)
		}

		dec := NewBERDecoder(enc.Bytes())
		got, err := dec.ReadBoolean()
		if err != nil {
			t.Fatalf("decode error: %v", err)
		}

		if got != v {
			t.Errorf("round-trip failed: expected %v, got %v", v, got)
		}
	}
}

func TestRoundTrip_Integer(t *testing.T) {
	tests := []int64{
		0, 1, -1, 127, 128, -128, -129,
		255, 256, -256, 32767, 32768, -32768, -32769,
		65535, 65536, -65536,
		2147483647, -2147483648,
		9223372036854775807, -9223372036854775808,
	}

	for _, v := range tests {
		enc := NewBEREncoder(64)
		err := enc.WriteInteger(v)
		if err != nil {
			t.Fatalf("encode error for %d: %v", v, err)
		}

		dec := NewBERDecoder(enc.Bytes())
		got, err := dec.ReadInteger()
		if err != nil {
			t.Fatalf("decode error for %d: %v", v, err)
		}

		if got != v {
			t.Errorf("round-trip failed: expected %d, got %d", v, got)
		}
	}
}

func TestRoundTrip_OctetString(t *testing.T) {
	tests := [][]byte{
		{},
		{0x00},
		{0xFF},
		[]byte("hello"),
		[]byte("hello world"),
		make([]byte, 200), // Long form length
	}

	// Fill the 200-byte slice with data
	for i := range tests[len(tests)-1] {
		tests[len(tests)-1][i] = byte(i % 256)
	}

	for i, v := range tests {
		enc := NewBEREncoder(256)
		err := enc.WriteOctetString(v)
		if err != nil {
			t.Fatalf("encode error for test %d: %v", i, err)
		}

		dec := NewBERDecoder(enc.Bytes())
		got, err := dec.ReadOctetString()
		if err != nil {
			t.Fatalf("decode error for test %d: %v", i, err)
		}

		if !bytes.Equal(got, v) {
			t.Errorf("round-trip failed for test %d: expected %v, got %v", i, v, got)
		}
	}
}

func TestRoundTrip_Enumerated(t *testing.T) {
	tests := []int64{
		0, 1, -1, 127, 128, -128, -129,
		255, 256, -256, 65536, -65536,
	}

	for _, v := range tests {
		enc := NewBEREncoder(64)
		err := enc.WriteEnumerated(v)
		if err != nil {
			t.Fatalf("encode error for %d: %v", v, err)
		}

		dec := NewBERDecoder(enc.Bytes())
		got, err := dec.ReadEnumerated()
		if err != nil {
			t.Fatalf("decode error for %d: %v", v, err)
		}

		if got != v {
			t.Errorf("round-trip failed: expected %d, got %d", v, got)
		}
	}
}

func TestRoundTrip_Null(t *testing.T) {
	enc := NewBEREncoder(64)
	err := enc.WriteNull()
	if err != nil {
		t.Fatalf("encode error: %v", err)
	}

	dec := NewBERDecoder(enc.Bytes())
	err = dec.ReadNull()
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
}

func TestRoundTrip_TaggedValue(t *testing.T) {
	tests := []struct {
		tagNumber   int
		constructed bool
		value       []byte
	}{
		{0, false, []byte{0x01, 0x02}},
		{3, true, []byte{0x04, 0x01, 0x41}},
		{0, false, []byte{}},
		{31, false, []byte{0x01}}, // Long form tag
	}

	for i, tt := range tests {
		enc := NewBEREncoder(64)
		err := enc.WriteTaggedValue(tt.tagNumber, tt.constructed, tt.value)
		if err != nil {
			t.Fatalf("encode error for test %d: %v", i, err)
		}

		dec := NewBERDecoder(enc.Bytes())
		tag, constructed, value, err := dec.ReadTaggedValue()
		if err != nil {
			t.Fatalf("decode error for test %d: %v", i, err)
		}

		if tag != tt.tagNumber {
			t.Errorf("test %d: tag mismatch: expected %d, got %d", i, tt.tagNumber, tag)
		}
		if constructed != tt.constructed {
			t.Errorf("test %d: constructed mismatch: expected %v, got %v", i, tt.constructed, constructed)
		}
		if !bytes.Equal(value, tt.value) {
			t.Errorf("test %d: value mismatch: expected %v, got %v", i, tt.value, value)
		}
	}
}

func TestRoundTrip_MultipleValues(t *testing.T) {
	// Encode multiple values
	enc := NewBEREncoder(128)
	enc.WriteBoolean(true)
	enc.WriteInteger(42)
	enc.WriteOctetString([]byte("test"))
	enc.WriteNull()
	enc.WriteEnumerated(7)

	// Decode them back
	dec := NewBERDecoder(enc.Bytes())

	b, err := dec.ReadBoolean()
	if err != nil || b != true {
		t.Errorf("boolean: expected true, got %v (err: %v)", b, err)
	}

	i, err := dec.ReadInteger()
	if err != nil || i != 42 {
		t.Errorf("integer: expected 42, got %d (err: %v)", i, err)
	}

	s, err := dec.ReadOctetString()
	if err != nil || !bytes.Equal(s, []byte("test")) {
		t.Errorf("octet string: expected 'test', got %v (err: %v)", s, err)
	}

	err = dec.ReadNull()
	if err != nil {
		t.Errorf("null: unexpected error: %v", err)
	}

	e, err := dec.ReadEnumerated()
	if err != nil || e != 7 {
		t.Errorf("enumerated: expected 7, got %d (err: %v)", e, err)
	}

	if dec.Remaining() != 0 {
		t.Errorf("expected 0 remaining bytes, got %d", dec.Remaining())
	}
}

// Error type tests

func TestDecodeError(t *testing.T) {
	err := NewDecodeError(10, "test message", ErrUnexpectedEOF)

	// Test Error() method
	errStr := err.Error()
	if errStr == "" {
		t.Error("expected non-empty error string")
	}

	// Test Unwrap() method
	unwrapped := err.Unwrap()
	if unwrapped != ErrUnexpectedEOF {
		t.Errorf("expected ErrUnexpectedEOF, got %v", unwrapped)
	}

	// Test errors.Is
	if !errors.Is(err, ErrUnexpectedEOF) {
		t.Error("errors.Is should return true for wrapped error")
	}
}

func TestDecodeError_NoWrapped(t *testing.T) {
	err := NewDecodeError(5, "test message", nil)

	errStr := err.Error()
	if errStr == "" {
		t.Error("expected non-empty error string")
	}

	if err.Unwrap() != nil {
		t.Error("expected nil from Unwrap")
	}
}

func TestTagMismatchError(t *testing.T) {
	err := &TagMismatchError{
		Offset:            10,
		ExpectedClass:     ClassUniversal,
		ExpectedNumber:    TagInteger,
		ActualClass:       ClassUniversal,
		ActualNumber:      TagBoolean,
		ActualConstructed: TypePrimitive,
	}

	// Test Error() method
	errStr := err.Error()
	if errStr == "" {
		t.Error("expected non-empty error string")
	}

	// Test Is() method
	if !errors.Is(err, ErrTagMismatch) {
		t.Error("errors.Is should return true for ErrTagMismatch")
	}
}

// Benchmark tests

func BenchmarkBERDecoder_ReadInteger(b *testing.B) {
	enc := NewBEREncoder(64)
	enc.WriteInteger(12345678)
	data := enc.Bytes()

	dec := NewBERDecoder(data)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		dec.Reset()
		dec.ReadInteger()
	}
}

func BenchmarkBERDecoder_ReadOctetString(b *testing.B) {
	enc := NewBEREncoder(256)
	enc.WriteOctetString([]byte("This is a test string for benchmarking"))
	data := enc.Bytes()

	dec := NewBERDecoder(data)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		dec.Reset()
		dec.ReadOctetString()
	}
}

func BenchmarkBERDecoder_ReadBoolean(b *testing.B) {
	enc := NewBEREncoder(64)
	enc.WriteBoolean(true)
	data := enc.Bytes()

	dec := NewBERDecoder(data)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		dec.Reset()
		dec.ReadBoolean()
	}
}

func BenchmarkBERDecoder_ReadTag(b *testing.B) {
	data := []byte{0x30} // Sequence tag
	dec := NewBERDecoder(data)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		dec.Reset()
		dec.ReadTag()
	}
}

func BenchmarkBERDecoder_ReadLength(b *testing.B) {
	data := []byte{0x82, 0x01, 0x00} // Length 256
	dec := NewBERDecoder(data)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		dec.Reset()
		dec.ReadLength()
	}
}
