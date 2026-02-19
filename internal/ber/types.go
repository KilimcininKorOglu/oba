// Package ber implements ASN.1 BER (Basic Encoding Rules) encoding
// as specified in ITU-T X.690.
package ber

// Tag class constants (bits 7-8 of the tag byte)
const (
	ClassUniversal       = 0x00 // 00xxxxxx
	ClassApplication     = 0x40 // 01xxxxxx
	ClassContextSpecific = 0x80 // 10xxxxxx
	ClassPrivate         = 0xC0 // 11xxxxxx
)

// Constructed flag (bit 6 of the tag byte)
const (
	TypePrimitive   = 0x00 // xx0xxxxx
	TypeConstructed = 0x20 // xx1xxxxx
)

// Universal tag numbers for primitive types
const (
	TagBoolean     = 0x01
	TagInteger     = 0x02
	TagBitString   = 0x03
	TagOctetString = 0x04
	TagNull        = 0x05
	TagOID         = 0x06
	TagEnumerated  = 0x0A
	TagUTF8String  = 0x0C
	TagSequence    = 0x10
	TagSet         = 0x11
)

// Length encoding constants
const (
	// LengthLongFormBit indicates long form length encoding (bit 8 set)
	LengthLongFormBit = 0x80
	// MaxShortFormLength is the maximum length encodable in short form (0-127)
	MaxShortFormLength = 127
)
