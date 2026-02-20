// Package server provides the LDAP server implementation.
package server

import (
	"net"
	"testing"

	"github.com/oba-ldap/oba/internal/acl"
	"github.com/oba-ldap/oba/internal/ldap"
	"github.com/oba-ldap/oba/internal/storage"
)

// aclTestConn implements net.Conn for ACL testing purposes.
type aclTestConn struct {
	net.Conn
}

func (m *aclTestConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345}
}

func (m *aclTestConn) LocalAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 389}
}

// createACLTestConnection creates a test connection with the given bind DN.
func createACLTestConnection(bindDN string) *Connection {
	conn := NewConnection(&aclTestConn{}, nil)
	conn.bindDN = bindDN
	conn.authenticated = bindDN != ""
	return conn
}

// TestSearchHandler_ACL_SearchPermission tests that search checks search permission.
func TestSearchHandler_ACL_SearchPermission(t *testing.T) {
	backend := newMockSearchBackend()

	// Add test entry
	entry := storage.NewEntry("uid=alice,ou=users,dc=example,dc=com")
	entry.SetStringAttribute("uid", "alice")
	entry.SetStringAttribute("cn", "Alice Smith")
	entry.SetStringAttribute("objectClass", "person")
	backend.addEntry(entry)

	// Create ACL config that denies search for anonymous users
	aclConfig := acl.NewConfig()
	aclConfig.SetDefaultPolicy("deny")
	// Allow admin to search
	aclConfig.AddRule(acl.NewACL("*", "cn=admin,dc=example,dc=com", acl.All))
	// Deny anonymous search
	aclEvaluator := acl.NewEvaluator(aclConfig)

	config := &SearchConfig{
		Backend:      backend,
		ACLEvaluator: aclEvaluator,
	}
	handler := NewSearchHandler(config)

	tests := []struct {
		name         string
		bindDN       string
		expectedCode ldap.ResultCode
	}{
		{
			name:         "anonymous user denied search",
			bindDN:       "",
			expectedCode: ldap.ResultInsufficientAccessRights,
		},
		{
			name:         "admin user allowed search",
			bindDN:       "cn=admin,dc=example,dc=com",
			expectedCode: ldap.ResultSuccess,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := createACLTestConnection(tt.bindDN)
			req := &ldap.SearchRequest{
				BaseObject: "uid=alice,ou=users,dc=example,dc=com",
				Scope:      ldap.ScopeBaseObject,
			}

			result := handler.Handle(conn, req)

			if result.ResultCode != tt.expectedCode {
				t.Errorf("Handle() ResultCode = %v, want %v", result.ResultCode, tt.expectedCode)
			}
		})
	}
}

// TestSearchHandler_ACL_AttributeFiltering tests that search filters attributes by read permission.
func TestSearchHandler_ACL_AttributeFiltering(t *testing.T) {
	backend := newMockSearchBackend()

	// Add test entry with multiple attributes
	entry := storage.NewEntry("uid=alice,ou=users,dc=example,dc=com")
	entry.SetStringAttribute("uid", "alice")
	entry.SetStringAttribute("cn", "Alice Smith")
	entry.SetStringAttribute("mail", "alice@example.com")
	entry.SetStringAttribute("userPassword", "secret")
	entry.SetStringAttribute("objectClass", "person")
	backend.addEntry(entry)

	// Create ACL config that allows search but restricts attribute access
	aclConfig := acl.NewConfig()
	aclConfig.SetDefaultPolicy("deny")
	// Allow authenticated users to search
	aclConfig.AddRule(acl.NewACL("*", "authenticated", acl.Search|acl.Read).WithAttributes("uid", "cn", "mail", "objectClass"))
	// Deny access to userPassword for non-admin
	aclConfig.AddRule(acl.NewACL("*", "cn=admin,dc=example,dc=com", acl.All))
	aclEvaluator := acl.NewEvaluator(aclConfig)

	config := &SearchConfig{
		Backend:      backend,
		ACLEvaluator: aclEvaluator,
	}
	handler := NewSearchHandler(config)

	conn := createACLTestConnection("uid=bob,ou=users,dc=example,dc=com")
	req := &ldap.SearchRequest{
		BaseObject: "uid=alice,ou=users,dc=example,dc=com",
		Scope:      ldap.ScopeBaseObject,
	}

	result := handler.Handle(conn, req)

	if result.ResultCode != ldap.ResultSuccess {
		t.Fatalf("Handle() ResultCode = %v, want %v", result.ResultCode, ldap.ResultSuccess)
	}

	if len(result.Entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(result.Entries))
	}

	// Check that userPassword is not in the result
	for _, attr := range result.Entries[0].Attributes {
		if attr.Type == "userPassword" {
			t.Error("userPassword should have been filtered out")
		}
	}
}

