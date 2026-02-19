package server

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

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

// generateTestCA creates a CA certificate for testing client auth.
func generateTestCA() (caPEM []byte, err error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test CA"},
			CommonName:   "Test CA",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, err
	}

	caPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	return caPEM, nil
}

// TestNewTLSConfig tests creating a new TLS config with defaults.
func TestNewTLSConfig(t *testing.T) {
	cfg := NewTLSConfig()

	if cfg == nil {
		t.Fatal("NewTLSConfig returned nil")
	}

	if cfg.MinVersion != TLSVersion12 {
		t.Errorf("expected MinVersion TLS 1.2, got 0x%04x", cfg.MinVersion)
	}

	if cfg.MaxVersion != TLSVersion13 {
		t.Errorf("expected MaxVersion TLS 1.3, got 0x%04x", cfg.MaxVersion)
	}

	if cfg.ClientAuth != tls.NoClientCert {
		t.Errorf("expected ClientAuth NoClientCert, got %v", cfg.ClientAuth)
	}
}

// TestTLSConfigChaining tests the builder pattern methods.
func TestTLSConfigChaining(t *testing.T) {
	cfg := NewTLSConfig().
		WithCertFile("/path/to/cert.pem", "/path/to/key.pem").
		WithMinVersion(TLSVersion12).
		WithMaxVersion(TLSVersion13).
		WithClientAuth(tls.RequireAndVerifyClientCert)

	if cfg.CertFile != "/path/to/cert.pem" {
		t.Errorf("expected CertFile /path/to/cert.pem, got %s", cfg.CertFile)
	}

	if cfg.KeyFile != "/path/to/key.pem" {
		t.Errorf("expected KeyFile /path/to/key.pem, got %s", cfg.KeyFile)
	}

	if cfg.MinVersion != TLSVersion12 {
		t.Errorf("expected MinVersion TLS 1.2, got 0x%04x", cfg.MinVersion)
	}

	if cfg.MaxVersion != TLSVersion13 {
		t.Errorf("expected MaxVersion TLS 1.3, got 0x%04x", cfg.MaxVersion)
	}

	if cfg.ClientAuth != tls.RequireAndVerifyClientCert {
		t.Errorf("expected ClientAuth RequireAndVerifyClientCert, got %v", cfg.ClientAuth)
	}
}

// TestLoadCertificateFromPEM tests loading certificates from PEM data.
func TestLoadCertificateFromPEM(t *testing.T) {
	certPEM, keyPEM, err := generateTestCertificate()
	if err != nil {
		t.Fatalf("failed to generate test certificate: %v", err)
	}

	tests := []struct {
		name    string
		certPEM []byte
		keyPEM  []byte
		wantErr error
	}{
		{
			name:    "valid certificate",
			certPEM: certPEM,
			keyPEM:  keyPEM,
			wantErr: nil,
		},
		{
			name:    "empty cert PEM",
			certPEM: nil,
			keyPEM:  keyPEM,
			wantErr: ErrNoCertificate,
		},
		{
			name:    "empty key PEM",
			certPEM: certPEM,
			keyPEM:  nil,
			wantErr: ErrNoPrivateKey,
		},
		{
			name:    "invalid cert PEM",
			certPEM: []byte("not a valid PEM"),
			keyPEM:  keyPEM,
			wantErr: ErrInvalidCertPEM,
		},
		{
			name:    "invalid key PEM",
			certPEM: certPEM,
			keyPEM:  []byte("not a valid PEM"),
			wantErr: ErrInvalidCertPEM, // Go's X509KeyPair reports cert error first
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cert, err := LoadCertificateFromPEM(tt.certPEM, tt.keyPEM)

			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("expected error %v, got nil", tt.wantErr)
					return
				}
				if err != tt.wantErr {
					t.Errorf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(cert.Certificate) == 0 {
				t.Error("expected certificate to have at least one cert")
			}
		})
	}
}

