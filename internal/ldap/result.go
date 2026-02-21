// Package ldap implements LDAP protocol message parsing and encoding
// as specified in RFC 4511.
package ldap

import (
	"github.com/KilimcininKorOglu/oba/internal/ber"
)

// Context-specific tags for response fields
const (
	// ContextTagReferral is the tag for referral URIs in LDAPResult [3]
	ContextTagReferral = 3
	// ContextTagServerSASLCreds is the tag for server SASL credentials in BindResponse [7]
	ContextTagServerSASLCreds = 7
)

// LDAPResult represents the common result structure used in most LDAP responses.
// Per RFC 4511 Section 4.1.9:
// LDAPResult ::= SEQUENCE {
//
//	resultCode         ENUMERATED { ... },
//	matchedDN          LDAPDN,
//	diagnosticMessage  LDAPString,
//	referral           [3] Referral OPTIONAL
//
// }
type LDAPResult struct {
	// ResultCode indicates the outcome of the operation
	ResultCode ResultCode
	// MatchedDN contains the DN of the last entry matched during processing
	MatchedDN string
	// DiagnosticMessage contains additional diagnostic information
	DiagnosticMessage string
	// Referral contains URIs to other servers (optional)
	Referral []string
}

// Encode encodes the LDAPResult to BER format (without outer tag).
// This is used as part of response encoding.
func (r *LDAPResult) Encode(encoder *ber.BEREncoder) error {
	// Write resultCode (ENUMERATED)
	if err := encoder.WriteEnumerated(int64(r.ResultCode)); err != nil {
		return err
	}

	// Write matchedDN (LDAPDN - OCTET STRING)
	if err := encoder.WriteOctetString([]byte(r.MatchedDN)); err != nil {
		return err
	}

	// Write diagnosticMessage (LDAPString - OCTET STRING)
	if err := encoder.WriteOctetString([]byte(r.DiagnosticMessage)); err != nil {
		return err
	}

	// Write referral [3] if present
	if len(r.Referral) > 0 {
		refPos := encoder.WriteContextTag(ContextTagReferral, true)
		for _, uri := range r.Referral {
			if err := encoder.WriteOctetString([]byte(uri)); err != nil {
				return err
			}
		}
		if err := encoder.EndContextTag(refPos); err != nil {
			return err
		}
	}

	return nil
}

// BindResponse represents an LDAP Bind response.
// Per RFC 4511 Section 4.2.2:
// BindResponse ::= [APPLICATION 1] SEQUENCE {
//
//	COMPONENTS OF LDAPResult,
//	serverSaslCreds    [7] OCTET STRING OPTIONAL
//
// }
type BindResponse struct {
	// LDAPResult contains the common result fields
	LDAPResult
	// ServerSASLCreds contains server SASL credentials (optional)
	ServerSASLCreds []byte
}

// Encode encodes the BindResponse to BER format.
func (r *BindResponse) Encode() ([]byte, error) {
	encoder := ber.NewBEREncoder(128)

	// Write APPLICATION 1 tag
	appPos := encoder.WriteApplicationTag(ApplicationBindResponse, true)

	// Write LDAPResult components
	if err := r.LDAPResult.Encode(encoder); err != nil {
		return nil, err
	}

	// Write serverSaslCreds [7] if present
	if len(r.ServerSASLCreds) > 0 {
		if err := encoder.WriteTaggedValue(ContextTagServerSASLCreds, false, r.ServerSASLCreds); err != nil {
			return nil, err
		}
	}

	if err := encoder.EndApplicationTag(appPos); err != nil {
		return nil, err
	}

	return encoder.Bytes(), nil
}

// PartialAttribute represents an attribute with its values.
// Per RFC 4511 Section 4.1.7:
// PartialAttribute ::= SEQUENCE {
//
//	type       AttributeDescription,
//	vals       SET OF value AttributeValue
//
// }
type PartialAttribute struct {
	// Type is the attribute description (name or OID)
	Type string
	// Values contains the attribute values
	Values [][]byte
}

// SearchResultEntry represents a search result entry.
// Per RFC 4511 Section 4.5.2:
// SearchResultEntry ::= [APPLICATION 4] SEQUENCE {
//
//	objectName      LDAPDN,
//	attributes      PartialAttributeList
//
// }
// PartialAttributeList ::= SEQUENCE OF partialAttribute PartialAttribute
type SearchResultEntry struct {
	// ObjectName is the DN of the entry
	ObjectName string
	// Attributes contains the entry's attributes
	Attributes []PartialAttribute
}

// Encode encodes the SearchResultEntry to BER format.
func (r *SearchResultEntry) Encode() ([]byte, error) {
	encoder := ber.NewBEREncoder(256)

	// Write APPLICATION 4 tag
	appPos := encoder.WriteApplicationTag(ApplicationSearchResultEntry, true)

	// Write objectName (LDAPDN - OCTET STRING)
	if err := encoder.WriteOctetString([]byte(r.ObjectName)); err != nil {
		return nil, err
	}

	// Write attributes (SEQUENCE OF PartialAttribute)
	attrSeqPos := encoder.BeginSequence()
	for _, attr := range r.Attributes {
		// Each PartialAttribute is a SEQUENCE
		partialAttrPos := encoder.BeginSequence()

		// Write type (AttributeDescription - OCTET STRING)
		if err := encoder.WriteOctetString([]byte(attr.Type)); err != nil {
			return nil, err
		}

		// Write vals (SET OF AttributeValue)
		valsPos := encoder.BeginSet()
		for _, val := range attr.Values {
			if err := encoder.WriteOctetString(val); err != nil {
				return nil, err
			}
		}
		if err := encoder.EndSet(valsPos); err != nil {
			return nil, err
		}

		if err := encoder.EndSequence(partialAttrPos); err != nil {
			return nil, err
		}
	}
	if err := encoder.EndSequence(attrSeqPos); err != nil {
		return nil, err
	}

	if err := encoder.EndApplicationTag(appPos); err != nil {
		return nil, err
	}

	return encoder.Bytes(), nil
}

