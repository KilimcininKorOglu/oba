// Package tests provides compatibility tests for the Oba LDAP server.
// These tests verify compatibility with common LDAP clients and tools.
package tests

import (
	"net"
	"testing"
	"time"

	"github.com/KilimcininKorOglu/oba/internal/backend"
	"github.com/KilimcininKorOglu/oba/internal/ber"
	"github.com/KilimcininKorOglu/oba/internal/ldap"
)

// TestClientCompatibility tests compatibility with various LDAP clients.
func TestClientCompatibility(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping compatibility test in short mode")
	}

	// Start test server
	srv, err := NewTestServer(nil)
	if err != nil {
		t.Fatalf("failed to create test server: %v", err)
	}

	if err := srv.Start(); err != nil {
		t.Fatalf("failed to start test server: %v", err)
	}
	defer srv.Stop()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Setup test data
	setupCompatTestData(t, srv)

	t.Run("protocol_compliance", func(t *testing.T) {
		testProtocolCompliance(t, srv)
	})

	t.Run("filter_types", func(t *testing.T) {
		testFilterTypes(t, srv)
	})

	t.Run("search_scopes", func(t *testing.T) {
		testSearchScopes(t, srv)
	})

	t.Run("error_codes", func(t *testing.T) {
		testErrorCodes(t, srv)
	})

	t.Run("attribute_handling", func(t *testing.T) {
		testAttributeHandling(t, srv)
	})

	t.Run("dn_handling", func(t *testing.T) {
		testDNHandling(t, srv)
	})

	t.Run("concurrent_operations", func(t *testing.T) {
		testConcurrentOperations(t, srv)
	})
}

// setupCompatTestData adds test entries for compatibility tests.
func setupCompatTestData(t *testing.T, srv *TestServer) {
	be := srv.Backend()

	// Add base entry
	baseEntry := backend.NewEntry("dc=test,dc=com")
	baseEntry.SetAttribute("objectclass", "domain", "top")
	baseEntry.SetAttribute("dc", "test")
	if err := be.Add(baseEntry); err != nil {
		t.Fatalf("failed to add base entry: %v", err)
	}

	// Add ou=users
	usersOU := backend.NewEntry("ou=users,dc=test,dc=com")
	usersOU.SetAttribute("objectclass", "organizationalUnit", "top")
	usersOU.SetAttribute("ou", "users")
	if err := be.Add(usersOU); err != nil {
		t.Fatalf("failed to add users OU: %v", err)
	}

	// Add ou=groups
	groupsOU := backend.NewEntry("ou=groups,dc=test,dc=com")
	groupsOU.SetAttribute("objectclass", "organizationalUnit", "top")
	groupsOU.SetAttribute("ou", "groups")
	if err := be.Add(groupsOU); err != nil {
		t.Fatalf("failed to add groups OU: %v", err)
	}

	// Add user entries with various attributes
	alice := backend.NewEntry("uid=alice,ou=users,dc=test,dc=com")
	alice.SetAttribute("objectclass", "inetOrgPerson", "person", "top")
	alice.SetAttribute("uid", "alice")
	alice.SetAttribute("cn", "Alice Smith")
	alice.SetAttribute("sn", "Smith")
	alice.SetAttribute("mail", "alice@test.com")
	alice.SetAttribute("description", "Test user Alice")
	alice.SetAttribute("telephoneNumber", "+1-555-0101")
	if err := be.Add(alice); err != nil {
		t.Fatalf("failed to add alice: %v", err)
	}

	bob := backend.NewEntry("uid=bob,ou=users,dc=test,dc=com")
	bob.SetAttribute("objectclass", "inetOrgPerson", "person", "top")
	bob.SetAttribute("uid", "bob")
	bob.SetAttribute("cn", "Bob Jones")
	bob.SetAttribute("sn", "Jones")
	bob.SetAttribute("mail", "bob@test.com")
	bob.SetAttribute("description", "Test user Bob")
	if err := be.Add(bob); err != nil {
		t.Fatalf("failed to add bob: %v", err)
	}

	charlie := backend.NewEntry("uid=charlie,ou=users,dc=test,dc=com")
	charlie.SetAttribute("objectclass", "inetOrgPerson", "person", "top")
	charlie.SetAttribute("uid", "charlie")
	charlie.SetAttribute("cn", "Charlie Admin Brown")
	charlie.SetAttribute("sn", "Brown")
	charlie.SetAttribute("mail", "charlie@test.com")
	charlie.SetAttribute("description", "Admin user Charlie")
	if err := be.Add(charlie); err != nil {
		t.Fatalf("failed to add charlie: %v", err)
	}

	// Add group entry
	admins := backend.NewEntry("cn=admins,ou=groups,dc=test,dc=com")
	admins.SetAttribute("objectclass", "groupOfNames", "top")
	admins.SetAttribute("cn", "admins")
	admins.SetAttribute("member", "uid=alice,ou=users,dc=test,dc=com", "uid=charlie,ou=users,dc=test,dc=com")
	if err := be.Add(admins); err != nil {
		t.Fatalf("failed to add admins group: %v", err)
	}
}

