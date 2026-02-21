// Package server provides the LDAP server implementation.
package server

import (
	"crypto/tls"
	"crypto/x509"
	"errors"

	"github.com/KilimcininKorOglu/oba/internal/ldap"
)

// Security policy errors
var (
	// ErrSecurityPolicyViolation is returned when a security policy is violated
	ErrSecurityPolicyViolation = errors.New("server: security policy violation")
	// ErrClientCertRequired is returned when client certificate is required but not provided
	ErrClientCertRequired = errors.New("server: client certificate required")
	// ErrTLSVersionTooLow is returned when TLS version is below minimum required
	ErrTLSVersionTooLow = errors.New("server: TLS version too low")
	// ErrCipherSuiteNotAllowed is returned when cipher suite is not in allowed list
	ErrCipherSuiteNotAllowed = errors.New("server: cipher suite not allowed")
)

// SecurityPolicy defines security requirements for LDAP operations.
type SecurityPolicy struct {
	// RequireTLS indicates whether TLS is required for all operations
	RequireTLS bool
	// RequireTLSForBind indicates whether TLS is required for bind operations
	RequireTLSForBind bool
	// RequireTLSForPasswordChange indicates whether TLS is required for password changes
	RequireTLSForPasswordChange bool
	// RequireClientCert indicates whether client certificate is required
	RequireClientCert bool
	// MinTLSVersion is the minimum TLS version required (0 means no minimum)
	MinTLSVersion uint16
	// AllowedCipherSuites is the list of allowed cipher suites (empty means all allowed)
	AllowedCipherSuites []uint16
}

// DefaultSecurityPolicy returns a security policy with sensible defaults.
// By default, TLS is required for password changes but not for other operations.
func DefaultSecurityPolicy() *SecurityPolicy {
	return &SecurityPolicy{
		RequireTLS:                  false,
		RequireTLSForBind:           false,
		RequireTLSForPasswordChange: true,
		RequireClientCert:           false,
		MinTLSVersion:               tls.VersionTLS12,
		AllowedCipherSuites:         nil, // All cipher suites allowed
	}
}

// StrictSecurityPolicy returns a strict security policy that requires TLS for all operations.
func StrictSecurityPolicy() *SecurityPolicy {
	return &SecurityPolicy{
		RequireTLS:                  true,
		RequireTLSForBind:           true,
		RequireTLSForPasswordChange: true,
		RequireClientCert:           false,
		MinTLSVersion:               tls.VersionTLS12,
		AllowedCipherSuites:         nil,
	}
}

// SecurityEnforcer enforces security policies on connections.
type SecurityEnforcer struct {
	policy *SecurityPolicy
}

// NewSecurityEnforcer creates a new SecurityEnforcer with the given policy.
func NewSecurityEnforcer(policy *SecurityPolicy) *SecurityEnforcer {
	if policy == nil {
		policy = DefaultSecurityPolicy()
	}
	return &SecurityEnforcer{
		policy: policy,
	}
}

// GetPolicy returns the current security policy.
func (e *SecurityEnforcer) GetPolicy() *SecurityPolicy {
	return e.policy
}

// SetPolicy sets the security policy.
func (e *SecurityEnforcer) SetPolicy(policy *SecurityPolicy) {
	e.policy = policy
}

// CheckTLS checks if the connection meets TLS requirements.
// Returns nil if requirements are met, or an error describing the violation.
func (e *SecurityEnforcer) CheckTLS(conn *Connection) error {
	if e.policy == nil {
		return nil
	}

	// Check if TLS is required
	if e.policy.RequireTLS && !conn.IsTLS() {
		return ErrTLSRequired
	}

	// If not using TLS, no further checks needed
	if !conn.IsTLS() {
		return nil
	}

	// Check minimum TLS version
	if e.policy.MinTLSVersion > 0 {
		tlsVersion := conn.GetTLSVersion()
		if tlsVersion < e.policy.MinTLSVersion {
			return ErrTLSVersionTooLow
		}
	}

	// Check cipher suite
	if len(e.policy.AllowedCipherSuites) > 0 {
		cipherSuite := conn.GetCipherSuite()
		allowed := false
		for _, suite := range e.policy.AllowedCipherSuites {
			if suite == cipherSuite {
				allowed = true
				break
			}
		}
		if !allowed {
			return ErrCipherSuiteNotAllowed
		}
	}

	// Check client certificate requirement
	if e.policy.RequireClientCert && conn.GetClientCertificate() == nil {
		return ErrClientCertRequired
	}

	return nil
}

// CheckBindOperation checks if a bind operation is allowed on the connection.
func (e *SecurityEnforcer) CheckBindOperation(conn *Connection) error {
	if e.policy == nil {
		return nil
	}

	// Check general TLS requirements first
	if err := e.CheckTLS(conn); err != nil {
		return err
	}

	// Check bind-specific TLS requirement
	if e.policy.RequireTLSForBind && !conn.IsTLS() {
		return ErrTLSRequired
	}

	return nil
}