// TestAddHandler_ACL_AddPermission tests that add checks add permission.
func TestAddHandler_ACL_AddPermission(t *testing.T) {
	// Create ACL config
	aclConfig := acl.NewConfig()
	aclConfig.SetDefaultPolicy("deny")
	// Allow admin to add
	aclConfig.AddRule(acl.NewACL("*", "cn=admin,dc=example,dc=com", acl.All))
	aclEvaluator := acl.NewEvaluator(aclConfig)

	tests := []struct {
		name         string
		bindDN       string
		expectedCode ldap.ResultCode
	}{
		{
			name:         "anonymous user denied add",
			bindDN:       "",
			expectedCode: ldap.ResultInsufficientAccessRights,
		},
		{
			name:         "regular user denied add",
			bindDN:       "uid=bob,ou=users,dc=example,dc=com",
			expectedCode: ldap.ResultInsufficientAccessRights,
		},
		{
			name:         "admin user allowed add",
			bindDN:       "cn=admin,dc=example,dc=com",
			expectedCode: ldap.ResultSuccess,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend := newMockAddBackend()
			// Add parent entry
			parent := storage.NewEntry("ou=users,dc=example,dc=com")
			parent.SetStringAttribute("objectclass", "organizationalUnit")
			backend.addEntry(parent)

			config := &AddConfig{
				Backend:      backend,
				ACLEvaluator: aclEvaluator,
			}
			handler := NewAddHandler(config)

			conn := createACLTestConnection(tt.bindDN)
			req := &ldap.AddRequest{
				Entry: "uid=newuser,ou=users,dc=example,dc=com",
				Attributes: []ldap.Attribute{
					{Type: "objectClass", Values: [][]byte{[]byte("person")}},
					{Type: "uid", Values: [][]byte{[]byte("newuser")}},
					{Type: "cn", Values: [][]byte{[]byte("New User")}},
				},
			}

			result := handler.Handle(conn, req)

			if result.ResultCode != tt.expectedCode {
				t.Errorf("Handle() ResultCode = %v, want %v", result.ResultCode, tt.expectedCode)
			}
		})
	}
}

// TestDeleteHandler_ACL_DeletePermission tests that delete checks delete permission.
func TestDeleteHandler_ACL_DeletePermission(t *testing.T) {
	// Create ACL config
	aclConfig := acl.NewConfig()
	aclConfig.SetDefaultPolicy("deny")
	// Allow admin to delete
	aclConfig.AddRule(acl.NewACL("*", "cn=admin,dc=example,dc=com", acl.All))
	aclEvaluator := acl.NewEvaluator(aclConfig)

	tests := []struct {
		name         string
		bindDN       string
		expectedCode ldap.ResultCode
	}{
		{
			name:         "anonymous user denied delete",
			bindDN:       "",
			expectedCode: ldap.ResultInsufficientAccessRights,
		},
		{
			name:         "regular user denied delete",
			bindDN:       "uid=bob,ou=users,dc=example,dc=com",
			expectedCode: ldap.ResultInsufficientAccessRights,
		},
		{
			name:         "admin user allowed delete",
			bindDN:       "cn=admin,dc=example,dc=com",
			expectedCode: ldap.ResultSuccess,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend := newMockDeleteBackend()
			// Add entry to delete
			entry := storage.NewEntry("uid=alice,ou=users,dc=example,dc=com")
			entry.SetStringAttribute("objectclass", "person")
			backend.addEntry(entry)

			config := &DeleteConfig{
				Backend:      backend,
				ACLEvaluator: aclEvaluator,
			}
			handler := NewDeleteHandler(config)

			conn := createACLTestConnection(tt.bindDN)
			req := &ldap.DeleteRequest{
				DN: "uid=alice,ou=users,dc=example,dc=com",
			}

			result := handler.Handle(conn, req)

			if result.ResultCode != tt.expectedCode {
				t.Errorf("Handle() ResultCode = %v, want %v", result.ResultCode, tt.expectedCode)
			}
		})
	}
}

