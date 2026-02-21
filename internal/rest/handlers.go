package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/oba-ldap/oba/internal/acl"
	"github.com/oba-ldap/oba/internal/backend"
	"github.com/oba-ldap/oba/internal/config"
	"github.com/oba-ldap/oba/internal/ldap"
	"github.com/oba-ldap/oba/internal/logging"
)

// Handlers contains all REST API handlers.
type Handlers struct {
	backend       *backend.ObaBackend
	auth          *Authenticator
	aclManager    *acl.Manager
	configManager *config.ConfigManager
	logger        logging.Logger
	startTime     time.Time
	requestCount  int64
	activeConns   int64
}

// NewHandlers creates new handlers.
func NewHandlers(be *backend.ObaBackend, auth *Authenticator) *Handlers {
	return &Handlers{
		backend:   be,
		auth:      auth,
		startTime: time.Now(),
	}
}

// SetACLManager sets the ACL manager for ACL-related endpoints.
func (h *Handlers) SetACLManager(m *acl.Manager) {
	h.aclManager = m
}

// SetConfigManager sets the config manager for config-related endpoints.
func (h *Handlers) SetConfigManager(m *config.ConfigManager) {
	h.configManager = m
}

// IncrementConnections increments active connection count.
func (h *Handlers) IncrementConnections() {
	atomic.AddInt64(&h.activeConns, 1)
}

// DecrementConnections decrements active connection count.
func (h *Handlers) DecrementConnections() {
	atomic.AddInt64(&h.activeConns, -1)
}

// HandleBind handles POST /api/v1/auth/bind
func (h *Handlers) HandleBind(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&h.requestCount, 1)

	var req BindRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}

	token, err := h.auth.Authenticate(req.DN, req.Password)
	if err != nil {
		if err == backend.ErrInvalidCredentials {
			writeError(w, http.StatusUnauthorized, "invalid_credentials", "invalid DN or password")
			return
		}
		writeError(w, http.StatusInternalServerError, "auth_error", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, BindResponse{
		Success: true,
		Token:   token,
	})
}

// HandleGetEntry handles GET /api/v1/entries/{dn}
func (h *Handlers) HandleGetEntry(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&h.requestCount, 1)

	dn := Param(r, "dn")
	if dn == "" {
		writeError(w, http.StatusBadRequest, "missing_dn", "DN is required")
		return
	}

	decodedDN, err := url.PathUnescape(dn)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_dn", "invalid DN encoding")
		return
	}

	entries, err := h.backend.Search(decodedDN, int(ldap.ScopeBaseObject), nil)
	if err != nil {
		status, code, msg := mapBackendError(err)
		writeError(w, status, code, msg)
		return
	}

	if len(entries) == 0 {
		writeError(w, http.StatusNotFound, "not_found", "entry not found")
		return
	}

	writeJSON(w, http.StatusOK, convertEntry(entries[0]))
}

