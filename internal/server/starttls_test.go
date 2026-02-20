package server

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/oba-ldap/oba/internal/ldap"
)

// generateStartTLSTestCertificate creates a self-signed certificate for testing.
func generateStartTLSTestCertificate() (certPEM, keyPEM []byte, err error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test Org"},
			CommonName:   "localhost",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, nil, err
	}

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	keyDER, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		return nil, nil, err
	}

	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	return certPEM, keyPEM, nil
}

// createTestTLSConfig creates a TLS config for testing.
func createTestTLSConfig(t *testing.T) *tls.Config {
	certPEM, keyPEM, err := generateStartTLSTestCertificate()
	if err != nil {
		t.Fatalf("failed to generate test certificate: %v", err)
	}

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("failed to load test certificate: %v", err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}
}

// TestStartTLSOID verifies the StartTLS OID constant.
func TestStartTLSOID(t *testing.T) {
	expected := "1.3.6.1.4.1.1466.20037"
	if StartTLSOID != expected {
		t.Errorf("StartTLSOID = %q, want %q", StartTLSOID, expected)
	}
}

// TestNewStartTLSHandler tests creating a new StartTLS handler.
func TestNewStartTLSHandler(t *testing.T) {
	tlsConfig := createTestTLSConfig(t)
	handler := NewStartTLSHandler(tlsConfig)

	if handler == nil {
		t.Fatal("NewStartTLSHandler returned nil")
	}

	if handler.GetTLSConfig() != tlsConfig {
		t.Error("TLS config not set correctly")
	}
}

// TestStartTLSHandler_SetTLSConfig tests updating the TLS configuration.
func TestStartTLSHandler_SetTLSConfig(t *testing.T) {
	handler := NewStartTLSHandler(nil)

	if handler.GetTLSConfig() != nil {
		t.Error("expected nil TLS config initially")
	}

	tlsConfig := createTestTLSConfig(t)
	handler.SetTLSConfig(tlsConfig)

	if handler.GetTLSConfig() != tlsConfig {
		t.Error("TLS config not updated correctly")
	}
}

// TestStartTLSHandler_Handle_InvalidOID tests handling a request with invalid OID.
func TestStartTLSHandler_Handle_InvalidOID(t *testing.T) {
	tlsConfig := createTestTLSConfig(t)
	handler := NewStartTLSHandler(tlsConfig)

	// Create a mock connection
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	conn := NewConnection(server, nil)
	tlsConn := NewTLSConnection(conn)

	req := &ExtendedRequest{
		OID: "1.2.3.4.5", // Invalid OID
	}

	resp, err := handler.Handle(tlsConn, req)
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if resp.Result.ResultCode != ldap.ResultProtocolError {
		t.Errorf("expected ResultProtocolError, got %v", resp.Result.ResultCode)
	}
}

// TestStartTLSHandler_Handle_NoTLSConfig tests handling when TLS config is not available.
func TestStartTLSHandler_Handle_NoTLSConfig(t *testing.T) {
	handler := NewStartTLSHandler(nil)

	// Create a mock connection
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	conn := NewConnection(server, nil)
	tlsConn := NewTLSConnection(conn)

	req := &ExtendedRequest{
		OID: StartTLSOID,
	}

	resp, err := handler.Handle(tlsConn, req)
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if resp.Result.ResultCode != ldap.ResultUnavailable {
		t.Errorf("expected ResultUnavailable, got %v", resp.Result.ResultCode)
	}
}

// TestStartTLSHandler_Handle_Success tests successful StartTLS handling.
func TestStartTLSHandler_Handle_Success(t *testing.T) {
	tlsConfig := createTestTLSConfig(t)
	handler := NewStartTLSHandler(tlsConfig)

	// Create a mock connection
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	conn := NewConnection(server, nil)
	tlsConn := NewTLSConnection(conn)

	req := &ExtendedRequest{
		OID: StartTLSOID,
	}

	resp, err := handler.Handle(tlsConn, req)
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if resp.Result.ResultCode != ldap.ResultSuccess {
		t.Errorf("expected ResultSuccess, got %v", resp.Result.ResultCode)
	}

	if resp.OID != StartTLSOID {
		t.Errorf("expected OID %q, got %q", StartTLSOID, resp.OID)
	}
}

// TestTLSConnection_IsTLS tests the IsTLS method.
func TestTLSConnection_IsTLS(t *testing.T) {
	// Test with plain connection
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	conn := NewConnection(server, nil)
	tlsConn := NewTLSConnection(conn)

	if tlsConn.IsTLS() {
		t.Error("expected IsTLS to return false for plain connection")
	}
}

