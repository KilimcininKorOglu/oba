// Package server provides the LDAP server implementation.
package server

import (
	"errors"
	"strings"
	"testing"

	"github.com/KilimcininKorOglu/oba/internal/ldap"
	"github.com/KilimcininKorOglu/oba/internal/storage"
)

// mockModifyBackend implements the ModifyBackend interface for testing.
type mockModifyBackend struct {
	entries   map[string]*storage.Entry
	getErr    error
	modifyErr error
}

func newMockModifyBackend() *mockModifyBackend {
	return &mockModifyBackend{
		entries: make(map[string]*storage.Entry),
	}
}

func (m *mockModifyBackend) GetEntry(dn string) (*storage.Entry, error) {
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

func (m *mockModifyBackend) ModifyEntry(dn string, changes []Modification) error {
	if m.modifyErr != nil {
		return m.modifyErr
	}
	normalizedDN := normalizeDN(dn)
	for storedDN, entry := range m.entries {
		if normalizeDN(storedDN) == normalizedDN {
			// Apply modifications to the entry
			for _, mod := range changes {
				attrName := strings.ToLower(mod.Attribute)
				switch mod.Type {
				case ModifyAdd:
					for _, value := range mod.Values {
						entry.AddAttributeValue(attrName, []byte(value))
					}
				case ModifyDelete:
					if len(mod.Values) == 0 {
						// Delete entire attribute
						delete(entry.Attributes, attrName)
					} else {
						// Delete specific values
						for _, value := range mod.Values {
							deleteAttributeValue(entry, attrName, []byte(value))
						}
					}
				case ModifyReplace:
					if len(mod.Values) == 0 {
						// Replace with empty = delete
						delete(entry.Attributes, attrName)
					} else {
						entry.SetStringAttribute(attrName, mod.Values...)
					}
				}
			}
			return nil
		}
	}
	return errors.New("entry not found")
}

// deleteAttributeValue removes a specific value from an attribute.
func deleteAttributeValue(entry *storage.Entry, attrName string, value []byte) {
	values := entry.GetAttribute(attrName)
	if len(values) == 0 {
		return
	}
	newValues := make([][]byte, 0, len(values))
	for _, v := range values {
		if string(v) != string(value) {
			newValues = append(newValues, v)
		}
	}
	if len(newValues) == 0 {
		delete(entry.Attributes, attrName)
	} else {
		entry.SetAttribute(attrName, newValues)
	}
}

// getStringAttribute returns string values for an attribute.
func getStringAttribute(entry *storage.Entry, attrName string) []string {
	values := entry.GetAttribute(attrName)
	if len(values) == 0 {
		return nil
	}
	result := make([]string, len(values))
	for i, v := range values {
		result[i] = string(v)
	}
	return result
}

func (m *mockModifyBackend) addEntry(entry *storage.Entry) {
	m.entries[entry.DN] = entry
}

// TestModifyHandlerImpl_Handle tests the modify handler.
func TestModifyHandlerImpl_Handle(t *testing.T) {
	tests := []struct {
		name         string
		setupBackend func(*mockModifyBackend)
		dn           string
		changes      []ldap.Modification
		expectedCode ldap.ResultCode
	}{
		{
			name: "successful add modification",
			setupBackend: func(b *mockModifyBackend) {
				entry := storage.NewEntry("uid=alice,ou=users,dc=example,dc=com")
				entry.SetStringAttribute("uid", "alice")
				entry.SetStringAttribute("cn", "Alice")
				b.addEntry(entry)
			},
			dn: "uid=alice,ou=users,dc=example,dc=com",
			changes: []ldap.Modification{
				{
					Operation: ldap.ModifyOperationAdd,
					Attribute: ldap.Attribute{
						Type:   "mail",
						Values: [][]byte{[]byte("alice@example.com")},
					},
				},
			},
			expectedCode: ldap.ResultSuccess,
		},
		{
			name: "successful delete modification",
			setupBackend: func(b *mockModifyBackend) {
				entry := storage.NewEntry("uid=alice,ou=users,dc=example,dc=com")
				entry.SetStringAttribute("uid", "alice")
				entry.SetStringAttribute("mail", "alice@example.com", "alice2@example.com")
				b.addEntry(entry)
			},
			dn: "uid=alice,ou=users,dc=example,dc=com",
			changes: []ldap.Modification{
				{
					Operation: ldap.ModifyOperationDelete,
					Attribute: ldap.Attribute{
						Type:   "mail",
						Values: [][]byte{[]byte("alice2@example.com")},
					},
				},
			},
			expectedCode: ldap.ResultSuccess,
		},
		{
			name: "successful replace modification",
			setupBackend: func(b *mockModifyBackend) {
				entry := storage.NewEntry("uid=alice,ou=users,dc=example,dc=com")
				entry.SetStringAttribute("uid", "alice")
				entry.SetStringAttribute("cn", "Alice")
				b.addEntry(entry)
			},
			dn: "uid=alice,ou=users,dc=example,dc=com",
			changes: []ldap.Modification{
				{
					Operation: ldap.ModifyOperationReplace,
					Attribute: ldap.Attribute{
						Type:   "cn",
						Values: [][]byte{[]byte("Alice Smith")},
					},
				},
			},
			expectedCode: ldap.ResultSuccess,
		},
		{
			name: "entry not found",
			setupBackend: func(b *mockModifyBackend) {
				// No entries added
			},
			dn: "uid=nonexistent,ou=users,dc=example,dc=com",
			changes: []ldap.Modification{
				{
					Operation: ldap.ModifyOperationAdd,
					Attribute: ldap.Attribute{
						Type:   "mail",
						Values: [][]byte{[]byte("test@example.com")},
					},
				},
			},
			expectedCode: ldap.ResultNoSuchObject,
		},
		{
			name: "empty DN - protocol error",
			setupBackend: func(b *mockModifyBackend) {
				// No setup needed
			},
			dn: "",
			changes: []ldap.Modification{
				{
					Operation: ldap.ModifyOperationAdd,
					Attribute: ldap.Attribute{
						Type:   "mail",
						Values: [][]byte{[]byte("test@example.com")},
					},
				},
			},
			expectedCode: ldap.ResultProtocolError,
		},
		{
			name: "empty modifications - protocol error",
			setupBackend: func(b *mockModifyBackend) {
				entry := storage.NewEntry("uid=alice,ou=users,dc=example,dc=com")
				b.addEntry(entry)
			},
			dn:           "uid=alice,ou=users,dc=example,dc=com",
			changes:      []ldap.Modification{},
			expectedCode: ldap.ResultProtocolError,
		},
		{
			name: "backend get error",
			setupBackend: func(b *mockModifyBackend) {
				b.getErr = errors.New("internal error")
			},
			dn: "uid=alice,ou=users,dc=example,dc=com",
			changes: []ldap.Modification{
				{
					Operation: ldap.ModifyOperationAdd,
					Attribute: ldap.Attribute{
						Type:   "mail",
						Values: [][]byte{[]byte("test@example.com")},
					},
				},
			},
			expectedCode: ldap.ResultOperationsError,
		},
		{
			name: "backend modify error",
			setupBackend: func(b *mockModifyBackend) {
				entry := storage.NewEntry("uid=alice,ou=users,dc=example,dc=com")
				b.addEntry(entry)
				b.modifyErr = errors.New("internal error")
			},
			dn: "uid=alice,ou=users,dc=example,dc=com",
			changes: []ldap.Modification{
				{
					Operation: ldap.ModifyOperationAdd,
					Attribute: ldap.Attribute{
						Type:   "mail",
						Values: [][]byte{[]byte("test@example.com")},
					},
				},
			},
			expectedCode: ldap.ResultOperationsError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockBackend := newMockModifyBackend()
			if tt.setupBackend != nil {
				tt.setupBackend(mockBackend)
			}

			config := &ModifyConfig{
				Backend: mockBackend,
			}
			handler := NewModifyHandler(config)

			req := &ldap.ModifyRequest{
				Object:  tt.dn,
				Changes: tt.changes,
			}

			result := handler.Handle(nil, req)

			if result.ResultCode != tt.expectedCode {
				t.Errorf("Handle() ResultCode = %v, want %v", result.ResultCode, tt.expectedCode)
			}
		})
	}
}

