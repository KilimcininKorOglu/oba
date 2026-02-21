package server

import (
	"net"
	"testing"

	"github.com/KilimcininKorOglu/oba/internal/ldap"
)

// TestWhoAmIOID tests that the OID constant is correct.
func TestWhoAmIOID(t *testing.T) {
	expectedOID := "1.3.6.1.4.1.4203.1.11.3"
	if WhoAmIOID != expectedOID {
		t.Errorf("WhoAmIOID = %q, want %q", WhoAmIOID, expectedOID)
	}
}

// TestNewWhoAmIHandler tests creating a new handler.
func TestNewWhoAmIHandler(t *testing.T) {
	handler := NewWhoAmIHandler()
	if handler == nil {
		t.Fatal("NewWhoAmIHandler returned nil")
	}
}

// TestWhoAmIHandler_OID tests that the handler returns the correct OID.
func TestWhoAmIHandler_OID(t *testing.T) {
	handler := NewWhoAmIHandler()
	if handler.OID() != WhoAmIOID {
		t.Errorf("OID() = %q, want %q", handler.OID(), WhoAmIOID)
	}
}

// TestWhoAmIHandler_Handle_Anonymous tests the handler with an anonymous connection.
func TestWhoAmIHandler_Handle_Anonymous(t *testing.T) {
	handler := NewWhoAmIHandler()

	// Create a mock connection (anonymous - no bind DN)
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	conn := NewConnection(server, nil)
	// Connection is anonymous by default (bindDN is empty)

	req := &ExtendedRequest{
		OID: WhoAmIOID,
	}

	resp, err := handler.Handle(conn, req)
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if resp == nil {
		t.Fatal("Handle returned nil response")
	}

	// Check result code
	if resp.Result.ResultCode != ldap.ResultSuccess {
		t.Errorf("ResultCode = %v, want %v", resp.Result.ResultCode, ldap.ResultSuccess)
	}

	// Check that value is empty for anonymous
	if len(resp.Value) != 0 {
		t.Errorf("Value = %q, want empty string for anonymous", string(resp.Value))
	}
}

// TestWhoAmIHandler_Handle_Authenticated tests the handler with an authenticated connection.
func TestWhoAmIHandler_Handle_Authenticated(t *testing.T) {
	handler := NewWhoAmIHandler()

	// Create a mock connection
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	conn := NewConnection(server, nil)

	// Simulate a successful bind by setting the bindDN
	testDN := "uid=alice,ou=users,dc=example,dc=com"
	conn.mu.Lock()
	conn.bindDN = testDN
	conn.authenticated = true
	conn.mu.Unlock()

	req := &ExtendedRequest{
		OID: WhoAmIOID,
	}

	resp, err := handler.Handle(conn, req)
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if resp == nil {
		t.Fatal("Handle returned nil response")
	}

	// Check result code
	if resp.Result.ResultCode != ldap.ResultSuccess {
		t.Errorf("ResultCode = %v, want %v", resp.Result.ResultCode, ldap.ResultSuccess)
	}

	// Check that value has "dn:" prefix
	expectedValue := "dn:" + testDN
	if string(resp.Value) != expectedValue {
		t.Errorf("Value = %q, want %q", string(resp.Value), expectedValue)
	}
}

// TestWhoAmIHandler_Handle_DifferentDNs tests the handler with various DN formats.
func TestWhoAmIHandler_Handle_DifferentDNs(t *testing.T) {
	tests := []struct {
		name          string
		bindDN        string
		expectedValue string
	}{
		{
			name:          "Simple DN",
			bindDN:        "cn=admin",
			expectedValue: "dn:cn=admin",
		},
		{
			name:          "Full DN",
			bindDN:        "uid=bob,ou=people,dc=example,dc=org",
			expectedValue: "dn:uid=bob,ou=people,dc=example,dc=org",
		},
		{
			name:          "DN with spaces",
			bindDN:        "cn=John Doe,ou=users,dc=example,dc=com",
			expectedValue: "dn:cn=John Doe,ou=users,dc=example,dc=com",
		},
		{
			name:          "DN with special characters",
			bindDN:        "cn=test+uid=123,ou=users,dc=example,dc=com",
			expectedValue: "dn:cn=test+uid=123,ou=users,dc=example,dc=com",
		},
		{
			name:          "Root DN",
			bindDN:        "cn=Directory Manager",
			expectedValue: "dn:cn=Directory Manager",
		},
	}

	handler := NewWhoAmIHandler()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, server := net.Pipe()
			defer client.Close()
			defer server.Close()

			conn := NewConnection(server, nil)
			conn.mu.Lock()
			conn.bindDN = tt.bindDN
			conn.authenticated = true
			conn.mu.Unlock()

			req := &ExtendedRequest{
				OID: WhoAmIOID,
			}

			resp, err := handler.Handle(conn, req)
			if err != nil {
				t.Fatalf("Handle returned error: %v", err)
			}

			if string(resp.Value) != tt.expectedValue {
				t.Errorf("Value = %q, want %q", string(resp.Value), tt.expectedValue)
			}
		})
	}
}

// TestWhoAmIHandler_RegisterWithDispatcher tests registering the handler with a dispatcher.
func TestWhoAmIHandler_RegisterWithDispatcher(t *testing.T) {
	dispatcher := NewExtendedDispatcher()
	handler := NewWhoAmIHandler()

	err := dispatcher.Register(handler)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if !dispatcher.HasHandler(WhoAmIOID) {
		t.Error("expected handler to be registered for WhoAmIOID")
	}

	// Verify the handler count
	if dispatcher.HandlerCount() != 1 {
		t.Errorf("HandlerCount = %d, want 1", dispatcher.HandlerCount())
	}

	// Verify the OID is in supported list
	oids := dispatcher.SupportedOIDs()
	found := false
	for _, oid := range oids {
		if oid == WhoAmIOID {
			found = true
			break
		}
	}
	if !found {
		t.Error("WhoAmIOID not found in SupportedOIDs")
	}
}

