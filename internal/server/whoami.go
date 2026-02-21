// Package server provides the LDAP server implementation.
package server

import (
	"github.com/KilimcininKorOglu/oba/internal/ldap"
)

// WhoAmIOID is the OID for the Who Am I extended operation.
// Per RFC 4532: "LDAP Who am I? Operation"
const WhoAmIOID = "1.3.6.1.4.1.4203.1.11.3"

// WhoAmIHandler implements the Who Am I extended operation.
// This operation returns the authorization identity of the current connection,
// which is useful for debugging and connection pooling scenarios.
//
// Per RFC 4532, the response format is:
// - Empty string for anonymous connections
// - "dn:<DN>" for connections bound with a distinguished name
type WhoAmIHandler struct{}

// NewWhoAmIHandler creates a new WhoAmIHandler.
func NewWhoAmIHandler() *WhoAmIHandler {
	return &WhoAmIHandler{}
}

// OID returns the object identifier for the Who Am I extended operation.
func (h *WhoAmIHandler) OID() string {
	return WhoAmIOID
}

// Handle processes the Who Am I extended request and returns the authorization identity.
// The response value contains:
// - Empty string for anonymous (unauthenticated) connections
// - "dn:<bindDN>" for authenticated connections
func (h *WhoAmIHandler) Handle(conn *Connection, req *ExtendedRequest) (*ExtendedResponse, error) {
	var authzID string

	bindDN := conn.BindDN()
	if bindDN == "" {
		// Anonymous connection - return empty authzID
		authzID = ""
	} else {
		// Authenticated connection - return "dn:" prefix followed by the bind DN
		authzID = "dn:" + bindDN
	}

	return &ExtendedResponse{
		Result: OperationResult{
			ResultCode: ldap.ResultSuccess,
		},
		Value: []byte(authzID),
	}, nil
}
