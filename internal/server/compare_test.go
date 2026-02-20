// Package server provides the LDAP server implementation.
package server

import (
	"errors"
	"testing"

	"github.com/oba-ldap/oba/internal/ldap"
	"github.com/oba-ldap/oba/internal/storage"
)

// mockCompareBackend implements the CompareBackend interface for testing.
type mockCompareBackend struct {
	entries map[string]*storage.Entry
	getErr  error
}

func newMockCompareBackend() *mockCompareBackend {
	return &mockCompareBackend{
		entries: make(map[string]*storage.Entry),
	}
}

func (m *mockCompareBackend) GetEntry(dn string) (*storage.Entry, error) {
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

func (m *mockCompareBackend) addEntry(entry *storage.Entry) {
	m.entries[entry.DN] = entry
}

// TestCompareHandlerImpl_Handle tests the compare handler.
func TestCompareHandlerImpl_Handle(t *testing.T) {
	tests := []struct {
		name         string
		setupBackend func(*mockCompareBackend)
		dn           string
		attribute    string
		value        []byte
		expectedCode ldap.ResultCode
	}{
		{
			name: "compare true - exact match",
			setupBackend: func(b *mockCompareBackend) {
				entry := storage.NewEntry("uid=alice,ou=users,dc=example,dc=com")
				entry.SetStringAttribute("uid", "alice")
				entry.SetStringAttribute("cn", "Alice Smith")
				b.addEntry(entry)
			},
			dn:           "uid=alice,ou=users,dc=example,dc=com",
			attribute:    "uid",
			value:        []byte("alice"),
			expectedCode: ldap.ResultCompareTrue,
		},
		{
			name: "compare false - value mismatch",
			setupBackend: func(b *mockCompareBackend) {
				entry := storage.NewEntry("uid=alice,ou=users,dc=example,dc=com")
				entry.SetStringAttribute("uid", "alice")
				b.addEntry(entry)
			},
			dn:           "uid=alice,ou=users,dc=example,dc=com",
			attribute:    "uid",
			value:        []byte("bob"),
			expectedCode: ldap.ResultCompareFalse,
		},
		{
			name: "compare true - case insensitive",
			setupBackend: func(b *mockCompareBackend) {
				entry := storage.NewEntry("uid=alice,ou=users,dc=example,dc=com")
				entry.SetStringAttribute("uid", "alice")
				b.addEntry(entry)
			},
			dn:           "uid=alice,ou=users,dc=example,dc=com",
			attribute:    "uid",
			value:        []byte("ALICE"),
			expectedCode: ldap.ResultCompareTrue,
		},
		{
			name: "entry not found",
			setupBackend: func(b *mockCompareBackend) {
				// No entries added
			},
			dn:           "uid=nonexistent,ou=users,dc=example,dc=com",
			attribute:    "uid",
			value:        []byte("alice"),
			expectedCode: ldap.ResultNoSuchObject,
		},
		{
			name: "attribute not found",
			setupBackend: func(b *mockCompareBackend) {
				entry := storage.NewEntry("uid=alice,ou=users,dc=example,dc=com")
				entry.SetStringAttribute("uid", "alice")
				b.addEntry(entry)
			},
			dn:           "uid=alice,ou=users,dc=example,dc=com",
			attribute:    "mail",
			value:        []byte("alice@example.com"),
			expectedCode: ldap.ResultNoSuchAttribute,
		},
		{
			name: "empty DN - protocol error",
			setupBackend: func(b *mockCompareBackend) {
				// No setup needed
			},
			dn:           "",
			attribute:    "uid",
			value:        []byte("alice"),
			expectedCode: ldap.ResultProtocolError,
		},
		{
			name: "empty attribute - protocol error",
			setupBackend: func(b *mockCompareBackend) {
				entry := storage.NewEntry("uid=alice,ou=users,dc=example,dc=com")
				entry.SetStringAttribute("uid", "alice")
				b.addEntry(entry)
			},
			dn:           "uid=alice,ou=users,dc=example,dc=com",
			attribute:    "",
			value:        []byte("alice"),
			expectedCode: ldap.ResultProtocolError,
		},
		{
			name: "backend get error",
			setupBackend: func(b *mockCompareBackend) {
				b.getErr = errors.New("internal error")
			},
			dn:           "uid=alice,ou=users,dc=example,dc=com",
			attribute:    "uid",
			value:        []byte("alice"),
			expectedCode: ldap.ResultOperationsError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockBackend := newMockCompareBackend()
			if tt.setupBackend != nil {
				tt.setupBackend(mockBackend)
			}

			config := &CompareConfig{
				Backend: mockBackend,
			}
			handler := NewCompareHandler(config)

			req := &ldap.CompareRequest{
				DN:        tt.dn,
				Attribute: tt.attribute,
				Value:     tt.value,
			}

			result := handler.Handle(nil, req)

			if result.ResultCode != tt.expectedCode {
				t.Errorf("Handle() ResultCode = %v, want %v", result.ResultCode, tt.expectedCode)
			}
		})
	}
}

