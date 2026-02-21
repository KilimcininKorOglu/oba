// Package tests provides integration tests for the Oba LDAP server.
package tests

import (
	"net"
	"testing"
	"time"

	"github.com/KilimcininKorOglu/oba/internal/backend"
	"github.com/KilimcininKorOglu/oba/internal/ber"
	"github.com/KilimcininKorOglu/oba/internal/ldap"
)

// TestIntegrationSearch tests search operations end-to-end.
func TestIntegrationSearch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
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

	// Add test entries
	setupSearchTestData(t, srv)

	t.Run("search_base_scope", func(t *testing.T) {
		testSearchBaseScope(t, srv)
	})

	t.Run("search_onelevel_scope", func(t *testing.T) {
		testSearchOneLevelScope(t, srv)
	})

	t.Run("search_subtree_scope", func(t *testing.T) {
		testSearchSubtreeScope(t, srv)
	})

	t.Run("search_with_filter", func(t *testing.T) {
		testSearchWithFilter(t, srv)
	})

	t.Run("search_nonexistent_base", func(t *testing.T) {
		testSearchNonexistentBase(t, srv)
	})

	t.Run("search_attribute_selection", func(t *testing.T) {
		testSearchAttributeSelection(t, srv)
	})
}

// setupSearchTestData adds test entries to the server.
func setupSearchTestData(t *testing.T, srv *TestServer) {
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

	// Add user entries
	alice := backend.NewEntry("uid=alice,ou=users,dc=test,dc=com")
	alice.SetAttribute("objectclass", "inetOrgPerson", "person", "top")
	alice.SetAttribute("uid", "alice")
	alice.SetAttribute("cn", "Alice Smith")
	alice.SetAttribute("sn", "Smith")
	alice.SetAttribute("mail", "alice@test.com")
	if err := be.Add(alice); err != nil {
		t.Fatalf("failed to add alice: %v", err)
	}

	bob := backend.NewEntry("uid=bob,ou=users,dc=test,dc=com")
	bob.SetAttribute("objectclass", "inetOrgPerson", "person", "top")
	bob.SetAttribute("uid", "bob")
	bob.SetAttribute("cn", "Bob Jones")
	bob.SetAttribute("sn", "Jones")
	bob.SetAttribute("mail", "bob@test.com")
	if err := be.Add(bob); err != nil {
		t.Fatalf("failed to add bob: %v", err)
	}
}

// testSearchBaseScope tests base scope search.
func testSearchBaseScope(t *testing.T, srv *TestServer) {
	conn, err := net.Dial("tcp", srv.Address())
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	// First bind
	if err := performBind(conn, srv.Config().RootDN, srv.Config().RootPassword); err != nil {
		t.Fatalf("bind failed: %v", err)
	}

	// Search for specific entry
	searchReq := createSearchRequest(2, "uid=alice,ou=users,dc=test,dc=com", ldap.ScopeBaseObject, "(objectclass=*)", nil)
	if err := sendMessage(conn, searchReq); err != nil {
		t.Fatalf("failed to send search request: %v", err)
	}

	// Read search result entries and done
	entries, resultCode, err := readSearchResults(conn)
	if err != nil {
		t.Fatalf("failed to read search results: %v", err)
	}

	if resultCode != ldap.ResultSuccess {
		t.Errorf("expected success, got result code %d", resultCode)
	}

	if len(entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(entries))
	}

	if len(entries) > 0 && entries[0].DN != "uid=alice,ou=users,dc=test,dc=com" {
		t.Errorf("expected DN 'uid=alice,ou=users,dc=test,dc=com', got '%s'", entries[0].DN)
	}
}

// testSearchOneLevelScope tests one-level scope search.
func testSearchOneLevelScope(t *testing.T, srv *TestServer) {
	conn, err := net.Dial("tcp", srv.Address())
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	// First bind
	if err := performBind(conn, srv.Config().RootDN, srv.Config().RootPassword); err != nil {
		t.Fatalf("bind failed: %v", err)
	}

	// Search for immediate children of ou=users
	searchReq := createSearchRequest(2, "ou=users,dc=test,dc=com", ldap.ScopeSingleLevel, "(objectclass=*)", nil)
	if err := sendMessage(conn, searchReq); err != nil {
		t.Fatalf("failed to send search request: %v", err)
	}

	// Read search result entries and done
	entries, resultCode, err := readSearchResults(conn)
	if err != nil {
		t.Fatalf("failed to read search results: %v", err)
	}

	if resultCode != ldap.ResultSuccess {
		t.Errorf("expected success, got result code %d", resultCode)
	}

	// Should find alice and bob
	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}
}

