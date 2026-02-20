// Package server provides the LDAP server implementation.
package server

import (
	"errors"
	"testing"

	"github.com/oba-ldap/oba/internal/ldap"
	"github.com/oba-ldap/oba/internal/storage"
)

// mockDeleteBackend implements the DeleteBackend interface for testing.
type mockDeleteBackend struct {
	entries      map[string]*storage.Entry
	children     map[string]bool
	getErr       error
	deleteErr    error
	hasChildErr  error
}

func newMockDeleteBackend() *mockDeleteBackend {
	return &mockDeleteBackend{
		entries:  make(map[string]*storage.Entry),
		children: make(map[string]bool),
	}
}

func (m *mockDeleteBackend) GetEntry(dn string) (*storage.Entry, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	normalizedDN := normalizeDN(dn)
	for storedDN, entry := range m.entries {
		if normalizeDN(storedDN) == normalizedDN {
			return entry, nil
		}
	}
	return nil, nil
}

func (m *mockDeleteBackend) DeleteEntry(dn string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	normalizedDN := normalizeDN(dn)
	for storedDN := range m.entries {
		if normalizeDN(storedDN) == normalizedDN {
			delete(m.entries, storedDN)
			return nil
		}
	}
	return errors.New("entry not found")
}

func (m *mockDeleteBackend) HasChildren(dn string) (bool, error) {
	if m.hasChildErr != nil {
		return false, m.hasChildErr
	}
	normalizedDN := normalizeDN(dn)
	return m.children[normalizedDN], nil
}

func (m *mockDeleteBackend) addEntry(entry *storage.Entry) {
	m.entries[entry.DN] = entry
}

func (m *mockDeleteBackend) setHasChildren(dn string, hasChildren bool) {
	m.children[normalizeDN(dn)] = hasChildren
}

// TestDeleteHandlerImpl_Handle tests the delete handler.
func TestDeleteHandlerImpl_Handle(t *testing.T) {
	tests := []struct {
		name           string
		setupBackend   func(*mockDeleteBackend)
		dn             string
		expectedCode   ldap.ResultCode
		expectedMsg    string
	}{
		{
			name: "successful delete",
			setupBackend: func(b *mockDeleteBackend) {
				entry := storage.NewEntry("uid=alice,ou=users,dc=example,dc=com")
				entry.SetStringAttribute("uid", "alice")
				b.addEntry(entry)
			},
			dn:           "uid=alice,ou=users,dc=example,dc=com",
			expectedCode: ldap.ResultSuccess,
		},
		{
			name: "entry not found",
			setupBackend: func(b *mockDeleteBackend) {
				// No entries added
			},
			dn:           "uid=nonexistent,ou=users,dc=example,dc=com",
			expectedCode: ldap.ResultNoSuchObject,
		},
		{
			name: "entry has children - not allowed on non-leaf",
			setupBackend: func(b *mockDeleteBackend) {
				entry := storage.NewEntry("ou=users,dc=example,dc=com")
				entry.SetStringAttribute("ou", "users")
				b.addEntry(entry)
				b.setHasChildren("ou=users,dc=example,dc=com", true)
			},
			dn:           "ou=users,dc=example,dc=com",
			expectedCode: ldap.ResultNotAllowedOnNonLeaf,
		},
		{
			name: "empty DN - protocol error",
			setupBackend: func(b *mockDeleteBackend) {
				// No setup needed
			},
			dn:           "",
			expectedCode: ldap.ResultProtocolError,
		},
		{
			name: "backend get error",
			setupBackend: func(b *mockDeleteBackend) {
				b.getErr = errors.New("internal error")
			},
			dn:           "uid=alice,ou=users,dc=example,dc=com",
			expectedCode: ldap.ResultOperationsError,
		},
		{
			name: "backend has children error",
			setupBackend: func(b *mockDeleteBackend) {
				entry := storage.NewEntry("uid=alice,ou=users,dc=example,dc=com")
				b.addEntry(entry)
				b.hasChildErr = errors.New("internal error")
			},
			dn:           "uid=alice,ou=users,dc=example,dc=com",
			expectedCode: ldap.ResultOperationsError,
		},
		{
			name: "backend delete error",
			setupBackend: func(b *mockDeleteBackend) {
				entry := storage.NewEntry("uid=alice,ou=users,dc=example,dc=com")
				b.addEntry(entry)
				b.deleteErr = errors.New("internal error")
			},
			dn:           "uid=alice,ou=users,dc=example,dc=com",
			expectedCode: ldap.ResultOperationsError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend := newMockDeleteBackend()
			if tt.setupBackend != nil {
				tt.setupBackend(backend)
			}

			config := &DeleteConfig{
				Backend: backend,
			}
			handler := NewDeleteHandler(config)

			req := &ldap.DeleteRequest{
				DN: tt.dn,
			}

			result := handler.Handle(nil, req)

			if result.ResultCode != tt.expectedCode {
				t.Errorf("Handle() ResultCode = %v, want %v", result.ResultCode, tt.expectedCode)
			}
		})
	}
}

