// Package ldap implements LDAP protocol message parsing and encoding
// as specified in RFC 4511.
//
// This package provides types and functions for working with LDAP protocol
// messages, including request parsing, response encoding, and result codes.
//
// # Message Structure
//
// All LDAP messages follow the LDAPMessage envelope structure:
//
//	LDAPMessage ::= SEQUENCE {
//	    messageID       MessageID,
//	    protocolOp      CHOICE { ... },
//	    controls        [0] Controls OPTIONAL
//	}
//
// Use ParseLDAPMessage to decode incoming messages:
//
//	msg, err := ldap.ParseLDAPMessage(data)
//	if err != nil {
//	    // handle error
//	}
//	switch msg.OperationType() {
//	case ldap.ApplicationBindRequest:
//	    req, err := ldap.ParseBindRequest(msg.Operation.Data)
//	    // handle bind request
//	case ldap.ApplicationSearchRequest:
//	    req, err := ldap.ParseSearchRequest(msg.Operation.Data)
//	    // handle search request
//	}
//
// # Supported Operations
//
// The package supports all core LDAP operations:
//
//   - Bind (APPLICATION 0): Authentication
//   - Unbind (APPLICATION 2): Connection termination
//   - Search (APPLICATION 3): Entry lookup
//   - Modify (APPLICATION 6): Entry modification
//   - Add (APPLICATION 8): Entry creation
//   - Delete (APPLICATION 10): Entry removal
//   - Extended (APPLICATION 23): Extended operations
//
// # Result Codes
//
// LDAP operations return standardized result codes defined in RFC 4511:
//
//	result := ldap.ResultSuccess           // Operation succeeded
//	result := ldap.ResultInvalidCredentials // Authentication failed
//	result := ldap.ResultNoSuchObject      // Entry not found
//
// # Search Filters
//
// Search filters are parsed into a tree structure:
//
//	// Equality filter: (uid=alice)
//	filter := &ldap.SearchFilter{
//	    Type:      ldap.FilterTagEquality,
//	    Attribute: "uid",
//	    Value:     []byte("alice"),
//	}
//
//	// AND filter: (&(objectClass=person)(uid=alice))
//	filter := &ldap.SearchFilter{
//	    Type: ldap.FilterTagAnd,
//	    Children: []*ldap.SearchFilter{
//	        {Type: ldap.FilterTagEquality, Attribute: "objectClass", Value: []byte("person")},
//	        {Type: ldap.FilterTagEquality, Attribute: "uid", Value: []byte("alice")},
//	    },
//	}
//
// # References
//
//   - RFC 4511: LDAP Protocol
//   - RFC 4512: LDAP Directory Information Models
//   - RFC 4513: LDAP Authentication Methods
package ldap