// TestWhoAmIHandler_DispatcherIntegration tests the full flow through the dispatcher.
func TestWhoAmIHandler_DispatcherIntegration(t *testing.T) {
	dispatcher := NewExtendedDispatcher()
	handler := NewWhoAmIHandler()

	if err := dispatcher.Register(handler); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Test anonymous connection through dispatcher
	t.Run("Anonymous via dispatcher", func(t *testing.T) {
		client, server := net.Pipe()
		defer client.Close()
		defer server.Close()

		conn := NewConnection(server, nil)
		req := &ExtendedRequest{OID: WhoAmIOID}

		resp, err := dispatcher.Handle(conn, req)
		if err != nil {
			t.Fatalf("Handle returned error: %v", err)
		}

		if resp.Result.ResultCode != ldap.ResultSuccess {
			t.Errorf("ResultCode = %v, want %v", resp.Result.ResultCode, ldap.ResultSuccess)
		}

		if len(resp.Value) != 0 {
			t.Errorf("Value = %q, want empty for anonymous", string(resp.Value))
		}
	})

	// Test authenticated connection through dispatcher
	t.Run("Authenticated via dispatcher", func(t *testing.T) {
		client, server := net.Pipe()
		defer client.Close()
		defer server.Close()

		conn := NewConnection(server, nil)
		conn.mu.Lock()
		conn.bindDN = "cn=admin,dc=example,dc=com"
		conn.authenticated = true
		conn.mu.Unlock()

		req := &ExtendedRequest{OID: WhoAmIOID}

		resp, err := dispatcher.Handle(conn, req)
		if err != nil {
			t.Fatalf("Handle returned error: %v", err)
		}

		if resp.Result.ResultCode != ldap.ResultSuccess {
			t.Errorf("ResultCode = %v, want %v", resp.Result.ResultCode, ldap.ResultSuccess)
		}

		expectedValue := "dn:cn=admin,dc=example,dc=com"
		if string(resp.Value) != expectedValue {
			t.Errorf("Value = %q, want %q", string(resp.Value), expectedValue)
		}
	})
}

// TestWhoAmIHandler_ResponseEncoding tests that the response is correctly encoded.
func TestWhoAmIHandler_ResponseEncoding(t *testing.T) {
	handler := NewWhoAmIHandler()

	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	conn := NewConnection(server, nil)
	conn.mu.Lock()
	conn.bindDN = "uid=test,dc=example,dc=com"
	conn.authenticated = true
	conn.mu.Unlock()

	req := &ExtendedRequest{OID: WhoAmIOID}

	resp, err := handler.Handle(conn, req)
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	// Create the extended response message
	msg := createExtendedResponse(1, resp)
	if msg == nil {
		t.Fatal("createExtendedResponse returned nil")
	}

	// Verify message structure
	if msg.MessageID != 1 {
		t.Errorf("MessageID = %d, want 1", msg.MessageID)
	}

	if msg.Operation == nil {
		t.Fatal("Operation is nil")
	}

	if msg.Operation.Tag != ldap.ApplicationExtendedResponse {
		t.Errorf("Tag = %d, want %d", msg.Operation.Tag, ldap.ApplicationExtendedResponse)
	}

	// Verify the operation data is not empty
	if len(msg.Operation.Data) == 0 {
		t.Error("Operation.Data should not be empty")
	}
}

// TestWhoAmIHandler_ImplementsInterface verifies the handler implements ExtendedHandler.
func TestWhoAmIHandler_ImplementsInterface(t *testing.T) {
	var _ ExtendedHandler = (*WhoAmIHandler)(nil)
	var _ ExtendedHandler = NewWhoAmIHandler()
}

// TestWhoAmIHandler_NilRequest tests handling with various request states.
func TestWhoAmIHandler_NilRequest(t *testing.T) {
	handler := NewWhoAmIHandler()

	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	conn := NewConnection(server, nil)

	// Request with no value (which is valid for Who Am I)
	req := &ExtendedRequest{
		OID:   WhoAmIOID,
		Value: nil,
	}

	resp, err := handler.Handle(conn, req)
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if resp.Result.ResultCode != ldap.ResultSuccess {
		t.Errorf("ResultCode = %v, want %v", resp.Result.ResultCode, ldap.ResultSuccess)
	}
}

// TestWhoAmIHandler_RequestWithValue tests that request value is ignored.
func TestWhoAmIHandler_RequestWithValue(t *testing.T) {
	handler := NewWhoAmIHandler()

	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	conn := NewConnection(server, nil)
	conn.mu.Lock()
	conn.bindDN = "cn=test"
	conn.authenticated = true
	conn.mu.Unlock()

	// Request with unexpected value (should be ignored per RFC 4532)
	req := &ExtendedRequest{
		OID:   WhoAmIOID,
		Value: []byte("unexpected value"),
	}

	resp, err := handler.Handle(conn, req)
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	// Should still succeed and return the correct authzID
	if resp.Result.ResultCode != ldap.ResultSuccess {
		t.Errorf("ResultCode = %v, want %v", resp.Result.ResultCode, ldap.ResultSuccess)
	}

	expectedValue := "dn:cn=test"
	if string(resp.Value) != expectedValue {
		t.Errorf("Value = %q, want %q", string(resp.Value), expectedValue)
	}
}
