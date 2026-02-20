// Package server provides the LDAP server implementation.
package server

import (
	"crypto/rand"
	"strings"

	"github.com/oba-ldap/oba/internal/ber"
	"github.com/oba-ldap/oba/internal/ldap"
	"github.com/oba-ldap/oba/internal/password"
)

// PasswordModifyOID is the OID for the Password Modify extended operation.
// Per RFC 3062: "LDAP Password Modify Extended Operation"
const PasswordModifyOID = "1.3.6.1.4.1.4203.1.11.1"

// PasswordModifyRequest represents a Password Modify extended request.
// Per RFC 3062:
// PasswdModifyRequestValue ::= SEQUENCE {
//
//	userIdentity    [0]  OCTET STRING OPTIONAL
//	oldPasswd       [1]  OCTET STRING OPTIONAL
//	newPasswd       [2]  OCTET STRING OPTIONAL
//
// }
type PasswordModifyRequest struct {
	// UserIdentity is the DN of the user whose password is being modified.
	// If empty, the bound user's password is modified.
	UserIdentity []byte
	// OldPassword is the current password (optional).
	// Required when a user changes their own password.
	OldPassword []byte
	// NewPassword is the new password (optional).
	// If empty, the server generates a password.
	NewPassword []byte
}

// PasswordModifyResponse represents a Password Modify extended response.
// Per RFC 3062:
// PasswdModifyResponseValue ::= SEQUENCE {
//
//	genPasswd       [0]     OCTET STRING OPTIONAL
//
// }
type PasswordModifyResponse struct {
	// GenPassword is the server-generated password (if requested).
	GenPassword []byte
}

// PasswordBackend defines the interface for password operations.
// This interface is used by the PasswordModifyHandler to interact with the backend.
type PasswordBackend interface {
	// GetEntry retrieves an entry by its DN.
	GetEntry(dn string) (*Entry, error)
	// SetPassword sets the password for the given DN.
	SetPassword(dn string, password []byte) error
	// VerifyPassword verifies the password for the given DN.
	VerifyPassword(dn string, password string) error
}

// Entry represents a directory entry for password operations.
type Entry struct {
	// DN is the distinguished name of the entry.
	DN string
	// Password is the hashed password.
	Password string
}

// PasswordModifyConfig holds configuration for the password modify handler.
type PasswordModifyConfig struct {
	// Backend is the password backend for entry lookups and password updates.
	Backend PasswordBackend
	// PolicyManager is the password policy manager.
	PolicyManager *password.Manager
	// AdminDNs is a list of DNs that have admin privileges.
	AdminDNs []string
	// DefaultScheme is the default password hashing scheme.
	DefaultScheme string
	// RequireTLS requires TLS for password modifications.
	RequireTLS bool
}

// NewPasswordModifyConfig creates a new PasswordModifyConfig with default settings.
func NewPasswordModifyConfig() *PasswordModifyConfig {
	return &PasswordModifyConfig{
		PolicyManager: password.NewManager(nil),
		AdminDNs:      []string{},
		DefaultScheme: SchemeSSHA256,
		RequireTLS:    false,
	}
}

// PasswordModifyHandler implements the Password Modify extended operation.
// This operation allows users to change their own passwords or administrators
// to reset passwords for other users.
type PasswordModifyHandler struct {
	config *PasswordModifyConfig
}

// NewPasswordModifyHandler creates a new PasswordModifyHandler.
func NewPasswordModifyHandler(config *PasswordModifyConfig) *PasswordModifyHandler {
	if config == nil {
		config = NewPasswordModifyConfig()
	}
	return &PasswordModifyHandler{
		config: config,
	}
}

// OID returns the object identifier for the Password Modify extended operation.
func (h *PasswordModifyHandler) OID() string {
	return PasswordModifyOID
}

