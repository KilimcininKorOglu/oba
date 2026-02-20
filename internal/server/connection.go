// Package server provides the LDAP server implementation.
package server

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"sync"
	"time"

	"github.com/oba-ldap/oba/internal/ber"
	"github.com/oba-ldap/oba/internal/ldap"
	"github.com/oba-ldap/oba/internal/logging"
)

// Connection errors
var (
	// ErrConnectionClosed is returned when the connection is closed
	ErrConnectionClosed = errors.New("server: connection closed")
	// ErrInvalidMessage is returned when a message cannot be parsed
	ErrInvalidMessage = errors.New("server: invalid message")
	// ErrMessageTooLarge is returned when a message exceeds the size limit
	ErrMessageTooLarge = errors.New("server: message too large")
	// ErrTLSRequired is returned when TLS is required but not active
	ErrTLSRequired = errors.New("server: TLS required for this operation")
)

// MaxMessageSize is the maximum size of an LDAP message (16 MB)
const MaxMessageSize = 16 * 1024 * 1024

// Connection represents an individual client connection to the LDAP server.
// It manages the connection state, reads LDAP messages from the network,
// dispatches them to appropriate handlers, and sends responses back.
type Connection struct {
	// conn is the underlying network connection
	conn net.Conn
	// server is the parent server instance
	server *Server
	// bindDN is the currently bound DN (empty for anonymous)
	bindDN string
	// authenticated indicates whether the connection is authenticated
	authenticated bool
	// messageID tracks the last processed message ID
	messageID int
	// mu protects concurrent access to connection state
	mu sync.Mutex
	// closed indicates whether the connection has been closed
	closed bool
	// handler is the operation handler for this connection
	handler *Handler
	// logger is the logger for this connection
	logger logging.Logger
	// requestID is the unique identifier for this connection
	requestID string
	// startTime is when the connection was established
	startTime time.Time
	// isTLS indicates whether the connection is using TLS
	isTLS bool
	// tlsState holds the TLS connection state (nil if not TLS)
	tlsState *tls.ConnectionState
	// clientCert holds the client certificate if provided (nil if not provided)
	clientCert *x509.Certificate
}

// Server represents the LDAP server (placeholder for now).
// This will be fully implemented in a separate task.
type Server struct {
	// Handler is the default handler for operations
	Handler *Handler
	// Logger is the server's logger
	Logger logging.Logger
}

// NewConnection creates a new Connection for the given network connection.
func NewConnection(conn net.Conn, server *Server) *Connection {
	requestID := logging.GenerateRequestID()

	// Get logger from server or create a nop logger
	var logger logging.Logger
	if server != nil && server.Logger != nil {
		logger = server.Logger.WithRequestID(requestID)
	} else {
		logger = logging.NewNop()
	}

	c := &Connection{
		conn:          conn,
		server:        server,
		bindDN:        "",
		authenticated: false,
		messageID:     0,
		closed:        false,
		logger:        logger,
		requestID:     requestID,
		startTime:     time.Now(),
		isTLS:         false,
	}

	// Create a handler for this connection
	if server != nil && server.Handler != nil {
		c.handler = server.Handler
	} else {
		c.handler = NewHandler()
	}

	return c
}

