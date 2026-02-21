package server

import (
	"errors"
	"net"
	"sync"
	"testing"

	"github.com/KilimcininKorOglu/oba/internal/ldap"
)

// mockExtendedHandler is a test implementation of ExtendedHandler.
type mockExtendedHandler struct {
	oid      string
	response *ExtendedResponse
	err      error
	called   bool
	mu       sync.Mutex
}

func newMockHandler(oid string, resp *ExtendedResponse, err error) *mockExtendedHandler {
	return &mockExtendedHandler{
		oid:      oid,
		response: resp,
		err:      err,
	}
}

func (h *mockExtendedHandler) OID() string {
	return h.oid
}

func (h *mockExtendedHandler) Handle(conn *Connection, req *ExtendedRequest) (*ExtendedResponse, error) {
	h.mu.Lock()
	h.called = true
	h.mu.Unlock()
	return h.response, h.err
}

func (h *mockExtendedHandler) wasCalled() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.called
}

// TestNewExtendedDispatcher tests creating a new dispatcher.
func TestNewExtendedDispatcher(t *testing.T) {
	d := NewExtendedDispatcher()
	if d == nil {
		t.Fatal("NewExtendedDispatcher returned nil")
	}
	if d.handlers == nil {
		t.Error("handlers map should not be nil")
	}
	if d.HandlerCount() != 0 {
		t.Errorf("expected 0 handlers, got %d", d.HandlerCount())
	}
}

// TestExtendedDispatcher_Register tests registering handlers.
func TestExtendedDispatcher_Register(t *testing.T) {
	d := NewExtendedDispatcher()

	handler := newMockHandler("1.2.3.4", &ExtendedResponse{
		Result: OperationResult{ResultCode: ldap.ResultSuccess},
	}, nil)

	err := d.Register(handler)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if d.HandlerCount() != 1 {
		t.Errorf("expected 1 handler, got %d", d.HandlerCount())
	}

	if !d.HasHandler("1.2.3.4") {
		t.Error("expected handler to be registered for OID 1.2.3.4")
	}
}

// TestExtendedDispatcher_Register_NilHandler tests registering a nil handler.
func TestExtendedDispatcher_Register_NilHandler(t *testing.T) {
	d := NewExtendedDispatcher()

	err := d.Register(nil)
	if err != ErrNilHandler {
		t.Errorf("expected ErrNilHandler, got %v", err)
	}
}

// TestExtendedDispatcher_Register_EmptyOID tests registering a handler with empty OID.
func TestExtendedDispatcher_Register_EmptyOID(t *testing.T) {
	d := NewExtendedDispatcher()

	handler := newMockHandler("", &ExtendedResponse{
		Result: OperationResult{ResultCode: ldap.ResultSuccess},
	}, nil)

	err := d.Register(handler)
	if err != ErrEmptyOID {
		t.Errorf("expected ErrEmptyOID, got %v", err)
	}
}

// TestExtendedDispatcher_Register_Replace tests replacing an existing handler.
func TestExtendedDispatcher_Register_Replace(t *testing.T) {
	d := NewExtendedDispatcher()

	handler1 := newMockHandler("1.2.3.4", &ExtendedResponse{
		Result: OperationResult{ResultCode: ldap.ResultSuccess},
	}, nil)

	handler2 := newMockHandler("1.2.3.4", &ExtendedResponse{
		Result: OperationResult{ResultCode: ldap.ResultOperationsError},
	}, nil)

	if err := d.Register(handler1); err != nil {
		t.Fatalf("Register handler1 failed: %v", err)
	}

	if err := d.Register(handler2); err != nil {
		t.Fatalf("Register handler2 failed: %v", err)
	}

	// Should still have only 1 handler
	if d.HandlerCount() != 1 {
		t.Errorf("expected 1 handler after replacement, got %d", d.HandlerCount())
	}

	// Create a mock connection for testing
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	conn := NewConnection(server, nil)
	req := &ExtendedRequest{OID: "1.2.3.4"}

	resp, _ := d.Handle(conn, req)
	if resp.Result.ResultCode != ldap.ResultOperationsError {
		t.Error("expected handler2 to be used after replacement")
	}
}

