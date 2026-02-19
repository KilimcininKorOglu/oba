// Package server provides TLS configuration handling for the Oba LDAP server.
package server

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
)

// TLS version constants for convenience.
const (
	TLSVersion10 = tls.VersionTLS10
	TLSVersion11 = tls.VersionTLS11
	TLSVersion12 = tls.VersionTLS12
	TLSVersion13 = tls.VersionTLS13
)

// Default cipher suites (secure defaults for TLS 1.2).
// TLS 1.3 cipher suites are automatically managed by Go.
var defaultCipherSuites = []uint16{
	tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
	tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
	tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
	tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
}

// TLS configuration errors.
var (
	ErrNoCertificate       = errors.New("no certificate provided")
	ErrNoPrivateKey        = errors.New("no private key provided")
	ErrCertKeyMismatch     = errors.New("certificate and private key do not match")
	ErrInvalidTLSVersion   = errors.New("invalid TLS version")
	ErrMinVersionTooHigh   = errors.New("minimum TLS version is higher than maximum")
	ErrInvalidCipherSuite  = errors.New("invalid cipher suite")
	ErrCertFileNotFound    = errors.New("certificate file not found")
	ErrKeyFileNotFound     = errors.New("private key file not found")
	ErrInvalidCertPEM      = errors.New("invalid certificate PEM data")
	ErrInvalidKeyPEM       = errors.New("invalid private key PEM data")
	ErrInvalidClientCAPEM  = errors.New("invalid client CA PEM data")
)

// TLSConfig holds the TLS configuration options.
type TLSConfig struct {
	// CertFile is the path to the certificate file.
	CertFile string

	// KeyFile is the path to the private key file.
	KeyFile string

	// CertPEM is the certificate in PEM format.
	CertPEM []byte

	// KeyPEM is the private key in PEM format.
	KeyPEM []byte

	// MinVersion is the minimum TLS version (default: TLS 1.2).
	MinVersion uint16

	// MaxVersion is the maximum TLS version (default: TLS 1.3).
	MaxVersion uint16

	// CipherSuites is the list of allowed cipher suites.
	// If empty, secure defaults are used.
	CipherSuites []uint16

	// ClientAuth specifies the client authentication policy.
	ClientAuth tls.ClientAuthType

	// ClientCAs is the pool of client certificate authorities.
	ClientCAs *x509.CertPool

	// ClientCAPEM is the client CA certificates in PEM format.
	// Used to build ClientCAs if ClientCAs is nil.
	ClientCAPEM []byte
}

// NewTLSConfig creates a new TLSConfig with secure defaults.
func NewTLSConfig() *TLSConfig {
	return &TLSConfig{
		MinVersion:   TLSVersion12,
		MaxVersion:   TLSVersion13,
		CipherSuites: nil, // Will use defaults
		ClientAuth:   tls.NoClientCert,
	}
}

// WithCertFile sets the certificate and key file paths.
func (c *TLSConfig) WithCertFile(certFile, keyFile string) *TLSConfig {
	c.CertFile = certFile
	c.KeyFile = keyFile
	return c
}

// WithCertPEM sets the certificate and key from PEM data.
func (c *TLSConfig) WithCertPEM(certPEM, keyPEM []byte) *TLSConfig {
	c.CertPEM = certPEM
	c.KeyPEM = keyPEM
	return c
}

// WithMinVersion sets the minimum TLS version.
func (c *TLSConfig) WithMinVersion(version uint16) *TLSConfig {
	c.MinVersion = version
	return c
}

// WithMaxVersion sets the maximum TLS version.
func (c *TLSConfig) WithMaxVersion(version uint16) *TLSConfig {
	c.MaxVersion = version
	return c
}

// WithCipherSuites sets the allowed cipher suites.
func (c *TLSConfig) WithCipherSuites(suites []uint16) *TLSConfig {
	c.CipherSuites = suites
	return c
}

// WithClientAuth sets the client authentication policy.
func (c *TLSConfig) WithClientAuth(authType tls.ClientAuthType) *TLSConfig {
	c.ClientAuth = authType
	return c
}

