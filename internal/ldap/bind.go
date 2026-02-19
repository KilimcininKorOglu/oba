// Package ldap implements LDAP protocol message parsing and encoding
// as specified in RFC 4511.
package ldap

import (
	"errors"

	"github.com/oba-ldap/oba/internal/ber"
)

// Authentication method tags (context-specific)
const (
	// AuthSimple is the tag for simple authentication [0]
	AuthSimple = 0
	// AuthSASL is the tag for SASL authentication [3]
	AuthSASL = 3
)

// AuthMethod represents the authentication method used in a BindRequest
type AuthMethod int

const (
	// AuthMethodSimple indicates simple (password) authentication
	AuthMethodSimple AuthMethod = iota
	// AuthMethodSASL indicates SASL authentication
	AuthMethodSASL
)

// String returns the string representation of the authentication method
func (a AuthMethod) String() string {
	switch a {
	case AuthMethodSimple:
		return "Simple"
	case AuthMethodSASL:
		return "SASL"
	default:
		return "Unknown"
	}
}

// SASLCredentials represents SASL authentication credentials
// SaslCredentials ::= SEQUENCE {
//
//	mechanism               LDAPString,
//	credentials             OCTET STRING OPTIONAL
//
// }
type SASLCredentials struct {
	// Mechanism is the SASL mechanism name (e.g., "PLAIN", "GSSAPI")
	Mechanism string
	// Credentials is the optional SASL credentials
	Credentials []byte
}

// BindRequest represents an LDAP Bind Request
// BindRequest ::= [APPLICATION 0] SEQUENCE {
//
//	version                 INTEGER (1 .. 127),
//	name                    LDAPDN,
//	authentication          AuthenticationChoice
//
// }
// AuthenticationChoice ::= CHOICE {
//
//	simple                  [0] OCTET STRING,
//	sasl                    [3] SaslCredentials
//
// }
type BindRequest struct {
	// Version is the LDAP protocol version (typically 3)
	Version int
	// Name is the DN of the user binding
	Name string
	// AuthMethod indicates the authentication method used
	AuthMethod AuthMethod
	// SimplePassword contains the password for simple authentication
	SimplePassword []byte
	// SASLCredentials contains SASL credentials for SASL authentication
	SASLCredentials *SASLCredentials
}

// Errors for BindRequest parsing
var (
	// ErrInvalidBindVersion is returned when the bind version is out of range
	ErrInvalidBindVersion = errors.New("ldap: bind version must be between 1 and 127")
	// ErrUnknownAuthMethod is returned when the authentication method is unknown
	ErrUnknownAuthMethod = errors.New("ldap: unknown authentication method")
	// ErrInvalidSASLCredentials is returned when SASL credentials are malformed
	ErrInvalidSASLCredentials = errors.New("ldap: invalid SASL credentials")
)

// ParseBindRequest parses a BindRequest from raw operation data.
// The data should be the contents of the APPLICATION 0 tag (without the tag and length).
func ParseBindRequest(data []byte) (*BindRequest, error) {
	if len(data) == 0 {
		return nil, NewParseError(0, "empty bind request data", nil)
	}

	decoder := ber.NewBERDecoder(data)
	req := &BindRequest{}

	// Read version (INTEGER)
	version, err := decoder.ReadInteger()
	if err != nil {
		return nil, NewParseError(decoder.Offset(), "failed to read bind version", err)
	}

	// Validate version range (1..127)
	if version < 1 || version > 127 {
		return nil, ErrInvalidBindVersion
	}
	req.Version = int(version)

	// Read name (LDAPDN - OCTET STRING)
	nameBytes, err := decoder.ReadOctetString()
	if err != nil {
		return nil, NewParseError(decoder.Offset(), "failed to read bind name", err)
	}
	req.Name = string(nameBytes)

	// Read authentication choice (context-specific tag)
	tagNum, constructed, authData, err := decoder.ReadTaggedValue()
	if err != nil {
		return nil, NewParseError(decoder.Offset(), "failed to read authentication", err)
	}

	switch tagNum {
	case AuthSimple:
		// Simple authentication: [0] OCTET STRING
		req.AuthMethod = AuthMethodSimple
		req.SimplePassword = authData

	case AuthSASL:
		// SASL authentication: [3] SEQUENCE { mechanism, credentials OPTIONAL }
		if !constructed {
			return nil, NewParseError(decoder.Offset(), "SASL credentials must be constructed", ErrInvalidSASLCredentials)
		}

		saslDecoder := ber.NewBERDecoder(authData)
		saslCreds := &SASLCredentials{}

		// Read mechanism (LDAPString - OCTET STRING)
		mechBytes, err := saslDecoder.ReadOctetString()
		if err != nil {
			return nil, NewParseError(decoder.Offset(), "failed to read SASL mechanism", err)
		}
		saslCreds.Mechanism = string(mechBytes)

		// Read optional credentials (OCTET STRING)
		if saslDecoder.Remaining() > 0 {
			credBytes, err := saslDecoder.ReadOctetString()
			if err != nil {
				return nil, NewParseError(decoder.Offset(), "failed to read SASL credentials", err)
			}
			saslCreds.Credentials = credBytes
		}

		req.AuthMethod = AuthMethodSASL
		req.SASLCredentials = saslCreds

	default:
		return nil, NewParseError(decoder.Offset(), "unknown authentication method tag", ErrUnknownAuthMethod)
	}

	return req, nil
}

// Encode encodes the BindRequest to BER format (without the APPLICATION tag).
func (r *BindRequest) Encode() ([]byte, error) {
	encoder := ber.NewBEREncoder(128)

	// Write version (INTEGER)
	if err := encoder.WriteInteger(int64(r.Version)); err != nil {
		return nil, err
	}

	// Write name (LDAPDN - OCTET STRING)
	if err := encoder.WriteOctetString([]byte(r.Name)); err != nil {
		return nil, err
	}

	// Write authentication choice
	switch r.AuthMethod {
	case AuthMethodSimple:
		// Simple authentication: [0] OCTET STRING (primitive)
		if err := encoder.WriteTaggedValue(AuthSimple, false, r.SimplePassword); err != nil {
			return nil, err
		}

	case AuthMethodSASL:
		// SASL authentication: [3] SEQUENCE { mechanism, credentials OPTIONAL }
		saslEncoder := ber.NewBEREncoder(64)

		// Write mechanism
		if err := saslEncoder.WriteOctetString([]byte(r.SASLCredentials.Mechanism)); err != nil {
			return nil, err
		}

		// Write optional credentials
		if len(r.SASLCredentials.Credentials) > 0 {
			if err := saslEncoder.WriteOctetString(r.SASLCredentials.Credentials); err != nil {
				return nil, err
			}
		}

		// Write as context tag [3] constructed
		if err := encoder.WriteTaggedValue(AuthSASL, true, saslEncoder.Bytes()); err != nil {
			return nil, err
		}

	default:
		return nil, ErrUnknownAuthMethod
	}

	return encoder.Bytes(), nil
}

// IsAnonymous returns true if this is an anonymous bind request.
// An anonymous bind has an empty name and empty simple password.
func (r *BindRequest) IsAnonymous() bool {
	return r.Name == "" && r.AuthMethod == AuthMethodSimple && len(r.SimplePassword) == 0
}
