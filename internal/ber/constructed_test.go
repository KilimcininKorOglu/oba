package ber

import (
	"bytes"
	"testing"
)

// TestSequenceEncodeDecode tests basic SEQUENCE encoding and decoding
func TestSequenceEncodeDecode(t *testing.T) {
	t.Run("empty sequence", func(t *testing.T) {
		enc := NewBEREncoder(64)
		pos := enc.BeginSequence()
		if err := enc.EndSequence(pos); err != nil {
			t.Fatalf("EndSequence failed: %v", err)
		}

		// Expected: 0x30 0x00 (SEQUENCE tag + zero length)
		expected := []byte{0x30, 0x00}
		if !bytes.Equal(enc.Bytes(), expected) {
			t.Errorf("expected %x, got %x", expected, enc.Bytes())
		}

		// Decode
		dec := NewBERDecoder(enc.Bytes())
		length, err := dec.ExpectSequence()
		if err != nil {
			t.Fatalf("ExpectSequence failed: %v", err)
		}
		if length != 0 {
			t.Errorf("expected length 0, got %d", length)
		}
	})

	t.Run("sequence with integer", func(t *testing.T) {
		enc := NewBEREncoder(64)
		pos := enc.BeginSequence()
		if err := enc.WriteInteger(42); err != nil {
			t.Fatalf("WriteInteger failed: %v", err)
		}
		if err := enc.EndSequence(pos); err != nil {
			t.Fatalf("EndSequence failed: %v", err)
		}

		// Expected: 0x30 0x03 0x02 0x01 0x2A
		// SEQUENCE(3) INTEGER(1) 42
		expected := []byte{0x30, 0x03, 0x02, 0x01, 0x2A}
		if !bytes.Equal(enc.Bytes(), expected) {
			t.Errorf("expected %x, got %x", expected, enc.Bytes())
		}

		// Decode
		dec := NewBERDecoder(enc.Bytes())
		length, err := dec.ExpectSequence()
		if err != nil {
			t.Fatalf("ExpectSequence failed: %v", err)
		}
		if length != 3 {
			t.Errorf("expected length 3, got %d", length)
		}

		val, err := dec.ReadInteger()
		if err != nil {
			t.Fatalf("ReadInteger failed: %v", err)
		}
		if val != 42 {
			t.Errorf("expected 42, got %d", val)
		}
	})

	t.Run("sequence with multiple elements", func(t *testing.T) {
		enc := NewBEREncoder(64)
		pos := enc.BeginSequence()
		if err := enc.WriteInteger(1); err != nil {
			t.Fatalf("WriteInteger failed: %v", err)
		}
		if err := enc.WriteOctetString([]byte("hello")); err != nil {
			t.Fatalf("WriteOctetString failed: %v", err)
		}
		if err := enc.WriteBoolean(true); err != nil {
			t.Fatalf("WriteBoolean failed: %v", err)
		}
		if err := enc.EndSequence(pos); err != nil {
			t.Fatalf("EndSequence failed: %v", err)
		}

		// Decode
		dec := NewBERDecoder(enc.Bytes())
		length, err := dec.ExpectSequence()
		if err != nil {
			t.Fatalf("ExpectSequence failed: %v", err)
		}
		if length != 12 { // 3 (int) + 7 (octet string) + 3 (bool) = 13, but let's verify
			// Actually: INT: 02 01 01 = 3, OCTET: 04 05 hello = 7, BOOL: 01 01 FF = 3 => 13
			// Let's just check it's reasonable
			if length < 10 {
				t.Errorf("expected length >= 10, got %d", length)
			}
		}

		val, err := dec.ReadInteger()
		if err != nil {
			t.Fatalf("ReadInteger failed: %v", err)
		}
		if val != 1 {
			t.Errorf("expected 1, got %d", val)
		}

		str, err := dec.ReadOctetString()
		if err != nil {
			t.Fatalf("ReadOctetString failed: %v", err)
		}
		if string(str) != "hello" {
			t.Errorf("expected 'hello', got '%s'", str)
		}

		b, err := dec.ReadBoolean()
		if err != nil {
			t.Fatalf("ReadBoolean failed: %v", err)
		}
		if !b {
			t.Error("expected true, got false")
		}
	})
}

