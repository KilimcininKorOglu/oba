// Package ber implements ASN.1 BER (Basic Encoding Rules) encoding
// as specified in ITU-T X.690.
package ber

import (
	"testing"
)

// BenchmarkBEREncodeInteger benchmarks integer encoding.
func BenchmarkBEREncodeInteger(b *testing.B) {
	enc := NewBEREncoder(64)
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		enc.Reset()
		_ = enc.WriteInteger(int64(i))
	}
}

// BenchmarkBERDecodeInteger benchmarks integer decoding.
func BenchmarkBERDecodeInteger(b *testing.B) {
	// Pre-encode a large integer: 0x7FFFFFFF (max int32)
	data := []byte{0x02, 0x04, 0x7f, 0xff, 0xff, 0xff}
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		dec := NewBERDecoder(data)
		_, _ = dec.ReadInteger()
	}
}

// BenchmarkBEREncodeBoolean benchmarks boolean encoding.
func BenchmarkBEREncodeBoolean(b *testing.B) {
	enc := NewBEREncoder(64)
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		enc.Reset()
		_ = enc.WriteBoolean(true)
	}
}

// BenchmarkBERDecodeBoolean benchmarks boolean decoding.
func BenchmarkBERDecodeBoolean(b *testing.B) {
	// Pre-encode TRUE
	data := []byte{0x01, 0x01, 0xFF}
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		dec := NewBERDecoder(data)
		_, _ = dec.ReadBoolean()
	}
}

// BenchmarkBEREncodeOctetString benchmarks octet string encoding.
func BenchmarkBEREncodeOctetString(b *testing.B) {
	enc := NewBEREncoder(256)
	testData := []byte("uid=alice,ou=users,dc=example,dc=com")
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		enc.Reset()
		_ = enc.WriteOctetString(testData)
	}
}

// BenchmarkBERDecodeOctetString benchmarks octet string decoding.
func BenchmarkBERDecodeOctetString(b *testing.B) {
	// Pre-encode an octet string
	enc := NewBEREncoder(256)
	_ = enc.WriteOctetString([]byte("uid=alice,ou=users,dc=example,dc=com"))
	data := enc.Bytes()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		dec := NewBERDecoder(data)
		_, _ = dec.ReadOctetString()
	}
}

// BenchmarkBEREncodeSequence benchmarks sequence encoding.
func BenchmarkBEREncodeSequence(b *testing.B) {
	enc := NewBEREncoder(256)
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		enc.Reset()
		pos := enc.BeginSequence()
		_ = enc.WriteInteger(int64(i))
		_ = enc.WriteOctetString([]byte("test"))
		_ = enc.EndSequence(pos)
	}
}

// BenchmarkBERDecodeSequence benchmarks sequence decoding.
func BenchmarkBERDecodeSequence(b *testing.B) {
	// Pre-encode a sequence with integer and octet string
	enc := NewBEREncoder(256)
	pos := enc.BeginSequence()
	_ = enc.WriteInteger(12345)
	_ = enc.WriteOctetString([]byte("test"))
	_ = enc.EndSequence(pos)
	data := enc.Bytes()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		dec := NewBERDecoder(data)
		_, _ = dec.ExpectSequence()
		_, _ = dec.ReadInteger()
		_, _ = dec.ReadOctetString()
	}
}

// BenchmarkBEREncodeEnumerated benchmarks enumerated encoding.
func BenchmarkBEREncodeEnumerated(b *testing.B) {
	enc := NewBEREncoder(64)
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		enc.Reset()
		_ = enc.WriteEnumerated(int64(i % 10))
	}
}

// BenchmarkBERDecodeEnumerated benchmarks enumerated decoding.
func BenchmarkBERDecodeEnumerated(b *testing.B) {
	// Pre-encode an enumerated value
	data := []byte{0x0A, 0x01, 0x02}
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		dec := NewBERDecoder(data)
		_, _ = dec.ReadEnumerated()
	}
}

// BenchmarkBEREncodeNull benchmarks null encoding.
func BenchmarkBEREncodeNull(b *testing.B) {
	enc := NewBEREncoder(64)
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		enc.Reset()
		_ = enc.WriteNull()
	}
}

// BenchmarkBERDecodeNull benchmarks null decoding.
func BenchmarkBERDecodeNull(b *testing.B) {
	// Pre-encode NULL
	data := []byte{0x05, 0x00}
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		dec := NewBERDecoder(data)
		_ = dec.ReadNull()
	}
}

// BenchmarkBEREncodeContextTag benchmarks context-specific tag encoding.
func BenchmarkBEREncodeContextTag(b *testing.B) {
	enc := NewBEREncoder(256)
	testData := []byte("test value")
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		enc.Reset()
		_ = enc.WriteTaggedValue(0, false, testData)
	}
}

// BenchmarkBERDecodeContextTag benchmarks context-specific tag decoding.
func BenchmarkBERDecodeContextTag(b *testing.B) {
	// Pre-encode a context-specific tagged value
	enc := NewBEREncoder(256)
	_ = enc.WriteTaggedValue(0, false, []byte("test value"))
	data := enc.Bytes()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		dec := NewBERDecoder(data)
		_, _ = dec.ExpectContextTag(0)
	}
}

