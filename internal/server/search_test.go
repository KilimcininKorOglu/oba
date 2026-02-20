// Package server provides the LDAP server implementation.
package server

import (
	"testing"

	"github.com/oba-ldap/oba/internal/ldap"
	"github.com/oba-ldap/oba/internal/storage"
)

// Note: mockBackend is defined in bind_test.go and reused here.

// TestSearchHandlerImpl_Handle_BaseScope tests base scope search.
func TestSearchHandlerImpl_Handle_BaseScope(t *testing.T) {
	backend := newMockBackend()

	// Add test entry
	entry := storage.NewEntry("uid=alice,ou=users,dc=example,dc=com")
	entry.SetStringAttribute("uid", "alice")
	entry.SetStringAttribute("cn", "Alice Smith")
	entry.SetStringAttribute("mail", "alice@example.com")
	entry.SetStringAttribute("objectClass", "inetOrgPerson", "person", "top")
	backend.addEntry(entry)

	config := &SearchConfig{
		Backend: backend,
	}
	handler := NewSearchHandler(config)

	tests := []struct {
		name           string
		baseDN         string
		filter         *ldap.SearchFilter
		attributes     []string
		typesOnly      bool
		expectedCode   ldap.ResultCode
		expectedCount  int
		checkEntry     func(t *testing.T, entries []*SearchEntry)
	}{
		{
			name:          "base scope - entry exists, no filter",
			baseDN:        "uid=alice,ou=users,dc=example,dc=com",
			filter:        nil,
			attributes:    nil,
			typesOnly:     false,
			expectedCode:  ldap.ResultSuccess,
			expectedCount: 1,
			checkEntry: func(t *testing.T, entries []*SearchEntry) {
				if entries[0].DN != "uid=alice,ou=users,dc=example,dc=com" {
					t.Errorf("Expected DN uid=alice,ou=users,dc=example,dc=com, got %s", entries[0].DN)
				}
			},
		},
		{
			name:          "base scope - entry not found",
			baseDN:        "uid=bob,ou=users,dc=example,dc=com",
			filter:        nil,
			attributes:    nil,
			typesOnly:     false,
			expectedCode:  ldap.ResultNoSuchObject,
			expectedCount: 0,
		},
		{
			name:   "base scope - equality filter matches",
			baseDN: "uid=alice,ou=users,dc=example,dc=com",
			filter: &ldap.SearchFilter{
				Type:      ldap.FilterTagEquality,
				Attribute: "uid",
				Value:     []byte("alice"),
			},
			attributes:    nil,
			typesOnly:     false,
			expectedCode:  ldap.ResultSuccess,
			expectedCount: 1,
		},
		{
			name:   "base scope - equality filter does not match",
			baseDN: "uid=alice,ou=users,dc=example,dc=com",
			filter: &ldap.SearchFilter{
				Type:      ldap.FilterTagEquality,
				Attribute: "uid",
				Value:     []byte("bob"),
			},
			attributes:    nil,
			typesOnly:     false,
			expectedCode:  ldap.ResultSuccess,
			expectedCount: 0,
		},
		{
			name:   "base scope - presence filter matches",
			baseDN: "uid=alice,ou=users,dc=example,dc=com",
			filter: &ldap.SearchFilter{
				Type:      ldap.FilterTagPresent,
				Attribute: "mail",
			},
			attributes:    nil,
			typesOnly:     false,
			expectedCode:  ldap.ResultSuccess,
			expectedCount: 1,
		},
		{
			name:   "base scope - presence filter does not match",
			baseDN: "uid=alice,ou=users,dc=example,dc=com",
			filter: &ldap.SearchFilter{
				Type:      ldap.FilterTagPresent,
				Attribute: "telephoneNumber",
			},
			attributes:    nil,
			typesOnly:     false,
			expectedCode:  ldap.ResultSuccess,
			expectedCount: 0,
		},
		{
			name:          "base scope - specific attributes",
			baseDN:        "uid=alice,ou=users,dc=example,dc=com",
			filter:        nil,
			attributes:    []string{"uid", "cn"},
			typesOnly:     false,
			expectedCode:  ldap.ResultSuccess,
			expectedCount: 1,
			checkEntry: func(t *testing.T, entries []*SearchEntry) {
				if len(entries[0].Attributes) != 2 {
					t.Errorf("Expected 2 attributes, got %d", len(entries[0].Attributes))
				}
			},
		},
		{
			name:          "base scope - typesOnly",
			baseDN:        "uid=alice,ou=users,dc=example,dc=com",
			filter:        nil,
			attributes:    []string{"uid"},
			typesOnly:     true,
			expectedCode:  ldap.ResultSuccess,
			expectedCount: 1,
			checkEntry: func(t *testing.T, entries []*SearchEntry) {
				for _, attr := range entries[0].Attributes {
					if len(attr.Values) > 0 {
						t.Errorf("Expected no values for typesOnly, got %d values for %s", len(attr.Values), attr.Type)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &ldap.SearchRequest{
				BaseObject: tt.baseDN,
				Scope:      ldap.ScopeBaseObject,
				Filter:     tt.filter,
				Attributes: tt.attributes,
				TypesOnly:  tt.typesOnly,
			}

			result := handler.Handle(nil, req)

			if result.ResultCode != tt.expectedCode {
				t.Errorf("Expected result code %v, got %v", tt.expectedCode, result.ResultCode)
			}

			if len(result.Entries) != tt.expectedCount {
				t.Errorf("Expected %d entries, got %d", tt.expectedCount, len(result.Entries))
			}

			if tt.checkEntry != nil && len(result.Entries) > 0 {
				tt.checkEntry(t, result.Entries)
			}
		})
	}
}

// TestSearchHandlerImpl_Handle_NoBackend tests search without backend.
func TestSearchHandlerImpl_Handle_NoBackend(t *testing.T) {
	config := &SearchConfig{
		Backend: nil,
	}
	handler := NewSearchHandler(config)

	req := &ldap.SearchRequest{
		BaseObject: "dc=example,dc=com",
		Scope:      ldap.ScopeBaseObject,
	}

	result := handler.Handle(nil, req)

	if result.ResultCode != ldap.ResultOperationsError {
		t.Errorf("Expected ResultOperationsError, got %v", result.ResultCode)
	}
}

// TestSearchHandlerImpl_Handle_UnsupportedScope tests unsupported search scopes.
func TestSearchHandlerImpl_Handle_UnsupportedScope(t *testing.T) {
	backend := newMockBackend()
	config := &SearchConfig{
		Backend: backend,
	}
	handler := NewSearchHandler(config)

	tests := []struct {
		name  string
		scope ldap.SearchScope
	}{
		{"single level scope", ldap.ScopeSingleLevel},
		{"subtree scope", ldap.ScopeWholeSubtree},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &ldap.SearchRequest{
				BaseObject: "dc=example,dc=com",
				Scope:      tt.scope,
			}

			result := handler.Handle(nil, req)

			if result.ResultCode != ldap.ResultUnwillingToPerform {
				t.Errorf("Expected ResultUnwillingToPerform, got %v", result.ResultCode)
			}
		})
	}
}