// testProtocolCompliance tests basic LDAP protocol compliance.
func testProtocolCompliance(t *testing.T, srv *TestServer) {
	t.Run("ldap_v3_bind", func(t *testing.T) {
		conn, err := net.Dial("tcp", srv.Address())
		if err != nil {
			t.Fatalf("failed to connect: %v", err)
		}
		defer conn.Close()

		// Test LDAP v3 bind
		bindReq := createBindRequest(1, 3, srv.Config().RootDN, srv.Config().RootPassword)
		if err := sendMessage(conn, bindReq); err != nil {
			t.Fatalf("failed to send bind request: %v", err)
		}

		resp, err := readMessage(conn)
		if err != nil {
			t.Fatalf("failed to read response: %v", err)
		}

		resultCode := parseBindResponse(resp)
		if resultCode != ldap.ResultSuccess {
			t.Errorf("expected success, got result code %d", resultCode)
		}
	})

	t.Run("anonymous_bind", func(t *testing.T) {
		conn, err := net.Dial("tcp", srv.Address())
		if err != nil {
			t.Fatalf("failed to connect: %v", err)
		}
		defer conn.Close()

		// Test anonymous bind
		bindReq := createBindRequest(1, 3, "", "")
		if err := sendMessage(conn, bindReq); err != nil {
			t.Fatalf("failed to send bind request: %v", err)
		}

		resp, err := readMessage(conn)
		if err != nil {
			t.Fatalf("failed to read response: %v", err)
		}

		resultCode := parseBindResponse(resp)
		if resultCode != ldap.ResultSuccess {
			t.Errorf("expected success for anonymous bind, got result code %d", resultCode)
		}
	})

	t.Run("message_id_tracking", func(t *testing.T) {
		conn, err := net.Dial("tcp", srv.Address())
		if err != nil {
			t.Fatalf("failed to connect: %v", err)
		}
		defer conn.Close()

		// Send bind with specific message ID
		bindReq := createBindRequest(42, 3, "", "")
		if err := sendMessage(conn, bindReq); err != nil {
			t.Fatalf("failed to send bind request: %v", err)
		}

		resp, err := readMessage(conn)
		if err != nil {
			t.Fatalf("failed to read response: %v", err)
		}

		if resp.MessageID != 42 {
			t.Errorf("expected message ID 42, got %d", resp.MessageID)
		}
	})
}

