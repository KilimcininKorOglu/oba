// Package tests provides integration tests for the Oba LDAP server.
// These tests verify end-to-end functionality using actual LDAP protocol communication.
package tests

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
	"os"
	"sync"
	"testing"
	"time"

	"github.com/KilimcininKorOglu/oba/internal/backend"
	"github.com/KilimcininKorOglu/oba/internal/config"
	"github.com/KilimcininKorOglu/oba/internal/ldap"
	"github.com/KilimcininKorOglu/oba/internal/logging"
	"github.com/KilimcininKorOglu/oba/internal/server"
	"github.com/KilimcininKorOglu/oba/internal/storage"
	"github.com/KilimcininKorOglu/oba/internal/storage/engine"
)

// TestConfig holds configuration for integration tests.
type TestConfig struct {
	Address      string
	TLSAddress   string
	BaseDN       string
	RootDN       string
	RootPassword string
	TLSConfig    *tls.Config
}

// DefaultTestConfig returns a default test configuration.
func DefaultTestConfig() *TestConfig {
	return &TestConfig{
		Address:      "127.0.0.1:0", // Use port 0 for random available port
		TLSAddress:   "127.0.0.1:0",
		BaseDN:       "dc=test,dc=com",
		RootDN:       "cn=admin,dc=test,dc=com",
		RootPassword: "secret",
	}
}

// TestServer wraps the LDAP server for integration testing.
type TestServer struct {
	config     *TestConfig
	listener   net.Listener
	tlsConfig  *tls.Config
	handler    *server.Handler
	backend    *backend.ObaBackend
	engine     storage.StorageEngine
	dataDir    string
	running    bool
	mu         sync.Mutex
	wg         sync.WaitGroup
	done       chan struct{}
	ldapServer *server.LDAPServer
}

// NewTestServer creates a new test server with the given configuration.
func NewTestServer(cfg *TestConfig) (*TestServer, error) {
	if cfg == nil {
		cfg = DefaultTestConfig()
	}

	// Create temporary directory for storage
	dataDir, err := os.MkdirTemp("", "oba-test-*")
	if err != nil {
		return nil, err
	}

	// Create storage engine
	eng, err := engine.Open(dataDir, storage.DefaultEngineOptions())
	if err != nil {
		os.RemoveAll(dataDir)
		return nil, err
	}

	// Create backend
	backendCfg := &config.Config{
		Directory: config.DirectoryConfig{
			BaseDN:       cfg.BaseDN,
			RootDN:       cfg.RootDN,
			RootPassword: "{CLEARTEXT}" + cfg.RootPassword,
		},
	}
	be := backend.NewBackend(eng, backendCfg)

	// Create handler
	handler := server.NewHandler()

	// Set up bind handler
	bindConfig := &server.BindConfig{
		Backend:        be,
		AllowAnonymous: true,
		RootDN:         cfg.RootDN,
		RootPassword:   "{CLEARTEXT}" + cfg.RootPassword,
	}
	bindHandler := server.NewBindHandler(bindConfig)
	handler.SetBindHandler(server.CreateBindHandler(bindHandler))

	// Set up search handler
	searchHandler := createSearchHandler(be, eng, cfg.BaseDN)
	handler.SetSearchHandler(searchHandler)

	// Set up add handler
	addHandler := createAddHandler(be)
	handler.SetAddHandler(addHandler)

	// Set up delete handler
	deleteHandler := createDeleteHandler(be)
	handler.SetDeleteHandler(deleteHandler)

	// Set up modify handler
	modifyHandler := createModifyHandler(be)
	handler.SetModifyHandler(modifyHandler)

	return &TestServer{
		config:  cfg,
		handler: handler,
		backend: be,
		engine:  eng,
		dataDir: dataDir,
		done:    make(chan struct{}),
	}, nil
}

// Start starts the test server.
func (s *TestServer) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return nil
	}

	// Create TCP listener
	listener, err := net.Listen("tcp", s.config.Address)
	if err != nil {
		return err
	}
	s.listener = listener

	// Update config with actual address
	s.config.Address = listener.Addr().String()

	s.running = true
	s.done = make(chan struct{})

	// Start accepting connections
	s.wg.Add(1)
	go s.acceptLoop()

	return nil
}

// Stop stops the test server gracefully.
func (s *TestServer) Stop() error {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = false
	close(s.done)

	if s.listener != nil {
		s.listener.Close()
	}
	s.mu.Unlock()

	// Wait for all connections to finish
	s.wg.Wait()

	// Close storage engine
	if s.engine != nil {
		s.engine.Close()
	}

	// Clean up data directory
	if s.dataDir != "" {
		os.RemoveAll(s.dataDir)
	}

	return nil
}

// Address returns the server's listening address.
func (s *TestServer) Address() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener != nil {
		return s.listener.Addr().String()
	}
	return s.config.Address
}

// acceptLoop accepts incoming connections.
func (s *TestServer) acceptLoop() {
	defer s.wg.Done()

	for {
		select {
		case <-s.done:
			return
		default:
		}

		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.done:
				return
			default:
				continue
			}
		}

		s.wg.Add(1)
		go s.handleConnection(conn)
	}
}

// handleConnection handles a single client connection.
func (s *TestServer) handleConnection(conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()

	serverRef := &server.Server{
		Handler: s.handler,
		Logger:  logging.NewNop(),
	}

	ldapConn := server.NewConnection(conn, serverRef)
	ldapConn.Handle()
}

