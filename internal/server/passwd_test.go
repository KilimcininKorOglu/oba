package server

import (
	"errors"
	"net"
	"testing"

	"github.com/KilimcininKorOglu/oba/internal/ber"
	"github.com/KilimcininKorOglu/oba/internal/ldap"
	"github.com/KilimcininKorOglu/oba/internal/password"
)

// mockPasswordBackend is a mock implementation of PasswordBackend for testing.
type mockPasswordBackend struct {
	entries   map[string]*Entry
	passwords map[string]string
}

func newMockPasswordBackend() *mockPasswordBackend {
	return &mockPasswordBackend{
		entries:   make(map[string]*Entry),
		passwords: make(map[string]string),
	}
}

func (m *mockPasswordBackend) GetEntry(dn string) (*Entry, error) {
	normalizedDN := normalizeDNForCompare(dn)
	entry, ok := m.entries[normalizedDN]
	if !ok {
		return nil, nil
	}
	return entry, nil
}

func (m *mockPasswordBackend) SetPassword(dn string, password []byte) error {
	normalizedDN := normalizeDNForCompare(dn)
	m.passwords[normalizedDN] = string(password)
	return nil
}

func (m *mockPasswordBackend) VerifyPassword(dn string, password string) error {
	normalizedDN := normalizeDNForCompare(dn)
	storedPassword, ok := m.passwords[normalizedDN]
	if !ok {
		return errors.New("user not found")
	}
	if storedPassword != password {
		return errors.New("invalid password")
	}
	return nil
}

func (m *mockPasswordBackend) addUser(dn, password string) {
	normalizedDN := normalizeDNForCompare(dn)
	m.entries[normalizedDN] = &Entry{DN: dn, Password: password}
	m.passwords[normalizedDN] = password
}

// TestPasswordModifyOID tests that the OID constant is correct.
func TestPasswordModifyOID(t *testing.T) {
	expectedOID := "1.3.6.1.4.1.4203.1.11.1"
	if PasswordModifyOID != expectedOID {
		t.Errorf("PasswordModifyOID = %q, want %q", PasswordModifyOID, expectedOID)
	}
}

// TestNewPasswordModifyHandler tests creating a new handler.
func TestNewPasswordModifyHandler(t *testing.T) {
	handler := NewPasswordModifyHandler(nil)
	if handler == nil {
		t.Fatal("NewPasswordModifyHandler returned nil")
	}
}

// TestPasswordModifyHandler_OID tests that the handler returns the correct OID.
func TestPasswordModifyHandler_OID(t *testing.T) {
	handler := NewPasswordModifyHandler(nil)
	if handler.OID() != PasswordModifyOID {
		t.Errorf("OID() = %q, want %q", handler.OID(), PasswordModifyOID)
	}
}

// TestPasswordModifyHandler_Handle_Anonymous tests that anonymous users cannot modify passwords.
func TestPasswordModifyHandler_Handle_Anonymous(t *testing.T) {
	handler := NewPasswordModifyHandler(nil)

	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	conn := NewConnection(server, nil)
	// Connection is anonymous by default (bindDN is empty)

	req := &ExtendedRequest{
		OID:   PasswordModifyOID,
		Value: nil,
	}

	resp, err := handler.Handle(conn, req)
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if resp == nil {
		t.Fatal("Handle returned nil response")
	}

	// Anonymous users should not be able to modify passwords
	if resp.Result.ResultCode != ldap.ResultUnwillingToPerform {
		t.Errorf("ResultCode = %v, want %v", resp.Result.ResultCode, ldap.ResultUnwillingToPerform)
	}
}

// TestPasswordModifyHandler_Handle_SelfChange tests a user changing their own password.
func TestPasswordModifyHandler_Handle_SelfChange(t *testing.T) {
	backend := newMockPasswordBackend()
	backend.addUser("uid=alice,ou=users,dc=example,dc=com", "oldpassword")

	config := NewPasswordModifyConfig()
	config.Backend = backend
	config.PolicyManager = password.NewManager(password.DisabledPolicy())

	handler := NewPasswordModifyHandler(config)

	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	conn := NewConnection(server, nil)
	conn.mu.Lock()
	conn.bindDN = "uid=alice,ou=users,dc=example,dc=com"
	conn.authenticated = true
	conn.mu.Unlock()

	// Create request with old and new password
	reqValue := encodePasswordModifyRequestForTest(nil, []byte("oldpassword"), []byte("newpassword123"))

	req := &ExtendedRequest{
		OID:   PasswordModifyOID,
		Value: reqValue,
	}

	resp, err := handler.Handle(conn, req)
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if resp.Result.ResultCode != ldap.ResultSuccess {
		t.Errorf("ResultCode = %v, want %v, message: %s",
			resp.Result.ResultCode, ldap.ResultSuccess, resp.Result.DiagnosticMessage)
	}

	// Verify password was changed
	normalizedDN := normalizeDNForCompare("uid=alice,ou=users,dc=example,dc=com")
	if backend.passwords[normalizedDN] != "newpassword123" {
		t.Errorf("Password was not updated correctly")
	}
}