// TestTLSConnection_IsTLS_AlreadyTLS tests IsTLS with an already-TLS connection.
func TestTLSConnection_IsTLS_AlreadyTLS(t *testing.T) {
	tlsConfig := createTestTLSConfig(t)

	// Create a TLS listener
	listener, err := tls.Listen("tcp", "127.0.0.1:0", tlsConfig)
	if err != nil {
		t.Fatalf("failed to create TLS listener: %v", err)
	}
	defer listener.Close()

	// Channel to signal errors
	errChan := make(chan error, 1)

	// Connect with TLS client
	go func() {
		clientConfig := &tls.Config{
			InsecureSkipVerify: true,
		}
		clientConn, err := tls.Dial("tcp", listener.Addr().String(), clientConfig)
		if err != nil {
			errChan <- err
			return
		}
		defer clientConn.Close()
		// Keep connection alive briefly
		time.Sleep(50 * time.Millisecond)
		errChan <- nil
	}()

	// Accept the connection
	serverConn, err := listener.Accept()
	if err != nil {
		t.Fatalf("failed to accept connection: %v", err)
	}
	defer serverConn.Close()

	// Perform TLS handshake on server side
	tlsServerConn := serverConn.(*tls.Conn)
	if err := tlsServerConn.Handshake(); err != nil {
		t.Fatalf("server TLS handshake failed: %v", err)
	}

	conn := NewConnection(tlsServerConn, nil)
	tlsConn := NewTLSConnection(conn)

	if !tlsConn.IsTLS() {
		t.Error("expected IsTLS to return true for TLS connection")
	}

	// Wait for client
	if err := <-errChan; err != nil {
		t.Fatalf("client error: %v", err)
	}
}

// TestStartTLSHandler_Handle_AlreadyTLS tests handling StartTLS on already-TLS connection.
func TestStartTLSHandler_Handle_AlreadyTLS(t *testing.T) {
	tlsConfig := createTestTLSConfig(t)
	handler := NewStartTLSHandler(tlsConfig)

	// Create a TLS listener
	listener, err := tls.Listen("tcp", "127.0.0.1:0", tlsConfig)
	if err != nil {
		t.Fatalf("failed to create TLS listener: %v", err)
	}
	defer listener.Close()

	// Channel to signal errors
	errChan := make(chan error, 1)

	// Connect with TLS client
	go func() {
		clientConfig := &tls.Config{
			InsecureSkipVerify: true,
		}
		clientConn, err := tls.Dial("tcp", listener.Addr().String(), clientConfig)
		if err != nil {
			errChan <- err
			return
		}
		defer clientConn.Close()
		time.Sleep(50 * time.Millisecond)
		errChan <- nil
	}()

	// Accept the connection
	serverConn, err := listener.Accept()
	if err != nil {
		t.Fatalf("failed to accept connection: %v", err)
	}
	defer serverConn.Close()

	// Perform TLS handshake on server side
	tlsServerConn := serverConn.(*tls.Conn)
	if err := tlsServerConn.Handshake(); err != nil {
		t.Fatalf("server TLS handshake failed: %v", err)
	}

	conn := NewConnection(tlsServerConn, nil)
	tlsConn := NewTLSConnection(conn)

	req := &ExtendedRequest{
		OID: StartTLSOID,
	}

	resp, err := handler.Handle(tlsConn, req)
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if resp.Result.ResultCode != ldap.ResultOperationsError {
		t.Errorf("expected ResultOperationsError for already-TLS connection, got %v", resp.Result.ResultCode)
	}

	// Wait for client
	if err := <-errChan; err != nil {
		t.Fatalf("client error: %v", err)
	}
}