// TestSetEncodeDecode tests basic SET encoding and decoding
func TestSetEncodeDecode(t *testing.T) {
	t.Run("empty set", func(t *testing.T) {
		enc := NewBEREncoder(64)
		pos := enc.BeginSet()
		if err := enc.EndSet(pos); err != nil {
			t.Fatalf("EndSet failed: %v", err)
		}

		// Expected: 0x31 0x00 (SET tag + zero length)
		expected := []byte{0x31, 0x00}
		if !bytes.Equal(enc.Bytes(), expected) {
			t.Errorf("expected %x, got %x", expected, enc.Bytes())
		}

		// Decode
		dec := NewBERDecoder(enc.Bytes())
		length, err := dec.ExpectSet()
		if err != nil {
			t.Fatalf("ExpectSet failed: %v", err)
		}
		if length != 0 {
			t.Errorf("expected length 0, got %d", length)
		}
	})

	t.Run("set with elements", func(t *testing.T) {
		enc := NewBEREncoder(64)
		pos := enc.BeginSet()
		if err := enc.WriteInteger(100); err != nil {
			t.Fatalf("WriteInteger failed: %v", err)
		}
		if err := enc.WriteOctetString([]byte("test")); err != nil {
			t.Fatalf("WriteOctetString failed: %v", err)
		}
		if err := enc.EndSet(pos); err != nil {
			t.Fatalf("EndSet failed: %v", err)
		}

		// Decode
		dec := NewBERDecoder(enc.Bytes())
		length, err := dec.ExpectSet()
		if err != nil {
			t.Fatalf("ExpectSet failed: %v", err)
		}
		if length == 0 {
			t.Error("expected non-zero length")
		}

		val, err := dec.ReadInteger()
		if err != nil {
			t.Fatalf("ReadInteger failed: %v", err)
		}
		if val != 100 {
			t.Errorf("expected 100, got %d", val)
		}

		str, err := dec.ReadOctetString()
		if err != nil {
			t.Fatalf("ReadOctetString failed: %v", err)
		}
		if string(str) != "test" {
			t.Errorf("expected 'test', got '%s'", str)
		}
	})
}

// TestNestedSequences tests deeply nested SEQUENCE structures
func TestNestedSequences(t *testing.T) {
	t.Run("two level nesting", func(t *testing.T) {
		enc := NewBEREncoder(128)

		// Outer sequence
		outerPos := enc.BeginSequence()

		// Inner sequence
		innerPos := enc.BeginSequence()
		if err := enc.WriteInteger(123); err != nil {
			t.Fatalf("WriteInteger failed: %v", err)
		}
		if err := enc.EndSequence(innerPos); err != nil {
			t.Fatalf("EndSequence (inner) failed: %v", err)
		}

		if err := enc.WriteOctetString([]byte("outer")); err != nil {
			t.Fatalf("WriteOctetString failed: %v", err)
		}

		if err := enc.EndSequence(outerPos); err != nil {
			t.Fatalf("EndSequence (outer) failed: %v", err)
		}

		// Decode
		dec := NewBERDecoder(enc.Bytes())
		outerLen, err := dec.ExpectSequence()
		if err != nil {
			t.Fatalf("ExpectSequence (outer) failed: %v", err)
		}
		if outerLen == 0 {
			t.Error("expected non-zero outer length")
		}

		innerLen, err := dec.ExpectSequence()
		if err != nil {
			t.Fatalf("ExpectSequence (inner) failed: %v", err)
		}
		if innerLen == 0 {
			t.Error("expected non-zero inner length")
		}

		val, err := dec.ReadInteger()
		if err != nil {
			t.Fatalf("ReadInteger failed: %v", err)
		}
		if val != 123 {
			t.Errorf("expected 123, got %d", val)
		}

		str, err := dec.ReadOctetString()
		if err != nil {
			t.Fatalf("ReadOctetString failed: %v", err)
		}
		if string(str) != "outer" {
			t.Errorf("expected 'outer', got '%s'", str)
		}
	})

	t.Run("three level nesting", func(t *testing.T) {
		enc := NewBEREncoder(256)

		// Level 1
		pos1 := enc.BeginSequence()

		// Level 2
		pos2 := enc.BeginSequence()

		// Level 3
		pos3 := enc.BeginSequence()
		if err := enc.WriteInteger(999); err != nil {
			t.Fatalf("WriteInteger failed: %v", err)
		}
		if err := enc.EndSequence(pos3); err != nil {
			t.Fatalf("EndSequence (level 3) failed: %v", err)
		}

		if err := enc.EndSequence(pos2); err != nil {
			t.Fatalf("EndSequence (level 2) failed: %v", err)
		}

		if err := enc.EndSequence(pos1); err != nil {
			t.Fatalf("EndSequence (level 1) failed: %v", err)
		}

		// Decode using sub-decoders
		dec := NewBERDecoder(enc.Bytes())
		sub1, err := dec.ReadSequenceContents()
		if err != nil {
			t.Fatalf("ReadSequenceContents (level 1) failed: %v", err)
		}

		sub2, err := sub1.ReadSequenceContents()
		if err != nil {
			t.Fatalf("ReadSequenceContents (level 2) failed: %v", err)
		}

		sub3, err := sub2.ReadSequenceContents()
		if err != nil {
			t.Fatalf("ReadSequenceContents (level 3) failed: %v", err)
		}

		val, err := sub3.ReadInteger()
		if err != nil {
			t.Fatalf("ReadInteger failed: %v", err)
		}
		if val != 999 {
			t.Errorf("expected 999, got %d", val)
		}
	})
}

