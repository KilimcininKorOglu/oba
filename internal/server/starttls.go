// Package server provides the LDAP server implementation.
package server

import (
	"crypto/tls"
	"errors"
	"net"
	"sync"

	"github.com/oba-ldap/oba/internal/ldap"
)

// StartTLS OID as defined in RFC 4511
const StartTLSOID = "1.3.6.1.4.1.1466.20037"

// StartTLS errors
var (
	// ErrAlreadyTLS is returned when StartTLS is requested on an already-TLS connection
	ErrAlreadyTLS = errors.New("server: connection already using TLS")
	// ErrTLSHandshakeFailed is returned when the TLS handshake fails
	ErrTLSHandshakeFailed = errors.New("server: TLS handshake failed")
	// ErrNoTLSConfig is returned when TLS configuration is not available
	ErrNoTLSConfig = errors.New("server: TLS configuration not available")
	// ErrStartTLSUnavailable is returned when StartTLS is not supported
	ErrStartTLSUnavailable = errors.New("server: StartTLS not available")
)

// StartTLSHandler handles the StartTLS extended operation.
// It upgrades a plain LDAP connection to TLS.
type StartTLSHandler struct {
	// tlsConfig is the TLS configuration to use for the upgrade
	tlsConfig *tls.Config
	// mu protects concurrent access
	mu sync.RWMutex
}

// NewStartTLSHandler creates a new StartTLSHandler with the given TLS configuration.
func NewStartTLSHandler(tlsConfig *tls.Config) *StartTLSHandler {
	return &StartTLSHandler{
		tlsConfig: tlsConfig,
	}
}

// SetTLSConfig updates the TLS configuration.
func (h *StartTLSHandler) SetTLSConfig(tlsConfig *tls.Config) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.tlsConfig = tlsConfig
}

// GetTLSConfig returns the current TLS configuration.
func (h *StartTLSHandler) GetTLSConfig() *tls.Config {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.tlsConfig
}

// Handle processes a StartTLS extended request.
// It returns the response to send and an error if the operation failed.
// Note: The response is sent before the TLS handshake, so the caller
// must handle the special case where the response is already sent.
func (h *StartTLSHandler) Handle(conn *TLSConnection, req *ExtendedRequest) (*ExtendedResponse, error) {
	// Verify the OID matches StartTLS
	if req.OID != StartTLSOID {
		return &ExtendedResponse{
			Result: OperationResult{
				ResultCode:        ldap.ResultProtocolError,
				DiagnosticMessage: "invalid OID for StartTLS",
			},
		}, nil
	}

	// Check if connection is already using TLS
	if conn.IsTLS() {
		return &ExtendedResponse{
			Result: OperationResult{
				ResultCode:        ldap.ResultOperationsError,
				DiagnosticMessage: "connection already using TLS",
			},
		}, nil
	}

	// Check if TLS configuration is available
	h.mu.RLock()
	tlsConfig := h.tlsConfig
	h.mu.RUnlock()

	if tlsConfig == nil {
		return &ExtendedResponse{
			Result: OperationResult{
				ResultCode:        ldap.ResultUnavailable,
				DiagnosticMessage: "TLS configuration not available",
			},
		}, nil
	}

	// Return success response - caller will send this before TLS handshake
	return &ExtendedResponse{
		Result: OperationResult{
			ResultCode: ldap.ResultSuccess,
		},
		OID: StartTLSOID,
	}, nil
}

// UpgradeToTLS performs the TLS handshake on the connection.
// This should be called after sending the success response.
func (h *StartTLSHandler) UpgradeToTLS(conn *TLSConnection) error {
	h.mu.RLock()
	tlsConfig := h.tlsConfig
	h.mu.RUnlock()

	if tlsConfig == nil {
		return ErrNoTLSConfig
	}

	return conn.UpgradeToTLS(tlsConfig)
}

// TLSConnection wraps a Connection with TLS upgrade capability.
// It tracks whether the connection is using TLS and provides
// methods to upgrade the connection.
type TLSConnection struct {
	*Connection
	// conn is the underlying network connection (may be upgraded to TLS)
	netConn net.Conn
	// isTLS indicates whether the connection is using TLS
	isTLS bool
	// tlsMu protects TLS state
	tlsMu sync.RWMutex
}

// NewTLSConnection creates a new TLSConnection wrapping the given Connection.
func NewTLSConnection(conn *Connection) *TLSConnection {
	tc := &TLSConnection{
		Connection: conn,
		netConn:    conn.conn,
		isTLS:      false,
	}

	// Check if the underlying connection is already TLS
	if _, ok := conn.conn.(*tls.Conn); ok {
		tc.isTLS = true
	}

	return tc
}

// IsTLS returns whether the connection is using TLS.
func (tc *TLSConnection) IsTLS() bool {
	tc.tlsMu.RLock()
	defer tc.tlsMu.RUnlock()
	return tc.isTLS
}

// UpgradeToTLS upgrades the connection to TLS.
// This should only be called after sending the StartTLS success response.
func (tc *TLSConnection) UpgradeToTLS(tlsConfig *tls.Config) error {
	tc.tlsMu.Lock()
	defer tc.tlsMu.Unlock()

	if tc.isTLS {
		return ErrAlreadyTLS
	}

	// Create TLS server connection
	tlsConn := tls.Server(tc.netConn, tlsConfig)

	// Perform TLS handshake
	if err := tlsConn.Handshake(); err != nil {
		return err
	}

	// Update the connection to use TLS
	tc.netConn = tlsConn
	tc.Connection.conn = tlsConn
	tc.isTLS = true

	return nil
}

// GetTLSConnectionState returns the TLS connection state if using TLS.
// Returns nil if not using TLS.
func (tc *TLSConnection) GetTLSConnectionState() *tls.ConnectionState {
	tc.tlsMu.RLock()
	defer tc.tlsMu.RUnlock()

	if !tc.isTLS {
		return nil
	}

	if tlsConn, ok := tc.netConn.(*tls.Conn); ok {
		state := tlsConn.ConnectionState()
		return &state
	}

	return nil
}

// WriteExtendedResponse writes an ExtendedResponse to the connection.
func (tc *TLSConnection) WriteExtendedResponse(messageID int, resp *ExtendedResponse) error {
	msg := createExtendedResponse(messageID, resp)
	if msg == nil {
		return errors.New("failed to create extended response")
	}
	return tc.WriteMessage(msg)
}

// HandleStartTLS is a convenience function that handles the complete StartTLS flow.
// It sends the response and performs the TLS upgrade if successful.
func HandleStartTLS(conn *TLSConnection, messageID int, req *ExtendedRequest, handler *StartTLSHandler) error {
	// Handle the request
	resp, err := handler.Handle(conn, req)
	if err != nil {
		return err
	}

	// Send the response
	if err := conn.WriteExtendedResponse(messageID, resp); err != nil {
		return err
	}

	// If successful, perform TLS upgrade
	if resp.Result.ResultCode == ldap.ResultSuccess {
		if err := handler.UpgradeToTLS(conn); err != nil {
			// Connection is in an undefined state after failed handshake
			conn.Close()
			return err
		}
	}

	return nil
}