// SearchResultDone represents the final response to a search operation.
// Per RFC 4511 Section 4.5.2:
// SearchResultDone ::= [APPLICATION 5] LDAPResult
type SearchResultDone struct {
	LDAPResult
}

// Encode encodes the SearchResultDone to BER format.
func (r *SearchResultDone) Encode() ([]byte, error) {
	encoder := ber.NewBEREncoder(64)

	// Write APPLICATION 5 tag
	appPos := encoder.WriteApplicationTag(ApplicationSearchResultDone, true)

	// Write LDAPResult components
	if err := r.LDAPResult.Encode(encoder); err != nil {
		return nil, err
	}

	if err := encoder.EndApplicationTag(appPos); err != nil {
		return nil, err
	}

	return encoder.Bytes(), nil
}

// ModifyResponse represents the response to a modify operation.
// Per RFC 4511 Section 4.6:
// ModifyResponse ::= [APPLICATION 7] LDAPResult
type ModifyResponse struct {
	LDAPResult
}

// Encode encodes the ModifyResponse to BER format.
func (r *ModifyResponse) Encode() ([]byte, error) {
	encoder := ber.NewBEREncoder(64)

	// Write APPLICATION 7 tag
	appPos := encoder.WriteApplicationTag(ApplicationModifyResponse, true)

	// Write LDAPResult components
	if err := r.LDAPResult.Encode(encoder); err != nil {
		return nil, err
	}

	if err := encoder.EndApplicationTag(appPos); err != nil {
		return nil, err
	}

	return encoder.Bytes(), nil
}

// AddResponse represents the response to an add operation.
// Per RFC 4511 Section 4.7:
// AddResponse ::= [APPLICATION 9] LDAPResult
type AddResponse struct {
	LDAPResult
}

// Encode encodes the AddResponse to BER format.
func (r *AddResponse) Encode() ([]byte, error) {
	encoder := ber.NewBEREncoder(64)

	// Write APPLICATION 9 tag
	appPos := encoder.WriteApplicationTag(ApplicationAddResponse, true)

	// Write LDAPResult components
	if err := r.LDAPResult.Encode(encoder); err != nil {
		return nil, err
	}

	if err := encoder.EndApplicationTag(appPos); err != nil {
		return nil, err
	}

	return encoder.Bytes(), nil
}

// DeleteResponse represents the response to a delete operation.
// Per RFC 4511 Section 4.8:
// DelResponse ::= [APPLICATION 11] LDAPResult
type DeleteResponse struct {
	LDAPResult
}

// Encode encodes the DeleteResponse to BER format.
func (r *DeleteResponse) Encode() ([]byte, error) {
	encoder := ber.NewBEREncoder(64)

	// Write APPLICATION 11 tag
	appPos := encoder.WriteApplicationTag(ApplicationDelResponse, true)

	// Write LDAPResult components
	if err := r.LDAPResult.Encode(encoder); err != nil {
		return nil, err
	}

	if err := encoder.EndApplicationTag(appPos); err != nil {
		return nil, err
	}

	return encoder.Bytes(), nil
}

// ModifyDNResponse represents the response to a modify DN operation.
// Per RFC 4511 Section 4.9:
// ModifyDNResponse ::= [APPLICATION 13] LDAPResult
type ModifyDNResponse struct {
	LDAPResult
}

// Encode encodes the ModifyDNResponse to BER format.
func (r *ModifyDNResponse) Encode() ([]byte, error) {
	encoder := ber.NewBEREncoder(64)

	// Write APPLICATION 13 tag
	appPos := encoder.WriteApplicationTag(ApplicationModifyDNResponse, true)

	// Write LDAPResult components
	if err := r.LDAPResult.Encode(encoder); err != nil {
		return nil, err
	}

	if err := encoder.EndApplicationTag(appPos); err != nil {
		return nil, err
	}

	return encoder.Bytes(), nil
}

// CompareResponse represents the response to a compare operation.
// Per RFC 4511 Section 4.10:
// CompareResponse ::= [APPLICATION 15] LDAPResult
type CompareResponse struct {
	LDAPResult
}

// Encode encodes the CompareResponse to BER format.
func (r *CompareResponse) Encode() ([]byte, error) {
	encoder := ber.NewBEREncoder(64)

	// Write APPLICATION 15 tag
	appPos := encoder.WriteApplicationTag(ApplicationCompareResponse, true)

	// Write LDAPResult components
	if err := r.LDAPResult.Encode(encoder); err != nil {
		return nil, err
	}

	if err := encoder.EndApplicationTag(appPos); err != nil {
		return nil, err
	}

	return encoder.Bytes(), nil
}

// NewSuccessResult creates a new LDAPResult with success status.
func NewSuccessResult() LDAPResult {
	return LDAPResult{
		ResultCode:        ResultSuccess,
		MatchedDN:         "",
		DiagnosticMessage: "",
	}
}

// NewErrorResult creates a new LDAPResult with the specified error.
func NewErrorResult(code ResultCode, message string) LDAPResult {
	return LDAPResult{
		ResultCode:        code,
		MatchedDN:         "",
		DiagnosticMessage: message,
	}
}

// NewErrorResultWithDN creates a new LDAPResult with error and matched DN.
func NewErrorResultWithDN(code ResultCode, matchedDN, message string) LDAPResult {
	return LDAPResult{
		ResultCode:        code,
		MatchedDN:         matchedDN,
		DiagnosticMessage: message,
	}
}