// CheckPasswordChangeOperation checks if a password change operation is allowed.
func (e *SecurityEnforcer) CheckPasswordChangeOperation(conn *Connection) error {
	if e.policy == nil {
		return nil
	}

	// Check general TLS requirements first
	if err := e.CheckTLS(conn); err != nil {
		return err
	}

	// Check password change-specific TLS requirement
	if e.policy.RequireTLSForPasswordChange && !conn.IsTLS() {
		return ErrTLSRequired
	}

	return nil
}

// TLSInfo contains information about a TLS connection for logging and auditing.
type TLSInfo struct {
	// IsTLS indicates whether the connection is using TLS
	IsTLS bool
	// Version is the TLS version (e.g., tls.VersionTLS12)
	Version uint16
	// VersionString is the human-readable TLS version
	VersionString string
	// CipherSuite is the cipher suite ID
	CipherSuite uint16
	// CipherSuiteName is the human-readable cipher suite name
	CipherSuiteName string
	// ServerName is the SNI server name
	ServerName string
	// HasClientCert indicates whether a client certificate was provided
	HasClientCert bool
	// ClientCertSubject is the subject of the client certificate
	ClientCertSubject string
	// ClientCertIssuer is the issuer of the client certificate
	ClientCertIssuer string
}

// GetTLSInfo extracts TLS information from a connection for logging.
func GetTLSInfo(conn *Connection) *TLSInfo {
	info := &TLSInfo{
		IsTLS: conn.IsTLS(),
	}

	if !info.IsTLS {
		return info
	}

	info.Version = conn.GetTLSVersion()
	info.VersionString = TLSVersionString(info.Version)
	info.CipherSuite = conn.GetCipherSuite()
	info.CipherSuiteName = CipherSuiteName(info.CipherSuite)
	info.ServerName = conn.GetServerName()

	clientCert := conn.GetClientCertificate()
	if clientCert != nil {
		info.HasClientCert = true
		info.ClientCertSubject = clientCert.Subject.String()
		info.ClientCertIssuer = clientCert.Issuer.String()
	}

	return info
}

// LogTLSInfo logs TLS connection information using the connection's logger.
func LogTLSInfo(conn *Connection) {
	info := GetTLSInfo(conn)
	logger := conn.Logger()

	if !info.IsTLS {
		logger.Debug("connection is not using TLS")
		return
	}

	logger.Info("TLS connection info",
		"version", info.VersionString,
		"cipher_suite", info.CipherSuiteName,
		"server_name", info.ServerName,
		"has_client_cert", info.HasClientCert)

	if info.HasClientCert {
		logger.Debug("client certificate info",
			"subject", info.ClientCertSubject,
			"issuer", info.ClientCertIssuer)
	}
}

// SecurityError represents a security policy violation with LDAP result code.
type SecurityError struct {
	// Code is the LDAP result code
	Code ldap.ResultCode
	// Message is the error message
	Message string
	// Err is the underlying error
	Err error
}

// Error implements the error interface.
func (e *SecurityError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

// Unwrap returns the underlying error.
func (e *SecurityError) Unwrap() error {
	return e.Err
}

// NewSecurityError creates a new SecurityError.
func NewSecurityError(code ldap.ResultCode, message string, err error) *SecurityError {
	return &SecurityError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// ToOperationResult converts a security error to an OperationResult.
func (e *SecurityError) ToOperationResult() *OperationResult {
	return &OperationResult{
		ResultCode:        e.Code,
		DiagnosticMessage: e.Message,
	}
}

// CheckAndReturnError checks a security condition and returns a SecurityError if violated.
// This is a helper function for common security checks.
func CheckAndReturnError(conn *Connection, enforcer *SecurityEnforcer, operation string) *SecurityError {
	var err error

	switch operation {
	case "bind":
		err = enforcer.CheckBindOperation(conn)
	case "password_change":
		err = enforcer.CheckPasswordChangeOperation(conn)
	default:
		err = enforcer.CheckTLS(conn)
	}

	if err == nil {
		return nil
	}

	// Map errors to LDAP result codes
	var code ldap.ResultCode
	var message string

	switch {
	case errors.Is(err, ErrTLSRequired):
		code = ldap.ResultConfidentialityRequired
		message = "TLS required for this operation"
	case errors.Is(err, ErrClientCertRequired):
		code = ldap.ResultStrongerAuthRequired
		message = "client certificate required"
	case errors.Is(err, ErrTLSVersionTooLow):
		code = ldap.ResultConfidentialityRequired
		message = "TLS version too low"
	case errors.Is(err, ErrCipherSuiteNotAllowed):
		code = ldap.ResultConfidentialityRequired
		message = "cipher suite not allowed"
	default:
		code = ldap.ResultOperationsError
		message = "security policy violation"
	}

	return NewSecurityError(code, message, err)
}

// ValidateClientCertificate validates a client certificate against a CA pool.
// Returns nil if the certificate is valid, or an error if validation fails.
func ValidateClientCertificate(cert *x509.Certificate, roots *x509.CertPool) error {
	if cert == nil {
		return ErrClientCertRequired
	}

	opts := x509.VerifyOptions{
		Roots: roots,
	}

	_, err := cert.Verify(opts)
	return err
}
