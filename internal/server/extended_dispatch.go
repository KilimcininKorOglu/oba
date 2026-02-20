// Package server provides the LDAP server implementation.
package server

import (
	"errors"
	"sort"
	"sync"

	"github.com/oba-ldap/oba/internal/ldap"
)

// Extended operation errors
var (
	// ErrUnknownOID is returned when no handler is registered for the requested OID.
	ErrUnknownOID = errors.New("server: unknown extended operation OID")
	// ErrNilHandler is returned when attempting to register a nil handler.
	ErrNilHandler = errors.New("server: cannot register nil handler")
	// ErrEmptyOID is returned when attempting to register a handler with an empty OID.
	ErrEmptyOID = errors.New("server: cannot register handler with empty OID")
)

// ExtendedDispatcher manages extended operation handlers and routes
// requests to the appropriate handler based on OID.
type ExtendedDispatcher struct {
	// handlers maps OIDs to their handlers
	handlers map[string]ExtendedHandler
	// mu protects concurrent access to handlers
	mu sync.RWMutex
}

// NewExtendedDispatcher creates a new ExtendedDispatcher.
func NewExtendedDispatcher() *ExtendedDispatcher {
	return &ExtendedDispatcher{
		handlers: make(map[string]ExtendedHandler),
	}
}

// Register registers an extended operation handler.
// The handler's OID() method is used to determine which OID it handles.
// If a handler is already registered for the OID, it will be replaced.
// Returns an error if the handler is nil or has an empty OID.
func (d *ExtendedDispatcher) Register(handler ExtendedHandler) error {
	if handler == nil {
		return ErrNilHandler
	}

	oid := handler.OID()
	if oid == "" {
		return ErrEmptyOID
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	d.handlers[oid] = handler
	return nil
}

// Unregister removes the handler for the specified OID.
// Returns true if a handler was removed, false if no handler was registered.
func (d *ExtendedDispatcher) Unregister(oid string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	if _, exists := d.handlers[oid]; exists {
		delete(d.handlers, oid)
		return true
	}
	return false
}

// Handle dispatches an extended request to the appropriate handler.
// Returns an error response if no handler is registered for the OID.
func (d *ExtendedDispatcher) Handle(conn *Connection, req *ExtendedRequest) (*ExtendedResponse, error) {
	d.mu.RLock()
	handler, exists := d.handlers[req.OID]
	d.mu.RUnlock()

	if !exists {
		return &ExtendedResponse{
			Result: OperationResult{
				ResultCode:        ldap.ResultProtocolError,
				DiagnosticMessage: "unsupported extended operation: " + req.OID,
			},
		}, ErrUnknownOID
	}

	return handler.Handle(conn, req)
}

// HasHandler returns true if a handler is registered for the specified OID.
func (d *ExtendedDispatcher) HasHandler(oid string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()

	_, exists := d.handlers[oid]
	return exists
}

// SupportedOIDs returns a sorted list of all registered OIDs.
func (d *ExtendedDispatcher) SupportedOIDs() []string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	oids := make([]string, 0, len(d.handlers))
	for oid := range d.handlers {
		oids = append(oids, oid)
	}
	sort.Strings(oids)
	return oids
}

// HandlerCount returns the number of registered handlers.
func (d *ExtendedDispatcher) HandlerCount() int {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return len(d.handlers)
}

// funcHandler wraps a function as an ExtendedHandler.
type funcHandler struct {
	oid     string
	handler ExtendedHandlerFunc
}

// OID returns the OID for this handler.
func (h *funcHandler) OID() string {
	return h.oid
}

// Handle processes the extended request.
func (h *funcHandler) Handle(conn *Connection, req *ExtendedRequest) (*ExtendedResponse, error) {
	return h.handler(conn, req)
}

// RegisterFunc registers a function as an extended operation handler.
// This is a convenience method for registering simple handlers without
// implementing the full ExtendedHandler interface.
func (d *ExtendedDispatcher) RegisterFunc(oid string, handler ExtendedHandlerFunc) error {
	if handler == nil {
		return ErrNilHandler
	}
	if oid == "" {
		return ErrEmptyOID
	}

	return d.Register(&funcHandler{
		oid:     oid,
		handler: handler,
	})
}