// WithClientCAs sets the client CA pool.
func (c *TLSConfig) WithClientCAs(pool *x509.CertPool) *TLSConfig {
	c.ClientCAs = pool
	return c
}

// WithClientCAPEM sets the client CA certificates from PEM data.
func (c *TLSConfig) WithClientCAPEM(pem []byte) *TLSConfig {
	c.ClientCAPEM = pem
	return c
}

// LoadTLSConfig creates a *tls.Config from the TLSConfig.
// It validates the configuration and loads certificates.
func LoadTLSConfig(cfg *TLSConfig) (*tls.Config, error) {
	if cfg == nil {
		return nil, ErrNoCertificate
	}

	// Validate TLS versions
	if err := validateTLSVersions(cfg.MinVersion, cfg.MaxVersion); err != nil {
		return nil, err
	}

	// Load certificate
	cert, err := loadCertificateFromConfig(cfg)
	if err != nil {
		return nil, err
	}

	// Build client CA pool if needed
	clientCAs := cfg.ClientCAs
	if clientCAs == nil && len(cfg.ClientCAPEM) > 0 {
		clientCAs = x509.NewCertPool()
		if !clientCAs.AppendCertsFromPEM(cfg.ClientCAPEM) {
			return nil, ErrInvalidClientCAPEM
		}
	}

	// Determine cipher suites
	cipherSuites := cfg.CipherSuites
	if len(cipherSuites) == 0 {
		cipherSuites = defaultCipherSuites
	}

	// Validate cipher suites
	if err := validateCipherSuites(cipherSuites); err != nil {
		return nil, err
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   cfg.MinVersion,
		MaxVersion:   cfg.MaxVersion,
		CipherSuites: cipherSuites,
		ClientAuth:   cfg.ClientAuth,
		ClientCAs:    clientCAs,
	}

	return tlsConfig, nil
}

// LoadCertificate loads a certificate from file paths.
func LoadCertificate(certFile, keyFile string) (tls.Certificate, error) {
	if certFile == "" {
		return tls.Certificate{}, ErrNoCertificate
	}
	if keyFile == "" {
		return tls.Certificate{}, ErrNoPrivateKey
	}

	// Check if files exist
	if _, err := os.Stat(certFile); os.IsNotExist(err) {
		return tls.Certificate{}, ErrCertFileNotFound
	}
	if _, err := os.Stat(keyFile); os.IsNotExist(err) {
		return tls.Certificate{}, ErrKeyFileNotFound
	}

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		// Try to provide more specific error
		if isKeyMismatchError(err) {
			return tls.Certificate{}, ErrCertKeyMismatch
		}
		return tls.Certificate{}, fmt.Errorf("failed to load certificate: %w", err)
	}

	return cert, nil
}

// LoadCertificateFromPEM loads a certificate from PEM-encoded data.
func LoadCertificateFromPEM(certPEM, keyPEM []byte) (tls.Certificate, error) {
	if len(certPEM) == 0 {
		return tls.Certificate{}, ErrNoCertificate
	}
	if len(keyPEM) == 0 {
		return tls.Certificate{}, ErrNoPrivateKey
	}

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		// Try to provide more specific error
		if isKeyMismatchError(err) {
			return tls.Certificate{}, ErrCertKeyMismatch
		}
		if isPEMParseError(err, "certificate") {
			return tls.Certificate{}, ErrInvalidCertPEM
		}
		if isPEMParseError(err, "key") {
			return tls.Certificate{}, ErrInvalidKeyPEM
		}
		return tls.Certificate{}, fmt.Errorf("failed to parse certificate: %w", err)
	}

	return cert, nil
}

// loadCertificateFromConfig loads a certificate from the TLSConfig.
// It prefers PEM data over file paths.
func loadCertificateFromConfig(cfg *TLSConfig) (tls.Certificate, error) {
	// Prefer PEM data if provided
	if len(cfg.CertPEM) > 0 || len(cfg.KeyPEM) > 0 {
		return LoadCertificateFromPEM(cfg.CertPEM, cfg.KeyPEM)
	}

	// Fall back to file paths
	if cfg.CertFile != "" || cfg.KeyFile != "" {
		return LoadCertificate(cfg.CertFile, cfg.KeyFile)
	}

	return tls.Certificate{}, ErrNoCertificate
}

