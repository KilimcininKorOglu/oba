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
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/KilimcininKorOglu/oba/internal/acl"
	"github.com/KilimcininKorOglu/oba/internal/backend"
	"github.com/KilimcininKorOglu/oba/internal/config"
	"github.com/KilimcininKorOglu/oba/internal/filter"
	"github.com/KilimcininKorOglu/oba/internal/ldap"
	"github.com/KilimcininKorOglu/oba/internal/logging"
	"github.com/KilimcininKorOglu/oba/internal/password"
	"github.com/KilimcininKorOglu/oba/internal/rest"
	"github.com/KilimcininKorOglu/oba/internal/server"
	"github.com/KilimcininKorOglu/oba/internal/storage"
	"github.com/KilimcininKorOglu/oba/internal/storage/engine"
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
	configFile              string
	configManager           *config.ConfigManager
	logger                  logging.Logger
	handler                 *server.Handler
	backend                 *backend.ObaBackend
	engine                  *engine.ObaDB
	listener                net.Listener
	tlsListener             net.Listener
	tlsConfig               *tls.Config
	tlsCertFile             string
	tlsKeyFile              string
	persistentSearchHandler *server.PersistentSearchHandler
	restServer              *rest.Server
	aclManager              *acl.Manager
	aclWatcher              *acl.FileWatcher
	configWatcher           *config.ConfigWatcher
	pidFile                 string
	running                 bool
	mu                      sync.Mutex
	wg                      sync.WaitGroup
	ctx                     context.Context
	cancel                  context.CancelFunc

	// Hot-reloadable settings
	maxConnections int
	readTimeout    time.Duration
	writeTimeout   time.Duration
	settingsMu     sync.RWMutex
}

