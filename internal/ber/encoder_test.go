package ber

import (
	"bytes"
	"testing"
)

func TestNewBEREncoder(t *testing.T) {
	t.Run("default capacity", func(t *testing.T) {
		enc := NewBEREncoder(0)
		if enc == nil {
			t.Fatal("expected non-nil encoder")
		}
		if cap(enc.buf) != 64 {
			t.Errorf("expected default capacity 64, got %d", cap(enc.buf))
		}
	})

	t.Run("custom capacity", func(t *testing.T) {
		enc := NewBEREncoder(128)
		if cap(enc.buf) != 128 {
			t.Errorf("expected capacity 128, got %d", cap(enc.buf))
		}
	})
}

func TestBEREncoder_Reset(t *testing.T) {
	enc := NewBEREncoder(64)
	enc.WriteNull()
	if enc.Len() == 0 {
		t.Fatal("expected non-zero length after write")
	}
	enc.Reset()
	if enc.Len() != 0 {
		t.Errorf("expected zero length after reset, got %d", enc.Len())
	}
}

func TestBEREncoder_WriteTag(t *testing.T) {
	tests := []struct {
		name        string
		class       int
		constructed int
		number      int
		expected    []byte
		wantErr     error
	}{
		// Universal class tags
		{
			name:        "universal primitive boolean",
			class:       ClassUniversal,
			constructed: TypePrimitive,
			number:      TagBoolean,
			expected:    []byte{0x01},
		},
		{
			name:        "universal primitive integer",
			class:       ClassUniversal,
			constructed: TypePrimitive,
			number:      TagInteger,
			expected:    []byte{0x02},
		},
		{
			name:        "universal primitive octet string",
			class:       ClassUniversal,
			constructed: TypePrimitive,
			number:      TagOctetString,
			expected:    []byte{0x04},
		},
		{
			name:        "universal primitive null",
			class:       ClassUniversal,
			constructed: TypePrimitive,
			number:      TagNull,
			expected:    []byte{0x05},
		},
		{
			name:        "universal primitive enumerated",
			class:       ClassUniversal,
			constructed: TypePrimitive,
			number:      TagEnumerated,
			expected:    []byte{0x0A},
		},
		{
			name:        "universal constructed sequence",
			class:       ClassUniversal,
			constructed: TypeConstructed,
			number:      TagSequence,
			expected:    []byte{0x30},
		},
		{
			name:        "universal constructed set",
			class:       ClassUniversal,
			constructed: TypeConstructed,
			number:      TagSet,
			expected:    []byte{0x31},
		},

		// Application class tags
		{
			name:        "application primitive tag 0",
			class:       ClassApplication,
			constructed: TypePrimitive,
			number:      0,
			expected:    []byte{0x40},
		},
		{
			name:        "application constructed tag 1",
			class:       ClassApplication,
			constructed: TypeConstructed,
			number:      1,
			expected:    []byte{0x61},
		},

		// Context-specific class tags
		{
			name:        "context-specific primitive tag 0",
			class:       ClassContextSpecific,
			constructed: TypePrimitive,
			number:      0,
			expected:    []byte{0x80},
		},
		{
			name:        "context-specific constructed tag 3",
			class:       ClassContextSpecific,
			constructed: TypeConstructed,
			number:      3,
			expected:    []byte{0xA3},
		},

		// Private class tags
		{
			name:        "private primitive tag 5",
			class:       ClassPrivate,
			constructed: TypePrimitive,
			number:      5,
			expected:    []byte{0xC5},
		},

		// Long form tags (number > 30)
		{
			name:        "universal primitive tag 31 (long form)",
			class:       ClassUniversal,
			constructed: TypePrimitive,
			number:      31,
			expected:    []byte{0x1F, 0x1F},
		},
		{
			name:        "universal primitive tag 127 (long form)",
			class:       ClassUniversal,
			constructed: TypePrimitive,
			number:      127,
			expected:    []byte{0x1F, 0x7F},
		},
		{
			name:        "universal primitive tag 128 (long form, 2 bytes)",
			class:       ClassUniversal,
			constructed: TypePrimitive,
			number:      128,
			expected:    []byte{0x1F, 0x81, 0x00},
		},
		{
			name:        "context-specific tag 256 (long form)",
			class:       ClassContextSpecific,
			constructed: TypePrimitive,
			number:      256,
			expected:    []byte{0x9F, 0x82, 0x00},
		},

		// Edge cases
		{
			name:        "tag number 30 (max short form)",
			class:       ClassUniversal,
			constructed: TypePrimitive,
			number:      30,
			expected:    []byte{0x1E},
		},
		{
			name:        "tag number 0",
			class:       ClassUniversal,
			constructed: TypePrimitive,
			number:      0,
			expected:    []byte{0x00},
		},

		// Error cases
		{
			name:        "invalid class",
			class:       0x30, // Invalid class value
			constructed: TypePrimitive,
			number:      1,
			wantErr:     ErrInvalidTagClass,
		},
		{
			name:        "negative tag number",
			class:       ClassUniversal,
			constructed: TypePrimitive,
			number:      -1,
			wantErr:     ErrInvalidTagNumber,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enc := NewBEREncoder(64)
			err := enc.WriteTag(tt.class, tt.constructed, tt.number)

			if tt.wantErr != nil {
				if err != tt.wantErr {
					t.Errorf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !bytes.Equal(enc.Bytes(), tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, enc.Bytes())
			}
		})
	}
}

