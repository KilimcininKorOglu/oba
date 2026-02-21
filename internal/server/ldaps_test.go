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

	"github.com/KilimcininKorOglu/oba/internal/ldap"
	"github.com/KilimcininKorOglu/oba/internal/logging"
)

// generateLDAPSTestCertificate creates a self-signed certificate for testing.
func generateLDAPSTestCertificate() (certPEM, keyPEM []byte, err error) {
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

// createLDAPSTestTLSConfig creates a TLS config for testing.
func createLDAPSTestTLSConfig(t *testing.T) *tls.Config {
	certPEM, keyPEM, err := generateLDAPSTestCertificate()
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

// TestDefaultLDAPSAddress verifies the default LDAPS address constant.
func TestDefaultLDAPSAddress(t *testing.T) {
	expected := ":636"
	if DefaultLDAPSAddress != expected {
		t.Errorf("DefaultLDAPSAddress = %q, want %q", DefaultLDAPSAddress, expected)
	}
}

// TestNewLDAPSListener tests creating a new LDAPS listener.
func TestNewLDAPSListener(t *testing.T) {
	tlsConfig := createLDAPSTestTLSConfig(t)

	tests := []struct {
		name        string
		config      *LDAPSConfig
		server      *LDAPServer
		expectError error
	}{
		{
			name:        "nil config",
			config:      nil,
			server:      nil,
			expectError: ErrLDAPSNoTLSConfig,
		},
		{
			name: "nil TLS config",
			config: &LDAPSConfig{
				Address:   ":636",
				TLSConfig: nil,
			},
			server:      nil,
			expectError: ErrLDAPSNoTLSConfig,
		},
		{
			name: "valid config with default address",
			config: &LDAPSConfig{
				Address:   "",
				TLSConfig: tlsConfig,
			},
			server:      nil,
			expectError: nil,
		},
		{
			name: "valid config with custom address",
			config: &LDAPSConfig{
				Address:   ":10636",
				TLSConfig: tlsConfig,
			},
			server:      nil,
			expectError: nil,
		},
		{
			name: "valid config with server",
			config: &LDAPSConfig{
				Address:   ":10636",
				TLSConfig: tlsConfig,
			},
			server:      NewLDAPServer(),
			expectError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			listener, err := NewLDAPSListener(tt.config, tt.server)

			if tt.expectError != nil {
				if err != tt.expectError {
					t.Errorf("expected error %v, got %v", tt.expectError, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if listener == nil {
				t.Fatal("expected non-nil listener")
			}

			// Verify default address is set
			if tt.config.Address == "" {
				if listener.config.Address != DefaultLDAPSAddress {
					t.Errorf("expected default address %q, got %q", DefaultLDAPSAddress, listener.config.Address)
				}
			}
		})
	}
}

// TestLDAPSListener_StartStop tests starting and stopping the LDAPS listener.
func TestLDAPSListener_StartStop(t *testing.T) {
	tlsConfig := createLDAPSTestTLSConfig(t)

	config := &LDAPSConfig{
		Address:   "127.0.0.1:0", // Use port 0 for random available port
		TLSConfig: tlsConfig,
	}

	listener, err := NewLDAPSListener(config, nil)
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}

	// Verify not running initially
	if listener.IsRunning() {
		t.Error("expected listener to not be running initially")
	}

	// Start the listener
	if err := listener.Start(); err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}

	// Verify running
	if !listener.IsRunning() {
		t.Error("expected listener to be running after Start")
	}

	// Verify address is set
	addr := listener.Address()
	if addr == "" {
		t.Error("expected non-empty address")
	}

	// Try to start again - should fail
	if err := listener.Start(); err != ErrLDAPSAlreadyRunning {
		t.Errorf("expected ErrLDAPSAlreadyRunning, got %v", err)
	}

	// Stop the listener
	if err := listener.Stop(); err != nil {
		t.Fatalf("failed to stop listener: %v", err)
	}

	// Verify not running
	if listener.IsRunning() {
		t.Error("expected listener to not be running after Stop")
	}

	// Try to stop again - should fail
	if err := listener.Stop(); err != ErrLDAPSNotRunning {
		t.Errorf("expected ErrLDAPSNotRunning, got %v", err)
	}
}

// TestLDAPSListener_AcceptConnection tests accepting TLS connections.
func TestLDAPSListener_AcceptConnection(t *testing.T) {
	tlsConfig := createLDAPSTestTLSConfig(t)

	server := NewLDAPServer()
	server.SetLogger(logging.NewNop())

	config := &LDAPSConfig{
		Address:   "127.0.0.1:0",
		TLSConfig: tlsConfig,
	}

	listener, err := NewLDAPSListener(config, server)
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}

	if err := listener.Start(); err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}
	defer listener.Stop()

	// Connect with TLS client
	clientConfig := &tls.Config{
		InsecureSkipVerify: true,
	}

	conn, err := tls.Dial("tcp", listener.Address(), clientConfig)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	// Verify TLS handshake completed
	state := conn.ConnectionState()
	if !state.HandshakeComplete {
		t.Error("expected TLS handshake to be complete")
	}

	// Verify TLS version
	if state.Version < tls.VersionTLS12 {
		t.Errorf("expected TLS 1.2 or higher, got 0x%04x", state.Version)
	}
}