// testFilterTypes tests various LDAP filter types.
func testFilterTypes(t *testing.T, srv *TestServer) {
	tests := []struct {
		name          string
		filter        string
		expectedCount int
		expectDN      string
	}{
		{
			name:          "equality_filter",
			filter:        "(uid=alice)",
			expectedCount: 1,
			expectDN:      "uid=alice,ou=users,dc=test,dc=com",
		},
		{
			name:          "presence_filter",
			filter:        "(mail=*)",
			expectedCount: 3, // alice, bob, charlie
		},
		{
			name:          "objectclass_filter",
			filter:        "(objectclass=inetOrgPerson)",
			expectedCount: 3,
		},
		{
			name:          "no_match_filter",
			filter:        "(uid=nonexistent)",
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn, err := net.Dial("tcp", srv.Address())
			if err != nil {
				t.Fatalf("failed to connect: %v", err)
			}
			defer conn.Close()

			// Bind first
			if err := performBind(conn, srv.Config().RootDN, srv.Config().RootPassword); err != nil {
				t.Fatalf("bind failed: %v", err)
			}

			// Search with filter
			searchReq := createSearchRequest(2, "dc=test,dc=com", ldap.ScopeWholeSubtree, tt.filter, nil)
			if err := sendMessage(conn, searchReq); err != nil {
				t.Fatalf("failed to send search request: %v", err)
			}

			entries, resultCode, err := readSearchResults(conn)
			if err != nil {
				t.Fatalf("failed to read search results: %v", err)
			}

			if resultCode != ldap.ResultSuccess {
				t.Errorf("expected success, got result code %d", resultCode)
			}

			if len(entries) != tt.expectedCount {
				t.Errorf("expected %d entries, got %d", tt.expectedCount, len(entries))
			}

			if tt.expectDN != "" && len(entries) > 0 {
				if entries[0].DN != tt.expectDN {
					t.Errorf("expected DN %s, got %s", tt.expectDN, entries[0].DN)
				}
			}
		})
	}

	// Test substring filters separately with proper encoding
	t.Run("substring_filters", func(t *testing.T) {
		testSubstringFilters(t, srv)
	})
}

// testSubstringFilters tests substring filter types with proper encoding.
func testSubstringFilters(t *testing.T, srv *TestServer) {
	tests := []struct {
		name          string
		attr          string
		initial       string
		any           []string
		final         string
		expectedCount int
		expectDN      string
	}{
		{
			name:          "prefix_filter",
			attr:          "cn",
			initial:       "Alice",
			expectedCount: 1,
			expectDN:      "uid=alice,ou=users,dc=test,dc=com",
		},
		{
			name:          "suffix_filter",
			attr:          "cn",
			final:         "Jones",
			expectedCount: 1,
			expectDN:      "uid=bob,ou=users,dc=test,dc=com",
		},
		{
			name:          "contains_filter",
			attr:          "description",
			any:           []string{"Admin"},
			expectedCount: 1,
			expectDN:      "uid=charlie,ou=users,dc=test,dc=com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn, err := net.Dial("tcp", srv.Address())
			if err != nil {
				t.Fatalf("failed to connect: %v", err)
			}
			defer conn.Close()

			// Bind first
			if err := performBind(conn, srv.Config().RootDN, srv.Config().RootPassword); err != nil {
				t.Fatalf("bind failed: %v", err)
			}

			// Create search request with substring filter
			searchReq := createSubstringSearchRequest(2, "dc=test,dc=com", ldap.ScopeWholeSubtree, tt.attr, tt.initial, tt.any, tt.final, nil)
			if err := sendMessage(conn, searchReq); err != nil {
				t.Fatalf("failed to send search request: %v", err)
			}

			entries, resultCode, err := readSearchResults(conn)
			if err != nil {
				t.Fatalf("failed to read search results: %v", err)
			}

			if resultCode != ldap.ResultSuccess {
				t.Errorf("expected success, got result code %d", resultCode)
			}

			if len(entries) != tt.expectedCount {
				t.Errorf("expected %d entries, got %d", tt.expectedCount, len(entries))
			}

			if tt.expectDN != "" && len(entries) > 0 {
				if entries[0].DN != tt.expectDN {
					t.Errorf("expected DN %s, got %s", tt.expectDN, entries[0].DN)
				}
			}
		})
	}
}

