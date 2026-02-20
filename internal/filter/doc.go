// Package filter provides LDAP search filter data structures and evaluation
// for the Oba LDAP server.
//
// # Overview
//
// The filter package implements LDAP search filter parsing, representation,
// and evaluation as defined in RFC 4511. It supports all standard filter types:
//
//   - AND (&): Logical conjunction of filters
//   - OR (|): Logical disjunction of filters
//   - NOT (!): Logical negation of a filter
//   - Equality (=): Exact attribute value match
//   - Substring (*): Pattern matching with wildcards
//   - Greater-or-Equal (>=): Comparison filter
//   - Less-or-Equal (<=): Comparison filter
//   - Present (=*): Attribute existence check
//   - Approximate (~=): Fuzzy matching
//
// # Filter Construction
//
// Filters can be constructed programmatically:
//
//	// Simple equality filter: (uid=alice)
//	f := filter.NewEqualityFilter("uid", []byte("alice"))
//
//	// Presence filter: (mail=*)
//	f := filter.NewPresentFilter("mail")
//
//	// AND filter: (&(objectClass=person)(uid=alice))
//	f := filter.NewAndFilter(
//	    filter.NewEqualityFilter("objectClass", []byte("person")),
//	    filter.NewEqualityFilter("uid", []byte("alice")),
//	)
//
//	// NOT filter: (!(status=disabled))
//	f := filter.NewNotFilter(
//	    filter.NewEqualityFilter("status", []byte("disabled")),
//	)
//
// # Substring Filters
//
// Substring filters support initial, any, and final components:
//
//	// (cn=John*)
//	sf := &filter.SubstringFilter{
//	    Attribute: "cn",
//	    Initial:   []byte("John"),
//	}
//	f := filter.NewSubstringFilter(sf)
//
//	// (cn=*Smith)
//	sf := &filter.SubstringFilter{
//	    Attribute: "cn",
//	    Final:     []byte("Smith"),
//	}
//
//	// (cn=*admin*)
//	sf := &filter.SubstringFilter{
//	    Attribute: "cn",
//	    Any:       [][]byte{[]byte("admin")},
//	}
//
// # Filter Evaluation
//
// Use the Evaluator to test entries against filters:
//
//	evaluator := filter.NewEvaluator(schema)
//
//	entry := filter.NewEntry("uid=alice,ou=users,dc=example,dc=com")
//	entry.SetStringAttribute("objectClass", "person", "top")
//	entry.SetStringAttribute("uid", "alice")
//	entry.SetStringAttribute("cn", "Alice Smith")
//
//	f := filter.NewEqualityFilter("uid", []byte("alice"))
//
//	if evaluator.Evaluate(f, entry) {
//	    // Entry matches filter
//	}
//
// # Filter Optimization
//
// The Optimizer can reorder and simplify filters for better performance:
//
//	optimizer := filter.NewOptimizer()
//	optimized := optimizer.Optimize(f)
//
// Optimization strategies include:
//
//   - Reordering AND/OR children by selectivity
//   - Eliminating redundant filters
//   - Simplifying nested structures
package filter