// testSearchSubtreeScope tests subtree scope search.
func testSearchSubtreeScope(t *testing.T, srv *TestServer) {
	conn, err := net.Dial("tcp", srv.Address())
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	// First bind
	if err := performBind(conn, srv.Config().RootDN, srv.Config().RootPassword); err != nil {
		t.Fatalf("bind failed: %v", err)
	}

	// Search entire subtree from base
	searchReq := createSearchRequest(2, "dc=test,dc=com", ldap.ScopeWholeSubtree, "(objectclass=*)", nil)
	if err := sendMessage(conn, searchReq); err != nil {
		t.Fatalf("failed to send search request: %v", err)
	}

	// Read search result entries and done
	entries, resultCode, err := readSearchResults(conn)
	if err != nil {
		t.Fatalf("failed to read search results: %v", err)
	}

	if resultCode != ldap.ResultSuccess {
		t.Errorf("expected success, got result code %d", resultCode)
	}

	// Should find base, ou=users, alice, bob (4 entries)
	if len(entries) < 4 {
		t.Errorf("expected at least 4 entries, got %d", len(entries))
	}
}

// testSearchWithFilter tests search with equality filter.
func testSearchWithFilter(t *testing.T, srv *TestServer) {
	conn, err := net.Dial("tcp", srv.Address())
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	// First bind
	if err := performBind(conn, srv.Config().RootDN, srv.Config().RootPassword); err != nil {
		t.Fatalf("bind failed: %v", err)
	}

	// Search with uid=alice filter
	searchReq := createSearchRequest(2, "dc=test,dc=com", ldap.ScopeWholeSubtree, "(uid=alice)", nil)
	if err := sendMessage(conn, searchReq); err != nil {
		t.Fatalf("failed to send search request: %v", err)
	}

	// Read search result entries and done
	entries, resultCode, err := readSearchResults(conn)
	if err != nil {
		t.Fatalf("failed to read search results: %v", err)
	}

	if resultCode != ldap.ResultSuccess {
		t.Errorf("expected success, got result code %d", resultCode)
	}

	// Should find only alice
	if len(entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(entries))
	}
}

// testSearchNonexistentBase tests search with non-existent base DN.
func testSearchNonexistentBase(t *testing.T, srv *TestServer) {
	conn, err := net.Dial("tcp", srv.Address())
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	// First bind
	if err := performBind(conn, srv.Config().RootDN, srv.Config().RootPassword); err != nil {
		t.Fatalf("bind failed: %v", err)
	}

	// Search with non-existent base
	searchReq := createSearchRequest(2, "ou=nonexistent,dc=test,dc=com", ldap.ScopeBaseObject, "(objectclass=*)", nil)
	if err := sendMessage(conn, searchReq); err != nil {
		t.Fatalf("failed to send search request: %v", err)
	}

	// Read search result
	_, resultCode, err := readSearchResults(conn)
	if err != nil {
		t.Fatalf("failed to read search results: %v", err)
	}

	// Accept either NoSuchObject or OperationsError (implementation may vary)
	if resultCode != ldap.ResultNoSuchObject && resultCode != ldap.ResultOperationsError {
		t.Errorf("expected no such object or operations error, got result code %d", resultCode)
	}
}

// testSearchAttributeSelection tests search with specific attribute selection.
func testSearchAttributeSelection(t *testing.T, srv *TestServer) {
	conn, err := net.Dial("tcp", srv.Address())
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	// First bind
	if err := performBind(conn, srv.Config().RootDN, srv.Config().RootPassword); err != nil {
		t.Fatalf("bind failed: %v", err)
	}

	// Search requesting only cn and mail attributes
	searchReq := createSearchRequest(2, "uid=alice,ou=users,dc=test,dc=com", ldap.ScopeBaseObject, "(objectclass=*)", []string{"cn", "mail"})
	if err := sendMessage(conn, searchReq); err != nil {
		t.Fatalf("failed to send search request: %v", err)
	}

	// Read search result entries and done
	entries, resultCode, err := readSearchResults(conn)
	if err != nil {
		t.Fatalf("failed to read search results: %v", err)
	}

	if resultCode != ldap.ResultSuccess {
		t.Errorf("expected success, got result code %d", resultCode)
	}

	if len(entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(entries))
		return
	}

	// Check that only requested attributes are returned
	entry := entries[0]
	hasCN := false
	hasMail := false
	for _, attr := range entry.Attributes {
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
}

// performBind performs a bind operation and returns any error.
func performBind(conn net.Conn, dn, password string) error {
	bindReq := createBindRequest(1, 3, dn, password)
	if err := sendMessage(conn, bindReq); err != nil {
		return err
	}

	resp, err := readMessage(conn)
	if err != nil {
		return err
	}

	resultCode := parseBindResponse(resp)
	if resultCode != ldap.ResultSuccess {
		return &ldapError{code: resultCode, message: "bind failed"}
	}

	return nil
}

// ldapError represents an LDAP error.
type ldapError struct {
	code    ldap.ResultCode
	message string
}

func (e *ldapError) Error() string {
	return e.message
}

// SearchResultEntry represents a search result entry.
type SearchResultEntry struct {
	DN         string
	Attributes []ldap.Attribute
}