// TestLDAPSListener_RejectInvalidCertificate tests rejecting connections without valid client certificates.
func TestLDAPSListener_RejectInvalidCertificate(t *testing.T) {
	// Create server with client certificate verification
	certPEM, keyPEM, err := generateLDAPSTestCertificate()
	if err != nil {
		t.Fatalf("failed to generate certificate: %v", err)
	}

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("failed to load certificate: %v", err)
	}

	// Parse the certificate to get the CA
	x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatalf("failed to parse certificate: %v", err)
	}

	// Create CA pool with our certificate
	caPool := x509.NewCertPool()
	caPool.AddCert(x509Cert)

	// Server requires client certificate
	serverTLSConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    caPool,
	}

	config := &LDAPSConfig{
		Address:   "127.0.0.1:0",
		TLSConfig: serverTLSConfig,
	}

	listener, err := NewLDAPSListener(config, nil)
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}

	if err := listener.Start(); err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}
	defer listener.Stop()

	// Try to connect without client certificate
	// The connection will be established but any data exchange should fail
	clientConfig := &tls.Config{
		InsecureSkipVerify: true,
	}

	conn, err := tls.Dial("tcp", listener.Address(), clientConfig)
	if err != nil {
		// Connection failed during dial - this is expected behavior
		return
	}
	defer conn.Close()

	// If we got here, try to send data - it should fail
	// because the server will reject the connection during handshake
	_, err = conn.Write([]byte("test"))
	if err != nil {
		// Write failed - expected because server rejected the connection
		return
	}

	// Try to read - should fail
	buf := make([]byte, 1024)
	conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	_, err = conn.Read(buf)
	if err == nil {
		t.Error("expected read to fail without client certificate")
	}
}

// TestLDAPSListener_LDAPOperations tests LDAP operations over TLS.
func TestLDAPSListener_LDAPOperations(t *testing.T) {
	tlsConfig := createLDAPSTestTLSConfig(t)

	server := NewLDAPServer()
	server.SetLogger(logging.NewNop())

	// Set up a bind handler that accepts anonymous binds
	handler := NewHandler()
	handler.SetBindHandler(func(conn *Connection, req *ldap.BindRequest) *OperationResult {
		if req.IsAnonymous() {
			return &OperationResult{
				ResultCode: ldap.ResultSuccess,
			}
		}
		return &OperationResult{
			ResultCode:        ldap.ResultInvalidCredentials,
			DiagnosticMessage: "invalid credentials",
		}
	})
	server.SetHandler(handler)

	config := &LDAPSConfig{
		Address:   "127.0.0.1:0",
		TLSConfig: tlsConfig,
	}

	listener, err := NewLDAPSListener(config, server)
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}

	if err := listener.Start(); err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}
	defer listener.Stop()

	// Connect with TLS client
	clientConfig := &tls.Config{
		InsecureSkipVerify: true,
	}

	conn, err := tls.Dial("tcp", listener.Address(), clientConfig)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	// Send an anonymous bind request
	bindReq := createAnonymousBindRequest(1)
	data, err := bindReq.Encode()
	if err != nil {
		t.Fatalf("failed to encode bind request: %v", err)
	}

	if _, err := conn.Write(data); err != nil {
		t.Fatalf("failed to send bind request: %v", err)
	}

	// Read response
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	if n == 0 {
		t.Error("expected non-empty response")
	}

	// Parse the response
	resp, err := ldap.ParseLDAPMessage(buf[:n])
	if err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.MessageID != 1 {
		t.Errorf("expected message ID 1, got %d", resp.MessageID)
	}

	if resp.Operation.Tag != ldap.ApplicationBindResponse {
		t.Errorf("expected BindResponse tag, got %d", resp.Operation.Tag)
	}
}

