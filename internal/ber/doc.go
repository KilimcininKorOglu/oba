// Package ber implements ASN.1 BER (Basic Encoding Rules) encoding and decoding
// as specified in ITU-T X.690.
//
// BER is the wire format used by LDAP for all protocol messages. This package
// provides low-level primitives for encoding and decoding BER data structures.
//
// # Tag Classes
//
// BER uses four tag classes to identify data types:
//
//   - Universal (0x00): Standard ASN.1 types like INTEGER, BOOLEAN, SEQUENCE
//   - Application (0x40): Protocol-specific types (LDAP operations)
//   - Context-specific (0x80): Context-dependent types within a structure
//   - Private (0xC0): Organization-specific types
//
// # Encoding
//
// Use BEREncoder to build BER-encoded data:
//
//	encoder := ber.NewBEREncoder(256)
//	encoder.WriteInteger(42)
//	encoder.WriteOctetString([]byte("hello"))
//	data := encoder.Bytes()
//
// For constructed types (SEQUENCE, SET), use Begin/End methods:
//
//	encoder := ber.NewBEREncoder(256)
//	pos := encoder.BeginSequence()
//	encoder.WriteInteger(1)
//	encoder.WriteInteger(2)
//	encoder.EndSequence(pos)
//
// # Decoding
//
// Use BERDecoder to parse BER-encoded data:
//
//	decoder := ber.NewBERDecoder(data)
//	value, err := decoder.ReadInteger()
//	if err != nil {
//	    // handle error
//	}
//
// For constructed types, use ExpectSequence to get the content length:
//
//	decoder := ber.NewBERDecoder(data)
//	length, err := decoder.ExpectSequence()
//	if err != nil {
//	    // handle error
//	}
//	// Read 'length' bytes of sequence content
//
// # Universal Tags
//
// The package defines constants for common universal tags:
//
//   - TagBoolean (0x01): Boolean values
//   - TagInteger (0x02): Integer values
//   - TagOctetString (0x04): Byte strings
//   - TagNull (0x05): Null value
//   - TagOID (0x06): Object identifiers
//   - TagEnumerated (0x0A): Enumerated values
//   - TagSequence (0x10): Ordered collection
//   - TagSet (0x11): Unordered collection
//
// # References
//
//   - ITU-T X.690: ASN.1 encoding rules
//   - RFC 4511: LDAP Protocol (uses BER encoding)
package ber