// TestExtendedDispatcher_Unregister tests unregistering handlers.
func TestExtendedDispatcher_Unregister(t *testing.T) {
	d := NewExtendedDispatcher()

	handler := newMockHandler("1.2.3.4", &ExtendedResponse{
		Result: OperationResult{ResultCode: ldap.ResultSuccess},
	}, nil)

	if err := d.Register(handler); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Unregister existing handler
	if !d.Unregister("1.2.3.4") {
		t.Error("expected Unregister to return true for existing handler")
	}

	if d.HandlerCount() != 0 {
		t.Errorf("expected 0 handlers after unregister, got %d", d.HandlerCount())
	}

	// Unregister non-existing handler
	if d.Unregister("1.2.3.4") {
		t.Error("expected Unregister to return false for non-existing handler")
	}
}

// TestExtendedDispatcher_Handle_Success tests successful request handling.
func TestExtendedDispatcher_Handle_Success(t *testing.T) {
	d := NewExtendedDispatcher()

	expectedResp := &ExtendedResponse{
		Result: OperationResult{ResultCode: ldap.ResultSuccess},
		OID:    "1.2.3.4",
		Value:  []byte("test response"),
	}

	handler := newMockHandler("1.2.3.4", expectedResp, nil)
	if err := d.Register(handler); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	conn := NewConnection(server, nil)
	req := &ExtendedRequest{
		OID:   "1.2.3.4",
		Value: []byte("test request"),
	}

	resp, err := d.Handle(conn, req)
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if !handler.wasCalled() {
		t.Error("handler was not called")
	}

	if resp.Result.ResultCode != ldap.ResultSuccess {
		t.Errorf("expected ResultSuccess, got %v", resp.Result.ResultCode)
	}

	if resp.OID != "1.2.3.4" {
		t.Errorf("expected OID 1.2.3.4, got %s", resp.OID)
	}

	if string(resp.Value) != "test response" {
		t.Errorf("expected value 'test response', got %s", string(resp.Value))
	}
}

// TestExtendedDispatcher_Handle_UnknownOID tests handling unknown OID.
func TestExtendedDispatcher_Handle_UnknownOID(t *testing.T) {
	d := NewExtendedDispatcher()

	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	conn := NewConnection(server, nil)
	req := &ExtendedRequest{OID: "1.2.3.4.5.6"}

	resp, err := d.Handle(conn, req)
	if !errors.Is(err, ErrUnknownOID) {
		t.Errorf("expected ErrUnknownOID, got %v", err)
	}

	if resp == nil {
		t.Fatal("expected error response, got nil")
	}

	if resp.Result.ResultCode != ldap.ResultProtocolError {
		t.Errorf("expected ResultProtocolError, got %v", resp.Result.ResultCode)
	}

	if resp.Result.DiagnosticMessage == "" {
		t.Error("expected diagnostic message for unknown OID")
	}
}

// TestExtendedDispatcher_Handle_HandlerError tests handling when handler returns error.
func TestExtendedDispatcher_Handle_HandlerError(t *testing.T) {
	d := NewExtendedDispatcher()

	expectedErr := errors.New("handler error")
	handler := newMockHandler("1.2.3.4", &ExtendedResponse{
		Result: OperationResult{
			ResultCode:        ldap.ResultOperationsError,
			DiagnosticMessage: "internal error",
		},
	}, expectedErr)

	if err := d.Register(handler); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	conn := NewConnection(server, nil)
	req := &ExtendedRequest{OID: "1.2.3.4"}

	resp, err := d.Handle(conn, req)
	if err != expectedErr {
		t.Errorf("expected handler error, got %v", err)
	}

	if resp.Result.ResultCode != ldap.ResultOperationsError {
		t.Errorf("expected ResultOperationsError, got %v", resp.Result.ResultCode)
	}
}

