// Package ldap implements LDAP protocol message parsing and encoding
// as specified in RFC 4511.
package ldap

import (
	"errors"
	"fmt"
)

// LDAP protocol operation tags (APPLICATION class)
// Per RFC 4511 Section 4.2
const (
	ApplicationBindRequest           = 0  // [APPLICATION 0]
	ApplicationBindResponse          = 1  // [APPLICATION 1]
	ApplicationUnbindRequest         = 2  // [APPLICATION 2]
	ApplicationSearchRequest         = 3  // [APPLICATION 3]
	ApplicationSearchResultEntry     = 4  // [APPLICATION 4]
	ApplicationSearchResultDone      = 5  // [APPLICATION 5]
	ApplicationModifyRequest         = 6  // [APPLICATION 6]
	ApplicationModifyResponse        = 7  // [APPLICATION 7]
	ApplicationAddRequest            = 8  // [APPLICATION 8]
	ApplicationAddResponse           = 9  // [APPLICATION 9]
	ApplicationDelRequest            = 10 // [APPLICATION 10]
	ApplicationDelResponse           = 11 // [APPLICATION 11]
	ApplicationModifyDNRequest       = 12 // [APPLICATION 12]
	ApplicationModifyDNResponse      = 13 // [APPLICATION 13]
	ApplicationCompareRequest        = 14 // [APPLICATION 14]
	ApplicationCompareResponse       = 15 // [APPLICATION 15]
	ApplicationAbandonRequest        = 16 // [APPLICATION 16]
	ApplicationSearchResultReference = 19 // [APPLICATION 19]
	ApplicationExtendedRequest       = 23 // [APPLICATION 23]
	ApplicationExtendedResponse      = 24 // [APPLICATION 24]
	ApplicationIntermediateResponse  = 25 // [APPLICATION 25]
)

// OperationType represents the type of LDAP operation
type OperationType int

// String returns the string representation of the operation type
func (o OperationType) String() string {
	switch o {
	case ApplicationBindRequest:
		return "BindRequest"
	case ApplicationBindResponse:
		return "BindResponse"
	case ApplicationUnbindRequest:
		return "UnbindRequest"
	case ApplicationSearchRequest:
		return "SearchRequest"
	case ApplicationSearchResultEntry:
		return "SearchResultEntry"
	case ApplicationSearchResultDone:
		return "SearchResultDone"
	case ApplicationModifyRequest:
		return "ModifyRequest"
	case ApplicationModifyResponse:
		return "ModifyResponse"
	case ApplicationAddRequest:
		return "AddRequest"
	case ApplicationAddResponse:
		return "AddResponse"
	case ApplicationDelRequest:
		return "DelRequest"
	case ApplicationDelResponse:
		return "DelResponse"
	case ApplicationModifyDNRequest:
		return "ModifyDNRequest"
	case ApplicationModifyDNResponse:
		return "ModifyDNResponse"
	case ApplicationCompareRequest:
		return "CompareRequest"
	case ApplicationCompareResponse:
		return "CompareResponse"
	case ApplicationAbandonRequest:
		return "AbandonRequest"
	case ApplicationSearchResultReference:
		return "SearchResultReference"
	case ApplicationExtendedRequest:
		return "ExtendedRequest"
	case ApplicationExtendedResponse:
		return "ExtendedResponse"
	case ApplicationIntermediateResponse:
		return "IntermediateResponse"
	default:
		return fmt.Sprintf("Unknown(%d)", o)
	}
}

// Context-specific tags for Controls
const (
	ContextTagControls = 0 // [0] Controls OPTIONAL
)

// LDAP Result Codes per RFC 4511 Section 4.1.9
type ResultCode int