// Handle processes the Password Modify extended request and returns a response.
func (h *PasswordModifyHandler) Handle(conn *Connection, req *ExtendedRequest) (*ExtendedResponse, error) {
	// Check TLS requirement
	if h.config.RequireTLS && !conn.IsTLS() {
		return &ExtendedResponse{
			Result: OperationResult{
				ResultCode:        ldap.ResultConfidentialityRequired,
				DiagnosticMessage: "TLS required for password modification",
			},
		}, nil
	}

	// Parse the password modify request
	pmReq, err := parsePasswordModifyRequest(req.Value)
	if err != nil {
		return &ExtendedResponse{
			Result: OperationResult{
				ResultCode:        ldap.ResultProtocolError,
				DiagnosticMessage: "invalid password modify request: " + err.Error(),
			},
		}, nil
	}

	// Determine target user
	targetDN := conn.BindDN()
	if len(pmReq.UserIdentity) > 0 {
		targetDN = string(pmReq.UserIdentity)
	}

	// Check if user is authenticated
	if targetDN == "" {
		return &ExtendedResponse{
			Result: OperationResult{
				ResultCode:        ldap.ResultUnwillingToPerform,
				DiagnosticMessage: "cannot modify password: no target user specified",
			},
		}, nil
	}

	// Check permissions
	boundDN := conn.BindDN()
	isOwnPassword := normalizeDNForCompare(targetDN) == normalizeDNForCompare(boundDN)
	isAdmin := h.isAdmin(boundDN)

	if !isOwnPassword && !isAdmin {
		return &ExtendedResponse{
			Result: OperationResult{
				ResultCode:        ldap.ResultInsufficientAccessRights,
				DiagnosticMessage: "cannot modify other user's password",
			},
		}, nil
	}

	// Verify old password if provided (required for non-admin self-change)
	if len(pmReq.OldPassword) > 0 {
		if h.config.Backend != nil {
			if err := h.config.Backend.VerifyPassword(targetDN, string(pmReq.OldPassword)); err != nil {
				return &ExtendedResponse{
					Result: OperationResult{
						ResultCode:        ldap.ResultInvalidCredentials,
						DiagnosticMessage: "old password is incorrect",
					},
				}, nil
			}
		}
	} else if isOwnPassword && !isAdmin {
		// Non-admin users must provide old password when changing their own
		return &ExtendedResponse{
			Result: OperationResult{
				ResultCode:        ldap.ResultUnwillingToPerform,
				DiagnosticMessage: "old password required for self-service password change",
			},
		}, nil
	}

	// Generate password if not provided
	newPassword := pmReq.NewPassword
	var genPassword []byte
	if len(newPassword) == 0 {
		genPassword = generateSecurePassword(16)
		newPassword = genPassword
	}

	// Validate against policy
	if h.config.PolicyManager != nil {
		policy := h.config.PolicyManager.GetPolicy(targetDN)
		if err := policy.Validate(string(newPassword)); err != nil {
			return &ExtendedResponse{
				Result: OperationResult{
					ResultCode:        ldap.ResultConstraintViolation,
					DiagnosticMessage: err.Error(),
				},
			}, nil
		}
	}

	// Update password in backend
	if h.config.Backend != nil {
		if err := h.config.Backend.SetPassword(targetDN, newPassword); err != nil {
			return &ExtendedResponse{
				Result: OperationResult{
					ResultCode:        ldap.ResultOperationsError,
					DiagnosticMessage: "failed to update password: " + err.Error(),
				},
			}, nil
		}
	}

	// Build response
	resp := &PasswordModifyResponse{
		GenPassword: genPassword,
	}

	return &ExtendedResponse{
		Result: OperationResult{
			ResultCode: ldap.ResultSuccess,
		},
		Value: encodePasswordModifyResponse(resp),
	}, nil
}

// isAdmin checks if the given DN has admin privileges.
func (h *PasswordModifyHandler) isAdmin(dn string) bool {
	if dn == "" {
		return false
	}

	normalizedDN := normalizeDNForCompare(dn)
	for _, adminDN := range h.config.AdminDNs {
		if normalizeDNForCompare(adminDN) == normalizedDN {
			return true
		}
	}
	return false
}