// TestTLSConnection_UpgradeToTLS tests upgrading a connection to TLS.
func TestTLSConnection_UpgradeToTLS(t *testing.T) {
	tlsConfig := createTestTLSConfig(t)

	// Create a plain TCP listener
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer listener.Close()

	// Channel to signal completion
	done := make(chan error, 1)

	// Start client that will upgrade to TLS
	go func() {
		clientConn, err := net.Dial("tcp", listener.Addr().String())
		if err != nil {
			done <- err
			return
		}
		defer clientConn.Close()

		// Wait a bit for server to be ready
		time.Sleep(50 * time.Millisecond)

		// Upgrade to TLS
		clientTLSConfig := &tls.Config{
			InsecureSkipVerify: true,
		}
		tlsClientConn := tls.Client(clientConn, clientTLSConfig)
		if err := tlsClientConn.Handshake(); err != nil {
			done <- err
			return
		}

		// Send some data to verify TLS works
		_, err = tlsClientConn.Write([]byte("hello"))
		done <- err
	}()

	// Accept the connection
	serverConn, err := listener.Accept()
	if err != nil {
		t.Fatalf("failed to accept connection: %v", err)
	}
	defer serverConn.Close()

	conn := NewConnection(serverConn, nil)
	tlsConn := NewTLSConnection(conn)

	// Verify not TLS initially
	if tlsConn.IsTLS() {
		t.Error("expected IsTLS to return false before upgrade")
	}

	// Upgrade to TLS
	if err := tlsConn.UpgradeToTLS(tlsConfig); err != nil {
		t.Fatalf("UpgradeToTLS failed: %v", err)
	}

	// Verify now TLS
	if !tlsConn.IsTLS() {
		t.Error("expected IsTLS to return true after upgrade")
	}

	// Verify TLS connection state is available
	state := tlsConn.GetTLSConnectionState()
	if state == nil {
		t.Error("expected TLS connection state to be available")
	}

	// Wait for client
	if err := <-done; err != nil {
		t.Fatalf("client error: %v", err)
	}
}

// TestTLSConnection_UpgradeToTLS_AlreadyTLS tests upgrading an already-TLS connection.
func TestTLSConnection_UpgradeToTLS_AlreadyTLS(t *testing.T) {
	tlsConfig := createTestTLSConfig(t)

	// Create a TLS listener
	listener, err := tls.Listen("tcp", "127.0.0.1:0", tlsConfig)
	if err != nil {
		t.Fatalf("failed to create TLS listener: %v", err)
	}
	defer listener.Close()

	// Channel to signal errors
	errChan := make(chan error, 1)

	// Connect with TLS client
	go func() {
		clientConfig := &tls.Config{
			InsecureSkipVerify: true,
		}
		clientConn, err := tls.Dial("tcp", listener.Addr().String(), clientConfig)
		if err != nil {
			errChan <- err
			return
		}
		defer clientConn.Close()
		time.Sleep(50 * time.Millisecond)
		errChan <- nil
	}()

	// Accept the connection
	serverConn, err := listener.Accept()
	if err != nil {
		t.Fatalf("failed to accept connection: %v", err)
	}
	defer serverConn.Close()

	// Perform TLS handshake on server side
	tlsServerConn := serverConn.(*tls.Conn)
	if err := tlsServerConn.Handshake(); err != nil {
		t.Fatalf("server TLS handshake failed: %v", err)
	}

	conn := NewConnection(tlsServerConn, nil)
	tlsConn := NewTLSConnection(conn)

	// Try to upgrade already-TLS connection
	err = tlsConn.UpgradeToTLS(tlsConfig)
	if err != ErrAlreadyTLS {
		t.Errorf("expected ErrAlreadyTLS, got %v", err)
	}

	// Wait for client
	if err := <-errChan; err != nil {
		t.Fatalf("client error: %v", err)
	}
}

// TestTLSConnection_GetTLSConnectionState_NotTLS tests getting state from non-TLS connection.
func TestTLSConnection_GetTLSConnectionState_NotTLS(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	conn := NewConnection(server, nil)
	tlsConn := NewTLSConnection(conn)

	state := tlsConn.GetTLSConnectionState()
	if state != nil {
		t.Error("expected nil TLS connection state for non-TLS connection")
	}
}

