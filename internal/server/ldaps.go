// Package server provides the LDAP server implementation.
package server

import (
	"crypto/tls"
	"errors"
	"net"
	"sync"
	"sync/atomic"

	"github.com/KilimcininKorOglu/oba/internal/logging"
)

// LDAPS errors
var (
	// ErrLDAPSAlreadyRunning is returned when LDAPS listener is already running
	ErrLDAPSAlreadyRunning = errors.New("server: LDAPS listener already running")
	// ErrLDAPSNotRunning is returned when LDAPS listener is not running
	ErrLDAPSNotRunning = errors.New("server: LDAPS listener not running")
	// ErrLDAPSNoTLSConfig is returned when TLS configuration is not provided
	ErrLDAPSNoTLSConfig = errors.New("server: TLS configuration required for LDAPS")
	// ErrLDAPSInvalidAddress is returned when the address is invalid
	ErrLDAPSInvalidAddress = errors.New("server: invalid LDAPS address")
)

// LDAPSConfig holds the configuration for the LDAPS listener.
type LDAPSConfig struct {
	// Address is the address to listen on (default: ":636")
	Address string
	// TLSConfig is the TLS configuration for the listener
	TLSConfig *tls.Config
}

// DefaultLDAPSAddress is the default LDAPS port.
const DefaultLDAPSAddress = ":636"

// LDAPSListener manages the LDAPS (LDAP over TLS) listener.
// It accepts TLS-encrypted connections on a dedicated port.
type LDAPSListener struct {
	// config holds the LDAPS configuration
	config *LDAPSConfig
	// listener is the underlying TLS listener
	listener net.Listener
	// server is the parent server instance
	server *LDAPServer
	// running indicates whether the listener is running
	running atomic.Bool
	// mu protects concurrent access
	mu sync.RWMutex
	// wg tracks active connections
	wg sync.WaitGroup
	// done signals shutdown
	done chan struct{}
	// logger is the logger for this listener
	logger logging.Logger
}

// LDAPServer represents the full LDAP server with both plain and TLS listeners.
type LDAPServer struct {
	// Handler is the operation handler
	Handler *Handler
	// Logger is the server's logger
	Logger logging.Logger
	// TLSConfig is the TLS configuration for LDAPS and StartTLS
	TLSConfig *tls.Config
	// ldapListener is the plain LDAP listener (port 389)
	ldapListener net.Listener
	// ldapsListener is the LDAPS listener (port 636)
	ldapsListener *LDAPSListener
	// mu protects concurrent access
	mu sync.RWMutex
	// running indicates whether the server is running
	running atomic.Bool
	// wg tracks active goroutines
	wg sync.WaitGroup
	// done signals shutdown
	done chan struct{}
}

// NewLDAPSListener creates a new LDAPS listener with the given configuration.
func NewLDAPSListener(config *LDAPSConfig, server *LDAPServer) (*LDAPSListener, error) {
	if config == nil {
		return nil, ErrLDAPSNoTLSConfig
	}

	if config.TLSConfig == nil {
		return nil, ErrLDAPSNoTLSConfig
	}

	// Set default address if not provided
	address := config.Address
	if address == "" {
		address = DefaultLDAPSAddress
	}

	// Get logger from server or create a nop logger
	var logger logging.Logger
	if server != nil && server.Logger != nil {
		logger = server.Logger
	} else {
		logger = logging.NewNop()
	}

	return &LDAPSListener{
		config: &LDAPSConfig{
			Address:   address,
			TLSConfig: config.TLSConfig,
		},
		server: server,
		done:   make(chan struct{}),
		logger: logger,
	}, nil
}

// Start starts the LDAPS listener.
func (l *LDAPSListener) Start() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.running.Load() {
		return ErrLDAPSAlreadyRunning
	}

	// Create TLS listener
	listener, err := tls.Listen("tcp", l.config.Address, l.config.TLSConfig)
	if err != nil {
		return err
	}

	l.listener = listener
	l.done = make(chan struct{})
	l.running.Store(true)

	l.logger.Info("LDAPS listener started",
		"address", l.config.Address)

	// Start accepting connections
	l.wg.Add(1)
	go l.acceptLoop()

	return nil
}

// Stop stops the LDAPS listener gracefully.
func (l *LDAPSListener) Stop() error {
	l.mu.Lock()

	if !l.running.Load() {
		l.mu.Unlock()
		return ErrLDAPSNotRunning
	}

	l.running.Store(false)

	// Signal shutdown
	close(l.done)

	// Close the listener to unblock Accept()
	if l.listener != nil {
		l.listener.Close()
	}

	l.mu.Unlock()

	// Wait for all connections to finish
	l.wg.Wait()

	l.logger.Info("LDAPS listener stopped",
		"address", l.config.Address)

	return nil
}

