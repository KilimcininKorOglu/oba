// Package server provides the LDAP server implementation.
package server

import (
	"bytes"
	"strings"

	"github.com/KilimcininKorOglu/oba/internal/ldap"
	"github.com/KilimcininKorOglu/oba/internal/storage"
)

// CompareResult represents the result of a compare operation.
type CompareResult int

const (
	// CompareTrue indicates the attribute value matches.
	CompareTrue CompareResult = iota
	// CompareFalse indicates the attribute value does not match.
	CompareFalse
	// CompareNoSuchAttribute indicates the attribute does not exist.
	CompareNoSuchAttribute
)

// CompareBackend defines the interface for the directory backend used by compare operations.
type CompareBackend interface {
	// GetEntry retrieves an entry by its DN.
	// Returns nil if the entry does not exist.
	GetEntry(dn string) (*storage.Entry, error)
}

// CompareConfig holds configuration for the compare handler.
type CompareConfig struct {
	// Backend is the directory backend for entry operations.
	Backend CompareBackend
}

// NewCompareConfig creates a new CompareConfig with default settings.
func NewCompareConfig() *CompareConfig {
	return &CompareConfig{}
}

// CompareHandlerImpl implements the compare operation handler.
type CompareHandlerImpl struct {
	config *CompareConfig
}

// NewCompareHandler creates a new compare handler with the given configuration.
func NewCompareHandler(config *CompareConfig) *CompareHandlerImpl {
	if config == nil {
		config = NewCompareConfig()
	}
	return &CompareHandlerImpl{
		config: config,
	}
}

// Handle processes a compare request and returns the result.
// It implements the CompareHandler function signature.
func (h *CompareHandlerImpl) Handle(conn *Connection, req *ldap.CompareRequest) *OperationResult {
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
	dn := normalizeDNForCompare(req.DN)

	// Step 4: Check if entry exists
	entry, err := h.config.Backend.GetEntry(dn)
	if err != nil {
		return &OperationResult{
			ResultCode:        ldap.ResultOperationsError,
			DiagnosticMessage: "internal error during compare",
		}
	}

	if entry == nil {
		return &OperationResult{
			ResultCode:        ldap.ResultNoSuchObject,
			MatchedDN:         findMatchedDNForCompare(dn),
			DiagnosticMessage: "entry does not exist",
		}
	}

	// Step 5: Perform the compare operation
	result := compareAttributeValue(entry, req.Attribute, req.Value)

	// Step 6: Return the appropriate result code
	switch result {
	case CompareTrue:
		return &OperationResult{
			ResultCode: ldap.ResultCompareTrue,
		}
	case CompareFalse:
		return &OperationResult{
			ResultCode: ldap.ResultCompareFalse,
		}
	case CompareNoSuchAttribute:
		return &OperationResult{
			ResultCode:        ldap.ResultNoSuchAttribute,
			DiagnosticMessage: "attribute does not exist",
		}
	default:
		return &OperationResult{
			ResultCode:        ldap.ResultOperationsError,
			DiagnosticMessage: "unexpected compare result",
		}
	}
}

// compareAttributeValue compares an attribute value in an entry.
// Returns CompareTrue if the entry has the attribute with the specified value,
// CompareFalse if the attribute exists but doesn't have the value,
// or CompareNoSuchAttribute if the attribute doesn't exist.
func compareAttributeValue(entry *storage.Entry, attribute string, value []byte) CompareResult {
	normalizedAttr := strings.ToLower(strings.TrimSpace(attribute))

	// Get attribute values from the entry
	values := entry.GetAttribute(normalizedAttr)
	if len(values) == 0 {
		return CompareNoSuchAttribute
	}

	// Compare the value against each stored value
	for _, storedValue := range values {
		if compareValues(storedValue, value) {
			return CompareTrue
		}
	}

	return CompareFalse
}

// compareValues compares two attribute values.
// This performs a case-insensitive comparison for string values.
func compareValues(stored, assertion []byte) bool {
	// First try exact match
	if bytes.Equal(stored, assertion) {
		return true
	}

	// Try case-insensitive comparison for string values
	return bytes.EqualFold(stored, assertion)
}

// findMatchedDNForCompare finds the longest existing parent DN for error reporting.
// For now, returns empty string as we don't have access to the full tree.
func findMatchedDNForCompare(dn string) string {
	// In a full implementation, this would traverse up the DN tree
	// to find the closest existing ancestor.
	// For now, return empty string.
	return ""
}

// CreateCompareHandler creates a CompareHandler function from a CompareHandlerImpl.
// This allows the CompareHandlerImpl to be used with the Handler's SetCompareHandler method.
func CreateCompareHandler(impl *CompareHandlerImpl) CompareHandler {
	return func(conn *Connection, req *ldap.CompareRequest) *OperationResult {
		return impl.Handle(conn, req)
	}
}