// HandleSearch handles GET /api/v1/search
func (h *Handlers) HandleSearch(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&h.requestCount, 1)
	query := r.URL.Query()

	baseDN := query.Get("baseDN")
	if baseDN == "" {
		writeError(w, http.StatusBadRequest, "missing_base_dn", "baseDN is required")
		return
	}

	scopeStr := query.Get("scope")
	scope := ldap.ScopeWholeSubtree
	switch scopeStr {
	case "base":
		scope = ldap.ScopeBaseObject
	case "one":
		scope = ldap.ScopeSingleLevel
	case "sub", "":
		scope = ldap.ScopeWholeSubtree
	default:
		writeError(w, http.StatusBadRequest, "invalid_scope", "scope must be base, one, or sub")
		return
	}

	// Note: filter parameter is accepted but not yet implemented
	// filterStr := query.Get("filter")

	offset := 0
	if o := query.Get("offset"); o != "" {
		offset, _ = strconv.Atoi(o)
		if offset < 0 {
			offset = 0
		}
	}

	limit := 0
	if l := query.Get("limit"); l != "" {
		limit, _ = strconv.Atoi(l)
		if limit < 0 {
			limit = 0
		}
	}

	var requestedAttrs []string
	if attrs := query.Get("attributes"); attrs != "" {
		requestedAttrs = strings.Split(attrs, ",")
		for i := range requestedAttrs {
			requestedAttrs[i] = strings.TrimSpace(requestedAttrs[i])
		}
	}

	timeLimit := 0
	if tl := query.Get("timeLimit"); tl != "" {
		timeLimit, _ = strconv.Atoi(tl)
	}

	ctx := r.Context()
	if timeLimit > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeLimit)*time.Second)
		defer cancel()
	}

	searchDone := make(chan struct{})
	var entries []*backend.Entry
	var searchErr error

	go func() {
		entries, searchErr = h.backend.Search(baseDN, int(scope), nil)
		close(searchDone)
	}()

	select {
	case <-ctx.Done():
		writeError(w, http.StatusRequestTimeout, "time_limit_exceeded", "search time limit exceeded")
		return
	case <-searchDone:
		if searchErr != nil {
			status, code, msg := mapBackendError(searchErr)
			writeError(w, status, code, msg)
			return
		}
	}

	totalCount := len(entries)

	hasMore := false
	if offset > 0 {
		if offset >= len(entries) {
			entries = nil
		} else {
			entries = entries[offset:]
		}
	}
	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
		hasMore = true
	}

	result := make([]*Entry, len(entries))
	for i, e := range entries {
		result[i] = convertEntryWithAttrs(e, requestedAttrs)
	}

	writeJSON(w, http.StatusOK, SearchResponse{
		Entries:    result,
		TotalCount: totalCount,
		Offset:     offset,
		Limit:      limit,
		HasMore:    hasMore,
	})
}

// HandleAddEntry handles POST /api/v1/entries
func (h *Handlers) HandleAddEntry(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&h.requestCount, 1)

	var req AddRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}

	if req.DN == "" {
		writeError(w, http.StatusBadRequest, "missing_dn", "DN is required")
		return
	}

	entry := &backend.Entry{
		DN:         req.DN,
		Attributes: req.Attributes,
	}

	bindDN := BindDN(r)
	err := h.backend.AddWithBindDN(entry, bindDN)
	if err != nil {
		status, code, msg := mapBackendError(err)
		writeError(w, status, code, msg)
		return
	}

	w.Header().Set("Location", "/api/v1/entries/"+url.PathEscape(req.DN))
	writeJSON(w, http.StatusCreated, convertEntry(entry))
}

// HandleModifyEntry handles PUT/PATCH /api/v1/entries/{dn}
func (h *Handlers) HandleModifyEntry(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&h.requestCount, 1)

	dn := Param(r, "dn")
	if dn == "" {
		writeError(w, http.StatusBadRequest, "missing_dn", "DN is required")
		return
	}

	decodedDN, err := url.PathUnescape(dn)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_dn", "invalid DN encoding")
		return
	}

	var req ModifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}

	changes := make([]backend.Modification, len(req.Changes))
	for i, c := range req.Changes {
		var modType backend.ModificationType
		switch c.Operation {
		case "add":
			modType = backend.ModAdd
		case "delete":
			modType = backend.ModDelete
		case "replace":
			modType = backend.ModReplace
		default:
			writeError(w, http.StatusBadRequest, "invalid_operation", "operation must be add, delete, or replace")
			return
		}

		changes[i] = backend.Modification{
			Type:      modType,
			Attribute: c.Attribute,
			Values:    c.Values,
		}
	}

	bindDN := BindDN(r)
	err = h.backend.ModifyWithBindDN(decodedDN, changes, bindDN)
	if err != nil {
		status, code, msg := mapBackendError(err)
		writeError(w, status, code, msg)
		return
	}

	entries, _ := h.backend.Search(decodedDN, int(ldap.ScopeBaseObject), nil)
	if len(entries) > 0 {
		writeJSON(w, http.StatusOK, convertEntry(entries[0]))
	} else {
		w.WriteHeader(http.StatusNoContent)
	}
}

