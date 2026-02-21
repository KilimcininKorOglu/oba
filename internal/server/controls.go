// Package server provides the LDAP server implementation.
package server

import (
	"github.com/KilimcininKorOglu/oba/internal/ber"
	"github.com/KilimcininKorOglu/oba/internal/ldap"
)

// Control OIDs as defined in RFC 2696 and other RFCs.
const (
	// PagedResultsOID is the OID for Simple Paged Results Control (RFC 2696).
	PagedResultsOID = "1.2.840.113556.1.4.319"
)

// PagedResultsControl represents the Simple Paged Results Control (RFC 2696).
// This control allows clients to retrieve search results in pages.
//
// realSearchControlValue ::= SEQUENCE {
//
//	size            INTEGER (0..maxInt),
//	                        -- requested page size from client
//	                        -- result set size estimate from server
//	cookie          OCTET STRING
//
// }
type PagedResultsControl struct {
	// Size is the requested page size (from client) or estimated total count (from server).
	Size int32
	// Cookie is an opaque cursor for pagination.
	// Empty cookie indicates the first page request or end of results.
	Cookie []byte
	// Criticality indicates whether the control is critical.
	Criticality bool
}

// ParsePagedResultsControl parses a PagedResultsControl from an LDAP Control.
// Returns nil if the control is not a paged results control.
func ParsePagedResultsControl(ctrl ldap.Control) (*PagedResultsControl, error) {
	if ctrl.OID != PagedResultsOID {
		return nil, nil
	}

	prc := &PagedResultsControl{
		Criticality: ctrl.Criticality,
	}

	// If no value, return with defaults (size=0, empty cookie)
	if len(ctrl.Value) == 0 {
		return prc, nil
	}

	// Parse the control value
	decoder := ber.NewBERDecoder(ctrl.Value)

	// Read the SEQUENCE
	_, err := decoder.ExpectSequence()
	if err != nil {
		return nil, err
	}

	// Read size (INTEGER)
	size, err := decoder.ReadInteger()
	if err != nil {
		return nil, err
	}
	prc.Size = int32(size)

	// Read cookie (OCTET STRING)
	cookie, err := decoder.ReadOctetString()
	if err != nil {
		return nil, err
	}
	prc.Cookie = cookie

	return prc, nil
}

// Encode encodes the PagedResultsControl to BER format for inclusion in a response.
func (p *PagedResultsControl) Encode() ([]byte, error) {
	encoder := ber.NewBEREncoder(64)

	// Start SEQUENCE
	seqPos := encoder.BeginSequence()

	// Write size (INTEGER)
	if err := encoder.WriteInteger(int64(p.Size)); err != nil {
		return nil, err
	}

	// Write cookie (OCTET STRING)
	if err := encoder.WriteOctetString(p.Cookie); err != nil {
		return nil, err
	}

	// End SEQUENCE
	if err := encoder.EndSequence(seqPos); err != nil {
		return nil, err
	}

	return encoder.Bytes(), nil
}

// ToLDAPControl converts the PagedResultsControl to an ldap.Control.
func (p *PagedResultsControl) ToLDAPControl() (ldap.Control, error) {
	value, err := p.Encode()
	if err != nil {
		return ldap.Control{}, err
	}

	return ldap.Control{
		OID:         PagedResultsOID,
		Criticality: p.Criticality,
		Value:       value,
	}, nil
}

// FindPagedResultsControl searches for a PagedResultsControl in a slice of controls.
// Returns nil if not found.
func FindPagedResultsControl(controls []ldap.Control) (*PagedResultsControl, error) {
	for _, ctrl := range controls {
		if ctrl.OID == PagedResultsOID {
			return ParsePagedResultsControl(ctrl)
		}
	}
	return nil, nil
}