// TestModifyHandlerImpl_Handle_NoBackend tests modify handler without backend.
func TestModifyHandlerImpl_Handle_NoBackend(t *testing.T) {
	config := &ModifyConfig{
		Backend: nil,
	}
	handler := NewModifyHandler(config)

	req := &ldap.ModifyRequest{
		Object: "uid=alice,ou=users,dc=example,dc=com",
		Changes: []ldap.Modification{
			{
				Operation: ldap.ModifyOperationAdd,
				Attribute: ldap.Attribute{
					Type:   "mail",
					Values: [][]byte{[]byte("test@example.com")},
				},
			},
		},
	}

	result := handler.Handle(nil, req)

	if result.ResultCode != ldap.ResultOperationsError {
		t.Errorf("Handle() ResultCode = %v, want %v", result.ResultCode, ldap.ResultOperationsError)
	}
}

// TestModifyHandlerImpl_Handle_AddAppendsValues verifies add operation appends values.
func TestModifyHandlerImpl_Handle_AddAppendsValues(t *testing.T) {
	mockBackend := newMockModifyBackend()
	entry := storage.NewEntry("uid=alice,ou=users,dc=example,dc=com")
	entry.SetStringAttribute("uid", "alice")
	entry.SetStringAttribute("mail", "alice@example.com")
	mockBackend.addEntry(entry)

	config := &ModifyConfig{
		Backend: mockBackend,
	}
	handler := NewModifyHandler(config)

	req := &ldap.ModifyRequest{
		Object: "uid=alice,ou=users,dc=example,dc=com",
		Changes: []ldap.Modification{
			{
				Operation: ldap.ModifyOperationAdd,
				Attribute: ldap.Attribute{
					Type:   "mail",
					Values: [][]byte{[]byte("alice2@example.com")},
				},
			},
		},
	}

	result := handler.Handle(nil, req)

	if result.ResultCode != ldap.ResultSuccess {
		t.Errorf("Handle() ResultCode = %v, want %v", result.ResultCode, ldap.ResultSuccess)
	}

	// Verify the value was appended
	modifiedEntry, _ := mockBackend.GetEntry("uid=alice,ou=users,dc=example,dc=com")
	if modifiedEntry == nil {
		t.Fatal("Entry should exist after modify")
	}

	mailValues := getStringAttribute(modifiedEntry, "mail")
	if len(mailValues) != 2 {
		t.Errorf("Expected 2 mail values, got %d", len(mailValues))
	}
}

