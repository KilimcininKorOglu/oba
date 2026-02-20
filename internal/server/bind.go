// Package server provides the LDAP server implementation.
package server

import (
	"strings"

	"github.com/oba-ldap/oba/internal/ldap"
	"github.com/oba-ldap/oba/internal/storage"
)

// LDAPVersion3 is the required LDAP protocol version.
const LDAPVersion3 = 3

// Backend defines the interface for the directory backend.
// It provides methods for looking up entries and verifying credentials.
type Backend interface {
	// GetEntry retrieves an entry by its DN.
	// Returns nil if the entry does not exist.
	GetEntry(dn string) (*storage.Entry, error)
}

// BindConfig holds configuration for the bind handler.
type BindConfig struct {
	// Backend is the directory backend for entry lookups.
	Backend Backend
	// AllowAnonymous controls whether anonymous binds are allowed.
	AllowAnonymous bool
	// RootDN is the administrator DN (optional).
	RootDN string
	// RootPassword is the administrator password hash (optional).
	RootPassword string
}

// NewBindConfig creates a new BindConfig with default settings.
func NewBindConfig() *BindConfig {
	return &BindConfig{
		AllowAnonymous: true,
	}
}

// BindHandlerImpl implements the bind operation handler.
type BindHandlerImpl struct {
	config *BindConfig
}

// NewBindHandler creates a new bind handler with the given configuration.
func NewBindHandler(config *BindConfig) *BindHandlerImpl {
	if config == nil {
		config = NewBindConfig()
	}
	return &BindHandlerImpl{
		config: config,
	}
}

// Handle processes a bind request and returns the result.
// It implements the BindHandler function signature.
func (h *BindHandlerImpl) Handle(conn *Connection, req *ldap.BindRequest) *OperationResult {
	// Step 1: Validate LDAP version (must be 3)
	if req.Version != LDAPVersion3 {
		return &OperationResult{
			ResultCode:        ldap.ResultProtocolError,
			DiagnosticMessage: "only LDAP version 3 is supported",
		}
	}

	// Step 2: Handle anonymous bind
	if req.IsAnonymous() {
		return h.handleAnonymousBind(conn)
	}

	// Step 3: Handle simple authentication
	if req.AuthMethod == ldap.AuthMethodSimple {
		return h.handleSimpleBind(conn, req)
	}

	// Step 4: SASL authentication is not supported yet
	if req.AuthMethod == ldap.AuthMethodSASL {
		return &OperationResult{
			ResultCode:        ldap.ResultAuthMethodNotSupported,
			DiagnosticMessage: "SASL authentication is not supported",
		}
	}

	// Unknown authentication method
	return &OperationResult{
		ResultCode:        ldap.ResultAuthMethodNotSupported,
		DiagnosticMessage: "unsupported authentication method",
	}
}

// handleAnonymousBind processes an anonymous bind request.
func (h *BindHandlerImpl) handleAnonymousBind(conn *Connection) *OperationResult {
	if !h.config.AllowAnonymous {
		return &OperationResult{
			ResultCode:        ldap.ResultInappropriateAuthentication,
			DiagnosticMessage: "anonymous bind is not allowed",
		}
	}

	// Anonymous bind successful - connection state is updated by the caller
	return &OperationResult{
		ResultCode: ldap.ResultSuccess,
	}
}

// handleSimpleBind processes a simple (DN + password) bind request.
func (h *BindHandlerImpl) handleSimpleBind(conn *Connection, req *ldap.BindRequest) *OperationResult {
	dn := req.Name
	password := string(req.SimplePassword)

	// Normalize the DN for comparison
	normalizedDN := normalizeDN(dn)

	// Check if this is a root DN bind
	if h.config.RootDN != "" && normalizeDN(h.config.RootDN) == normalizedDN {
		return h.verifyRootBind(password)
	}

	// Look up the entry in the backend
	if h.config.Backend == nil {
		return &OperationResult{
			ResultCode:        ldap.ResultOperationsError,
			DiagnosticMessage: "backend not configured",
		}
	}

	entry, err := h.config.Backend.GetEntry(dn)
	if err != nil {
		return &OperationResult{
			ResultCode:        ldap.ResultOperationsError,
			DiagnosticMessage: "internal error during authentication",
		}
	}

	if entry == nil {
		// Entry not found - return invalid credentials
		// (We don't reveal whether the DN exists or not for security)
		return &OperationResult{
			ResultCode:        ldap.ResultInvalidCredentials,
			DiagnosticMessage: "invalid credentials",
		}
	}

	// Verify the password
	return h.verifyEntryPassword(entry, password)
}

// verifyRootBind verifies the root DN password.
func (h *BindHandlerImpl) verifyRootBind(password string) *OperationResult {
	if h.config.RootPassword == "" {
		return &OperationResult{
			ResultCode:        ldap.ResultInvalidCredentials,
			DiagnosticMessage: "invalid credentials",
		}
	}

	err := VerifyPassword(password, h.config.RootPassword)
	if err != nil {
		return &OperationResult{
			ResultCode:        ldap.ResultInvalidCredentials,
			DiagnosticMessage: "invalid credentials",
		}
	}

	return &OperationResult{
		ResultCode: ldap.ResultSuccess,
	}
}

// verifyEntryPassword verifies the password against the entry's userPassword attribute.
func (h *BindHandlerImpl) verifyEntryPassword(entry *storage.Entry, password string) *OperationResult {
	// Get the userPassword attribute
	passwords := entry.GetAttribute(PasswordAttribute)
	if len(passwords) == 0 {
		// No password set - authentication fails
		return &OperationResult{
			ResultCode:        ldap.ResultInvalidCredentials,
			DiagnosticMessage: "invalid credentials",
		}
	}

	// Try each stored password (there may be multiple)
	for _, storedPassword := range passwords {
		err := VerifyPassword(password, string(storedPassword))
		if err == nil {
			// Password matches
			return &OperationResult{
				ResultCode: ldap.ResultSuccess,
			}
		}
	}

	// No password matched
	return &OperationResult{
		ResultCode:        ldap.ResultInvalidCredentials,
		DiagnosticMessage: "invalid credentials",
	}
}

// normalizeDN normalizes a DN for consistent comparison.
// It converts to lowercase and trims whitespace.
func normalizeDN(dn string) string {
	return strings.ToLower(strings.TrimSpace(dn))
}

// CreateBindHandler creates a BindHandler function from a BindHandlerImpl.
// This allows the BindHandlerImpl to be used with the Handler's SetBindHandler method.
func CreateBindHandler(impl *BindHandlerImpl) BindHandler {
	return func(conn *Connection, req *ldap.BindRequest) *OperationResult {
		return impl.Handle(conn, req)
	}
}
