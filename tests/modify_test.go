// Package tests provides integration tests for the Oba LDAP server.
package tests

import (
	"net"
	"testing"
	"time"

	"github.com/oba-ldap/oba/internal/backend"
	"github.com/oba-ldap/oba/internal/ber"
	"github.com/oba-ldap/oba/internal/ldap"
)

// TestIntegrationModify tests add, modify, and delete operations end-to-end.
func TestIntegrationModify(t *testing.T) {
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

	// Add base entry for tests
	setupModifyTestData(t, srv)

	t.Run("add_entry", func(t *testing.T) {
		testAddEntry(t, srv)
	})

	t.Run("add_duplicate_entry", func(t *testing.T) {
		testAddDuplicateEntry(t, srv)
	})

	t.Run("modify_entry_add_attribute", func(t *testing.T) {
		testModifyEntryAddAttribute(t, srv)
	})

	t.Run("modify_entry_replace_attribute", func(t *testing.T) {
		testModifyEntryReplaceAttribute(t, srv)
	})

	t.Run("modify_entry_delete_attribute", func(t *testing.T) {
		testModifyEntryDeleteAttribute(t, srv)
	})

	t.Run("modify_nonexistent_entry", func(t *testing.T) {
		testModifyNonexistentEntry(t, srv)
	})

	t.Run("delete_entry", func(t *testing.T) {
		testDeleteEntry(t, srv)
	})

	t.Run("delete_nonexistent_entry", func(t *testing.T) {
		testDeleteNonexistentEntry(t, srv)
	})
}

// setupModifyTestData adds base entries for modify tests.
func setupModifyTestData(t *testing.T, srv *TestServer) {
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
}

// testAddEntry tests adding a new entry.
func testAddEntry(t *testing.T, srv *TestServer) {
	conn, err := net.Dial("tcp", srv.Address())
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	// First bind
	if err := performBind(conn, srv.Config().RootDN, srv.Config().RootPassword); err != nil {
		t.Fatalf("bind failed: %v", err)
	}

	// Add new entry
	attrs := []ldap.Attribute{
		{Type: "objectclass", Values: [][]byte{[]byte("inetOrgPerson"), []byte("person"), []byte("top")}},
		{Type: "uid", Values: [][]byte{[]byte("testuser")}},
		{Type: "cn", Values: [][]byte{[]byte("Test User")}},
		{Type: "sn", Values: [][]byte{[]byte("User")}},
	}

	addReq := createAddRequest(2, "uid=testuser,ou=users,dc=test,dc=com", attrs)
	if err := sendMessage(conn, addReq); err != nil {
		t.Fatalf("failed to send add request: %v", err)
	}

	// Read response
	resp, err := readMessage(conn)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	resultCode := parseAddResponse(resp)
	if resultCode != ldap.ResultSuccess {
		t.Errorf("expected success, got result code %d", resultCode)
	}
}

// testAddDuplicateEntry tests adding a duplicate entry.
func testAddDuplicateEntry(t *testing.T, srv *TestServer) {
	conn, err := net.Dial("tcp", srv.Address())
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	// First bind
	if err := performBind(conn, srv.Config().RootDN, srv.Config().RootPassword); err != nil {
		t.Fatalf("bind failed: %v", err)
	}

	// Try to add duplicate entry (ou=users already exists)
	attrs := []ldap.Attribute{
		{Type: "objectclass", Values: [][]byte{[]byte("organizationalUnit"), []byte("top")}},
		{Type: "ou", Values: [][]byte{[]byte("users")}},
	}

	addReq := createAddRequest(2, "ou=users,dc=test,dc=com", attrs)
	if err := sendMessage(conn, addReq); err != nil {
		t.Fatalf("failed to send add request: %v", err)
	}

	// Read response
	resp, err := readMessage(conn)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	resultCode := parseAddResponse(resp)
	if resultCode != ldap.ResultEntryAlreadyExists {
		t.Errorf("expected entry already exists, got result code %d", resultCode)
	}
}

