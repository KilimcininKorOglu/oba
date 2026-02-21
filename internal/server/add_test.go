// Package server provides the LDAP server implementation.
package server

import (
	"errors"
	"testing"

	"github.com/KilimcininKorOglu/oba/internal/ldap"
	"github.com/KilimcininKorOglu/oba/internal/storage"
)

// mockAddBackend implements the AddBackend interface for testing.
type mockAddBackend struct {
	entries map[string]*storage.Entry
	getErr  error
	addErr  error
}

func newMockAddBackend() *mockAddBackend {
	return &mockAddBackend{
		entries: make(map[string]*storage.Entry),
	}
}

func (m *mockAddBackend) GetEntry(dn string) (*storage.Entry, error) {
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

func (m *mockAddBackend) AddEntry(entry *storage.Entry) error {
	if m.addErr != nil {
		return m.addErr
	}
	normalizedDN := normalizeDN(entry.DN)

	// Check if entry already exists
	for storedDN := range m.entries {
		if normalizeDN(storedDN) == normalizedDN {
			return errors.New("entry already exists")
		}
	}

	// Check if parent exists (for non-root entries)
	parentDN := getParentDN(normalizedDN)
	if parentDN != "" {
		parentExists := false
		for storedDN := range m.entries {
			if normalizeDN(storedDN) == parentDN {
				parentExists = true
				break
			}
		}
		if !parentExists {
			return errors.New("parent entry not found")
		}
	}

	m.entries[normalizedDN] = entry
	return nil
}

func (m *mockAddBackend) addEntry(entry *storage.Entry) {
	m.entries[normalizeDN(entry.DN)] = entry
}

// getParentDN extracts the parent DN from a DN string.
func getParentDN(dn string) string {
	idx := -1
	for i, c := range dn {
		if c == ',' {
			idx = i
			break
		}
	}
	if idx == -1 {
		return ""
	}
	return dn[idx+1:]
}

// TestAddHandlerImpl_Handle tests the add handler.
func TestAddHandlerImpl_Handle(t *testing.T) {
	tests := []struct {
		name         string
		setupBackend func(*mockAddBackend)
		entry        string
		attributes   []ldap.Attribute
		expectedCode ldap.ResultCode
	}{
		{
			name: "successful add",
			setupBackend: func(b *mockAddBackend) {
				// Add parent entry
				parent := storage.NewEntry("ou=users,dc=example,dc=com")
				parent.SetStringAttribute("objectclass", "organizationalUnit")
				b.addEntry(parent)
			},
			entry: "uid=alice,ou=users,dc=example,dc=com",
			attributes: []ldap.Attribute{
				{Type: "objectClass", Values: [][]byte{[]byte("person")}},
				{Type: "uid", Values: [][]byte{[]byte("alice")}},
				{Type: "cn", Values: [][]byte{[]byte("Alice")}},
			},
			expectedCode: ldap.ResultSuccess,
		},
		{
			name: "entry already exists",
			setupBackend: func(b *mockAddBackend) {
				// Add parent entry
				parent := storage.NewEntry("ou=users,dc=example,dc=com")
				parent.SetStringAttribute("objectclass", "organizationalUnit")
				b.addEntry(parent)
				// Add existing entry
				entry := storage.NewEntry("uid=alice,ou=users,dc=example,dc=com")
				entry.SetStringAttribute("objectclass", "person")
				b.addEntry(entry)
			},
			entry: "uid=alice,ou=users,dc=example,dc=com",
			attributes: []ldap.Attribute{
				{Type: "objectClass", Values: [][]byte{[]byte("person")}},
				{Type: "uid", Values: [][]byte{[]byte("alice")}},
			},
			expectedCode: ldap.ResultEntryAlreadyExists,
		},
		{
			name: "parent does not exist",
			setupBackend: func(b *mockAddBackend) {
				// No parent entry added
			},
			entry: "uid=alice,ou=users,dc=example,dc=com",
			attributes: []ldap.Attribute{
				{Type: "objectClass", Values: [][]byte{[]byte("person")}},
				{Type: "uid", Values: [][]byte{[]byte("alice")}},
			},
			expectedCode: ldap.ResultNoSuchObject,
		},
		{
			name: "missing objectClass",
			setupBackend: func(b *mockAddBackend) {
				// Add parent entry
				parent := storage.NewEntry("ou=users,dc=example,dc=com")
				parent.SetStringAttribute("objectclass", "organizationalUnit")
				b.addEntry(parent)
			},
			entry: "uid=alice,ou=users,dc=example,dc=com",
			attributes: []ldap.Attribute{
				{Type: "uid", Values: [][]byte{[]byte("alice")}},
				{Type: "cn", Values: [][]byte{[]byte("Alice")}},
			},
			expectedCode: ldap.ResultObjectClassViolation,
		},
		{
			name: "empty DN - protocol error",
			setupBackend: func(b *mockAddBackend) {
				// No setup needed
			},
			entry: "",
			attributes: []ldap.Attribute{
				{Type: "objectClass", Values: [][]byte{[]byte("person")}},
			},
			expectedCode: ldap.ResultProtocolError,
		},
		{
			name: "empty objectClass values",
			setupBackend: func(b *mockAddBackend) {
				// Add parent entry
				parent := storage.NewEntry("ou=users,dc=example,dc=com")
				parent.SetStringAttribute("objectclass", "organizationalUnit")
				b.addEntry(parent)
			},
			entry: "uid=alice,ou=users,dc=example,dc=com",
			attributes: []ldap.Attribute{
				{Type: "objectClass", Values: [][]byte{}},
				{Type: "uid", Values: [][]byte{[]byte("alice")}},
			},
			expectedCode: ldap.ResultObjectClassViolation,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockBackend := newMockAddBackend()
			if tt.setupBackend != nil {
				tt.setupBackend(mockBackend)
			}

			config := &AddConfig{
				Backend: mockBackend,
			}
			handler := NewAddHandler(config)

			req := &ldap.AddRequest{
				Entry:      tt.entry,
				Attributes: tt.attributes,
			}

			result := handler.Handle(nil, req)

			if result.ResultCode != tt.expectedCode {
				t.Errorf("Handle() ResultCode = %v, want %v", result.ResultCode, tt.expectedCode)
			}
		})
	}
}

