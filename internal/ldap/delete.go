// Package ldap implements LDAP protocol message parsing and encoding
// as specified in RFC 4511.
package ldap

import (
	"errors"

	"github.com/oba-ldap/oba/internal/ber"
)

// DeleteRequest represents an LDAP Delete Request
// DelRequest ::= [APPLICATION 10] LDAPDN
// Note: DelRequest is a primitive type (just an LDAPDN), not a SEQUENCE
type DeleteRequest struct {
	// DN is the distinguished name of the entry to delete
	DN string
}

// Errors for DeleteRequest parsing
var (
	// ErrEmptyDeleteDN is returned when the DN to delete is empty
	ErrEmptyDeleteDN = errors.New("ldap: delete DN cannot be empty")
)

// ParseDeleteRequest parses a DeleteRequest from raw operation data.
// The data should be the contents of the APPLICATION 10 tag (without the tag and length).
// Note: DelRequest is just an LDAPDN (OCTET STRING), not a SEQUENCE.
func ParseDeleteRequest(data []byte) (*DeleteRequest, error) {
	// DelRequest is just the DN bytes directly (no SEQUENCE wrapper)
	// The APPLICATION 10 tag wraps the DN directly
	req := &DeleteRequest{
		DN: string(data),
	}

	return req, nil
}

// Encode encodes the DeleteRequest to BER format (without the APPLICATION tag).
// Returns just the DN bytes since DelRequest is a primitive LDAPDN.
func (r *DeleteRequest) Encode() ([]byte, error) {
	return []byte(r.DN), nil
}

// Validate validates the DeleteRequest.
func (r *DeleteRequest) Validate() error {
	if r.DN == "" {
		return ErrEmptyDeleteDN
	}
	return nil
}

// UnbindRequest represents an LDAP Unbind Request
// UnbindRequest ::= [APPLICATION 2] NULL
// Note: UnbindRequest has no content (NULL type)
type UnbindRequest struct{}

// ParseUnbindRequest parses an UnbindRequest from raw operation data.
// The data should be empty since UnbindRequest is NULL.
func ParseUnbindRequest(data []byte) (*UnbindRequest, error) {
	// UnbindRequest is NULL, so data should be empty
	// We accept any data (including empty) for robustness
	return &UnbindRequest{}, nil
}

// Encode encodes the UnbindRequest to BER format (without the APPLICATION tag).
// Returns empty bytes since UnbindRequest is NULL.
func (r *UnbindRequest) Encode() ([]byte, error) {
	return []byte{}, nil
}

// AbandonRequest represents an LDAP Abandon Request
// AbandonRequest ::= [APPLICATION 16] MessageID
// Note: AbandonRequest is a primitive INTEGER (MessageID)
type AbandonRequest struct {
	// MessageID is the ID of the message to abandon
	MessageID int
}

// ParseAbandonRequest parses an AbandonRequest from raw operation data.
// The data should be the contents of the APPLICATION 16 tag (without the tag and length).
// Note: AbandonRequest is just a MessageID (INTEGER), not a SEQUENCE.
func ParseAbandonRequest(data []byte) (*AbandonRequest, error) {
	if len(data) == 0 {
		return nil, NewParseError(0, "empty abandon request data", nil)
	}

	// The data is the raw integer bytes (without INTEGER tag)
	// We need to decode it as a two's complement integer
	var msgID int64

	// Read first byte to determine sign
	firstByte := data[0]
	if firstByte&0x80 != 0 {
		// Negative number (shouldn't happen for MessageID, but handle it)
		msgID = -1
	}

	// Shift in all bytes
	for _, b := range data {
		msgID = (msgID << 8) | int64(b)
	}

	return &AbandonRequest{
		MessageID: int(msgID),
	}, nil
}

// Encode encodes the AbandonRequest to BER format (without the APPLICATION tag).
// Returns the MessageID as raw integer bytes.
func (r *AbandonRequest) Encode() ([]byte, error) {
	// Encode the message ID as a minimal two's complement integer
	encoder := ber.NewBEREncoder(8)
	if err := encoder.WriteInteger(int64(r.MessageID)); err != nil {
		return nil, err
	}

	// The encoder writes the full INTEGER TLV, but we only need the value bytes
	// For APPLICATION 16, we need just the raw integer bytes
	encoded := encoder.Bytes()
	// Skip tag (1 byte) and length (1 byte for small integers)
	if len(encoded) >= 2 {
		return encoded[2:], nil
	}
	return []byte{0}, nil
}