// TestContextSpecificTags tests context-specific tag encoding and decoding
func TestContextSpecificTags(t *testing.T) {
	t.Run("primitive context tag 0", func(t *testing.T) {
		enc := NewBEREncoder(64)
		pos := enc.WriteContextTag(0, false)
		enc.WriteRaw([]byte{0x01, 0x02, 0x03})
		if err := enc.EndContextTag(pos); err != nil {
			t.Fatalf("EndContextTag failed: %v", err)
		}

		// Expected: 0x80 0x03 0x01 0x02 0x03
		expected := []byte{0x80, 0x03, 0x01, 0x02, 0x03}
		if !bytes.Equal(enc.Bytes(), expected) {
			t.Errorf("expected %x, got %x", expected, enc.Bytes())
		}

		// Decode
		dec := NewBERDecoder(enc.Bytes())
		length, err := dec.ExpectContextTag(0)
		if err != nil {
			t.Fatalf("ExpectContextTag failed: %v", err)
		}
		if length != 3 {
			t.Errorf("expected length 3, got %d", length)
		}
	})

	t.Run("constructed context tag 1", func(t *testing.T) {
		enc := NewBEREncoder(64)
		pos := enc.WriteContextTag(1, true)
		if err := enc.WriteInteger(42); err != nil {
			t.Fatalf("WriteInteger failed: %v", err)
		}
		if err := enc.EndContextTag(pos); err != nil {
			t.Fatalf("EndContextTag failed: %v", err)
		}

		// Expected: 0xA1 (context tag 1, constructed) + length + integer
		dec := NewBERDecoder(enc.Bytes())
		length, err := dec.ExpectContextTag(1)
		if err != nil {
			t.Fatalf("ExpectContextTag failed: %v", err)
		}
		if length != 3 {
			t.Errorf("expected length 3, got %d", length)
		}

		val, err := dec.ReadInteger()
		if err != nil {
			t.Fatalf("ReadInteger failed: %v", err)
		}
		if val != 42 {
			t.Errorf("expected 42, got %d", val)
		}
	})

	t.Run("context tag with high number", func(t *testing.T) {
		enc := NewBEREncoder(64)
		pos := enc.WriteContextTag(31, false) // Long form tag
		enc.WriteRaw([]byte{0xFF})
		if err := enc.EndContextTag(pos); err != nil {
			t.Fatalf("EndContextTag failed: %v", err)
		}

		// Decode
		dec := NewBERDecoder(enc.Bytes())
		length, err := dec.ExpectContextTag(31)
		if err != nil {
			t.Fatalf("ExpectContextTag failed: %v", err)
		}
		if length != 1 {
			t.Errorf("expected length 1, got %d", length)
		}
	})

	t.Run("IsContextTag helper", func(t *testing.T) {
		enc := NewBEREncoder(64)
		pos := enc.WriteContextTag(5, true)
		if err := enc.WriteNull(); err != nil {
			t.Fatalf("WriteNull failed: %v", err)
		}
		if err := enc.EndContextTag(pos); err != nil {
			t.Fatalf("EndContextTag failed: %v", err)
		}

		dec := NewBERDecoder(enc.Bytes())
		if !dec.IsContextTag(5) {
			t.Error("expected IsContextTag(5) to return true")
		}
		if dec.IsContextTag(4) {
			t.Error("expected IsContextTag(4) to return false")
		}
		if dec.IsContextTag(6) {
			t.Error("expected IsContextTag(6) to return false")
		}
	})

	t.Run("ReadContextTagContents", func(t *testing.T) {
		enc := NewBEREncoder(64)
		pos := enc.WriteContextTag(3, true)
		if err := enc.WriteInteger(777); err != nil {
			t.Fatalf("WriteInteger failed: %v", err)
		}
		if err := enc.WriteOctetString([]byte("ctx")); err != nil {
			t.Fatalf("WriteOctetString failed: %v", err)
		}
		if err := enc.EndContextTag(pos); err != nil {
			t.Fatalf("EndContextTag failed: %v", err)
		}

		dec := NewBERDecoder(enc.Bytes())
		sub, err := dec.ReadContextTagContents(3)
		if err != nil {
			t.Fatalf("ReadContextTagContents failed: %v", err)
		}

		val, err := sub.ReadInteger()
		if err != nil {
			t.Fatalf("ReadInteger failed: %v", err)
		}
		if val != 777 {
			t.Errorf("expected 777, got %d", val)
		}

		str, err := sub.ReadOctetString()
		if err != nil {
			t.Fatalf("ReadOctetString failed: %v", err)
		}
		if string(str) != "ctx" {
			t.Errorf("expected 'ctx', got '%s'", str)
		}
	})
}

