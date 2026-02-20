// Package server provides the LDAP server implementation.
package server

// UnbindRequest represents an LDAP Unbind request.
// Per RFC 4511 Section 4.3:
// UnbindRequest ::= [APPLICATION 2] NULL
//
// The Unbind operation has no fields - it's simply a signal to disconnect.
// The server does not send a response; it simply closes the connection.
type UnbindRequest struct {
	// Unbind has no fields - it's just a signal to disconnect
}

// UnbindHandler handles unbind requests.
// Per RFC 4511, the unbind operation does not return a response.
// The handler should clean up any connection resources and signal
// that the connection should be closed.
type UnbindHandler func(conn *Connection, req *UnbindRequest) error

// UnbindProcessor manages the unbind operation for a connection.
type UnbindProcessor struct {
	// handler is the custom unbind handler (optional)
	handler UnbindHandler
}

// NewUnbindProcessor creates a new UnbindProcessor with default behavior.
func NewUnbindProcessor() *UnbindProcessor {
	return &UnbindProcessor{
		handler: defaultUnbindHandler,
	}
}

// SetHandler sets a custom unbind handler.
func (p *UnbindProcessor) SetHandler(handler UnbindHandler) {
	p.handler = handler
}

// Handle processes an unbind request.
// It calls the configured handler to perform any cleanup,
// then returns to signal that the connection should be closed.
// Per RFC 4511, no response is sent for unbind requests.
func (p *UnbindProcessor) Handle(conn *Connection, req *UnbindRequest) error {
	if p.handler == nil {
		return defaultUnbindHandler(conn, req)
	}
	return p.handler(conn, req)
}

// defaultUnbindHandler is the default unbind handler.
// It performs basic cleanup by resetting the connection's authentication state.
func defaultUnbindHandler(conn *Connection, _ *UnbindRequest) error {
	if conn == nil {
		return nil
	}

	// Reset authentication state
	conn.mu.Lock()
	conn.bindDN = ""
	conn.authenticated = false
	conn.mu.Unlock()

	return nil
}

// ParseUnbindRequest parses an unbind request from raw BER data.
// Per RFC 4511, UnbindRequest is NULL (empty), so there's nothing to parse.
// This function exists for API consistency with other request parsers.
func ParseUnbindRequest(_ []byte) (*UnbindRequest, error) {
	// UnbindRequest has no content - it's just a NULL value
	return &UnbindRequest{}, nil
}
