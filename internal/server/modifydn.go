// Package server provides the LDAP server implementation.
package server

import (
	"github.com/KilimcininKorOglu/oba/internal/ldap"
)

// ModifyDNRequest represents a request to rename or move an entry.
// This is the server-side representation used by the handler.
type ModifyDNRequestData struct {
	// DN is the distinguished name of the entry to rename/move.
	DN string
	// NewRDN is the new relative distinguished name.
	NewRDN string
	// DeleteOldRDN indicates whether to delete the old RDN attribute values.
	DeleteOldRDN bool
	// NewSuperior is the optional new parent DN (for moving entries).
	NewSuperior string
}

// ModifyDNBackend defines the interface for the directory backend's ModifyDN operation.
type ModifyDNBackend interface {
	// ModifyDN renames or moves an entry in the directory.
	ModifyDN(req *ModifyDNRequestData) error
}

// ModifyDN error types for mapping to LDAP result codes.
type ModifyDNError int

const (
	ModifyDNErrNone ModifyDNError = iota
	ModifyDNErrEntryNotFound
	ModifyDNErrEntryExists
	ModifyDNErrInvalidDN
	ModifyDNErrNewSuperiorNotFound
	ModifyDNErrNotAllowedOnNonLeaf
	ModifyDNErrAffectsMultipleDSAs
	ModifyDNErrOther
)

// ModifyDNConfig holds configuration for the ModifyDN handler.
type ModifyDNConfig struct {
	// Backend is the directory backend for ModifyDN operations.
	Backend ModifyDNBackend
	// ErrorMapper maps backend errors to ModifyDNError types.
	ErrorMapper func(error) ModifyDNError
}

// NewModifyDNConfig creates a new ModifyDNConfig with default settings.
func NewModifyDNConfig() *ModifyDNConfig {
	return &ModifyDNConfig{}
}

// ModifyDNHandlerImpl implements the ModifyDN operation handler.
type ModifyDNHandlerImpl struct {
	config *ModifyDNConfig
}

// NewModifyDNHandler creates a new ModifyDN handler with the given configuration.
func NewModifyDNHandler(config *ModifyDNConfig) *ModifyDNHandlerImpl {
	if config == nil {
		config = NewModifyDNConfig()
	}
	return &ModifyDNHandlerImpl{
		config: config,
	}
}

// Handle processes a ModifyDN request and returns the result.
func (h *ModifyDNHandlerImpl) Handle(conn *Connection, req *ldap.ModifyDNRequest) *OperationResult {
	// Validate the request
	if err := req.Validate(); err != nil {
		return &OperationResult{
			ResultCode:        ldap.ResultProtocolError,
			DiagnosticMessage: err.Error(),
		}
	}

	// Check if backend is configured
	if h.config.Backend == nil {
		return &OperationResult{
			ResultCode:        ldap.ResultOperationsError,
			DiagnosticMessage: "backend not configured",
		}
	}

	// Create the backend request
	backendReq := &ModifyDNRequestData{
		DN:           req.Entry,
		NewRDN:       req.NewRDN,
		DeleteOldRDN: req.DeleteOldRDN,
		NewSuperior:  req.NewSuperior,
	}

	// Execute the ModifyDN operation
	err := h.config.Backend.ModifyDN(backendReq)
	if err != nil {
		return h.mapError(err, req.Entry)
	}

	return &OperationResult{
		ResultCode: ldap.ResultSuccess,
	}
}

// mapError maps backend errors to LDAP result codes.
func (h *ModifyDNHandlerImpl) mapError(err error, dn string) *OperationResult {
	// Use custom error mapper if provided
	if h.config.ErrorMapper != nil {
		errType := h.config.ErrorMapper(err)
		return h.mapErrorType(errType, dn, err.Error())
	}

	// Default: return operations error
	return &OperationResult{
		ResultCode:        ldap.ResultOperationsError,
		DiagnosticMessage: "internal error: " + err.Error(),
	}
}

// mapErrorType maps ModifyDNError types to LDAP result codes.
func (h *ModifyDNHandlerImpl) mapErrorType(errType ModifyDNError, dn string, msg string) *OperationResult {
	switch errType {
	case ModifyDNErrEntryNotFound:
		return &OperationResult{
			ResultCode:        ldap.ResultNoSuchObject,
			MatchedDN:         "",
			DiagnosticMessage: "entry not found",
		}
	case ModifyDNErrEntryExists:
		return &OperationResult{
			ResultCode:        ldap.ResultEntryAlreadyExists,
			DiagnosticMessage: "new entry already exists",
		}
	case ModifyDNErrInvalidDN:
		return &OperationResult{
			ResultCode:        ldap.ResultInvalidDNSyntax,
			DiagnosticMessage: "invalid DN syntax",
		}
	case ModifyDNErrNewSuperiorNotFound:
		return &OperationResult{
			ResultCode:        ldap.ResultNoSuchObject,
			DiagnosticMessage: "new superior not found",
		}
	case ModifyDNErrNotAllowedOnNonLeaf:
		return &OperationResult{
			ResultCode:        ldap.ResultNotAllowedOnNonLeaf,
			DiagnosticMessage: "operation not allowed on non-leaf entry",
		}
	case ModifyDNErrAffectsMultipleDSAs:
		return &OperationResult{
			ResultCode:        ldap.ResultAffectsMultipleDSAs,
			DiagnosticMessage: "operation affects multiple DSAs",
		}
	default:
		return &OperationResult{
			ResultCode:        ldap.ResultOperationsError,
			DiagnosticMessage: "internal error: " + msg,
		}
	}
}

// CreateModifyDNHandler creates a ModifyDNHandler function from a ModifyDNHandlerImpl.
func CreateModifyDNHandler(impl *ModifyDNHandlerImpl) ModifyDNHandler {
	return func(conn *Connection, req *ldap.ModifyDNRequest) *OperationResult {
		return impl.Handle(conn, req)
	}
}