const (
	ResultSuccess                      ResultCode = 0
	ResultOperationsError              ResultCode = 1
	ResultProtocolError                ResultCode = 2
	ResultTimeLimitExceeded            ResultCode = 3
	ResultSizeLimitExceeded            ResultCode = 4
	ResultCompareFalse                 ResultCode = 5
	ResultCompareTrue                  ResultCode = 6
	ResultAuthMethodNotSupported       ResultCode = 7
	ResultStrongerAuthRequired         ResultCode = 8
	ResultReferral                     ResultCode = 10
	ResultAdminLimitExceeded           ResultCode = 11
	ResultUnavailableCriticalExtension ResultCode = 12
	ResultConfidentialityRequired      ResultCode = 13
	ResultSaslBindInProgress           ResultCode = 14
	ResultNoSuchAttribute              ResultCode = 16
	ResultUndefinedAttributeType       ResultCode = 17
	ResultInappropriateMatching        ResultCode = 18
	ResultConstraintViolation          ResultCode = 19
	ResultAttributeOrValueExists       ResultCode = 20
	ResultInvalidAttributeSyntax       ResultCode = 21
	ResultNoSuchObject                 ResultCode = 32
	ResultAliasProblem                 ResultCode = 33
	ResultInvalidDNSyntax              ResultCode = 34
	ResultAliasDereferencingProblem    ResultCode = 36
	ResultInappropriateAuthentication  ResultCode = 48
	ResultInvalidCredentials           ResultCode = 49
	ResultInsufficientAccessRights     ResultCode = 50
	ResultBusy                         ResultCode = 51
	ResultUnavailable                  ResultCode = 52
	ResultUnwillingToPerform           ResultCode = 53
	ResultLoopDetect                   ResultCode = 54
	ResultNamingViolation              ResultCode = 64
	ResultObjectClassViolation         ResultCode = 65
	ResultNotAllowedOnNonLeaf          ResultCode = 66
	ResultNotAllowedOnRDN              ResultCode = 67
	ResultEntryAlreadyExists           ResultCode = 68
	ResultObjectClassModsProhibited    ResultCode = 69
	ResultAffectsMultipleDSAs          ResultCode = 71
	ResultOther                        ResultCode = 80
)

// String returns the string representation of the result code
func (r ResultCode) String() string {
	switch r {
	case ResultSuccess:
		return "Success"
	case ResultOperationsError:
		return "OperationsError"
	case ResultProtocolError:
		return "ProtocolError"
	case ResultTimeLimitExceeded:
		return "TimeLimitExceeded"
	case ResultSizeLimitExceeded:
		return "SizeLimitExceeded"
	case ResultCompareFalse:
		return "CompareFalse"
	case ResultCompareTrue:
		return "CompareTrue"
	case ResultAuthMethodNotSupported:
		return "AuthMethodNotSupported"
	case ResultStrongerAuthRequired:
		return "StrongerAuthRequired"
	case ResultReferral:
		return "Referral"
	case ResultAdminLimitExceeded:
		return "AdminLimitExceeded"
	case ResultUnavailableCriticalExtension:
		return "UnavailableCriticalExtension"
	case ResultConfidentialityRequired:
		return "ConfidentialityRequired"
	case ResultSaslBindInProgress:
		return "SaslBindInProgress"
	case ResultNoSuchAttribute:
		return "NoSuchAttribute"
	case ResultUndefinedAttributeType:
		return "UndefinedAttributeType"
	case ResultInappropriateMatching:
		return "InappropriateMatching"
	case ResultConstraintViolation:
		return "ConstraintViolation"
	case ResultAttributeOrValueExists:
		return "AttributeOrValueExists"
	case ResultInvalidAttributeSyntax:
		return "InvalidAttributeSyntax"
	case ResultNoSuchObject:
		return "NoSuchObject"
	case ResultAliasProblem:
		return "AliasProblem"
	case ResultInvalidDNSyntax:
		return "InvalidDNSyntax"
	case ResultAliasDereferencingProblem:
		return "AliasDereferencingProblem"
	case ResultInappropriateAuthentication:
		return "InappropriateAuthentication"
	case ResultInvalidCredentials:
		return "InvalidCredentials"
	case ResultInsufficientAccessRights:
		return "InsufficientAccessRights"
	case ResultBusy:
		return "Busy"
	case ResultUnavailable:
		return "Unavailable"
	case ResultUnwillingToPerform:
		return "UnwillingToPerform"
	case ResultLoopDetect:
		return "LoopDetect"
	case ResultNamingViolation:
		return "NamingViolation"
	case ResultObjectClassViolation:
		return "ObjectClassViolation"
	case ResultNotAllowedOnNonLeaf:
		return "NotAllowedOnNonLeaf"
	case ResultNotAllowedOnRDN:
		return "NotAllowedOnRDN"
	case ResultEntryAlreadyExists:
		return "EntryAlreadyExists"
	case ResultObjectClassModsProhibited:
		return "ObjectClassModsProhibited"
	case ResultAffectsMultipleDSAs:
		return "AffectsMultipleDSAs"
	case ResultOther:
		return "Other"
	default:
		return fmt.Sprintf("Unknown(%d)", r)
	}
}