// TestBaseSearcher_Search tests the BaseSearcher directly.
func TestBaseSearcher_Search(t *testing.T) {
	backend := newMockBackend()

	// Add test entry
	entry := storage.NewEntry("cn=test,dc=example,dc=com")
	entry.SetStringAttribute("cn", "test")
	entry.SetStringAttribute("description", "Test entry")
	backend.addEntry(entry)

	searcher := NewBaseSearcher(backend)

	tests := []struct {
		name          string
		baseDN        string
		filter        *ldap.SearchFilter
		expectedCode  ldap.ResultCode
		expectedCount int
	}{
		{
			name:          "entry exists",
			baseDN:        "cn=test,dc=example,dc=com",
			filter:        nil,
			expectedCode:  ldap.ResultSuccess,
			expectedCount: 1,
		},
		{
			name:          "entry not found",
			baseDN:        "cn=notfound,dc=example,dc=com",
			filter:        nil,
			expectedCode:  ldap.ResultNoSuchObject,
			expectedCount: 0,
		},
		{
			name:   "filter matches",
			baseDN: "cn=test,dc=example,dc=com",
			filter: &ldap.SearchFilter{
				Type:      ldap.FilterTagEquality,
				Attribute: "cn",
				Value:     []byte("test"),
			},
			expectedCode:  ldap.ResultSuccess,
			expectedCount: 1,
		},
		{
			name:   "filter does not match",
			baseDN: "cn=test,dc=example,dc=com",
			filter: &ldap.SearchFilter{
				Type:      ldap.FilterTagEquality,
				Attribute: "cn",
				Value:     []byte("other"),
			},
			expectedCode:  ldap.ResultSuccess,
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &ldap.SearchRequest{
				BaseObject: tt.baseDN,
				Scope:      ldap.ScopeBaseObject,
				Filter:     tt.filter,
			}

			result := searcher.Search(req)

			if result.ResultCode != tt.expectedCode {
				t.Errorf("Expected result code %v, got %v", tt.expectedCode, result.ResultCode)
			}

			if len(result.Entries) != tt.expectedCount {
				t.Errorf("Expected %d entries, got %d", tt.expectedCount, len(result.Entries))
			}
		})
	}
}