// TestPasswordModifyHandler_Handle_SelfChangeWithoutOldPassword tests that non-admin users must provide old password.
func TestPasswordModifyHandler_Handle_SelfChangeWithoutOldPassword(t *testing.T) {
	backend := newMockPasswordBackend()
	backend.addUser("uid=alice,ou=users,dc=example,dc=com", "oldpassword")

	config := NewPasswordModifyConfig()
	config.Backend = backend
	config.PolicyManager = password.NewManager(password.DisabledPolicy())

	handler := NewPasswordModifyHandler(config)

	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	conn := NewConnection(server, nil)
	conn.mu.Lock()
	conn.bindDN = "uid=alice,ou=users,dc=example,dc=com"
	conn.authenticated = true
	conn.mu.Unlock()

	// Create request without old password
	reqValue := encodePasswordModifyRequestForTest(nil, nil, []byte("newpassword123"))

	req := &ExtendedRequest{
		OID:   PasswordModifyOID,
		Value: reqValue,
	}

	resp, err := handler.Handle(conn, req)
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	// Should fail because old password is required for self-service change
	if resp.Result.ResultCode != ldap.ResultUnwillingToPerform {
		t.Errorf("ResultCode = %v, want %v", resp.Result.ResultCode, ldap.ResultUnwillingToPerform)
	}
}

// TestPasswordModifyHandler_Handle_AdminReset tests an admin resetting another user's password.
func TestPasswordModifyHandler_Handle_AdminReset(t *testing.T) {
	backend := newMockPasswordBackend()
	backend.addUser("uid=alice,ou=users,dc=example,dc=com", "oldpassword")
	backend.addUser("cn=admin,dc=example,dc=com", "adminpassword")

	config := NewPasswordModifyConfig()
	config.Backend = backend
	config.PolicyManager = password.NewManager(password.DisabledPolicy())
	config.AdminDNs = []string{"cn=admin,dc=example,dc=com"}

	handler := NewPasswordModifyHandler(config)

	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	conn := NewConnection(server, nil)
	conn.mu.Lock()
	conn.bindDN = "cn=admin,dc=example,dc=com"
	conn.authenticated = true
	conn.mu.Unlock()

	// Admin resets alice's password without providing old password
	reqValue := encodePasswordModifyRequestForTest(
		[]byte("uid=alice,ou=users,dc=example,dc=com"),
		nil,
		[]byte("newpassword456"),
	)

	req := &ExtendedRequest{
		OID:   PasswordModifyOID,
		Value: reqValue,
	}

	resp, err := handler.Handle(conn, req)
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if resp.Result.ResultCode != ldap.ResultSuccess {
		t.Errorf("ResultCode = %v, want %v, message: %s",
			resp.Result.ResultCode, ldap.ResultSuccess, resp.Result.DiagnosticMessage)
	}

	// Verify password was changed
	normalizedDN := normalizeDNForCompare("uid=alice,ou=users,dc=example,dc=com")
	if backend.passwords[normalizedDN] != "newpassword456" {
		t.Errorf("Password was not updated correctly")
	}
}