// TestAddHandlerImpl_Handle_NoBackend tests add handler without backend.
func TestAddHandlerImpl_Handle_NoBackend(t *testing.T) {
	config := &AddConfig{
		Backend: nil,
	}
	handler := NewAddHandler(config)

	req := &ldap.AddRequest{
		Entry: "uid=alice,ou=users,dc=example,dc=com",
		Attributes: []ldap.Attribute{
			{Type: "objectClass", Values: [][]byte{[]byte("person")}},
		},
	}

	result := handler.Handle(nil, req)

	if result.ResultCode != ldap.ResultOperationsError {
		t.Errorf("Handle() ResultCode = %v, want %v", result.ResultCode, ldap.ResultOperationsError)
	}
}

// TestAddHandlerImpl_Handle_EntryActuallyAdded verifies entry is added.
func TestAddHandlerImpl_Handle_EntryActuallyAdded(t *testing.T) {
	mockBackend := newMockAddBackend()

	// Add parent entry
	parent := storage.NewEntry("ou=users,dc=example,dc=com")
	parent.SetStringAttribute("objectclass", "organizationalUnit")
	mockBackend.addEntry(parent)

	config := &AddConfig{
		Backend: mockBackend,
	}
	handler := NewAddHandler(config)

	// Verify entry does not exist before add
	existingEntry, _ := mockBackend.GetEntry("uid=alice,ou=users,dc=example,dc=com")
	if existingEntry != nil {
		t.Fatal("Entry should not exist before add")
	}

	req := &ldap.AddRequest{
		Entry: "uid=alice,ou=users,dc=example,dc=com",
		Attributes: []ldap.Attribute{
			{Type: "objectClass", Values: [][]byte{[]byte("person")}},
			{Type: "uid", Values: [][]byte{[]byte("alice")}},
			{Type: "cn", Values: [][]byte{[]byte("Alice")}},
		},
	}

	result := handler.Handle(nil, req)

	if result.ResultCode != ldap.ResultSuccess {
		t.Errorf("Handle() ResultCode = %v, want %v", result.ResultCode, ldap.ResultSuccess)
	}

	// Verify entry is added
	addedEntry, _ := mockBackend.GetEntry("uid=alice,ou=users,dc=example,dc=com")
	if addedEntry == nil {
		t.Error("Entry should exist after successful add operation")
	}
}