// TestBaseSearcher_ComplexFilters tests complex filter evaluation.
func TestBaseSearcher_ComplexFilters(t *testing.T) {
	backend := newMockBackend()

	// Add test entry
	entry := storage.NewEntry("uid=alice,ou=users,dc=example,dc=com")
	entry.SetStringAttribute("uid", "alice")
	entry.SetStringAttribute("cn", "Alice Smith")
	entry.SetStringAttribute("mail", "alice@example.com")
	entry.SetStringAttribute("objectClass", "inetOrgPerson", "person", "top")
	backend.addEntry(entry)

	searcher := NewBaseSearcher(backend)

	tests := []struct {
		name          string
		filter        *ldap.SearchFilter
		expectedMatch bool
	}{
		{
			name: "AND filter - all match",
			filter: &ldap.SearchFilter{
				Type: ldap.FilterTagAnd,
				Children: []*ldap.SearchFilter{
					{Type: ldap.FilterTagEquality, Attribute: "uid", Value: []byte("alice")},
					{Type: ldap.FilterTagPresent, Attribute: "mail"},
				},
			},
			expectedMatch: true,
		},
		{
			name: "AND filter - one doesn't match",
			filter: &ldap.SearchFilter{
				Type: ldap.FilterTagAnd,
				Children: []*ldap.SearchFilter{
					{Type: ldap.FilterTagEquality, Attribute: "uid", Value: []byte("alice")},
					{Type: ldap.FilterTagEquality, Attribute: "cn", Value: []byte("Bob")},
				},
			},
			expectedMatch: false,
		},
		{
			name: "OR filter - one matches",
			filter: &ldap.SearchFilter{
				Type: ldap.FilterTagOr,
				Children: []*ldap.SearchFilter{
					{Type: ldap.FilterTagEquality, Attribute: "uid", Value: []byte("bob")},
					{Type: ldap.FilterTagEquality, Attribute: "uid", Value: []byte("alice")},
				},
			},
			expectedMatch: true,
		},
		{
			name: "OR filter - none match",
			filter: &ldap.SearchFilter{
				Type: ldap.FilterTagOr,
				Children: []*ldap.SearchFilter{
					{Type: ldap.FilterTagEquality, Attribute: "uid", Value: []byte("bob")},
					{Type: ldap.FilterTagEquality, Attribute: "uid", Value: []byte("charlie")},
				},
			},
			expectedMatch: false,
		},
		{
			name: "NOT filter - negates match",
			filter: &ldap.SearchFilter{
				Type: ldap.FilterTagNot,
				Child: &ldap.SearchFilter{
					Type:      ldap.FilterTagEquality,
					Attribute: "uid",
					Value:     []byte("bob"),
				},
			},
			expectedMatch: true,
		},
		{
			name: "NOT filter - negates non-match",
			filter: &ldap.SearchFilter{
				Type: ldap.FilterTagNot,
				Child: &ldap.SearchFilter{
					Type:      ldap.FilterTagEquality,
					Attribute: "uid",
					Value:     []byte("alice"),
				},
			},
			expectedMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &ldap.SearchRequest{
				BaseObject: "uid=alice,ou=users,dc=example,dc=com",
				Scope:      ldap.ScopeBaseObject,
				Filter:     tt.filter,
			}

			result := searcher.Search(req)

			if result.ResultCode != ldap.ResultSuccess {
				t.Errorf("Expected ResultSuccess, got %v", result.ResultCode)
			}

			hasEntries := len(result.Entries) > 0
			if hasEntries != tt.expectedMatch {
				t.Errorf("Expected match=%v, got match=%v", tt.expectedMatch, hasEntries)
			}
		})
	}
}