// BenchmarkBEREncodeApplicationTag benchmarks application-specific tag encoding.
func BenchmarkBEREncodeApplicationTag(b *testing.B) {
	enc := NewBEREncoder(256)
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		enc.Reset()
		pos := enc.WriteApplicationTag(3, true)
		_ = enc.WriteOctetString([]byte("dc=example,dc=com"))
		_ = enc.EndApplicationTag(pos)
	}
}

// BenchmarkBERDecodeApplicationTag benchmarks application-specific tag decoding.
func BenchmarkBERDecodeApplicationTag(b *testing.B) {
	// Pre-encode an application-specific tagged value
	enc := NewBEREncoder(256)
	pos := enc.WriteApplicationTag(3, true)
	_ = enc.WriteOctetString([]byte("dc=example,dc=com"))
	_ = enc.EndApplicationTag(pos)
	data := enc.Bytes()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		dec := NewBERDecoder(data)
		_, _ = dec.ExpectApplicationTag(3)
	}
}

// BenchmarkBEREncodeLargeOctetString benchmarks encoding large octet strings.
func BenchmarkBEREncodeLargeOctetString(b *testing.B) {
	enc := NewBEREncoder(8192)
	// 4KB octet string
	testData := make([]byte, 4096)
	for i := range testData {
		testData[i] = byte(i % 256)
	}
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		enc.Reset()
		_ = enc.WriteOctetString(testData)
	}
}

// BenchmarkBERDecodeLargeOctetString benchmarks decoding large octet strings.
func BenchmarkBERDecodeLargeOctetString(b *testing.B) {
	// Pre-encode a 4KB octet string
	enc := NewBEREncoder(8192)
	testData := make([]byte, 4096)
	for i := range testData {
		testData[i] = byte(i % 256)
	}
	_ = enc.WriteOctetString(testData)
	data := enc.Bytes()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		dec := NewBERDecoder(data)
		_, _ = dec.ReadOctetString()
	}
}

// BenchmarkBEREncodeNestedSequence benchmarks encoding nested sequences.
func BenchmarkBEREncodeNestedSequence(b *testing.B) {
	enc := NewBEREncoder(512)
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		enc.Reset()
		pos1 := enc.BeginSequence()
		_ = enc.WriteInteger(1)
		pos2 := enc.BeginSequence()
		_ = enc.WriteOctetString([]byte("nested"))
		_ = enc.WriteBoolean(true)
		_ = enc.EndSequence(pos2)
		_ = enc.WriteInteger(2)
		_ = enc.EndSequence(pos1)
	}
}

// BenchmarkBERDecodeNestedSequence benchmarks decoding nested sequences.
func BenchmarkBERDecodeNestedSequence(b *testing.B) {
	// Pre-encode nested sequences
	enc := NewBEREncoder(512)
	pos1 := enc.BeginSequence()
	_ = enc.WriteInteger(1)
	pos2 := enc.BeginSequence()
	_ = enc.WriteOctetString([]byte("nested"))
	_ = enc.WriteBoolean(true)
	_ = enc.EndSequence(pos2)
	_ = enc.WriteInteger(2)
	_ = enc.EndSequence(pos1)
	data := enc.Bytes()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		dec := NewBERDecoder(data)
		_, _ = dec.ExpectSequence()
		_, _ = dec.ReadInteger()
		_, _ = dec.ExpectSequence()
		_, _ = dec.ReadOctetString()
		_, _ = dec.ReadBoolean()
		_, _ = dec.ReadInteger()
	}
}

// BenchmarkBEREncodeLDAPMessage benchmarks encoding a typical LDAP message.
func BenchmarkBEREncodeLDAPMessage(b *testing.B) {
	enc := NewBEREncoder(512)
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		enc.Reset()
		// LDAPMessage envelope
		msgPos := enc.BeginSequence()
		_ = enc.WriteInteger(int64(i)) // messageID
		// SearchRequest [APPLICATION 3]
		reqPos := enc.WriteApplicationTag(3, true)
		_ = enc.WriteOctetString([]byte("dc=example,dc=com")) // baseObject
		_ = enc.WriteEnumerated(2)                            // scope: wholeSubtree
		_ = enc.WriteEnumerated(0)                            // derefAliases: never
		_ = enc.WriteInteger(0)                               // sizeLimit
		_ = enc.WriteInteger(0)                               // timeLimit
		_ = enc.WriteBoolean(false)                           // typesOnly
		// Filter: (objectClass=*)
		_ = enc.WriteTaggedValue(7, false, []byte("objectClass"))
		// Attributes
		attrPos := enc.BeginSequence()
		_ = enc.WriteOctetString([]byte("cn"))
		_ = enc.WriteOctetString([]byte("mail"))
		_ = enc.EndSequence(attrPos)
		_ = enc.EndApplicationTag(reqPos)
		_ = enc.EndSequence(msgPos)
	}
}

