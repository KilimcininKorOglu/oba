// Package server provides the LDAP server implementation.
package server

import (
	"errors"
	"testing"

	"github.com/oba-ldap/oba/internal/ldap"
)

// mockModifyDNBackend implements the ModifyDNBackend interface for testing.
type mockModifyDNBackend struct {
	entries      map[string]bool
	modifyDNFunc func(req *ModifyDNRequestData) error
	err          error
}

func newMockModifyDNBackend() *mockModifyDNBackend {
	return &mockModifyDNBackend{
		entries: make(map[string]bool),
	}
}

func (m *mockModifyDNBackend) ModifyDN(req *ModifyDNRequestData) error {
	if m.modifyDNFunc != nil {
		return m.modifyDNFunc(req)
	}
	if m.err != nil {
		return m.err
	}
	return nil
}

func (m *mockModifyDNBackend) addEntry(dn string) {
	m.entries[normalizeDN(dn)] = true
}

// TestModifyDNHandler_BasicRename tests basic RDN change functionality.
func TestModifyDNHandler_BasicRename(t *testing.T) {
	backend := newMockModifyDNBackend()
	backend.addEntry("uid=alice,ou=users,dc=example,dc=com")

	config := &ModifyDNConfig{
		Backend: backend,
	}
	handler := NewModifyDNHandler(config)

	req := &ldap.ModifyDNRequest{
		Entry:        "uid=alice,ou=users,dc=example,dc=com",
		NewRDN:       "uid=alice2",
		DeleteOldRDN: true,
	}

	result := handler.Handle(nil, req)

	if result.ResultCode != ldap.ResultSuccess {
		t.Errorf("Handle() ResultCode = %v, want %v", result.ResultCode, ldap.ResultSuccess)
	}
}

// TestModifyDNHandler_MoveToNewParent tests moving an entry to a new parent.
func TestModifyDNHandler_MoveToNewParent(t *testing.T) {
	backend := newMockModifyDNBackend()
	backend.addEntry("uid=alice,ou=users,dc=example,dc=com")
	backend.addEntry("ou=people,dc=example,dc=com")

	config := &ModifyDNConfig{
		Backend: backend,
	}
	handler := NewModifyDNHandler(config)

	req := &ldap.ModifyDNRequest{
		Entry:        "uid=alice,ou=users,dc=example,dc=com",
		NewRDN:       "uid=alice",
		DeleteOldRDN: false,
		NewSuperior:  "ou=people,dc=example,dc=com",
	}

	result := handler.Handle(nil, req)

	if result.ResultCode != ldap.ResultSuccess {
		t.Errorf("Handle() ResultCode = %v, want %v", result.ResultCode, ldap.ResultSuccess)
	}
}

// TestModifyDNHandler_EntryNotFound tests error when entry doesn't exist.
func TestModifyDNHandler_EntryNotFound(t *testing.T) {
	backend := newMockModifyDNBackend()
	backend.err = errors.New("entry not found")

	config := &ModifyDNConfig{
		Backend: backend,
		ErrorMapper: func(err error) ModifyDNError {
			return ModifyDNErrEntryNotFound
		},
	}
	handler := NewModifyDNHandler(config)

	req := &ldap.ModifyDNRequest{
		Entry:        "uid=nonexistent,ou=users,dc=example,dc=com",
		NewRDN:       "uid=newname",
		DeleteOldRDN: true,
	}

	result := handler.Handle(nil, req)

	if result.ResultCode != ldap.ResultNoSuchObject {
		t.Errorf("Handle() ResultCode = %v, want %v", result.ResultCode, ldap.ResultNoSuchObject)
	}
}

// TestModifyDNHandler_EntryAlreadyExists tests error when new DN already exists.
func TestModifyDNHandler_EntryAlreadyExists(t *testing.T) {
	backend := newMockModifyDNBackend()
	backend.err = errors.New("entry already exists")

	config := &ModifyDNConfig{
		Backend: backend,
		ErrorMapper: func(err error) ModifyDNError {
			return ModifyDNErrEntryExists
		},
	}
	handler := NewModifyDNHandler(config)

	req := &ldap.ModifyDNRequest{
		Entry:        "uid=alice,ou=users,dc=example,dc=com",
		NewRDN:       "uid=bob",
		DeleteOldRDN: true,
	}

	result := handler.Handle(nil, req)

	if result.ResultCode != ldap.ResultEntryAlreadyExists {
		t.Errorf("Handle() ResultCode = %v, want %v", result.ResultCode, ldap.ResultEntryAlreadyExists)
	}
}