// Handle is the main message loop for the connection.
// It reads LDAP messages, dispatches them to handlers, and sends responses.
// This method blocks until the connection is closed or an error occurs.
func (c *Connection) Handle() {
	// Log connection established
	c.logger.Info("connection established",
		"client", c.conn.RemoteAddr().String(),
		"tls", c.isTLS)

	defer func() {
		// Log connection closed with duration
		c.logger.Info("connection closed",
			"client", c.conn.RemoteAddr().String(),
			"duration_ms", time.Since(c.startTime).Milliseconds())
		c.Close()
	}()

	for {
		// Check if connection is closed
		c.mu.Lock()
		if c.closed {
			c.mu.Unlock()
			return
		}
		c.mu.Unlock()

		// Read the next message
		msg, err := c.ReadMessage()
		if err != nil {
			// Check for expected closure conditions
			if err == io.EOF || errors.Is(err, net.ErrClosed) || c.isClosed() {
				return
			}
			// Log error and continue or close based on severity
			if errors.Is(err, ErrMessageTooLarge) || errors.Is(err, ErrInvalidMessage) {
				// Protocol error - close connection
				c.logger.Warn("protocol error",
					"error", err.Error(),
					"client", c.conn.RemoteAddr().String())
				return
			}
			// Network error - close connection
			c.logger.Warn("network error",
				"error", err.Error(),
				"client", c.conn.RemoteAddr().String())
			return
		}

		// Handle unbind request specially - it closes the connection
		if msg.OperationType() == ldap.OperationType(ldap.ApplicationUnbindRequest) {
			c.logger.Debug("unbind request received",
				"message_id", msg.MessageID)
			return
		}

		// Dispatch the message to the appropriate handler
		response := c.dispatchMessage(msg)

		// Send response(s) if any
		if response != nil {
			if err := c.WriteMessage(response); err != nil {
				// Write error - close connection
				c.logger.Warn("write error",
					"error", err.Error(),
					"client", c.conn.RemoteAddr().String())
				return
			}
		}
	}
}

// dispatchMessage dispatches a message to the appropriate handler.
// It returns the response message(s) to send back to the client.
func (c *Connection) dispatchMessage(msg *ldap.LDAPMessage) *ldap.LDAPMessage {
	if c.handler == nil {
		return c.createErrorResponse(msg.MessageID, ldap.ResultUnwillingToPerform, "no handler configured")
	}

	// Dispatch based on operation type
	switch msg.OperationType() {
	case ldap.OperationType(ldap.ApplicationBindRequest):
		return c.handleBind(msg)
	case ldap.OperationType(ldap.ApplicationSearchRequest):
		return c.handleSearch(msg)
	case ldap.OperationType(ldap.ApplicationAddRequest):
		return c.handleAdd(msg)
	case ldap.OperationType(ldap.ApplicationDelRequest):
		return c.handleDelete(msg)
	case ldap.OperationType(ldap.ApplicationModifyRequest):
		return c.handleModify(msg)
	case ldap.OperationType(ldap.ApplicationModifyDNRequest):
		return c.handleModifyDN(msg)
	case ldap.OperationType(ldap.ApplicationCompareRequest):
		return c.handleCompare(msg)
	case ldap.OperationType(ldap.ApplicationAbandonRequest):
		// Abandon requests don't get a response
		return nil
	default:
		return c.createErrorResponse(msg.MessageID, ldap.ResultProtocolError, "unsupported operation")
	}
}

// handleBind handles a bind request.
func (c *Connection) handleBind(msg *ldap.LDAPMessage) *ldap.LDAPMessage {
	start := time.Now()

	req, err := ldap.ParseBindRequest(msg.Operation.Data)
	if err != nil {
		c.logger.Warn("bind request parse error",
			"error", err.Error(),
			"message_id", msg.MessageID)
		return c.createBindResponse(msg.MessageID, ldap.ResultProtocolError, "", "invalid bind request")
	}

	c.logger.Debug("bind request",
		"dn", req.Name,
		"version", req.Version,
		"auth_method", req.AuthMethod.String(),
		"message_id", msg.MessageID)

	// Call the handler
	result := c.handler.HandleBind(c, req)

	// Update connection state on successful bind
	if result.ResultCode == ldap.ResultSuccess {
		c.mu.Lock()
		c.bindDN = req.Name
		c.authenticated = !req.IsAnonymous()
		c.mu.Unlock()

		c.logger.Info("bind successful",
			"dn", req.Name,
			"duration_ms", time.Since(start).Milliseconds())
	} else {
		c.logger.Warn("bind failed",
			"dn", req.Name,
			"result_code", result.ResultCode.String(),
			"error", result.DiagnosticMessage,
			"duration_ms", time.Since(start).Milliseconds())
	}

	return c.createBindResponse(msg.MessageID, result.ResultCode, result.MatchedDN, result.DiagnosticMessage)
}