// createSubstringSearchRequest creates a search request with a substring filter.
func createSubstringSearchRequest(messageID int, baseDN string, scope ldap.SearchScope, attr, initial string, any []string, final string, attrs []string) *ldap.LDAPMessage {
	encoder := ber.NewBEREncoder(256)

	// Write baseObject (LDAPDN - OCTET STRING)
	if err := encoder.WriteOctetString([]byte(baseDN)); err != nil {
		return nil
	}

	// Write scope (ENUMERATED)
	if err := encoder.WriteEnumerated(int64(scope)); err != nil {
		return nil
	}

	// Write derefAliases (ENUMERATED) - never
	if err := encoder.WriteEnumerated(0); err != nil {
		return nil
	}

	// Write sizeLimit (INTEGER)
	if err := encoder.WriteInteger(0); err != nil {
		return nil
	}

	// Write timeLimit (INTEGER)
	if err := encoder.WriteInteger(0); err != nil {
		return nil
	}

	// Write typesOnly (BOOLEAN)
	if err := encoder.WriteBoolean(false); err != nil {
		return nil
	}

	// Write substring filter
	subEncoder := ber.NewBEREncoder(128)
	subEncoder.WriteOctetString([]byte(attr))

	// substrings SEQUENCE
	subSeqPos := subEncoder.BeginSequence()
	if initial != "" {
		subEncoder.WriteTaggedValue(ldap.SubstringInitial, false, []byte(initial))
	}
	for _, a := range any {
		subEncoder.WriteTaggedValue(ldap.SubstringAny, false, []byte(a))
	}
	if final != "" {
		subEncoder.WriteTaggedValue(ldap.SubstringFinal, false, []byte(final))
	}
	subEncoder.EndSequence(subSeqPos)

	encoder.WriteTaggedValue(ldap.FilterTagSubstrings, true, subEncoder.Bytes())

	// Write attributes (SEQUENCE OF AttributeDescription)
	attrPos := encoder.BeginSequence()
	for _, a := range attrs {
		if err := encoder.WriteOctetString([]byte(a)); err != nil {
			return nil
		}
	}
	if err := encoder.EndSequence(attrPos); err != nil {
		return nil
	}

	return &ldap.LDAPMessage{
		MessageID: messageID,
		Operation: &ldap.RawOperation{
			Tag:  ldap.ApplicationSearchRequest,
			Data: encoder.Bytes(),
		},
	}
}

// testSearchScopes tests different search scopes.
func testSearchScopes(t *testing.T, srv *TestServer) {
	t.Run("base_scope", func(t *testing.T) {
		conn, err := net.Dial("tcp", srv.Address())
		if err != nil {
			t.Fatalf("failed to connect: %v", err)
		}
		defer conn.Close()

		if err := performBind(conn, srv.Config().RootDN, srv.Config().RootPassword); err != nil {
			t.Fatalf("bind failed: %v", err)
		}

		searchReq := createSearchRequest(2, "uid=alice,ou=users,dc=test,dc=com", ldap.ScopeBaseObject, "(objectclass=*)", nil)
		if err := sendMessage(conn, searchReq); err != nil {
			t.Fatalf("failed to send search request: %v", err)
		}

		entries, resultCode, err := readSearchResults(conn)
		if err != nil {
			t.Fatalf("failed to read search results: %v", err)
		}

		if resultCode != ldap.ResultSuccess {
			t.Errorf("expected success, got result code %d", resultCode)
		}

		if len(entries) != 1 {
			t.Errorf("expected 1 entry for base scope, got %d", len(entries))
		}
	})

	t.Run("onelevel_scope", func(t *testing.T) {
		conn, err := net.Dial("tcp", srv.Address())
		if err != nil {
			t.Fatalf("failed to connect: %v", err)
		}
		defer conn.Close()

		if err := performBind(conn, srv.Config().RootDN, srv.Config().RootPassword); err != nil {
			t.Fatalf("bind failed: %v", err)
		}

		searchReq := createSearchRequest(2, "ou=users,dc=test,dc=com", ldap.ScopeSingleLevel, "(objectclass=*)", nil)
		if err := sendMessage(conn, searchReq); err != nil {
			t.Fatalf("failed to send search request: %v", err)
		}

		entries, resultCode, err := readSearchResults(conn)
		if err != nil {
			t.Fatalf("failed to read search results: %v", err)
		}

		if resultCode != ldap.ResultSuccess {
			t.Errorf("expected success, got result code %d", resultCode)
		}

		// Should find alice, bob, charlie (3 users)
		if len(entries) != 3 {
			t.Errorf("expected 3 entries for onelevel scope, got %d", len(entries))
		}
	})

	t.Run("subtree_scope", func(t *testing.T) {
		conn, err := net.Dial("tcp", srv.Address())
		if err != nil {
			t.Fatalf("failed to connect: %v", err)
		}
		defer conn.Close()

		if err := performBind(conn, srv.Config().RootDN, srv.Config().RootPassword); err != nil {
			t.Fatalf("bind failed: %v", err)
		}

		searchReq := createSearchRequest(2, "dc=test,dc=com", ldap.ScopeWholeSubtree, "(objectclass=*)", nil)
		if err := sendMessage(conn, searchReq); err != nil {
			t.Fatalf("failed to send search request: %v", err)
		}

		entries, resultCode, err := readSearchResults(conn)
		if err != nil {
			t.Fatalf("failed to read search results: %v", err)
		}

		if resultCode != ldap.ResultSuccess {
			t.Errorf("expected success, got result code %d", resultCode)
		}

		// Should find: base, ou=users, ou=groups, alice, bob, charlie, admins (7 entries)
		if len(entries) < 7 {
			t.Errorf("expected at least 7 entries for subtree scope, got %d", len(entries))
		}
	})
}

