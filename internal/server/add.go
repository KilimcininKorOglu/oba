// Package server provides the LDAP server implementation.
package server

import (
	"strings"

	"github.com/oba-ldap/oba/internal/ldap"
	"github.com/oba-ldap/oba/internal/storage"
)

// AddBackend defines the interface for the directory backend used by add operations.
// It extends the basic Backend interface with add-specific methods.
type AddBackend interface {
	Backend
	// AddEntry adds a new entry to the directory.
	// Returns an error if the entry already exists, parent doesn't exist,
	// or required attributes are missing.
	AddEntry(entry *storage.Entry) error
}

// AddConfig holds configuration for the add handler.
type AddConfig struct {
	// Backend is the directory backend for entry operations.
	Backend AddBackend
}

// NewAddConfig creates a new AddConfig with default settings.
func NewAddConfig() *AddConfig {
	return &AddConfig{}
}

// AddHandlerImpl implements the add operation handler.
type AddHandlerImpl struct {
	config *AddConfig
}

// NewAddHandler creates a new add handler with the given configuration.
func NewAddHandler(config *AddConfig) *AddHandlerImpl {
	if config == nil {
		config = NewAddConfig()
	}
	return &AddHandlerImpl{
		config: config,
	}
}

// Handle processes an add request and returns the result.
// It implements the AddHandler function signature.
func (h *AddHandlerImpl) Handle(conn *Connection, req *ldap.AddRequest) *OperationResult {
	// Step 1: Validate the request
	if err := validateAddRequest(req); err != nil {
		return &OperationResult{
			ResultCode:        ldap.ResultProtocolError,
			DiagnosticMessage: err.Error(),
		}
	}

	// Step 2: Check if backend is configured
	if h.config.Backend == nil {
		return &OperationResult{
			ResultCode:        ldap.ResultOperationsError,
			DiagnosticMessage: "backend not configured",
		}
	}

	// Step 3: Normalize the DN
	dn := normalizeDNForAdd(req.Entry)

	// Step 4: Check if entry already exists
	existingEntry, err := h.config.Backend.GetEntry(dn)
	if err != nil {
		return &OperationResult{
			ResultCode:        ldap.ResultOperationsError,
			DiagnosticMessage: "internal error during add",
		}
	}

	if existingEntry != nil {
		return &OperationResult{
			ResultCode:        ldap.ResultEntryAlreadyExists,
			DiagnosticMessage: "entry already exists",
		}
	}

	// Step 5: Check if objectClass attribute is present
	if !hasObjectClassAttribute(req) {
		return &OperationResult{
			ResultCode:        ldap.ResultObjectClassViolation,
			DiagnosticMessage: "objectClass attribute is required",
		}
	}

	// Step 6: Convert request to backend entry
	entry := convertAddRequestToEntry(req)

	// Step 7: Add the entry
	if err := h.config.Backend.AddEntry(entry); err != nil {
		return mapAddError(err, dn)
	}

	// Step 8: Return success
	return &OperationResult{
		ResultCode: ldap.ResultSuccess,
	}
}

// validateAddRequest validates the add request.
func validateAddRequest(req *ldap.AddRequest) error {
	if req == nil {
		return ldap.ErrEmptyEntry
	}
	if req.Entry == "" {
		return ldap.ErrEmptyEntry
	}
	return nil
}

// normalizeDNForAdd normalizes a DN for consistent comparison.
// It converts to lowercase and trims whitespace.
func normalizeDNForAdd(dn string) string {
	return strings.ToLower(strings.TrimSpace(dn))
}

// hasObjectClassAttribute checks if the add request contains an objectClass attribute.
func hasObjectClassAttribute(req *ldap.AddRequest) bool {
	for _, attr := range req.Attributes {
		if strings.EqualFold(attr.Type, "objectclass") {
			return len(attr.Values) > 0
		}
	}
	return false
}

// convertAddRequestToEntry converts an LDAP AddRequest to a storage Entry.
func convertAddRequestToEntry(req *ldap.AddRequest) *storage.Entry {
	entry := storage.NewEntry(req.Entry)

	for _, attr := range req.Attributes {
		attrName := strings.ToLower(attr.Type)
		entry.SetAttribute(attrName, attr.Values)
	}

	return entry
}

// mapAddError maps backend errors to LDAP result codes.
func mapAddError(err error, dn string) *OperationResult {
	if err == nil {
		return &OperationResult{
			ResultCode: ldap.ResultSuccess,
		}
	}

	errStr := err.Error()

	// Check for specific error types
	if strings.Contains(errStr, "already exists") {
		return &OperationResult{
			ResultCode:        ldap.ResultEntryAlreadyExists,
			DiagnosticMessage: "entry already exists",
		}
	}

	if strings.Contains(errStr, "not found") || strings.Contains(errStr, "no parent") {
		return &OperationResult{
			ResultCode:        ldap.ResultNoSuchObject,
			MatchedDN:         findMatchedDNForAdd(dn),
			DiagnosticMessage: "parent entry does not exist",
		}
	}

	if strings.Contains(errStr, "invalid") {
		return &OperationResult{
			ResultCode:        ldap.ResultInvalidDNSyntax,
			DiagnosticMessage: "invalid DN syntax",
		}
	}

	if strings.Contains(errStr, "objectclass") || strings.Contains(errStr, "object class") {
		return &OperationResult{
			ResultCode:        ldap.ResultObjectClassViolation,
			DiagnosticMessage: "objectClass attribute is required",
		}
	}

	return &OperationResult{
		ResultCode:        ldap.ResultOperationsError,
		DiagnosticMessage: "failed to add entry: " + errStr,
	}
}

// findMatchedDNForAdd finds the longest existing parent DN for error reporting.
// For now, returns empty string as we don't have access to the full tree.
func findMatchedDNForAdd(dn string) string {
	// In a full implementation, this would traverse up the DN tree
	// to find the closest existing ancestor.
	// For now, return empty string.
	return ""
}

// CreateAddHandler creates an AddHandler function from an AddHandlerImpl.
// This allows the AddHandlerImpl to be used with the Handler's SetAddHandler method.
func CreateAddHandler(impl *AddHandlerImpl) AddHandler {
	return func(conn *Connection, req *ldap.AddRequest) *OperationResult {
		return impl.Handle(conn, req)
	}
}
