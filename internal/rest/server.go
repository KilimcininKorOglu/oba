package rest

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/KilimcininKorOglu/oba/internal/acl"
	"github.com/KilimcininKorOglu/oba/internal/backend"
	"github.com/KilimcininKorOglu/oba/internal/config"
	"github.com/KilimcininKorOglu/oba/internal/logging"
)

// ServerConfig holds REST server configuration.
type ServerConfig struct {
	Address      string
	TLSAddress   string
	TLSCert      string
	TLSKey       string
	JWTSecret    string
	TokenTTL     time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
	RateLimit    int
	CORSOrigins  []string
	AdminDNs     []string
}

// DefaultServerConfig returns default configuration.
func DefaultServerConfig() *ServerConfig {
	return &ServerConfig{
		Address:      ":8080",
		TokenTTL:     24 * time.Hour,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
		RateLimit:    100,
		CORSOrigins:  []string{"*"},
	}
}

// Server is the REST API server.
type Server struct {
	config    *ServerConfig
	backend   *backend.ObaBackend
	logger    logging.Logger
	auth      *Authenticator
	handlers  *Handlers
	router    *Router
	server    *http.Server
	tlsServer *http.Server

	// Hot-reloadable settings
	rateLimit   int32 // atomic
	tokenTTL    int64 // atomic (nanoseconds)
	corsOrigins []string
	corsMu      sync.RWMutex
}

// NewServer creates a new REST server.
func NewServer(cfg *ServerConfig, be *backend.ObaBackend, logger logging.Logger) *Server {
	auth := NewAuthenticator(be, cfg.JWTSecret, cfg.TokenTTL)
	handlers := NewHandlers(be, auth)

	router := NewRouter()

	s := &Server{
		config:      cfg,
		backend:     be,
		logger:      logger,
		auth:        auth,
		handlers:    handlers,
		router:      router,
		rateLimit:   int32(cfg.RateLimit),
		tokenTTL:    int64(cfg.TokenTTL),
		corsOrigins: cfg.CORSOrigins,
	}

	s.setupRoutes()
	s.setupMiddleware()

	return s
}

func (s *Server) setupRoutes() {
	s.router.GET("/api/v1/health", s.handlers.HandleHealth)
	s.router.GET("/api/v1/config/public", s.handlers.HandleGetPublicConfig)

	s.router.POST("/api/v1/auth/bind", s.handlers.HandleBind)

	s.router.GET("/api/v1/entries/{dn}", s.handlers.HandleGetEntry)
	s.router.POST("/api/v1/entries", s.handlers.HandleAddEntry)
	s.router.PUT("/api/v1/entries/{dn}", s.handlers.HandleModifyEntry)
	s.router.PATCH("/api/v1/entries/{dn}", s.handlers.HandleModifyEntry)
	s.router.DELETE("/api/v1/entries/{dn}", s.handlers.HandleDeleteEntry)
	s.router.POST("/api/v1/entries/{dn}/move", s.handlers.HandleModifyDN)
	s.router.POST("/api/v1/entries/{dn}/disable", s.handlers.HandleDisableEntry)
	s.router.POST("/api/v1/entries/{dn}/enable", s.handlers.HandleEnableEntry)
	s.router.POST("/api/v1/entries/{dn}/unlock", s.handlers.HandleUnlockEntry)
	s.router.GET("/api/v1/entries/{dn}/lock-status", s.handlers.HandleGetLockStatus)

	s.router.GET("/api/v1/search", s.handlers.HandleSearch)
	s.router.GET("/api/v1/search/stream", s.handlers.HandleStreamSearch)

	s.router.POST("/api/v1/bulk", s.handlers.HandleBulk)

	s.router.POST("/api/v1/compare", s.handlers.HandleCompare)

	// ACL management endpoints
	s.router.GET("/api/v1/acl", s.handlers.HandleGetACL)
	s.router.GET("/api/v1/acl/rules", s.handlers.HandleGetACLRules)
	s.router.GET("/api/v1/acl/rules/{index}", s.handlers.HandleGetACLRule)
	s.router.POST("/api/v1/acl/rules", s.handlers.HandleAddACLRule)
	s.router.PUT("/api/v1/acl/rules/{index}", s.handlers.HandleUpdateACLRule)
	s.router.DELETE("/api/v1/acl/rules/{index}", s.handlers.HandleDeleteACLRule)
	s.router.PUT("/api/v1/acl/default", s.handlers.HandleSetDefaultPolicy)
	s.router.POST("/api/v1/acl/reload", s.handlers.HandleReloadACL)
	s.router.POST("/api/v1/acl/save", s.handlers.HandleSaveACL)
	s.router.POST("/api/v1/acl/validate", s.handlers.HandleValidateACL)

	// Config management endpoints
	s.router.GET("/api/v1/config", s.handlers.HandleGetConfig)
	s.router.GET("/api/v1/config/{section}", s.handlers.HandleGetConfigSection)
	s.router.PATCH("/api/v1/config/{section}", s.handlers.HandleUpdateConfigSection)
	s.router.POST("/api/v1/config/reload", s.handlers.HandleReloadConfig)
	s.router.POST("/api/v1/config/save", s.handlers.HandleSaveConfig)
	s.router.POST("/api/v1/config/validate", s.handlers.HandleValidateConfig)

	// Log management endpoints
	s.router.GET("/api/v1/logs", s.handlers.HandleGetLogs)
	s.router.GET("/api/v1/logs/stats", s.handlers.HandleGetLogStats)
	s.router.DELETE("/api/v1/logs", s.handlers.HandleClearLogs)
	s.router.GET("/api/v1/logs/export", s.handlers.HandleExportLogs)
}

