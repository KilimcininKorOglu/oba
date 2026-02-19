// Package ldap implements LDAP protocol message parsing and encoding
// as specified in RFC 4511.
package ldap

import (
	"errors"

	"github.com/oba-ldap/oba/internal/ber"
)

// ModifyOperation represents the type of modification operation
type ModifyOperation int

const (
	// ModifyOperationAdd adds values to an attribute
	ModifyOperationAdd ModifyOperation = 0
	// ModifyOperationDelete deletes values from an attribute
	ModifyOperationDelete ModifyOperation = 1
	// ModifyOperationReplace replaces all values of an attribute
	ModifyOperationReplace ModifyOperation = 2
)

// String returns the string representation of the modify operation
func (m ModifyOperation) String() string {
	switch m {
	case ModifyOperationAdd:
		return "Add"
	case ModifyOperationDelete:
		return "Delete"
	case ModifyOperationReplace:
		return "Replace"
	default:
		return "Unknown"
	}
}

// Modification represents a single modification in a ModifyRequest
// Change ::= SEQUENCE {
//
//	operation       ENUMERATED { add(0), delete(1), replace(2) },
//	modification    PartialAttribute
//
// }
type Modification struct {
	// Operation is the type of modification
	Operation ModifyOperation
	// Attribute contains the attribute type and values for the modification
	Attribute Attribute
}

// ModifyRequest represents an LDAP Modify Request
// ModifyRequest ::= [APPLICATION 6] SEQUENCE {
//
//	object          LDAPDN,
//	changes         SEQUENCE OF change SEQUENCE {
//	                    operation       ENUMERATED { add(0), delete(1), replace(2) },
//	                    modification    PartialAttribute
//	                }
//
// }
type ModifyRequest struct {
	// Object is the DN of the entry to modify
	Object string
	// Changes contains the list of modifications to apply
	Changes []Modification
}

// Errors for ModifyRequest parsing
var (
	// ErrEmptyModifyObject is returned when the object DN is empty
	ErrEmptyModifyObject = errors.New("ldap: modify object DN cannot be empty")
	// ErrInvalidModifyOperation is returned when the modify operation is invalid
	ErrInvalidModifyOperation = errors.New("ldap: invalid modify operation")
	// ErrEmptyModifications is returned when there are no modifications
	ErrEmptyModifications = errors.New("ldap: modify request must have at least one modification")
)

// ParseModifyRequest parses a ModifyRequest from raw operation data.
// The data should be the contents of the APPLICATION 6 tag (without the tag and length).
func ParseModifyRequest(data []byte) (*ModifyRequest, error) {
	if len(data) == 0 {
		return nil, NewParseError(0, "empty modify request data", nil)
	}

	decoder := ber.NewBERDecoder(data)
	req := &ModifyRequest{}

	// Read object DN (LDAPDN - OCTET STRING)
	objectBytes, err := decoder.ReadOctetString()
	if err != nil {
		return nil, NewParseError(decoder.Offset(), "failed to read object DN", err)
	}
	req.Object = string(objectBytes)

	// Read changes (SEQUENCE OF Change)
	changesLen, err := decoder.ExpectSequence()
	if err != nil {
		return nil, NewParseError(decoder.Offset(), "failed to read changes sequence", err)
	}

	changesEnd := decoder.Offset() + changesLen
	var changes []Modification

	for decoder.Offset() < changesEnd && decoder.Remaining() > 0 {
		change, err := parseModification(decoder)
		if err != nil {
			return nil, err
		}
		changes = append(changes, change)
	}

	req.Changes = changes
	return req, nil
}

// parseModification parses a single modification from the decoder
// Change ::= SEQUENCE {
//
//	operation       ENUMERATED { add(0), delete(1), replace(2) },
//	modification    PartialAttribute
//
// }
func parseModification(decoder *ber.BERDecoder) (Modification, error) {
	mod := Modification{}

	// Read the change SEQUENCE
	changeDecoder, err := decoder.ReadSequenceContents()
	if err != nil {
		return mod, NewParseError(decoder.Offset(), "failed to read change sequence", err)
	}

	// Read operation (ENUMERATED)
	operation, err := changeDecoder.ReadEnumerated()
	if err != nil {
		return mod, NewParseError(decoder.Offset(), "failed to read operation", err)
	}

	if operation < 0 || operation > 2 {
		return mod, ErrInvalidModifyOperation
	}
	mod.Operation = ModifyOperation(operation)

	// Read modification (PartialAttribute)
	attr, err := parsePartialAttribute(changeDecoder)
	if err != nil {
		return mod, err
	}
	mod.Attribute = attr

	return mod, nil
}

// parsePartialAttribute parses a partial attribute from the decoder
// PartialAttribute ::= SEQUENCE {
//
//	type       AttributeDescription,
//	vals       SET OF value AttributeValue
//
// }
func parsePartialAttribute(decoder *ber.BERDecoder) (Attribute, error) {
	attr := Attribute{}

	// Read the attribute SEQUENCE
	attrDecoder, err := decoder.ReadSequenceContents()
	if err != nil {
		return attr, NewParseError(decoder.Offset(), "failed to read partial attribute sequence", err)
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

// Encode encodes the ModifyRequest to BER format (without the APPLICATION tag).
func (r *ModifyRequest) Encode() ([]byte, error) {
	encoder := ber.NewBEREncoder(256)

	// Write object DN (OCTET STRING)
	if err := encoder.WriteOctetString([]byte(r.Object)); err != nil {
		return nil, err
	}

	// Write changes (SEQUENCE OF Change)
	changesPos := encoder.BeginSequence()

	for _, change := range r.Changes {
		if err := encodeModification(encoder, change); err != nil {
			return nil, err
		}
	}

	if err := encoder.EndSequence(changesPos); err != nil {
		return nil, err
	}

	return encoder.Bytes(), nil
}

// encodeModification encodes a single modification
func encodeModification(encoder *ber.BEREncoder, mod Modification) error {
	// Start change SEQUENCE
	changePos := encoder.BeginSequence()

	// Write operation (ENUMERATED)
	if err := encoder.WriteEnumerated(int64(mod.Operation)); err != nil {
		return err
	}

	// Write modification (PartialAttribute)
	if err := encodeAttribute(encoder, mod.Attribute); err != nil {
		return err
	}

	return encoder.EndSequence(changePos)
}

// Validate validates the ModifyRequest.
func (r *ModifyRequest) Validate() error {
	if r.Object == "" {
		return ErrEmptyModifyObject
	}
	if len(r.Changes) == 0 {
		return ErrEmptyModifications
	}
	for _, change := range r.Changes {
		if change.Operation < 0 || change.Operation > 2 {
			return ErrInvalidModifyOperation
		}
	}
	return nil
}

// AddModification adds a modification to the request.
func (r *ModifyRequest) AddModification(op ModifyOperation, attrType string, values ...[]byte) {
	r.Changes = append(r.Changes, Modification{
		Operation: op,
		Attribute: Attribute{
			Type:   attrType,
			Values: values,
		},
	})
}

// AddStringModification adds a modification with string values to the request.
func (r *ModifyRequest) AddStringModification(op ModifyOperation, attrType string, values ...string) {
	byteValues := make([][]byte, len(values))
	for i, v := range values {
		byteValues[i] = []byte(v)
	}
	r.AddModification(op, attrType, byteValues...)
}