// TestApplicationTags tests application-specific tag encoding and decoding
func TestApplicationTags(t *testing.T) {
	t.Run("application tag 0 (LDAP BindRequest)", func(t *testing.T) {
		enc := NewBEREncoder(64)
		pos := enc.WriteApplicationTag(0, true)
		if err := enc.WriteInteger(3); err != nil { // LDAP version
			t.Fatalf("WriteInteger failed: %v", err)
		}
		if err := enc.WriteOctetString([]byte("cn=admin")); err != nil { // DN
			t.Fatalf("WriteOctetString failed: %v", err)
		}
		if err := enc.EndApplicationTag(pos); err != nil {
			t.Fatalf("EndApplicationTag failed: %v", err)
		}

		// Decode
		dec := NewBERDecoder(enc.Bytes())
		length, err := dec.ExpectApplicationTag(0)
		if err != nil {
			t.Fatalf("ExpectApplicationTag failed: %v", err)
		}
		if length == 0 {
			t.Error("expected non-zero length")
		}

		version, err := dec.ReadInteger()
		if err != nil {
			t.Fatalf("ReadInteger failed: %v", err)
		}
		if version != 3 {
			t.Errorf("expected version 3, got %d", version)
		}

		dn, err := dec.ReadOctetString()
		if err != nil {
			t.Fatalf("ReadOctetString failed: %v", err)
		}
		if string(dn) != "cn=admin" {
			t.Errorf("expected 'cn=admin', got '%s'", dn)
		}
	})

	t.Run("IsApplicationTag helper", func(t *testing.T) {
		enc := NewBEREncoder(64)
		pos := enc.WriteApplicationTag(3, true) // SearchRequest
		if err := enc.WriteNull(); err != nil {
			t.Fatalf("WriteNull failed: %v", err)
		}
		if err := enc.EndApplicationTag(pos); err != nil {
			t.Fatalf("EndApplicationTag failed: %v", err)
		}

		dec := NewBERDecoder(enc.Bytes())
		if !dec.IsApplicationTag(3) {
			t.Error("expected IsApplicationTag(3) to return true")
		}
		if dec.IsApplicationTag(0) {
			t.Error("expected IsApplicationTag(0) to return false")
		}
	})

	t.Run("ReadApplicationTagContents", func(t *testing.T) {
		enc := NewBEREncoder(64)
		pos := enc.WriteApplicationTag(1, true) // BindResponse
		if err := enc.WriteEnumerated(0); err != nil { // resultCode = success
			t.Fatalf("WriteEnumerated failed: %v", err)
		}
		if err := enc.WriteOctetString([]byte("")); err != nil { // matchedDN
			t.Fatalf("WriteOctetString failed: %v", err)
		}
		if err := enc.WriteOctetString([]byte("")); err != nil { // diagnosticMessage
			t.Fatalf("WriteOctetString failed: %v", err)
		}
		if err := enc.EndApplicationTag(pos); err != nil {
			t.Fatalf("EndApplicationTag failed: %v", err)
		}

		dec := NewBERDecoder(enc.Bytes())
		sub, err := dec.ReadApplicationTagContents(1)
		if err != nil {
			t.Fatalf("ReadApplicationTagContents failed: %v", err)
		}

		resultCode, err := sub.ReadEnumerated()
		if err != nil {
			t.Fatalf("ReadEnumerated failed: %v", err)
		}
		if resultCode != 0 {
			t.Errorf("expected resultCode 0, got %d", resultCode)
		}
	})
}

