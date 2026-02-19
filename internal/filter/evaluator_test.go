package filter

import (
	"testing"

	"github.com/oba-ldap/oba/internal/schema"
)

// Helper function to create a test entry
func createTestEntry(dn string, attrs map[string][]string) *Entry {
	entry := NewEntry(dn)
	for name, values := range attrs {
		entry.SetStringAttribute(name, values...)
	}
	return entry
}

func TestNewEvaluator(t *testing.T) {
	t.Run("with nil schema", func(t *testing.T) {
		e := NewEvaluator(nil)
		if e == nil {
			t.Fatal("expected non-nil evaluator")
		}
		if e.schema != nil {
			t.Error("expected nil schema")
		}
	})

	t.Run("with schema", func(t *testing.T) {
		s := schema.NewSchema()
		e := NewEvaluator(s)
		if e == nil {
			t.Fatal("expected non-nil evaluator")
		}
		if e.schema != s {
			t.Error("expected schema to be set")
		}
	})
}

func TestEvaluateNilInputs(t *testing.T) {
	e := NewEvaluator(nil)
	entry := createTestEntry("uid=test,dc=example,dc=com", map[string][]string{
		"uid": {"test"},
	})

	t.Run("nil filter", func(t *testing.T) {
		if e.Evaluate(nil, entry) {
			t.Error("expected false for nil filter")
		}
	})

	t.Run("nil entry", func(t *testing.T) {
		filter := NewPresentFilter("uid")
		if e.Evaluate(filter, nil) {
			t.Error("expected false for nil entry")
		}
	})

	t.Run("both nil", func(t *testing.T) {
		if e.Evaluate(nil, nil) {
			t.Error("expected false for both nil")
		}
	})
}

