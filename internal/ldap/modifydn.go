// Package ldap implements LDAP protocol message parsing and encoding
// as specified in RFC 4511.
package ldap

import (
	"errors"

	"github.com/KilimcininKorOglu/oba/internal/ber"
)

// ModifyDNRequest represents an LDAP ModifyDN Request
// ModifyDNRequest ::= [APPLICATION 12] SEQUENCE {
//
//	entry           LDAPDN,
//	newrdn          RelativeLDAPDN,
//	deleteoldrdn    BOOLEAN,
//	newSuperior     [0] LDAPDN OPTIONAL
//
// }
type ModifyDNRequest struct {
	// Entry is the DN of the entry to rename/move
	Entry string
	// NewRDN is the new relative distinguished name
	NewRDN string
	// DeleteOldRDN indicates whether to delete the old RDN attribute values
	DeleteOldRDN bool
	// NewSuperior is the optional new parent DN (for moving entries)
	NewSuperior string
}

// Errors for ModifyDNRequest parsing
var (
	// ErrEmptyModifyDNEntry is returned when the entry DN is empty
	ErrEmptyModifyDNEntry = errors.New("ldap: modifydn entry DN cannot be empty")
	// ErrEmptyNewRDN is returned when the new RDN is empty
	ErrEmptyNewRDN = errors.New("ldap: modifydn new RDN cannot be empty")
)

// ParseModifyDNRequest parses a ModifyDNRequest from raw operation data.
// The data should be the contents of the APPLICATION 12 tag (without the tag and length).
func ParseModifyDNRequest(data []byte) (*ModifyDNRequest, error) {
	if len(data) == 0 {
		return nil, NewParseError(0, "empty modifydn request data", nil)
	}

	decoder := ber.NewBERDecoder(data)
	req := &ModifyDNRequest{}

	// Read entry DN (LDAPDN - OCTET STRING)
	entryBytes, err := decoder.ReadOctetString()
	if err != nil {
		return nil, NewParseError(decoder.Offset(), "failed to read entry DN", err)
	}
	req.Entry = string(entryBytes)

	// Read newrdn (RelativeLDAPDN - OCTET STRING)
	newRDNBytes, err := decoder.ReadOctetString()
	if err != nil {
		return nil, NewParseError(decoder.Offset(), "failed to read new RDN", err)
	}
	req.NewRDN = string(newRDNBytes)

	// Read deleteoldrdn (BOOLEAN)
	deleteOldRDN, err := decoder.ReadBoolean()
	if err != nil {
		return nil, NewParseError(decoder.Offset(), "failed to read deleteoldrdn", err)
	}
	req.DeleteOldRDN = deleteOldRDN

	// Check for optional newSuperior [0] LDAPDN
	if decoder.Remaining() > 0 {
		// Check if next element is context tag [0]
		if decoder.IsContextTag(0) {
			// Read the context-tagged value
			tagNum, _, value, err := decoder.ReadTaggedValue()
			if err != nil {
				return nil, NewParseError(decoder.Offset(), "failed to read newSuperior", err)
			}
			if tagNum == 0 {
				req.NewSuperior = string(value)
			}
		}
	}

	return req, nil
}

// Encode encodes the ModifyDNRequest to BER format (without the APPLICATION tag).
func (r *ModifyDNRequest) Encode() ([]byte, error) {
	encoder := ber.NewBEREncoder(256)

	// Write entry DN (OCTET STRING)
	if err := encoder.WriteOctetString([]byte(r.Entry)); err != nil {
		return nil, err
	}

	// Write newrdn (OCTET STRING)
	if err := encoder.WriteOctetString([]byte(r.NewRDN)); err != nil {
		return nil, err
	}

	// Write deleteoldrdn (BOOLEAN)
	if err := encoder.WriteBoolean(r.DeleteOldRDN); err != nil {
		return nil, err
	}

	// Write newSuperior if present (context tag [0])
	if r.NewSuperior != "" {
		ctxPos := encoder.WriteContextTag(0, false)
		encoder.WriteRaw([]byte(r.NewSuperior))
		if err := encoder.EndContextTag(ctxPos); err != nil {
			return nil, err
		}
	}

	return encoder.Bytes(), nil
}

// Validate validates the ModifyDNRequest.
func (r *ModifyDNRequest) Validate() error {
	if r.Entry == "" {
		return ErrEmptyModifyDNEntry
	}
	if r.NewRDN == "" {
		return ErrEmptyNewRDN
	}
	return nil
}

// HasNewSuperior returns true if a new superior (parent) DN is specified.
func (r *ModifyDNRequest) HasNewSuperior() bool {
	return r.NewSuperior != ""
}