// acceptLoop accepts incoming TLS connections.
func (l *LDAPSListener) acceptLoop() {
	defer l.wg.Done()

	for {
		// Check if we should stop
		select {
		case <-l.done:
			return
		default:
		}

		// Accept new connection
		conn, err := l.listener.Accept()
		if err != nil {
			// Check if we're shutting down
			select {
			case <-l.done:
				return
			default:
			}

			// Log error and continue
			l.logger.Warn("LDAPS accept error",
				"error", err.Error())
			continue
		}

		// Handle connection in a new goroutine
		l.wg.Add(1)
		go l.handleConnection(conn)
	}
}

// handleConnection handles a single LDAPS connection.
func (l *LDAPSListener) handleConnection(conn net.Conn) {
	defer l.wg.Done()
	defer conn.Close()

	// The connection is already TLS (from tls.Listen)
	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		l.logger.Error("LDAPS connection is not TLS",
			"client", conn.RemoteAddr().String())
		return
	}

	// Perform TLS handshake explicitly to catch errors early
	if err := tlsConn.Handshake(); err != nil {
		l.logger.Warn("LDAPS TLS handshake failed",
			"client", conn.RemoteAddr().String(),
			"error", err.Error())
		return
	}

	// Log successful TLS connection
	state := tlsConn.ConnectionState()
	l.logger.Debug("LDAPS TLS handshake completed",
		"client", conn.RemoteAddr().String(),
		"version", TLSVersionString(state.Version),
		"cipher", CipherSuiteName(state.CipherSuite))

	// Create server reference for connection
	var serverRef *Server
	if l.server != nil {
		serverRef = &Server{
			Handler: l.server.Handler,
			Logger:  l.server.Logger,
		}
	}

	// Create connection handler
	ldapConn := NewConnection(tlsConn, serverRef)
	ldapConn.SetTLS(true)

	// Handle the connection
	ldapConn.Handle()
}

// IsRunning returns whether the LDAPS listener is running.
func (l *LDAPSListener) IsRunning() bool {
	return l.running.Load()
}

// Address returns the address the listener is bound to.
func (l *LDAPSListener) Address() string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if l.listener != nil {
		return l.listener.Addr().String()
	}
	return l.config.Address
}

// GetTLSConfig returns the TLS configuration.
func (l *LDAPSListener) GetTLSConfig() *tls.Config {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.config.TLSConfig
}

// NewLDAPServer creates a new LDAP server.
func NewLDAPServer() *LDAPServer {
	return &LDAPServer{
		Handler: NewHandler(),
		Logger:  logging.NewNop(),
		done:    make(chan struct{}),
	}
}

// SetTLSConfig sets the TLS configuration for LDAPS and StartTLS.
func (s *LDAPServer) SetTLSConfig(tlsConfig *tls.Config) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TLSConfig = tlsConfig
}

// SetLogger sets the logger for the server.
func (s *LDAPServer) SetLogger(logger logging.Logger) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Logger = logger
}

// SetHandler sets the operation handler.
func (s *LDAPServer) SetHandler(handler *Handler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Handler = handler
}

// StartLDAPS starts the LDAPS listener on the specified address.
func (s *LDAPServer) StartLDAPS(address string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.TLSConfig == nil {
		return ErrLDAPSNoTLSConfig
	}

	// Use default address if not provided
	if address == "" {
		address = DefaultLDAPSAddress
	}

	// Create LDAPS listener
	config := &LDAPSConfig{
		Address:   address,
		TLSConfig: s.TLSConfig,
	}

	listener, err := NewLDAPSListener(config, s)
	if err != nil {
		return err
	}

	// Start the listener
	if err := listener.Start(); err != nil {
		return err
	}

	s.ldapsListener = listener
	return nil
}

// StopLDAPS stops the LDAPS listener.
func (s *LDAPServer) StopLDAPS() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ldapsListener == nil {
		return ErrLDAPSNotRunning
	}

	err := s.ldapsListener.Stop()
	s.ldapsListener = nil
	return err
}

// IsLDAPSRunning returns whether the LDAPS listener is running.
func (s *LDAPServer) IsLDAPSRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.ldapsListener == nil {
		return false
	}
	return s.ldapsListener.IsRunning()
}

// LDAPSAddress returns the LDAPS listener address.
func (s *LDAPServer) LDAPSAddress() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.ldapsListener == nil {
		return ""
	}
	return s.ldapsListener.Address()
}