// handleSearch handles a search request.
func (c *Connection) handleSearch(msg *ldap.LDAPMessage) *ldap.LDAPMessage {
	start := time.Now()

	req, err := ldap.ParseSearchRequest(msg.Operation.Data)
	if err != nil {
		c.logger.Warn("search request parse error",
			"error", err.Error(),
			"message_id", msg.MessageID)
		return c.createSearchDoneResponse(msg.MessageID, ldap.ResultProtocolError, "", "invalid search request")
	}

	c.logger.Debug("search request",
		"base_dn", req.BaseObject,
		"scope", req.Scope.String(),
		"size_limit", req.SizeLimit,
		"time_limit", req.TimeLimit,
		"message_id", msg.MessageID)

	// Call the handler
	result := c.handler.HandleSearch(c, req)

	// Send search result entries first
	for _, entry := range result.Entries {
		entryMsg := c.createSearchEntryResponse(msg.MessageID, entry)
		if err := c.WriteMessage(entryMsg); err != nil {
			c.logger.Warn("search entry write error",
				"error", err.Error(),
				"base_dn", req.BaseObject)
			return nil // Connection error, will be handled by caller
		}
	}

	// Log search completion
	if result.ResultCode == ldap.ResultSuccess {
		c.logger.Info("search completed",
			"base_dn", req.BaseObject,
			"scope", req.Scope.String(),
			"results", len(result.Entries),
			"duration_ms", time.Since(start).Milliseconds())
	} else {
		c.logger.Warn("search failed",
			"base_dn", req.BaseObject,
			"scope", req.Scope.String(),
			"result_code", result.ResultCode.String(),
			"error", result.DiagnosticMessage,
			"duration_ms", time.Since(start).Milliseconds())
	}

	// Return the search done response
	return c.createSearchDoneResponse(msg.MessageID, result.ResultCode, result.MatchedDN, result.DiagnosticMessage)
}

// handleAdd handles an add request.
func (c *Connection) handleAdd(msg *ldap.LDAPMessage) *ldap.LDAPMessage {
	start := time.Now()

	req, err := ldap.ParseAddRequest(msg.Operation.Data)
	if err != nil {
		c.logger.Warn("add request parse error",
			"error", err.Error(),
			"message_id", msg.MessageID)
		return c.createAddResponse(msg.MessageID, ldap.ResultProtocolError, "", "invalid add request")
	}

	c.logger.Debug("add request",
		"entry", req.Entry,
		"attributes_count", len(req.Attributes),
		"message_id", msg.MessageID)

	// Call the handler
	result := c.handler.HandleAdd(c, req)

	// Log result
	if result.ResultCode == ldap.ResultSuccess {
		c.logger.Info("add successful",
			"entry", req.Entry,
			"duration_ms", time.Since(start).Milliseconds())
	} else {
		c.logger.Warn("add failed",
			"entry", req.Entry,
			"result_code", result.ResultCode.String(),
			"error", result.DiagnosticMessage,
			"duration_ms", time.Since(start).Milliseconds())
	}

	return c.createAddResponse(msg.MessageID, result.ResultCode, result.MatchedDN, result.DiagnosticMessage)
}

// handleDelete handles a delete request.
func (c *Connection) handleDelete(msg *ldap.LDAPMessage) *ldap.LDAPMessage {
	start := time.Now()

	req, err := ldap.ParseDeleteRequest(msg.Operation.Data)
	if err != nil {
		c.logger.Warn("delete request parse error",
			"error", err.Error(),
			"message_id", msg.MessageID)
		return c.createDeleteResponse(msg.MessageID, ldap.ResultProtocolError, "", "invalid delete request")
	}

	c.logger.Debug("delete request",
		"dn", req.DN,
		"message_id", msg.MessageID)

	// Call the handler
	result := c.handler.HandleDelete(c, req)

	// Log result
	if result.ResultCode == ldap.ResultSuccess {
		c.logger.Info("delete successful",
			"dn", req.DN,
			"duration_ms", time.Since(start).Milliseconds())
	} else {
		c.logger.Warn("delete failed",
			"dn", req.DN,
			"result_code", result.ResultCode.String(),
			"error", result.DiagnosticMessage,
			"duration_ms", time.Since(start).Milliseconds())
	}

	return c.createDeleteResponse(msg.MessageID, result.ResultCode, result.MatchedDN, result.DiagnosticMessage)
}