// TestLoadCertificate tests loading certificates from files.
func TestLoadCertificate(t *testing.T) {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "tls_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Generate test certificate
	certPEM, keyPEM, err := generateTestCertificate()
	if err != nil {
		t.Fatalf("failed to generate test certificate: %v", err)
	}

	// Write cert and key files
	certFile := filepath.Join(tempDir, "cert.pem")
	keyFile := filepath.Join(tempDir, "key.pem")

	if err := os.WriteFile(certFile, certPEM, 0600); err != nil {
		t.Fatalf("failed to write cert file: %v", err)
	}
	if err := os.WriteFile(keyFile, keyPEM, 0600); err != nil {
		t.Fatalf("failed to write key file: %v", err)
	}

	tests := []struct {
		name     string
		certFile string
		keyFile  string
		wantErr  error
	}{
		{
			name:     "valid files",
			certFile: certFile,
			keyFile:  keyFile,
			wantErr:  nil,
		},
		{
			name:     "empty cert path",
			certFile: "",
			keyFile:  keyFile,
			wantErr:  ErrNoCertificate,
		},
		{
			name:     "empty key path",
			certFile: certFile,
			keyFile:  "",
			wantErr:  ErrNoPrivateKey,
		},
		{
			name:     "cert file not found",
			certFile: "/nonexistent/cert.pem",
			keyFile:  keyFile,
			wantErr:  ErrCertFileNotFound,
		},
		{
			name:     "key file not found",
			certFile: certFile,
			keyFile:  "/nonexistent/key.pem",
			wantErr:  ErrKeyFileNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cert, err := LoadCertificate(tt.certFile, tt.keyFile)

			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("expected error %v, got nil", tt.wantErr)
					return
				}
				if err != tt.wantErr {
					t.Errorf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(cert.Certificate) == 0 {
				t.Error("expected certificate to have at least one cert")
			}
		})
	}
}

// TestLoadTLSConfig tests the full TLS config loading.
func TestLoadTLSConfig(t *testing.T) {
	certPEM, keyPEM, err := generateTestCertificate()
	if err != nil {
		t.Fatalf("failed to generate test certificate: %v", err)
	}

	caPEM, err := generateTestCA()
	if err != nil {
		t.Fatalf("failed to generate test CA: %v", err)
	}

	tests := []struct {
		name    string
		cfg     *TLSConfig
		wantErr error
	}{
		{
			name:    "nil config",
			cfg:     nil,
			wantErr: ErrNoCertificate,
		},
		{
			name: "valid PEM config",
			cfg: &TLSConfig{
				CertPEM:    certPEM,
				KeyPEM:     keyPEM,
				MinVersion: TLSVersion12,
				MaxVersion: TLSVersion13,
			},
			wantErr: nil,
		},
		{
			name: "valid config with client CA",
			cfg: &TLSConfig{
				CertPEM:     certPEM,
				KeyPEM:      keyPEM,
				MinVersion:  TLSVersion12,
				MaxVersion:  TLSVersion13,
				ClientAuth:  tls.RequireAndVerifyClientCert,
				ClientCAPEM: caPEM,
			},
			wantErr: nil,
		},
		{
			name: "invalid min version",
			cfg: &TLSConfig{
				CertPEM:    certPEM,
				KeyPEM:     keyPEM,
				MinVersion: 0x0999,
			},
			wantErr: ErrInvalidTLSVersion,
		},
		{
			name: "invalid max version",
			cfg: &TLSConfig{
				CertPEM:    certPEM,
				KeyPEM:     keyPEM,
				MaxVersion: 0x0999,
			},
			wantErr: ErrInvalidTLSVersion,
		},
		{
			name: "min version higher than max",
			cfg: &TLSConfig{
				CertPEM:    certPEM,
				KeyPEM:     keyPEM,
				MinVersion: TLSVersion13,
				MaxVersion: TLSVersion12,
			},
			wantErr: ErrMinVersionTooHigh,
		},
		{
			name: "no certificate",
			cfg: &TLSConfig{
				MinVersion: TLSVersion12,
				MaxVersion: TLSVersion13,
			},
			wantErr: ErrNoCertificate,
		},
		{
			name: "invalid client CA PEM",
			cfg: &TLSConfig{
				CertPEM:     certPEM,
				KeyPEM:      keyPEM,
				ClientCAPEM: []byte("invalid PEM"),
			},
			wantErr: ErrInvalidClientCAPEM,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tlsCfg, err := LoadTLSConfig(tt.cfg)

			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("expected error %v, got nil", tt.wantErr)
					return
				}
				// Check if error matches or wraps expected error
				if err != tt.wantErr && !containsError(err, tt.wantErr) {
					t.Errorf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tlsCfg == nil {
				t.Error("expected non-nil tls.Config")
				return
			}

			if len(tlsCfg.Certificates) == 0 {
				t.Error("expected at least one certificate")
			}
		})
	}
}