// testModifyEntryAddAttribute tests adding an attribute to an entry.
func testModifyEntryAddAttribute(t *testing.T, srv *TestServer) {
	// First add an entry to modify
	be := srv.Backend()
	entry := backend.NewEntry("uid=modifytest,ou=users,dc=test,dc=com")
	entry.SetAttribute("objectclass", "inetOrgPerson", "person", "top")
	entry.SetAttribute("uid", "modifytest")
	entry.SetAttribute("cn", "Modify Test")
	entry.SetAttribute("sn", "Test")
	if err := be.Add(entry); err != nil {
		t.Fatalf("failed to add test entry: %v", err)
	}

	conn, err := net.Dial("tcp", srv.Address())
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	// First bind
	if err := performBind(conn, srv.Config().RootDN, srv.Config().RootPassword); err != nil {
		t.Fatalf("bind failed: %v", err)
	}

	// Modify entry - add mail attribute
	changes := []ModifyChange{
		{Operation: ldap.ModifyOperationAdd, Attribute: "mail", Values: [][]byte{[]byte("modifytest@test.com")}},
	}

	modifyReq := createModifyRequest(2, "uid=modifytest,ou=users,dc=test,dc=com", changes)
	if err := sendMessage(conn, modifyReq); err != nil {
		t.Fatalf("failed to send modify request: %v", err)
	}

	// Read response
	resp, err := readMessage(conn)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	resultCode := parseModifyResponse(resp)
	if resultCode != ldap.ResultSuccess {
		t.Errorf("expected success, got result code %d", resultCode)
	}
}

// testModifyEntryReplaceAttribute tests replacing an attribute value.
func testModifyEntryReplaceAttribute(t *testing.T, srv *TestServer) {
	conn, err := net.Dial("tcp", srv.Address())
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	// First bind
	if err := performBind(conn, srv.Config().RootDN, srv.Config().RootPassword); err != nil {
		t.Fatalf("bind failed: %v", err)
	}

	// Modify entry - replace cn attribute
	changes := []ModifyChange{
		{Operation: ldap.ModifyOperationReplace, Attribute: "cn", Values: [][]byte{[]byte("Modified Test User")}},
	}

	modifyReq := createModifyRequest(2, "uid=modifytest,ou=users,dc=test,dc=com", changes)
	if err := sendMessage(conn, modifyReq); err != nil {
		t.Fatalf("failed to send modify request: %v", err)
	}

	// Read response
	resp, err := readMessage(conn)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	resultCode := parseModifyResponse(resp)
	if resultCode != ldap.ResultSuccess {
		t.Errorf("expected success, got result code %d", resultCode)
	}
}

// testModifyEntryDeleteAttribute tests deleting an attribute.
func testModifyEntryDeleteAttribute(t *testing.T, srv *TestServer) {
	conn, err := net.Dial("tcp", srv.Address())
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	// First bind
	if err := performBind(conn, srv.Config().RootDN, srv.Config().RootPassword); err != nil {
		t.Fatalf("bind failed: %v", err)
	}

	// Modify entry - delete mail attribute
	changes := []ModifyChange{
		{Operation: ldap.ModifyOperationDelete, Attribute: "mail", Values: nil},
	}

	modifyReq := createModifyRequest(2, "uid=modifytest,ou=users,dc=test,dc=com", changes)
	if err := sendMessage(conn, modifyReq); err != nil {
		t.Fatalf("failed to send modify request: %v", err)
	}

	// Read response
	resp, err := readMessage(conn)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	resultCode := parseModifyResponse(resp)
	if resultCode != ldap.ResultSuccess {
		t.Errorf("expected success, got result code %d", resultCode)
	}
}

