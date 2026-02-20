// Package acl provides Access Control List (ACL) data structures and evaluation
// for the Oba LDAP server.
//
// # Overview
//
// The acl package implements access control for LDAP operations. It provides:
//
//   - ACL rule definitions with target, subject, and rights
//   - Scope-based matching (base, one-level, subtree)
//   - Attribute-level permissions
//   - First-match evaluation with default policy
//
// # Access Rights
//
// Rights are bit flags that can be combined:
//
//	acl.Read     // Read entry attributes
//	acl.Write    // Modify entry attributes
//	acl.Add      // Create new entries
//	acl.Delete   // Remove entries
//	acl.Search   // Search for entries
//	acl.Compare  // Compare attribute values
//	acl.All      // All rights combined
//
// Example:
//
//	rights := acl.Read | acl.Search
//	if rights.Has(acl.Read) {
//	    // Read access granted
//	}
//
// # ACL Rules
//
// Create ACL rules to define access permissions:
//
//	// Allow admin full access to everything
//	rule := acl.NewACL("*", "cn=admin,dc=example,dc=com", acl.All)
//
//	// Allow authenticated users to read user entries
//	rule := acl.NewACL("ou=users,dc=example,dc=com", "authenticated", acl.Read|acl.Search).
//	    WithScope(acl.ScopeSubtree).
//	    WithAttributes("cn", "mail", "uid")
//
//	// Deny anonymous access to passwords
//	rule := acl.NewACL("*", "anonymous", acl.Read).
//	    WithAttributes("userPassword").
//	    WithDeny(true)
//
// # Subject Types
//
// The Subject field supports special values:
//
//   - "anonymous": Unauthenticated connections
//   - "authenticated": Any authenticated user
//   - "self": The entry being accessed (for self-modification)
//   - "*": Everyone (anonymous and authenticated)
//   - DN: Specific user DN
//
// # ACL Configuration
//
// Configure ACL with default policy and rules:
//
//	config := acl.NewConfig()
//	config.SetDefaultPolicy("deny") // Default deny
//
//	// Add rules in order of evaluation
//	config.AddRule(acl.NewACL("*", "cn=admin,dc=example,dc=com", acl.All))
//	config.AddRule(acl.NewACL("ou=users,dc=example,dc=com", "authenticated", acl.Read|acl.Search))
//
// # Access Evaluation
//
// Use the Evaluator to check access:
//
//	evaluator := acl.NewEvaluator(config)
//
//	ctx := acl.NewAccessContext(
//	    "uid=alice,ou=users,dc=example,dc=com", // Bind DN
//	    "uid=bob,ou=users,dc=example,dc=com",   // Target DN
//	    acl.Read,                                // Operation
//	).WithAttributes("cn", "mail")
//
//	if evaluator.Evaluate(ctx) {
//	    // Access granted
//	}
//
// # Evaluation Order
//
// Rules are evaluated in order; first match wins:
//
//  1. Check each rule in order
//  2. If rule matches target, subject, and operation, apply allow/deny
//  3. If no rule matches, apply default policy
package acl