// HandleDeleteEntry handles DELETE /api/v1/entries/{dn}
func (h *Handlers) HandleDeleteEntry(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&h.requestCount, 1)

	dn := Param(r, "dn")
	if dn == "" {
		writeError(w, http.StatusBadRequest, "missing_dn", "DN is required")
		return
	}

	decodedDN, err := url.PathUnescape(dn)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_dn", "invalid DN encoding")
		return
	}

	err = h.backend.Delete(decodedDN)
	if err != nil {
		status, code, msg := mapBackendError(err)
		writeError(w, status, code, msg)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// HandleModifyDN handles POST /api/v1/entries/{dn}/move
func (h *Handlers) HandleModifyDN(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&h.requestCount, 1)

	dn := Param(r, "dn")
	if dn == "" {
		writeError(w, http.StatusBadRequest, "missing_dn", "DN is required")
		return
	}

	decodedDN, err := url.PathUnescape(dn)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_dn", "invalid DN encoding")
		return
	}

	var req ModifyDNRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}

	if req.NewRDN == "" {
		writeError(w, http.StatusBadRequest, "missing_new_rdn", "newRDN is required")
		return
	}

	entries, err := h.backend.Search(decodedDN, int(ldap.ScopeBaseObject), nil)
	if err != nil || len(entries) == 0 {
		writeError(w, http.StatusNotFound, "not_found", "entry not found")
		return
	}
	oldEntry := entries[0]

	newDN := calculateNewDN(decodedDN, req.NewRDN, req.NewSuperior)

	existingEntries, _ := h.backend.Search(newDN, int(ldap.ScopeBaseObject), nil)
	if len(existingEntries) > 0 {
		writeError(w, http.StatusConflict, "entry_exists", "entry with new DN already exists")
		return
	}

	newEntry := &backend.Entry{
		DN:         newDN,
		Attributes: copyAttributes(oldEntry.Attributes),
	}

	if req.DeleteOldRDN {
		rdnAttr, rdnValue := parseRDN(req.NewRDN)
		if rdnAttr != "" {
			newEntry.Attributes[rdnAttr] = []string{rdnValue}
		}
	}

	bindDN := BindDN(r)
	if err := h.backend.AddWithBindDN(newEntry, bindDN); err != nil {
		status, code, msg := mapBackendError(err)
		writeError(w, status, code, msg)
		return
	}

	if err := h.backend.Delete(decodedDN); err != nil {
		h.backend.Delete(newDN)
		status, code, msg := mapBackendError(err)
		writeError(w, status, code, msg)
		return
	}

	w.Header().Set("Location", "/api/v1/entries/"+url.PathEscape(newDN))
	writeJSON(w, http.StatusOK, convertEntry(newEntry))
}

// HandleCompare handles POST /api/v1/compare
func (h *Handlers) HandleCompare(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&h.requestCount, 1)

	var req CompareRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}

	entries, err := h.backend.Search(req.DN, int(ldap.ScopeBaseObject), nil)
	if err != nil || len(entries) == 0 {
		writeError(w, http.StatusNotFound, "not_found", "entry not found")
		return
	}

	entry := entries[0]
	values := entry.GetAttribute(req.Attribute)
	match := false
	for _, v := range values {
		if v == req.Value {
			match = true
			break
		}
	}

	writeJSON(w, http.StatusOK, CompareResponse{Match: match})
}

// HandleHealth handles GET /api/v1/health
func (h *Handlers) HandleHealth(w http.ResponseWriter, r *http.Request) {
	uptime := time.Since(h.startTime)

	writeJSON(w, http.StatusOK, HealthResponse{
		Status:      "ok",
		Version:     "1.0.0",
		Uptime:      uptime.String(),
		UptimeSecs:  int64(uptime.Seconds()),
		StartTime:   h.startTime,
		Connections: int(atomic.LoadInt64(&h.activeConns)),
		Requests:    atomic.LoadInt64(&h.requestCount),
	})
}

