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

	"github.com/oba-ldap/oba/internal/backend"
	"github.com/oba-ldap/oba/internal/config"
	"github.com/oba-ldap/oba/internal/filter"
	"github.com/oba-ldap/oba/internal/ldap"
	"github.com/oba-ldap/oba/internal/logging"
	"github.com/oba-ldap/oba/internal/rest"
	"github.com/oba-ldap/oba/internal/server"
	"github.com/oba-ldap/oba/internal/storage"
	"github.com/oba-ldap/oba/internal/storage/engine"
)

// Server errors.
var (
	ErrServerAlreadyRunning = errors.New("server is already running")
	ErrServerNotRunning     = errors.New("server is not running")
	ErrListenerFailed       = errors.New("failed to create listener")
)

// LDAPServer represents the LDAP server instance.
type LDAPServer struct {
	config                  *config.Config
	logger                  logging.Logger
	handler                 *server.Handler
	backend                 *backend.ObaBackend
	engine                  *engine.ObaDB
	listener                net.Listener
	tlsListener             net.Listener
	tlsConfig               *tls.Config
	persistentSearchHandler *server.PersistentSearchHandler
	restServer              *rest.Server
	running                 bool
	mu                      sync.Mutex
	wg                      sync.WaitGroup
	ctx                     context.Context
	cancel                  context.CancelFunc
}

