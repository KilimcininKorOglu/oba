// Package ldap implements LDAP protocol message parsing and encoding
// as specified in RFC 4511.
package ldap

// ResultCode represents an LDAP result code as defined in RFC 4511 Section 4.1.9.
// resultCode ENUMERATED {
//
//	success                      (0),
//	operationsError              (1),
//	protocolError                (2),
//	timeLimitExceeded            (3),
//	sizeLimitExceeded            (4),
//	compareFalse                 (5),
//	compareTrue                  (6),
//	authMethodNotSupported       (7),
//	strongerAuthRequired         (8),
//	-- 9 reserved --
//	referral                     (10),
//	adminLimitExceeded           (11),
//	unavailableCriticalExtension (12),
//	confidentialityRequired      (13),
//	saslBindInProgress           (14),
//	noSuchAttribute              (16),
//	undefinedAttributeType       (17),
//	inappropriateMatching        (18),
//	constraintViolation          (19),
//	attributeOrValueExists       (20),
//	invalidAttributeSyntax       (21),
//	-- 22-31 unused --
//	noSuchObject                 (32),
//	aliasProblem                 (33),
//	invalidDNSyntax              (34),
//	-- 35 reserved for undefined isLeaf --
//	aliasDereferencingProblem    (36),
//	-- 37-47 unused --
//	inappropriateAuthentication  (48),
//	invalidCredentials           (49),
//	insufficientAccessRights     (50),
//	busy                         (51),
//	unavailable                  (52),
//	unwillingToPerform           (53),
//	loopDetect                   (54),
//	-- 55-63 unused --
//	namingViolation              (64),
//	objectClassViolation         (65),
//	notAllowedOnNonLeaf          (66),
//	notAllowedOnRDN              (67),
//	entryAlreadyExists           (68),
//	objectClassModsProhibited    (69),
//	-- 70 reserved for CLDAP --
//	affectsMultipleDSAs          (71),
//	-- 72-79 unused --
//	other                        (80),
//	...
//
// }
type ResultCode int

// LDAP result codes per RFC 4511 Section 4.1.9
const (
	// ResultSuccess indicates the operation completed successfully.
	ResultSuccess ResultCode = 0

	// ResultOperationsError indicates an error occurred during processing
	// that is not covered by another result code.
	ResultOperationsError ResultCode = 1

	// ResultProtocolError indicates the server received data that is not
	// well-formed or violates the protocol.
	ResultProtocolError ResultCode = 2

	// ResultTimeLimitExceeded indicates the time limit specified by the
	// client was exceeded before the operation could be completed.
	ResultTimeLimitExceeded ResultCode = 3

	// ResultSizeLimitExceeded indicates the size limit specified by the
	// client was exceeded before the operation could be completed.
	ResultSizeLimitExceeded ResultCode = 4

	// ResultCompareFalse indicates the compare operation completed and
	// the assertion was false.
	ResultCompareFalse ResultCode = 5

	// ResultCompareTrue indicates the compare operation completed and
	// the assertion was true.
	ResultCompareTrue ResultCode = 6

	// ResultAuthMethodNotSupported indicates the authentication method
	// or mechanism is not supported.
	ResultAuthMethodNotSupported ResultCode = 7

	// ResultStrongerAuthRequired indicates the server requires the client
	// to authenticate with a stronger mechanism.
	ResultStrongerAuthRequired ResultCode = 8

	// ResultReferral indicates the server is referring the client to
	// another server or set of servers.
	ResultReferral ResultCode = 10

	// ResultAdminLimitExceeded indicates an administrative limit was
	// exceeded.
	ResultAdminLimitExceeded ResultCode = 11

	// ResultUnavailableCriticalExtension indicates a critical control
	// was not recognized or is not supported.
	ResultUnavailableCriticalExtension ResultCode = 12

	// ResultConfidentialityRequired indicates the operation requires
	// confidentiality protection (e.g., TLS).
	ResultConfidentialityRequired ResultCode = 13

	// ResultSASLBindInProgress indicates the server requires the client
	// to send a new bind request with the same SASL mechanism.
	ResultSASLBindInProgress ResultCode = 14

	// ResultNoSuchAttribute indicates the specified attribute does not
	// exist in the entry.
	ResultNoSuchAttribute ResultCode = 16

	// ResultUndefinedAttributeType indicates the specified attribute
	// type is not defined in the schema.
	ResultUndefinedAttributeType ResultCode = 17

	// ResultInappropriateMatching indicates the matching rule is not
	// appropriate for the attribute type.
	ResultInappropriateMatching ResultCode = 18

	// ResultConstraintViolation indicates a constraint defined in the
	// schema was violated.
	ResultConstraintViolation ResultCode = 19

	// ResultAttributeOrValueExists indicates the attribute or value
	// already exists in the entry.
	ResultAttributeOrValueExists ResultCode = 20

	// ResultInvalidAttributeSyntax indicates the attribute value does
	// not conform to the attribute syntax.
	ResultInvalidAttributeSyntax ResultCode = 21

	// ResultNoSuchObject indicates the specified object does not exist
	// in the DIT.
	ResultNoSuchObject ResultCode = 32

	// ResultAliasProblem indicates an alias was encountered in a
	// situation where it is not allowed.
	ResultAliasProblem ResultCode = 33

	// ResultInvalidDNSyntax indicates the DN syntax is invalid.
	ResultInvalidDNSyntax ResultCode = 34

	// ResultAliasDereferencingProblem indicates a problem occurred
	// while dereferencing an alias.
	ResultAliasDereferencingProblem ResultCode = 36

	// ResultInappropriateAuthentication indicates the authentication
	// method is inappropriate for the operation.
	ResultInappropriateAuthentication ResultCode = 48

	// ResultInvalidCredentials indicates the supplied credentials are
	// invalid.
	ResultInvalidCredentials ResultCode = 49

	// ResultInsufficientAccessRights indicates the client does not have
	// sufficient access rights to perform the operation.
	ResultInsufficientAccessRights ResultCode = 50

	// ResultBusy indicates the server is too busy to service the
	// operation.
	ResultBusy ResultCode = 51

	// ResultUnavailable indicates the server is unavailable.
	ResultUnavailable ResultCode = 52

	// ResultUnwillingToPerform indicates the server is unwilling to
	// perform the operation.
	ResultUnwillingToPerform ResultCode = 53

	// ResultLoopDetect indicates a loop was detected.
	ResultLoopDetect ResultCode = 54

	// ResultNamingViolation indicates the operation would violate
	// naming constraints.
	ResultNamingViolation ResultCode = 64

	// ResultObjectClassViolation indicates the operation would violate
	// object class constraints.
	ResultObjectClassViolation ResultCode = 65

	// ResultNotAllowedOnNonLeaf indicates the operation is not allowed
	// on a non-leaf entry.
	ResultNotAllowedOnNonLeaf ResultCode = 66

	// ResultNotAllowedOnRDN indicates the operation is not allowed on
	// the RDN.
	ResultNotAllowedOnRDN ResultCode = 67

	// ResultEntryAlreadyExists indicates the entry already exists.
	ResultEntryAlreadyExists ResultCode = 68

	// ResultObjectClassModsProhibited indicates object class
	// modifications are prohibited.
	ResultObjectClassModsProhibited ResultCode = 69

	// ResultAffectsMultipleDSAs indicates the operation affects
	// multiple DSAs.
	ResultAffectsMultipleDSAs ResultCode = 71

	// ResultOther indicates an error not covered by other result codes.
	ResultOther ResultCode = 80
)