// TestAttributeSelector tests attribute selection functionality.
func TestAttributeSelector(t *testing.T) {
	// Create test entry with user and operational attributes
	entry := storage.NewEntry("uid=test,dc=example,dc=com")
	entry.SetStringAttribute("uid", "test")
	entry.SetStringAttribute("cn", "Test User")
	entry.SetStringAttribute("mail", "test@example.com")
	entry.SetStringAttribute("createTimestamp", "20240101000000Z")
	entry.SetStringAttribute("modifyTimestamp", "20240102000000Z")

	tests := []struct {
		name           string
		requestedAttrs []string
		expectedAttrs  []string
		notExpected    []string
	}{
		{
			name:           "no attributes - return all user attributes",
			requestedAttrs: nil,
			expectedAttrs:  []string{"uid", "cn", "mail"},
			notExpected:    []string{"createTimestamp", "modifyTimestamp"},
		},
		{
			name:           "empty attributes - return all user attributes",
			requestedAttrs: []string{},
			expectedAttrs:  []string{"uid", "cn", "mail"},
			notExpected:    []string{"createTimestamp", "modifyTimestamp"},
		},
		{
			name:           "specific attributes",
			requestedAttrs: []string{"uid", "cn"},
			expectedAttrs:  []string{"uid", "cn"},
			notExpected:    []string{"mail", "createTimestamp"},
		},
		{
			name:           "all user attributes with *",
			requestedAttrs: []string{"*"},
			expectedAttrs:  []string{"uid", "cn", "mail"},
			notExpected:    []string{"createTimestamp", "modifyTimestamp"},
		},
		{
			name:           "all operational attributes with +",
			requestedAttrs: []string{"+"},
			expectedAttrs:  []string{"createTimestamp", "modifyTimestamp"},
			notExpected:    []string{"uid", "cn", "mail"},
		},
		{
			name:           "both user and operational with * and +",
			requestedAttrs: []string{"*", "+"},
			expectedAttrs:  []string{"uid", "cn", "mail", "createTimestamp", "modifyTimestamp"},
			notExpected:    nil,
		},
		{
			name:           "specific and all user",
			requestedAttrs: []string{"*", "createTimestamp"},
			expectedAttrs:  []string{"uid", "cn", "mail", "createTimestamp"},
			notExpected:    []string{"modifyTimestamp"},
		},
		{
			name:           "1.1 - no attributes",
			requestedAttrs: []string{"1.1"},
			expectedAttrs:  nil,
			notExpected:    []string{"uid", "cn", "mail", "createTimestamp"},
		},
		{
			name:           "case insensitive attribute names",
			requestedAttrs: []string{"UID", "CN"},
			expectedAttrs:  []string{"uid", "cn"},
			notExpected:    []string{"mail"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selector := NewAttributeSelector(tt.requestedAttrs)
			result := selector.Select(entry)

			// Check expected attributes are present
			for _, attr := range tt.expectedAttrs {
				found := false
				for name := range result {
					if NormalizeAttributeName(name) == NormalizeAttributeName(attr) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected attribute %s not found in result", attr)
				}
			}

			// Check not expected attributes are absent
			for _, attr := range tt.notExpected {
				for name := range result {
					if NormalizeAttributeName(name) == NormalizeAttributeName(attr) {
						t.Errorf("Unexpected attribute %s found in result", attr)
					}
				}
			}
		})
	}
}

// TestSelectAttributes tests the SelectAttributes convenience function.
func TestSelectAttributes(t *testing.T) {
	entry := storage.NewEntry("uid=test,dc=example,dc=com")
	entry.SetStringAttribute("uid", "test")
	entry.SetStringAttribute("cn", "Test User")

	// Test with specific attributes
	result := SelectAttributes(entry, []string{"uid"})
	if len(result) != 1 {
		t.Errorf("Expected 1 attribute, got %d", len(result))
	}

	// Test with nil entry
	result = SelectAttributes(nil, []string{"uid"})
	if len(result) != 0 {
		t.Errorf("Expected 0 attributes for nil entry, got %d", len(result))
	}
}

// TestIsOperationalAttribute tests operational attribute detection.
func TestIsOperationalAttribute(t *testing.T) {
	tests := []struct {
		name     string
		attr     string
		expected bool
	}{
		{"createTimestamp", "createTimestamp", true},
		{"modifyTimestamp", "modifyTimestamp", true},
		{"creatorsName", "creatorsName", true},
		{"modifiersName", "modifiersName", true},
		{"entryDN", "entryDN", true},
		{"entryUUID", "entryUUID", true},
		{"uid - not operational", "uid", false},
		{"cn - not operational", "cn", false},
		{"mail - not operational", "mail", false},
		{"objectClass - not operational", "objectClass", false},
		{"case insensitive - CREATETIMESTAMP", "CREATETIMESTAMP", true},
		{"case insensitive - CreateTimestamp", "CreateTimestamp", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsOperationalAttribute(tt.attr)
			if result != tt.expected {
				t.Errorf("IsOperationalAttribute(%s) = %v, expected %v", tt.attr, result, tt.expected)
			}
		})
	}
}