// TestModifyHandlerImpl_Handle_DeleteRemovesValues verifies delete operation removes values.
func TestModifyHandlerImpl_Handle_DeleteRemovesValues(t *testing.T) {
	mockBackend := newMockModifyBackend()
	entry := storage.NewEntry("uid=alice,ou=users,dc=example,dc=com")
	entry.SetStringAttribute("uid", "alice")
	entry.SetStringAttribute("mail", "alice@example.com", "alice2@example.com")
	mockBackend.addEntry(entry)

	config := &ModifyConfig{
		Backend: mockBackend,
	}
	handler := NewModifyHandler(config)

	req := &ldap.ModifyRequest{
		Object: "uid=alice,ou=users,dc=example,dc=com",
		Changes: []ldap.Modification{
			{
				Operation: ldap.ModifyOperationDelete,
				Attribute: ldap.Attribute{
					Type:   "mail",
					Values: [][]byte{[]byte("alice2@example.com")},
				},
			},
		},
	}

	result := handler.Handle(nil, req)

	if result.ResultCode != ldap.ResultSuccess {
		t.Errorf("Handle() ResultCode = %v, want %v", result.ResultCode, ldap.ResultSuccess)
	}

	// Verify the value was removed
	modifiedEntry, _ := mockBackend.GetEntry("uid=alice,ou=users,dc=example,dc=com")
	if modifiedEntry == nil {
		t.Fatal("Entry should exist after modify")
	}

	mailValues := getStringAttribute(modifiedEntry, "mail")
	if len(mailValues) != 1 {
		t.Errorf("Expected 1 mail value, got %d", len(mailValues))
	}
}

