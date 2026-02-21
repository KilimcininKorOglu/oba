// Package server provides the LDAP server implementation.
package server

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/KilimcininKorOglu/oba/internal/ldap"
)

// generateTestTLSCertificate generates a self-signed certificate for testing.
func generateTestTLSCertificate() (tls.Certificate, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test Org"},
			CommonName:   "test.example.com",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, err
	}

	cert := tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  priv,
	}

	// Parse the certificate to populate the Leaf field
	parsedCert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return tls.Certificate{}, err
	}
	cert.Leaf = parsedCert

	return cert, nil
}

// mockTLSConn implements net.Conn for testing TLS connections
type mockTLSConn struct {
	*mockConn
	state tls.ConnectionState
}

func newMockTLSConn(version uint16, cipherSuite uint16, peerCerts []*x509.Certificate) *mockTLSConn {
	return &mockTLSConn{
		mockConn: newMockConn(),
		state: tls.ConnectionState{
			Version:          version,
			CipherSuite:      cipherSuite,
			PeerCertificates: peerCerts,
			ServerName:       "test.example.com",
		},
	}
}

func (m *mockTLSConn) ConnectionState() tls.ConnectionState {
	return m.state
}

func TestDefaultSecurityPolicy(t *testing.T) {
	policy := DefaultSecurityPolicy()

	if policy.RequireTLS {
		t.Error("Default policy should not require TLS for all operations")
	}
	if policy.RequireTLSForBind {
		t.Error("Default policy should not require TLS for bind")
	}
	if !policy.RequireTLSForPasswordChange {
		t.Error("Default policy should require TLS for password change")
	}
	if policy.RequireClientCert {
		t.Error("Default policy should not require client certificate")
	}
	if policy.MinTLSVersion != tls.VersionTLS12 {
		t.Errorf("Expected MinTLSVersion TLS 1.2, got %d", policy.MinTLSVersion)
	}
}

func TestStrictSecurityPolicy(t *testing.T) {
	policy := StrictSecurityPolicy()

	if !policy.RequireTLS {
		t.Error("Strict policy should require TLS for all operations")
	}
	if !policy.RequireTLSForBind {
		t.Error("Strict policy should require TLS for bind")
	}
	if !policy.RequireTLSForPasswordChange {
		t.Error("Strict policy should require TLS for password change")
	}
}