// TestLoadTLSConfigWithFiles tests loading TLS config from files.
func TestLoadTLSConfigWithFiles(t *testing.T) {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "tls_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Generate and write test certificate
	certPEM, keyPEM, err := generateTestCertificate()
	if err != nil {
		t.Fatalf("failed to generate test certificate: %v", err)
	}

	certFile := filepath.Join(tempDir, "cert.pem")
	keyFile := filepath.Join(tempDir, "key.pem")

	if err := os.WriteFile(certFile, certPEM, 0600); err != nil {
		t.Fatalf("failed to write cert file: %v", err)
	}
	if err := os.WriteFile(keyFile, keyPEM, 0600); err != nil {
		t.Fatalf("failed to write key file: %v", err)
	}

	cfg := NewTLSConfig().WithCertFile(certFile, keyFile)

	tlsCfg, err := LoadTLSConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tlsCfg == nil {
		t.Fatal("expected non-nil tls.Config")
	}

	if len(tlsCfg.Certificates) == 0 {
		t.Error("expected at least one certificate")
	}

	if tlsCfg.MinVersion != TLSVersion12 {
		t.Errorf("expected MinVersion TLS 1.2, got 0x%04x", tlsCfg.MinVersion)
	}

	if tlsCfg.MaxVersion != TLSVersion13 {
		t.Errorf("expected MaxVersion TLS 1.3, got 0x%04x", tlsCfg.MaxVersion)
	}
}