func (s *Server) setupMiddleware() {
	// Logging middleware first (outermost) to capture user info after auth
	s.router.Use(LoggingMiddleware(s.logger))

	s.router.Use(RecoveryMiddleware(s.logger))
	s.router.Use(ConnectionTrackingMiddleware(s.handlers))

	if len(s.config.CORSOrigins) > 0 {
		s.router.Use(CORSMiddleware(s.config.CORSOrigins))
	}

	if s.config.RateLimit > 0 {
		s.router.Use(RateLimitMiddleware(s.config.RateLimit))
	}

	s.router.Use(AuthMiddleware(s.auth, []string{
		"/api/v1/health",
		"/api/v1/auth/bind",
		"/api/v1/config/public",
	}))

	// Admin-only endpoints
	if len(s.config.AdminDNs) > 0 {
		s.router.Use(AdminOnlyMiddleware(s.config.AdminDNs, []string{
			"/api/v1/acl",
			"/api/v1/config",
		}, []string{
			"/api/v1/config/public",
		}))
	}
}

// Start starts the REST server.
func (s *Server) Start() error {
	s.server = &http.Server{
		Addr:         s.config.Address,
		Handler:      s.router,
		ReadTimeout:  s.config.ReadTimeout,
		WriteTimeout: s.config.WriteTimeout,
		IdleTimeout:  s.config.IdleTimeout,
	}

	listener, err := net.Listen("tcp", s.config.Address)
	if err != nil {
		return err
	}

	s.logger.WithSource("system").Info("REST server started", "address", s.config.Address)

	go s.server.Serve(listener)

	if s.config.TLSAddress != "" && s.config.TLSCert != "" && s.config.TLSKey != "" {
		cert, err := tls.LoadX509KeyPair(s.config.TLSCert, s.config.TLSKey)
		if err != nil {
			return err
		}

		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}

		s.tlsServer = &http.Server{
			Addr:         s.config.TLSAddress,
			Handler:      s.router,
			TLSConfig:    tlsConfig,
			ReadTimeout:  s.config.ReadTimeout,
			WriteTimeout: s.config.WriteTimeout,
			IdleTimeout:  s.config.IdleTimeout,
		}

		tlsListener, err := tls.Listen("tcp", s.config.TLSAddress, tlsConfig)
		if err != nil {
			return err
		}

		s.logger.WithSource("system").Info("REST TLS server started", "address", s.config.TLSAddress)

		go s.tlsServer.Serve(tlsListener)
	}

	return nil
}

// Stop gracefully stops the REST server.
func (s *Server) Stop(ctx context.Context) error {
	if s.server != nil {
		if err := s.server.Shutdown(ctx); err != nil {
			return err
		}
	}

	if s.tlsServer != nil {
		if err := s.tlsServer.Shutdown(ctx); err != nil {
			return err
		}
	}

	s.logger.WithSource("system").Info("REST server stopped")
	return nil
}

// SetRateLimit updates the rate limit at runtime.
func (s *Server) SetRateLimit(requestsPerSecond int) {
	atomic.StoreInt32(&s.rateLimit, int32(requestsPerSecond))
}

// GetRateLimit returns the current rate limit.
func (s *Server) GetRateLimit() int {
	return int(atomic.LoadInt32(&s.rateLimit))
}

// SetTokenTTL updates the JWT token TTL at runtime.
func (s *Server) SetTokenTTL(ttl time.Duration) {
	atomic.StoreInt64(&s.tokenTTL, int64(ttl))
	if s.auth != nil {
		s.auth.SetTokenTTL(ttl)
	}
}

// GetTokenTTL returns the current token TTL.
func (s *Server) GetTokenTTL() time.Duration {
	return time.Duration(atomic.LoadInt64(&s.tokenTTL))
}

// SetCORSOrigins updates the allowed CORS origins at runtime.
func (s *Server) SetCORSOrigins(origins []string) {
	s.corsMu.Lock()
	defer s.corsMu.Unlock()
	s.corsOrigins = origins
}

// GetCORSOrigins returns the current CORS origins.
func (s *Server) GetCORSOrigins() []string {
	s.corsMu.RLock()
	defer s.corsMu.RUnlock()
	result := make([]string, len(s.corsOrigins))
	copy(result, s.corsOrigins)
	return result
}

// SetACLManager sets the ACL manager for ACL-related endpoints.
func (s *Server) SetACLManager(m *acl.Manager) {
	s.handlers.SetACLManager(m)
}

// SetConfigManager sets the config manager for config-related endpoints.
func (s *Server) SetConfigManager(m *config.ConfigManager) {
	s.handlers.SetConfigManager(m)
}

// SetLogger sets the logger for log-related endpoints.
func (s *Server) SetLogger(logger logging.Logger) {
	s.handlers.SetLogger(logger)
}
