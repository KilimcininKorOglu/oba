// Package examples provides runnable examples demonstrating Oba LDAP server usage.
//
// This package contains example code showing how to use the various components
// of the Oba LDAP server. Each example is designed to be self-contained and
// demonstrates a specific feature or use case.
//
// # Available Examples
//
// The following examples are provided:
//
//   - basic_server: Simple LDAP server setup
//   - backend_usage: Working with the backend interface
//   - filter_evaluation: Building and evaluating search filters
//   - schema_validation: Validating entries against schema
//   - password_policy: Implementing password policies
//   - acl_configuration: Setting up access control
//
// # Running Examples
//
// Examples can be run directly:
//
//	go run examples/basic_server/main.go
//
// Or built and executed:
//
//	go build -o server examples/basic_server/main.go
//	./server
//
// # Testing with LDAP Clients
//
// After starting an example server, test with standard LDAP tools:
//
//	# Search
//	ldapsearch -x -H ldap://localhost:389 -b "dc=example,dc=com" "(objectClass=*)"
//
//	# Bind
//	ldapsearch -x -H ldap://localhost:389 -D "cn=admin,dc=example,dc=com" -w secret -b "dc=example,dc=com"
//
//	# Add entry
//	ldapadd -x -H ldap://localhost:389 -D "cn=admin,dc=example,dc=com" -w secret -f entry.ldif
package examples