// HandleBulk handles POST /api/v1/bulk
func (h *Handlers) HandleBulk(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&h.requestCount, 1)

	var req BulkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}

	if len(req.Operations) == 0 {
		writeError(w, http.StatusBadRequest, "empty_operations", "at least one operation is required")
		return
	}

	bindDN := BindDN(r)
	results := make([]BulkOperationResult, len(req.Operations))
	succeeded := 0
	failed := 0

	for i, op := range req.Operations {
		result := BulkOperationResult{
			Index:     i,
			DN:        op.DN,
			Operation: op.Operation,
		}

		var err error
		switch op.Operation {
		case "add":
			entry := &backend.Entry{
				DN:         op.DN,
				Attributes: op.Attributes,
			}
			err = h.backend.AddWithBindDN(entry, bindDN)

		case "modify":
			changes := make([]backend.Modification, len(op.Changes))
			for j, c := range op.Changes {
				var modType backend.ModificationType
				switch c.Operation {
				case "add":
					modType = backend.ModAdd
				case "delete":
					modType = backend.ModDelete
				case "replace":
					modType = backend.ModReplace
				}
				changes[j] = backend.Modification{
					Type:      modType,
					Attribute: c.Attribute,
					Values:    c.Values,
				}
			}
			err = h.backend.ModifyWithBindDN(op.DN, changes, bindDN)

		case "delete":
			err = h.backend.Delete(op.DN)

		default:
			err = fmt.Errorf("unknown operation: %s", op.Operation)
		}

		if err != nil {
			result.Success = false
			result.Error = err.Error()
			result.ResultCode = ldapResultCodeFromError(err)
			failed++

			if req.StopOnError {
				results[i] = result
				for j := i + 1; j < len(req.Operations); j++ {
					results[j] = BulkOperationResult{
						Index:     j,
						DN:        req.Operations[j].DN,
						Operation: req.Operations[j].Operation,
						Success:   false,
						Error:     "skipped due to previous error",
					}
				}
				break
			}
		} else {
			result.Success = true
			succeeded++
		}

		results[i] = result
	}

	status := http.StatusOK
	if failed > 0 && succeeded == 0 {
		status = http.StatusBadRequest
	} else if failed > 0 {
		status = http.StatusMultiStatus
	}

	writeJSON(w, status, BulkResponse{
		Success:    failed == 0,
		TotalCount: len(req.Operations),
		Succeeded:  succeeded,
		Failed:     failed,
		Results:    results,
	})
}

// HandleStreamSearch handles GET /api/v1/search/stream
func (h *Handlers) HandleStreamSearch(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&h.requestCount, 1)
	query := r.URL.Query()

	baseDN := query.Get("baseDN")
	if baseDN == "" {
		writeError(w, http.StatusBadRequest, "missing_base_dn", "baseDN is required")
		return
	}

	scopeStr := query.Get("scope")
	scope := ldap.ScopeWholeSubtree
	switch scopeStr {
	case "base":
		scope = ldap.ScopeBaseObject
	case "one":
		scope = ldap.ScopeSingleLevel
	}

	// Note: filter parameter is accepted but not yet implemented
	// filterStr := query.Get("filter")

	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)

	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	entries, err := h.backend.Search(baseDN, int(scope), nil)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	encoder := json.NewEncoder(w)
	for _, e := range entries {
		entry := convertEntry(e)
		if err := encoder.Encode(entry); err != nil {
			return
		}
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}

	encoder.Encode(map[string]interface{}{"done": true, "count": len(entries)})
}

// Helper functions

func convertEntry(e *backend.Entry) *Entry {
	return &Entry{
		DN:         e.DN,
		Attributes: e.Attributes,
	}
}

func convertEntryWithAttrs(e *backend.Entry, attrs []string) *Entry {
	if len(attrs) == 0 {
		return convertEntry(e)
	}

	filtered := make(map[string][]string)
	for _, attr := range attrs {
		attrLower := strings.ToLower(attr)
		for k, v := range e.Attributes {
			if strings.ToLower(k) == attrLower {
				filtered[k] = v
				break
			}
		}
	}

	return &Entry{
		DN:         e.DN,
		Attributes: filtered,
	}
}

func calculateNewDN(oldDN, newRDN, newSuperior string) string {
	if newSuperior != "" {
		return newRDN + "," + newSuperior
	}
	parts := strings.SplitN(oldDN, ",", 2)
	if len(parts) == 2 {
		return newRDN + "," + parts[1]
	}
	return newRDN
}

func parseRDN(rdn string) (string, string) {
	parts := strings.SplitN(rdn, "=", 2)
	if len(parts) == 2 {
		return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	}
	return "", ""
}

func copyAttributes(attrs map[string][]string) map[string][]string {
	result := make(map[string][]string)
	for k, v := range attrs {
		copied := make([]string, len(v))
		copy(copied, v)
		result[k] = copied
	}
	return result
}
