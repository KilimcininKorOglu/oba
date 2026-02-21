package rest

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"time"

	"github.com/oba-ldap/oba/internal/backend"
	"github.com/oba-ldap/oba/internal/logging"
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
}

// NewServer creates a new REST server.
func NewServer(cfg *ServerConfig, be *backend.ObaBackend, logger logging.Logger) *Server {
	auth := NewAuthenticator(be, cfg.JWTSecret, cfg.TokenTTL)
	handlers := NewHandlers(be, auth)

	router := NewRouter()

	s := &Server{
		config:   cfg,
		backend:  be,
		logger:   logger,
		auth:     auth,
		handlers: handlers,
		router:   router,
	}

	s.setupRoutes()
	s.setupMiddleware()

	return s
}

func (s *Server) setupRoutes() {
	s.router.GET("/api/v1/health", s.handlers.HandleHealth)

	s.router.POST("/api/v1/auth/bind", s.handlers.HandleBind)

	s.router.GET("/api/v1/entries/{dn}", s.handlers.HandleGetEntry)
	s.router.POST("/api/v1/entries", s.handlers.HandleAddEntry)
	s.router.PUT("/api/v1/entries/{dn}", s.handlers.HandleModifyEntry)
	s.router.PATCH("/api/v1/entries/{dn}", s.handlers.HandleModifyEntry)
	s.router.DELETE("/api/v1/entries/{dn}", s.handlers.HandleDeleteEntry)
	s.router.POST("/api/v1/entries/{dn}/move", s.handlers.HandleModifyDN)

	s.router.GET("/api/v1/search", s.handlers.HandleSearch)
	s.router.GET("/api/v1/search/stream", s.handlers.HandleStreamSearch)

	s.router.POST("/api/v1/bulk", s.handlers.HandleBulk)

	s.router.POST("/api/v1/compare", s.handlers.HandleCompare)
}

func (s *Server) setupMiddleware() {
	s.router.Use(RecoveryMiddleware(s.logger))
	s.router.Use(LoggingMiddleware(s.logger))
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
	}))
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

	s.logger.Info("REST server started", "address", s.config.Address)

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

		s.logger.Info("REST TLS server started", "address", s.config.TLSAddress)

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

	s.logger.Info("REST server stopped")
	return nil
}