// String returns the string representation of the result code.
func (r ResultCode) String() string {
	switch r {
	case ResultSuccess:
		return "success"
	case ResultOperationsError:
		return "operationsError"
	case ResultProtocolError:
		return "protocolError"
	case ResultTimeLimitExceeded:
		return "timeLimitExceeded"
	case ResultSizeLimitExceeded:
		return "sizeLimitExceeded"
	case ResultCompareFalse:
		return "compareFalse"
	case ResultCompareTrue:
		return "compareTrue"
	case ResultAuthMethodNotSupported:
		return "authMethodNotSupported"
	case ResultStrongerAuthRequired:
		return "strongerAuthRequired"
	case ResultReferral:
		return "referral"
	case ResultAdminLimitExceeded:
		return "adminLimitExceeded"
	case ResultUnavailableCriticalExtension:
		return "unavailableCriticalExtension"
	case ResultConfidentialityRequired:
		return "confidentialityRequired"
	case ResultSASLBindInProgress:
		return "saslBindInProgress"
	case ResultNoSuchAttribute:
		return "noSuchAttribute"
	case ResultUndefinedAttributeType:
		return "undefinedAttributeType"
	case ResultInappropriateMatching:
		return "inappropriateMatching"
	case ResultConstraintViolation:
		return "constraintViolation"
	case ResultAttributeOrValueExists:
		return "attributeOrValueExists"
	case ResultInvalidAttributeSyntax:
		return "invalidAttributeSyntax"
	case ResultNoSuchObject:
		return "noSuchObject"
	case ResultAliasProblem:
		return "aliasProblem"
	case ResultInvalidDNSyntax:
		return "invalidDNSyntax"
	case ResultAliasDereferencingProblem:
		return "aliasDereferencingProblem"
	case ResultInappropriateAuthentication:
		return "inappropriateAuthentication"
	case ResultInvalidCredentials:
		return "invalidCredentials"
	case ResultInsufficientAccessRights:
		return "insufficientAccessRights"
	case ResultBusy:
		return "busy"
	case ResultUnavailable:
		return "unavailable"
	case ResultUnwillingToPerform:
		return "unwillingToPerform"
	case ResultLoopDetect:
		return "loopDetect"
	case ResultNamingViolation:
		return "namingViolation"
	case ResultObjectClassViolation:
		return "objectClassViolation"
	case ResultNotAllowedOnNonLeaf:
		return "notAllowedOnNonLeaf"
	case ResultNotAllowedOnRDN:
		return "notAllowedOnRDN"
	case ResultEntryAlreadyExists:
		return "entryAlreadyExists"
	case ResultObjectClassModsProhibited:
		return "objectClassModsProhibited"
	case ResultAffectsMultipleDSAs:
		return "affectsMultipleDSAs"
	case ResultOther:
		return "other"
	default:
		return "unknown"
	}
}

// IsSuccess returns true if the result code indicates success.
func (r ResultCode) IsSuccess() bool {
	return r == ResultSuccess
}

// IsError returns true if the result code indicates an error.
func (r ResultCode) IsError() bool {
	return r != ResultSuccess && r != ResultCompareFalse && r != ResultCompareTrue && r != ResultReferral && r != ResultSASLBindInProgress
}