// NewServer creates a new LDAP server with the given configuration.
func NewServer(cfg *config.Config) (*LDAPServer, error) {
	// Create logger
	logger := logging.New(logging.Config{
		Level:  cfg.Logging.Level,
		Format: cfg.Logging.Format,
		Output: cfg.Logging.Output,
	})

	// Open storage engine
	engineOpts := storage.DefaultEngineOptions().
		WithPageSize(cfg.Storage.PageSize).
		WithCreateIfNotExists(true).
		WithSyncOnWrite(true)

	// Configure encryption if enabled
	if cfg.Security.Encryption.Enabled && cfg.Security.Encryption.KeyFile != "" {
		engineOpts = engineOpts.WithEncryptionKeyFile(cfg.Security.Encryption.KeyFile)
		logger.Info("encryption enabled", "keyFile", cfg.Security.Encryption.KeyFile)
	}

	db, err := engine.Open(cfg.Storage.DataDir, engineOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to open storage engine: %w", err)
	}

	// Create backend
	be := backend.NewBackend(db, cfg)

	// Create handler with backend integration
	handler := server.NewHandler()
	setupHandlers(handler, be, logger)

	// Create TLS config if certificates are provided
	var tlsConfig *tls.Config
	if cfg.Server.TLSCert != "" && cfg.Server.TLSKey != "" {
		tlsCfg := server.NewTLSConfig().WithCertFile(cfg.Server.TLSCert, cfg.Server.TLSKey)
		var err error
		tlsConfig, err = server.LoadTLSConfig(tlsCfg)
		if err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to load TLS config: %w", err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Create persistent search handler
	psHandler := server.NewPersistentSearchHandler(be)

	// Create REST server if enabled
	var restServer *rest.Server
	if cfg.REST.Enabled {
		restCfg := &rest.ServerConfig{
			Address:      cfg.REST.Address,
			TLSAddress:   cfg.REST.TLSAddress,
			TLSCert:      cfg.Server.TLSCert,
			TLSKey:       cfg.Server.TLSKey,
			JWTSecret:    cfg.REST.JWTSecret,
			TokenTTL:     cfg.REST.TokenTTL,
			RateLimit:    cfg.REST.RateLimit,
			CORSOrigins:  cfg.REST.CORSOrigins,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  120 * time.Second,
		}
		restServer = rest.NewServer(restCfg, be, logger)
		logger.Info("REST API enabled", "address", cfg.REST.Address)
	}

	return &LDAPServer{
		config:                  cfg,
		logger:                  logger,
		handler:                 handler,
		backend:                 be,
		engine:                  db,
		tlsConfig:               tlsConfig,
		persistentSearchHandler: psHandler,
		restServer:              restServer,
		ctx:                     ctx,
		cancel:                  cancel,
	}, nil
}

// setupHandlers configures the LDAP operation handlers with backend integration.
func setupHandlers(h *server.Handler, be backend.Backend, logger logging.Logger) {
	// Bind handler
	h.SetBindHandler(func(conn *server.Connection, req *ldap.BindRequest) *server.OperationResult {
		if req.IsAnonymous() {
			return &server.OperationResult{ResultCode: ldap.ResultSuccess}
		}

		err := be.Bind(req.Name, string(req.SimplePassword))
		if err != nil {
			logger.Debug("bind failed", "dn", req.Name, "error", err.Error())
			return &server.OperationResult{
				ResultCode:        ldap.ResultInvalidCredentials,
				DiagnosticMessage: "invalid credentials",
			}
		}

		logger.Info("bind successful", "dn", req.Name)
		return &server.OperationResult{ResultCode: ldap.ResultSuccess}
	})

	// Search handler
	h.SetSearchHandler(func(conn *server.Connection, req *ldap.SearchRequest) *server.SearchResult {
		// Convert LDAP filter to backend filter
		var f *filter.Filter
		if req.Filter != nil {
			f = convertSearchFilter(req.Filter)
		}

		entries, err := be.Search(req.BaseObject, int(req.Scope), f)
		if err != nil {
			logger.Debug("search failed", "baseDN", req.BaseObject, "error", err.Error())
			return &server.SearchResult{
				OperationResult: server.OperationResult{
					ResultCode:        ldap.ResultOperationsError,
					DiagnosticMessage: err.Error(),
				},
			}
		}

		// Convert backend entries to server entries
		serverEntries := make([]*server.SearchEntry, len(entries))
		for i, entry := range entries {
			serverEntries[i] = &server.SearchEntry{
				DN:         entry.DN,
				Attributes: convertAttributes(entry),
			}
		}

		return &server.SearchResult{
			OperationResult: server.OperationResult{ResultCode: ldap.ResultSuccess},
			Entries:         serverEntries,
		}
	})

	// Add handler
	h.SetAddHandler(func(conn *server.Connection, req *ldap.AddRequest) *server.OperationResult {
		entry := backend.NewEntry(req.Entry)
		for _, attr := range req.Attributes {
			values := make([]string, len(attr.Values))
			for i, v := range attr.Values {
				values[i] = string(v)
			}
			entry.SetAttribute(attr.Type, values...)
		}

		err := be.AddWithBindDN(entry, conn.BindDN())
		if err != nil {
			logger.Debug("add failed", "dn", req.Entry, "error", err.Error())
			if err == backend.ErrEntryExists {
				return &server.OperationResult{
					ResultCode:        ldap.ResultEntryAlreadyExists,
					DiagnosticMessage: "entry already exists",
				}
			}
			return &server.OperationResult{
				ResultCode:        ldap.ResultOperationsError,
				DiagnosticMessage: err.Error(),
			}
		}

		logger.Info("entry added", "dn", req.Entry)
		return &server.OperationResult{ResultCode: ldap.ResultSuccess}
	})

	// Delete handler
	h.SetDeleteHandler(func(conn *server.Connection, req *ldap.DeleteRequest) *server.OperationResult {
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

		err = be.Delete(req.DN)
		if err != nil {
			logger.Debug("delete failed", "dn", req.DN, "error", err.Error())
			if err == backend.ErrEntryNotFound {
				return &server.OperationResult{
					ResultCode:        ldap.ResultNoSuchObject,
					DiagnosticMessage: "entry not found",
				}
			}
			return &server.OperationResult{
				ResultCode:        ldap.ResultOperationsError,
				DiagnosticMessage: err.Error(),
			}
		}

		logger.Info("entry deleted", "dn", req.DN)
		return &server.OperationResult{ResultCode: ldap.ResultSuccess}
	})

	// Modify handler
	h.SetModifyHandler(func(conn *server.Connection, req *ldap.ModifyRequest) *server.OperationResult {
		changes := make([]backend.Modification, len(req.Changes))
		for i, change := range req.Changes {
			values := make([]string, len(change.Attribute.Values))
			for j, v := range change.Attribute.Values {
				values[j] = string(v)
			}
			changes[i] = backend.Modification{
				Type:      backend.ModificationType(change.Operation),
				Attribute: change.Attribute.Type,
				Values:    values,
			}
		}

		err := be.ModifyWithBindDN(req.Object, changes, conn.BindDN())
		if err != nil {
			logger.Debug("modify failed", "dn", req.Object, "error", err.Error())
			if err == backend.ErrEntryNotFound {
				return &server.OperationResult{
					ResultCode:        ldap.ResultNoSuchObject,
					DiagnosticMessage: "entry not found",
				}
			}
			return &server.OperationResult{
				ResultCode:        ldap.ResultOperationsError,
				DiagnosticMessage: err.Error(),
			}
		}

		logger.Info("entry modified", "dn", req.Object)
		return &server.OperationResult{ResultCode: ldap.ResultSuccess}
	})
}

// convertSearchFilter converts an LDAP search filter to a backend filter.
func convertSearchFilter(sf *ldap.SearchFilter) *filter.Filter {
	if sf == nil {
		return nil
	}

	f := &filter.Filter{
		Type:      filter.FilterType(sf.Type),
		Attribute: sf.Attribute,
		Value:     sf.Value,
	}

	if sf.Substrings != nil {
		f.Substring = &filter.SubstringFilter{
			Attribute: sf.Attribute,
			Initial:   sf.Substrings.Initial,
			Any:       sf.Substrings.Any,
			Final:     sf.Substrings.Final,
		}
	}

	for _, child := range sf.Children {
		f.Children = append(f.Children, convertSearchFilter(child))
	}

	if sf.Child != nil {
		f.Child = convertSearchFilter(sf.Child)
	}

	return f
}

// convertAttributes converts backend entry attributes to LDAP attributes.
func convertAttributes(entry *backend.Entry) []ldap.Attribute {
	attrs := make([]ldap.Attribute, 0, len(entry.Attributes))
	for name, values := range entry.Attributes {
		byteValues := make([][]byte, len(values))
		for i, v := range values {
			byteValues[i] = []byte(v)
		}
		attrs = append(attrs, ldap.Attribute{
			Type:   name,
			Values: byteValues,
		})
	}
	return attrs
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

	// Start REST server if enabled
	if s.restServer != nil {
		if err := s.restServer.Start(); err != nil {
			// Close LDAP listeners if REST fails
			if s.listener != nil {
				s.listener.Close()
			}
			if s.tlsListener != nil {
				s.tlsListener.Close()
			}
			return fmt.Errorf("failed to start REST server: %w", err)
		}
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

	// Stop REST server
	if s.restServer != nil {
		s.restServer.Stop(ctx)
	}

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
		// Close storage engine
		if s.engine != nil {
			s.engine.Close()
		}
		s.logger.Info("server stopped gracefully")
		return nil
	case <-ctx.Done():
		// Close storage engine even on timeout
		if s.engine != nil {
			s.engine.Close()
		}
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
	c.SetPersistentSearchHandler(s.persistentSearchHandler)
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

	// Resolve relative paths to absolute paths
	if err := cfg.ResolvePaths(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to resolve paths: %v\n", err)
		return 1
	}

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
