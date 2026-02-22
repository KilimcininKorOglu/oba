package rest

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/KilimcininKorOglu/oba/internal/backend"
	"github.com/KilimcininKorOglu/oba/internal/ldap"
)

// LDAP result code to HTTP status code mapping.
var ldapToHTTPStatus = map[ldap.ResultCode]int{
	ldap.ResultSuccess:                     http.StatusOK,
	ldap.ResultOperationsError:             http.StatusInternalServerError,
	ldap.ResultProtocolError:               http.StatusBadRequest,
	ldap.ResultTimeLimitExceeded:           http.StatusRequestTimeout,
	ldap.ResultSizeLimitExceeded:           http.StatusRequestEntityTooLarge,
	ldap.ResultAuthMethodNotSupported:      http.StatusNotImplemented,
	ldap.ResultStrongerAuthRequired:        http.StatusUnauthorized,
	ldap.ResultNoSuchAttribute:             http.StatusBadRequest,
	ldap.ResultUndefinedAttributeType:      http.StatusBadRequest,
	ldap.ResultInappropriateMatching:       http.StatusBadRequest,
	ldap.ResultConstraintViolation:         http.StatusBadRequest,
	ldap.ResultAttributeOrValueExists:      http.StatusConflict,
	ldap.ResultInvalidAttributeSyntax:      http.StatusBadRequest,
	ldap.ResultNoSuchObject:                http.StatusNotFound,
	ldap.ResultAliasProblem:                http.StatusBadRequest,
	ldap.ResultInvalidDNSyntax:             http.StatusBadRequest,
	ldap.ResultInappropriateAuthentication: http.StatusUnauthorized,
	ldap.ResultInvalidCredentials:          http.StatusUnauthorized,
	ldap.ResultInsufficientAccessRights:    http.StatusForbidden,
	ldap.ResultBusy:                        http.StatusServiceUnavailable,
	ldap.ResultUnavailable:                 http.StatusServiceUnavailable,
	ldap.ResultUnwillingToPerform:          http.StatusBadRequest,
	ldap.ResultLoopDetect:                  http.StatusLoopDetected,
	ldap.ResultNamingViolation:             http.StatusBadRequest,
	ldap.ResultObjectClassViolation:        http.StatusBadRequest,
	ldap.ResultNotAllowedOnNonLeaf:         http.StatusConflict,
	ldap.ResultNotAllowedOnRDN:             http.StatusBadRequest,
	ldap.ResultEntryAlreadyExists:          http.StatusConflict,
	ldap.ResultObjectClassModsProhibited:   http.StatusBadRequest,
	ldap.ResultOther:                       http.StatusInternalServerError,
}

// mapBackendError maps a backend error to HTTP status and error code.
func mapBackendError(err error) (int, string, string) {
	switch err {
	case backend.ErrInvalidCredentials:
		return http.StatusUnauthorized, "invalid_credentials", "invalid credentials"
	case backend.ErrEntryNotFound:
		return http.StatusNotFound, "not_found", "entry not found"
	case backend.ErrEntryExists:
		return http.StatusConflict, "entry_exists", "entry already exists"
	case backend.ErrInvalidDN:
		return http.StatusBadRequest, "invalid_dn", "invalid DN"
	case backend.ErrInvalidEntry:
		return http.StatusBadRequest, "invalid_entry", "invalid entry"
	case backend.ErrNoPassword:
		return http.StatusBadRequest, "no_password", "no password attribute"
	case backend.ErrStorageError:
		return http.StatusInternalServerError, "storage_error", "storage error"
	case backend.ErrNotAllowedOnNonLeaf:
		return http.StatusConflict, "not_allowed_on_non_leaf", "operation not allowed on non-leaf entry"
	default:
		if strings.Contains(strings.ToLower(err.Error()), "uid attribute") &&
			strings.Contains(strings.ToLower(err.Error()), "unique") {
			return http.StatusConflict, "uid_not_unique", "uid attribute value must be unique"
		}
		return http.StatusInternalServerError, "internal_error", err.Error()
	}
}

// mapLDAPResultCode maps an LDAP result code to HTTP status.
func mapLDAPResultCode(code ldap.ResultCode) int {
	if status, ok := ldapToHTTPStatus[code]; ok {
		return status
	}
	return http.StatusInternalServerError
}

// ldapResultCodeFromError returns LDAP result code from backend error.
func ldapResultCodeFromError(err error) int {
	switch err {
	case backend.ErrInvalidCredentials:
		return int(ldap.ResultInvalidCredentials)
	case backend.ErrEntryNotFound:
		return int(ldap.ResultNoSuchObject)
	case backend.ErrEntryExists:
		return int(ldap.ResultEntryAlreadyExists)
	case backend.ErrInvalidDN:
		return int(ldap.ResultInvalidDNSyntax)
	case backend.ErrNotAllowedOnNonLeaf:
		return int(ldap.ResultNotAllowedOnNonLeaf)
	default:
		return int(ldap.ResultOther)
	}
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{
		Error:   code,
		Code:    status,
		Message: message,
	})
}
