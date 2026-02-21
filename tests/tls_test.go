// Package tests provides integration tests for the Oba LDAP server.
package tests

import (
	"crypto/tls"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/KilimcininKorOglu/oba/internal/ldap"
	"github.com/KilimcininKorOglu/oba/internal/logging"
	"github.com/KilimcininKorOglu/oba/internal/server"
)

// TestIntegrationTLS tests TLS connection functionality.
func TestIntegrationTLS(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	t.Run("ldaps_connection", func(t *testing.T) {
		testLDAPSConnection(t)
	})

	t.Run("ldaps_bind", func(t *testing.T) {
		testLDAPSBind(t)
	})

	t.Run("tls_version_negotiation", func(t *testing.T) {
		testTLSVersionNegotiation(t)
	})
}

// TLSTestServer wraps a test server with TLS support.
type TLSTestServer struct {
	*TestServer
	tlsListener net.Listener
	tlsConfig   *tls.Config
	tlsAddress  string
	tlsRunning  bool
	tlsMu       sync.Mutex
	tlsWg       sync.WaitGroup
	tlsDone     chan struct{}
}

// NewTLSTestServer creates a new test server with TLS support.
func NewTLSTestServer(t *testing.T) (*TLSTestServer, error) {
	srv, err := NewTestServer(nil)
	if err != nil {
		return nil, err
	}

	tlsConfig := createTestTLSConfig(t)

	return &TLSTestServer{
		TestServer: srv,
		tlsConfig:  tlsConfig,
		tlsDone:    make(chan struct{}),
	}, nil
}

// StartTLS starts the TLS listener.
func (s *TLSTestServer) StartTLS() error {
	s.tlsMu.Lock()
	defer s.tlsMu.Unlock()

	if s.tlsRunning {
		return nil
	}

	// Create TLS listener
	listener, err := tls.Listen("tcp", "127.0.0.1:0", s.tlsConfig)
	if err != nil {
		return err
	}
	s.tlsListener = listener
	s.tlsAddress = listener.Addr().String()

	s.tlsRunning = true
	s.tlsDone = make(chan struct{})

	// Start accepting TLS connections
	s.tlsWg.Add(1)
	go s.acceptTLSLoop()

	return nil
}

// StopTLS stops the TLS listener.
func (s *TLSTestServer) StopTLS() error {
	s.tlsMu.Lock()
	if !s.tlsRunning {
		s.tlsMu.Unlock()
		return nil
	}
	s.tlsRunning = false
	close(s.tlsDone)

	if s.tlsListener != nil {
		s.tlsListener.Close()
	}
	s.tlsMu.Unlock()

	s.tlsWg.Wait()
	return nil
}

// TLSAddress returns the TLS listener address.
func (s *TLSTestServer) TLSAddress() string {
	s.tlsMu.Lock()
	defer s.tlsMu.Unlock()
	return s.tlsAddress
}

// acceptTLSLoop accepts incoming TLS connections.
func (s *TLSTestServer) acceptTLSLoop() {
	defer s.tlsWg.Done()

	for {
		select {
		case <-s.tlsDone:
			return
		default:
		}

		conn, err := s.tlsListener.Accept()
		if err != nil {
			select {
			case <-s.tlsDone:
				return
			default:
				continue
			}
		}

		s.tlsWg.Add(1)
		go s.handleTLSConnection(conn)
	}
}

// handleTLSConnection handles a single TLS client connection.
func (s *TLSTestServer) handleTLSConnection(conn net.Conn) {
	defer s.tlsWg.Done()
	defer conn.Close()

	// Perform TLS handshake
	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		return
	}

	if err := tlsConn.Handshake(); err != nil {
		return
	}

	serverRef := &server.Server{
		Handler: s.handler,
		Logger:  logging.NewNop(),
	}

	ldapConn := server.NewConnection(tlsConn, serverRef)
	ldapConn.SetTLS(true)
	ldapConn.Handle()
}

// testLDAPSConnection tests basic LDAPS connection.
func testLDAPSConnection(t *testing.T) {
	srv, err := NewTLSTestServer(t)
	if err != nil {
		t.Fatalf("failed to create TLS test server: %v", err)
	}

	if err := srv.Start(); err != nil {
		t.Fatalf("failed to start test server: %v", err)
	}
	defer srv.Stop()

	if err := srv.StartTLS(); err != nil {
		t.Fatalf("failed to start TLS listener: %v", err)
	}
	defer srv.StopTLS()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Connect with TLS
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true, // Skip verification for self-signed cert
	}

	conn, err := tls.Dial("tcp", srv.TLSAddress(), tlsConfig)
	if err != nil {
		t.Fatalf("failed to connect with TLS: %v", err)
	}
	defer conn.Close()

	// Verify TLS connection state
	state := conn.ConnectionState()
	if !state.HandshakeComplete {
		t.Error("TLS handshake not complete")
	}

	if state.Version < tls.VersionTLS12 {
		t.Errorf("expected TLS 1.2 or higher, got version 0x%04x", state.Version)
	}
}