// TestExtendedDispatcher_HasHandler tests checking for handler existence.
func TestExtendedDispatcher_HasHandler(t *testing.T) {
	d := NewExtendedDispatcher()

	if d.HasHandler("1.2.3.4") {
		t.Error("expected HasHandler to return false for unregistered OID")
	}

	handler := newMockHandler("1.2.3.4", &ExtendedResponse{
		Result: OperationResult{ResultCode: ldap.ResultSuccess},
	}, nil)

	if err := d.Register(handler); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if !d.HasHandler("1.2.3.4") {
		t.Error("expected HasHandler to return true for registered OID")
	}

	if d.HasHandler("5.6.7.8") {
		t.Error("expected HasHandler to return false for different OID")
	}
}

// TestExtendedDispatcher_SupportedOIDs tests getting supported OIDs.
func TestExtendedDispatcher_SupportedOIDs(t *testing.T) {
	d := NewExtendedDispatcher()

	// Empty dispatcher
	oids := d.SupportedOIDs()
	if len(oids) != 0 {
		t.Errorf("expected 0 OIDs, got %d", len(oids))
	}

	// Register multiple handlers
	handlers := []string{"3.3.3.3", "1.1.1.1", "2.2.2.2"}
	for _, oid := range handlers {
		handler := newMockHandler(oid, &ExtendedResponse{
			Result: OperationResult{ResultCode: ldap.ResultSuccess},
		}, nil)
		if err := d.Register(handler); err != nil {
			t.Fatalf("Register failed: %v", err)
		}
	}

	oids = d.SupportedOIDs()
	if len(oids) != 3 {
		t.Errorf("expected 3 OIDs, got %d", len(oids))
	}

	// Verify sorted order
	expected := []string{"1.1.1.1", "2.2.2.2", "3.3.3.3"}
	for i, oid := range oids {
		if oid != expected[i] {
			t.Errorf("OID at index %d: expected %s, got %s", i, expected[i], oid)
		}
	}
}

// TestExtendedDispatcher_RegisterFunc tests registering a function handler.
func TestExtendedDispatcher_RegisterFunc(t *testing.T) {
	d := NewExtendedDispatcher()

	called := false
	err := d.RegisterFunc("1.2.3.4", func(conn *Connection, req *ExtendedRequest) (*ExtendedResponse, error) {
		called = true
		return &ExtendedResponse{
			Result: OperationResult{ResultCode: ldap.ResultSuccess},
		}, nil
	})

	if err != nil {
		t.Fatalf("RegisterFunc failed: %v", err)
	}

	if !d.HasHandler("1.2.3.4") {
		t.Error("expected handler to be registered")
	}

	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	conn := NewConnection(server, nil)
	req := &ExtendedRequest{OID: "1.2.3.4"}

	resp, err := d.Handle(conn, req)
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if !called {
		t.Error("function handler was not called")
	}

	if resp.Result.ResultCode != ldap.ResultSuccess {
		t.Errorf("expected ResultSuccess, got %v", resp.Result.ResultCode)
	}
}

// TestExtendedDispatcher_RegisterFunc_NilHandler tests registering nil function.
func TestExtendedDispatcher_RegisterFunc_NilHandler(t *testing.T) {
	d := NewExtendedDispatcher()

	err := d.RegisterFunc("1.2.3.4", nil)
	if err != ErrNilHandler {
		t.Errorf("expected ErrNilHandler, got %v", err)
	}
}

// TestExtendedDispatcher_RegisterFunc_EmptyOID tests registering function with empty OID.
func TestExtendedDispatcher_RegisterFunc_EmptyOID(t *testing.T) {
	d := NewExtendedDispatcher()

	err := d.RegisterFunc("", func(conn *Connection, req *ExtendedRequest) (*ExtendedResponse, error) {
		return nil, nil
	})
	if err != ErrEmptyOID {
		t.Errorf("expected ErrEmptyOID, got %v", err)
	}
}