func TestBEREncoder_WriteLength(t *testing.T) {
	tests := []struct {
		name     string
		length   int
		expected []byte
		wantErr  error
	}{
		// Short form (0-127)
		{
			name:     "length 0",
			length:   0,
			expected: []byte{0x00},
		},
		{
			name:     "length 1",
			length:   1,
			expected: []byte{0x01},
		},
		{
			name:     "length 127 (max short form)",
			length:   127,
			expected: []byte{0x7F},
		},

		// Long form (>127)
		{
			name:     "length 128 (min long form)",
			length:   128,
			expected: []byte{0x81, 0x80},
		},
		{
			name:     "length 255",
			length:   255,
			expected: []byte{0x81, 0xFF},
		},
		{
			name:     "length 256",
			length:   256,
			expected: []byte{0x82, 0x01, 0x00},
		},
		{
			name:     "length 65535",
			length:   65535,
			expected: []byte{0x82, 0xFF, 0xFF},
		},
		{
			name:     "length 65536",
			length:   65536,
			expected: []byte{0x83, 0x01, 0x00, 0x00},
		},
		{
			name:     "length 16777215",
			length:   16777215,
			expected: []byte{0x83, 0xFF, 0xFF, 0xFF},
		},
		{
			name:     "length 16777216",
			length:   16777216,
			expected: []byte{0x84, 0x01, 0x00, 0x00, 0x00},
		},

		// Error cases
		{
			name:    "negative length",
			length:  -1,
			wantErr: ErrNegativeLength,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enc := NewBEREncoder(64)
			err := enc.WriteLength(tt.length)

			if tt.wantErr != nil {
				if err != tt.wantErr {
					t.Errorf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !bytes.Equal(enc.Bytes(), tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, enc.Bytes())
			}
		})
	}
}

func TestBEREncoder_WriteBoolean(t *testing.T) {
	tests := []struct {
		name     string
		value    bool
		expected []byte
	}{
		{
			name:     "false",
			value:    false,
			expected: []byte{0x01, 0x01, 0x00}, // Tag, Length, Value
		},
		{
			name:     "true",
			value:    true,
			expected: []byte{0x01, 0x01, 0xFF}, // Tag, Length, Value (0xFF for true)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enc := NewBEREncoder(64)
			err := enc.WriteBoolean(tt.value)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !bytes.Equal(enc.Bytes(), tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, enc.Bytes())
			}
		})
	}
}