// TestModifyHandler_ACL_WritePermission tests that modify checks write permission.
func TestModifyHandler_ACL_WritePermission(t *testing.T) {
	// Create ACL config
	aclConfig := acl.NewConfig()
	aclConfig.SetDefaultPolicy("deny")
	// Allow admin to write
	aclConfig.AddRule(acl.NewACL("*", "cn=admin,dc=example,dc=com", acl.All))
	// Allow users to modify their own entries
	aclConfig.AddRule(acl.NewACL("*", "self", acl.Read|acl.Write))
	aclEvaluator := acl.NewEvaluator(aclConfig)

	tests := []struct {
		name         string
		bindDN       string
		targetDN     string
		expectedCode ldap.ResultCode
	}{
		{
			name:         "anonymous user denied modify",
			bindDN:       "",
			targetDN:     "uid=alice,ou=users,dc=example,dc=com",
			expectedCode: ldap.ResultInsufficientAccessRights,
		},
		{
			name:         "regular user denied modify other",
			bindDN:       "uid=bob,ou=users,dc=example,dc=com",
			targetDN:     "uid=alice,ou=users,dc=example,dc=com",
			expectedCode: ldap.ResultInsufficientAccessRights,
		},
		{
			name:         "user allowed modify self",
			bindDN:       "uid=alice,ou=users,dc=example,dc=com",
			targetDN:     "uid=alice,ou=users,dc=example,dc=com",
			expectedCode: ldap.ResultSuccess,
		},
		{
			name:         "admin user allowed modify",
			bindDN:       "cn=admin,dc=example,dc=com",
			targetDN:     "uid=alice,ou=users,dc=example,dc=com",
			expectedCode: ldap.ResultSuccess,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend := newMockModifyBackend()
			// Add entry to modify
			entry := storage.NewEntry("uid=alice,ou=users,dc=example,dc=com")
			entry.SetStringAttribute("objectclass", "person")
			entry.SetStringAttribute("cn", "Alice")
			backend.addEntry(entry)

			config := &ModifyConfig{
				Backend:      backend,
				ACLEvaluator: aclEvaluator,
			}
			handler := NewModifyHandler(config)

			conn := createACLTestConnection(tt.bindDN)
			req := &ldap.ModifyRequest{
				Object: tt.targetDN,
				Changes: []ldap.Modification{
					{
						Operation: ldap.ModifyOperationReplace,
						Attribute: ldap.Attribute{
							Type:   "cn",
							Values: [][]byte{[]byte("Alice Updated")},
						},
					},
				},
			}

			result := handler.Handle(conn, req)

			if result.ResultCode != tt.expectedCode {
				t.Errorf("Handle() ResultCode = %v, want %v", result.ResultCode, tt.expectedCode)
			}
		})
	}
}