// testLDAPSBind tests bind operation over LDAPS.
func testLDAPSBind(t *testing.T) {
	srv, err := NewTLSTestServer(t)
	if err != nil {
		t.Fatalf("failed to create TLS test server: %v", err)
	}

	if err := srv.Start(); err != nil {
		t.Fatalf("failed to start test server: %v", err)
	}
	defer srv.Stop()

	if err := srv.StartTLS(); err != nil {
		t.Fatalf("failed to start TLS listener: %v", err)
	}
	defer srv.StopTLS()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Connect with TLS
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
	}

	conn, err := tls.Dial("tcp", srv.TLSAddress(), tlsConfig)
	if err != nil {
		t.Fatalf("failed to connect with TLS: %v", err)
	}
	defer conn.Close()

	cfg := srv.Config()

	// Send bind request
	bindReq := createBindRequest(1, 3, cfg.RootDN, cfg.RootPassword)
	if err := sendMessage(conn, bindReq); err != nil {
		t.Fatalf("failed to send bind request: %v", err)
	}

	// Read response
	resp, err := readMessage(conn)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	resultCode := parseBindResponse(resp)
	if resultCode != ldap.ResultSuccess {
		t.Errorf("expected success, got result code %d", resultCode)
	}
}

// testTLSVersionNegotiation tests TLS version negotiation.
func testTLSVersionNegotiation(t *testing.T) {
	srv, err := NewTLSTestServer(t)
	if err != nil {
		t.Fatalf("failed to create TLS test server: %v", err)
	}

	if err := srv.Start(); err != nil {
		t.Fatalf("failed to start test server: %v", err)
	}
	defer srv.Stop()

	if err := srv.StartTLS(); err != nil {
		t.Fatalf("failed to start TLS listener: %v", err)
	}
	defer srv.StopTLS()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	tests := []struct {
		name       string
		minVersion uint16
		maxVersion uint16
		shouldFail bool
	}{
		{"TLS 1.2", tls.VersionTLS12, tls.VersionTLS12, false},
		{"TLS 1.3", tls.VersionTLS13, tls.VersionTLS13, false},
		{"TLS 1.2-1.3", tls.VersionTLS12, tls.VersionTLS13, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tlsConfig := &tls.Config{
				InsecureSkipVerify: true,
				MinVersion:         tt.minVersion,
				MaxVersion:         tt.maxVersion,
			}

			conn, err := tls.Dial("tcp", srv.TLSAddress(), tlsConfig)
			if tt.shouldFail {
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
			if state.Version < tt.minVersion || state.Version > tt.maxVersion {
				t.Errorf("unexpected TLS version: 0x%04x", state.Version)
			}
		})
	}
}

// TestIntegrationConcurrent tests concurrent operations.
func TestIntegrationConcurrent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Start test server
	srv, err := NewTestServer(nil)
	if err != nil {
		t.Fatalf("failed to create test server: %v", err)
	}

	if err := srv.Start(); err != nil {
		t.Fatalf("failed to start test server: %v", err)
	}
	defer srv.Stop()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	t.Run("concurrent_binds", func(t *testing.T) {
		testConcurrentBinds(t, srv)
	})

	t.Run("concurrent_connections", func(t *testing.T) {
		testConcurrentConnections(t, srv)
	})
}

// testConcurrentBinds tests multiple concurrent bind operations.
func testConcurrentBinds(t *testing.T, srv *TestServer) {
	const numGoroutines = 10
	const numBindsPerGoroutine = 5

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*numBindsPerGoroutine)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for j := 0; j < numBindsPerGoroutine; j++ {
				conn, err := net.Dial("tcp", srv.Address())
				if err != nil {
					errors <- err
					continue
				}

				// Perform bind
				bindReq := createBindRequest(1, 3, srv.Config().RootDN, srv.Config().RootPassword)
				if err := sendMessage(conn, bindReq); err != nil {
					conn.Close()
					errors <- err
					continue
				}

				resp, err := readMessage(conn)
				if err != nil {
					conn.Close()
					errors <- err
					continue
				}

				resultCode := parseBindResponse(resp)
				if resultCode != ldap.ResultSuccess {
					errors <- &ldapError{code: resultCode, message: "bind failed"}
				}

				conn.Close()
			}
		}()
	}

	wg.Wait()
	close(errors)

	// Check for errors
	var errCount int
	for err := range errors {
		t.Errorf("concurrent bind error: %v", err)
		errCount++
	}

	if errCount > 0 {
		t.Errorf("total errors: %d", errCount)
	}
}

// testConcurrentConnections tests multiple concurrent connections.
func testConcurrentConnections(t *testing.T, srv *TestServer) {
	const numConnections = 20

	var wg sync.WaitGroup
	connections := make([]net.Conn, numConnections)
	errors := make(chan error, numConnections)

	// Open all connections
	for i := 0; i < numConnections; i++ {
		conn, err := net.Dial("tcp", srv.Address())
		if err != nil {
			errors <- err
			continue
		}
		connections[i] = conn
	}

	// Perform operations on all connections concurrently
	for i, conn := range connections {
		if conn == nil {
			continue
		}
		wg.Add(1)
		go func(c net.Conn, idx int) {
			defer wg.Done()
			defer c.Close()

			// Perform anonymous bind
			bindReq := createBindRequest(1, 3, "", "")
			if err := sendMessage(c, bindReq); err != nil {
				errors <- err
				return
			}

			resp, err := readMessage(c)
			if err != nil {
				errors <- err
				return
			}

			resultCode := parseBindResponse(resp)
			if resultCode != ldap.ResultSuccess {
				errors <- &ldapError{code: resultCode, message: "bind failed"}
			}
		}(conn, i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	var errCount int
	for err := range errors {
		t.Errorf("concurrent connection error: %v", err)
		errCount++
	}

	if errCount > 0 {
		t.Errorf("total errors: %d", errCount)
	}
}