// handleModify handles a modify request.
func (c *Connection) handleModify(msg *ldap.LDAPMessage) *ldap.LDAPMessage {
	start := time.Now()

	req, err := ldap.ParseModifyRequest(msg.Operation.Data)
	if err != nil {
		c.logger.Warn("modify request parse error",
			"error", err.Error(),
			"message_id", msg.MessageID)
		return c.createModifyResponse(msg.MessageID, ldap.ResultProtocolError, "", "invalid modify request")
	}

	c.logger.Debug("modify request",
		"object", req.Object,
		"changes_count", len(req.Changes),
		"message_id", msg.MessageID)

	// Call the handler
	result := c.handler.HandleModify(c, req)

	// Log result
	if result.ResultCode == ldap.ResultSuccess {
		c.logger.Info("modify successful",
			"object", req.Object,
			"changes_count", len(req.Changes),
			"duration_ms", time.Since(start).Milliseconds())
	} else {
		c.logger.Warn("modify failed",
			"object", req.Object,
			"result_code", result.ResultCode.String(),
			"error", result.DiagnosticMessage,
			"duration_ms", time.Since(start).Milliseconds())
	}

	return c.createModifyResponse(msg.MessageID, result.ResultCode, result.MatchedDN, result.DiagnosticMessage)
}

// handleModifyDN handles a modifydn request.
func (c *Connection) handleModifyDN(msg *ldap.LDAPMessage) *ldap.LDAPMessage {
	start := time.Now()

	req, err := ldap.ParseModifyDNRequest(msg.Operation.Data)
	if err != nil {
		c.logger.Warn("modifydn request parse error",
			"error", err.Error(),
			"message_id", msg.MessageID)
		return c.createModifyDNResponse(msg.MessageID, ldap.ResultProtocolError, "", "invalid modifydn request")
	}

	c.logger.Debug("modifydn request",
		"entry", req.Entry,
		"new_rdn", req.NewRDN,
		"delete_old_rdn", req.DeleteOldRDN,
		"new_superior", req.NewSuperior,
		"message_id", msg.MessageID)

	// Call the handler
	result := c.handler.HandleModifyDN(c, req)

	// Log result
	if result.ResultCode == ldap.ResultSuccess {
		c.logger.Info("modifydn successful",
			"entry", req.Entry,
			"new_rdn", req.NewRDN,
			"duration_ms", time.Since(start).Milliseconds())
	} else {
		c.logger.Warn("modifydn failed",
			"entry", req.Entry,
			"result_code", result.ResultCode.String(),
			"error", result.DiagnosticMessage,
			"duration_ms", time.Since(start).Milliseconds())
	}

	return c.createModifyDNResponse(msg.MessageID, result.ResultCode, result.MatchedDN, result.DiagnosticMessage)
}

// handleCompare handles a compare request.
func (c *Connection) handleCompare(msg *ldap.LDAPMessage) *ldap.LDAPMessage {
	start := time.Now()

	req, err := ldap.ParseCompareRequest(msg.Operation.Data)
	if err != nil {
		c.logger.Warn("compare request parse error",
			"error", err.Error(),
			"message_id", msg.MessageID)
		return c.createCompareResponse(msg.MessageID, ldap.ResultProtocolError, "", "invalid compare request")
	}

	c.logger.Debug("compare request",
		"dn", req.DN,
		"attribute", req.Attribute,
		"message_id", msg.MessageID)

	// Call the handler
	result := c.handler.HandleCompare(c, req)

	// Log result
	if result.ResultCode == ldap.ResultCompareTrue {
		c.logger.Info("compare result: true",
			"dn", req.DN,
			"attribute", req.Attribute,
			"duration_ms", time.Since(start).Milliseconds())
	} else if result.ResultCode == ldap.ResultCompareFalse {
		c.logger.Info("compare result: false",
			"dn", req.DN,
			"attribute", req.Attribute,
			"duration_ms", time.Since(start).Milliseconds())
	} else {
		c.logger.Warn("compare failed",
			"dn", req.DN,
			"attribute", req.Attribute,
			"result_code", result.ResultCode.String(),
			"error", result.DiagnosticMessage,
			"duration_ms", time.Since(start).Milliseconds())
	}

	return c.createCompareResponse(msg.MessageID, result.ResultCode, result.MatchedDN, result.DiagnosticMessage)
}