// NewServer creates a new LDAP server with the given configuration.
func NewServer(cfg *config.Config) (*LDAPServer, error) {
	// Create logger
	logger := logging.New(logging.Config{
		Level:  cfg.Logging.Level,
		Format: cfg.Logging.Format,
		Output: cfg.Logging.Output,
	})

	// System logger for non-audit operational logs
	sysLogger := logger.WithSource("system")

	// Create log store if enabled
	if cfg.Logging.Store.Enabled && cfg.Logging.Store.DBPath != "" {
		logStore, err := logging.NewLogStore(logging.LogStoreConfig{
			Enabled:    true,
			DBPath:     cfg.Logging.Store.DBPath,
			MaxEntries: cfg.Logging.Store.MaxEntries,
		})
		if err != nil {
			sysLogger.Warn("failed to create log store", "error", err)
		} else {
			logger.SetStore(logStore)
			sysLogger.Info("log store enabled", "path", cfg.Logging.Store.DBPath)
		}
	}

	// Open storage engine
	engineOpts := storage.DefaultEngineOptions().
		WithPageSize(cfg.Storage.PageSize).
		WithCreateIfNotExists(true).
		WithSyncOnWrite(true)

	// Configure encryption if enabled
	if cfg.Security.Encryption.Enabled && cfg.Security.Encryption.KeyFile != "" {
		engineOpts = engineOpts.WithEncryptionKeyFile(cfg.Security.Encryption.KeyFile)
		sysLogger.Info("encryption enabled", "keyFile", cfg.Security.Encryption.KeyFile)
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

	// Create ACL manager and watcher
	var aclManager *acl.Manager
	var aclWatcher *acl.FileWatcher
	if cfg.ACLFile != "" {
		// Load ACL from external file (hot reload supported)
		var err error
		aclManager, err = acl.NewManager(&acl.ManagerConfig{
			FilePath: cfg.ACLFile,
			Logger:   sysLogger,
		})
		if err != nil {
			db.Close()
			cancel()
			return nil, fmt.Errorf("failed to load ACL file: %w", err)
		}
		sysLogger.Info("ACL loaded from file", "file", cfg.ACLFile)

		// Create file watcher for automatic reload
		aclWatcher, err = acl.NewFileWatcher(&acl.WatcherConfig{
			FilePath: cfg.ACLFile,
			Manager:  aclManager,
			Logger:   sysLogger,
		})
		if err != nil {
			db.Close()
			cancel()
			return nil, fmt.Errorf("failed to create ACL watcher: %w", err)
		}
	} else if len(cfg.ACL.Rules) > 0 {
		// Load ACL from embedded config (no hot reload)
		embeddedConfig := convertACLConfig(&cfg.ACL)
		var err error
		aclManager, err = acl.NewManager(&acl.ManagerConfig{
			EmbeddedConfig: embeddedConfig,
			Logger:         sysLogger,
		})
		if err != nil {
			db.Close()
			cancel()
			return nil, fmt.Errorf("failed to create ACL manager: %w", err)
		}
		sysLogger.Info("ACL loaded from config", "rules", len(cfg.ACL.Rules))
	}

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
			AdminDNs:     []string{cfg.Directory.RootDN},
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  120 * time.Second,
		}
		restServer = rest.NewServer(restCfg, be, logger)

		// Set ACL manager for REST API
		if aclManager != nil {
			restServer.SetACLManager(aclManager)
		}

		// Set logger for log endpoints
		restServer.SetLogger(logger)

		sysLogger.Info("REST API enabled", "address", cfg.REST.Address)
	}

	return &LDAPServer{
		config:                  cfg,
		logger:                  logger,
		handler:                 handler,
		backend:                 be,
		engine:                  db,
		tlsConfig:               tlsConfig,
		tlsCertFile:             cfg.Server.TLSCert,
		tlsKeyFile:              cfg.Server.TLSKey,
		persistentSearchHandler: psHandler,
		restServer:              restServer,
		aclManager:              aclManager,
		aclWatcher:              aclWatcher,
		maxConnections:          cfg.Server.MaxConnections,
		readTimeout:             cfg.Server.ReadTimeout,
		writeTimeout:            cfg.Server.WriteTimeout,
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

		// Check if account is locked
		if be.IsAccountLocked(req.Name) {
			return &server.OperationResult{
				ResultCode:        ldap.ResultInvalidCredentials,
				DiagnosticMessage: "account is locked due to too many failed attempts",
			}
		}

		err := be.Bind(req.Name, string(req.SimplePassword))
		if err != nil {
			// Record failed attempt
			be.RecordAuthFailure(req.Name)
			return &server.OperationResult{
				ResultCode:        ldap.ResultInvalidCredentials,
				DiagnosticMessage: "invalid credentials",
			}
		}

		// Record successful authentication
		be.RecordAuthSuccess(req.Name)
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
		s.mu.Lock()
		s.listener = listener
		s.mu.Unlock()
		s.logger.WithSource("system").Info("LDAP server listening", "address", s.config.Server.Address)

		s.wg.Add(1)
		go s.acceptConnections(listener, false)
	}

	// Start TLS listener if configured
	if s.config.Server.TLSAddress != "" && s.tlsConfig != nil {
		listener, err := tls.Listen("tcp", s.config.Server.TLSAddress, s.tlsConfig)
		if err != nil {
			// Close plain listener if TLS fails
			s.mu.Lock()
			if s.listener != nil {
				s.listener.Close()
			}
			s.mu.Unlock()
			return fmt.Errorf("%w: %v", ErrListenerFailed, err)
		}
		s.mu.Lock()
		s.tlsListener = listener
		s.mu.Unlock()
		s.logger.WithSource("system").Info("LDAPS server listening", "address", s.config.Server.TLSAddress)

		s.wg.Add(1)
		go s.acceptConnections(listener, true)
	}

	// Start REST server if enabled
	if s.restServer != nil {
		if err := s.restServer.Start(); err != nil {
			// Close LDAP listeners if REST fails
			s.mu.Lock()
			if s.listener != nil {
				s.listener.Close()
			}
			if s.tlsListener != nil {
				s.tlsListener.Close()
			}
			s.mu.Unlock()
			return fmt.Errorf("failed to start REST server: %w", err)
		}
	}

	// Start ACL file watcher if configured
	if s.aclWatcher != nil {
		s.aclWatcher.Start()
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
	
	// Copy references while holding lock
	listener := s.listener
	tlsListener := s.tlsListener
	restServer := s.restServer
	aclWatcher := s.aclWatcher
	s.mu.Unlock()

	// Cancel the server context
	s.cancel()

	// Stop ACL file watcher
	if aclWatcher != nil {
		aclWatcher.Stop()
	}

	// Stop REST server
	if restServer != nil {
		restServer.Stop(ctx)
	}

	// Close listeners
	if listener != nil {
		listener.Close()
	}
	if tlsListener != nil {
		tlsListener.Close()
	}

	// Wait for connections to finish with timeout
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Close log store first to flush pending writes
		if s.logger != nil {
			s.logger.CloseStore()
		}
		// Close storage engine
		if s.engine != nil {
			s.engine.Close()
		}
		s.logger.WithSource("system").Info("server stopped gracefully")
		return nil
	case <-ctx.Done():
		// Close log store first to flush pending writes
		if s.logger != nil {
			s.logger.CloseStore()
		}
		// Close storage engine even on timeout
		if s.engine != nil {
			s.engine.Close()
		}
		s.logger.WithSource("system").Warn("server shutdown timed out")
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

	// Store config file path for watcher
	srv.configFile = *configFile

	// Create config manager and set it on REST server
	if *configFile != "" {
		srv.configManager = config.NewConfigManager(cfg, *configFile)
		srv.configManager.SetOnUpdate(srv.handleConfigReload)
		if srv.restServer != nil {
			srv.restServer.SetConfigManager(srv.configManager)
		}
	}

	// Write PID file
	if cfg.Server.PIDFile != "" {
		if err := srv.writePIDFile(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write PID file: %v\n", err)
			return 1
		}
		defer srv.removePIDFile()
	}

	// Start config file watcher if config file is specified
	if *configFile != "" {
		configWatcher, err := config.NewConfigWatcher(&config.WatcherConfig{
			FilePath: *configFile,
			OnChange: srv.handleConfigReload,
		})
		if err != nil {
			srv.logger.WithSource("system").Warn("failed to create config watcher", "error", err)
		} else {
			srv.configWatcher = configWatcher
			configWatcher.Start()
			srv.logger.WithSource("system").Info("config file watcher started", "file", *configFile)
			defer configWatcher.Stop()
		}
	}

	// Handle signals for graceful shutdown and reload
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	// Start server in a goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start()
	}()

	// Wait for signal or error
	for {
		select {
		case sig := <-sigCh:
			switch sig {
			case syscall.SIGHUP:
				srv.handleSIGHUP()
			case syscall.SIGINT, syscall.SIGTERM:
				srv.logger.Info("received signal, shutting down", "signal", sig.String())

				// Create shutdown context with timeout
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()

				if err := srv.Stop(shutdownCtx); err != nil {
					fmt.Fprintf(os.Stderr, "Shutdown error: %v\n", err)
					return 1
				}
				return 0
			}

		case err := <-errCh:
			if err != nil {
				fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
				return 1
			}
			return 0
		}
	}
}