// TestNormalizeAttributeName tests attribute name normalization.
func TestNormalizeAttributeName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"uid", "uid"},
		{"UID", "uid"},
		{"Uid", "uid"},
		{"  uid  ", "uid"},
		{"CN", "cn"},
		{"objectClass", "objectclass"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := NormalizeAttributeName(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeAttributeName(%s) = %s, expected %s", tt.input, result, tt.expected)
			}
		})
	}
}

// TestAttributeExists tests attribute existence checking.
func TestAttributeExists(t *testing.T) {
	entry := storage.NewEntry("uid=test,dc=example,dc=com")
	entry.SetStringAttribute("uid", "test")
	entry.SetStringAttribute("cn", "Test User")

	tests := []struct {
		name     string
		attr     string
		expected bool
	}{
		{"existing attribute", "uid", true},
		{"existing attribute case insensitive", "UID", true},
		{"non-existing attribute", "mail", false},
		{"empty attribute name", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AttributeExists(entry, tt.attr)
			if result != tt.expected {
				t.Errorf("AttributeExists(entry, %s) = %v, expected %v", tt.attr, result, tt.expected)
			}
		})
	}

	// Test with nil entry
	if AttributeExists(nil, "uid") {
		t.Error("AttributeExists(nil, uid) should return false")
	}
}

// TestGetAttributeValues tests attribute value retrieval.
func TestGetAttributeValues(t *testing.T) {
	entry := storage.NewEntry("uid=test,dc=example,dc=com")
	entry.SetStringAttribute("uid", "test")
	entry.SetStringAttribute("objectClass", "person", "top")

	// Test existing attribute
	values := GetAttributeValues(entry, "uid")
	if len(values) != 1 || string(values[0]) != "test" {
		t.Errorf("Expected [test], got %v", values)
	}

	// Test case insensitive
	values = GetAttributeValues(entry, "UID")
	if len(values) != 1 || string(values[0]) != "test" {
		t.Errorf("Expected [test] for UID, got %v", values)
	}

	// Test multi-valued attribute
	values = GetAttributeValues(entry, "objectClass")
	if len(values) != 2 {
		t.Errorf("Expected 2 values for objectClass, got %d", len(values))
	}

	// Test non-existing attribute
	values = GetAttributeValues(entry, "mail")
	if values != nil {
		t.Errorf("Expected nil for non-existing attribute, got %v", values)
	}

	// Test nil entry
	values = GetAttributeValues(nil, "uid")
	if values != nil {
		t.Errorf("Expected nil for nil entry, got %v", values)
	}
}

// TestFilterAttributeValues tests the typesOnly filtering.
func TestFilterAttributeValues(t *testing.T) {
	values := [][]byte{[]byte("value1"), []byte("value2")}

	// typesOnly = false should return values
	result := FilterAttributeValues(values, false)
	if len(result) != 2 {
		t.Errorf("Expected 2 values when typesOnly=false, got %d", len(result))
	}

	// typesOnly = true should return nil
	result = FilterAttributeValues(values, true)
	if result != nil {
		t.Errorf("Expected nil when typesOnly=true, got %v", result)
	}
}

// TestCreateSearchHandler tests the CreateSearchHandler function.
func TestCreateSearchHandler(t *testing.T) {
	backend := newMockBackend()
	entry := storage.NewEntry("dc=example,dc=com")
	entry.SetStringAttribute("dc", "example")
	backend.addEntry(entry)

	config := &SearchConfig{Backend: backend}
	impl := NewSearchHandler(config)
	handler := CreateSearchHandler(impl)

	req := &ldap.SearchRequest{
		BaseObject: "dc=example,dc=com",
		Scope:      ldap.ScopeBaseObject,
	}

	result := handler(nil, req)
	if result.ResultCode != ldap.ResultSuccess {
		t.Errorf("Expected ResultSuccess, got %v", result.ResultCode)
	}
	if len(result.Entries) != 1 {
		t.Errorf("Expected 1 entry, got %d", len(result.Entries))
	}
}

// TestNewSearchConfig tests default search configuration.
func TestNewSearchConfig(t *testing.T) {
	config := NewSearchConfig()

	if config.MaxSizeLimit != 1000 {
		t.Errorf("Expected MaxSizeLimit=1000, got %d", config.MaxSizeLimit)
	}
	if config.MaxTimeLimit != 60 {
		t.Errorf("Expected MaxTimeLimit=60, got %d", config.MaxTimeLimit)
	}
	if config.DefaultSizeLimit != 100 {
		t.Errorf("Expected DefaultSizeLimit=100, got %d", config.DefaultSizeLimit)
	}
	if config.DefaultTimeLimit != 30 {
		t.Errorf("Expected DefaultTimeLimit=30, got %d", config.DefaultTimeLimit)
	}
}