func TestSecurityEnforcerCheckTLS(t *testing.T) {
	tests := []struct {
		name          string
		policy        *SecurityPolicy
		isTLS         bool
		tlsVersion    uint16
		cipherSuite   uint16
		hasClientCert bool
		wantErr       error
	}{
		{
			name:    "no TLS required, no TLS connection",
			policy:  &SecurityPolicy{RequireTLS: false},
			isTLS:   false,
			wantErr: nil,
		},
		{
			name:    "TLS required, no TLS connection",
			policy:  &SecurityPolicy{RequireTLS: true},
			isTLS:   false,
			wantErr: ErrTLSRequired,
		},
		{
			name:       "TLS required, TLS connection",
			policy:     &SecurityPolicy{RequireTLS: true},
			isTLS:      true,
			tlsVersion: tls.VersionTLS12,
			wantErr:    nil,
		},
		{
			name:       "min TLS version check - pass",
			policy:     &SecurityPolicy{MinTLSVersion: tls.VersionTLS12},
			isTLS:      true,
			tlsVersion: tls.VersionTLS13,
			wantErr:    nil,
		},
		{
			name:       "min TLS version check - fail",
			policy:     &SecurityPolicy{MinTLSVersion: tls.VersionTLS13},
			isTLS:      true,
			tlsVersion: tls.VersionTLS12,
			wantErr:    ErrTLSVersionTooLow,
		},
		{
			name:        "cipher suite check - pass",
			policy:      &SecurityPolicy{AllowedCipherSuites: []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256}},
			isTLS:       true,
			tlsVersion:  tls.VersionTLS12,
			cipherSuite: tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			wantErr:     nil,
		},
		{
			name:        "cipher suite check - fail",
			policy:      &SecurityPolicy{AllowedCipherSuites: []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256}},
			isTLS:       true,
			tlsVersion:  tls.VersionTLS12,
			cipherSuite: tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			wantErr:     ErrCipherSuiteNotAllowed,
		},
		{
			name:          "client cert required - pass",
			policy:        &SecurityPolicy{RequireClientCert: true},
			isTLS:         true,
			tlsVersion:    tls.VersionTLS12,
			hasClientCert: true,
			wantErr:       nil,
		},
		{
			name:          "client cert required - fail",
			policy:        &SecurityPolicy{RequireClientCert: true},
			isTLS:         true,
			tlsVersion:    tls.VersionTLS12,
			hasClientCert: false,
			wantErr:       ErrClientCertRequired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enforcer := NewSecurityEnforcer(tt.policy)
			conn := createTestConnection(tt.isTLS, tt.tlsVersion, tt.cipherSuite, tt.hasClientCert)

			err := enforcer.CheckTLS(conn)

			if tt.wantErr == nil && err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
			if tt.wantErr != nil && err != tt.wantErr {
				t.Errorf("Expected error %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestSecurityEnforcerCheckBindOperation(t *testing.T) {
	tests := []struct {
		name    string
		policy  *SecurityPolicy
		isTLS   bool
		wantErr error
	}{
		{
			name:    "bind TLS not required, no TLS",
			policy:  &SecurityPolicy{RequireTLSForBind: false},
			isTLS:   false,
			wantErr: nil,
		},
		{
			name:    "bind TLS required, no TLS",
			policy:  &SecurityPolicy{RequireTLSForBind: true},
			isTLS:   false,
			wantErr: ErrTLSRequired,
		},
		{
			name:    "bind TLS required, with TLS",
			policy:  &SecurityPolicy{RequireTLSForBind: true},
			isTLS:   true,
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enforcer := NewSecurityEnforcer(tt.policy)
			conn := createTestConnection(tt.isTLS, tls.VersionTLS12, 0, false)

			err := enforcer.CheckBindOperation(conn)

			if tt.wantErr == nil && err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
			if tt.wantErr != nil && err != tt.wantErr {
				t.Errorf("Expected error %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestSecurityEnforcerCheckPasswordChangeOperation(t *testing.T) {
	tests := []struct {
		name    string
		policy  *SecurityPolicy
		isTLS   bool
		wantErr error
	}{
		{
			name:    "password change TLS not required, no TLS",
			policy:  &SecurityPolicy{RequireTLSForPasswordChange: false},
			isTLS:   false,
			wantErr: nil,
		},
		{
			name:    "password change TLS required, no TLS",
			policy:  &SecurityPolicy{RequireTLSForPasswordChange: true},
			isTLS:   false,
			wantErr: ErrTLSRequired,
		},
		{
			name:    "password change TLS required, with TLS",
			policy:  &SecurityPolicy{RequireTLSForPasswordChange: true},
			isTLS:   true,
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enforcer := NewSecurityEnforcer(tt.policy)
			conn := createTestConnection(tt.isTLS, tls.VersionTLS12, 0, false)

			err := enforcer.CheckPasswordChangeOperation(conn)

			if tt.wantErr == nil && err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
			if tt.wantErr != nil && err != tt.wantErr {
				t.Errorf("Expected error %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestGetTLSInfo(t *testing.T) {
	// Test non-TLS connection
	t.Run("non-TLS connection", func(t *testing.T) {
		conn := createTestConnection(false, 0, 0, false)
		info := GetTLSInfo(conn)

		if info.IsTLS {
			t.Error("Expected IsTLS to be false")
		}
		if info.Version != 0 {
			t.Errorf("Expected Version 0, got %d", info.Version)
		}
	})

	// Test TLS connection
	t.Run("TLS connection", func(t *testing.T) {
		conn := createTestConnection(true, tls.VersionTLS13, tls.TLS_AES_128_GCM_SHA256, false)
		info := GetTLSInfo(conn)

		if !info.IsTLS {
			t.Error("Expected IsTLS to be true")
		}
		if info.Version != tls.VersionTLS13 {
			t.Errorf("Expected Version TLS 1.3, got %d", info.Version)
		}
		if info.VersionString != "TLS 1.3" {
			t.Errorf("Expected VersionString 'TLS 1.3', got %s", info.VersionString)
		}
	})

	// Test TLS connection with client cert
	t.Run("TLS connection with client cert", func(t *testing.T) {
		conn := createTestConnection(true, tls.VersionTLS12, tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256, true)
		info := GetTLSInfo(conn)

		if !info.HasClientCert {
			t.Error("Expected HasClientCert to be true")
		}
		if info.ClientCertSubject == "" {
			t.Error("Expected ClientCertSubject to be non-empty")
		}
	})
}

func TestSecurityError(t *testing.T) {
	err := NewSecurityError(ldap.ResultConfidentialityRequired, "TLS required", ErrTLSRequired)

	if err.Code != ldap.ResultConfidentialityRequired {
		t.Errorf("Expected code %d, got %d", ldap.ResultConfidentialityRequired, err.Code)
	}
	if err.Message != "TLS required" {
		t.Errorf("Expected message 'TLS required', got %s", err.Message)
	}
	if err.Unwrap() != ErrTLSRequired {
		t.Error("Unwrap should return underlying error")
	}

	// Test Error() method
	errStr := err.Error()
	if errStr != "TLS required: server: TLS required for this operation" {
		t.Errorf("Unexpected error string: %s", errStr)
	}

	// Test ToOperationResult
	result := err.ToOperationResult()
	if result.ResultCode != ldap.ResultConfidentialityRequired {
		t.Errorf("Expected result code %d, got %d", ldap.ResultConfidentialityRequired, result.ResultCode)
	}
	if result.DiagnosticMessage != "TLS required" {
		t.Errorf("Expected diagnostic message 'TLS required', got %s", result.DiagnosticMessage)
	}
}

func TestCheckAndReturnError(t *testing.T) {
	tests := []struct {
		name      string
		policy    *SecurityPolicy
		isTLS     bool
		operation string
		wantCode  ldap.ResultCode
		wantNil   bool
	}{
		{
			name:      "bind operation - TLS required, no TLS",
			policy:    &SecurityPolicy{RequireTLSForBind: true},
			isTLS:     false,
			operation: "bind",
			wantCode:  ldap.ResultConfidentialityRequired,
			wantNil:   false,
		},
		{
			name:      "bind operation - TLS required, with TLS",
			policy:    &SecurityPolicy{RequireTLSForBind: true},
			isTLS:     true,
			operation: "bind",
			wantNil:   true,
		},
		{
			name:      "password change - TLS required, no TLS",
			policy:    &SecurityPolicy{RequireTLSForPasswordChange: true},
			isTLS:     false,
			operation: "password_change",
			wantCode:  ldap.ResultConfidentialityRequired,
			wantNil:   false,
		},
		{
			name:      "general operation - TLS required, no TLS",
			policy:    &SecurityPolicy{RequireTLS: true},
			isTLS:     false,
			operation: "search",
			wantCode:  ldap.ResultConfidentialityRequired,
			wantNil:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enforcer := NewSecurityEnforcer(tt.policy)
			conn := createTestConnection(tt.isTLS, tls.VersionTLS12, 0, false)

			secErr := CheckAndReturnError(conn, enforcer, tt.operation)

			if tt.wantNil && secErr != nil {
				t.Errorf("Expected nil error, got %v", secErr)
			}
			if !tt.wantNil && secErr == nil {
				t.Error("Expected error, got nil")
			}
			if !tt.wantNil && secErr != nil && secErr.Code != tt.wantCode {
				t.Errorf("Expected code %d, got %d", tt.wantCode, secErr.Code)
			}
		})
	}
}

func TestNewSecurityEnforcerWithNilPolicy(t *testing.T) {
	enforcer := NewSecurityEnforcer(nil)

	if enforcer.policy == nil {
		t.Error("Expected default policy to be set")
	}

	// Should use default policy
	conn := createTestConnection(false, 0, 0, false)
	err := enforcer.CheckTLS(conn)
	if err != nil {
		t.Errorf("Expected no error with default policy, got %v", err)
	}
}

func TestSecurityEnforcerGetSetPolicy(t *testing.T) {
	enforcer := NewSecurityEnforcer(DefaultSecurityPolicy())

	// Get policy
	policy := enforcer.GetPolicy()
	if policy == nil {
		t.Error("GetPolicy should not return nil")
	}

	// Set new policy
	strictPolicy := StrictSecurityPolicy()
	enforcer.SetPolicy(strictPolicy)

	if enforcer.GetPolicy() != strictPolicy {
		t.Error("SetPolicy did not update the policy")
	}
}

// createTestConnection creates a test connection with the specified TLS settings.
func createTestConnection(isTLS bool, tlsVersion uint16, cipherSuite uint16, hasClientCert bool) *Connection {
	mockConn := newMockConn()
	conn := NewConnection(mockConn, nil)

	if isTLS {
		// Manually set TLS state since we can't use a real TLS connection in tests
		conn.mu.Lock()
		conn.isTLS = true
		conn.tlsState = &tls.ConnectionState{
			Version:     tlsVersion,
			CipherSuite: cipherSuite,
			ServerName:  "test.example.com",
		}
		if hasClientCert {
			// Create a test certificate
			cert, err := generateTestTLSCertificate()
			if err == nil && cert.Leaf != nil {
				conn.clientCert = cert.Leaf
				conn.tlsState.PeerCertificates = []*x509.Certificate{cert.Leaf}
			}
		}
		conn.mu.Unlock()
	}

	return conn
}

func TestConnectionRequireTLS(t *testing.T) {
	// Test non-TLS connection
	t.Run("non-TLS connection", func(t *testing.T) {
		conn := createTestConnection(false, 0, 0, false)
		err := conn.RequireTLS()
		if err != ErrTLSRequired {
			t.Errorf("Expected ErrTLSRequired, got %v", err)
		}
	})

	// Test TLS connection
	t.Run("TLS connection", func(t *testing.T) {
		conn := createTestConnection(true, tls.VersionTLS12, 0, false)
		err := conn.RequireTLS()
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})
}

func TestConnectionGetTLSVersion(t *testing.T) {
	// Test non-TLS connection
	t.Run("non-TLS connection", func(t *testing.T) {
		conn := createTestConnection(false, 0, 0, false)
		version := conn.GetTLSVersion()
		if version != 0 {
			t.Errorf("Expected 0, got %d", version)
		}
	})

	// Test TLS 1.2 connection
	t.Run("TLS 1.2 connection", func(t *testing.T) {
		conn := createTestConnection(true, tls.VersionTLS12, 0, false)
		version := conn.GetTLSVersion()
		if version != tls.VersionTLS12 {
			t.Errorf("Expected TLS 1.2, got %d", version)
		}
	})

	// Test TLS 1.3 connection
	t.Run("TLS 1.3 connection", func(t *testing.T) {
		conn := createTestConnection(true, tls.VersionTLS13, 0, false)
		version := conn.GetTLSVersion()
		if version != tls.VersionTLS13 {
			t.Errorf("Expected TLS 1.3, got %d", version)
		}
	})
}

func TestConnectionGetCipherSuite(t *testing.T) {
	// Test non-TLS connection
	t.Run("non-TLS connection", func(t *testing.T) {
		conn := createTestConnection(false, 0, 0, false)
		suite := conn.GetCipherSuite()
		if suite != 0 {
			t.Errorf("Expected 0, got %d", suite)
		}
	})

	// Test TLS connection with cipher suite
	t.Run("TLS connection with cipher suite", func(t *testing.T) {
		conn := createTestConnection(true, tls.VersionTLS12, tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256, false)
		suite := conn.GetCipherSuite()
		if suite != tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256 {
			t.Errorf("Expected cipher suite %d, got %d", tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256, suite)
		}
	})
}

func TestConnectionGetClientCertificate(t *testing.T) {
	// Test non-TLS connection
	t.Run("non-TLS connection", func(t *testing.T) {
		conn := createTestConnection(false, 0, 0, false)
		cert := conn.GetClientCertificate()
		if cert != nil {
			t.Error("Expected nil certificate")
		}
	})

	// Test TLS connection without client cert
	t.Run("TLS connection without client cert", func(t *testing.T) {
		conn := createTestConnection(true, tls.VersionTLS12, 0, false)
		cert := conn.GetClientCertificate()
		if cert != nil {
			t.Error("Expected nil certificate")
		}
	})

	// Test TLS connection with client cert
	t.Run("TLS connection with client cert", func(t *testing.T) {
		conn := createTestConnection(true, tls.VersionTLS12, 0, true)
		cert := conn.GetClientCertificate()
		if cert == nil {
			t.Error("Expected non-nil certificate")
		}
	})
}

func TestConnectionGetServerName(t *testing.T) {
	// Test non-TLS connection
	t.Run("non-TLS connection", func(t *testing.T) {
		conn := createTestConnection(false, 0, 0, false)
		name := conn.GetServerName()
		if name != "" {
			t.Errorf("Expected empty string, got %s", name)
		}
	})

	// Test TLS connection
	t.Run("TLS connection", func(t *testing.T) {
		conn := createTestConnection(true, tls.VersionTLS12, 0, false)
		name := conn.GetServerName()
		if name != "test.example.com" {
			t.Errorf("Expected 'test.example.com', got %s", name)
		}
	})
}

func TestConnectionGetTLSState(t *testing.T) {
	// Test non-TLS connection
	t.Run("non-TLS connection", func(t *testing.T) {
		conn := createTestConnection(false, 0, 0, false)
		state := conn.GetTLSState()
		if state != nil {
			t.Error("Expected nil state")
		}
	})

	// Test TLS connection
	t.Run("TLS connection", func(t *testing.T) {
		conn := createTestConnection(true, tls.VersionTLS12, tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256, false)
		state := conn.GetTLSState()
		if state == nil {
			t.Error("Expected non-nil state")
		}
		if state.Version != tls.VersionTLS12 {
			t.Errorf("Expected TLS 1.2, got %d", state.Version)
		}
	})
}

func TestLogTLSInfo(t *testing.T) {
	// Test non-TLS connection - should not panic
	t.Run("non-TLS connection", func(t *testing.T) {
		conn := createTestConnection(false, 0, 0, false)
		// Should not panic
		LogTLSInfo(conn)
	})

	// Test TLS connection - should not panic
	t.Run("TLS connection", func(t *testing.T) {
		conn := createTestConnection(true, tls.VersionTLS12, tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256, false)
		// Should not panic
		LogTLSInfo(conn)
	})

	// Test TLS connection with client cert - should not panic
	t.Run("TLS connection with client cert", func(t *testing.T) {
		conn := createTestConnection(true, tls.VersionTLS12, tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256, true)
		// Should not panic
		LogTLSInfo(conn)
	})
}

func TestValidateClientCertificate(t *testing.T) {
	// Test nil certificate
	t.Run("nil certificate", func(t *testing.T) {
		err := ValidateClientCertificate(nil, nil)
		if err != ErrClientCertRequired {
			t.Errorf("Expected ErrClientCertRequired, got %v", err)
		}
	})

	// Test valid certificate with matching CA
	t.Run("valid certificate", func(t *testing.T) {
		cert, err := generateTestTLSCertificate()
		if err != nil {
			t.Fatalf("Failed to generate test certificate: %v", err)
		}

		// Create a CA pool with the self-signed cert
		roots := x509.NewCertPool()
		roots.AddCert(cert.Leaf)

		err = ValidateClientCertificate(cert.Leaf, roots)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})

	// Test certificate with non-matching CA
	t.Run("certificate with non-matching CA", func(t *testing.T) {
		cert, err := generateTestTLSCertificate()
		if err != nil {
			t.Fatalf("Failed to generate test certificate: %v", err)
		}

		// Create an empty CA pool
		roots := x509.NewCertPool()

		err = ValidateClientCertificate(cert.Leaf, roots)
		if err == nil {
			t.Error("Expected error for non-matching CA")
		}
	})
}