// TestPasswordModifyHandler_Handle_NonAdminCannotResetOthers tests that non-admin users cannot reset other users' passwords.
func TestPasswordModifyHandler_Handle_NonAdminCannotResetOthers(t *testing.T) {
	backend := newMockPasswordBackend()
	backend.addUser("uid=alice,ou=users,dc=example,dc=com", "alicepassword")
	backend.addUser("uid=bob,ou=users,dc=example,dc=com", "bobpassword")

	config := NewPasswordModifyConfig()
	config.Backend = backend
	config.PolicyManager = password.NewManager(password.DisabledPolicy())

	handler := NewPasswordModifyHandler(config)

	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	conn := NewConnection(server, nil)
	conn.mu.Lock()
	conn.bindDN = "uid=alice,ou=users,dc=example,dc=com"
	conn.authenticated = true
	conn.mu.Unlock()

	// Alice tries to reset Bob's password
	reqValue := encodePasswordModifyRequestForTest(
		[]byte("uid=bob,ou=users,dc=example,dc=com"),
		nil,
		[]byte("hackedpassword"),
	)

	req := &ExtendedRequest{
		OID:   PasswordModifyOID,
		Value: reqValue,
	}

	resp, err := handler.Handle(conn, req)
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	// Should fail with insufficient access
	if resp.Result.ResultCode != ldap.ResultInsufficientAccessRights {
		t.Errorf("ResultCode = %v, want %v", resp.Result.ResultCode, ldap.ResultInsufficientAccessRights)
	}
}

// TestPasswordModifyHandler_Handle_WrongOldPassword tests that wrong old password is rejected.
func TestPasswordModifyHandler_Handle_WrongOldPassword(t *testing.T) {
	backend := newMockPasswordBackend()
	backend.addUser("uid=alice,ou=users,dc=example,dc=com", "correctpassword")

	config := NewPasswordModifyConfig()
	config.Backend = backend
	config.PolicyManager = password.NewManager(password.DisabledPolicy())

	handler := NewPasswordModifyHandler(config)

	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	conn := NewConnection(server, nil)
	conn.mu.Lock()
	conn.bindDN = "uid=alice,ou=users,dc=example,dc=com"
	conn.authenticated = true
	conn.mu.Unlock()

	// Provide wrong old password
	reqValue := encodePasswordModifyRequestForTest(nil, []byte("wrongpassword"), []byte("newpassword"))

	req := &ExtendedRequest{
		OID:   PasswordModifyOID,
		Value: reqValue,
	}

	resp, err := handler.Handle(conn, req)
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	// Should fail with invalid credentials
	if resp.Result.ResultCode != ldap.ResultInvalidCredentials {
		t.Errorf("ResultCode = %v, want %v", resp.Result.ResultCode, ldap.ResultInvalidCredentials)
	}
}

// TestPasswordModifyHandler_Handle_GeneratePassword tests server-generated password.
func TestPasswordModifyHandler_Handle_GeneratePassword(t *testing.T) {
	backend := newMockPasswordBackend()
	backend.addUser("uid=alice,ou=users,dc=example,dc=com", "oldpassword")

	config := NewPasswordModifyConfig()
	config.Backend = backend
	config.PolicyManager = password.NewManager(password.DisabledPolicy())
	config.AdminDNs = []string{"cn=admin,dc=example,dc=com"}

	handler := NewPasswordModifyHandler(config)

	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	conn := NewConnection(server, nil)
	conn.mu.Lock()
	conn.bindDN = "cn=admin,dc=example,dc=com"
	conn.authenticated = true
	conn.mu.Unlock()

	// Admin resets password without providing new password (server generates)
	reqValue := encodePasswordModifyRequestForTest(
		[]byte("uid=alice,ou=users,dc=example,dc=com"),
		nil,
		nil,
	)

	req := &ExtendedRequest{
		OID:   PasswordModifyOID,
		Value: reqValue,
	}

	resp, err := handler.Handle(conn, req)
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if resp.Result.ResultCode != ldap.ResultSuccess {
		t.Errorf("ResultCode = %v, want %v, message: %s",
			resp.Result.ResultCode, ldap.ResultSuccess, resp.Result.DiagnosticMessage)
	}

	// Response should contain generated password
	if len(resp.Value) == 0 {
		t.Error("Expected generated password in response")
	}
}