// TestExtendedDispatcher_Concurrent tests concurrent access to dispatcher.
func TestExtendedDispatcher_Concurrent(t *testing.T) {
	d := NewExtendedDispatcher()

	// Register a handler
	handler := newMockHandler("1.2.3.4", &ExtendedResponse{
		Result: OperationResult{ResultCode: ldap.ResultSuccess},
	}, nil)
	if err := d.Register(handler); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	var wg sync.WaitGroup
	numGoroutines := 100

	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d.HasHandler("1.2.3.4")
			d.SupportedOIDs()
			d.HandlerCount()
		}()
	}

	// Concurrent handles
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client, server := net.Pipe()
			defer client.Close()
			defer server.Close()

			conn := NewConnection(server, nil)
			req := &ExtendedRequest{OID: "1.2.3.4"}
			d.Handle(conn, req)
		}()
	}

	wg.Wait()
}

// TestParseExtendedRequest_Valid tests parsing valid extended requests.
func TestParseExtendedRequest_Valid(t *testing.T) {
	tests := []struct {
		name        string
		data        []byte
		expectedOID string
		expectedVal []byte
	}{
		{
			name: "OID only",
			data: []byte{
				0x80, 0x07, // Context tag [0], length 7
				'1', '.', '2', '.', '3', '.', '4',
			},
			expectedOID: "1.2.3.4",
			expectedVal: nil,
		},
		{
			name: "OID with value",
			data: []byte{
				0x80, 0x07, // Context tag [0], length 7
				'1', '.', '2', '.', '3', '.', '4',
				0x81, 0x05, // Context tag [1], length 5
				'h', 'e', 'l', 'l', 'o',
			},
			expectedOID: "1.2.3.4",
			expectedVal: []byte("hello"),
		},
		{
			name: "Long OID",
			data: []byte{
				0x80, 0x17, // Context tag [0], length 23
				'1', '.', '3', '.', '6', '.', '1', '.', '4', '.', '1', '.', '1', '4', '6', '6', '.', '2', '0', '0', '3', '7', '0',
			},
			expectedOID: "1.3.6.1.4.1.1466.200370",
			expectedVal: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := ParseExtendedRequest(tt.data)
			if err != nil {
				t.Fatalf("ParseExtendedRequest failed: %v", err)
			}

			if req.OID != tt.expectedOID {
				t.Errorf("OID = %q, want %q", req.OID, tt.expectedOID)
			}

			if string(req.Value) != string(tt.expectedVal) {
				t.Errorf("Value = %q, want %q", req.Value, tt.expectedVal)
			}
		})
	}
}