// TestModifyHandlerImpl_Handle_ReplaceReplacesAllValues verifies replace operation.
func TestModifyHandlerImpl_Handle_ReplaceReplacesAllValues(t *testing.T) {
	mockBackend := newMockModifyBackend()
	entry := storage.NewEntry("uid=alice,ou=users,dc=example,dc=com")
	entry.SetStringAttribute("uid", "alice")
	entry.SetStringAttribute("cn", "Alice", "Alice Smith")
	mockBackend.addEntry(entry)

	config := &ModifyConfig{
		Backend: mockBackend,
	}
	handler := NewModifyHandler(config)

	req := &ldap.ModifyRequest{
		Object: "uid=alice,ou=users,dc=example,dc=com",
		Changes: []ldap.Modification{
			{
				Operation: ldap.ModifyOperationReplace,
				Attribute: ldap.Attribute{
					Type:   "cn",
					Values: [][]byte{[]byte("Alice Johnson")},
				},
			},
		},
	}

	result := handler.Handle(nil, req)

	if result.ResultCode != ldap.ResultSuccess {
		t.Errorf("Handle() ResultCode = %v, want %v", result.ResultCode, ldap.ResultSuccess)
	}

	// Verify the values were replaced
	modifiedEntry, _ := mockBackend.GetEntry("uid=alice,ou=users,dc=example,dc=com")
	if modifiedEntry == nil {
		t.Fatal("Entry should exist after modify")
	}

	cnValues := getStringAttribute(modifiedEntry, "cn")
	if len(cnValues) != 1 {
		t.Errorf("Expected 1 cn value, got %d", len(cnValues))
	}
	if cnValues[0] != "Alice Johnson" {
		t.Errorf("Expected cn value 'Alice Johnson', got '%s'", cnValues[0])
	}
}

// TestModifyHandlerImpl_Handle_MultipleModifications tests multiple modifications atomically.
func TestModifyHandlerImpl_Handle_MultipleModifications(t *testing.T) {
	mockBackend := newMockModifyBackend()
	entry := storage.NewEntry("uid=alice,ou=users,dc=example,dc=com")
	entry.SetStringAttribute("uid", "alice")
	entry.SetStringAttribute("cn", "Alice")
	entry.SetStringAttribute("mail", "old@example.com")
	mockBackend.addEntry(entry)

	config := &ModifyConfig{
		Backend: mockBackend,
	}
	handler := NewModifyHandler(config)

	req := &ldap.ModifyRequest{
		Object: "uid=alice,ou=users,dc=example,dc=com",
		Changes: []ldap.Modification{
			{
				Operation: ldap.ModifyOperationReplace,
				Attribute: ldap.Attribute{
					Type:   "cn",
					Values: [][]byte{[]byte("Alice Smith")},
				},
			},
			{
				Operation: ldap.ModifyOperationDelete,
				Attribute: ldap.Attribute{
					Type:   "mail",
					Values: [][]byte{},
				},
			},
			{
				Operation: ldap.ModifyOperationAdd,
				Attribute: ldap.Attribute{
					Type:   "telephoneNumber",
					Values: [][]byte{[]byte("+1-555-1234")},
				},
			},
		},
	}

	result := handler.Handle(nil, req)

	if result.ResultCode != ldap.ResultSuccess {
		t.Errorf("Handle() ResultCode = %v, want %v", result.ResultCode, ldap.ResultSuccess)
	}

	// Verify all modifications were applied
	modifiedEntry, _ := mockBackend.GetEntry("uid=alice,ou=users,dc=example,dc=com")
	if modifiedEntry == nil {
		t.Fatal("Entry should exist after modify")
	}

	// Check cn was replaced
	cnValues := getStringAttribute(modifiedEntry, "cn")
	if len(cnValues) != 1 || cnValues[0] != "Alice Smith" {
		t.Errorf("cn not replaced correctly: %v", cnValues)
	}

	// Check mail was deleted
	mailValues := getStringAttribute(modifiedEntry, "mail")
	if len(mailValues) != 0 {
		t.Errorf("mail should be deleted, got: %v", mailValues)
	}

	// Check telephoneNumber was added
	phoneValues := getStringAttribute(modifiedEntry, "telephonenumber")
	if len(phoneValues) != 1 || phoneValues[0] != "+1-555-1234" {
		t.Errorf("telephoneNumber not added correctly: %v", phoneValues)
	}
}