// TestAddHandlerImpl_Handle_CaseInsensitiveDN tests case-insensitive DN handling.
func TestAddHandlerImpl_Handle_CaseInsensitiveDN(t *testing.T) {
	mockBackend := newMockAddBackend()

	// Add parent entry with lowercase
	parent := storage.NewEntry("ou=users,dc=example,dc=com")
	parent.SetStringAttribute("objectclass", "organizationalUnit")
	mockBackend.addEntry(parent)

	config := &AddConfig{
		Backend: mockBackend,
	}
	handler := NewAddHandler(config)

	// Use uppercase DN
	req := &ldap.AddRequest{
		Entry: "UID=ALICE,OU=USERS,DC=EXAMPLE,DC=COM",
		Attributes: []ldap.Attribute{
			{Type: "objectClass", Values: [][]byte{[]byte("person")}},
			{Type: "uid", Values: [][]byte{[]byte("alice")}},
		},
	}

	result := handler.Handle(nil, req)

	if result.ResultCode != ldap.ResultSuccess {
		t.Errorf("Handle() ResultCode = %v, want %v", result.ResultCode, ldap.ResultSuccess)
	}
}

// TestAddHandlerImpl_Handle_MultipleObjectClasses tests adding entry with multiple objectClasses.
func TestAddHandlerImpl_Handle_MultipleObjectClasses(t *testing.T) {
	mockBackend := newMockAddBackend()

	// Add parent entry
	parent := storage.NewEntry("ou=users,dc=example,dc=com")
	parent.SetStringAttribute("objectclass", "organizationalUnit")
	mockBackend.addEntry(parent)

	config := &AddConfig{
		Backend: mockBackend,
	}
	handler := NewAddHandler(config)

	req := &ldap.AddRequest{
		Entry: "uid=alice,ou=users,dc=example,dc=com",
		Attributes: []ldap.Attribute{
			{Type: "objectClass", Values: [][]byte{
				[]byte("top"),
				[]byte("person"),
				[]byte("organizationalPerson"),
				[]byte("inetOrgPerson"),
			}},
			{Type: "uid", Values: [][]byte{[]byte("alice")}},
			{Type: "cn", Values: [][]byte{[]byte("Alice")}},
			{Type: "sn", Values: [][]byte{[]byte("Smith")}},
		},
	}

	result := handler.Handle(nil, req)

	if result.ResultCode != ldap.ResultSuccess {
		t.Errorf("Handle() ResultCode = %v, want %v", result.ResultCode, ldap.ResultSuccess)
	}
}

// TestAddHandlerImpl_Handle_BackendGetError tests backend get error handling.
func TestAddHandlerImpl_Handle_BackendGetError(t *testing.T) {
	mockBackend := newMockAddBackend()
	mockBackend.getErr = errors.New("internal error")

	config := &AddConfig{
		Backend: mockBackend,
	}
	handler := NewAddHandler(config)

	req := &ldap.AddRequest{
		Entry: "uid=alice,ou=users,dc=example,dc=com",
		Attributes: []ldap.Attribute{
			{Type: "objectClass", Values: [][]byte{[]byte("person")}},
		},
	}

	result := handler.Handle(nil, req)

	if result.ResultCode != ldap.ResultOperationsError {
		t.Errorf("Handle() ResultCode = %v, want %v", result.ResultCode, ldap.ResultOperationsError)
	}
}