// TestModifyDNHandler_NewSuperiorNotFound tests error when new superior doesn't exist.
func TestModifyDNHandler_NewSuperiorNotFound(t *testing.T) {
	backend := newMockModifyDNBackend()
	backend.err = errors.New("new superior not found")

	config := &ModifyDNConfig{
		Backend: backend,
		ErrorMapper: func(err error) ModifyDNError {
			return ModifyDNErrNewSuperiorNotFound
		},
	}
	handler := NewModifyDNHandler(config)

	req := &ldap.ModifyDNRequest{
		Entry:        "uid=alice,ou=users,dc=example,dc=com",
		NewRDN:       "uid=alice",
		DeleteOldRDN: false,
		NewSuperior:  "ou=nonexistent,dc=example,dc=com",
	}

	result := handler.Handle(nil, req)

	if result.ResultCode != ldap.ResultNoSuchObject {
		t.Errorf("Handle() ResultCode = %v, want %v", result.ResultCode, ldap.ResultNoSuchObject)
	}
}

// TestModifyDNHandler_InvalidDN tests error for invalid DN syntax.
func TestModifyDNHandler_InvalidDN(t *testing.T) {
	backend := newMockModifyDNBackend()
	backend.err = errors.New("invalid DN")

	config := &ModifyDNConfig{
		Backend: backend,
		ErrorMapper: func(err error) ModifyDNError {
			return ModifyDNErrInvalidDN
		},
	}
	handler := NewModifyDNHandler(config)

	req := &ldap.ModifyDNRequest{
		Entry:        "uid=alice,ou=users,dc=example,dc=com",
		NewRDN:       "invalid",
		DeleteOldRDN: true,
	}

	result := handler.Handle(nil, req)

	if result.ResultCode != ldap.ResultInvalidDNSyntax {
		t.Errorf("Handle() ResultCode = %v, want %v", result.ResultCode, ldap.ResultInvalidDNSyntax)
	}
}

// TestModifyDNHandler_EmptyEntry tests validation for empty entry DN.
func TestModifyDNHandler_EmptyEntry(t *testing.T) {
	backend := newMockModifyDNBackend()

	config := &ModifyDNConfig{
		Backend: backend,
	}
	handler := NewModifyDNHandler(config)

	req := &ldap.ModifyDNRequest{
		Entry:        "",
		NewRDN:       "uid=newname",
		DeleteOldRDN: true,
	}

	result := handler.Handle(nil, req)

	if result.ResultCode != ldap.ResultProtocolError {
		t.Errorf("Handle() ResultCode = %v, want %v", result.ResultCode, ldap.ResultProtocolError)
	}
}

// TestModifyDNHandler_EmptyNewRDN tests validation for empty new RDN.
func TestModifyDNHandler_EmptyNewRDN(t *testing.T) {
	backend := newMockModifyDNBackend()

	config := &ModifyDNConfig{
		Backend: backend,
	}
	handler := NewModifyDNHandler(config)

	req := &ldap.ModifyDNRequest{
		Entry:        "uid=alice,ou=users,dc=example,dc=com",
		NewRDN:       "",
		DeleteOldRDN: true,
	}

	result := handler.Handle(nil, req)

	if result.ResultCode != ldap.ResultProtocolError {
		t.Errorf("Handle() ResultCode = %v, want %v", result.ResultCode, ldap.ResultProtocolError)
	}
}

// TestModifyDNHandler_NoBackend tests error when backend is not configured.
func TestModifyDNHandler_NoBackend(t *testing.T) {
	config := &ModifyDNConfig{
		Backend: nil,
	}
	handler := NewModifyDNHandler(config)

	req := &ldap.ModifyDNRequest{
		Entry:        "uid=alice,ou=users,dc=example,dc=com",
		NewRDN:       "uid=newname",
		DeleteOldRDN: true,
	}

	result := handler.Handle(nil, req)

	if result.ResultCode != ldap.ResultOperationsError {
		t.Errorf("Handle() ResultCode = %v, want %v", result.ResultCode, ldap.ResultOperationsError)
	}
}

// TestModifyDNHandler_DeleteOldRDN tests the DeleteOldRDN flag handling.
func TestModifyDNHandler_DeleteOldRDN(t *testing.T) {
	tests := []struct {
		name         string
		deleteOldRDN bool
	}{
		{
			name:         "delete old RDN true",
			deleteOldRDN: true,
		},
		{
			name:         "delete old RDN false",
			deleteOldRDN: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedReq *ModifyDNRequestData
			backend := newMockModifyDNBackend()
			backend.modifyDNFunc = func(req *ModifyDNRequestData) error {
				capturedReq = req
				return nil
			}

			config := &ModifyDNConfig{
				Backend: backend,
			}
			handler := NewModifyDNHandler(config)

			req := &ldap.ModifyDNRequest{
				Entry:        "uid=alice,ou=users,dc=example,dc=com",
				NewRDN:       "uid=alice2",
				DeleteOldRDN: tt.deleteOldRDN,
			}

			result := handler.Handle(nil, req)

			if result.ResultCode != ldap.ResultSuccess {
				t.Errorf("Handle() ResultCode = %v, want %v", result.ResultCode, ldap.ResultSuccess)
			}

			if capturedReq == nil {
				t.Fatal("Backend ModifyDN was not called")
			}

			if capturedReq.DeleteOldRDN != tt.deleteOldRDN {
				t.Errorf("DeleteOldRDN = %v, want %v", capturedReq.DeleteOldRDN, tt.deleteOldRDN)
			}
		})
	}
}

