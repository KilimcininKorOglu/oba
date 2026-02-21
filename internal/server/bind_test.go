// Package server provides the LDAP server implementation.
package server

import (
	"testing"

	"github.com/KilimcininKorOglu/oba/internal/ldap"
	"github.com/KilimcininKorOglu/oba/internal/storage"
)

// mockBackend implements the Backend interface for testing.
type mockBackend struct {
	entries map[string]*storage.Entry
	err     error
}

func newMockBackend() *mockBackend {
	return &mockBackend{
		entries: make(map[string]*storage.Entry),
	}
}

func (m *mockBackend) GetEntry(dn string) (*storage.Entry, error) {
	if m.err != nil {
		return nil, m.err
	}
	normalizedDN := normalizeDN(dn)
	for storedDN, entry := range m.entries {
		if normalizeDN(storedDN) == normalizedDN {
			return entry, nil
		}
	}
	return nil, nil
}

func (m *mockBackend) addEntry(entry *storage.Entry) {
	m.entries[entry.DN] = entry
}

// TestBindHandler_AnonymousBind tests anonymous bind functionality.
func TestBindHandler_AnonymousBind(t *testing.T) {
	tests := []struct {
		name           string
		allowAnonymous bool
		wantResultCode ldap.ResultCode
	}{
		{
			name:           "anonymous bind allowed",
			allowAnonymous: true,
			wantResultCode: ldap.ResultSuccess,
		},
		{
			name:           "anonymous bind not allowed",
			allowAnonymous: false,
			wantResultCode: ldap.ResultInappropriateAuthentication,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &BindConfig{
				AllowAnonymous: tt.allowAnonymous,
			}
			handler := NewBindHandler(config)

			req := &ldap.BindRequest{
				Version:        LDAPVersion3,
				Name:           "",
				AuthMethod:     ldap.AuthMethodSimple,
				SimplePassword: []byte{},
			}

			result := handler.Handle(nil, req)

			if result.ResultCode != tt.wantResultCode {
				t.Errorf("Handle() ResultCode = %v, want %v", result.ResultCode, tt.wantResultCode)
			}
		})
	}
}

// TestBindHandler_VersionValidation tests LDAP version validation.
func TestBindHandler_VersionValidation(t *testing.T) {
	tests := []struct {
		name           string
		version        int
		wantResultCode ldap.ResultCode
	}{
		{
			name:           "version 3 accepted",
			version:        3,
			wantResultCode: ldap.ResultSuccess,
		},
		{
			name:           "version 2 rejected",
			version:        2,
			wantResultCode: ldap.ResultProtocolError,
		},
		{
			name:           "version 1 rejected",
			version:        1,
			wantResultCode: ldap.ResultProtocolError,
		},
		{
			name:           "version 4 rejected",
			version:        4,
			wantResultCode: ldap.ResultProtocolError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &BindConfig{
				AllowAnonymous: true,
			}
			handler := NewBindHandler(config)

			req := &ldap.BindRequest{
				Version:        tt.version,
				Name:           "",
				AuthMethod:     ldap.AuthMethodSimple,
				SimplePassword: []byte{},
			}

			result := handler.Handle(nil, req)

			if result.ResultCode != tt.wantResultCode {
				t.Errorf("Handle() ResultCode = %v, want %v", result.ResultCode, tt.wantResultCode)
			}
		})
	}
}

// TestBindHandler_SimpleBind tests simple bind with DN and password.
func TestBindHandler_SimpleBind(t *testing.T) {
	backend := newMockBackend()

	// Create a test entry with a cleartext password
	entry := storage.NewEntry("uid=alice,ou=users,dc=example,dc=com")
	entry.SetStringAttribute(PasswordAttribute, "{CLEARTEXT}secret123")
	backend.addEntry(entry)

	// Create another entry with SHA256 password
	hashedPassword, _ := HashPassword("password456", SchemeSHA256)
	entry2 := storage.NewEntry("uid=bob,ou=users,dc=example,dc=com")
	entry2.SetStringAttribute(PasswordAttribute, hashedPassword)
	backend.addEntry(entry2)

	config := &BindConfig{
		Backend:        backend,
		AllowAnonymous: true,
	}
	handler := NewBindHandler(config)

	tests := []struct {
		name           string
		dn             string
		password       string
		wantResultCode ldap.ResultCode
	}{
		{
			name:           "valid credentials cleartext",
			dn:             "uid=alice,ou=users,dc=example,dc=com",
			password:       "secret123",
			wantResultCode: ldap.ResultSuccess,
		},
		{
			name:           "valid credentials SHA256",
			dn:             "uid=bob,ou=users,dc=example,dc=com",
			password:       "password456",
			wantResultCode: ldap.ResultSuccess,
		},
		{
			name:           "invalid password",
			dn:             "uid=alice,ou=users,dc=example,dc=com",
			password:       "wrongpassword",
			wantResultCode: ldap.ResultInvalidCredentials,
		},
		{
			name:           "non-existent user",
			dn:             "uid=nonexistent,ou=users,dc=example,dc=com",
			password:       "anypassword",
			wantResultCode: ldap.ResultInvalidCredentials,
		},
		{
			name:           "case insensitive DN",
			dn:             "UID=ALICE,OU=USERS,DC=EXAMPLE,DC=COM",
			password:       "secret123",
			wantResultCode: ldap.ResultSuccess,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &ldap.BindRequest{
				Version:        LDAPVersion3,
				Name:           tt.dn,
				AuthMethod:     ldap.AuthMethodSimple,
				SimplePassword: []byte(tt.password),
			}

			result := handler.Handle(nil, req)

			if result.ResultCode != tt.wantResultCode {
				t.Errorf("Handle() ResultCode = %v, want %v", result.ResultCode, tt.wantResultCode)
			}
		})
	}
}