// testErrorCodes tests that correct LDAP error codes are returned.
func testErrorCodes(t *testing.T, srv *TestServer) {
	t.Run("invalid_credentials", func(t *testing.T) {
		conn, err := net.Dial("tcp", srv.Address())
		if err != nil {
			t.Fatalf("failed to connect: %v", err)
		}
		defer conn.Close()

		bindReq := createBindRequest(1, 3, srv.Config().RootDN, "wrongpassword")
		if err := sendMessage(conn, bindReq); err != nil {
			t.Fatalf("failed to send bind request: %v", err)
		}

		resp, err := readMessage(conn)
		if err != nil {
			t.Fatalf("failed to read response: %v", err)
		}

		resultCode := parseBindResponse(resp)
		if resultCode != ldap.ResultInvalidCredentials {
			t.Errorf("expected invalid credentials (49), got result code %d", resultCode)
		}
	})

	t.Run("no_such_object", func(t *testing.T) {
		conn, err := net.Dial("tcp", srv.Address())
		if err != nil {
			t.Fatalf("failed to connect: %v", err)
		}
		defer conn.Close()

		if err := performBind(conn, srv.Config().RootDN, srv.Config().RootPassword); err != nil {
			t.Fatalf("bind failed: %v", err)
		}

		// Search for non-existent base DN
		searchReq := createSearchRequest(2, "ou=nonexistent,dc=test,dc=com", ldap.ScopeBaseObject, "(objectclass=*)", nil)
		if err := sendMessage(conn, searchReq); err != nil {
			t.Fatalf("failed to send search request: %v", err)
		}

		_, resultCode, err := readSearchResults(conn)
		if err != nil {
			t.Fatalf("failed to read search results: %v", err)
		}

		// Accept either NoSuchObject or OperationsError
		if resultCode != ldap.ResultNoSuchObject && resultCode != ldap.ResultOperationsError {
			t.Errorf("expected no such object (32) or operations error (1), got result code %d", resultCode)
		}
	})

	t.Run("entry_already_exists", func(t *testing.T) {
		conn, err := net.Dial("tcp", srv.Address())
		if err != nil {
			t.Fatalf("failed to connect: %v", err)
		}
		defer conn.Close()

		if err := performBind(conn, srv.Config().RootDN, srv.Config().RootPassword); err != nil {
			t.Fatalf("bind failed: %v", err)
		}

		// Try to add an entry that already exists
		attrs := []ldap.Attribute{
			{Type: "objectclass", Values: [][]byte{[]byte("inetOrgPerson"), []byte("person"), []byte("top")}},
			{Type: "uid", Values: [][]byte{[]byte("alice")}},
			{Type: "cn", Values: [][]byte{[]byte("Alice Duplicate")}},
			{Type: "sn", Values: [][]byte{[]byte("Duplicate")}},
		}
		addReq := createAddRequest(2, "uid=alice,ou=users,dc=test,dc=com", attrs)
		if err := sendMessage(conn, addReq); err != nil {
			t.Fatalf("failed to send add request: %v", err)
		}

		resp, err := readMessage(conn)
		if err != nil {
			t.Fatalf("failed to read response: %v", err)
		}

		resultCode := parseAddResponse(resp)
		if resultCode != ldap.ResultEntryAlreadyExists {
			t.Errorf("expected entry already exists (68), got result code %d", resultCode)
		}
	})

	t.Run("protocol_error_ldap_v2", func(t *testing.T) {
		conn, err := net.Dial("tcp", srv.Address())
		if err != nil {
			t.Fatalf("failed to connect: %v", err)
		}
		defer conn.Close()

		// Try LDAP v2 bind
		bindReq := createBindRequest(1, 2, "", "")
		if err := sendMessage(conn, bindReq); err != nil {
			t.Fatalf("failed to send bind request: %v", err)
		}

		resp, err := readMessage(conn)
		if err != nil {
			t.Fatalf("failed to read response: %v", err)
		}

		resultCode := parseBindResponse(resp)
		if resultCode != ldap.ResultProtocolError {
			t.Errorf("expected protocol error (2), got result code %d", resultCode)
		}
	})
}