func TestBEREncoder_WriteInteger(t *testing.T) {
	tests := []struct {
		name     string
		value    int64
		expected []byte
	}{
		// Zero
		{
			name:     "zero",
			value:    0,
			expected: []byte{0x02, 0x01, 0x00},
		},

		// Positive values
		{
			name:     "positive 1",
			value:    1,
			expected: []byte{0x02, 0x01, 0x01},
		},
		{
			name:     "positive 127",
			value:    127,
			expected: []byte{0x02, 0x01, 0x7F},
		},
		{
			name:     "positive 128 (needs padding)",
			value:    128,
			expected: []byte{0x02, 0x02, 0x00, 0x80},
		},
		{
			name:     "positive 255",
			value:    255,
			expected: []byte{0x02, 0x02, 0x00, 0xFF},
		},
		{
			name:     "positive 256",
			value:    256,
			expected: []byte{0x02, 0x02, 0x01, 0x00},
		},
		{
			name:     "positive 32767",
			value:    32767,
			expected: []byte{0x02, 0x02, 0x7F, 0xFF},
		},
		{
			name:     "positive 32768 (needs padding)",
			value:    32768,
			expected: []byte{0x02, 0x03, 0x00, 0x80, 0x00},
		},
		{
			name:     "positive 65535",
			value:    65535,
			expected: []byte{0x02, 0x03, 0x00, 0xFF, 0xFF},
		},

		// Negative values (two's complement)
		{
			name:     "negative -1",
			value:    -1,
			expected: []byte{0x02, 0x01, 0xFF},
		},
		{
			name:     "negative -128",
			value:    -128,
			expected: []byte{0x02, 0x01, 0x80},
		},
		{
			name:     "negative -129",
			value:    -129,
			expected: []byte{0x02, 0x02, 0xFF, 0x7F},
		},
		{
			name:     "negative -256",
			value:    -256,
			expected: []byte{0x02, 0x02, 0xFF, 0x00},
		},
		{
			name:     "negative -32768",
			value:    -32768,
			expected: []byte{0x02, 0x02, 0x80, 0x00},
		},
		{
			name:     "negative -32769",
			value:    -32769,
			expected: []byte{0x02, 0x03, 0xFF, 0x7F, 0xFF},
		},

		// Large values
		{
			name:     "max int32",
			value:    2147483647,
			expected: []byte{0x02, 0x04, 0x7F, 0xFF, 0xFF, 0xFF},
		},
		{
			name:     "min int32",
			value:    -2147483648,
			expected: []byte{0x02, 0x04, 0x80, 0x00, 0x00, 0x00},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enc := NewBEREncoder(64)
			err := enc.WriteInteger(tt.value)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !bytes.Equal(enc.Bytes(), tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, enc.Bytes())
			}
		})
	}
}

func TestBEREncoder_WriteOctetString(t *testing.T) {
	tests := []struct {
		name     string
		value    []byte
		expected []byte
	}{
		{
			name:     "empty string",
			value:    []byte{},
			expected: []byte{0x04, 0x00},
		},
		{
			name:     "single byte",
			value:    []byte{0x41},
			expected: []byte{0x04, 0x01, 0x41},
		},
		{
			name:     "hello",
			value:    []byte("hello"),
			expected: []byte{0x04, 0x05, 'h', 'e', 'l', 'l', 'o'},
		},
		{
			name:     "binary data",
			value:    []byte{0x00, 0xFF, 0x80, 0x7F},
			expected: []byte{0x04, 0x04, 0x00, 0xFF, 0x80, 0x7F},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enc := NewBEREncoder(64)
			err := enc.WriteOctetString(tt.value)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !bytes.Equal(enc.Bytes(), tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, enc.Bytes())
			}
		})
	}
}

func TestBEREncoder_WriteOctetString_LongForm(t *testing.T) {
	// Test with a string longer than 127 bytes (requires long form length)
	value := make([]byte, 200)
	for i := range value {
		value[i] = byte(i % 256)
	}

	enc := NewBEREncoder(256)
	err := enc.WriteOctetString(value)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := enc.Bytes()

	// Check tag
	if result[0] != 0x04 {
		t.Errorf("expected tag 0x04, got 0x%02X", result[0])
	}

	// Check length encoding (long form: 0x81 0xC8 for 200)
	if result[1] != 0x81 || result[2] != 0xC8 {
		t.Errorf("expected length bytes [0x81, 0xC8], got [0x%02X, 0x%02X]", result[1], result[2])
	}

	// Check total length
	expectedLen := 1 + 2 + 200 // tag + length bytes + value
	if len(result) != expectedLen {
		t.Errorf("expected total length %d, got %d", expectedLen, len(result))
	}

	// Check value
	if !bytes.Equal(result[3:], value) {
		t.Error("value mismatch")
	}
}

func TestBEREncoder_WriteEnumerated(t *testing.T) {
	tests := []struct {
		name     string
		value    int64
		expected []byte
	}{
		{
			name:     "zero",
			value:    0,
			expected: []byte{0x0A, 0x01, 0x00},
		},
		{
			name:     "positive 1",
			value:    1,
			expected: []byte{0x0A, 0x01, 0x01},
		},
		{
			name:     "positive 255",
			value:    255,
			expected: []byte{0x0A, 0x02, 0x00, 0xFF},
		},
		{
			name:     "negative -1",
			value:    -1,
			expected: []byte{0x0A, 0x01, 0xFF},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enc := NewBEREncoder(64)
			err := enc.WriteEnumerated(tt.value)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !bytes.Equal(enc.Bytes(), tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, enc.Bytes())
			}
		})
	}
}

