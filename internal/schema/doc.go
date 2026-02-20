// Package schema provides LDAP schema data structures including object classes,
// attribute types, syntaxes, and matching rules.
//
// # Overview
//
// The schema package implements LDAP schema validation as defined in RFC 4512.
// It provides:
//
//   - Object class definitions (structural, auxiliary, abstract)
//   - Attribute type definitions with syntax and matching rules
//   - Syntax definitions for value validation
//   - Entry validation against schema rules
//
// # Schema Structure
//
// A Schema contains all definitions:
//
//	schema := schema.NewSchema()
//
//	// Add object class
//	oc := schema.NewObjectClass("2.5.6.6", "person")
//	oc.Kind = schema.ObjectClassStructural
//	oc.Must = []string{"cn", "sn"}
//	oc.May = []string{"description", "telephoneNumber"}
//	schema.AddObjectClass(oc)
//
//	// Add attribute type
//	at := schema.NewAttributeType("2.5.4.3", "cn")
//	at.Syntax = "1.3.6.1.4.1.1466.115.121.1.15" // DirectoryString
//	at.SingleValue = false
//	schema.AddAttributeType(at)
//
// # Object Classes
//
// Object classes define the structure of entries:
//
//   - Structural: Primary object class (exactly one per entry)
//   - Auxiliary: Additional attributes (zero or more per entry)
//   - Abstract: Base classes that cannot be instantiated directly
//
// Example:
//
//	oc := &schema.ObjectClass{
//	    OID:      "2.5.6.6",
//	    Name:     "person",
//	    Superior: "top",
//	    Kind:     schema.ObjectClassStructural,
//	    Must:     []string{"cn", "sn"},
//	    May:      []string{"description", "telephoneNumber", "userPassword"},
//	}
//
// # Attribute Types
//
// Attribute types define attribute properties:
//
//	at := &schema.AttributeType{
//	    OID:         "2.5.4.3",
//	    Name:        "cn",
//	    Description: "Common Name",
//	    Syntax:      "1.3.6.1.4.1.1466.115.121.1.15",
//	    SingleValue: false,
//	    NoUserMod:   false,
//	}
//
// # Entry Validation
//
// Use the Validator to check entries against schema:
//
//	validator := schema.NewValidator(schema)
//
//	entry := &schema.Entry{
//	    DN: "uid=alice,ou=users,dc=example,dc=com",
//	    Attributes: map[string][][]byte{
//	        "objectClass": {[]byte("person"), []byte("top")},
//	        "cn":          {[]byte("Alice Smith")},
//	        "sn":          {[]byte("Smith")},
//	    },
//	}
//
//	if err := validator.ValidateEntry(entry); err != nil {
//	    // Entry violates schema
//	}
//
// # Loading Schema
//
// Load schema from LDIF or use built-in defaults:
//
//	// Load default schema
//	schema := schema.LoadDefaults()
//
//	// Load from LDIF file
//	schema, err := schema.LoadFromLDIF("/path/to/schema.ldif")
//
// # Standard Syntaxes
//
// Common LDAP syntaxes:
//
//   - 1.3.6.1.4.1.1466.115.121.1.15: DirectoryString (UTF-8)
//   - 1.3.6.1.4.1.1466.115.121.1.12: DN (Distinguished Name)
//   - 1.3.6.1.4.1.1466.115.121.1.27: Integer
//   - 1.3.6.1.4.1.1466.115.121.1.7: Boolean
//   - 1.3.6.1.4.1.1466.115.121.1.24: GeneralizedTime
package schema