// TestAddHandlerImpl_Handle_BackendAddError tests backend add error handling.
func TestAddHandlerImpl_Handle_BackendAddError(t *testing.T) {
	mockBackend := newMockAddBackend()

	// Add parent entry
	parent := storage.NewEntry("ou=users,dc=example,dc=com")
	parent.SetStringAttribute("objectclass", "organizationalUnit")
	mockBackend.addEntry(parent)

	mockBackend.addErr = errors.New("internal error")

	config := &AddConfig{
		Backend: mockBackend,
	}
	handler := NewAddHandler(config)

	req := &ldap.AddRequest{
		Entry: "uid=alice,ou=users,dc=example,dc=com",
		Attributes: []ldap.Attribute{
			{Type: "objectClass", Values: [][]byte{[]byte("person")}},
		},
	}

	result := handler.Handle(nil, req)

	if result.ResultCode != ldap.ResultOperationsError {
		t.Errorf("Handle() ResultCode = %v, want %v", result.ResultCode, ldap.ResultOperationsError)
	}
}

// TestCreateAddHandler tests the CreateAddHandler function.
func TestCreateAddHandler(t *testing.T) {
	mockBackend := newMockAddBackend()

	// Add parent entry
	parent := storage.NewEntry("ou=users,dc=example,dc=com")
	parent.SetStringAttribute("objectclass", "organizationalUnit")
	mockBackend.addEntry(parent)

	config := &AddConfig{
		Backend: mockBackend,
	}
	impl := NewAddHandler(config)
	handler := CreateAddHandler(impl)

	req := &ldap.AddRequest{
		Entry: "uid=alice,ou=users,dc=example,dc=com",
		Attributes: []ldap.Attribute{
			{Type: "objectClass", Values: [][]byte{[]byte("person")}},
		},
	}

	result := handler(nil, req)

	if result.ResultCode != ldap.ResultSuccess {
		t.Errorf("handler() ResultCode = %v, want %v", result.ResultCode, ldap.ResultSuccess)
	}
}

// TestNewAddConfig tests the NewAddConfig function.
func TestNewAddConfig(t *testing.T) {
	config := NewAddConfig()

	if config == nil {
		t.Error("NewAddConfig() returned nil")
	}

	if config.Backend != nil {
		t.Error("NewAddConfig() should have nil Backend by default")
	}
}

// TestNewAddHandler_NilConfig tests NewAddHandler with nil config.
func TestNewAddHandler_NilConfig(t *testing.T) {
	handler := NewAddHandler(nil)

	if handler == nil {
		t.Error("NewAddHandler(nil) returned nil")
	}

	if handler.config == nil {
		t.Error("NewAddHandler(nil) should create default config")
	}
}

