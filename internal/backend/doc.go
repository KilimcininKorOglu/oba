// Package backend provides the LDAP backend interface that wraps the storage engine
// and provides LDAP-specific operations including authentication, entry validation,
// and coordination with the storage layer.
//
// # Overview
//
// The backend package serves as the bridge between the LDAP protocol layer and
// the storage engine. It handles:
//
//   - User authentication (Bind operations)
//   - Entry CRUD operations (Add, Delete, Modify)
//   - Search operations with filter evaluation
//   - Schema validation
//   - DN normalization
//
// # Backend Interface
//
// The Backend interface defines the core operations:
//
//	type Backend interface {
//	    Bind(dn, password string) error
//	    Search(baseDN string, scope int, f *filter.Filter) ([]*Entry, error)
//	    Add(entry *Entry) error
//	    Delete(dn string) error
//	    Modify(dn string, changes []Modification) error
//	}
//
// # Creating a Backend
//
// Create a new backend with a storage engine and configuration:
//
//	engine, err := storage.NewEngine(opts)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	cfg := &config.Config{
//	    Directory: config.DirectoryConfig{
//	        RootDN:       "cn=admin,dc=example,dc=com",
//	        RootPassword: "secret",
//	    },
//	}
//
//	backend := backend.NewBackend(engine, cfg)
//
// # Entry Operations
//
// Entries are represented with string attribute values for easy manipulation:
//
//	entry := backend.NewEntry("uid=alice,ou=users,dc=example,dc=com")
//	entry.SetAttribute("objectClass", "inetOrgPerson", "person", "top")
//	entry.SetAttribute("cn", "Alice Smith")
//	entry.SetAttribute("uid", "alice")
//	entry.SetAttribute("userPassword", "{SSHA}...")
//
//	if err := backend.Add(entry); err != nil {
//	    // handle error
//	}
//
// # Modifications
//
// Use Modification to describe entry changes:
//
//	changes := []backend.Modification{
//	    {Type: backend.ModReplace, Attribute: "mail", Values: []string{"alice@example.com"}},
//	    {Type: backend.ModAdd, Attribute: "telephoneNumber", Values: []string{"+1-555-1234"}},
//	    {Type: backend.ModDelete, Attribute: "description", Values: nil}, // delete entire attribute
//	}
//
//	if err := backend.Modify("uid=alice,ou=users,dc=example,dc=com", changes); err != nil {
//	    // handle error
//	}
//
// # Error Handling
//
// The package defines specific errors for common failure conditions:
//
//   - ErrInvalidCredentials: Authentication failed
//   - ErrEntryNotFound: Entry does not exist
//   - ErrEntryExists: Entry already exists (on Add)
//   - ErrInvalidDN: Malformed distinguished name
//   - ErrInvalidEntry: Entry validation failed
package backend