// TestModifyHandlerImpl_Handle_CaseInsensitiveDN tests case-insensitive DN handling.
func TestModifyHandlerImpl_Handle_CaseInsensitiveDN(t *testing.T) {
	mockBackend := newMockModifyBackend()
	entry := storage.NewEntry("uid=alice,ou=users,dc=example,dc=com")
	entry.SetStringAttribute("uid", "alice")
	mockBackend.addEntry(entry)

	config := &ModifyConfig{
		Backend: mockBackend,
	}
	handler := NewModifyHandler(config)

	// Use uppercase DN
	req := &ldap.ModifyRequest{
		Object: "UID=ALICE,OU=USERS,DC=EXAMPLE,DC=COM",
		Changes: []ldap.Modification{
			{
				Operation: ldap.ModifyOperationAdd,
				Attribute: ldap.Attribute{
					Type:   "mail",
					Values: [][]byte{[]byte("alice@example.com")},
				},
			},
		},
	}

	result := handler.Handle(nil, req)

	if result.ResultCode != ldap.ResultSuccess {
		t.Errorf("Handle() ResultCode = %v, want %v", result.ResultCode, ldap.ResultSuccess)
	}
}

// TestCreateModifyHandler tests the CreateModifyHandler function.
func TestCreateModifyHandler(t *testing.T) {
	mockBackend := newMockModifyBackend()
	entry := storage.NewEntry("uid=alice,ou=users,dc=example,dc=com")
	mockBackend.addEntry(entry)

	config := &ModifyConfig{
		Backend: mockBackend,
	}
	impl := NewModifyHandler(config)
	handler := CreateModifyHandler(impl)

	req := &ldap.ModifyRequest{
		Object: "uid=alice,ou=users,dc=example,dc=com",
		Changes: []ldap.Modification{
			{
				Operation: ldap.ModifyOperationAdd,
				Attribute: ldap.Attribute{
					Type:   "mail",
					Values: [][]byte{[]byte("test@example.com")},
				},
			},
		},
	}

	result := handler(nil, req)

	if result.ResultCode != ldap.ResultSuccess {
		t.Errorf("handler() ResultCode = %v, want %v", result.ResultCode, ldap.ResultSuccess)
	}
}

// TestNewModifyConfig tests the NewModifyConfig function.
func TestNewModifyConfig(t *testing.T) {
	config := NewModifyConfig()

	if config == nil {
		t.Error("NewModifyConfig() returned nil")
	}

	if config.Backend != nil {
		t.Error("NewModifyConfig() should have nil Backend by default")
	}
}

// TestNewModifyHandler_NilConfig tests NewModifyHandler with nil config.
func TestNewModifyHandler_NilConfig(t *testing.T) {
	handler := NewModifyHandler(nil)

	if handler == nil {
		t.Error("NewModifyHandler(nil) returned nil")
	}

	if handler.config == nil {
		t.Error("NewModifyHandler(nil) should create default config")
	}
}

// TestModifyHandlerImpl_Handle_DeleteEntireAttribute tests deleting an entire attribute.
func TestModifyHandlerImpl_Handle_DeleteEntireAttribute(t *testing.T) {
	mockBackend := newMockModifyBackend()
	entry := storage.NewEntry("uid=alice,ou=users,dc=example,dc=com")
	entry.SetStringAttribute("uid", "alice")
	entry.SetStringAttribute("description", "Test user", "Another description")
	mockBackend.addEntry(entry)

	config := &ModifyConfig{
		Backend: mockBackend,
	}
	handler := NewModifyHandler(config)

	req := &ldap.ModifyRequest{
		Object: "uid=alice,ou=users,dc=example,dc=com",
		Changes: []ldap.Modification{
			{
				Operation: ldap.ModifyOperationDelete,
				Attribute: ldap.Attribute{
					Type:   "description",
					Values: [][]byte{}, // Empty values means delete entire attribute
				},
			},
		},
	}

	result := handler.Handle(nil, req)

	if result.ResultCode != ldap.ResultSuccess {
		t.Errorf("Handle() ResultCode = %v, want %v", result.ResultCode, ldap.ResultSuccess)
	}

	// Verify the attribute was deleted
	modifiedEntry, _ := mockBackend.GetEntry("uid=alice,ou=users,dc=example,dc=com")
	if modifiedEntry == nil {
		t.Fatal("Entry should exist after modify")
	}

	descValues := getStringAttribute(modifiedEntry, "description")
	if len(descValues) != 0 {
		t.Errorf("Expected description to be deleted, got: %v", descValues)
	}
}

