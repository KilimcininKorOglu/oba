// Package server provides the LDAP server implementation.
package server

import (
	"strings"

	"github.com/oba-ldap/oba/internal/acl"
	"github.com/oba-ldap/oba/internal/ldap"
)

// DeleteBackend defines the interface for the directory backend used by delete operations.
// It extends the basic Backend interface with delete-specific methods.
type DeleteBackend interface {
	Backend
	// DeleteEntry deletes an entry by its DN.
	// Returns an error if the entry does not exist or has children.
	DeleteEntry(dn string) error
	// HasChildren returns true if the entry has child entries.
	HasChildren(dn string) (bool, error)
}

// DeleteConfig holds configuration for the delete handler.
type DeleteConfig struct {
	// Backend is the directory backend for entry operations.
	Backend DeleteBackend
	// ACLEvaluator is the ACL evaluator for access control checks.
	// If nil, no ACL checks are performed.
	ACLEvaluator *acl.Evaluator
}

// NewDeleteConfig creates a new DeleteConfig with default settings.
func NewDeleteConfig() *DeleteConfig {
	return &DeleteConfig{}
}

// DeleteHandlerImpl implements the delete operation handler.
type DeleteHandlerImpl struct {
	config *DeleteConfig
}

// NewDeleteHandler creates a new delete handler with the given configuration.
func NewDeleteHandler(config *DeleteConfig) *DeleteHandlerImpl {
	if config == nil {
		config = NewDeleteConfig()
	}
	return &DeleteHandlerImpl{
		config: config,
	}
}

// Handle processes a delete request and returns the result.
// It implements the DeleteHandler function signature.
func (h *DeleteHandlerImpl) Handle(conn *Connection, req *ldap.DeleteRequest) *OperationResult {
	// Step 1: Validate the request
	if err := req.Validate(); err != nil {
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
	dn := normalizeDNForDelete(req.DN)

	// Step 4: Check ACL delete permission
	if h.config.ACLEvaluator != nil {
		bindDN := ""
		if conn != nil {
			bindDN = conn.BindDN()
		}
		if !h.config.ACLEvaluator.CanDelete(bindDN, dn) {
			return &OperationResult{
				ResultCode:        ldap.ResultInsufficientAccessRights,
				DiagnosticMessage: "insufficient access rights",
			}
		}
	}

	// Step 5: Check if entry exists
	entry, err := h.config.Backend.GetEntry(dn)
	if err != nil {
		return &OperationResult{
			ResultCode:        ldap.ResultOperationsError,
			DiagnosticMessage: "internal error during delete",
		}
	}

	if entry == nil {
		return &OperationResult{
			ResultCode:        ldap.ResultNoSuchObject,
			MatchedDN:         findMatchedDNForDelete(dn),
			DiagnosticMessage: "entry does not exist",
		}
	}

	// Step 6: Check if entry has children (LDAP doesn't allow deleting non-leaf entries)
	hasChildren, err := h.config.Backend.HasChildren(dn)
	if err != nil {
		return &OperationResult{
			ResultCode:        ldap.ResultOperationsError,
			DiagnosticMessage: "internal error checking children",
		}
	}

	if hasChildren {
		return &OperationResult{
			ResultCode:        ldap.ResultNotAllowedOnNonLeaf,
			DiagnosticMessage: "entry has subordinate entries",
		}
	}

	// Step 7: Delete the entry
	if err := h.config.Backend.DeleteEntry(dn); err != nil {
		// Check for specific error types
		if strings.Contains(err.Error(), "not found") {
			return &OperationResult{
				ResultCode:        ldap.ResultNoSuchObject,
				MatchedDN:         findMatchedDNForDelete(dn),
				DiagnosticMessage: "entry does not exist",
			}
		}
		return &OperationResult{
			ResultCode:        ldap.ResultOperationsError,
			DiagnosticMessage: "failed to delete entry: " + err.Error(),
		}
	}

	// Step 8: Return success
	return &OperationResult{
		ResultCode: ldap.ResultSuccess,
	}
}

// normalizeDNForDelete normalizes a DN for consistent comparison.
// It converts to lowercase and trims whitespace.
func normalizeDNForDelete(dn string) string {
	return strings.ToLower(strings.TrimSpace(dn))
}

// findMatchedDNForDelete finds the longest existing parent DN for error reporting.
// For now, returns empty string as we don't have access to the full tree.
func findMatchedDNForDelete(dn string) string {
	// In a full implementation, this would traverse up the DN tree
	// to find the closest existing ancestor.
	// For now, return empty string.
	return ""
}

// CreateDeleteHandler creates a DeleteHandler function from a DeleteHandlerImpl.
// This allows the DeleteHandlerImpl to be used with the Handler's SetDeleteHandler method.
func CreateDeleteHandler(impl *DeleteHandlerImpl) DeleteHandler {
	return func(conn *Connection, req *ldap.DeleteRequest) *OperationResult {
		return impl.Handle(conn, req)
	}
}