// createAnonymousBindRequest creates an anonymous bind request message.
func createAnonymousBindRequest(messageID int) *ldap.LDAPMessage {
	// Create bind request: version=3, name="", simple auth=""
	req := &ldap.BindRequest{
		Version:        3,
		Name:           "",
		AuthMethod:     ldap.AuthMethodSimple,
		SimplePassword: []byte{},
	}

	data, _ := req.Encode()

	return &ldap.LDAPMessage{
		MessageID: messageID,
		Operation: &ldap.RawOperation{
			Tag:  ldap.ApplicationBindRequest,
			Data: data,
		},
	}
}

// TestLDAPServer_LDAPS tests the LDAPServer LDAPS methods.
func TestLDAPServer_LDAPS(t *testing.T) {
	tlsConfig := createLDAPSTestTLSConfig(t)

	server := NewLDAPServer()
	server.SetLogger(logging.NewNop())

	// Try to start LDAPS without TLS config - should fail
	if err := server.StartLDAPS(""); err != ErrLDAPSNoTLSConfig {
		t.Errorf("expected ErrLDAPSNoTLSConfig, got %v", err)
	}

	// Set TLS config
	server.SetTLSConfig(tlsConfig)

	// Verify not running initially
	if server.IsLDAPSRunning() {
		t.Error("expected LDAPS to not be running initially")
	}

	// Start LDAPS with default address
	if err := server.StartLDAPS("127.0.0.1:0"); err != nil {
		t.Fatalf("failed to start LDAPS: %v", err)
	}

	// Verify running
	if !server.IsLDAPSRunning() {
		t.Error("expected LDAPS to be running")
	}

	// Verify address is set
	addr := server.LDAPSAddress()
	if addr == "" {
		t.Error("expected non-empty LDAPS address")
	}

	// Stop LDAPS
	if err := server.StopLDAPS(); err != nil {
		t.Fatalf("failed to stop LDAPS: %v", err)
	}

	// Verify not running
	if server.IsLDAPSRunning() {
		t.Error("expected LDAPS to not be running after stop")
	}

	// Verify address is empty
	addr = server.LDAPSAddress()
	if addr != "" {
		t.Errorf("expected empty LDAPS address, got %q", addr)
	}

	// Try to stop again - should fail
	if err := server.StopLDAPS(); err != ErrLDAPSNotRunning {
		t.Errorf("expected ErrLDAPSNotRunning, got %v", err)
	}
}

// TestLDAPSListener_GetTLSConfig tests getting the TLS configuration.
func TestLDAPSListener_GetTLSConfig(t *testing.T) {
	tlsConfig := createLDAPSTestTLSConfig(t)

	config := &LDAPSConfig{
		Address:   "127.0.0.1:0",
		TLSConfig: tlsConfig,
	}

	listener, err := NewLDAPSListener(config, nil)
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}

	gotConfig := listener.GetTLSConfig()
	if gotConfig != tlsConfig {
		t.Error("expected TLS config to match")
	}
}

// TestLDAPSListener_MultipleConnections tests handling multiple concurrent connections.
func TestLDAPSListener_MultipleConnections(t *testing.T) {
	tlsConfig := createLDAPSTestTLSConfig(t)

	server := NewLDAPServer()
	server.SetLogger(logging.NewNop())

	config := &LDAPSConfig{
		Address:   "127.0.0.1:0",
		TLSConfig: tlsConfig,
	}

	listener, err := NewLDAPSListener(config, server)
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}

	if err := listener.Start(); err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}
	defer listener.Stop()

	// Connect multiple clients
	numClients := 5
	conns := make([]*tls.Conn, numClients)

	clientConfig := &tls.Config{
		InsecureSkipVerify: true,
	}

	for i := 0; i < numClients; i++ {
		conn, err := tls.Dial("tcp", listener.Address(), clientConfig)
		if err != nil {
			t.Fatalf("client %d failed to connect: %v", i, err)
		}
		conns[i] = conn
	}

	// Verify all connections are established
	for i, conn := range conns {
		state := conn.ConnectionState()
		if !state.HandshakeComplete {
			t.Errorf("client %d: expected TLS handshake to be complete", i)
		}
	}

	// Close all connections
	for _, conn := range conns {
		conn.Close()
	}
}