func TestEvaluateEquality(t *testing.T) {
	e := NewEvaluator(nil)
	entry := createTestEntry("uid=alice,dc=example,dc=com", map[string][]string{
		"uid":         {"alice"},
		"cn":          {"Alice Smith"},
		"mail":        {"alice@example.com"},
		"objectClass": {"person", "inetOrgPerson"},
	})

	tests := []struct {
		name     string
		attr     string
		value    string
		expected bool
	}{
		{"exact match", "uid", "alice", true},
		{"case insensitive match", "uid", "ALICE", true},
		{"case insensitive match mixed", "uid", "AlIcE", true},
		{"no match", "uid", "bob", false},
		{"attribute not present", "description", "test", false},
		{"multi-value match first", "objectClass", "person", true},
		{"multi-value match second", "objectClass", "inetOrgPerson", true},
		{"multi-value no match", "objectClass", "group", false},
		{"case insensitive attr name", "UID", "alice", true},
		{"case insensitive attr name mixed", "Uid", "alice", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := NewEqualityFilter(tt.attr, []byte(tt.value))
			result := e.Evaluate(filter, entry)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestEvaluatePresent(t *testing.T) {
	e := NewEvaluator(nil)
	entry := createTestEntry("uid=alice,dc=example,dc=com", map[string][]string{
		"uid":  {"alice"},
		"cn":   {"Alice Smith"},
		"mail": {"alice@example.com"},
	})

	tests := []struct {
		name     string
		attr     string
		expected bool
	}{
		{"attribute present", "uid", true},
		{"attribute present case insensitive", "UID", true},
		{"attribute not present", "description", false},
		{"another present attribute", "mail", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := NewPresentFilter(tt.attr)
			result := e.Evaluate(filter, entry)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestEvaluateSubstring(t *testing.T) {
	e := NewEvaluator(nil)
	entry := createTestEntry("uid=alice,dc=example,dc=com", map[string][]string{
		"cn":   {"Alice Smith"},
		"mail": {"alice@example.com"},
	})

	tests := []struct {
		name     string
		attr     string
		initial  string
		any      []string
		final    string
		expected bool
	}{
		{"initial only match", "cn", "Alice", nil, "", true},
		{"initial only no match", "cn", "Bob", nil, "", false},
		{"final only match", "cn", "", nil, "Smith", true},
		{"final only no match", "cn", "", nil, "Jones", false},
		{"any only match", "cn", "", []string{"ice"}, "", true},
		{"any only no match", "cn", "", []string{"xyz"}, "", false},
		{"initial and final match", "cn", "Alice", nil, "Smith", true},
		{"initial and final no match", "cn", "Alice", nil, "Jones", false},
		{"all components match", "mail", "alice", []string{"example"}, "com", true},
		{"case insensitive initial", "cn", "ALICE", nil, "", true},
		{"case insensitive final", "cn", "", nil, "SMITH", true},
		{"case insensitive any", "cn", "", []string{"ICE"}, "", true},
		{"multiple any match", "mail", "", []string{"@", "."}, "", true},
		{"attribute not present", "description", "test", nil, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var anyBytes [][]byte
			for _, s := range tt.any {
				anyBytes = append(anyBytes, []byte(s))
			}
			sf := &SubstringFilter{
				Attribute: tt.attr,
				Initial:   []byte(tt.initial),
				Any:       anyBytes,
				Final:     []byte(tt.final),
			}
			filter := NewSubstringFilter(sf)
			result := e.Evaluate(filter, entry)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestEvaluateSubstringNilFilter(t *testing.T) {
	e := NewEvaluator(nil)
	entry := createTestEntry("uid=test,dc=example,dc=com", map[string][]string{
		"uid": {"test"},
	})

	filter := &Filter{
		Type:      FilterSubstring,
		Attribute: "uid",
		Substring: nil,
	}

	if e.Evaluate(filter, entry) {
		t.Error("expected false for nil substring filter")
	}
}

func TestEvaluateGreaterOrEqual(t *testing.T) {
	e := NewEvaluator(nil)
	entry := createTestEntry("uid=alice,dc=example,dc=com", map[string][]string{
		"cn":          {"Alice"},
		"uidNumber":   {"1000"},
		"objectClass": {"person", "inetOrgPerson"},
	})

	tests := []struct {
		name     string
		attr     string
		value    string
		expected bool
	}{
		{"equal value", "cn", "Alice", true},
		{"equal value case insensitive", "cn", "alice", true},
		{"greater value", "cn", "Aaron", true},
		{"less value", "cn", "Bob", false},
		{"numeric equal", "uidNumber", "1000", true},
		// Note: lexicographic comparison - "1000" < "500" because "1" < "5"
		{"numeric lexicographic less", "uidNumber", "500", false},
		{"numeric lexicographic greater", "uidNumber", "0999", true},
		{"attribute not present", "description", "test", false},
		{"multi-value one matches", "objectClass", "person", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := NewGreaterOrEqualFilter(tt.attr, []byte(tt.value))
			result := e.Evaluate(filter, entry)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestEvaluateLessOrEqual(t *testing.T) {
	e := NewEvaluator(nil)
	entry := createTestEntry("uid=alice,dc=example,dc=com", map[string][]string{
		"cn":        {"Alice"},
		"uidNumber": {"1000"},
	})

	tests := []struct {
		name     string
		attr     string
		value    string
		expected bool
	}{
		{"equal value", "cn", "Alice", true},
		{"equal value case insensitive", "cn", "alice", true},
		{"less value", "cn", "Bob", true},
		{"greater value", "cn", "Aaron", false},
		{"numeric equal", "uidNumber", "1000", true},
		// Note: lexicographic comparison - "1000" < "2000" because "1" < "2"
		{"numeric lexicographic less", "uidNumber", "2000", true},
		{"numeric lexicographic greater", "uidNumber", "0999", false},
		{"attribute not present", "description", "test", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := NewLessOrEqualFilter(tt.attr, []byte(tt.value))
			result := e.Evaluate(filter, entry)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestEvaluateApproxMatch(t *testing.T) {
	e := NewEvaluator(nil)
	entry := createTestEntry("uid=alice,dc=example,dc=com", map[string][]string{
		"cn": {"Alice  Smith"},
	})

	tests := []struct {
		name     string
		attr     string
		value    string
		expected bool
	}{
		{"exact match", "cn", "Alice  Smith", true},
		{"normalized whitespace", "cn", "Alice Smith", true},
		{"case insensitive", "cn", "alice smith", true},
		{"extra whitespace in value", "cn", "Alice   Smith", true},
		{"no match", "cn", "Bob Jones", false},
		{"attribute not present", "description", "test", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := NewApproxMatchFilter(tt.attr, []byte(tt.value))
			result := e.Evaluate(filter, entry)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestEvaluateAnd(t *testing.T) {
	e := NewEvaluator(nil)
	entry := createTestEntry("uid=alice,dc=example,dc=com", map[string][]string{
		"uid":         {"alice"},
		"cn":          {"Alice Smith"},
		"objectClass": {"person", "inetOrgPerson"},
	})

	t.Run("all match", func(t *testing.T) {
		filter := NewAndFilter(
			NewEqualityFilter("uid", []byte("alice")),
			NewEqualityFilter("cn", []byte("Alice Smith")),
		)
		if !e.Evaluate(filter, entry) {
			t.Error("expected true when all children match")
		}
	})

	t.Run("one does not match", func(t *testing.T) {
		filter := NewAndFilter(
			NewEqualityFilter("uid", []byte("alice")),
			NewEqualityFilter("cn", []byte("Bob Jones")),
		)
		if e.Evaluate(filter, entry) {
			t.Error("expected false when one child does not match")
		}
	})

	t.Run("none match", func(t *testing.T) {
		filter := NewAndFilter(
			NewEqualityFilter("uid", []byte("bob")),
			NewEqualityFilter("cn", []byte("Bob Jones")),
		)
		if e.Evaluate(filter, entry) {
			t.Error("expected false when no children match")
		}
	})

	t.Run("empty AND", func(t *testing.T) {
		filter := NewAndFilter()
		if !e.Evaluate(filter, entry) {
			t.Error("expected true for empty AND (vacuous truth)")
		}
	})

	t.Run("single child match", func(t *testing.T) {
		filter := NewAndFilter(
			NewEqualityFilter("uid", []byte("alice")),
		)
		if !e.Evaluate(filter, entry) {
			t.Error("expected true when single child matches")
		}
	})

	t.Run("single child no match", func(t *testing.T) {
		filter := NewAndFilter(
			NewEqualityFilter("uid", []byte("bob")),
		)
		if e.Evaluate(filter, entry) {
			t.Error("expected false when single child does not match")
		}
	})

	t.Run("three children all match", func(t *testing.T) {
		filter := NewAndFilter(
			NewEqualityFilter("uid", []byte("alice")),
			NewPresentFilter("cn"),
			NewEqualityFilter("objectClass", []byte("person")),
		)
		if !e.Evaluate(filter, entry) {
			t.Error("expected true when all three children match")
		}
	})
}

func TestEvaluateOr(t *testing.T) {
	e := NewEvaluator(nil)
	entry := createTestEntry("uid=alice,dc=example,dc=com", map[string][]string{
		"uid":         {"alice"},
		"cn":          {"Alice Smith"},
		"objectClass": {"person", "inetOrgPerson"},
	})

	t.Run("all match", func(t *testing.T) {
		filter := NewOrFilter(
			NewEqualityFilter("uid", []byte("alice")),
			NewEqualityFilter("cn", []byte("Alice Smith")),
		)
		if !e.Evaluate(filter, entry) {
			t.Error("expected true when all children match")
		}
	})

	t.Run("one matches", func(t *testing.T) {
		filter := NewOrFilter(
			NewEqualityFilter("uid", []byte("alice")),
			NewEqualityFilter("cn", []byte("Bob Jones")),
		)
		if !e.Evaluate(filter, entry) {
			t.Error("expected true when one child matches")
		}
	})

	t.Run("none match", func(t *testing.T) {
		filter := NewOrFilter(
			NewEqualityFilter("uid", []byte("bob")),
			NewEqualityFilter("cn", []byte("Bob Jones")),
		)
		if e.Evaluate(filter, entry) {
			t.Error("expected false when no children match")
		}
	})

	t.Run("empty OR", func(t *testing.T) {
		filter := NewOrFilter()
		if e.Evaluate(filter, entry) {
			t.Error("expected false for empty OR")
		}
	})

	t.Run("single child match", func(t *testing.T) {
		filter := NewOrFilter(
			NewEqualityFilter("uid", []byte("alice")),
		)
		if !e.Evaluate(filter, entry) {
			t.Error("expected true when single child matches")
		}
	})

	t.Run("single child no match", func(t *testing.T) {
		filter := NewOrFilter(
			NewEqualityFilter("uid", []byte("bob")),
		)
		if e.Evaluate(filter, entry) {
			t.Error("expected false when single child does not match")
		}
	})

	t.Run("three children one matches", func(t *testing.T) {
		filter := NewOrFilter(
			NewEqualityFilter("uid", []byte("bob")),
			NewEqualityFilter("cn", []byte("Bob Jones")),
			NewEqualityFilter("objectClass", []byte("person")),
		)
		if !e.Evaluate(filter, entry) {
			t.Error("expected true when one of three children matches")
		}
	})
}

func TestEvaluateNot(t *testing.T) {
	e := NewEvaluator(nil)
	entry := createTestEntry("uid=alice,dc=example,dc=com", map[string][]string{
		"uid": {"alice"},
		"cn":  {"Alice Smith"},
	})

	t.Run("negates true to false", func(t *testing.T) {
		filter := NewNotFilter(NewEqualityFilter("uid", []byte("alice")))
		if e.Evaluate(filter, entry) {
			t.Error("expected false when child matches")
		}
	})

	t.Run("negates false to true", func(t *testing.T) {
		filter := NewNotFilter(NewEqualityFilter("uid", []byte("bob")))
		if !e.Evaluate(filter, entry) {
			t.Error("expected true when child does not match")
		}
	})

	t.Run("nil child", func(t *testing.T) {
		filter := &Filter{
			Type:  FilterNot,
			Child: nil,
		}
		if e.Evaluate(filter, entry) {
			t.Error("expected false for nil child")
		}
	})

	t.Run("double negation", func(t *testing.T) {
		filter := NewNotFilter(NewNotFilter(NewEqualityFilter("uid", []byte("alice"))))
		if !e.Evaluate(filter, entry) {
			t.Error("expected true for double negation of matching filter")
		}
	})
}

func TestComplexFilters(t *testing.T) {
	e := NewEvaluator(nil)
	entry := createTestEntry("uid=alice,dc=example,dc=com", map[string][]string{
		"uid":         {"alice"},
		"cn":          {"Alice Smith"},
		"mail":        {"alice@example.com"},
		"objectClass": {"person", "inetOrgPerson"},
		"uidNumber":   {"1000"},
	})

	t.Run("AND with OR child", func(t *testing.T) {
		// (&(objectClass=person)(|(uid=alice)(uid=bob)))
		filter := NewAndFilter(
			NewEqualityFilter("objectClass", []byte("person")),
			NewOrFilter(
				NewEqualityFilter("uid", []byte("alice")),
				NewEqualityFilter("uid", []byte("bob")),
			),
		)
		if !e.Evaluate(filter, entry) {
			t.Error("expected true for complex AND with OR")
		}
	})

	t.Run("OR with AND children", func(t *testing.T) {
		// (|(&(uid=alice)(cn=Alice Smith))(&(uid=bob)(cn=Bob Jones)))
		filter := NewOrFilter(
			NewAndFilter(
				NewEqualityFilter("uid", []byte("alice")),
				NewEqualityFilter("cn", []byte("Alice Smith")),
			),
			NewAndFilter(
				NewEqualityFilter("uid", []byte("bob")),
				NewEqualityFilter("cn", []byte("Bob Jones")),
			),
		)
		if !e.Evaluate(filter, entry) {
			t.Error("expected true for complex OR with AND children")
		}
	})

	t.Run("NOT with AND child", func(t *testing.T) {
		// (!(&(uid=bob)(cn=Bob Jones)))
		filter := NewNotFilter(
			NewAndFilter(
				NewEqualityFilter("uid", []byte("bob")),
				NewEqualityFilter("cn", []byte("Bob Jones")),
			),
		)
		if !e.Evaluate(filter, entry) {
			t.Error("expected true for NOT with non-matching AND")
		}
	})

	t.Run("deeply nested filter", func(t *testing.T) {
		// (&(objectClass=person)(|(uid=alice)(&(cn=*Smith)(mail=*@example.com))))
		filter := NewAndFilter(
			NewEqualityFilter("objectClass", []byte("person")),
			NewOrFilter(
				NewEqualityFilter("uid", []byte("alice")),
				NewAndFilter(
					NewSubstringFilter(&SubstringFilter{
						Attribute: "cn",
						Final:     []byte("Smith"),
					}),
					NewSubstringFilter(&SubstringFilter{
						Attribute: "mail",
						Any:       [][]byte{[]byte("@example.com")},
					}),
				),
			),
		)
		if !e.Evaluate(filter, entry) {
			t.Error("expected true for deeply nested filter")
		}
	})

	t.Run("mixed filter types", func(t *testing.T) {
		// (&(uid=*)(cn=Alice*)(uidNumber>=0500)(!(objectClass=group)))
		// Note: using "0500" because lexicographic comparison: "1000" >= "0500" is true
		filter := NewAndFilter(
			NewPresentFilter("uid"),
			NewSubstringFilter(&SubstringFilter{
				Attribute: "cn",
				Initial:   []byte("Alice"),
			}),
			NewGreaterOrEqualFilter("uidNumber", []byte("0500")),
			NewNotFilter(NewEqualityFilter("objectClass", []byte("group"))),
		)
		if !e.Evaluate(filter, entry) {
			t.Error("expected true for mixed filter types")
		}
	})
}

func TestFilterTypeString(t *testing.T) {
	tests := []struct {
		ft       FilterType
		expected string
	}{
		{FilterAnd, "AND"},
		{FilterOr, "OR"},
		{FilterNot, "NOT"},
		{FilterEquality, "EQUALITY"},
		{FilterSubstring, "SUBSTRING"},
		{FilterGreaterOrEqual, "GREATER_OR_EQUAL"},
		{FilterLessOrEqual, "LESS_OR_EQUAL"},
		{FilterPresent, "PRESENT"},
		{FilterApproxMatch, "APPROX_MATCH"},
		{FilterExtensibleMatch, "EXTENSIBLE_MATCH"},
		{FilterType(999), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.ft.String(); got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestEntryMethods(t *testing.T) {
	t.Run("NewEntry", func(t *testing.T) {
		entry := NewEntry("uid=test,dc=example,dc=com")
		if entry.DN != "uid=test,dc=example,dc=com" {
			t.Errorf("unexpected DN: %s", entry.DN)
		}
		if entry.Attributes == nil {
			t.Error("expected non-nil Attributes map")
		}
	})

	t.Run("SetAttribute and GetAttribute", func(t *testing.T) {
		entry := NewEntry("uid=test,dc=example,dc=com")
		entry.SetAttribute("uid", []byte("test"))
		values := entry.GetAttribute("uid")
		if len(values) != 1 || string(values[0]) != "test" {
			t.Error("unexpected attribute values")
		}
	})

	t.Run("SetStringAttribute", func(t *testing.T) {
		entry := NewEntry("uid=test,dc=example,dc=com")
		entry.SetStringAttribute("cn", "Test User", "Another Name")
		values := entry.GetAttribute("cn")
		if len(values) != 2 {
			t.Errorf("expected 2 values, got %d", len(values))
		}
		if string(values[0]) != "Test User" {
			t.Errorf("unexpected first value: %s", string(values[0]))
		}
	})

	t.Run("HasAttribute", func(t *testing.T) {
		entry := NewEntry("uid=test,dc=example,dc=com")
		entry.SetStringAttribute("uid", "test")
		if !entry.HasAttribute("uid") {
			t.Error("expected HasAttribute to return true")
		}
		if entry.HasAttribute("cn") {
			t.Error("expected HasAttribute to return false for missing attribute")
		}
	})

	t.Run("Clone", func(t *testing.T) {
		entry := NewEntry("uid=test,dc=example,dc=com")
		entry.SetStringAttribute("uid", "test")
		entry.SetStringAttribute("cn", "Test User")

		clone := entry.Clone()
		if clone.DN != entry.DN {
			t.Error("clone DN mismatch")
		}
		if len(clone.Attributes) != len(entry.Attributes) {
			t.Error("clone attributes count mismatch")
		}

		// Modify original and verify clone is independent
		entry.SetStringAttribute("uid", "modified")
		if string(clone.GetAttribute("uid")[0]) == "modified" {
			t.Error("clone should be independent of original")
		}
	})

	t.Run("Clone nil", func(t *testing.T) {
		var entry *Entry
		clone := entry.Clone()
		if clone != nil {
			t.Error("expected nil clone for nil entry")
		}
	})
}

func TestEvaluatorSchemaAccessors(t *testing.T) {
	t.Run("GetSchema", func(t *testing.T) {
		s := schema.NewSchema()
		e := NewEvaluator(s)
		if e.GetSchema() != s {
			t.Error("GetSchema returned wrong schema")
		}
	})

	t.Run("SetSchema", func(t *testing.T) {
		e := NewEvaluator(nil)
		s := schema.NewSchema()
		e.SetSchema(s)
		if e.GetSchema() != s {
			t.Error("SetSchema did not update schema")
		}
	})
}

func TestUnknownFilterType(t *testing.T) {
	e := NewEvaluator(nil)
	entry := createTestEntry("uid=test,dc=example,dc=com", map[string][]string{
		"uid": {"test"},
	})

	filter := &Filter{
		Type: FilterType(999),
	}

	if e.Evaluate(filter, entry) {
		t.Error("expected false for unknown filter type")
	}
}
