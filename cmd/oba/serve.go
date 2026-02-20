// Package main provides the serve command for the oba LDAP server.
package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/oba-ldap/oba/internal/config"
	"github.com/oba-ldap/oba/internal/logging"
	"github.com/oba-ldap/oba/internal/server"
)

// Server errors.
var (
	ErrServerAlreadyRunning = errors.New("server is already running")
	ErrServerNotRunning     = errors.New("server is not running")
	ErrListenerFailed       = errors.New("failed to create listener")
)

// LDAPServer represents the LDAP server instance.
type LDAPServer struct {
	config    *config.Config
	logger    logging.Logger
	handler   *server.Handler
	listener  net.Listener
	tlsListener net.Listener
	tlsConfig *tls.Config
	running   bool
	mu        sync.Mutex
	wg        sync.WaitGroup
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewServer creates a new LDAP server with the given configuration.
func NewServer(cfg *config.Config) (*LDAPServer, error) {
	// Create logger
	logger := logging.New(logging.Config{
		Level:  cfg.Logging.Level,
		Format: cfg.Logging.Format,
		Output: cfg.Logging.Output,
	})

	// Create handler
	handler := server.NewHandler()

	// Create TLS config if certificates are provided
	var tlsConfig *tls.Config
	if cfg.Server.TLSCert != "" && cfg.Server.TLSKey != "" {
		tlsCfg := server.NewTLSConfig().WithCertFile(cfg.Server.TLSCert, cfg.Server.TLSKey)
		var err error
		tlsConfig, err = server.LoadTLSConfig(tlsCfg)
		if err != nil {
			return nil, fmt.Errorf("failed to load TLS config: %w", err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &LDAPServer{
		config:    cfg,
		logger:    logger,
		handler:   handler,
		tlsConfig: tlsConfig,
		ctx:       ctx,
		cancel:    cancel,
	}, nil
}

// Start starts the LDAP server.
func (s *LDAPServer) Start() error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return ErrServerAlreadyRunning
	}
	s.running = true
	s.mu.Unlock()

	// Start plain LDAP listener
	if s.config.Server.Address != "" {
		listener, err := net.Listen("tcp", s.config.Server.Address)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrListenerFailed, err)
		}
		s.listener = listener
		s.logger.Info("LDAP server listening", "address", s.config.Server.Address)

		s.wg.Add(1)
		go s.acceptConnections(listener, false)
	}

	// Start TLS listener if configured
	if s.config.Server.TLSAddress != "" && s.tlsConfig != nil {
		listener, err := tls.Listen("tcp", s.config.Server.TLSAddress, s.tlsConfig)
		if err != nil {
			// Close plain listener if TLS fails
			if s.listener != nil {
				s.listener.Close()
			}
			return fmt.Errorf("%w: %v", ErrListenerFailed, err)
		}
		s.tlsListener = listener
		s.logger.Info("LDAPS server listening", "address", s.config.Server.TLSAddress)

		s.wg.Add(1)
		go s.acceptConnections(listener, true)
	}

	// Wait for all connections to finish
	s.wg.Wait()
	return nil
}

// Stop gracefully stops the LDAP server.
func (s *LDAPServer) Stop(ctx context.Context) error {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return ErrServerNotRunning
	}
	s.running = false
	s.mu.Unlock()

	// Cancel the server context
	s.cancel()

	// Close listeners
	if s.listener != nil {
		s.listener.Close()
	}
	if s.tlsListener != nil {
		s.tlsListener.Close()
	}

	// Wait for connections to finish with timeout
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.logger.Info("server stopped gracefully")
		return nil
	case <-ctx.Done():
		s.logger.Warn("server shutdown timed out")
		return ctx.Err()
	}
}

// acceptConnections accepts incoming connections on the listener.
func (s *LDAPServer) acceptConnections(listener net.Listener, isTLS bool) {
	defer s.wg.Done()

	for {
		conn, err := listener.Accept()
		if err != nil {
			// Check if server is shutting down
			select {
			case <-s.ctx.Done():
				return
			default:
				// Check for closed listener error
				if isClosedError(err) {
					return
				}
				s.logger.Warn("accept error", "error", err.Error())
				continue
			}
		}

		// Handle connection in a goroutine
		s.wg.Add(1)
		go s.handleConnection(conn, isTLS)
	}
}

// handleConnection handles a single client connection.
func (s *LDAPServer) handleConnection(conn net.Conn, isTLS bool) {
	defer s.wg.Done()

	// Create server struct for connection
	srv := &server.Server{
		Handler: s.handler,
		Logger:  s.logger,
	}

	// Create and handle connection
	c := server.NewConnection(conn, srv)
	c.SetTLS(isTLS)
	c.Handle()
}

// isClosedError checks if the error is due to a closed listener.
func isClosedError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, net.ErrClosed) ||
		containsString(err.Error(), "use of closed network connection")
}

// containsString checks if s contains substr.
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && findSubstr(s, substr)
}

// findSubstr performs a simple substring search.
func findSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// serveCmd handles the serve command.
func serveCmd(args []string) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	configFile := fs.String("config", "", "Path to configuration file")
	address := fs.String("address", "", "Listen address (overrides config)")
	tlsAddress := fs.String("tls-address", "", "TLS listen address (overrides config)")
	dataDir := fs.String("data-dir", "", "Data directory path (overrides config)")
	logLevel := fs.String("log-level", "", "Log level: debug, info, warn, error (overrides config)")
	help := fs.Bool("h", false, "Show help message")
	helpLong := fs.Bool("help", false, "Show help message")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if *help || *helpLong {
		printServeUsage(os.Stdout)
		return 0
	}

	// Load configuration
	var cfg *config.Config
	var err error

	if *configFile != "" {
		cfg, err = config.LoadConfig(*configFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
			return 1
		}
	} else {
		cfg = config.DefaultConfig()
	}

	// Apply command-line overrides (higher priority than config file)
	if *address != "" {
		cfg.Server.Address = *address
	}
	if *tlsAddress != "" {
		cfg.Server.TLSAddress = *tlsAddress
	}
	if *dataDir != "" {
		cfg.Storage.DataDir = *dataDir
	}
	if *logLevel != "" {
		cfg.Logging.Level = *logLevel
	}

	// Apply environment variable overrides (highest priority)
	applyEnvOverrides(cfg)

	// Validate configuration
	errs := config.ValidateConfig(cfg)
	if len(errs) > 0 {
		fmt.Fprintln(os.Stderr, "Configuration errors:")
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "  - %s\n", e)
		}
		return 1
	}

	// Create server
	srv, err := NewServer(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create server: %v\n", err)
		return 1
	}

	// Handle signals for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start server in a goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start()
	}()

	// Wait for signal or error
	select {
	case sig := <-sigCh:
		srv.logger.Info("received signal, shutting down", "signal", sig.String())

		// Create shutdown context with timeout
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := srv.Stop(shutdownCtx); err != nil {
			fmt.Fprintf(os.Stderr, "Shutdown error: %v\n", err)
			return 1
		}
		return 0

	case err := <-errCh:
		if err != nil {
			fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
			return 1
		}
		return 0
	}
}
