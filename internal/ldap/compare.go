// Package ldap implements LDAP protocol message parsing and encoding
// as specified in RFC 4511.
package ldap

import (
	"errors"

	"github.com/KilimcininKorOglu/oba/internal/ber"
)

// CompareRequest represents an LDAP Compare Request
// CompareRequest ::= [APPLICATION 14] SEQUENCE {
//
//	entry           LDAPDN,
//	ava             AttributeValueAssertion
//
// }
// AttributeValueAssertion ::= SEQUENCE {
//
//	attributeDesc   AttributeDescription,
//	assertionValue  AssertionValue
//
// }
type CompareRequest struct {
	// DN is the distinguished name of the entry to compare
	DN string
	// Attribute is the attribute type to compare
	Attribute string
	// Value is the assertion value to compare against
	Value []byte
}

// Errors for CompareRequest parsing
var (
	// ErrEmptyCompareDN is returned when the DN to compare is empty
	ErrEmptyCompareDN = errors.New("ldap: compare DN cannot be empty")
	// ErrEmptyCompareAttribute is returned when the attribute to compare is empty
	ErrEmptyCompareAttribute = errors.New("ldap: compare attribute cannot be empty")
)

// ParseCompareRequest parses a CompareRequest from raw operation data.
// The data should be the contents of the APPLICATION 14 tag (without the tag and length).
func ParseCompareRequest(data []byte) (*CompareRequest, error) {
	if len(data) == 0 {
		return nil, NewParseError(0, "empty compare request data", nil)
	}

	decoder := ber.NewBERDecoder(data)
	req := &CompareRequest{}

	// Read entry DN (LDAPDN - OCTET STRING)
	dnBytes, err := decoder.ReadOctetString()
	if err != nil {
		return nil, NewParseError(decoder.Offset(), "failed to read entry DN", err)
	}
	req.DN = string(dnBytes)

	// Read AttributeValueAssertion (SEQUENCE)
	avaDecoder, err := decoder.ReadSequenceContents()
	if err != nil {
		return nil, NewParseError(decoder.Offset(), "failed to read AttributeValueAssertion", err)
	}

	// Read attributeDesc (AttributeDescription - OCTET STRING)
	attrBytes, err := avaDecoder.ReadOctetString()
	if err != nil {
		return nil, NewParseError(decoder.Offset(), "failed to read attribute description", err)
	}
	req.Attribute = string(attrBytes)

	// Read assertionValue (AssertionValue - OCTET STRING)
	valueBytes, err := avaDecoder.ReadOctetString()
	if err != nil {
		return nil, NewParseError(decoder.Offset(), "failed to read assertion value", err)
	}
	req.Value = valueBytes

	return req, nil
}

// Encode encodes the CompareRequest to BER format (without the APPLICATION tag).
func (r *CompareRequest) Encode() ([]byte, error) {
	encoder := ber.NewBEREncoder(128)

	// Write entry DN (OCTET STRING)
	if err := encoder.WriteOctetString([]byte(r.DN)); err != nil {
		return nil, err
	}

	// Write AttributeValueAssertion (SEQUENCE)
	avaPos := encoder.BeginSequence()

	// Write attributeDesc (OCTET STRING)
	if err := encoder.WriteOctetString([]byte(r.Attribute)); err != nil {
		return nil, err
	}

	// Write assertionValue (OCTET STRING)
	if err := encoder.WriteOctetString(r.Value); err != nil {
		return nil, err
	}

	if err := encoder.EndSequence(avaPos); err != nil {
		return nil, err
	}

	return encoder.Bytes(), nil
}

// Validate validates the CompareRequest.
func (r *CompareRequest) Validate() error {
	if r.DN == "" {
		return ErrEmptyCompareDN
	}
	if r.Attribute == "" {
		return ErrEmptyCompareAttribute
	}
	return nil
}