// TestDeleteHandlerImpl_Handle_NoBackend tests delete handler without backend.
func TestDeleteHandlerImpl_Handle_NoBackend(t *testing.T) {
	config := &DeleteConfig{
		Backend: nil,
	}
	handler := NewDeleteHandler(config)

	req := &ldap.DeleteRequest{
		DN: "uid=alice,ou=users,dc=example,dc=com",
	}

	result := handler.Handle(nil, req)

	if result.ResultCode != ldap.ResultOperationsError {
		t.Errorf("Handle() ResultCode = %v, want %v", result.ResultCode, ldap.ResultOperationsError)
	}
}

// TestDeleteHandlerImpl_Handle_EntryActuallyDeleted verifies entry is removed.
func TestDeleteHandlerImpl_Handle_EntryActuallyDeleted(t *testing.T) {
	backend := newMockDeleteBackend()
	entry := storage.NewEntry("uid=alice,ou=users,dc=example,dc=com")
	entry.SetStringAttribute("uid", "alice")
	backend.addEntry(entry)

	config := &DeleteConfig{
		Backend: backend,
	}
	handler := NewDeleteHandler(config)

	// Verify entry exists before delete
	existingEntry, _ := backend.GetEntry("uid=alice,ou=users,dc=example,dc=com")
	if existingEntry == nil {
		t.Fatal("Entry should exist before delete")
	}

	req := &ldap.DeleteRequest{
		DN: "uid=alice,ou=users,dc=example,dc=com",
	}

	result := handler.Handle(nil, req)

	if result.ResultCode != ldap.ResultSuccess {
		t.Errorf("Handle() ResultCode = %v, want %v", result.ResultCode, ldap.ResultSuccess)
	}

	// Verify entry is deleted
	deletedEntry, _ := backend.GetEntry("uid=alice,ou=users,dc=example,dc=com")
	if deletedEntry != nil {
		t.Error("Entry should be deleted after successful delete operation")
	}
}

// TestDeleteHandlerImpl_Handle_CaseInsensitiveDN tests case-insensitive DN handling.
func TestDeleteHandlerImpl_Handle_CaseInsensitiveDN(t *testing.T) {
	backend := newMockDeleteBackend()
	entry := storage.NewEntry("uid=alice,ou=users,dc=example,dc=com")
	entry.SetStringAttribute("uid", "alice")
	backend.addEntry(entry)

	config := &DeleteConfig{
		Backend: backend,
	}
	handler := NewDeleteHandler(config)

	// Use uppercase DN
	req := &ldap.DeleteRequest{
		DN: "UID=ALICE,OU=USERS,DC=EXAMPLE,DC=COM",
	}

	result := handler.Handle(nil, req)

	if result.ResultCode != ldap.ResultSuccess {
		t.Errorf("Handle() ResultCode = %v, want %v", result.ResultCode, ldap.ResultSuccess)
	}
}

// TestCreateDeleteHandler tests the CreateDeleteHandler function.
func TestCreateDeleteHandler(t *testing.T) {
	backend := newMockDeleteBackend()
	entry := storage.NewEntry("uid=alice,ou=users,dc=example,dc=com")
	backend.addEntry(entry)

	config := &DeleteConfig{
		Backend: backend,
	}
	impl := NewDeleteHandler(config)
	handler := CreateDeleteHandler(impl)

	req := &ldap.DeleteRequest{
		DN: "uid=alice,ou=users,dc=example,dc=com",
	}

	result := handler(nil, req)

	if result.ResultCode != ldap.ResultSuccess {
		t.Errorf("handler() ResultCode = %v, want %v", result.ResultCode, ldap.ResultSuccess)
	}
}

// TestNewDeleteConfig tests the NewDeleteConfig function.
func TestNewDeleteConfig(t *testing.T) {
	config := NewDeleteConfig()

	if config == nil {
		t.Error("NewDeleteConfig() returned nil")
	}

	if config.Backend != nil {
		t.Error("NewDeleteConfig() should have nil Backend by default")
	}
}

// TestNewDeleteHandler_NilConfig tests NewDeleteHandler with nil config.
func TestNewDeleteHandler_NilConfig(t *testing.T) {
	handler := NewDeleteHandler(nil)

	if handler == nil {
		t.Error("NewDeleteHandler(nil) returned nil")
	}

	if handler.config == nil {
		t.Error("NewDeleteHandler(nil) should create default config")
	}
}