// TestPasswordModifyHandler_Handle_PolicyValidation tests password policy validation.
func TestPasswordModifyHandler_Handle_PolicyValidation(t *testing.T) {
	backend := newMockPasswordBackend()
	backend.addUser("uid=alice,ou=users,dc=example,dc=com", "oldpassword")

	// Create a strict policy
	policy := &password.Policy{
		Enabled:          true,
		MinLength:        12,
		RequireUppercase: true,
		RequireLowercase: true,
		RequireDigit:     true,
	}

	config := NewPasswordModifyConfig()
	config.Backend = backend
	config.PolicyManager = password.NewManager(policy)

	handler := NewPasswordModifyHandler(config)

	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	conn := NewConnection(server, nil)
	conn.mu.Lock()
	conn.bindDN = "uid=alice,ou=users,dc=example,dc=com"
	conn.authenticated = true
	conn.mu.Unlock()

	// Try to set a password that doesn't meet policy (too short)
	reqValue := encodePasswordModifyRequestForTest(nil, []byte("oldpassword"), []byte("short"))

	req := &ExtendedRequest{
		OID:   PasswordModifyOID,
		Value: reqValue,
	}

	resp, err := handler.Handle(conn, req)
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	// Should fail with constraint violation
	if resp.Result.ResultCode != ldap.ResultConstraintViolation {
		t.Errorf("ResultCode = %v, want %v", resp.Result.ResultCode, ldap.ResultConstraintViolation)
	}
}

// TestPasswordModifyHandler_RegisterWithDispatcher tests registering the handler with a dispatcher.
func TestPasswordModifyHandler_RegisterWithDispatcher(t *testing.T) {
	dispatcher := NewExtendedDispatcher()
	handler := NewPasswordModifyHandler(nil)

	err := dispatcher.Register(handler)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if !dispatcher.HasHandler(PasswordModifyOID) {
		t.Error("expected handler to be registered for PasswordModifyOID")
	}
}

// TestPasswordModifyHandler_ImplementsInterface verifies the handler implements ExtendedHandler.
func TestPasswordModifyHandler_ImplementsInterface(t *testing.T) {
	var _ ExtendedHandler = (*PasswordModifyHandler)(nil)
	var _ ExtendedHandler = NewPasswordModifyHandler(nil)
}

// TestParsePasswordModifyRequest_Empty tests parsing an empty request.
func TestParsePasswordModifyRequest_Empty(t *testing.T) {
	req, err := parsePasswordModifyRequest(nil)
	if err != nil {
		t.Fatalf("parsePasswordModifyRequest returned error: %v", err)
	}

	if len(req.UserIdentity) != 0 {
		t.Errorf("UserIdentity = %v, want empty", req.UserIdentity)
	}
	if len(req.OldPassword) != 0 {
		t.Errorf("OldPassword = %v, want empty", req.OldPassword)
	}
	if len(req.NewPassword) != 0 {
		t.Errorf("NewPassword = %v, want empty", req.NewPassword)
	}
}

// TestParsePasswordModifyRequest_AllFields tests parsing a request with all fields.
func TestParsePasswordModifyRequest_AllFields(t *testing.T) {
	reqValue := encodePasswordModifyRequestForTest(
		[]byte("uid=test,dc=example,dc=com"),
		[]byte("oldpass"),
		[]byte("newpass"),
	)

	req, err := parsePasswordModifyRequest(reqValue)
	if err != nil {
		t.Fatalf("parsePasswordModifyRequest returned error: %v", err)
	}

	if string(req.UserIdentity) != "uid=test,dc=example,dc=com" {
		t.Errorf("UserIdentity = %q, want %q", string(req.UserIdentity), "uid=test,dc=example,dc=com")
	}
	if string(req.OldPassword) != "oldpass" {
		t.Errorf("OldPassword = %q, want %q", string(req.OldPassword), "oldpass")
	}
	if string(req.NewPassword) != "newpass" {
		t.Errorf("NewPassword = %q, want %q", string(req.NewPassword), "newpass")
	}
}

// TestGenerateSecurePassword tests password generation.
func TestGenerateSecurePassword(t *testing.T) {
	password := generateSecurePassword(16)

	if len(password) != 16 {
		t.Errorf("Password length = %d, want 16", len(password))
	}

	// Generate another password and verify they're different
	password2 := generateSecurePassword(16)
	if string(password) == string(password2) {
		t.Error("Generated passwords should be different")
	}
}

// TestEncodePasswordModifyResponse tests response encoding.
func TestEncodePasswordModifyResponse(t *testing.T) {
	// Test with generated password
	resp := &PasswordModifyResponse{
		GenPassword: []byte("generatedpass"),
	}

	encoded := encodePasswordModifyResponse(resp)
	if len(encoded) == 0 {
		t.Error("Expected non-empty encoded response")
	}

	// Test without generated password
	resp2 := &PasswordModifyResponse{}
	encoded2 := encodePasswordModifyResponse(resp2)
	if len(encoded2) != 0 {
		t.Error("Expected empty encoded response when no generated password")
	}
}