// TestBindHandler_RootDNBind tests binding as the root DN.
func TestBindHandler_RootDNBind(t *testing.T) {
	rootPassword, _ := HashPassword("adminpass", SchemeSHA256)

	config := &BindConfig{
		Backend:        newMockBackend(),
		AllowAnonymous: true,
		RootDN:         "cn=admin,dc=example,dc=com",
		RootPassword:   rootPassword,
	}
	handler := NewBindHandler(config)

	tests := []struct {
		name           string
		dn             string
		password       string
		wantResultCode ldap.ResultCode
	}{
		{
			name:           "valid root credentials",
			dn:             "cn=admin,dc=example,dc=com",
			password:       "adminpass",
			wantResultCode: ldap.ResultSuccess,
		},
		{
			name:           "invalid root password",
			dn:             "cn=admin,dc=example,dc=com",
			password:       "wrongpass",
			wantResultCode: ldap.ResultInvalidCredentials,
		},
		{
			name:           "root DN case insensitive",
			dn:             "CN=ADMIN,DC=EXAMPLE,DC=COM",
			password:       "adminpass",
			wantResultCode: ldap.ResultSuccess,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &ldap.BindRequest{
				Version:        LDAPVersion3,
				Name:           tt.dn,
				AuthMethod:     ldap.AuthMethodSimple,
				SimplePassword: []byte(tt.password),
			}

			result := handler.Handle(nil, req)

			if result.ResultCode != tt.wantResultCode {
				t.Errorf("Handle() ResultCode = %v, want %v", result.ResultCode, tt.wantResultCode)
			}
		})
	}
}

// TestBindHandler_SASLNotSupported tests that SASL authentication returns appropriate error.
func TestBindHandler_SASLNotSupported(t *testing.T) {
	config := &BindConfig{
		AllowAnonymous: true,
	}
	handler := NewBindHandler(config)

	req := &ldap.BindRequest{
		Version:    LDAPVersion3,
		Name:       "uid=alice,ou=users,dc=example,dc=com",
		AuthMethod: ldap.AuthMethodSASL,
		SASLCredentials: &ldap.SASLCredentials{
			Mechanism:   "PLAIN",
			Credentials: []byte("credentials"),
		},
	}

	result := handler.Handle(nil, req)

	if result.ResultCode != ldap.ResultAuthMethodNotSupported {
		t.Errorf("Handle() ResultCode = %v, want %v", result.ResultCode, ldap.ResultAuthMethodNotSupported)
	}
}

// TestBindHandler_NoBackend tests behavior when backend is not configured.
func TestBindHandler_NoBackend(t *testing.T) {
	config := &BindConfig{
		Backend:        nil,
		AllowAnonymous: true,
	}
	handler := NewBindHandler(config)

	req := &ldap.BindRequest{
		Version:        LDAPVersion3,
		Name:           "uid=alice,ou=users,dc=example,dc=com",
		AuthMethod:     ldap.AuthMethodSimple,
		SimplePassword: []byte("password"),
	}

	result := handler.Handle(nil, req)

	if result.ResultCode != ldap.ResultOperationsError {
		t.Errorf("Handle() ResultCode = %v, want %v", result.ResultCode, ldap.ResultOperationsError)
	}
}

// TestBindHandler_EntryWithoutPassword tests binding to entry without password attribute.
func TestBindHandler_EntryWithoutPassword(t *testing.T) {
	backend := newMockBackend()

	// Create an entry without a password
	entry := storage.NewEntry("uid=nopass,ou=users,dc=example,dc=com")
	entry.SetStringAttribute("cn", "No Password User")
	backend.addEntry(entry)

	config := &BindConfig{
		Backend:        backend,
		AllowAnonymous: true,
	}
	handler := NewBindHandler(config)

	req := &ldap.BindRequest{
		Version:        LDAPVersion3,
		Name:           "uid=nopass,ou=users,dc=example,dc=com",
		AuthMethod:     ldap.AuthMethodSimple,
		SimplePassword: []byte("anypassword"),
	}

	result := handler.Handle(nil, req)

	if result.ResultCode != ldap.ResultInvalidCredentials {
		t.Errorf("Handle() ResultCode = %v, want %v", result.ResultCode, ldap.ResultInvalidCredentials)
	}
}