// handleSIGHUP handles the SIGHUP signal for ACL reload.
func (s *LDAPServer) handleSIGHUP() {
	sysLogger := s.logger.WithSource("system")
	sysLogger.Info("received SIGHUP, reloading ACL configuration")

	if s.aclManager == nil {
		sysLogger.Warn("ACL manager not configured, nothing to reload")
		return
	}

	if !s.aclManager.IsFileMode() {
		sysLogger.Warn("ACL loaded from embedded config, hot reload not supported")
		return
	}

	if err := s.aclManager.Reload(); err != nil {
		sysLogger.Error("ACL reload failed", "error", err)
		return
	}

	stats := s.aclManager.Stats()
	sysLogger.Info("ACL configuration reloaded successfully",
		"rules", stats.RuleCount,
		"defaultPolicy", stats.DefaultPolicy,
		"reloadCount", stats.ReloadCount,
	)
}

// writePIDFile writes the process ID to the configured PID file.
func (s *LDAPServer) writePIDFile() error {
	pidFile := s.config.Server.PIDFile
	if pidFile == "" {
		return nil
	}

	pid := os.Getpid()
	data := []byte(strconv.Itoa(pid))

	if err := os.WriteFile(pidFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	s.pidFile = pidFile
	s.logger.WithSource("system").Info("PID file written", "file", pidFile, "pid", pid)
	return nil
}

// removePIDFile removes the PID file.
func (s *LDAPServer) removePIDFile() {
	if s.pidFile != "" {
		os.Remove(s.pidFile)
		s.logger.WithSource("system").Debug("PID file removed", "file", s.pidFile)
	}
}

// convertACLConfig converts config.ACLConfig to acl.Config.
func convertACLConfig(cfg *config.ACLConfig) *acl.Config {
	aclConfig := acl.NewConfig()
	aclConfig.SetDefaultPolicy(cfg.DefaultPolicy)

	for _, rule := range cfg.Rules {
		rights, _ := acl.ParseRights(rule.Rights)
		aclRule := acl.NewACL(rule.Target, rule.Subject, rights)
		if len(rule.Attributes) > 0 {
			aclRule.WithAttributes(rule.Attributes...)
		}
		aclConfig.AddRule(aclRule)
	}

	return aclConfig
}

// SetMaxConnections updates the maximum connections limit at runtime.
func (s *LDAPServer) SetMaxConnections(max int) {
	s.settingsMu.Lock()
	defer s.settingsMu.Unlock()
	s.maxConnections = max
}

// GetMaxConnections returns the current maximum connections limit.
func (s *LDAPServer) GetMaxConnections() int {
	s.settingsMu.RLock()
	defer s.settingsMu.RUnlock()
	return s.maxConnections
}

// SetReadTimeout updates the read timeout for new connections.
func (s *LDAPServer) SetReadTimeout(timeout time.Duration) {
	s.settingsMu.Lock()
	defer s.settingsMu.Unlock()
	s.readTimeout = timeout
}

// GetReadTimeout returns the current read timeout.
func (s *LDAPServer) GetReadTimeout() time.Duration {
	s.settingsMu.RLock()
	defer s.settingsMu.RUnlock()
	return s.readTimeout
}

// SetWriteTimeout updates the write timeout for new connections.
func (s *LDAPServer) SetWriteTimeout(timeout time.Duration) {
	s.settingsMu.Lock()
	defer s.settingsMu.Unlock()
	s.writeTimeout = timeout
}

// GetWriteTimeout returns the current write timeout.
func (s *LDAPServer) GetWriteTimeout() time.Duration {
	s.settingsMu.RLock()
	defer s.settingsMu.RUnlock()
	return s.writeTimeout
}

// ReloadTLSCert reloads TLS certificate and key from files.
func (s *LDAPServer) ReloadTLSCert(certFile, keyFile string) error {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return fmt.Errorf("failed to load TLS certificate: %w", err)
	}

	s.settingsMu.Lock()
	defer s.settingsMu.Unlock()

	// Update TLS config with new certificate
	if s.tlsConfig != nil {
		s.tlsConfig.Certificates = []tls.Certificate{cert}
	}
	s.tlsCertFile = certFile
	s.tlsKeyFile = keyFile

	return nil
}

