// Package server provides the LDAP server implementation.
package server

import (
	"strings"

	"github.com/oba-ldap/oba/internal/acl"
	"github.com/oba-ldap/oba/internal/ldap"
)

// ModificationType represents the type of modification operation.
type ModificationType int

const (
	// ModifyAdd adds values to an attribute.
	ModifyAdd ModificationType = iota
	// ModifyDelete removes values from an attribute.
	ModifyDelete
	// ModifyReplace replaces all values of an attribute.
	ModifyReplace
)

// Modification represents a single modification to an entry.
type Modification struct {
	// Type is the type of modification (add, delete, replace).
	Type ModificationType
	// Attribute is the name of the attribute to modify.
	Attribute string
	// Values are the values to add, delete, or replace.
	Values []string
}

// ModifyBackend defines the interface for the directory backend used by modify operations.
// It extends the basic Backend interface with modify-specific methods.
type ModifyBackend interface {
	Backend
	// ModifyEntry modifies an entry by its DN with the given changes.
	// Returns an error if the entry does not exist or modifications are invalid.
	ModifyEntry(dn string, changes []Modification) error
}

// ModifyConfig holds configuration for the modify handler.
type ModifyConfig struct {
	// Backend is the directory backend for entry operations.
	Backend ModifyBackend
	// ACLEvaluator is the ACL evaluator for access control checks.
	// If nil, no ACL checks are performed.
	ACLEvaluator *acl.Evaluator
}

// NewModifyConfig creates a new ModifyConfig with default settings.
func NewModifyConfig() *ModifyConfig {
	return &ModifyConfig{}
}

// ModifyHandlerImpl implements the modify operation handler.
type ModifyHandlerImpl struct {
	config *ModifyConfig
}

// NewModifyHandler creates a new modify handler with the given configuration.
func NewModifyHandler(config *ModifyConfig) *ModifyHandlerImpl {
	if config == nil {
		config = NewModifyConfig()
	}
	return &ModifyHandlerImpl{
		config: config,
	}
}

// Handle processes a modify request and returns the result.
// It implements the ModifyHandler function signature.
func (h *ModifyHandlerImpl) Handle(conn *Connection, req *ldap.ModifyRequest) *OperationResult {
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
	dn := normalizeDNForModify(req.Object)

	// Step 4: Check ACL write permission
	if h.config.ACLEvaluator != nil {
		bindDN := ""
		if conn != nil {
			bindDN = conn.BindDN()
		}

		// Get the list of attributes being modified
		modifiedAttrs := getModifiedAttributes(req.Changes)

		// Create access context with attributes
		ctx := acl.NewAccessContext(bindDN, dn, acl.Write).WithAttributes(modifiedAttrs...)

		if !h.config.ACLEvaluator.CheckAccess(ctx) {
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
			DiagnosticMessage: "internal error during modify",
		}
	}

	if entry == nil {
		return &OperationResult{
			ResultCode:        ldap.ResultNoSuchObject,
			MatchedDN:         findMatchedDNForModify(dn),
			DiagnosticMessage: "entry does not exist",
		}
	}

	// Step 6: Convert LDAP modifications to backend modifications
	backendChanges := convertToBackendModifications(req.Changes)

	// Step 7: Apply the modifications
	if err := h.config.Backend.ModifyEntry(dn, backendChanges); err != nil {
		return h.mapError(err, dn)
	}

	// Step 8: Return success
	return &OperationResult{
		ResultCode: ldap.ResultSuccess,
	}
}

// getModifiedAttributes extracts the list of attribute names being modified.
func getModifiedAttributes(changes []ldap.Modification) []string {
	attrs := make([]string, 0, len(changes))
	seen := make(map[string]bool)

	for _, change := range changes {
		attrName := strings.ToLower(change.Attribute.Type)
		if !seen[attrName] {
			attrs = append(attrs, attrName)
			seen[attrName] = true
		}
	}

	return attrs
}

// mapError maps backend errors to LDAP result codes.
func (h *ModifyHandlerImpl) mapError(err error, dn string) *OperationResult {
	errStr := err.Error()

	// Check for specific error types
	if strings.Contains(errStr, "not found") {
		return &OperationResult{
			ResultCode:        ldap.ResultNoSuchObject,
			MatchedDN:         findMatchedDNForModify(dn),
			DiagnosticMessage: "entry does not exist",
		}
	}

	if strings.Contains(errStr, "invalid") {
		return &OperationResult{
			ResultCode:        ldap.ResultConstraintViolation,
			DiagnosticMessage: "modification violates constraints: " + errStr,
		}
	}

	if strings.Contains(errStr, "schema") || strings.Contains(errStr, "objectclass") {
		return &OperationResult{
			ResultCode:        ldap.ResultObjectClassViolation,
			DiagnosticMessage: "schema violation: " + errStr,
		}
	}

	if strings.Contains(errStr, "attribute") && strings.Contains(errStr, "required") {
		return &OperationResult{
			ResultCode:        ldap.ResultObjectClassViolation,
			DiagnosticMessage: "required attribute missing: " + errStr,
		}
	}

	return &OperationResult{
		ResultCode:        ldap.ResultOperationsError,
		DiagnosticMessage: "failed to modify entry: " + errStr,
	}
}

// convertToBackendModifications converts LDAP modifications to server modifications.
func convertToBackendModifications(changes []ldap.Modification) []Modification {
	result := make([]Modification, len(changes))

	for i, change := range changes {
		// Convert values from [][]byte to []string
		values := make([]string, len(change.Attribute.Values))
		for j, v := range change.Attribute.Values {
			values[j] = string(v)
		}

		// Map LDAP operation to modification type
		var modType ModificationType
		switch change.Operation {
		case ldap.ModifyOperationAdd:
			modType = ModifyAdd
		case ldap.ModifyOperationDelete:
			modType = ModifyDelete
		case ldap.ModifyOperationReplace:
			modType = ModifyReplace
		}

		result[i] = Modification{
			Type:      modType,
			Attribute: change.Attribute.Type,
			Values:    values,
		}
	}

	return result
}

// normalizeDNForModify normalizes a DN for consistent comparison.
// It converts to lowercase and trims whitespace.
func normalizeDNForModify(dn string) string {
	return strings.ToLower(strings.TrimSpace(dn))
}

// findMatchedDNForModify finds the longest existing parent DN for error reporting.
// For now, returns empty string as we don't have access to the full tree.
func findMatchedDNForModify(dn string) string {
	// In a full implementation, this would traverse up the DN tree
	// to find the closest existing ancestor.
	// For now, return empty string.
	return ""
}

// CreateModifyHandler creates a ModifyHandler function from a ModifyHandlerImpl.
// This allows the ModifyHandlerImpl to be used with the Handler's SetModifyHandler method.
func CreateModifyHandler(impl *ModifyHandlerImpl) ModifyHandler {
	return func(conn *Connection, req *ldap.ModifyRequest) *OperationResult {
		return impl.Handle(conn, req)
	}
}