// Backend returns the backend for direct manipulation in tests.
func (s *TestServer) Backend() *backend.ObaBackend {
	return s.backend
}

// Engine returns the storage engine for direct manipulation in tests.
func (s *TestServer) Engine() storage.StorageEngine {
	return s.engine
}

// Config returns the test configuration.
func (s *TestServer) Config() *TestConfig {
	return s.config
}

// searchBackendWrapper wraps the backend to implement SearchBackend interface.
type searchBackendWrapper struct {
	backend *backend.ObaBackend
	engine  storage.StorageEngine
}

func (w *searchBackendWrapper) GetEntry(dn string) (*storage.Entry, error) {
	txn, err := w.engine.Begin()
	if err != nil {
		return nil, err
	}
	defer w.engine.Rollback(txn)
	return w.engine.Get(txn, dn)
}

func (w *searchBackendWrapper) SearchByDN(baseDN string, scope storage.Scope) storage.Iterator {
	txn, err := w.engine.Begin()
	if err != nil {
		return &emptyIterator{}
	}
	return w.engine.SearchByDN(txn, baseDN, scope)
}

// emptyIterator is an iterator that returns no results.
type emptyIterator struct{}

func (e *emptyIterator) Next() bool            { return false }
func (e *emptyIterator) Entry() *storage.Entry { return nil }
func (e *emptyIterator) Error() error          { return nil }
func (e *emptyIterator) Close()                {}

// createSearchHandler creates a search handler for the test server.
func createSearchHandler(be *backend.ObaBackend, eng storage.StorageEngine, baseDN string) server.SearchHandler {
	wrapper := &searchBackendWrapper{backend: be, engine: eng}
	searchConfig := &server.SearchConfig{
		Backend:          wrapper,
		SearchBackend:    wrapper,
		MaxSizeLimit:     1000,
		MaxTimeLimit:     60,
		DefaultSizeLimit: 100,
		DefaultTimeLimit: 30,
	}
	searchHandler := server.NewSearchHandler(searchConfig)
	return server.CreateSearchHandler(searchHandler)
}

// createAddHandler creates an add handler for the test server.
func createAddHandler(be *backend.ObaBackend) server.AddHandler {
	return func(conn *server.Connection, req *ldap.AddRequest) *server.OperationResult {
		// Create entry from request
		entry := backend.NewEntry(req.Entry)
		for _, attr := range req.Attributes {
			var values []string
			for _, v := range attr.Values {
				values = append(values, string(v))
			}
			entry.SetAttribute(attr.Type, values...)
		}

		// Add entry
		err := be.AddWithBindDN(entry, conn.BindDN())
		if err != nil {
			resultCode := ldap.ResultOperationsError
			if err == backend.ErrEntryExists {
				resultCode = ldap.ResultEntryAlreadyExists
			} else if err == backend.ErrInvalidEntry {
				resultCode = ldap.ResultInvalidDNSyntax
			}
			return &server.OperationResult{
				ResultCode:        resultCode,
				DiagnosticMessage: err.Error(),
			}
		}

		return &server.OperationResult{
			ResultCode: ldap.ResultSuccess,
		}
	}
}

// createDeleteHandler creates a delete handler for the test server.
func createDeleteHandler(be *backend.ObaBackend) server.DeleteHandler {
	return func(conn *server.Connection, req *ldap.DeleteRequest) *server.OperationResult {
		// Check for children
		hasChildren, err := be.HasChildren(req.DN)
		if err != nil {
			return &server.OperationResult{
				ResultCode:        ldap.ResultOperationsError,
				DiagnosticMessage: err.Error(),
			}
		}
		if hasChildren {
			return &server.OperationResult{
				ResultCode:        ldap.ResultNotAllowedOnNonLeaf,
				DiagnosticMessage: "entry has children",
			}
		}

		// Delete entry
		err = be.Delete(req.DN)
		if err != nil {
			resultCode := ldap.ResultOperationsError
			if err == backend.ErrEntryNotFound {
				resultCode = ldap.ResultNoSuchObject
			}
			return &server.OperationResult{
				ResultCode:        resultCode,
				DiagnosticMessage: err.Error(),
			}
		}

		return &server.OperationResult{
			ResultCode: ldap.ResultSuccess,
		}
	}
}

// createModifyHandler creates a modify handler for the test server.
func createModifyHandler(be *backend.ObaBackend) server.ModifyHandler {
	return func(conn *server.Connection, req *ldap.ModifyRequest) *server.OperationResult {
		// Convert LDAP changes to backend modifications
		var changes []backend.Modification
		for _, change := range req.Changes {
			var values []string
			for _, v := range change.Attribute.Values {
				values = append(values, string(v))
			}
			mod := backend.Modification{
				Type:      backend.ModificationType(change.Operation),
				Attribute: change.Attribute.Type,
				Values:    values,
			}
			changes = append(changes, mod)
		}

		// Modify entry
		err := be.ModifyWithBindDN(req.Object, changes, conn.BindDN())
		if err != nil {
			resultCode := ldap.ResultOperationsError
			if err == backend.ErrEntryNotFound {
				resultCode = ldap.ResultNoSuchObject
			}
			return &server.OperationResult{
				ResultCode:        resultCode,
				DiagnosticMessage: err.Error(),
			}
		}

		return &server.OperationResult{
			ResultCode: ldap.ResultSuccess,
		}
	}
}

// generateTestCertificate creates a self-signed certificate for testing.
func generateTestCertificate() (certPEM, keyPEM []byte, err error) {
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
	certPEM, keyPEM, err := generateTestCertificate()
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