// BenchmarkBERDecodeLDAPMessage benchmarks decoding a typical LDAP message.
func BenchmarkBERDecodeLDAPMessage(b *testing.B) {
	// Pre-encode an LDAP SearchRequest message
	enc := NewBEREncoder(512)
	msgPos := enc.BeginSequence()
	_ = enc.WriteInteger(1) // messageID
	reqPos := enc.WriteApplicationTag(3, true)
	_ = enc.WriteOctetString([]byte("dc=example,dc=com"))
	_ = enc.WriteEnumerated(2)
	_ = enc.WriteEnumerated(0)
	_ = enc.WriteInteger(0)
	_ = enc.WriteInteger(0)
	_ = enc.WriteBoolean(false)
	_ = enc.WriteTaggedValue(7, false, []byte("objectClass"))
	attrPos := enc.BeginSequence()
	_ = enc.WriteOctetString([]byte("cn"))
	_ = enc.WriteOctetString([]byte("mail"))
	_ = enc.EndSequence(attrPos)
	_ = enc.EndApplicationTag(reqPos)
	_ = enc.EndSequence(msgPos)
	data := enc.Bytes()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		dec := NewBERDecoder(data)
		_, _ = dec.ExpectSequence()
		_, _ = dec.ReadInteger()
		_, _ = dec.ExpectApplicationTag(3)
		_, _ = dec.ReadOctetString()
		_, _ = dec.ReadEnumerated()
		_, _ = dec.ReadEnumerated()
		_, _ = dec.ReadInteger()
		_, _ = dec.ReadInteger()
		_, _ = dec.ReadBoolean()
		_, _, _, _ = dec.ReadTaggedValue()
		_, _ = dec.ExpectSequence()
		_, _ = dec.ReadOctetString()
		_, _ = dec.ReadOctetString()
	}
}

// BenchmarkBEREncodeSet benchmarks set encoding.
func BenchmarkBEREncodeSet(b *testing.B) {
	enc := NewBEREncoder(256)
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		enc.Reset()
		pos := enc.BeginSet()
		_ = enc.WriteOctetString([]byte("value1"))
		_ = enc.WriteOctetString([]byte("value2"))
		_ = enc.WriteOctetString([]byte("value3"))
		_ = enc.EndSet(pos)
	}
}

// BenchmarkBERDecodeSet benchmarks set decoding.
func BenchmarkBERDecodeSet(b *testing.B) {
	// Pre-encode a set
	enc := NewBEREncoder(256)
	pos := enc.BeginSet()
	_ = enc.WriteOctetString([]byte("value1"))
	_ = enc.WriteOctetString([]byte("value2"))
	_ = enc.WriteOctetString([]byte("value3"))
	_ = enc.EndSet(pos)
	data := enc.Bytes()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		dec := NewBERDecoder(data)
		_, _ = dec.ExpectSet()
		_, _ = dec.ReadOctetString()
		_, _ = dec.ReadOctetString()
		_, _ = dec.ReadOctetString()
	}
}

// BenchmarkBERSkip benchmarks skipping TLV elements.
func BenchmarkBERSkip(b *testing.B) {
	// Pre-encode a sequence with multiple elements
	enc := NewBEREncoder(512)
	pos := enc.BeginSequence()
	_ = enc.WriteInteger(12345)
	_ = enc.WriteOctetString([]byte("test string"))
	_ = enc.WriteBoolean(true)
	_ = enc.EndSequence(pos)
	data := enc.Bytes()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		dec := NewBERDecoder(data)
		_ = dec.Skip()
	}
}

// BenchmarkBERPeekTag benchmarks peeking at tags.
func BenchmarkBERPeekTag(b *testing.B) {
	data := []byte{0x02, 0x01, 0x05} // INTEGER 5
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		dec := NewBERDecoder(data)
		_, _, _, _ = dec.PeekTag()
	}
}

// BenchmarkBERReadRawValue benchmarks reading raw TLV values.
func BenchmarkBERReadRawValue(b *testing.B) {
	// Pre-encode a sequence
	enc := NewBEREncoder(256)
	pos := enc.BeginSequence()
	_ = enc.WriteInteger(12345)
	_ = enc.WriteOctetString([]byte("test"))
	_ = enc.EndSequence(pos)
	data := enc.Bytes()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		dec := NewBERDecoder(data)
		_, _ = dec.ReadRawValue()
	}
}

// BenchmarkBEREncoderReset benchmarks encoder reset performance.
func BenchmarkBEREncoderReset(b *testing.B) {
	enc := NewBEREncoder(256)
	// Fill with some data first
	_ = enc.WriteInteger(12345)
	_ = enc.WriteOctetString([]byte("test"))
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		enc.Reset()
	}
}

// BenchmarkBERDecoderReset benchmarks decoder reset performance.
func BenchmarkBERDecoderReset(b *testing.B) {
	data := []byte{0x02, 0x01, 0x05}
	dec := NewBERDecoder(data)
	// Read some data first
	_, _ = dec.ReadInteger()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		dec.Reset()
	}
}