// ReadMessage reads the next LDAP message from the connection.
func (c *Connection) ReadMessage() (*ldap.LDAPMessage, error) {
	// Read the tag byte
	tagBuf := make([]byte, 1)
	if _, err := io.ReadFull(c.conn, tagBuf); err != nil {
		return nil, err
	}

	// Verify it's a SEQUENCE tag (0x30)
	if tagBuf[0] != byte(ber.ClassUniversal|ber.TypeConstructed|ber.TagSequence) {
		return nil, ErrInvalidMessage
	}

	// Read the length
	length, lengthBytes, err := c.readLength()
	if err != nil {
		return nil, err
	}

	// Check message size limit
	if length > MaxMessageSize {
		return nil, ErrMessageTooLarge
	}

	// Read the message content
	content := make([]byte, length)
	if _, err := io.ReadFull(c.conn, content); err != nil {
		return nil, err
	}

	// Reconstruct the full message with tag and length
	fullMessage := make([]byte, 1+len(lengthBytes)+length)
	fullMessage[0] = tagBuf[0]
	copy(fullMessage[1:], lengthBytes)
	copy(fullMessage[1+len(lengthBytes):], content)

	// Parse the LDAP message
	msg, err := ldap.ParseLDAPMessage(fullMessage)
	if err != nil {
		return nil, err
	}

	// Update message ID tracking
	c.mu.Lock()
	c.messageID = msg.MessageID
	c.mu.Unlock()

	return msg, nil
}

// readLength reads a BER length from the connection.
// Returns the length value and the raw length bytes.
func (c *Connection) readLength() (int, []byte, error) {
	// Read the first length byte
	firstByte := make([]byte, 1)
	if _, err := io.ReadFull(c.conn, firstByte); err != nil {
		return 0, nil, err
	}

	// Short form: bit 8 is 0, bits 1-7 contain the length
	if firstByte[0]&ber.LengthLongFormBit == 0 {
		return int(firstByte[0]), firstByte, nil
	}

	// Long form: bit 8 is 1, bits 1-7 contain the number of subsequent length bytes
	numBytes := int(firstByte[0] & 0x7F)

	// Check for indefinite length (0x80) - not supported
	if numBytes == 0 {
		return 0, nil, ErrInvalidMessage
	}

	// Read the length bytes
	lengthBytes := make([]byte, numBytes)
	if _, err := io.ReadFull(c.conn, lengthBytes); err != nil {
		return 0, nil, err
	}

	// Calculate the length value
	length := 0
	for _, b := range lengthBytes {
		length = (length << 8) | int(b)
	}

	// Return all length bytes (first byte + subsequent bytes)
	allLengthBytes := make([]byte, 1+numBytes)
	allLengthBytes[0] = firstByte[0]
	copy(allLengthBytes[1:], lengthBytes)

	return length, allLengthBytes, nil
}

// WriteMessage writes an LDAP message to the connection.
func (c *Connection) WriteMessage(msg *ldap.LDAPMessage) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return ErrConnectionClosed
	}
	c.mu.Unlock()

	// Encode the message
	data, err := msg.Encode()
	if err != nil {
		return err
	}

	// Write to the connection
	_, err = c.conn.Write(data)
	return err
}

// Close closes the connection.
func (c *Connection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true
	return c.conn.Close()
}

// isClosed returns whether the connection is closed.
func (c *Connection) isClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

// BindDN returns the currently bound DN.
func (c *Connection) BindDN() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.bindDN
}

// IsAuthenticated returns whether the connection is authenticated.
func (c *Connection) IsAuthenticated() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.authenticated
}