// TestSearchHandler_ACL_NilEvaluator tests that search works without ACL evaluator.
func TestSearchHandler_ACL_NilEvaluator(t *testing.T) {
	backend := newMockSearchBackend()

	// Add test entry
	entry := storage.NewEntry("uid=alice,ou=users,dc=example,dc=com")
	entry.SetStringAttribute("uid", "alice")
	entry.SetStringAttribute("objectClass", "person")
	backend.addEntry(entry)

	config := &SearchConfig{
		Backend:      backend,
		ACLEvaluator: nil, // No ACL evaluator
	}
	handler := NewSearchHandler(config)

	req := &ldap.SearchRequest{
		BaseObject: "uid=alice,ou=users,dc=example,dc=com",
		Scope:      ldap.ScopeBaseObject,
	}

	// Should work without ACL checks
	result := handler.Handle(nil, req)

	if result.ResultCode != ldap.ResultSuccess {
		t.Errorf("Handle() ResultCode = %v, want %v", result.ResultCode, ldap.ResultSuccess)
	}
}

// TestAddHandler_ACL_NilEvaluator tests that add works without ACL evaluator.
func TestAddHandler_ACL_NilEvaluator(t *testing.T) {
	backend := newMockAddBackend()
	// Add parent entry
	parent := storage.NewEntry("ou=users,dc=example,dc=com")
	parent.SetStringAttribute("objectclass", "organizationalUnit")
	backend.addEntry(parent)

	config := &AddConfig{
		Backend:      backend,
		ACLEvaluator: nil, // No ACL evaluator
	}
	handler := NewAddHandler(config)

	req := &ldap.AddRequest{
		Entry: "uid=newuser,ou=users,dc=example,dc=com",
		Attributes: []ldap.Attribute{
			{Type: "objectClass", Values: [][]byte{[]byte("person")}},
			{Type: "uid", Values: [][]byte{[]byte("newuser")}},
		},
	}

	// Should work without ACL checks
	result := handler.Handle(nil, req)

	if result.ResultCode != ldap.ResultSuccess {
		t.Errorf("Handle() ResultCode = %v, want %v", result.ResultCode, ldap.ResultSuccess)
	}
}

// TestDeleteHandler_ACL_NilEvaluator tests that delete works without ACL evaluator.
func TestDeleteHandler_ACL_NilEvaluator(t *testing.T) {
	backend := newMockDeleteBackend()
	// Add entry to delete
	entry := storage.NewEntry("uid=alice,ou=users,dc=example,dc=com")
	entry.SetStringAttribute("objectclass", "person")
	backend.addEntry(entry)

	config := &DeleteConfig{
		Backend:      backend,
		ACLEvaluator: nil, // No ACL evaluator
	}
	handler := NewDeleteHandler(config)

	req := &ldap.DeleteRequest{
		DN: "uid=alice,ou=users,dc=example,dc=com",
	}

	// Should work without ACL checks
	result := handler.Handle(nil, req)

	if result.ResultCode != ldap.ResultSuccess {
		t.Errorf("Handle() ResultCode = %v, want %v", result.ResultCode, ldap.ResultSuccess)
	}
}

// TestModifyHandler_ACL_NilEvaluator tests that modify works without ACL evaluator.
func TestModifyHandler_ACL_NilEvaluator(t *testing.T) {
	backend := newMockModifyBackend()
	// Add entry to modify
	entry := storage.NewEntry("uid=alice,ou=users,dc=example,dc=com")
	entry.SetStringAttribute("objectclass", "person")
	entry.SetStringAttribute("cn", "Alice")
	backend.addEntry(entry)

	config := &ModifyConfig{
		Backend:      backend,
		ACLEvaluator: nil, // No ACL evaluator
	}
	handler := NewModifyHandler(config)

	req := &ldap.ModifyRequest{
		Object: "uid=alice,ou=users,dc=example,dc=com",
		Changes: []ldap.Modification{
			{
				Operation: ldap.ModifyOperationReplace,
				Attribute: ldap.Attribute{
					Type:   "cn",
					Values: [][]byte{[]byte("Alice Updated")},
				},
			},
		},
	}

	// Should work without ACL checks
	result := handler.Handle(nil, req)

	if result.ResultCode != ldap.ResultSuccess {
		t.Errorf("Handle() ResultCode = %v, want %v", result.ResultCode, ldap.ResultSuccess)
	}
}