// TestCompareHandlerImpl_Handle_NoBackend tests compare handler without backend.
func TestCompareHandlerImpl_Handle_NoBackend(t *testing.T) {
	config := &CompareConfig{
		Backend: nil,
	}
	handler := NewCompareHandler(config)

	req := &ldap.CompareRequest{
		DN:        "uid=alice,ou=users,dc=example,dc=com",
		Attribute: "uid",
		Value:     []byte("alice"),
	}

	result := handler.Handle(nil, req)

	if result.ResultCode != ldap.ResultOperationsError {
		t.Errorf("Handle() ResultCode = %v, want %v", result.ResultCode, ldap.ResultOperationsError)
	}
}

// TestCompareHandlerImpl_Handle_MultiValueAttribute tests compare with multi-value attribute.
func TestCompareHandlerImpl_Handle_MultiValueAttribute(t *testing.T) {
	mockBackend := newMockCompareBackend()
	entry := storage.NewEntry("uid=alice,ou=users,dc=example,dc=com")
	entry.SetAttribute("mail", [][]byte{
		[]byte("alice@example.com"),
		[]byte("alice.smith@example.com"),
	})
	mockBackend.addEntry(entry)

	config := &CompareConfig{
		Backend: mockBackend,
	}
	handler := NewCompareHandler(config)

	// Test matching first value
	req := &ldap.CompareRequest{
		DN:        "uid=alice,ou=users,dc=example,dc=com",
		Attribute: "mail",
		Value:     []byte("alice@example.com"),
	}

	result := handler.Handle(nil, req)
	if result.ResultCode != ldap.ResultCompareTrue {
		t.Errorf("Handle() ResultCode = %v, want %v", result.ResultCode, ldap.ResultCompareTrue)
	}

	// Test matching second value
	req.Value = []byte("alice.smith@example.com")
	result = handler.Handle(nil, req)
	if result.ResultCode != ldap.ResultCompareTrue {
		t.Errorf("Handle() ResultCode = %v, want %v", result.ResultCode, ldap.ResultCompareTrue)
	}

	// Test non-matching value
	req.Value = []byte("bob@example.com")
	result = handler.Handle(nil, req)
	if result.ResultCode != ldap.ResultCompareFalse {
		t.Errorf("Handle() ResultCode = %v, want %v", result.ResultCode, ldap.ResultCompareFalse)
	}
}

// TestCompareHandlerImpl_Handle_CaseInsensitiveDN tests case-insensitive DN handling.
func TestCompareHandlerImpl_Handle_CaseInsensitiveDN(t *testing.T) {
	mockBackend := newMockCompareBackend()
	entry := storage.NewEntry("uid=alice,ou=users,dc=example,dc=com")
	entry.SetStringAttribute("uid", "alice")
	mockBackend.addEntry(entry)

	config := &CompareConfig{
		Backend: mockBackend,
	}
	handler := NewCompareHandler(config)

	// Use uppercase DN
	req := &ldap.CompareRequest{
		DN:        "UID=ALICE,OU=USERS,DC=EXAMPLE,DC=COM",
		Attribute: "uid",
		Value:     []byte("alice"),
	}

	result := handler.Handle(nil, req)

	if result.ResultCode != ldap.ResultCompareTrue {
		t.Errorf("Handle() ResultCode = %v, want %v", result.ResultCode, ldap.ResultCompareTrue)
	}
}

// TestCreateCompareHandler tests the CreateCompareHandler function.
func TestCreateCompareHandler(t *testing.T) {
	mockBackend := newMockCompareBackend()
	entry := storage.NewEntry("uid=alice,ou=users,dc=example,dc=com")
	entry.SetStringAttribute("uid", "alice")
	mockBackend.addEntry(entry)

	config := &CompareConfig{
		Backend: mockBackend,
	}
	impl := NewCompareHandler(config)
	handler := CreateCompareHandler(impl)

	req := &ldap.CompareRequest{
		DN:        "uid=alice,ou=users,dc=example,dc=com",
		Attribute: "uid",
		Value:     []byte("alice"),
	}

	result := handler(nil, req)

	if result.ResultCode != ldap.ResultCompareTrue {
		t.Errorf("handler() ResultCode = %v, want %v", result.ResultCode, ldap.ResultCompareTrue)
	}
}

// TestNewCompareConfig tests the NewCompareConfig function.
func TestNewCompareConfig(t *testing.T) {
	config := NewCompareConfig()

	if config == nil {
		t.Error("NewCompareConfig() returned nil")
	}

	if config.Backend != nil {
		t.Error("NewCompareConfig() should have nil Backend by default")
	}
}

// TestNewCompareHandler_NilConfig tests NewCompareHandler with nil config.
func TestNewCompareHandler_NilConfig(t *testing.T) {
	handler := NewCompareHandler(nil)

	if handler == nil {
		t.Error("NewCompareHandler(nil) returned nil")
	}

	if handler.config == nil {
		t.Error("NewCompareHandler(nil) should create default config")
	}
}