// RemoteAddr returns the remote address of the connection.
func (c *Connection) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

// LocalAddr returns the local address of the connection.
func (c *Connection) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

// createErrorResponse creates a generic error response.
func (c *Connection) createErrorResponse(messageID int, resultCode ldap.ResultCode, diagnosticMessage string) *ldap.LDAPMessage {
	// Create a generic response - use BindResponse as a fallback
	return c.createBindResponse(messageID, resultCode, "", diagnosticMessage)
}

// createBindResponse creates a BindResponse message.
// BindResponse ::= [APPLICATION 1] SEQUENCE {
//
//	COMPONENTS OF LDAPResult,
//	serverSaslCreds    [7] OCTET STRING OPTIONAL
//
// }
func (c *Connection) createBindResponse(messageID int, resultCode ldap.ResultCode, matchedDN, diagnosticMessage string) *ldap.LDAPMessage {
	encoder := ber.NewBEREncoder(128)

	// Write resultCode (ENUMERATED)
	if err := encoder.WriteEnumerated(int64(resultCode)); err != nil {
		return nil
	}

	// Write matchedDN (LDAPDN - OCTET STRING)
	if err := encoder.WriteOctetString([]byte(matchedDN)); err != nil {
		return nil
	}

	// Write diagnosticMessage (LDAPString - OCTET STRING)
	if err := encoder.WriteOctetString([]byte(diagnosticMessage)); err != nil {
		return nil
	}

	return &ldap.LDAPMessage{
		MessageID: messageID,
		Operation: &ldap.RawOperation{
			Tag:  ldap.ApplicationBindResponse,
			Data: encoder.Bytes(),
		},
	}
}

// createSearchDoneResponse creates a SearchResultDone message.
// SearchResultDone ::= [APPLICATION 5] LDAPResult
func (c *Connection) createSearchDoneResponse(messageID int, resultCode ldap.ResultCode, matchedDN, diagnosticMessage string) *ldap.LDAPMessage {
	encoder := ber.NewBEREncoder(128)

	// Write resultCode (ENUMERATED)
	if err := encoder.WriteEnumerated(int64(resultCode)); err != nil {
		return nil
	}

	// Write matchedDN (LDAPDN - OCTET STRING)
	if err := encoder.WriteOctetString([]byte(matchedDN)); err != nil {
		return nil
	}

	// Write diagnosticMessage (LDAPString - OCTET STRING)
	if err := encoder.WriteOctetString([]byte(diagnosticMessage)); err != nil {
		return nil
	}

	return &ldap.LDAPMessage{
		MessageID: messageID,
		Operation: &ldap.RawOperation{
			Tag:  ldap.ApplicationSearchResultDone,
			Data: encoder.Bytes(),
		},
	}
}

// createSearchEntryResponse creates a SearchResultEntry message.
// SearchResultEntry ::= [APPLICATION 4] SEQUENCE {
//
//	objectName      LDAPDN,
//	attributes      PartialAttributeList
//
// }
func (c *Connection) createSearchEntryResponse(messageID int, entry *SearchEntry) *ldap.LDAPMessage {
	encoder := ber.NewBEREncoder(256)

	// Write objectName (LDAPDN - OCTET STRING)
	if err := encoder.WriteOctetString([]byte(entry.DN)); err != nil {
		return nil
	}

	// Write attributes (SEQUENCE OF PartialAttribute)
	attrListPos := encoder.BeginSequence()

	for _, attr := range entry.Attributes {
		// Write PartialAttribute SEQUENCE
		attrPos := encoder.BeginSequence()

		// Write attribute type
		if err := encoder.WriteOctetString([]byte(attr.Type)); err != nil {
			return nil
		}

		// Write attribute values (SET OF OCTET STRING)
		valSetPos := encoder.BeginSet()
		for _, value := range attr.Values {
			if err := encoder.WriteOctetString(value); err != nil {
				return nil
			}
		}
		if err := encoder.EndSet(valSetPos); err != nil {
			return nil
		}

		if err := encoder.EndSequence(attrPos); err != nil {
			return nil
		}
	}

	if err := encoder.EndSequence(attrListPos); err != nil {
		return nil
	}

	return &ldap.LDAPMessage{
		MessageID: messageID,
		Operation: &ldap.RawOperation{
			Tag:  ldap.ApplicationSearchResultEntry,
			Data: encoder.Bytes(),
		},
	}
}