// handleConfigReload handles config file changes and applies hot-reloadable settings.
func (s *LDAPServer) handleConfigReload(oldCfg, newCfg *config.Config) {
	s.logger.Info("config file changed, applying hot-reloadable settings")

	// Logging settings
	if oldCfg.Logging.Level != newCfg.Logging.Level {
		s.logger.SetLevel(logging.ParseLevel(newCfg.Logging.Level))
		s.logger.Info("log level changed", "old", oldCfg.Logging.Level, "new", newCfg.Logging.Level)
	}
	if oldCfg.Logging.Format != newCfg.Logging.Format {
		s.logger.SetFormat(logging.ParseFormat(newCfg.Logging.Format))
		s.logger.Info("log format changed", "old", oldCfg.Logging.Format, "new", newCfg.Logging.Format)
	}

	// Server settings
	if oldCfg.Server.MaxConnections != newCfg.Server.MaxConnections {
		s.SetMaxConnections(newCfg.Server.MaxConnections)
		s.logger.Info("max connections changed", "old", oldCfg.Server.MaxConnections, "new", newCfg.Server.MaxConnections)
	}
	if oldCfg.Server.ReadTimeout != newCfg.Server.ReadTimeout {
		s.SetReadTimeout(newCfg.Server.ReadTimeout)
		s.logger.Info("read timeout changed", "old", oldCfg.Server.ReadTimeout, "new", newCfg.Server.ReadTimeout)
	}
	if oldCfg.Server.WriteTimeout != newCfg.Server.WriteTimeout {
		s.SetWriteTimeout(newCfg.Server.WriteTimeout)
		s.logger.Info("write timeout changed", "old", oldCfg.Server.WriteTimeout, "new", newCfg.Server.WriteTimeout)
	}

	// TLS certificate reload
	if oldCfg.Server.TLSCert != newCfg.Server.TLSCert || oldCfg.Server.TLSKey != newCfg.Server.TLSKey {
		if newCfg.Server.TLSCert != "" && newCfg.Server.TLSKey != "" {
			if err := s.ReloadTLSCert(newCfg.Server.TLSCert, newCfg.Server.TLSKey); err != nil {
				s.logger.Error("failed to reload TLS certificate", "error", err)
			} else {
				s.logger.Info("TLS certificate reloaded")
			}
		}
	}

	// Security rate limit settings
	if oldCfg.Security.RateLimit.Enabled != newCfg.Security.RateLimit.Enabled ||
		oldCfg.Security.RateLimit.MaxAttempts != newCfg.Security.RateLimit.MaxAttempts ||
		oldCfg.Security.RateLimit.LockoutDuration != newCfg.Security.RateLimit.LockoutDuration {
		s.backend.SetRateLimitConfig(
			newCfg.Security.RateLimit.Enabled,
			newCfg.Security.RateLimit.MaxAttempts,
			newCfg.Security.RateLimit.LockoutDuration,
		)
		s.logger.Info("rate limit config changed",
			"enabled", newCfg.Security.RateLimit.Enabled,
			"maxAttempts", newCfg.Security.RateLimit.MaxAttempts,
			"lockoutDuration", newCfg.Security.RateLimit.LockoutDuration,
		)
	}

	// Password policy settings
	if s.passwordPolicyChanged(oldCfg, newCfg) {
		policy := s.convertPasswordPolicy(&newCfg.Security.PasswordPolicy)
		s.backend.SetPasswordPolicy(policy)
		s.logger.Info("password policy changed", "enabled", newCfg.Security.PasswordPolicy.Enabled)
	}

	// REST API settings
	if s.restServer != nil {
		if oldCfg.REST.RateLimit != newCfg.REST.RateLimit {
			s.restServer.SetRateLimit(newCfg.REST.RateLimit)
			s.logger.Info("REST rate limit changed", "old", oldCfg.REST.RateLimit, "new", newCfg.REST.RateLimit)
		}
		if oldCfg.REST.TokenTTL != newCfg.REST.TokenTTL {
			s.restServer.SetTokenTTL(newCfg.REST.TokenTTL)
			s.logger.Info("REST token TTL changed", "old", oldCfg.REST.TokenTTL, "new", newCfg.REST.TokenTTL)
		}
		if !stringSliceEqual(oldCfg.REST.CORSOrigins, newCfg.REST.CORSOrigins) {
			s.restServer.SetCORSOrigins(newCfg.REST.CORSOrigins)
			s.logger.Info("REST CORS origins changed")
		}
	}

	// Update stored config
	s.config = newCfg
	s.logger.Info("config reload completed")
}