// MaxMessageID is the maximum valid message ID per RFC 4511
// MessageID ::= INTEGER (0 .. maxInt)
// maxInt INTEGER ::= 2147483647 -- (2^^31 - 1)
const MaxMessageID = 2147483647

// MinMessageID is the minimum valid message ID
const MinMessageID = 0

// Control represents an LDAP control as defined in RFC 4511 Section 4.1.11
// Control ::= SEQUENCE {
//
//	controlType             LDAPOID,
//	criticality             BOOLEAN DEFAULT FALSE,
//	controlValue            OCTET STRING OPTIONAL
//
// }
type Control struct {
	// OID is the control type OID
	OID string
	// Criticality indicates whether the control is critical
	Criticality bool
	// Value is the optional control value
	Value []byte
}

// RawOperation holds the raw bytes and tag of an unparsed LDAP operation.
// This allows the message envelope to be parsed without fully parsing
// the operation contents.
type RawOperation struct {
	// Tag is the APPLICATION tag number identifying the operation type
	Tag int
	// Data contains the raw BER-encoded operation data (without tag and length)
	Data []byte
}

// LDAPMessage represents an LDAP protocol message envelope.
// Per RFC 4511 Section 4.1.1:
// LDAPMessage ::= SEQUENCE {
//
//	messageID       MessageID,
//	protocolOp      CHOICE { ... },
//	controls        [0] Controls OPTIONAL
//
// }
type LDAPMessage struct {
	// MessageID uniquely identifies the message within a connection
	MessageID int
	// Operation holds the raw protocol operation
	Operation *RawOperation
	// Controls contains optional message controls
	Controls []Control
}

// OperationType returns the type of operation in this message
func (m *LDAPMessage) OperationType() OperationType {
	if m.Operation == nil {
		return -1
	}
	return OperationType(m.Operation.Tag)
}

// Errors for LDAP message parsing
var (
	// ErrInvalidMessageID is returned when the message ID is out of valid range
	ErrInvalidMessageID = errors.New("ldap: message ID out of valid range (0 to 2147483647)")

	// ErrMissingOperation is returned when the protocol operation is missing
	ErrMissingOperation = errors.New("ldap: missing protocol operation")

	// ErrInvalidOperation is returned when the protocol operation has invalid tag class
	ErrInvalidOperation = errors.New("ldap: protocol operation must have APPLICATION tag class")

	// ErrInvalidControlSequence is returned when controls are malformed
	ErrInvalidControlSequence = errors.New("ldap: invalid control sequence")

	// ErrInvalidControlOID is returned when a control OID is invalid
	ErrInvalidControlOID = errors.New("ldap: invalid control OID")

	// ErrEmptyMessage is returned when trying to parse empty data
	ErrEmptyMessage = errors.New("ldap: empty message data")
)

// ParseError provides detailed information about a parsing failure
type ParseError struct {
	Offset  int
	Message string
	Err     error
}

// Error implements the error interface
func (e *ParseError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("ldap: parse error at offset %d: %s: %v", e.Offset, e.Message, e.Err)
	}
	return fmt.Sprintf("ldap: parse error at offset %d: %s", e.Offset, e.Message)
}

// Unwrap returns the underlying error
func (e *ParseError) Unwrap() error {
	return e.Err
}

// NewParseError creates a new ParseError
func NewParseError(offset int, message string, err error) *ParseError {
	return &ParseError{
		Offset:  offset,
		Message: message,
		Err:     err,
	}
}