// TestBindHandler_MultiplePasswords tests entry with multiple password values.
func TestBindHandler_MultiplePasswords(t *testing.T) {
	backend := newMockBackend()

	// Create an entry with multiple passwords
	entry := storage.NewEntry("uid=multipass,ou=users,dc=example,dc=com")
	pass1, _ := HashPassword("password1", SchemeSHA256)
	pass2, _ := HashPassword("password2", SchemeSHA256)
	entry.SetAttribute(PasswordAttribute, [][]byte{[]byte(pass1), []byte(pass2)})
	backend.addEntry(entry)

	config := &BindConfig{
		Backend:        backend,
		AllowAnonymous: true,
	}
	handler := NewBindHandler(config)

	tests := []struct {
		name           string
		password       string
		wantResultCode ldap.ResultCode
	}{
		{
			name:           "first password",
			password:       "password1",
			wantResultCode: ldap.ResultSuccess,
		},
		{
			name:           "second password",
			password:       "password2",
			wantResultCode: ldap.ResultSuccess,
		},
		{
			name:           "wrong password",
			password:       "wrongpassword",
			wantResultCode: ldap.ResultInvalidCredentials,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &ldap.BindRequest{
				Version:        LDAPVersion3,
				Name:           "uid=multipass,ou=users,dc=example,dc=com",
				AuthMethod:     ldap.AuthMethodSimple,
				SimplePassword: []byte(tt.password),
			}

			result := handler.Handle(nil, req)

			if result.ResultCode != tt.wantResultCode {
				t.Errorf("Handle() ResultCode = %v, want %v", result.ResultCode, tt.wantResultCode)
			}
		})
	}
}

// TestCreateBindHandler tests the CreateBindHandler function.
func TestCreateBindHandler(t *testing.T) {
	config := &BindConfig{
		AllowAnonymous: true,
	}
	impl := NewBindHandler(config)
	handler := CreateBindHandler(impl)

	req := &ldap.BindRequest{
		Version:        LDAPVersion3,
		Name:           "",
		AuthMethod:     ldap.AuthMethodSimple,
		SimplePassword: []byte{},
	}

	result := handler(nil, req)

	if result.ResultCode != ldap.ResultSuccess {
		t.Errorf("handler() ResultCode = %v, want %v", result.ResultCode, ldap.ResultSuccess)
	}
}

// TestNewBindConfig tests the NewBindConfig function.
func TestNewBindConfig(t *testing.T) {
	config := NewBindConfig()

	if config == nil {
		t.Fatal("NewBindConfig() returned nil")
	}

	if !config.AllowAnonymous {
		t.Error("NewBindConfig() AllowAnonymous should be true by default")
	}

	if config.Backend != nil {
		t.Error("NewBindConfig() Backend should be nil by default")
	}

	if config.RootDN != "" {
		t.Error("NewBindConfig() RootDN should be empty by default")
	}

	if config.RootPassword != "" {
		t.Error("NewBindConfig() RootPassword should be empty by default")
	}
}

// TestNewBindHandler_NilConfig tests NewBindHandler with nil config.
func TestNewBindHandler_NilConfig(t *testing.T) {
	handler := NewBindHandler(nil)

	if handler == nil {
		t.Fatal("NewBindHandler(nil) returned nil")
	}

	if handler.config == nil {
		t.Fatal("NewBindHandler(nil) config is nil")
	}

	// Should use default config with anonymous allowed
	req := &ldap.BindRequest{
		Version:        LDAPVersion3,
		Name:           "",
		AuthMethod:     ldap.AuthMethodSimple,
		SimplePassword: []byte{},
	}

	result := handler.Handle(nil, req)

	if result.ResultCode != ldap.ResultSuccess {
		t.Errorf("Handle() ResultCode = %v, want %v", result.ResultCode, ldap.ResultSuccess)
	}
}

// TestNormalizeDN tests the normalizeDN function.
func TestNormalizeDN(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"uid=alice,dc=example,dc=com", "uid=alice,dc=example,dc=com"},
		{"UID=ALICE,DC=EXAMPLE,DC=COM", "uid=alice,dc=example,dc=com"},
		{"  uid=alice,dc=example,dc=com  ", "uid=alice,dc=example,dc=com"},
		{"", ""},
		{"   ", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeDN(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeDN(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