func TestBEREncoder_WriteNull(t *testing.T) {
	enc := NewBEREncoder(64)
	err := enc.WriteNull()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []byte{0x05, 0x00}
	if !bytes.Equal(enc.Bytes(), expected) {
		t.Errorf("expected %v, got %v", expected, enc.Bytes())
	}
}

func TestBEREncoder_WriteRaw(t *testing.T) {
	enc := NewBEREncoder(64)
	enc.WriteRaw([]byte{0x01, 0x02, 0x03})

	expected := []byte{0x01, 0x02, 0x03}
	if !bytes.Equal(enc.Bytes(), expected) {
		t.Errorf("expected %v, got %v", expected, enc.Bytes())
	}
}

func TestBEREncoder_WriteTaggedValue(t *testing.T) {
	tests := []struct {
		name        string
		tagNumber   int
		constructed bool
		value       []byte
		expected    []byte
	}{
		{
			name:        "context tag 0 primitive",
			tagNumber:   0,
			constructed: false,
			value:       []byte{0x01, 0x02},
			expected:    []byte{0x80, 0x02, 0x01, 0x02},
		},
		{
			name:        "context tag 3 constructed",
			tagNumber:   3,
			constructed: true,
			value:       []byte{0x04, 0x01, 0x41},
			expected:    []byte{0xA3, 0x03, 0x04, 0x01, 0x41},
		},
		{
			name:        "context tag 0 empty",
			tagNumber:   0,
			constructed: false,
			value:       []byte{},
			expected:    []byte{0x80, 0x00},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enc := NewBEREncoder(64)
			err := enc.WriteTaggedValue(tt.tagNumber, tt.constructed, tt.value)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !bytes.Equal(enc.Bytes(), tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, enc.Bytes())
			}
		})
	}
}

func TestBEREncoder_MultipleWrites(t *testing.T) {
	enc := NewBEREncoder(64)

	// Write multiple values
	enc.WriteBoolean(true)
	enc.WriteInteger(42)
	enc.WriteOctetString([]byte("test"))
	enc.WriteNull()

	expected := []byte{
		0x01, 0x01, 0xFF, // Boolean true
		0x02, 0x01, 0x2A, // Integer 42
		0x04, 0x04, 't', 'e', 's', 't', // Octet string "test"
		0x05, 0x00, // Null
	}

	if !bytes.Equal(enc.Bytes(), expected) {
		t.Errorf("expected %v, got %v", expected, enc.Bytes())
	}
}

func TestEncodeInteger_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		value    int64
		expected []byte
	}{
		// Boundary values that test sign bit handling
		{
			name:     "127 (max positive 1-byte)",
			value:    127,
			expected: []byte{0x7F},
		},
		{
			name:     "128 (needs 0x00 prefix)",
			value:    128,
			expected: []byte{0x00, 0x80},
		},
		{
			name:     "-128 (min negative 1-byte)",
			value:    -128,
			expected: []byte{0x80},
		},
		{
			name:     "-129 (needs 2 bytes)",
			value:    -129,
			expected: []byte{0xFF, 0x7F},
		},
		{
			name:     "max int64",
			value:    9223372036854775807,
			expected: []byte{0x7F, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
		},
		{
			name:     "min int64",
			value:    -9223372036854775808,
			expected: []byte{0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := encodeInteger(tt.value)
			if !bytes.Equal(result, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// Benchmark tests
func BenchmarkBEREncoder_WriteInteger(b *testing.B) {
	enc := NewBEREncoder(64)
	for i := 0; i < b.N; i++ {
		enc.Reset()
		enc.WriteInteger(int64(i))
	}
}

func BenchmarkBEREncoder_WriteOctetString(b *testing.B) {
	enc := NewBEREncoder(256)
	data := []byte("This is a test string for benchmarking")
	for i := 0; i < b.N; i++ {
		enc.Reset()
		enc.WriteOctetString(data)
	}
}

func BenchmarkBEREncoder_WriteBoolean(b *testing.B) {
	enc := NewBEREncoder(64)
	for i := 0; i < b.N; i++ {
		enc.Reset()
		enc.WriteBoolean(i%2 == 0)
	}
}