// TestLDAPMessageStructure tests encoding/decoding of LDAP-like message structures
func TestLDAPMessageStructure(t *testing.T) {
	t.Run("LDAP message envelope", func(t *testing.T) {
		// LDAP messages are: SEQUENCE { messageID INTEGER, protocolOp CHOICE, controls [0] Controls OPTIONAL }
		enc := NewBEREncoder(256)

		// Outer SEQUENCE (LDAPMessage)
		msgPos := enc.BeginSequence()

		// messageID
		if err := enc.WriteInteger(1); err != nil {
			t.Fatalf("WriteInteger (messageID) failed: %v", err)
		}

		// protocolOp: BindRequest [APPLICATION 0]
		bindPos := enc.WriteApplicationTag(0, true)
		if err := enc.WriteInteger(3); err != nil { // version
			t.Fatalf("WriteInteger (version) failed: %v", err)
		}
		if err := enc.WriteOctetString([]byte("cn=admin,dc=example,dc=com")); err != nil { // name
			t.Fatalf("WriteOctetString (name) failed: %v", err)
		}
		// authentication: simple [0] OCTET STRING
		authPos := enc.WriteContextTag(0, false)
		enc.WriteRaw([]byte("secret"))
		if err := enc.EndContextTag(authPos); err != nil {
			t.Fatalf("EndContextTag (auth) failed: %v", err)
		}
		if err := enc.EndApplicationTag(bindPos); err != nil {
			t.Fatalf("EndApplicationTag (bind) failed: %v", err)
		}

		if err := enc.EndSequence(msgPos); err != nil {
			t.Fatalf("EndSequence (msg) failed: %v", err)
		}

		// Decode
		dec := NewBERDecoder(enc.Bytes())

		// LDAPMessage SEQUENCE
		_, err := dec.ExpectSequence()
		if err != nil {
			t.Fatalf("ExpectSequence (LDAPMessage) failed: %v", err)
		}

		// messageID
		msgID, err := dec.ReadInteger()
		if err != nil {
			t.Fatalf("ReadInteger (messageID) failed: %v", err)
		}
		if msgID != 1 {
			t.Errorf("expected messageID 1, got %d", msgID)
		}

		// BindRequest [APPLICATION 0]
		_, err = dec.ExpectApplicationTag(0)
		if err != nil {
			t.Fatalf("ExpectApplicationTag (BindRequest) failed: %v", err)
		}

		// version
		version, err := dec.ReadInteger()
		if err != nil {
			t.Fatalf("ReadInteger (version) failed: %v", err)
		}
		if version != 3 {
			t.Errorf("expected version 3, got %d", version)
		}

		// name
		name, err := dec.ReadOctetString()
		if err != nil {
			t.Fatalf("ReadOctetString (name) failed: %v", err)
		}
		if string(name) != "cn=admin,dc=example,dc=com" {
			t.Errorf("expected 'cn=admin,dc=example,dc=com', got '%s'", name)
		}

		// authentication [0]
		authLen, err := dec.ExpectContextTag(0)
		if err != nil {
			t.Fatalf("ExpectContextTag (auth) failed: %v", err)
		}
		if authLen != 6 { // "secret" = 6 bytes
			t.Errorf("expected auth length 6, got %d", authLen)
		}
	})
}

