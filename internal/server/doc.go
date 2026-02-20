// Package server provides the LDAP server implementation including connection
// handling, request dispatching, and response encoding.
//
// # Overview
//
// The server package implements the network layer of the LDAP server. It handles:
//
//   - TCP/TLS connection management
//   - LDAP message reading and writing
//   - Request dispatching to operation handlers
//   - Response encoding and transmission
//   - Connection state management (bind DN, authentication status)
//
// # Connection Handling
//
// Each client connection is managed by a Connection instance:
//
//	conn := server.NewConnection(netConn, srv)
//	go conn.Handle() // Start message loop
//
// The Handle method runs the main message loop, reading LDAP messages,
// dispatching them to handlers, and sending responses.
//
// # Operation Handlers
//
// Custom handlers can be registered for each LDAP operation:
//
//	handler := server.NewHandler()
//
//	handler.SetBindHandler(func(conn *server.Connection, req *ldap.BindRequest) *server.OperationResult {
//	    // Authenticate user
//	    if req.Name == "cn=admin,dc=example,dc=com" && verifyPassword(req.Password) {
//	        return &server.OperationResult{ResultCode: ldap.ResultSuccess}
//	    }
//	    return &server.OperationResult{ResultCode: ldap.ResultInvalidCredentials}
//	})
//
//	handler.SetSearchHandler(func(conn *server.Connection, req *ldap.SearchRequest) *server.SearchResult {
//	    // Perform search and return results
//	    entries := performSearch(req)
//	    return &server.SearchResult{
//	        OperationResult: server.OperationResult{ResultCode: ldap.ResultSuccess},
//	        Entries:         entries,
//	    }
//	})
//
// # TLS Support
//
// Connections can be established over TLS or upgraded via StartTLS:
//
//	// Check if connection is using TLS
//	if conn.IsTLS() {
//	    state := conn.GetTLSState()
//	    // Access TLS connection details
//	}
//
//	// Require TLS for sensitive operations
//	if err := conn.RequireTLS(); err != nil {
//	    return &server.OperationResult{
//	        ResultCode:        ldap.ResultConfidentialityRequired,
//	        DiagnosticMessage: "TLS required for this operation",
//	    }
//	}
//
// # Connection State
//
// Connection state is accessible through getter methods:
//
//	bindDN := conn.BindDN()           // Currently bound DN
//	isAuth := conn.IsAuthenticated()  // Whether authenticated
//	remote := conn.RemoteAddr()       // Client address
//	reqID := conn.RequestID()         // Unique request ID for logging
//
// # Logging
//
// Each connection has an associated logger with request ID:
//
//	logger := conn.Logger()
//	logger.Info("processing request", "operation", "search", "base_dn", baseDN)
package server