// testAttributeHandling tests attribute handling compatibility.
func testAttributeHandling(t *testing.T, srv *TestServer) {
	t.Run("attribute_selection", func(t *testing.T) {
		conn, err := net.Dial("tcp", srv.Address())
		if err != nil {
			t.Fatalf("failed to connect: %v", err)
		}
		defer conn.Close()

		if err := performBind(conn, srv.Config().RootDN, srv.Config().RootPassword); err != nil {
			t.Fatalf("bind failed: %v", err)
		}

		// Request only specific attributes
		searchReq := createSearchRequest(2, "uid=alice,ou=users,dc=test,dc=com", ldap.ScopeBaseObject, "(objectclass=*)", []string{"cn", "mail"})
		if err := sendMessage(conn, searchReq); err != nil {
			t.Fatalf("failed to send search request: %v", err)
		}

		entries, resultCode, err := readSearchResults(conn)
		if err != nil {
			t.Fatalf("failed to read search results: %v", err)
		}

		if resultCode != ldap.ResultSuccess {
			t.Errorf("expected success, got result code %d", resultCode)
		}

		if len(entries) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(entries))
		}

		// Check that requested attributes are present
		hasCN := false
		hasMail := false
		for _, attr := range entries[0].Attributes {
			if attr.Type == "cn" {
				hasCN = true
			}
			if attr.Type == "mail" {
				hasMail = true
			}
		}

		if !hasCN {
			t.Error("expected cn attribute in response")
		}
		if !hasMail {
			t.Error("expected mail attribute in response")
		}
	})

	t.Run("multi_value_attributes", func(t *testing.T) {
		conn, err := net.Dial("tcp", srv.Address())
		if err != nil {
			t.Fatalf("failed to connect: %v", err)
		}
		defer conn.Close()

		if err := performBind(conn, srv.Config().RootDN, srv.Config().RootPassword); err != nil {
			t.Fatalf("bind failed: %v", err)
		}

		// Search for admins group which has multiple member values
		searchReq := createSearchRequest(2, "cn=admins,ou=groups,dc=test,dc=com", ldap.ScopeBaseObject, "(objectclass=*)", []string{"member"})
		if err := sendMessage(conn, searchReq); err != nil {
			t.Fatalf("failed to send search request: %v", err)
		}

		entries, resultCode, err := readSearchResults(conn)
		if err != nil {
			t.Fatalf("failed to read search results: %v", err)
		}

		if resultCode != ldap.ResultSuccess {
			t.Errorf("expected success, got result code %d", resultCode)
		}

		if len(entries) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(entries))
		}

		// Check for member attribute with multiple values
		for _, attr := range entries[0].Attributes {
			if attr.Type == "member" {
				if len(attr.Values) < 2 {
					t.Errorf("expected at least 2 member values, got %d", len(attr.Values))
				}
				return
			}
		}
		t.Error("expected member attribute in response")
	})
}