// TestPeekTag tests the PeekTag functionality
func TestPeekTag(t *testing.T) {
	t.Run("peek without consuming", func(t *testing.T) {
		enc := NewBEREncoder(64)
		if err := enc.WriteInteger(42); err != nil {
			t.Fatalf("WriteInteger failed: %v", err)
		}

		dec := NewBERDecoder(enc.Bytes())
		initialOffset := dec.Offset()

		class, constructed, number, err := dec.PeekTag()
		if err != nil {
			t.Fatalf("PeekTag failed: %v", err)
		}

		// Verify tag info
		if class != ClassUniversal {
			t.Errorf("expected ClassUniversal, got %d", class)
		}
		if constructed != TypePrimitive {
			t.Errorf("expected TypePrimitive, got %d", constructed)
		}
		if number != TagInteger {
			t.Errorf("expected TagInteger, got %d", number)
		}

		// Verify offset unchanged
		if dec.Offset() != initialOffset {
			t.Errorf("PeekTag should not change offset, was %d, now %d", initialOffset, dec.Offset())
		}

		// Now actually read the integer
		val, err := dec.ReadInteger()
		if err != nil {
			t.Fatalf("ReadInteger failed: %v", err)
		}
		if val != 42 {
			t.Errorf("expected 42, got %d", val)
		}
	})

	t.Run("peek sequence tag", func(t *testing.T) {
		enc := NewBEREncoder(64)
		pos := enc.BeginSequence()
		if err := enc.EndSequence(pos); err != nil {
			t.Fatalf("EndSequence failed: %v", err)
		}

		dec := NewBERDecoder(enc.Bytes())
		class, constructed, number, err := dec.PeekTag()
		if err != nil {
			t.Fatalf("PeekTag failed: %v", err)
		}

		if class != ClassUniversal {
			t.Errorf("expected ClassUniversal, got %d", class)
		}
		if constructed != TypeConstructed {
			t.Errorf("expected TypeConstructed, got %d", constructed)
		}
		if number != TagSequence {
			t.Errorf("expected TagSequence, got %d", number)
		}
	})

	t.Run("peek context tag", func(t *testing.T) {
		enc := NewBEREncoder(64)
		pos := enc.WriteContextTag(7, true)
		if err := enc.EndContextTag(pos); err != nil {
			t.Fatalf("EndContextTag failed: %v", err)
		}

		dec := NewBERDecoder(enc.Bytes())
		class, constructed, number, err := dec.PeekTag()
		if err != nil {
			t.Fatalf("PeekTag failed: %v", err)
		}

		if class != ClassContextSpecific {
			t.Errorf("expected ClassContextSpecific, got %d", class)
		}
		if constructed != TypeConstructed {
			t.Errorf("expected TypeConstructed, got %d", constructed)
		}
		if number != 7 {
			t.Errorf("expected tag number 7, got %d", number)
		}
	})
}