// TestTLSVersionValidation tests TLS version validation.
func TestTLSVersionValidation(t *testing.T) {
	tests := []struct {
		name       string
		minVersion uint16
		maxVersion uint16
		wantErr    bool
	}{
		{
			name:       "both zero (defaults)",
			minVersion: 0,
			maxVersion: 0,
			wantErr:    false,
		},
		{
			name:       "valid TLS 1.2 to 1.3",
			minVersion: TLSVersion12,
			maxVersion: TLSVersion13,
			wantErr:    false,
		},
		{
			name:       "valid TLS 1.0 to 1.3",
			minVersion: TLSVersion10,
			maxVersion: TLSVersion13,
			wantErr:    false,
		},
		{
			name:       "same min and max",
			minVersion: TLSVersion12,
			maxVersion: TLSVersion12,
			wantErr:    false,
		},
		{
			name:       "min higher than max",
			minVersion: TLSVersion13,
			maxVersion: TLSVersion12,
			wantErr:    true,
		},
		{
			name:       "invalid min version",
			minVersion: 0x0999,
			maxVersion: TLSVersion13,
			wantErr:    true,
		},
		{
			name:       "invalid max version",
			minVersion: TLSVersion12,
			maxVersion: 0x0999,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTLSVersions(tt.minVersion, tt.maxVersion)
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestCipherSuiteValidation tests cipher suite validation.
func TestCipherSuiteValidation(t *testing.T) {
	tests := []struct {
		name    string
		suites  []uint16
		wantErr bool
	}{
		{
			name:    "empty suites",
			suites:  []uint16{},
			wantErr: false,
		},
		{
			name:    "default suites",
			suites:  defaultCipherSuites,
			wantErr: false,
		},
		{
			name: "valid single suite",
			suites: []uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			},
			wantErr: false,
		},
		{
			name: "invalid suite",
			suites: []uint16{
				0xFFFF,
			},
			wantErr: true,
		},
		{
			name: "mixed valid and invalid",
			suites: []uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				0xFFFF,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCipherSuites(tt.suites)
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestGetDefaultCipherSuites tests getting default cipher suites.
func TestGetDefaultCipherSuites(t *testing.T) {
	suites := GetDefaultCipherSuites()

	if len(suites) == 0 {
		t.Error("expected non-empty cipher suites")
	}

	if len(suites) != len(defaultCipherSuites) {
		t.Errorf("expected %d suites, got %d", len(defaultCipherSuites), len(suites))
	}

	// Verify it's a copy
	suites[0] = 0xFFFF
	if defaultCipherSuites[0] == 0xFFFF {
		t.Error("GetDefaultCipherSuites should return a copy")
	}
}

// TestTLSVersionString tests TLS version string conversion.
func TestTLSVersionString(t *testing.T) {
	tests := []struct {
		version uint16
		want    string
	}{
		{TLSVersion10, "TLS 1.0"},
		{TLSVersion11, "TLS 1.1"},
		{TLSVersion12, "TLS 1.2"},
		{TLSVersion13, "TLS 1.3"},
		{0x0999, "unknown (0x0999)"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := TLSVersionString(tt.version)
			if got != tt.want {
				t.Errorf("TLSVersionString(0x%04x) = %s, want %s", tt.version, got, tt.want)
			}
		})
	}
}

// TestCipherSuiteName tests cipher suite name lookup.
func TestCipherSuiteName(t *testing.T) {
	// Test known cipher suite
	name := CipherSuiteName(tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256)
	if name == "" || name[:7] == "unknown" {
		t.Errorf("expected known cipher suite name, got %s", name)
	}

	// Test unknown cipher suite
	name = CipherSuiteName(0xFFFF)
	if name != "unknown (0xffff)" {
		t.Errorf("expected unknown cipher suite, got %s", name)
	}
}

// TestIsValidTLSVersion tests TLS version validation.
func TestIsValidTLSVersion(t *testing.T) {
	validVersions := []uint16{TLSVersion10, TLSVersion11, TLSVersion12, TLSVersion13}
	for _, v := range validVersions {
		if !isValidTLSVersion(v) {
			t.Errorf("expected version 0x%04x to be valid", v)
		}
	}

	invalidVersions := []uint16{0x0000, 0x0999, 0xFFFF}
	for _, v := range invalidVersions {
		if isValidTLSVersion(v) {
			t.Errorf("expected version 0x%04x to be invalid", v)
		}
	}
}

// TestTLSConfigWithCipherSuites tests custom cipher suite configuration.
func TestTLSConfigWithCipherSuites(t *testing.T) {
	certPEM, keyPEM, err := generateTestCertificate()
	if err != nil {
		t.Fatalf("failed to generate test certificate: %v", err)
	}

	customSuites := []uint16{
		tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	}

	cfg := NewTLSConfig().
		WithCertPEM(certPEM, keyPEM).
		WithCipherSuites(customSuites)

	tlsCfg, err := LoadTLSConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tlsCfg.CipherSuites) != len(customSuites) {
		t.Errorf("expected %d cipher suites, got %d", len(customSuites), len(tlsCfg.CipherSuites))
	}

	for i, suite := range tlsCfg.CipherSuites {
		if suite != customSuites[i] {
			t.Errorf("cipher suite %d: expected 0x%04x, got 0x%04x", i, customSuites[i], suite)
		}
	}
}

// TestTLSConfigWithClientCAs tests client CA configuration.
func TestTLSConfigWithClientCAs(t *testing.T) {
	certPEM, keyPEM, err := generateTestCertificate()
	if err != nil {
		t.Fatalf("failed to generate test certificate: %v", err)
	}

	caPEM, err := generateTestCA()
	if err != nil {
		t.Fatalf("failed to generate test CA: %v", err)
	}

	// Test with pre-built pool
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(caPEM)

	cfg := NewTLSConfig().
		WithCertPEM(certPEM, keyPEM).
		WithClientAuth(tls.RequireAndVerifyClientCert).
		WithClientCAs(pool)

	tlsCfg, err := LoadTLSConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tlsCfg.ClientCAs == nil {
		t.Error("expected ClientCAs to be set")
	}

	if tlsCfg.ClientAuth != tls.RequireAndVerifyClientCert {
		t.Errorf("expected ClientAuth RequireAndVerifyClientCert, got %v", tlsCfg.ClientAuth)
	}
}

// TestTLSConfigPEMPreference tests that PEM data is preferred over files.
func TestTLSConfigPEMPreference(t *testing.T) {
	certPEM, keyPEM, err := generateTestCertificate()
	if err != nil {
		t.Fatalf("failed to generate test certificate: %v", err)
	}

	// Config with both PEM and file paths (PEM should be used)
	cfg := &TLSConfig{
		CertFile:   "/nonexistent/cert.pem",
		KeyFile:    "/nonexistent/key.pem",
		CertPEM:    certPEM,
		KeyPEM:     keyPEM,
		MinVersion: TLSVersion12,
		MaxVersion: TLSVersion13,
	}

	// Should succeed because PEM is preferred
	tlsCfg, err := LoadTLSConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tlsCfg == nil {
		t.Error("expected non-nil tls.Config")
	}
}

// TestCertKeyMismatch tests detection of certificate/key mismatch.
func TestCertKeyMismatch(t *testing.T) {
	// Generate two different certificates
	certPEM1, _, err := generateTestCertificate()
	if err != nil {
		t.Fatalf("failed to generate first certificate: %v", err)
	}

	_, keyPEM2, err := generateTestCertificate()
	if err != nil {
		t.Fatalf("failed to generate second certificate: %v", err)
	}

	// Try to load with mismatched cert and key
	_, err = LoadCertificateFromPEM(certPEM1, keyPEM2)
	if err == nil {
		t.Error("expected error for mismatched cert/key")
	}
}

// TestTLSVersionConstants tests that version constants match crypto/tls.
func TestTLSVersionConstants(t *testing.T) {
	if TLSVersion10 != tls.VersionTLS10 {
		t.Errorf("TLSVersion10 mismatch: got 0x%04x, want 0x%04x", TLSVersion10, tls.VersionTLS10)
	}
	if TLSVersion11 != tls.VersionTLS11 {
		t.Errorf("TLSVersion11 mismatch: got 0x%04x, want 0x%04x", TLSVersion11, tls.VersionTLS11)
	}
	if TLSVersion12 != tls.VersionTLS12 {
		t.Errorf("TLSVersion12 mismatch: got 0x%04x, want 0x%04x", TLSVersion12, tls.VersionTLS12)
	}
	if TLSVersion13 != tls.VersionTLS13 {
		t.Errorf("TLSVersion13 mismatch: got 0x%04x, want 0x%04x", TLSVersion13, tls.VersionTLS13)
	}
}

// TestDefaultCipherSuitesAreSecure tests that default cipher suites are secure.
func TestDefaultCipherSuitesAreSecure(t *testing.T) {
	insecureSuites := make(map[uint16]bool)
	for _, suite := range tls.InsecureCipherSuites() {
		insecureSuites[suite.ID] = true
	}

	for _, suite := range defaultCipherSuites {
		if insecureSuites[suite] {
			t.Errorf("default cipher suite 0x%04x is insecure", suite)
		}
	}
}

// containsError checks if err contains or equals target.
func containsError(err, target error) bool {
	if err == target {
		return true
	}
	if err == nil || target == nil {
		return false
	}
	return contains(err.Error(), target.Error())
}