// TestPasswordModifyHandler_SetBackend tests setting the backend.
func TestPasswordModifyHandler_SetBackend(t *testing.T) {
	handler := NewPasswordModifyHandler(nil)
	backend := newMockPasswordBackend()

	handler.SetBackend(backend)

	if handler.config.Backend != backend {
		t.Error("Backend was not set correctly")
	}
}

// TestPasswordModifyHandler_SetPolicyManager tests setting the policy manager.
func TestPasswordModifyHandler_SetPolicyManager(t *testing.T) {
	handler := NewPasswordModifyHandler(nil)
	manager := password.NewManager(nil)

	handler.SetPolicyManager(manager)

	if handler.config.PolicyManager != manager {
		t.Error("PolicyManager was not set correctly")
	}
}

// TestPasswordModifyHandler_AddAdminDN tests adding admin DNs.
func TestPasswordModifyHandler_AddAdminDN(t *testing.T) {
	handler := NewPasswordModifyHandler(nil)

	handler.AddAdminDN("cn=admin,dc=example,dc=com")
	handler.AddAdminDN("cn=superuser,dc=example,dc=com")

	if len(handler.config.AdminDNs) != 2 {
		t.Errorf("AdminDNs count = %d, want 2", len(handler.config.AdminDNs))
	}
}

// TestPasswordModifyHandler_SetRequireTLS tests setting TLS requirement.
func TestPasswordModifyHandler_SetRequireTLS(t *testing.T) {
	handler := NewPasswordModifyHandler(nil)

	handler.SetRequireTLS(true)

	if !handler.config.RequireTLS {
		t.Error("RequireTLS was not set correctly")
	}
}

// TestPasswordModifyHandler_Handle_RequireTLS tests TLS requirement enforcement.
func TestPasswordModifyHandler_Handle_RequireTLS(t *testing.T) {
	config := NewPasswordModifyConfig()
	config.RequireTLS = true

	handler := NewPasswordModifyHandler(config)

	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	conn := NewConnection(server, nil)
	conn.mu.Lock()
	conn.bindDN = "uid=alice,dc=example,dc=com"
	conn.authenticated = true
	conn.isTLS = false // Not using TLS
	conn.mu.Unlock()

	req := &ExtendedRequest{
		OID:   PasswordModifyOID,
		Value: nil,
	}

	resp, err := handler.Handle(conn, req)
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	// Should fail because TLS is required
	if resp.Result.ResultCode != ldap.ResultConfidentialityRequired {
		t.Errorf("ResultCode = %v, want %v", resp.Result.ResultCode, ldap.ResultConfidentialityRequired)
	}
}

// TestPasswordModifyHandler_isAdmin tests admin detection.
func TestPasswordModifyHandler_isAdmin(t *testing.T) {
	config := NewPasswordModifyConfig()
	config.AdminDNs = []string{
		"cn=admin,dc=example,dc=com",
		"cn=Directory Manager",
	}

	handler := NewPasswordModifyHandler(config)

	tests := []struct {
		dn       string
		expected bool
	}{
		{"cn=admin,dc=example,dc=com", true},
		{"CN=ADMIN,DC=EXAMPLE,DC=COM", true}, // Case insensitive
		{"cn=Directory Manager", true},
		{"uid=alice,ou=users,dc=example,dc=com", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.dn, func(t *testing.T) {
			result := handler.isAdmin(tt.dn)
			if result != tt.expected {
				t.Errorf("isAdmin(%q) = %v, want %v", tt.dn, result, tt.expected)
			}
		})
	}
}

// encodePasswordModifyRequestForTest creates a BER-encoded password modify request for testing.
func encodePasswordModifyRequestForTest(userIdentity, oldPassword, newPassword []byte) []byte {
	encoder := ber.NewBEREncoder(128)

	seqPos := encoder.BeginSequence()

	if len(userIdentity) > 0 {
		encoder.WriteTaggedValue(0, false, userIdentity)
	}
	if len(oldPassword) > 0 {
		encoder.WriteTaggedValue(1, false, oldPassword)
	}
	if len(newPassword) > 0 {
		encoder.WriteTaggedValue(2, false, newPassword)
	}

	encoder.EndSequence(seqPos)

	return encoder.Bytes()
}