// testModifyNonexistentEntry tests modifying a non-existent entry.
func testModifyNonexistentEntry(t *testing.T, srv *TestServer) {
	conn, err := net.Dial("tcp", srv.Address())
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	// First bind
	if err := performBind(conn, srv.Config().RootDN, srv.Config().RootPassword); err != nil {
		t.Fatalf("bind failed: %v", err)
	}

	// Try to modify non-existent entry
	changes := []ModifyChange{
		{Operation: ldap.ModifyOperationReplace, Attribute: "cn", Values: [][]byte{[]byte("New Name")}},
	}

	modifyReq := createModifyRequest(2, "uid=nonexistent,ou=users,dc=test,dc=com", changes)
	if err := sendMessage(conn, modifyReq); err != nil {
		t.Fatalf("failed to send modify request: %v", err)
	}

	// Read response
	resp, err := readMessage(conn)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	resultCode := parseModifyResponse(resp)
	if resultCode != ldap.ResultNoSuchObject {
		t.Errorf("expected no such object, got result code %d", resultCode)
	}
}

// testDeleteEntry tests deleting an entry.
func testDeleteEntry(t *testing.T, srv *TestServer) {
	// First add an entry to delete
	be := srv.Backend()
	entry := backend.NewEntry("uid=deletetest,ou=users,dc=test,dc=com")
	entry.SetAttribute("objectclass", "inetOrgPerson", "person", "top")
	entry.SetAttribute("uid", "deletetest")
	entry.SetAttribute("cn", "Delete Test")
	entry.SetAttribute("sn", "Test")
	if err := be.Add(entry); err != nil {
		t.Fatalf("failed to add test entry: %v", err)
	}

	conn, err := net.Dial("tcp", srv.Address())
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	// First bind
	if err := performBind(conn, srv.Config().RootDN, srv.Config().RootPassword); err != nil {
		t.Fatalf("bind failed: %v", err)
	}

	// Delete entry
	deleteReq := createDeleteRequest(2, "uid=deletetest,ou=users,dc=test,dc=com")
	if err := sendMessage(conn, deleteReq); err != nil {
		t.Fatalf("failed to send delete request: %v", err)
	}

	// Read response
	resp, err := readMessage(conn)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	resultCode := parseDeleteResponse(resp)
	if resultCode != ldap.ResultSuccess {
		t.Errorf("expected success, got result code %d", resultCode)
	}
}

// testDeleteNonexistentEntry tests deleting a non-existent entry.
func testDeleteNonexistentEntry(t *testing.T, srv *TestServer) {
	conn, err := net.Dial("tcp", srv.Address())
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	// First bind
	if err := performBind(conn, srv.Config().RootDN, srv.Config().RootPassword); err != nil {
		t.Fatalf("bind failed: %v", err)
	}

	// Try to delete non-existent entry
	deleteReq := createDeleteRequest(2, "uid=nonexistent,ou=users,dc=test,dc=com")
	if err := sendMessage(conn, deleteReq); err != nil {
		t.Fatalf("failed to send delete request: %v", err)
	}

	// Read response
	resp, err := readMessage(conn)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	resultCode := parseDeleteResponse(resp)
	if resultCode != ldap.ResultNoSuchObject {
		t.Errorf("expected no such object, got result code %d", resultCode)
	}
}

// ModifyChange represents a single modification change.
type ModifyChange struct {
	Operation ldap.ModifyOperation
	Attribute string
	Values    [][]byte
}

// createAddRequest creates an add request message.
func createAddRequest(messageID int, dn string, attrs []ldap.Attribute) *ldap.LDAPMessage {
	encoder := ber.NewBEREncoder(256)

	// Write entry DN (LDAPDN - OCTET STRING)
	if err := encoder.WriteOctetString([]byte(dn)); err != nil {
		return nil
	}

	// Write attributes (SEQUENCE OF Attribute)
	attrListPos := encoder.BeginSequence()
	for _, attr := range attrs {
		// Write Attribute SEQUENCE
		attrPos := encoder.BeginSequence()

		// Write attribute type
		if err := encoder.WriteOctetString([]byte(attr.Type)); err != nil {
			return nil
		}

		// Write attribute values (SET OF OCTET STRING)
		valSetPos := encoder.BeginSet()
		for _, value := range attr.Values {
			if err := encoder.WriteOctetString(value); err != nil {
				return nil
			}
		}
		if err := encoder.EndSet(valSetPos); err != nil {
			return nil
		}

		if err := encoder.EndSequence(attrPos); err != nil {
			return nil
		}
	}
	if err := encoder.EndSequence(attrListPos); err != nil {
		return nil
	}

	return &ldap.LDAPMessage{
		MessageID: messageID,
		Operation: &ldap.RawOperation{
			Tag:  ldap.ApplicationAddRequest,
			Data: encoder.Bytes(),
		},
	}
}