// passwordPolicyChanged checks if password policy settings changed.
func (s *LDAPServer) passwordPolicyChanged(oldCfg, newCfg *config.Config) bool {
	old := &oldCfg.Security.PasswordPolicy
	new := &newCfg.Security.PasswordPolicy
	return old.Enabled != new.Enabled ||
		old.MinLength != new.MinLength ||
		old.RequireUppercase != new.RequireUppercase ||
		old.RequireLowercase != new.RequireLowercase ||
		old.RequireDigit != new.RequireDigit ||
		old.RequireSpecial != new.RequireSpecial ||
		old.MaxAge != new.MaxAge ||
		old.HistoryCount != new.HistoryCount
}

// convertPasswordPolicy converts config password policy to password.Policy.
func (s *LDAPServer) convertPasswordPolicy(cfg *config.PasswordPolicyConfig) *password.Policy {
	return &password.Policy{
		Enabled:          cfg.Enabled,
		MinLength:        cfg.MinLength,
		RequireUppercase: cfg.RequireUppercase,
		RequireLowercase: cfg.RequireLowercase,
		RequireDigit:     cfg.RequireDigit,
		RequireSpecial:   cfg.RequireSpecial,
		MaxAge:           cfg.MaxAge,
		HistoryCount:     cfg.HistoryCount,
	}
}

// stringSliceEqual compares two string slices for equality.
func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