// TestLDAPSListener_TLSVersions tests TLS version negotiation.
func TestLDAPSListener_TLSVersions(t *testing.T) {
	certPEM, keyPEM, err := generateLDAPSTestCertificate()
	if err != nil {
		t.Fatalf("failed to generate certificate: %v", err)
	}

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("failed to load certificate: %v", err)
	}

	tests := []struct {
		name          string
		serverMin     uint16
		serverMax     uint16
		clientMin     uint16
		clientMax     uint16
		expectVersion uint16
		expectError   bool
	}{
		{
			name:          "TLS 1.2 only",
			serverMin:     tls.VersionTLS12,
			serverMax:     tls.VersionTLS12,
			clientMin:     tls.VersionTLS12,
			clientMax:     tls.VersionTLS12,
			expectVersion: tls.VersionTLS12,
			expectError:   false,
		},
		{
			name:          "TLS 1.3 preferred",
			serverMin:     tls.VersionTLS12,
			serverMax:     tls.VersionTLS13,
			clientMin:     tls.VersionTLS12,
			clientMax:     tls.VersionTLS13,
			expectVersion: tls.VersionTLS13,
			expectError:   false,
		},
		{
			name:          "Server TLS 1.3 only, client TLS 1.2 only",
			serverMin:     tls.VersionTLS13,
			serverMax:     tls.VersionTLS13,
			clientMin:     tls.VersionTLS12,
			clientMax:     tls.VersionTLS12,
			expectVersion: 0,
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			serverTLSConfig := &tls.Config{
				Certificates: []tls.Certificate{cert},
				MinVersion:   tt.serverMin,
				MaxVersion:   tt.serverMax,
			}

			config := &LDAPSConfig{
				Address:   "127.0.0.1:0",
				TLSConfig: serverTLSConfig,
			}

			listener, err := NewLDAPSListener(config, nil)
			if err != nil {
				t.Fatalf("failed to create listener: %v", err)
			}

			if err := listener.Start(); err != nil {
				t.Fatalf("failed to start listener: %v", err)
			}
			defer listener.Stop()

			clientConfig := &tls.Config{
				InsecureSkipVerify: true,
				MinVersion:         tt.clientMin,
				MaxVersion:         tt.clientMax,
			}

			conn, err := tls.Dial("tcp", listener.Address(), clientConfig)
			if tt.expectError {
				if err == nil {
					conn.Close()
					t.Error("expected connection to fail")
				}
				return
			}

			if err != nil {
				t.Fatalf("failed to connect: %v", err)
			}
			defer conn.Close()

			state := conn.ConnectionState()
			if state.Version != tt.expectVersion {
				t.Errorf("expected TLS version 0x%04x, got 0x%04x", tt.expectVersion, state.Version)
			}
		})
	}
}

// TestNewLDAPServer tests creating a new LDAP server.
func TestNewLDAPServer(t *testing.T) {
	server := NewLDAPServer()

	if server == nil {
		t.Fatal("expected non-nil server")
	}

	if server.Handler == nil {
		t.Error("expected non-nil handler")
	}

	if server.Logger == nil {
		t.Error("expected non-nil logger")
	}
}

// TestLDAPServer_SetMethods tests the setter methods.
func TestLDAPServer_SetMethods(t *testing.T) {
	server := NewLDAPServer()

	// Test SetTLSConfig
	tlsConfig := createLDAPSTestTLSConfig(t)
	server.SetTLSConfig(tlsConfig)
	if server.TLSConfig != tlsConfig {
		t.Error("TLS config not set correctly")
	}

	// Test SetLogger
	logger := logging.NewNop()
	server.SetLogger(logger)
	if server.Logger != logger {
		t.Error("logger not set correctly")
	}

	// Test SetHandler
	handler := NewHandler()
	server.SetHandler(handler)
	if server.Handler != handler {
		t.Error("handler not set correctly")
	}
}