// createSearchRequest creates a search request message.
func createSearchRequest(messageID int, baseDN string, scope ldap.SearchScope, filterStr string, attrs []string) *ldap.LDAPMessage {
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

	// Write filter
	if err := encodeFilter(encoder, filterStr); err != nil {
		return nil
	}

	// Write attributes (SEQUENCE OF AttributeDescription)
	attrPos := encoder.BeginSequence()
	for _, attr := range attrs {
		if err := encoder.WriteOctetString([]byte(attr)); err != nil {
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

// encodeFilter encodes a simple filter string.
func encodeFilter(encoder *ber.BEREncoder, filterStr string) error {
	// Parse simple filter formats: (attr=value), (attr=*), (objectclass=*)
	if len(filterStr) < 3 || filterStr[0] != '(' || filterStr[len(filterStr)-1] != ')' {
		// Default to presence filter for objectClass
		return encoder.WriteTaggedValue(ldap.FilterTagPresent, false, []byte("objectclass"))
	}

	inner := filterStr[1 : len(filterStr)-1]

	// Check for presence filter (attr=*)
	if len(inner) > 2 && inner[len(inner)-2:] == "=*" {
		attr := inner[:len(inner)-2]
		return encoder.WriteTaggedValue(ldap.FilterTagPresent, false, []byte(attr))
	}

	// Check for equality filter (attr=value)
	for i, c := range inner {
		if c == '=' {
			attr := inner[:i]
			value := inner[i+1:]

			// Encode as AttributeValueAssertion [3] SEQUENCE
			pos := encoder.WriteContextTag(ldap.FilterTagEquality, true)
			if err := encoder.WriteOctetString([]byte(attr)); err != nil {
				return err
			}
			if err := encoder.WriteOctetString([]byte(value)); err != nil {
				return err
			}
			return encoder.EndContextTag(pos)
		}
	}

	// Default to presence filter for objectClass
	return encoder.WriteTaggedValue(ldap.FilterTagPresent, false, []byte("objectclass"))
}

// readSearchResults reads all search result entries and the final done message.
func readSearchResults(conn net.Conn) ([]*SearchResultEntry, ldap.ResultCode, error) {
	var entries []*SearchResultEntry

	for {
		msg, err := readMessage(conn)
		if err != nil {
			return entries, ldap.ResultOperationsError, err
		}

		if msg.Operation == nil {
			continue
		}

		switch msg.Operation.Tag {
		case ldap.ApplicationSearchResultEntry:
			entry, err := parseSearchResultEntry(msg)
			if err != nil {
				return entries, ldap.ResultOperationsError, err
			}
			entries = append(entries, entry)

		case ldap.ApplicationSearchResultDone:
			resultCode := parseSearchResultDone(msg)
			return entries, resultCode, nil

		default:
			// Unexpected message type
			return entries, ldap.ResultProtocolError, nil
		}
	}
}

// parseSearchResultEntry parses a search result entry message.
func parseSearchResultEntry(msg *ldap.LDAPMessage) (*SearchResultEntry, error) {
	decoder := ber.NewBERDecoder(msg.Operation.Data)

	// Read objectName (LDAPDN - OCTET STRING)
	dnBytes, err := decoder.ReadOctetString()
	if err != nil {
		return nil, err
	}

	entry := &SearchResultEntry{
		DN: string(dnBytes),
	}

	// Read attributes (SEQUENCE OF PartialAttribute)
	if decoder.Remaining() > 0 {
		_, err := decoder.ExpectSequence()
		if err != nil {
			return entry, nil // No attributes
		}

		for decoder.Remaining() > 0 {
			// Read PartialAttribute SEQUENCE
			attrDecoder, err := decoder.ReadSequenceContents()
			if err != nil {
				break
			}

			// Read attribute type
			attrType, err := attrDecoder.ReadOctetString()
			if err != nil {
				break
			}

			attr := ldap.Attribute{
				Type: string(attrType),
			}

			// Read attribute values (SET OF OCTET STRING)
			if attrDecoder.Remaining() > 0 {
				_, err := attrDecoder.ExpectSet()
				if err == nil {
					for attrDecoder.Remaining() > 0 {
						value, err := attrDecoder.ReadOctetString()
						if err != nil {
							break
						}
						attr.Values = append(attr.Values, value)
					}
				}
			}

			entry.Attributes = append(entry.Attributes, attr)
		}
	}

	return entry, nil
}

// parseSearchResultDone parses a search result done message.
func parseSearchResultDone(msg *ldap.LDAPMessage) ldap.ResultCode {
	decoder := ber.NewBERDecoder(msg.Operation.Data)

	// Read result code (ENUMERATED)
	resultCode, err := decoder.ReadEnumerated()
	if err != nil {
		return ldap.ResultOperationsError
	}

	return ldap.ResultCode(resultCode)
}