// TestModifyDNHandler_NotAllowedOnNonLeaf tests error for non-leaf entries.
func TestModifyDNHandler_NotAllowedOnNonLeaf(t *testing.T) {
	backend := newMockModifyDNBackend()
	backend.err = errors.New("not allowed on non-leaf")

	config := &ModifyDNConfig{
		Backend: backend,
		ErrorMapper: func(err error) ModifyDNError {
			return ModifyDNErrNotAllowedOnNonLeaf
		},
	}
	handler := NewModifyDNHandler(config)

	req := &ldap.ModifyDNRequest{
		Entry:        "ou=users,dc=example,dc=com",
		NewRDN:       "ou=people",
		DeleteOldRDN: true,
	}

	result := handler.Handle(nil, req)

	if result.ResultCode != ldap.ResultNotAllowedOnNonLeaf {
		t.Errorf("Handle() ResultCode = %v, want %v", result.ResultCode, ldap.ResultNotAllowedOnNonLeaf)
	}
}

// TestNewModifyDNConfig tests the NewModifyDNConfig function.
func TestNewModifyDNConfig(t *testing.T) {
	config := NewModifyDNConfig()

	if config == nil {
		t.Fatal("NewModifyDNConfig() returned nil")
	}

	if config.Backend != nil {
		t.Error("NewModifyDNConfig() Backend should be nil by default")
	}
}

// TestNewModifyDNHandler_NilConfig tests NewModifyDNHandler with nil config.
func TestNewModifyDNHandler_NilConfig(t *testing.T) {
	handler := NewModifyDNHandler(nil)

	if handler == nil {
		t.Fatal("NewModifyDNHandler(nil) returned nil")
	}

	if handler.config == nil {
		t.Fatal("NewModifyDNHandler(nil) config is nil")
	}
}

// TestCreateModifyDNHandler tests the CreateModifyDNHandler function.
func TestCreateModifyDNHandler(t *testing.T) {
	backend := newMockModifyDNBackend()

	config := &ModifyDNConfig{
		Backend: backend,
	}
	impl := NewModifyDNHandler(config)
	handler := CreateModifyDNHandler(impl)

	req := &ldap.ModifyDNRequest{
		Entry:        "uid=alice,ou=users,dc=example,dc=com",
		NewRDN:       "uid=alice2",
		DeleteOldRDN: true,
	}

	result := handler(nil, req)

	if result.ResultCode != ldap.ResultSuccess {
		t.Errorf("handler() ResultCode = %v, want %v", result.ResultCode, ldap.ResultSuccess)
	}
}

// TestModifyDNHandler_RequestDataMapping tests that request data is correctly mapped.
func TestModifyDNHandler_RequestDataMapping(t *testing.T) {
	var capturedReq *ModifyDNRequestData
	backend := newMockModifyDNBackend()
	backend.modifyDNFunc = func(req *ModifyDNRequestData) error {
		capturedReq = req
		return nil
	}

	config := &ModifyDNConfig{
		Backend: backend,
	}
	handler := NewModifyDNHandler(config)

	req := &ldap.ModifyDNRequest{
		Entry:        "uid=alice,ou=users,dc=example,dc=com",
		NewRDN:       "uid=alice2",
		DeleteOldRDN: true,
		NewSuperior:  "ou=people,dc=example,dc=com",
	}

	result := handler.Handle(nil, req)

	if result.ResultCode != ldap.ResultSuccess {
		t.Errorf("Handle() ResultCode = %v, want %v", result.ResultCode, ldap.ResultSuccess)
	}

	if capturedReq == nil {
		t.Fatal("Backend ModifyDN was not called")
	}

	if capturedReq.DN != req.Entry {
		t.Errorf("DN = %v, want %v", capturedReq.DN, req.Entry)
	}

	if capturedReq.NewRDN != req.NewRDN {
		t.Errorf("NewRDN = %v, want %v", capturedReq.NewRDN, req.NewRDN)
	}

	if capturedReq.DeleteOldRDN != req.DeleteOldRDN {
		t.Errorf("DeleteOldRDN = %v, want %v", capturedReq.DeleteOldRDN, req.DeleteOldRDN)
	}

	if capturedReq.NewSuperior != req.NewSuperior {
		t.Errorf("NewSuperior = %v, want %v", capturedReq.NewSuperior, req.NewSuperior)
	}
}
