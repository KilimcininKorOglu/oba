// Example acl_example demonstrates access control list configuration and evaluation.
//
// This example shows how to:
//   - Define ACL rules with targets, subjects, and rights
//   - Configure default policies
//   - Evaluate access requests
//   - Use attribute-level permissions
//
// Run with: go run examples/acl_example/main.go
package main

import (
	"fmt"

	"github.com/oba-ldap/oba/internal/acl"
)

func main() {
	fmt.Println("=== ACL Configuration Examples ===")
	fmt.Println()

	// Create ACL configuration
	config := acl.NewConfig()
	config.SetDefaultPolicy("deny")

	// Rule 1: Admin has full access to everything
	adminRule := acl.NewACL("*", "cn=admin,dc=example,dc=com", acl.All)
	config.AddRule(adminRule)

	// Rule 2: Authenticated users can read user entries
	authReadRule := acl.NewACL("ou=users,dc=example,dc=com", "authenticated", acl.Read|acl.Search).
		WithScope(acl.ScopeSubtree)
	config.AddRule(authReadRule)

	// Rule 3: Users can modify their own entries
	selfModifyRule := acl.NewACL("ou=users,dc=example,dc=com", "self", acl.Read|acl.Write).
		WithScope(acl.ScopeSubtree)
	config.AddRule(selfModifyRule)

	// Rule 4: Deny anonymous access to passwords
	denyPasswordRule := acl.NewACL("*", "anonymous", acl.Read).
		WithAttributes("userPassword").
		WithDeny(true)
	config.AddRule(denyPasswordRule)

	// Rule 5: Allow anonymous read access to public attributes
	anonReadRule := acl.NewACL("ou=users,dc=example,dc=com", "anonymous", acl.Read|acl.Search).
		WithScope(acl.ScopeSubtree).
		WithAttributes("cn", "mail", "uid")
	config.AddRule(anonReadRule)

	fmt.Println("ACL Rules:")
	fmt.Println("  1. Admin has full access to everything")
	fmt.Println("  2. Authenticated users can read user entries")
	fmt.Println("  3. Users can modify their own entries")
	fmt.Println("  4. Deny anonymous access to passwords")
	fmt.Println("  5. Allow anonymous read access to public attributes")
	fmt.Printf("  Default Policy: %s\n", config.DefaultPolicy)
	fmt.Println()

	// Create evaluator
	evaluator := acl.NewEvaluator(config)

	// Test scenarios
	testCases := []struct {
		name      string
		bindDN    string
		targetDN  string
		operation acl.Right
		attrs     []string
	}{
		{
			name:      "Admin reading any entry",
			bindDN:    "cn=admin,dc=example,dc=com",
			targetDN:  "uid=alice,ou=users,dc=example,dc=com",
			operation: acl.Read,
		},
		{
			name:      "Admin deleting entry",
			bindDN:    "cn=admin,dc=example,dc=com",
			targetDN:  "uid=alice,ou=users,dc=example,dc=com",
			operation: acl.Delete,
		},
		{
			name:      "User reading another user",
			bindDN:    "uid=bob,ou=users,dc=example,dc=com",
			targetDN:  "uid=alice,ou=users,dc=example,dc=com",
			operation: acl.Read,
		},
		{
			name:      "User modifying own entry",
			bindDN:    "uid=alice,ou=users,dc=example,dc=com",
			targetDN:  "uid=alice,ou=users,dc=example,dc=com",
			operation: acl.Write,
		},
		{
			name:      "User modifying another user",
			bindDN:    "uid=bob,ou=users,dc=example,dc=com",
			targetDN:  "uid=alice,ou=users,dc=example,dc=com",
			operation: acl.Write,
		},
		{
			name:      "Anonymous reading public attrs",
			bindDN:    "",
			targetDN:  "uid=alice,ou=users,dc=example,dc=com",
			operation: acl.Read,
			attrs:     []string{"cn", "mail"},
		},
		{
			name:      "Anonymous reading password",
			bindDN:    "",
			targetDN:  "uid=alice,ou=users,dc=example,dc=com",
			operation: acl.Read,
			attrs:     []string{"userPassword"},
		},
		{
			name:      "Anonymous searching users",
			bindDN:    "",
			targetDN:  "ou=users,dc=example,dc=com",
			operation: acl.Search,
		},
	}

	fmt.Println("Access Evaluation Results:")
	for _, tc := range testCases {
		ctx := acl.NewAccessContext(tc.bindDN, tc.targetDN, tc.operation)
		if len(tc.attrs) > 0 {
			ctx = ctx.WithAttributes(tc.attrs...)
		}

		allowed := evaluator.CheckAccess(ctx)
		status := "DENIED"
		if allowed {
			status = "ALLOWED"
		}

		fmt.Printf("  %-35s %s\n", tc.name+":", status)
	}
	fmt.Println()

	// Demonstrate rights checking
	fmt.Println("Rights Checking:")
	rights := acl.Read | acl.Search
	fmt.Printf("  Rights: Read|Search\n")
	fmt.Printf("  Has Read: %v\n", rights.Has(acl.Read))
	fmt.Printf("  Has Write: %v\n", rights.Has(acl.Write))
	fmt.Printf("  Has Search: %v\n", rights.Has(acl.Search))
	fmt.Printf("  Has Delete: %v\n", rights.Has(acl.Delete))
}
