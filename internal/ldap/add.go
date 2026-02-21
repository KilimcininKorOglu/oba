// Package ldap implements LDAP protocol message parsing and encoding
// as specified in RFC 4511.
package ldap

import (
	"errors"

	"github.com/KilimcininKorOglu/oba/internal/ber"
)

// Attribute represents an LDAP attribute with its values
type Attribute struct {
	// Type is the attribute type name
	Type string
	// Values contains the attribute values
	Values [][]byte
}

// AddRequest represents an LDAP Add Request
// AddRequest ::= [APPLICATION 8] SEQUENCE {
//
//	entry           LDAPDN,
//	attributes      AttributeList
//
// }
// AttributeList ::= SEQUENCE OF attribute Attribute
// Attribute ::= PartialAttribute(WITH VALUES)
// PartialAttribute ::= SEQUENCE {
//
//	type       AttributeDescription,
//	vals       SET OF value AttributeValue
//
// }
type AddRequest struct {
	// Entry is the DN of the entry to add
	Entry string
	// Attributes contains the attributes for the new entry
	Attributes []Attribute
}

// Errors for AddRequest parsing
var (
	// ErrEmptyEntry is returned when the entry DN is empty
	ErrEmptyEntry = errors.New("ldap: entry DN cannot be empty")
	// ErrInvalidAttribute is returned when an attribute is malformed
	ErrInvalidAttribute = errors.New("ldap: invalid attribute")
	// ErrEmptyAttributeValues is returned when an attribute has no values
	ErrEmptyAttributeValues = errors.New("ldap: attribute must have at least one value")
)

// ParseAddRequest parses an AddRequest from raw operation data.
// The data should be the contents of the APPLICATION 8 tag (without the tag and length).
func ParseAddRequest(data []byte) (*AddRequest, error) {
	if len(data) == 0 {
		return nil, NewParseError(0, "empty add request data", nil)
	}

	decoder := ber.NewBERDecoder(data)
	req := &AddRequest{}

	// Read entry DN (LDAPDN - OCTET STRING)
	entryBytes, err := decoder.ReadOctetString()
	if err != nil {
		return nil, NewParseError(decoder.Offset(), "failed to read entry DN", err)
	}
	req.Entry = string(entryBytes)

	// Read attributes (SEQUENCE OF Attribute)
	attrListLen, err := decoder.ExpectSequence()
	if err != nil {
		return nil, NewParseError(decoder.Offset(), "failed to read attributes sequence", err)
	}

	attrListEnd := decoder.Offset() + attrListLen
	var attributes []Attribute

	for decoder.Offset() < attrListEnd && decoder.Remaining() > 0 {
		attr, err := parseAttribute(decoder)
		if err != nil {
			return nil, err
		}
		attributes = append(attributes, attr)
	}

	req.Attributes = attributes
	return req, nil
}

// parseAttribute parses a single attribute from the decoder
// Attribute ::= PartialAttribute(WITH VALUES)
// PartialAttribute ::= SEQUENCE {
//
//	type       AttributeDescription,
//	vals       SET OF value AttributeValue
//
// }
func parseAttribute(decoder *ber.BERDecoder) (Attribute, error) {
	attr := Attribute{}

	// Read the attribute SEQUENCE
	attrDecoder, err := decoder.ReadSequenceContents()
	if err != nil {
		return attr, NewParseError(decoder.Offset(), "failed to read attribute sequence", err)
	}

	// Read attribute type (OCTET STRING)
	typeBytes, err := attrDecoder.ReadOctetString()
	if err != nil {
		return attr, NewParseError(decoder.Offset(), "failed to read attribute type", err)
	}
	attr.Type = string(typeBytes)

	// Read attribute values (SET OF OCTET STRING)
	valSetLen, err := attrDecoder.ExpectSet()
	if err != nil {
		return attr, NewParseError(decoder.Offset(), "failed to read attribute values set", err)
	}

	valSetEnd := attrDecoder.Offset() + valSetLen
	var values [][]byte

	for attrDecoder.Offset() < valSetEnd && attrDecoder.Remaining() > 0 {
		valueBytes, err := attrDecoder.ReadOctetString()
		if err != nil {
			return attr, NewParseError(decoder.Offset(), "failed to read attribute value", err)
		}
		values = append(values, valueBytes)
	}

	attr.Values = values
	return attr, nil
}

// Encode encodes the AddRequest to BER format (without the APPLICATION tag).
func (r *AddRequest) Encode() ([]byte, error) {
	encoder := ber.NewBEREncoder(256)

	// Write entry DN (OCTET STRING)
	if err := encoder.WriteOctetString([]byte(r.Entry)); err != nil {
		return nil, err
	}

	// Write attributes (SEQUENCE OF Attribute)
	attrListPos := encoder.BeginSequence()

	for _, attr := range r.Attributes {
		if err := encodeAttribute(encoder, attr); err != nil {
			return nil, err
		}
	}

	if err := encoder.EndSequence(attrListPos); err != nil {
		return nil, err
	}

	return encoder.Bytes(), nil
}

// encodeAttribute encodes a single attribute
func encodeAttribute(encoder *ber.BEREncoder, attr Attribute) error {
	// Start attribute SEQUENCE
	attrPos := encoder.BeginSequence()

	// Write attribute type
	if err := encoder.WriteOctetString([]byte(attr.Type)); err != nil {
		return err
	}

	// Write attribute values (SET OF OCTET STRING)
	valSetPos := encoder.BeginSet()

	for _, value := range attr.Values {
		if err := encoder.WriteOctetString(value); err != nil {
			return err
		}
	}

	if err := encoder.EndSet(valSetPos); err != nil {
		return err
	}

	return encoder.EndSequence(attrPos)
}

// GetAttribute returns the first attribute with the given type name, or nil if not found.
func (r *AddRequest) GetAttribute(attrType string) *Attribute {
	for i := range r.Attributes {
		if r.Attributes[i].Type == attrType {
			return &r.Attributes[i]
		}
	}
	return nil
}

// GetAttributeValues returns all values for the given attribute type, or nil if not found.
func (r *AddRequest) GetAttributeValues(attrType string) [][]byte {
	attr := r.GetAttribute(attrType)
	if attr == nil {
		return nil
	}
	return attr.Values
}

// GetAttributeStringValues returns all values for the given attribute type as strings.
func (r *AddRequest) GetAttributeStringValues(attrType string) []string {
	values := r.GetAttributeValues(attrType)
	if values == nil {
		return nil
	}
	result := make([]string, len(values))
	for i, v := range values {
		result[i] = string(v)
	}
	return result
}
