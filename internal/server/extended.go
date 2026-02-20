// Package server provides the LDAP server implementation.
package server

import (
	"github.com/oba-ldap/oba/internal/ber"
	"github.com/oba-ldap/oba/internal/ldap"
)

// ExtendedRequest represents an LDAP Extended Request.
// Per RFC 4511 Section 4.12:
// ExtendedRequest ::= [APPLICATION 23] SEQUENCE {
//
//	requestName      [0] LDAPOID,
//	requestValue     [1] OCTET STRING OPTIONAL
//
// }
type ExtendedRequest struct {
	// OID is the object identifier for the extended operation
	OID string
	// Value is the optional request value
	Value []byte
}

// ExtendedResponse represents an LDAP Extended Response.
// Per RFC 4511 Section 4.12:
// ExtendedResponse ::= [APPLICATION 24] SEQUENCE {
//
//	COMPONENTS OF LDAPResult,
//	responseName     [10] LDAPOID OPTIONAL,
//	responseValue    [11] OCTET STRING OPTIONAL
//
// }
type ExtendedResponse struct {
	// Result contains the LDAP result code and messages
	Result OperationResult
	// OID is the optional response OID
	OID string
	// Value is the optional response value
	Value []byte
}

// ExtendedHandler is a function that handles extended requests.
type ExtendedHandler func(conn *Connection, req *ExtendedRequest) (*ExtendedResponse, error)

// ParseExtendedRequest parses an ExtendedRequest from raw BER data.
func ParseExtendedRequest(data []byte) (*ExtendedRequest, error) {
	if len(data) == 0 {
		return nil, ldap.NewParseError(0, "empty extended request data", nil)
	}

	decoder := ber.NewBERDecoder(data)
	req := &ExtendedRequest{}

	// Read requestName [0] LDAPOID (context-specific tag 0)
	if !decoder.IsContextTag(0) {
		return nil, ldap.NewParseError(decoder.Offset(), "expected context tag [0] for requestName", nil)
	}

	// Use ReadTaggedValue to read the context-tagged OID
	tagNum, _, oidBytes, err := decoder.ReadTaggedValue()
	if err != nil {
		return nil, ldap.NewParseError(decoder.Offset(), "failed to read requestName", err)
	}
	if tagNum != 0 {
		return nil, ldap.NewParseError(decoder.Offset(), "expected context tag [0] for requestName", nil)
	}
	req.OID = string(oidBytes)

	// Read optional requestValue [1] OCTET STRING
	if decoder.Remaining() > 0 && decoder.IsContextTag(1) {
		tagNum, _, valueBytes, err := decoder.ReadTaggedValue()
		if err != nil {
			return nil, ldap.NewParseError(decoder.Offset(), "failed to read requestValue", err)
		}
		if tagNum != 1 {
			return nil, ldap.NewParseError(decoder.Offset(), "expected context tag [1] for requestValue", nil)
		}
		req.Value = valueBytes
	}

	return req, nil
}

// createExtendedResponse creates an ExtendedResponse LDAP message.
// ExtendedResponse ::= [APPLICATION 24] SEQUENCE {
//
//	COMPONENTS OF LDAPResult,
//	responseName     [10] LDAPOID OPTIONAL,
//	responseValue    [11] OCTET STRING OPTIONAL
//
// }
func createExtendedResponse(messageID int, resp *ExtendedResponse) *ldap.LDAPMessage {
	encoder := ber.NewBEREncoder(128)

	// Write resultCode (ENUMERATED)
	if err := encoder.WriteEnumerated(int64(resp.Result.ResultCode)); err != nil {
		return nil
	}

	// Write matchedDN (LDAPDN - OCTET STRING)
	if err := encoder.WriteOctetString([]byte(resp.Result.MatchedDN)); err != nil {
		return nil
	}

	// Write diagnosticMessage (LDAPString - OCTET STRING)
	if err := encoder.WriteOctetString([]byte(resp.Result.DiagnosticMessage)); err != nil {
		return nil
	}

	// Write optional responseName [10] LDAPOID
	if resp.OID != "" {
		if err := encoder.WriteTaggedValue(10, false, []byte(resp.OID)); err != nil {
			return nil
		}
	}

	// Write optional responseValue [11] OCTET STRING
	if len(resp.Value) > 0 {
		if err := encoder.WriteTaggedValue(11, false, resp.Value); err != nil {
			return nil
		}
	}

	return &ldap.LDAPMessage{
		MessageID: messageID,
		Operation: &ldap.RawOperation{
			Tag:  ldap.ApplicationExtendedResponse,
			Data: encoder.Bytes(),
		},
	}
}