// createAddResponse creates an AddResponse message.
// AddResponse ::= [APPLICATION 9] LDAPResult
func (c *Connection) createAddResponse(messageID int, resultCode ldap.ResultCode, matchedDN, diagnosticMessage string) *ldap.LDAPMessage {
	encoder := ber.NewBEREncoder(128)

	// Write resultCode (ENUMERATED)
	if err := encoder.WriteEnumerated(int64(resultCode)); err != nil {
		return nil
	}

	// Write matchedDN (LDAPDN - OCTET STRING)
	if err := encoder.WriteOctetString([]byte(matchedDN)); err != nil {
		return nil
	}

	// Write diagnosticMessage (LDAPString - OCTET STRING)
	if err := encoder.WriteOctetString([]byte(diagnosticMessage)); err != nil {
		return nil
	}

	return &ldap.LDAPMessage{
		MessageID: messageID,
		Operation: &ldap.RawOperation{
			Tag:  ldap.ApplicationAddResponse,
			Data: encoder.Bytes(),
		},
	}
}

// createDeleteResponse creates a DelResponse message.
// DelResponse ::= [APPLICATION 11] LDAPResult
func (c *Connection) createDeleteResponse(messageID int, resultCode ldap.ResultCode, matchedDN, diagnosticMessage string) *ldap.LDAPMessage {
	encoder := ber.NewBEREncoder(128)

	// Write resultCode (ENUMERATED)
	if err := encoder.WriteEnumerated(int64(resultCode)); err != nil {
		return nil
	}

	// Write matchedDN (LDAPDN - OCTET STRING)
	if err := encoder.WriteOctetString([]byte(matchedDN)); err != nil {
		return nil
	}

	// Write diagnosticMessage (LDAPString - OCTET STRING)
	if err := encoder.WriteOctetString([]byte(diagnosticMessage)); err != nil {
		return nil
	}

	return &ldap.LDAPMessage{
		MessageID: messageID,
		Operation: &ldap.RawOperation{
			Tag:  ldap.ApplicationDelResponse,
			Data: encoder.Bytes(),
		},
	}
}

// createModifyResponse creates a ModifyResponse message.
// ModifyResponse ::= [APPLICATION 7] LDAPResult
func (c *Connection) createModifyResponse(messageID int, resultCode ldap.ResultCode, matchedDN, diagnosticMessage string) *ldap.LDAPMessage {
	encoder := ber.NewBEREncoder(128)

	// Write resultCode (ENUMERATED)
	if err := encoder.WriteEnumerated(int64(resultCode)); err != nil {
		return nil
	}

	// Write matchedDN (LDAPDN - OCTET STRING)
	if err := encoder.WriteOctetString([]byte(matchedDN)); err != nil {
		return nil
	}

	// Write diagnosticMessage (LDAPString - OCTET STRING)
	if err := encoder.WriteOctetString([]byte(diagnosticMessage)); err != nil {
		return nil
	}

	return &ldap.LDAPMessage{
		MessageID: messageID,
		Operation: &ldap.RawOperation{
			Tag:  ldap.ApplicationModifyResponse,
			Data: encoder.Bytes(),
		},
	}
}

// createModifyDNResponse creates a ModifyDNResponse message.
// ModifyDNResponse ::= [APPLICATION 13] LDAPResult
func (c *Connection) createModifyDNResponse(messageID int, resultCode ldap.ResultCode, matchedDN, diagnosticMessage string) *ldap.LDAPMessage {
	encoder := ber.NewBEREncoder(128)

	// Write resultCode (ENUMERATED)
	if err := encoder.WriteEnumerated(int64(resultCode)); err != nil {
		return nil
	}

	// Write matchedDN (LDAPDN - OCTET STRING)
	if err := encoder.WriteOctetString([]byte(matchedDN)); err != nil {
		return nil
	}

	// Write diagnosticMessage (LDAPString - OCTET STRING)
	if err := encoder.WriteOctetString([]byte(diagnosticMessage)); err != nil {
		return nil
	}

	return &ldap.LDAPMessage{
		MessageID: messageID,
		Operation: &ldap.RawOperation{
			Tag:  ldap.ApplicationModifyDNResponse,
			Data: encoder.Bytes(),
		},
	}
}

