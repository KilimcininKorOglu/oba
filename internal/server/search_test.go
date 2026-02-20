// Package server provides the LDAP server implementation.
package server

import (
	"strings"
	"testing"

	"github.com/oba-ldap/oba/internal/ldap"
	"github.com/oba-ldap/oba/internal/storage"
)

// Note: mockBackend is defined in bind_test.go and reused here.

// mockSearchBackend implements the SearchBackend interface for testing.
type mockSearchBackend struct {
	entries map[string]*storage.Entry
	err     error
}

func newMockSearchBackend() *mockSearchBackend {
	return &mockSearchBackend{
		entries: make(map[string]*storage.Entry),
	}
}

func (m *mockSearchBackend) GetEntry(dn string) (*storage.Entry, error) {
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

func (m *mockSearchBackend) addEntry(entry *storage.Entry) {
	m.entries[entry.DN] = entry
}

// SearchByDN implements SearchBackend.SearchByDN for testing.
func (m *mockSearchBackend) SearchByDN(baseDN string, scope storage.Scope) storage.Iterator {
	normalizedBaseDN := normalizeDN(baseDN)
	
	var entries []*storage.Entry
	
	switch scope {
	case storage.ScopeBase:
		// Return only the base entry
		for storedDN, entry := range m.entries {
			if normalizeDN(storedDN) == normalizedBaseDN {
				entries = append(entries, entry)
				break
			}
		}
	case storage.ScopeOneLevel:
		// Return immediate children of the base DN
		for storedDN, entry := range m.entries {
			if isImmediateChild(normalizedBaseDN, normalizeDN(storedDN)) {
				entries = append(entries, entry)
			}
		}
	case storage.ScopeSubtree:
		// Return base and all descendants
		for storedDN, entry := range m.entries {
			normalizedStoredDN := normalizeDN(storedDN)
			if normalizedStoredDN == normalizedBaseDN || isDescendant(normalizedBaseDN, normalizedStoredDN) {
				entries = append(entries, entry)
			}
		}
	}
	
	return &mockIterator{entries: entries, index: -1}
}

// isImmediateChild checks if childDN is an immediate child of parentDN.
func isImmediateChild(parentDN, childDN string) bool {
	if parentDN == "" {
		// Root - check if childDN has only one component
		parts := strings.Split(childDN, ",")
		return len(parts) == 1 && childDN != ""
	}
	
	// childDN should end with ",parentDN" and have exactly one more component
	suffix := "," + parentDN
	if !strings.HasSuffix(childDN, suffix) {
		return false
	}
	
	// Get the prefix (the part before the parent DN)
	prefix := strings.TrimSuffix(childDN, suffix)
	
	// The prefix should not contain any commas (single component)
	return prefix != "" && !strings.Contains(prefix, ",")
}

// isDescendant checks if childDN is a descendant of parentDN.
func isDescendant(parentDN, childDN string) bool {
	if parentDN == "" {
		return childDN != ""
	}
	
	suffix := "," + parentDN
	return strings.HasSuffix(childDN, suffix)
}

// mockIterator implements storage.Iterator for testing.
type mockIterator struct {
	entries []*storage.Entry
	index   int
	err     error
}

func (it *mockIterator) Next() bool {
	it.index++
	return it.index < len(it.entries)
}

func (it *mockIterator) Entry() *storage.Entry {
	if it.index < 0 || it.index >= len(it.entries) {
		return nil
	}
	return it.entries[it.index]
}

func (it *mockIterator) Error() error {
	return it.err
}

func (it *mockIterator) Close() {
	// No-op for mock
}

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
// When SearchBackend is not configured, OneLevel and Subtree should return UnwillingToPerform.
func TestSearchHandlerImpl_Handle_UnsupportedScope(t *testing.T) {
	backend := newMockBackend()
	config := &SearchConfig{
		Backend: backend,
		// SearchBackend is nil, so OneLevel and Subtree are not supported
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

// TestSearchHandlerImpl_Handle_OneLevelScope tests one-level scope search.
func TestSearchHandlerImpl_Handle_OneLevelScope(t *testing.T) {
	backend := newMockSearchBackend()

	// Create a directory structure:
	// dc=example,dc=com (base)
	//   ou=users,dc=example,dc=com (child)
	//     uid=alice,ou=users,dc=example,dc=com (grandchild)
	//     uid=bob,ou=users,dc=example,dc=com (grandchild)
	//   ou=groups,dc=example,dc=com (child)
	//     cn=admins,ou=groups,dc=example,dc=com (grandchild)

	baseEntry := storage.NewEntry("dc=example,dc=com")
	baseEntry.SetStringAttribute("dc", "example")
	baseEntry.SetStringAttribute("objectClass", "domain")
	backend.addEntry(baseEntry)

	usersOU := storage.NewEntry("ou=users,dc=example,dc=com")
	usersOU.SetStringAttribute("ou", "users")
	usersOU.SetStringAttribute("objectClass", "organizationalUnit")
	backend.addEntry(usersOU)

	groupsOU := storage.NewEntry("ou=groups,dc=example,dc=com")
	groupsOU.SetStringAttribute("ou", "groups")
	groupsOU.SetStringAttribute("objectClass", "organizationalUnit")
	backend.addEntry(groupsOU)

	alice := storage.NewEntry("uid=alice,ou=users,dc=example,dc=com")
	alice.SetStringAttribute("uid", "alice")
	alice.SetStringAttribute("cn", "Alice Smith")
	alice.SetStringAttribute("objectClass", "inetOrgPerson")
	backend.addEntry(alice)

	bob := storage.NewEntry("uid=bob,ou=users,dc=example,dc=com")
	bob.SetStringAttribute("uid", "bob")
	bob.SetStringAttribute("cn", "Bob Jones")
	bob.SetStringAttribute("objectClass", "inetOrgPerson")
	backend.addEntry(bob)

	admins := storage.NewEntry("cn=admins,ou=groups,dc=example,dc=com")
	admins.SetStringAttribute("cn", "admins")
	admins.SetStringAttribute("objectClass", "groupOfNames")
	backend.addEntry(admins)

	config := &SearchConfig{
		Backend:       backend,
		SearchBackend: backend,
	}
	handler := NewSearchHandler(config)

	tests := []struct {
		name          string
		baseDN        string
		filter        *ldap.SearchFilter
		expectedCode  ldap.ResultCode
		expectedCount int
		checkEntries  func(t *testing.T, entries []*SearchEntry)
	}{
		{
			name:          "one level from root - returns immediate children",
			baseDN:        "dc=example,dc=com",
			filter:        nil,
			expectedCode:  ldap.ResultSuccess,
			expectedCount: 2, // ou=users and ou=groups
			checkEntries: func(t *testing.T, entries []*SearchEntry) {
				dns := make(map[string]bool)
				for _, e := range entries {
					dns[normalizeDN(e.DN)] = true
				}
				if !dns["ou=users,dc=example,dc=com"] {
					t.Error("Expected ou=users,dc=example,dc=com in results")
				}
				if !dns["ou=groups,dc=example,dc=com"] {
					t.Error("Expected ou=groups,dc=example,dc=com in results")
				}
			},
		},
		{
			name:          "one level from ou=users - returns users",
			baseDN:        "ou=users,dc=example,dc=com",
			filter:        nil,
			expectedCode:  ldap.ResultSuccess,
			expectedCount: 2, // uid=alice and uid=bob
			checkEntries: func(t *testing.T, entries []*SearchEntry) {
				dns := make(map[string]bool)
				for _, e := range entries {
					dns[normalizeDN(e.DN)] = true
				}
				if !dns["uid=alice,ou=users,dc=example,dc=com"] {
					t.Error("Expected uid=alice,ou=users,dc=example,dc=com in results")
				}
				if !dns["uid=bob,ou=users,dc=example,dc=com"] {
					t.Error("Expected uid=bob,ou=users,dc=example,dc=com in results")
				}
			},
		},
		{
			name:   "one level with filter - returns matching children",
			baseDN: "ou=users,dc=example,dc=com",
			filter: &ldap.SearchFilter{
				Type:      ldap.FilterTagEquality,
				Attribute: "uid",
				Value:     []byte("alice"),
			},
			expectedCode:  ldap.ResultSuccess,
			expectedCount: 1,
			checkEntries: func(t *testing.T, entries []*SearchEntry) {
				if normalizeDN(entries[0].DN) != "uid=alice,ou=users,dc=example,dc=com" {
					t.Errorf("Expected uid=alice,ou=users,dc=example,dc=com, got %s", entries[0].DN)
				}
			},
		},
		{
			name:          "one level from leaf - returns no entries",
			baseDN:        "uid=alice,ou=users,dc=example,dc=com",
			filter:        nil,
			expectedCode:  ldap.ResultSuccess,
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &ldap.SearchRequest{
				BaseObject: tt.baseDN,
				Scope:      ldap.ScopeSingleLevel,
				Filter:     tt.filter,
			}

			result := handler.Handle(nil, req)

			if result.ResultCode != tt.expectedCode {
				t.Errorf("Expected result code %v, got %v", tt.expectedCode, result.ResultCode)
			}

			if len(result.Entries) != tt.expectedCount {
				t.Errorf("Expected %d entries, got %d", tt.expectedCount, len(result.Entries))
			}

			if tt.checkEntries != nil && len(result.Entries) > 0 {
				tt.checkEntries(t, result.Entries)
			}
		})
	}
}

// TestSearchHandlerImpl_Handle_SubtreeScope tests subtree scope search.
func TestSearchHandlerImpl_Handle_SubtreeScope(t *testing.T) {
	backend := newMockSearchBackend()

	// Create a directory structure (same as OneLevel test)
	baseEntry := storage.NewEntry("dc=example,dc=com")
	baseEntry.SetStringAttribute("dc", "example")
	baseEntry.SetStringAttribute("objectClass", "domain")
	backend.addEntry(baseEntry)

	usersOU := storage.NewEntry("ou=users,dc=example,dc=com")
	usersOU.SetStringAttribute("ou", "users")
	usersOU.SetStringAttribute("objectClass", "organizationalUnit")
	backend.addEntry(usersOU)

	groupsOU := storage.NewEntry("ou=groups,dc=example,dc=com")
	groupsOU.SetStringAttribute("ou", "groups")
	groupsOU.SetStringAttribute("objectClass", "organizationalUnit")
	backend.addEntry(groupsOU)

	alice := storage.NewEntry("uid=alice,ou=users,dc=example,dc=com")
	alice.SetStringAttribute("uid", "alice")
	alice.SetStringAttribute("cn", "Alice Smith")
	alice.SetStringAttribute("objectClass", "inetOrgPerson")
	backend.addEntry(alice)

	bob := storage.NewEntry("uid=bob,ou=users,dc=example,dc=com")
	bob.SetStringAttribute("uid", "bob")
	bob.SetStringAttribute("cn", "Bob Jones")
	bob.SetStringAttribute("objectClass", "inetOrgPerson")
	backend.addEntry(bob)

	admins := storage.NewEntry("cn=admins,ou=groups,dc=example,dc=com")
	admins.SetStringAttribute("cn", "admins")
	admins.SetStringAttribute("objectClass", "groupOfNames")
	backend.addEntry(admins)

	config := &SearchConfig{
		Backend:       backend,
		SearchBackend: backend,
	}
	handler := NewSearchHandler(config)

	tests := []struct {
		name          string
		baseDN        string
		filter        *ldap.SearchFilter
		expectedCode  ldap.ResultCode
		expectedCount int
		checkEntries  func(t *testing.T, entries []*SearchEntry)
	}{
		{
			name:          "subtree from root - returns all entries",
			baseDN:        "dc=example,dc=com",
			filter:        nil,
			expectedCode:  ldap.ResultSuccess,
			expectedCount: 6, // base + 2 OUs + 2 users + 1 group
		},
		{
			name:          "subtree from ou=users - returns OU and users",
			baseDN:        "ou=users,dc=example,dc=com",
			filter:        nil,
			expectedCode:  ldap.ResultSuccess,
			expectedCount: 3, // ou=users + alice + bob
			checkEntries: func(t *testing.T, entries []*SearchEntry) {
				dns := make(map[string]bool)
				for _, e := range entries {
					dns[normalizeDN(e.DN)] = true
				}
				if !dns["ou=users,dc=example,dc=com"] {
					t.Error("Expected ou=users,dc=example,dc=com in results")
				}
				if !dns["uid=alice,ou=users,dc=example,dc=com"] {
					t.Error("Expected uid=alice,ou=users,dc=example,dc=com in results")
				}
				if !dns["uid=bob,ou=users,dc=example,dc=com"] {
					t.Error("Expected uid=bob,ou=users,dc=example,dc=com in results")
				}
			},
		},
		{
			name:   "subtree with filter - returns matching entries",
			baseDN: "dc=example,dc=com",
			filter: &ldap.SearchFilter{
				Type:      ldap.FilterTagEquality,
				Attribute: "objectClass",
				Value:     []byte("inetOrgPerson"),
			},
			expectedCode:  ldap.ResultSuccess,
			expectedCount: 2, // alice and bob
		},
		{
			name:          "subtree from leaf - returns only the leaf",
			baseDN:        "uid=alice,ou=users,dc=example,dc=com",
			filter:        nil,
			expectedCode:  ldap.ResultSuccess,
			expectedCount: 1,
			checkEntries: func(t *testing.T, entries []*SearchEntry) {
				if normalizeDN(entries[0].DN) != "uid=alice,ou=users,dc=example,dc=com" {
					t.Errorf("Expected uid=alice,ou=users,dc=example,dc=com, got %s", entries[0].DN)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &ldap.SearchRequest{
				BaseObject: tt.baseDN,
				Scope:      ldap.ScopeWholeSubtree,
				Filter:     tt.filter,
			}

			result := handler.Handle(nil, req)

			if result.ResultCode != tt.expectedCode {
				t.Errorf("Expected result code %v, got %v", tt.expectedCode, result.ResultCode)
			}

			if len(result.Entries) != tt.expectedCount {
				t.Errorf("Expected %d entries, got %d", tt.expectedCount, len(result.Entries))
			}

			if tt.checkEntries != nil && len(result.Entries) > 0 {
				tt.checkEntries(t, result.Entries)
			}
		})
	}
}

// TestSearchHandlerImpl_Handle_SizeLimit tests size limit enforcement.
func TestSearchHandlerImpl_Handle_SizeLimit(t *testing.T) {
	backend := newMockSearchBackend()

	// Create multiple entries
	for i := 0; i < 10; i++ {
		entry := storage.NewEntry("uid=user" + string(rune('0'+i)) + ",ou=users,dc=example,dc=com")
		entry.SetStringAttribute("uid", "user"+string(rune('0'+i)))
		entry.SetStringAttribute("objectClass", "inetOrgPerson")
		backend.addEntry(entry)
	}

	config := &SearchConfig{
		Backend:       backend,
		SearchBackend: backend,
		MaxSizeLimit:  100,
	}
	handler := NewSearchHandler(config)

	tests := []struct {
		name          string
		sizeLimit     int
		expectedCode  ldap.ResultCode
		expectedCount int
	}{
		{
			name:          "size limit 3 - returns 3 entries with SizeLimitExceeded",
			sizeLimit:     3,
			expectedCode:  ldap.ResultSizeLimitExceeded,
			expectedCount: 3,
		},
		{
			name:          "size limit 0 (unlimited) - returns all entries",
			sizeLimit:     0,
			expectedCode:  ldap.ResultSuccess,
			expectedCount: 10,
		},
		{
			name:          "size limit larger than results - returns all entries",
			sizeLimit:     100,
			expectedCode:  ldap.ResultSuccess,
			expectedCount: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &ldap.SearchRequest{
				BaseObject: "ou=users,dc=example,dc=com",
				Scope:      ldap.ScopeSingleLevel,
				SizeLimit:  tt.sizeLimit,
			}

			result := handler.Handle(nil, req)

			if result.ResultCode != tt.expectedCode {
				t.Errorf("Expected result code %v, got %v", tt.expectedCode, result.ResultCode)
			}

			if len(result.Entries) != tt.expectedCount {
				t.Errorf("Expected %d entries, got %d", tt.expectedCount, len(result.Entries))
			}
		})
	}
}

// TestOneLevelSearcher_Search tests the OneLevelSearcher directly.
func TestOneLevelSearcher_Search(t *testing.T) {
	backend := newMockSearchBackend()

	// Add test entries
	parent := storage.NewEntry("dc=example,dc=com")
	parent.SetStringAttribute("dc", "example")
	backend.addEntry(parent)

	child1 := storage.NewEntry("ou=users,dc=example,dc=com")
	child1.SetStringAttribute("ou", "users")
	backend.addEntry(child1)

	child2 := storage.NewEntry("ou=groups,dc=example,dc=com")
	child2.SetStringAttribute("ou", "groups")
	backend.addEntry(child2)

	searcher := NewOneLevelSearcher(backend)

	req := &ldap.SearchRequest{
		BaseObject: "dc=example,dc=com",
		Scope:      ldap.ScopeSingleLevel,
	}

	result := searcher.Search(req, nil)

	if result.ResultCode != ldap.ResultSuccess {
		t.Errorf("Expected ResultSuccess, got %v", result.ResultCode)
	}

	if len(result.Entries) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(result.Entries))
	}
}

// TestSubtreeSearcher_Search tests the SubtreeSearcher directly.
func TestSubtreeSearcher_Search(t *testing.T) {
	backend := newMockSearchBackend()

	// Add test entries
	parent := storage.NewEntry("dc=example,dc=com")
	parent.SetStringAttribute("dc", "example")
	backend.addEntry(parent)

	child := storage.NewEntry("ou=users,dc=example,dc=com")
	child.SetStringAttribute("ou", "users")
	backend.addEntry(child)

	grandchild := storage.NewEntry("uid=alice,ou=users,dc=example,dc=com")
	grandchild.SetStringAttribute("uid", "alice")
	backend.addEntry(grandchild)

	searcher := NewSubtreeSearcher(backend)

	req := &ldap.SearchRequest{
		BaseObject: "dc=example,dc=com",
		Scope:      ldap.ScopeWholeSubtree,
	}

	result := searcher.Search(req, nil)

	if result.ResultCode != ldap.ResultSuccess {
		t.Errorf("Expected ResultSuccess, got %v", result.ResultCode)
	}

	if len(result.Entries) != 3 {
		t.Errorf("Expected 3 entries, got %d", len(result.Entries))
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