// TestValidateAddRequest tests the validateAddRequest function.
func TestValidateAddRequest(t *testing.T) {
	tests := []struct {
		name    string
		req     *ldap.AddRequest
		wantErr bool
	}{
		{
			name:    "nil request",
			req:     nil,
			wantErr: true,
		},
		{
			name:    "empty entry",
			req:     &ldap.AddRequest{Entry: ""},
			wantErr: true,
		},
		{
			name:    "valid request",
			req:     &ldap.AddRequest{Entry: "uid=alice,dc=example,dc=com"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAddRequest(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateAddRequest() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestHasObjectClassAttribute tests the hasObjectClassAttribute function.
func TestHasObjectClassAttribute(t *testing.T) {
	tests := []struct {
		name       string
		attributes []ldap.Attribute
		want       bool
	}{
		{
			name:       "no attributes",
			attributes: []ldap.Attribute{},
			want:       false,
		},
		{
			name: "has objectClass",
			attributes: []ldap.Attribute{
				{Type: "objectClass", Values: [][]byte{[]byte("person")}},
			},
			want: true,
		},
		{
			name: "has objectclass lowercase",
			attributes: []ldap.Attribute{
				{Type: "objectclass", Values: [][]byte{[]byte("person")}},
			},
			want: true,
		},
		{
			name: "has OBJECTCLASS uppercase",
			attributes: []ldap.Attribute{
				{Type: "OBJECTCLASS", Values: [][]byte{[]byte("person")}},
			},
			want: true,
		},
		{
			name: "objectClass with empty values",
			attributes: []ldap.Attribute{
				{Type: "objectClass", Values: [][]byte{}},
			},
			want: false,
		},
		{
			name: "no objectClass",
			attributes: []ldap.Attribute{
				{Type: "uid", Values: [][]byte{[]byte("alice")}},
				{Type: "cn", Values: [][]byte{[]byte("Alice")}},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &ldap.AddRequest{
				Entry:      "uid=test,dc=example,dc=com",
				Attributes: tt.attributes,
			}
			if got := hasObjectClassAttribute(req); got != tt.want {
				t.Errorf("hasObjectClassAttribute() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestConvertAddRequestToEntry tests the convertAddRequestToEntry function.
func TestConvertAddRequestToEntry(t *testing.T) {
	req := &ldap.AddRequest{
		Entry: "uid=alice,ou=users,dc=example,dc=com",
		Attributes: []ldap.Attribute{
			{Type: "objectClass", Values: [][]byte{[]byte("person"), []byte("top")}},
			{Type: "uid", Values: [][]byte{[]byte("alice")}},
			{Type: "cn", Values: [][]byte{[]byte("Alice")}},
		},
	}

	entry := convertAddRequestToEntry(req)

	if entry.DN != req.Entry {
		t.Errorf("DN = %q, want %q", entry.DN, req.Entry)
	}

	// Check objectclass (lowercase)
	objectClasses := entry.GetAttribute("objectclass")
	if len(objectClasses) != 2 {
		t.Errorf("objectclass count = %d, want 2", len(objectClasses))
	}

	// Check uid
	uids := entry.GetAttribute("uid")
	if len(uids) != 1 || string(uids[0]) != "alice" {
		t.Errorf("uid = %v, want [alice]", uids)
	}

	// Check cn
	cns := entry.GetAttribute("cn")
	if len(cns) != 1 || string(cns[0]) != "Alice" {
		t.Errorf("cn = %v, want [Alice]", cns)
	}
}

// TestMapAddError tests the mapAddError function.
func TestMapAddError(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		expectedCode ldap.ResultCode
	}{
		{
			name:         "nil error",
			err:          nil,
			expectedCode: ldap.ResultSuccess,
		},
		{
			name:         "entry already exists",
			err:          errors.New("entry already exists"),
			expectedCode: ldap.ResultEntryAlreadyExists,
		},
		{
			name:         "parent not found",
			err:          errors.New("parent entry not found"),
			expectedCode: ldap.ResultNoSuchObject,
		},
		{
			name:         "invalid DN",
			err:          errors.New("invalid DN syntax"),
			expectedCode: ldap.ResultInvalidDNSyntax,
		},
		{
			name:         "objectclass violation",
			err:          errors.New("objectclass required"),
			expectedCode: ldap.ResultObjectClassViolation,
		},
		{
			name:         "generic error",
			err:          errors.New("some other error"),
			expectedCode: ldap.ResultOperationsError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapAddError(tt.err, "uid=test,dc=example,dc=com")
			if result.ResultCode != tt.expectedCode {
				t.Errorf("mapAddError() ResultCode = %v, want %v", result.ResultCode, tt.expectedCode)
			}
		})
	}
}

// TestAddHandlerImpl_Handle_RootEntry tests adding a root entry.
func TestAddHandlerImpl_Handle_RootEntry(t *testing.T) {
	mockBackend := newMockAddBackend()

	config := &AddConfig{
		Backend: mockBackend,
	}
	handler := NewAddHandler(config)

	// Add a true root entry (single component DN)
	req := &ldap.AddRequest{
		Entry: "dc=com",
		Attributes: []ldap.Attribute{
			{Type: "objectClass", Values: [][]byte{[]byte("domain")}},
			{Type: "dc", Values: [][]byte{[]byte("com")}},
		},
	}

	result := handler.Handle(nil, req)

	if result.ResultCode != ldap.ResultSuccess {
		t.Errorf("Handle() ResultCode = %v, want %v", result.ResultCode, ldap.ResultSuccess)
	}

	// Verify entry is added
	addedEntry, _ := mockBackend.GetEntry("dc=com")
	if addedEntry == nil {
		t.Error("Root entry should exist after successful add operation")
	}
}