// validateTLSVersions validates the TLS version configuration.
func validateTLSVersions(minVersion, maxVersion uint16) error {
	// If both are zero, use defaults
	if minVersion == 0 && maxVersion == 0 {
		return nil
	}

	// Validate min version
	if minVersion != 0 && !isValidTLSVersion(minVersion) {
		return fmt.Errorf("%w: min version 0x%04x", ErrInvalidTLSVersion, minVersion)
	}

	// Validate max version
	if maxVersion != 0 && !isValidTLSVersion(maxVersion) {
		return fmt.Errorf("%w: max version 0x%04x", ErrInvalidTLSVersion, maxVersion)
	}

	// Check min <= max
	if minVersion != 0 && maxVersion != 0 && minVersion > maxVersion {
		return ErrMinVersionTooHigh
	}

	return nil
}

// isValidTLSVersion checks if the given version is a valid TLS version.
func isValidTLSVersion(version uint16) bool {
	switch version {
	case tls.VersionTLS10, tls.VersionTLS11, tls.VersionTLS12, tls.VersionTLS13:
		return true
	default:
		return false
	}
}

// validateCipherSuites validates the cipher suite configuration.
func validateCipherSuites(suites []uint16) error {
	validSuites := make(map[uint16]bool)
	for _, suite := range tls.CipherSuites() {
		validSuites[suite.ID] = true
	}
	// Also include insecure suites for validation (they might be intentionally used)
	for _, suite := range tls.InsecureCipherSuites() {
		validSuites[suite.ID] = true
	}

	for _, suite := range suites {
		if !validSuites[suite] {
			return fmt.Errorf("%w: 0x%04x", ErrInvalidCipherSuite, suite)
		}
	}

	return nil
}

// isKeyMismatchError checks if the error indicates a certificate/key mismatch.
func isKeyMismatchError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return contains(errStr, "private key does not match") ||
		contains(errStr, "private key type does not match")
}

// isPEMParseError checks if the error indicates a PEM parsing failure.
func isPEMParseError(err error, pemType string) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return contains(errStr, "failed to find any PEM data") ||
		(pemType == "certificate" && contains(errStr, "failed to parse certificate")) ||
		(pemType == "key" && contains(errStr, "failed to parse private key"))
}

// contains checks if s contains substr (case-insensitive).
func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

// findSubstring performs a simple substring search.
func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if equalFold(s[i:i+len(substr)], substr) {
			return true
		}
	}
	return false
}

// equalFold compares two strings case-insensitively.
func equalFold(s, t string) bool {
	if len(s) != len(t) {
		return false
	}
	for i := 0; i < len(s); i++ {
		if toLower(s[i]) != toLower(t[i]) {
			return false
		}
	}
	return true
}

// toLower converts a byte to lowercase.
func toLower(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + ('a' - 'A')
	}
	return c
}

// GetDefaultCipherSuites returns a copy of the default cipher suites.
func GetDefaultCipherSuites() []uint16 {
	result := make([]uint16, len(defaultCipherSuites))
	copy(result, defaultCipherSuites)
	return result
}

// TLSVersionString returns a human-readable string for a TLS version.
func TLSVersionString(version uint16) string {
	switch version {
	case tls.VersionTLS10:
		return "TLS 1.0"
	case tls.VersionTLS11:
		return "TLS 1.1"
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS13:
		return "TLS 1.3"
	default:
		return fmt.Sprintf("unknown (0x%04x)", version)
	}
}

// CipherSuiteName returns the name of a cipher suite.
func CipherSuiteName(id uint16) string {
	// Check standard cipher suites
	for _, suite := range tls.CipherSuites() {
		if suite.ID == id {
			return suite.Name
		}
	}
	// Check insecure cipher suites
	for _, suite := range tls.InsecureCipherSuites() {
		if suite.ID == id {
			return suite.Name
		}
	}
	return fmt.Sprintf("unknown (0x%04x)", id)
}