// testDNHandling tests DN handling compatibility.
func testDNHandling(t *testing.T, srv *TestServer) {
	t.Run("case_insensitive_dn", func(t *testing.T) {
		conn, err := net.Dial("tcp", srv.Address())
		if err != nil {
			t.Fatalf("failed to connect: %v", err)
		}
		defer conn.Close()

		if err := performBind(conn, srv.Config().RootDN, srv.Config().RootPassword); err != nil {
			t.Fatalf("bind failed: %v", err)
		}

		// Search with different case DN
		searchReq := createSearchRequest(2, "UID=ALICE,OU=USERS,DC=TEST,DC=COM", ldap.ScopeBaseObject, "(objectclass=*)", nil)
		if err := sendMessage(conn, searchReq); err != nil {
			t.Fatalf("failed to send search request: %v", err)
		}

		entries, resultCode, err := readSearchResults(conn)
		if err != nil {
			t.Fatalf("failed to read search results: %v", err)
		}

		if resultCode != ldap.ResultSuccess {
			t.Errorf("expected success, got result code %d", resultCode)
		}

		if len(entries) != 1 {
			t.Errorf("expected 1 entry for case-insensitive DN, got %d", len(entries))
		}
	})

	t.Run("dn_with_spaces", func(t *testing.T) {
		// Add an entry with spaces in DN component
		be := srv.Backend()
		spaceEntry := backend.NewEntry("cn=Test User,ou=users,dc=test,dc=com")
		spaceEntry.SetAttribute("objectclass", "inetOrgPerson", "person", "top")
		spaceEntry.SetAttribute("cn", "Test User")
		spaceEntry.SetAttribute("sn", "User")
		if err := be.Add(spaceEntry); err != nil {
			t.Fatalf("failed to add entry with spaces: %v", err)
		}

		conn, err := net.Dial("tcp", srv.Address())
		if err != nil {
			t.Fatalf("failed to connect: %v", err)
		}
		defer conn.Close()

		if err := performBind(conn, srv.Config().RootDN, srv.Config().RootPassword); err != nil {
			t.Fatalf("bind failed: %v", err)
		}

		// Search for entry with spaces
		searchReq := createSearchRequest(2, "cn=Test User,ou=users,dc=test,dc=com", ldap.ScopeBaseObject, "(objectclass=*)", nil)
		if err := sendMessage(conn, searchReq); err != nil {
			t.Fatalf("failed to send search request: %v", err)
		}

		entries, resultCode, err := readSearchResults(conn)
		if err != nil {
			t.Fatalf("failed to read search results: %v", err)
		}

		if resultCode != ldap.ResultSuccess {
			t.Errorf("expected success, got result code %d", resultCode)
		}

		if len(entries) != 1 {
			t.Errorf("expected 1 entry for DN with spaces, got %d", len(entries))
		}
	})
}

// testConcurrentOperations tests concurrent LDAP operations.
func testConcurrentOperations(t *testing.T, srv *TestServer) {
	t.Run("concurrent_searches", func(t *testing.T) {
		const numGoroutines = 10
		const numSearches = 5

		errChan := make(chan error, numGoroutines*numSearches)
		doneChan := make(chan bool, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func() {
				conn, err := net.Dial("tcp", srv.Address())
				if err != nil {
					errChan <- err
					doneChan <- true
					return
				}
				defer conn.Close()

				if err := performBind(conn, srv.Config().RootDN, srv.Config().RootPassword); err != nil {
					errChan <- err
					doneChan <- true
					return
				}

				for j := 0; j < numSearches; j++ {
					searchReq := createSearchRequest(j+2, "dc=test,dc=com", ldap.ScopeWholeSubtree, "(objectclass=*)", nil)
					if err := sendMessage(conn, searchReq); err != nil {
						errChan <- err
						continue
					}

					_, resultCode, err := readSearchResults(conn)
					if err != nil {
						errChan <- err
						continue
					}

					if resultCode != ldap.ResultSuccess {
						errChan <- &ldapError{code: resultCode, message: "search failed"}
					}
				}
				doneChan <- true
			}()
		}

		// Wait for all goroutines
		for i := 0; i < numGoroutines; i++ {
			<-doneChan
		}

		close(errChan)
		var errors []error
		for err := range errChan {
			errors = append(errors, err)
		}

		if len(errors) > 0 {
			t.Errorf("concurrent searches had %d errors: %v", len(errors), errors[0])
		}
	})

	t.Run("concurrent_binds", func(t *testing.T) {
		const numGoroutines = 10

		errChan := make(chan error, numGoroutines)
		doneChan := make(chan bool, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func() {
				conn, err := net.Dial("tcp", srv.Address())
				if err != nil {
					errChan <- err
					doneChan <- true
					return
				}
				defer conn.Close()

				if err := performBind(conn, srv.Config().RootDN, srv.Config().RootPassword); err != nil {
					errChan <- err
				}
				doneChan <- true
			}()
		}

		// Wait for all goroutines
		for i := 0; i < numGoroutines; i++ {
			<-doneChan
		}

		close(errChan)
		var errors []error
		for err := range errChan {
			errors = append(errors, err)
		}

		if len(errors) > 0 {
			t.Errorf("concurrent binds had %d errors: %v", len(errors), errors[0])
		}
	})
}
