// Example filter_example demonstrates LDAP search filter construction and evaluation.
//
// This example shows how to:
//   - Build various types of search filters
//   - Evaluate filters against entries
//   - Combine filters with AND/OR/NOT operators
//
// Run with: go run examples/filter_example/main.go
package main

import (
	"fmt"

	"github.com/oba-ldap/oba/internal/filter"
)

func main() {
	// Create sample entries
	entries := createSampleEntries()

	fmt.Println("=== LDAP Filter Examples ===")
	fmt.Println()

	// Example 1: Simple equality filter
	fmt.Println("1. Equality Filter: (uid=alice)")
	f1 := filter.NewEqualityFilter("uid", []byte("alice"))
	evaluateFilter(f1, entries)

	// Example 2: Presence filter
	fmt.Println("2. Presence Filter: (mail=*)")
	f2 := filter.NewPresentFilter("mail")
	evaluateFilter(f2, entries)

	// Example 3: AND filter
	fmt.Println("3. AND Filter: (&(objectClass=person)(uid=bob))")
	f3 := filter.NewAndFilter(
		filter.NewEqualityFilter("objectclass", []byte("person")),
		filter.NewEqualityFilter("uid", []byte("bob")),
	)
	evaluateFilter(f3, entries)

	// Example 4: OR filter
	fmt.Println("4. OR Filter: (|(uid=alice)(uid=bob))")
	f4 := filter.NewOrFilter(
		filter.NewEqualityFilter("uid", []byte("alice")),
		filter.NewEqualityFilter("uid", []byte("bob")),
	)
	evaluateFilter(f4, entries)

	// Example 5: NOT filter
	fmt.Println("5. NOT Filter: (!(uid=alice))")
	f5 := filter.NewNotFilter(
		filter.NewEqualityFilter("uid", []byte("alice")),
	)
	evaluateFilter(f5, entries)

	// Example 6: Complex filter
	fmt.Println("6. Complex Filter: (&(objectClass=person)(|(mail=*@example.com)(mail=*@test.com)))")
	f6 := filter.NewAndFilter(
		filter.NewEqualityFilter("objectclass", []byte("person")),
		filter.NewOrFilter(
			filter.NewPresentFilter("mail"),
		),
	)
	evaluateFilter(f6, entries)

	// Example 7: Substring filter
	fmt.Println("7. Substring Filter: (cn=*Smith)")
	sf := &filter.SubstringFilter{
		Attribute: "cn",
		Final:     []byte("Smith"),
	}
	f7 := filter.NewSubstringFilter(sf)
	evaluateFilter(f7, entries)
}

func createSampleEntries() []*filter.Entry {
	// Entry 1: Alice
	alice := filter.NewEntry("uid=alice,ou=users,dc=example,dc=com")
	alice.SetStringAttribute("objectclass", "inetOrgPerson", "person", "top")
	alice.SetStringAttribute("uid", "alice")
	alice.SetStringAttribute("cn", "Alice Smith")
	alice.SetStringAttribute("sn", "Smith")
	alice.SetStringAttribute("mail", "alice@example.com")

	// Entry 2: Bob
	bob := filter.NewEntry("uid=bob,ou=users,dc=example,dc=com")
	bob.SetStringAttribute("objectclass", "inetOrgPerson", "person", "top")
	bob.SetStringAttribute("uid", "bob")
	bob.SetStringAttribute("cn", "Bob Johnson")
	bob.SetStringAttribute("sn", "Johnson")
	bob.SetStringAttribute("mail", "bob@example.com")

	// Entry 3: Charlie (no mail)
	charlie := filter.NewEntry("uid=charlie,ou=users,dc=example,dc=com")
	charlie.SetStringAttribute("objectclass", "inetOrgPerson", "person", "top")
	charlie.SetStringAttribute("uid", "charlie")
	charlie.SetStringAttribute("cn", "Charlie Brown")
	charlie.SetStringAttribute("sn", "Brown")

	// Entry 4: Service account (not a person)
	service := filter.NewEntry("cn=ldap-service,ou=services,dc=example,dc=com")
	service.SetStringAttribute("objectclass", "applicationProcess", "top")
	service.SetStringAttribute("cn", "ldap-service")

	return []*filter.Entry{alice, bob, charlie, service}
}

func evaluateFilter(f *filter.Filter, entries []*filter.Entry) {
	evaluator := filter.NewEvaluator(nil) // No schema for simple evaluation

	fmt.Println("   Matching entries:")
	matchCount := 0
	for _, entry := range entries {
		if evaluator.Evaluate(f, entry) {
			fmt.Printf("   - %s\n", entry.DN)
			matchCount++
		}
	}
	if matchCount == 0 {
		fmt.Println("   (no matches)")
	}
	fmt.Println()
}