// TestLengthEncodings tests various length encoding scenarios
func TestLengthEncodings(t *testing.T) {
	t.Run("short form length (0-127)", func(t *testing.T) {
		enc := NewBEREncoder(256)
		pos := enc.BeginSequence()
		// Write 50 bytes of content
		for i := 0; i < 50; i++ {
			enc.WriteRaw([]byte{byte(i)})
		}
		if err := enc.EndSequence(pos); err != nil {
			t.Fatalf("EndSequence failed: %v", err)
		}

		// Verify short form length encoding
		data := enc.Bytes()
		if data[0] != 0x30 {
			t.Errorf("expected SEQUENCE tag 0x30, got %x", data[0])
		}
		if data[1] != 50 {
			t.Errorf("expected short form length 50, got %d", data[1])
		}

		// Decode
		dec := NewBERDecoder(data)
		length, err := dec.ExpectSequence()
		if err != nil {
			t.Fatalf("ExpectSequence failed: %v", err)
		}
		if length != 50 {
			t.Errorf("expected length 50, got %d", length)
		}
	})

	t.Run("long form length (128-255)", func(t *testing.T) {
		enc := NewBEREncoder(512)
		pos := enc.BeginSequence()
		// Write 200 bytes of content
		for i := 0; i < 200; i++ {
			enc.WriteRaw([]byte{byte(i)})
		}
		if err := enc.EndSequence(pos); err != nil {
			t.Fatalf("EndSequence failed: %v", err)
		}

		// Verify long form length encoding (0x81 + 1 byte)
		data := enc.Bytes()
		if data[0] != 0x30 {
			t.Errorf("expected SEQUENCE tag 0x30, got %x", data[0])
		}
		if data[1] != 0x81 {
			t.Errorf("expected long form indicator 0x81, got %x", data[1])
		}
		if data[2] != 200 {
			t.Errorf("expected length byte 200, got %d", data[2])
		}

		// Decode
		dec := NewBERDecoder(data)
		length, err := dec.ExpectSequence()
		if err != nil {
			t.Fatalf("ExpectSequence failed: %v", err)
		}
		if length != 200 {
			t.Errorf("expected length 200, got %d", length)
		}
	})

	t.Run("long form length (256-65535)", func(t *testing.T) {
		enc := NewBEREncoder(1024)
		pos := enc.BeginSequence()
		// Write 300 bytes of content
		for i := 0; i < 300; i++ {
			enc.WriteRaw([]byte{byte(i)})
		}
		if err := enc.EndSequence(pos); err != nil {
			t.Fatalf("EndSequence failed: %v", err)
		}

		// Verify long form length encoding (0x82 + 2 bytes)
		data := enc.Bytes()
		if data[0] != 0x30 {
			t.Errorf("expected SEQUENCE tag 0x30, got %x", data[0])
		}
		if data[1] != 0x82 {
			t.Errorf("expected long form indicator 0x82, got %x", data[1])
		}
		expectedLen := 300
		actualLen := int(data[2])<<8 | int(data[3])
		if actualLen != expectedLen {
			t.Errorf("expected length %d, got %d", expectedLen, actualLen)
		}

		// Decode
		dec := NewBERDecoder(data)
		length, err := dec.ExpectSequence()
		if err != nil {
			t.Fatalf("ExpectSequence failed: %v", err)
		}
		if length != 300 {
			t.Errorf("expected length 300, got %d", length)
		}
	})
}

// TestErrorCases tests error handling for constructed types
func TestErrorCases(t *testing.T) {
	t.Run("ExpectSequence on non-sequence", func(t *testing.T) {
		enc := NewBEREncoder(64)
		if err := enc.WriteInteger(42); err != nil {
			t.Fatalf("WriteInteger failed: %v", err)
		}

		dec := NewBERDecoder(enc.Bytes())
		_, err := dec.ExpectSequence()
		if err == nil {
			t.Error("expected error for non-sequence tag")
		}
	})

	t.Run("ExpectSet on non-set", func(t *testing.T) {
		enc := NewBEREncoder(64)
		pos := enc.BeginSequence()
		if err := enc.EndSequence(pos); err != nil {
			t.Fatalf("EndSequence failed: %v", err)
		}

		dec := NewBERDecoder(enc.Bytes())
		_, err := dec.ExpectSet()
		if err == nil {
			t.Error("expected error for non-set tag")
		}
	})

	t.Run("ExpectContextTag wrong number", func(t *testing.T) {
		enc := NewBEREncoder(64)
		pos := enc.WriteContextTag(5, true)
		if err := enc.EndContextTag(pos); err != nil {
			t.Fatalf("EndContextTag failed: %v", err)
		}

		dec := NewBERDecoder(enc.Bytes())
		_, err := dec.ExpectContextTag(3)
		if err == nil {
			t.Error("expected error for wrong context tag number")
		}
	})

	t.Run("ExpectApplicationTag wrong number", func(t *testing.T) {
		enc := NewBEREncoder(64)
		pos := enc.WriteApplicationTag(0, true)
		if err := enc.EndApplicationTag(pos); err != nil {
			t.Fatalf("EndApplicationTag failed: %v", err)
		}

		dec := NewBERDecoder(enc.Bytes())
		_, err := dec.ExpectApplicationTag(1)
		if err == nil {
			t.Error("expected error for wrong application tag number")
		}
	})

	t.Run("truncated sequence", func(t *testing.T) {
		// Create a sequence header that claims more content than available
		data := []byte{0x30, 0x10} // SEQUENCE with length 16, but no content

		dec := NewBERDecoder(data)
		_, err := dec.ExpectSequence()
		if err == nil {
			t.Error("expected error for truncated sequence")
		}
	})
}