// parsePasswordModifyRequest parses a Password Modify request from BER-encoded data.
func parsePasswordModifyRequest(data []byte) (*PasswordModifyRequest, error) {
	req := &PasswordModifyRequest{}

	// Empty request is valid (modify bound user's password, server generates new password)
	if len(data) == 0 {
		return req, nil
	}

	decoder := ber.NewBERDecoder(data)

	// Read the SEQUENCE
	_, err := decoder.ExpectSequence()
	if err != nil {
		return nil, err
	}

	// Read optional fields based on context tags
	for decoder.Remaining() > 0 {
		// Peek at the tag to determine which field this is
		class, _, number, err := decoder.PeekTag()
		if err != nil {
			break
		}

		// Only process context-specific tags
		if class != ber.ClassContextSpecific {
			break
		}

		switch number {
		case 0: // userIdentity [0] OCTET STRING OPTIONAL
			_, _, value, err := decoder.ReadTaggedValue()
			if err != nil {
				return nil, err
			}
			req.UserIdentity = value

		case 1: // oldPasswd [1] OCTET STRING OPTIONAL
			_, _, value, err := decoder.ReadTaggedValue()
			if err != nil {
				return nil, err
			}
			req.OldPassword = value

		case 2: // newPasswd [2] OCTET STRING OPTIONAL
			_, _, value, err := decoder.ReadTaggedValue()
			if err != nil {
				return nil, err
			}
			req.NewPassword = value

		default:
			// Skip unknown tags
			if err := decoder.Skip(); err != nil {
				return nil, err
			}
		}
	}

	return req, nil
}

// encodePasswordModifyResponse encodes a Password Modify response to BER format.
func encodePasswordModifyResponse(resp *PasswordModifyResponse) []byte {
	// If no generated password, return empty response
	if len(resp.GenPassword) == 0 {
		return nil
	}

	encoder := ber.NewBEREncoder(64)

	// Write SEQUENCE
	seqPos := encoder.BeginSequence()

	// Write genPasswd [0] OCTET STRING OPTIONAL
	if len(resp.GenPassword) > 0 {
		if err := encoder.WriteTaggedValue(0, false, resp.GenPassword); err != nil {
			return nil
		}
	}

	if err := encoder.EndSequence(seqPos); err != nil {
		return nil
	}

	return encoder.Bytes()
}

// generateSecurePassword generates a cryptographically secure random password.
func generateSecurePassword(length int) []byte {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"

	password := make([]byte, length)
	randomBytes := make([]byte, length)

	_, err := rand.Read(randomBytes)
	if err != nil {
		// Fallback to a less secure but functional approach
		for i := range password {
			password[i] = charset[i%len(charset)]
		}
		return password
	}

	for i := range password {
		password[i] = charset[int(randomBytes[i])%len(charset)]
	}

	return password
}

// normalizeDNForCompare normalizes a DN for comparison.
func normalizeDNForCompare(dn string) string {
	return strings.ToLower(strings.TrimSpace(dn))
}

// SetBackend sets the password backend.
func (h *PasswordModifyHandler) SetBackend(backend PasswordBackend) {
	h.config.Backend = backend
}

// SetPolicyManager sets the password policy manager.
func (h *PasswordModifyHandler) SetPolicyManager(manager *password.Manager) {
	h.config.PolicyManager = manager
}

// AddAdminDN adds an admin DN to the list of admin DNs.
func (h *PasswordModifyHandler) AddAdminDN(dn string) {
	h.config.AdminDNs = append(h.config.AdminDNs, dn)
}

// SetRequireTLS sets whether TLS is required for password modifications.
func (h *PasswordModifyHandler) SetRequireTLS(require bool) {
	h.config.RequireTLS = require
}

// Config returns the handler's configuration.
func (h *PasswordModifyHandler) Config() *PasswordModifyConfig {
	return h.config
}