// createCompareResponse creates a CompareResponse message.
// CompareResponse ::= [APPLICATION 15] LDAPResult
func (c *Connection) createCompareResponse(messageID int, resultCode ldap.ResultCode, matchedDN, diagnosticMessage string) *ldap.LDAPMessage {
	encoder := ber.NewBEREncoder(128)

	// Write resultCode (ENUMERATED)
	if err := encoder.WriteEnumerated(int64(resultCode)); err != nil {
		return nil
	}

	// Write matchedDN (LDAPDN - OCTET STRING)
	if err := encoder.WriteOctetString([]byte(matchedDN)); err != nil {
		return nil
	}

	// Write diagnosticMessage (LDAPString - OCTET STRING)
	if err := encoder.WriteOctetString([]byte(diagnosticMessage)); err != nil {
		return nil
	}

	return &ldap.LDAPMessage{
		MessageID: messageID,
		Operation: &ldap.RawOperation{
			Tag:  ldap.ApplicationCompareResponse,
			Data: encoder.Bytes(),
		},
	}
}

// Logger returns the logger for this connection.
func (c *Connection) Logger() logging.Logger {
	return c.logger
}

// RequestID returns the unique request ID for this connection.
func (c *Connection) RequestID() string {
	return c.requestID
}

// SetTLS sets whether the connection is using TLS and captures the TLS state.
// If the underlying connection is a TLS connection, it extracts the connection
// state and client certificate (if provided).
func (c *Connection) SetTLS(isTLS bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.isTLS = isTLS

	// If TLS is enabled, try to extract TLS state from the connection
	if isTLS {
		if tlsConn, ok := c.conn.(*tls.Conn); ok {
			state := tlsConn.ConnectionState()
			c.tlsState = &state

			// Extract client certificate if provided
			if len(state.PeerCertificates) > 0 {
				c.clientCert = state.PeerCertificates[0]
			}
		}
	} else {
		c.tlsState = nil
		c.clientCert = nil
	}
}

// IsTLS returns whether the connection is using TLS.
func (c *Connection) IsTLS() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.isTLS
}

// RequireTLS checks if the connection is using TLS and returns an error if not.
// This is used to enforce TLS for security-sensitive operations like password changes.
func (c *Connection) RequireTLS() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.isTLS {
		return ErrTLSRequired
	}
	return nil
}

// GetTLSState returns the TLS connection state if the connection is using TLS.
// Returns nil if the connection is not using TLS.
func (c *Connection) GetTLSState() *tls.ConnectionState {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.tlsState
}

// GetClientCertificate returns the client certificate if one was provided during
// the TLS handshake. Returns nil if no client certificate was provided or if
// the connection is not using TLS.
func (c *Connection) GetClientCertificate() *x509.Certificate {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.clientCert
}

// GetTLSVersion returns the TLS version being used by the connection.
// Returns 0 if the connection is not using TLS.
func (c *Connection) GetTLSVersion() uint16 {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.tlsState == nil {
		return 0
	}
	return c.tlsState.Version
}

// GetCipherSuite returns the cipher suite being used by the TLS connection.
// Returns 0 if the connection is not using TLS.
func (c *Connection) GetCipherSuite() uint16 {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.tlsState == nil {
		return 0
	}
	return c.tlsState.CipherSuite
}

// GetServerName returns the server name indicated by the client during TLS handshake.
// Returns an empty string if the connection is not using TLS or if SNI was not used.
func (c *Connection) GetServerName() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.tlsState == nil {
		return ""
	}
	return c.tlsState.ServerName
}

// SetLogger sets the logger for this connection.
func (c *Connection) SetLogger(logger logging.Logger) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.logger = logger
}