// TestParseExtendedRequest tests parsing an ExtendedRequest.
func TestParseExtendedRequest(t *testing.T) {
	tests := []struct {
		name        string
		data        []byte
		expectedOID string
		expectedVal []byte
		expectError bool
	}{
		{
			name: "StartTLS request without value",
			// Context tag [0] (0x80), length 22, then the OID string
			data: []byte{
				0x80, 0x16, // Context tag [0], length 22
				'1', '.', '3', '.', '6', '.', '1', '.', '4', '.', '1', '.', '1', '4', '6', '6', '.', '2', '0', '0', '3', '7',
			},
			expectedOID: StartTLSOID,
			expectedVal: nil,
			expectError: false,
		},
		{
			name: "Extended request with value",
			data: []byte{
				0x80, 0x05, // Context tag [0], length 5
				'1', '.', '2', '.', '3',
				0x81, 0x04, // Context tag [1], length 4
				't', 'e', 's', 't',
			},
			expectedOID: "1.2.3",
			expectedVal: []byte("test"),
			expectError: false,
		},
		{
			name:        "Empty data",
			data:        []byte{},
			expectError: true,
		},
		{
			name: "Missing OID tag",
			data: []byte{
				0x81, 0x04, // Context tag [1] instead of [0]
				't', 'e', 's', 't',
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := ParseExtendedRequest(tt.data)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
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

// TestCreateExtendedResponse tests creating an ExtendedResponse message.
func TestCreateExtendedResponse(t *testing.T) {
	tests := []struct {
		name      string
		messageID int
		resp      *ExtendedResponse
	}{
		{
			name:      "Success response with OID",
			messageID: 1,
			resp: &ExtendedResponse{
				Result: OperationResult{
					ResultCode: ldap.ResultSuccess,
				},
				OID: StartTLSOID,
			},
		},
		{
			name:      "Error response",
			messageID: 2,
			resp: &ExtendedResponse{
				Result: OperationResult{
					ResultCode:        ldap.ResultOperationsError,
					DiagnosticMessage: "already using TLS",
				},
			},
		},
		{
			name:      "Response with value",
			messageID: 3,
			resp: &ExtendedResponse{
				Result: OperationResult{
					ResultCode: ldap.ResultSuccess,
				},
				OID:   "1.2.3.4",
				Value: []byte("response data"),
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
		})
	}
}

// TestHandleStartTLS_FullFlow tests the complete StartTLS flow.
func TestHandleStartTLS_FullFlow(t *testing.T) {
	tlsConfig := createTestTLSConfig(t)
	handler := NewStartTLSHandler(tlsConfig)

	// Create a plain TCP listener
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer listener.Close()

	// Channel to signal completion
	done := make(chan error, 1)

	// Start client
	go func() {
		clientConn, err := net.Dial("tcp", listener.Addr().String())
		if err != nil {
			done <- err
			return
		}
		defer clientConn.Close()

		// Read the extended response (we'll just read some bytes)
		buf := make([]byte, 256)
		_, err = clientConn.Read(buf)
		if err != nil {
			done <- err
			return
		}

		// Upgrade to TLS
		clientTLSConfig := &tls.Config{
			InsecureSkipVerify: true,
		}
		tlsClientConn := tls.Client(clientConn, clientTLSConfig)
		if err := tlsClientConn.Handshake(); err != nil {
			done <- err
			return
		}

		// Send test data over TLS
		_, err = tlsClientConn.Write([]byte("secure data"))
		done <- err
	}()

	// Accept the connection
	serverConn, err := listener.Accept()
	if err != nil {
		t.Fatalf("failed to accept connection: %v", err)
	}
	defer serverConn.Close()

	conn := NewConnection(serverConn, nil)
	tlsConn := NewTLSConnection(conn)

	req := &ExtendedRequest{
		OID: StartTLSOID,
	}

	// Handle the StartTLS request
	err = HandleStartTLS(tlsConn, 1, req, handler)
	if err != nil {
		t.Fatalf("HandleStartTLS failed: %v", err)
	}

	// Verify connection is now TLS
	if !tlsConn.IsTLS() {
		t.Error("expected connection to be TLS after HandleStartTLS")
	}

	// Read the test data from client
	buf := make([]byte, 256)
	n, err := tlsConn.netConn.Read(buf)
	if err != nil {
		t.Fatalf("failed to read from TLS connection: %v", err)
	}

	if string(buf[:n]) != "secure data" {
		t.Errorf("received data = %q, want %q", string(buf[:n]), "secure data")
	}

	// Wait for client
	if err := <-done; err != nil {
		t.Fatalf("client error: %v", err)
	}
}

// TestStartTLSHandler_UpgradeToTLS_NoConfig tests upgrade with no TLS config.
func TestStartTLSHandler_UpgradeToTLS_NoConfig(t *testing.T) {
	handler := NewStartTLSHandler(nil)

	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	conn := NewConnection(server, nil)
	tlsConn := NewTLSConnection(conn)

	err := handler.UpgradeToTLS(tlsConn)
	if err != ErrNoTLSConfig {
		t.Errorf("expected ErrNoTLSConfig, got %v", err)
	}
}

// TestTLSConnection_WriteExtendedResponse tests writing an extended response.
func TestTLSConnection_WriteExtendedResponse(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	conn := NewConnection(server, nil)
	tlsConn := NewTLSConnection(conn)

	// Write response in goroutine
	done := make(chan error, 1)
	go func() {
		resp := &ExtendedResponse{
			Result: OperationResult{
				ResultCode: ldap.ResultSuccess,
			},
			OID: StartTLSOID,
		}
		done <- tlsConn.WriteExtendedResponse(1, resp)
	}()

	// Read from client side
	buf := make([]byte, 256)
	n, err := client.Read(buf)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	if n == 0 {
		t.Error("expected to read some data")
	}

	// Wait for write to complete
	if err := <-done; err != nil {
		t.Fatalf("WriteExtendedResponse failed: %v", err)
	}
}