// TestParseExtendedRequest_Invalid tests parsing invalid extended requests.
func TestParseExtendedRequest_Invalid(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "Empty data",
			data: []byte{},
		},
		{
			name: "Wrong tag",
			data: []byte{
				0x81, 0x07, // Context tag [1] instead of [0]
				'1', '.', '2', '.', '3', '.', '4',
			},
		},
		{
			name: "Truncated data",
			data: []byte{
				0x80, 0x10, // Context tag [0], length 16 (but only 7 bytes follow)
				'1', '.', '2', '.', '3', '.', '4',
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseExtendedRequest(tt.data)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

// TestCreateExtendedResponse_Valid tests creating valid extended responses.
func TestCreateExtendedResponse_Valid(t *testing.T) {
	tests := []struct {
		name      string
		messageID int
		resp      *ExtendedResponse
	}{
		{
			name:      "Success without OID or value",
			messageID: 1,
			resp: &ExtendedResponse{
				Result: OperationResult{
					ResultCode: ldap.ResultSuccess,
				},
			},
		},
		{
			name:      "Success with OID",
			messageID: 2,
			resp: &ExtendedResponse{
				Result: OperationResult{
					ResultCode: ldap.ResultSuccess,
				},
				OID: "1.2.3.4",
			},
		},
		{
			name:      "Success with OID and value",
			messageID: 3,
			resp: &ExtendedResponse{
				Result: OperationResult{
					ResultCode: ldap.ResultSuccess,
				},
				OID:   "1.2.3.4",
				Value: []byte("response data"),
			},
		},
		{
			name:      "Error response",
			messageID: 4,
			resp: &ExtendedResponse{
				Result: OperationResult{
					ResultCode:        ldap.ResultProtocolError,
					MatchedDN:         "dc=example,dc=com",
					DiagnosticMessage: "unsupported operation",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := createExtendedResponse(tt.messageID, tt.resp)
			if msg == nil {
				t.Fatal("createExtendedResponse returned nil")
			}

			if msg.MessageID != tt.messageID {
				t.Errorf("MessageID = %d, want %d", msg.MessageID, tt.messageID)
			}

			if msg.Operation == nil {
				t.Fatal("Operation is nil")
			}

			if msg.Operation.Tag != ldap.ApplicationExtendedResponse {
				t.Errorf("Tag = %d, want %d", msg.Operation.Tag, ldap.ApplicationExtendedResponse)
			}

			if len(msg.Operation.Data) == 0 {
				t.Error("Operation.Data should not be empty")
			}
		})
	}
}

// TestExtendedRequest_Fields tests ExtendedRequest field access.
func TestExtendedRequest_Fields(t *testing.T) {
	req := &ExtendedRequest{
		OID:   "1.2.3.4.5",
		Value: []byte("test value"),
	}

	if req.OID != "1.2.3.4.5" {
		t.Errorf("OID = %q, want %q", req.OID, "1.2.3.4.5")
	}

	if string(req.Value) != "test value" {
		t.Errorf("Value = %q, want %q", string(req.Value), "test value")
	}
}

// TestExtendedResponse_Fields tests ExtendedResponse field access.
func TestExtendedResponse_Fields(t *testing.T) {
	resp := &ExtendedResponse{
		Result: OperationResult{
			ResultCode:        ldap.ResultSuccess,
			MatchedDN:         "dc=example,dc=com",
			DiagnosticMessage: "operation completed",
		},
		OID:   "1.2.3.4.5",
		Value: []byte("response value"),
	}

	if resp.Result.ResultCode != ldap.ResultSuccess {
		t.Errorf("ResultCode = %v, want %v", resp.Result.ResultCode, ldap.ResultSuccess)
	}

	if resp.Result.MatchedDN != "dc=example,dc=com" {
		t.Errorf("MatchedDN = %q, want %q", resp.Result.MatchedDN, "dc=example,dc=com")
	}

	if resp.Result.DiagnosticMessage != "operation completed" {
		t.Errorf("DiagnosticMessage = %q, want %q", resp.Result.DiagnosticMessage, "operation completed")
	}

	if resp.OID != "1.2.3.4.5" {
		t.Errorf("OID = %q, want %q", resp.OID, "1.2.3.4.5")
	}

	if string(resp.Value) != "response value" {
		t.Errorf("Value = %q, want %q", string(resp.Value), "response value")
	}
}

// TestFuncHandler tests the funcHandler wrapper.
func TestFuncHandler(t *testing.T) {
	h := &funcHandler{
		oid: "1.2.3.4",
		handler: func(conn *Connection, req *ExtendedRequest) (*ExtendedResponse, error) {
			return &ExtendedResponse{
				Result: OperationResult{ResultCode: ldap.ResultSuccess},
			}, nil
		},
	}

	if h.OID() != "1.2.3.4" {
		t.Errorf("OID() = %q, want %q", h.OID(), "1.2.3.4")
	}

	resp, err := h.Handle(nil, &ExtendedRequest{OID: "1.2.3.4"})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if resp.Result.ResultCode != ldap.ResultSuccess {
		t.Errorf("ResultCode = %v, want %v", resp.Result.ResultCode, ldap.ResultSuccess)
	}
}