// createModifyRequest creates a modify request message.
func createModifyRequest(messageID int, dn string, changes []ModifyChange) *ldap.LDAPMessage {
	encoder := ber.NewBEREncoder(256)

	// Write object DN (LDAPDN - OCTET STRING)
	if err := encoder.WriteOctetString([]byte(dn)); err != nil {
		return nil
	}

	// Write changes (SEQUENCE OF change)
	changesPos := encoder.BeginSequence()
	for _, change := range changes {
		// Write change SEQUENCE
		changePos := encoder.BeginSequence()

		// Write operation (ENUMERATED)
		if err := encoder.WriteEnumerated(int64(change.Operation)); err != nil {
			return nil
		}

		// Write modification (PartialAttribute SEQUENCE)
		modPos := encoder.BeginSequence()

		// Write attribute type
		if err := encoder.WriteOctetString([]byte(change.Attribute)); err != nil {
			return nil
		}

		// Write attribute values (SET OF OCTET STRING)
		valSetPos := encoder.BeginSet()
		for _, value := range change.Values {
			if err := encoder.WriteOctetString(value); err != nil {
				return nil
			}
		}
		if err := encoder.EndSet(valSetPos); err != nil {
			return nil
		}

		if err := encoder.EndSequence(modPos); err != nil {
			return nil
		}

		if err := encoder.EndSequence(changePos); err != nil {
			return nil
		}
	}
	if err := encoder.EndSequence(changesPos); err != nil {
		return nil
	}

	return &ldap.LDAPMessage{
		MessageID: messageID,
		Operation: &ldap.RawOperation{
			Tag:  ldap.ApplicationModifyRequest,
			Data: encoder.Bytes(),
		},
	}
}

// createDeleteRequest creates a delete request message.
func createDeleteRequest(messageID int, dn string) *ldap.LDAPMessage {
	return &ldap.LDAPMessage{
		MessageID: messageID,
		Operation: &ldap.RawOperation{
			Tag:  ldap.ApplicationDelRequest,
			Data: []byte(dn),
		},
	}
}

// parseAddResponse parses an add response and returns the result code.
func parseAddResponse(msg *ldap.LDAPMessage) ldap.ResultCode {
	if msg.Operation == nil {
		return ldap.ResultOperationsError
	}

	if msg.Operation.Tag != ldap.ApplicationAddResponse {
		return ldap.ResultOperationsError
	}

	decoder := ber.NewBERDecoder(msg.Operation.Data)
	resultCode, err := decoder.ReadEnumerated()
	if err != nil {
		return ldap.ResultOperationsError
	}

	return ldap.ResultCode(resultCode)
}

// parseModifyResponse parses a modify response and returns the result code.
func parseModifyResponse(msg *ldap.LDAPMessage) ldap.ResultCode {
	if msg.Operation == nil {
		return ldap.ResultOperationsError
	}

	if msg.Operation.Tag != ldap.ApplicationModifyResponse {
		return ldap.ResultOperationsError
	}

	decoder := ber.NewBERDecoder(msg.Operation.Data)
	resultCode, err := decoder.ReadEnumerated()
	if err != nil {
		return ldap.ResultOperationsError
	}

	return ldap.ResultCode(resultCode)
}

// parseDeleteResponse parses a delete response and returns the result code.
func parseDeleteResponse(msg *ldap.LDAPMessage) ldap.ResultCode {
	if msg.Operation == nil {
		return ldap.ResultOperationsError
	}

	if msg.Operation.Tag != ldap.ApplicationDelResponse {
		return ldap.ResultOperationsError
	}

	decoder := ber.NewBERDecoder(msg.Operation.Data)
	resultCode, err := decoder.ReadEnumerated()
	if err != nil {
		return ldap.ResultOperationsError
	}

	return ldap.ResultCode(resultCode)
}