// TestModifyHandlerImpl_Handle_ReplaceWithEmpty tests replacing with empty values (delete).
func TestModifyHandlerImpl_Handle_ReplaceWithEmpty(t *testing.T) {
	mockBackend := newMockModifyBackend()
	entry := storage.NewEntry("uid=alice,ou=users,dc=example,dc=com")
	entry.SetStringAttribute("uid", "alice")
	entry.SetStringAttribute("description", "Test user")
	mockBackend.addEntry(entry)

	config := &ModifyConfig{
		Backend: mockBackend,
	}
	handler := NewModifyHandler(config)

	req := &ldap.ModifyRequest{
		Object: "uid=alice,ou=users,dc=example,dc=com",
		Changes: []ldap.Modification{
			{
				Operation: ldap.ModifyOperationReplace,
				Attribute: ldap.Attribute{
					Type:   "description",
					Values: [][]byte{}, // Empty values means delete
				},
			},
		},
	}

	result := handler.Handle(nil, req)

	if result.ResultCode != ldap.ResultSuccess {
		t.Errorf("Handle() ResultCode = %v, want %v", result.ResultCode, ldap.ResultSuccess)
	}

	// Verify the attribute was deleted
	modifiedEntry, _ := mockBackend.GetEntry("uid=alice,ou=users,dc=example,dc=com")
	if modifiedEntry == nil {
		t.Fatal("Entry should exist after modify")
	}

	descValues := getStringAttribute(modifiedEntry, "description")
	if len(descValues) != 0 {
		t.Errorf("Expected description to be deleted, got: %v", descValues)
	}
}

// TestConvertToBackendModifications tests the conversion function.
func TestConvertToBackendModifications(t *testing.T) {
	changes := []ldap.Modification{
		{
			Operation: ldap.ModifyOperationAdd,
			Attribute: ldap.Attribute{
				Type:   "mail",
				Values: [][]byte{[]byte("test@example.com")},
			},
		},
		{
			Operation: ldap.ModifyOperationDelete,
			Attribute: ldap.Attribute{
				Type:   "description",
				Values: [][]byte{[]byte("old description")},
			},
		},
		{
			Operation: ldap.ModifyOperationReplace,
			Attribute: ldap.Attribute{
				Type:   "cn",
				Values: [][]byte{[]byte("New Name")},
			},
		},
	}

	result := convertToBackendModifications(changes)

	if len(result) != 3 {
		t.Fatalf("Expected 3 modifications, got %d", len(result))
	}

	// Check add modification
	if result[0].Type != ModifyAdd {
		t.Errorf("Expected ModifyAdd, got %v", result[0].Type)
	}
	if result[0].Attribute != "mail" {
		t.Errorf("Expected attribute 'mail', got '%s'", result[0].Attribute)
	}
	if len(result[0].Values) != 1 || result[0].Values[0] != "test@example.com" {
		t.Errorf("Unexpected values: %v", result[0].Values)
	}

	// Check delete modification
	if result[1].Type != ModifyDelete {
		t.Errorf("Expected ModifyDelete, got %v", result[1].Type)
	}

	// Check replace modification
	if result[2].Type != ModifyReplace {
		t.Errorf("Expected ModifyReplace, got %v", result[2].Type)
	}
}

// TestModifyHandlerImpl_mapError tests error mapping.
func TestModifyHandlerImpl_mapError(t *testing.T) {
	handler := NewModifyHandler(nil)

	tests := []struct {
		name         string
		err          error
		expectedCode ldap.ResultCode
	}{
		{
			name:         "not found error",
			err:          errors.New("entry not found"),
			expectedCode: ldap.ResultNoSuchObject,
		},
		{
			name:         "invalid error",
			err:          errors.New("invalid modification"),
			expectedCode: ldap.ResultConstraintViolation,
		},
		{
			name:         "schema error",
			err:          errors.New("schema violation"),
			expectedCode: ldap.ResultObjectClassViolation,
		},
		{
			name:         "objectclass error",
			err:          errors.New("objectclass violation"),
			expectedCode: ldap.ResultObjectClassViolation,
		},
		{
			name:         "required attribute error",
			err:          errors.New("attribute cn is required"),
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
			result := handler.mapError(tt.err, "uid=test,dc=example,dc=com")
			if result.ResultCode != tt.expectedCode {
				t.Errorf("mapError() ResultCode = %v, want %v", result.ResultCode, tt.expectedCode)
			}
		})
	}
}
