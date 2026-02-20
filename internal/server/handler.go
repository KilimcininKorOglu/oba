// Package server provides the LDAP server implementation.
package server

import (
	"github.com/oba-ldap/oba/internal/ldap"
)

// OperationResult represents the result of an LDAP operation.
type OperationResult struct {
	// ResultCode is the LDAP result code
	ResultCode ldap.ResultCode
	// MatchedDN is the matched DN (for certain error conditions)
	MatchedDN string
	// DiagnosticMessage is an optional diagnostic message
	DiagnosticMessage string
}

// SearchEntry represents a single search result entry.
type SearchEntry struct {
	// DN is the distinguished name of the entry
	DN string
	// Attributes contains the entry's attributes
	Attributes []ldap.Attribute
}

// SearchResult represents the result of a search operation.
type SearchResult struct {
	// OperationResult contains the result code and messages
	OperationResult
	// Entries contains the search result entries
	Entries []*SearchEntry
}

// BindHandler handles bind requests.
type BindHandler func(conn *Connection, req *ldap.BindRequest) *OperationResult

// SearchHandler handles search requests.
type SearchHandler func(conn *Connection, req *ldap.SearchRequest) *SearchResult

// AddHandler handles add requests.
type AddHandler func(conn *Connection, req *ldap.AddRequest) *OperationResult

// DeleteHandler handles delete requests.
type DeleteHandler func(conn *Connection, req *ldap.DeleteRequest) *OperationResult

// ModifyHandler handles modify requests.
type ModifyHandler func(conn *Connection, req *ldap.ModifyRequest) *OperationResult

// Handler manages operation handlers for the LDAP server.
type Handler struct {
	// bindHandler handles bind requests
	bindHandler BindHandler
	// searchHandler handles search requests
	searchHandler SearchHandler
	// addHandler handles add requests
	addHandler AddHandler
	// deleteHandler handles delete requests
	deleteHandler DeleteHandler
	// modifyHandler handles modify requests
	modifyHandler ModifyHandler
}

// NewHandler creates a new Handler with default handlers.
func NewHandler() *Handler {
	return &Handler{
		bindHandler:   defaultBindHandler,
		searchHandler: defaultSearchHandler,
		addHandler:    defaultAddHandler,
		deleteHandler: defaultDeleteHandler,
		modifyHandler: defaultModifyHandler,
	}
}

// SetBindHandler sets the bind handler.
func (h *Handler) SetBindHandler(handler BindHandler) {
	h.bindHandler = handler
}

// SetSearchHandler sets the search handler.
func (h *Handler) SetSearchHandler(handler SearchHandler) {
	h.searchHandler = handler
}

// SetAddHandler sets the add handler.
func (h *Handler) SetAddHandler(handler AddHandler) {
	h.addHandler = handler
}

// SetDeleteHandler sets the delete handler.
func (h *Handler) SetDeleteHandler(handler DeleteHandler) {
	h.deleteHandler = handler
}

// SetModifyHandler sets the modify handler.
func (h *Handler) SetModifyHandler(handler ModifyHandler) {
	h.modifyHandler = handler
}

// HandleBind handles a bind request.
func (h *Handler) HandleBind(conn *Connection, req *ldap.BindRequest) *OperationResult {
	if h.bindHandler == nil {
		return &OperationResult{
			ResultCode:        ldap.ResultUnwillingToPerform,
			DiagnosticMessage: "bind handler not configured",
		}
	}
	return h.bindHandler(conn, req)
}

// HandleSearch handles a search request.
func (h *Handler) HandleSearch(conn *Connection, req *ldap.SearchRequest) *SearchResult {
	if h.searchHandler == nil {
		return &SearchResult{
			OperationResult: OperationResult{
				ResultCode:        ldap.ResultUnwillingToPerform,
				DiagnosticMessage: "search handler not configured",
			},
		}
	}
	return h.searchHandler(conn, req)
}

// HandleAdd handles an add request.
func (h *Handler) HandleAdd(conn *Connection, req *ldap.AddRequest) *OperationResult {
	if h.addHandler == nil {
		return &OperationResult{
			ResultCode:        ldap.ResultUnwillingToPerform,
			DiagnosticMessage: "add handler not configured",
		}
	}
	return h.addHandler(conn, req)
}

// HandleDelete handles a delete request.
func (h *Handler) HandleDelete(conn *Connection, req *ldap.DeleteRequest) *OperationResult {
	if h.deleteHandler == nil {
		return &OperationResult{
			ResultCode:        ldap.ResultUnwillingToPerform,
			DiagnosticMessage: "delete handler not configured",
		}
	}
	return h.deleteHandler(conn, req)
}

// HandleModify handles a modify request.
func (h *Handler) HandleModify(conn *Connection, req *ldap.ModifyRequest) *OperationResult {
	if h.modifyHandler == nil {
		return &OperationResult{
			ResultCode:        ldap.ResultUnwillingToPerform,
			DiagnosticMessage: "modify handler not configured",
		}
	}
	return h.modifyHandler(conn, req)
}

// Default handlers that return "unwilling to perform"

func defaultBindHandler(_ *Connection, req *ldap.BindRequest) *OperationResult {
	// Allow anonymous binds by default
	if req.IsAnonymous() {
		return &OperationResult{
			ResultCode: ldap.ResultSuccess,
		}
	}
	return &OperationResult{
		ResultCode:        ldap.ResultInvalidCredentials,
		DiagnosticMessage: "authentication not configured",
	}
}

func defaultSearchHandler(_ *Connection, _ *ldap.SearchRequest) *SearchResult {
	return &SearchResult{
		OperationResult: OperationResult{
			ResultCode:        ldap.ResultUnwillingToPerform,
			DiagnosticMessage: "search not implemented",
		},
	}
}

func defaultAddHandler(_ *Connection, _ *ldap.AddRequest) *OperationResult {
	return &OperationResult{
		ResultCode:        ldap.ResultUnwillingToPerform,
		DiagnosticMessage: "add not implemented",
	}
}

func defaultDeleteHandler(_ *Connection, _ *ldap.DeleteRequest) *OperationResult {
	return &OperationResult{
		ResultCode:        ldap.ResultUnwillingToPerform,
		DiagnosticMessage: "delete not implemented",
	}
}

func defaultModifyHandler(_ *Connection, _ *ldap.ModifyRequest) *OperationResult {
	return &OperationResult{
		ResultCode:        ldap.ResultUnwillingToPerform,
		DiagnosticMessage: "modify not implemented",
	}
}
