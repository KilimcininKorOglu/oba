package acl

import (
	"testing"
)

func TestMatcherMatchesTarget(t *testing.T) {
	m := NewMatcher()

	tests := []struct {
		name     string
		target   string
		scope    Scope
		targetDN string
		expected bool
	}{
		// Wildcard tests
		{"wildcard matches any", "*", ScopeSubtree, "uid=alice,ou=users,dc=example,dc=com", true},
		{"wildcard matches root", "*", ScopeSubtree, "dc=com", true},

		// Base scope tests
		{"base exact match", "ou=users,dc=example,dc=com", ScopeBase, "ou=users,dc=example,dc=com", true},
		{"base no match child", "ou=users,dc=example,dc=com", ScopeBase, "uid=alice,ou=users,dc=example,dc=com", false},
		{"base no match parent", "ou=users,dc=example,dc=com", ScopeBase, "dc=example,dc=com", false},

		// One scope tests
		{"one matches immediate child", "ou=users,dc=example,dc=com", ScopeOne, "uid=alice,ou=users,dc=example,dc=com", true},
		{"one no match self", "ou=users,dc=example,dc=com", ScopeOne, "ou=users,dc=example,dc=com", false},
		{"one no match grandchild", "dc=example,dc=com", ScopeOne, "uid=alice,ou=users,dc=example,dc=com", false},
		{"one matches direct child", "dc=example,dc=com", ScopeOne, "ou=users,dc=example,dc=com", true},

		// Subtree scope tests
		{"subtree matches self", "ou=users,dc=example,dc=com", ScopeSubtree, "ou=users,dc=example,dc=com", true},
		{"subtree matches child", "ou=users,dc=example,dc=com", ScopeSubtree, "uid=alice,ou=users,dc=example,dc=com", true},
		{"subtree matches grandchild", "dc=example,dc=com", ScopeSubtree, "uid=alice,ou=users,dc=example,dc=com", true},
		{"subtree no match sibling", "ou=groups,dc=example,dc=com", ScopeSubtree, "uid=alice,ou=users,dc=example,dc=com", false},
		{"subtree no match parent", "ou=users,dc=example,dc=com", ScopeSubtree, "dc=example,dc=com", false},

		// Case insensitivity
		{"case insensitive match", "OU=USERS,DC=EXAMPLE,DC=COM", ScopeBase, "ou=users,dc=example,dc=com", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := &ACL{Target: tt.target, Scope: tt.scope}
			result := m.MatchesTarget(rule, tt.targetDN)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestMatcherMatchesSubject(t *testing.T) {
	m := NewMatcher()

	tests := []struct {
		name     string
		subject  string
		bindDN   string
		targetDN string
		expected bool
	}{
		// Anonymous tests
		{"anonymous matches empty", "anonymous", "", "ou=users,dc=example,dc=com", true},
		{"anonymous no match authenticated", "anonymous", "uid=alice,dc=example,dc=com", "ou=users,dc=example,dc=com", false},

		// Authenticated tests
		{"authenticated matches non-empty", "authenticated", "uid=alice,dc=example,dc=com", "ou=users,dc=example,dc=com", true},
		{"authenticated no match empty", "authenticated", "", "ou=users,dc=example,dc=com", false},

		// Self tests
		{"self matches same DN", "self", "uid=alice,dc=example,dc=com", "uid=alice,dc=example,dc=com", true},
		{"self no match different DN", "self", "uid=alice,dc=example,dc=com", "uid=bob,dc=example,dc=com", false},
		{"self no match anonymous", "self", "", "uid=alice,dc=example,dc=com", false},
		{"self case insensitive", "self", "UID=Alice,DC=Example,DC=Com", "uid=alice,dc=example,dc=com", true},

		// Wildcard tests
		{"wildcard matches authenticated", "*", "uid=alice,dc=example,dc=com", "ou=users,dc=example,dc=com", true},
		{"wildcard matches anonymous", "*", "", "ou=users,dc=example,dc=com", true},

		// Specific DN tests
		{"specific DN matches exact", "uid=admin,dc=example,dc=com", "uid=admin,dc=example,dc=com", "ou=users,dc=example,dc=com", true},
		{"specific DN no match different", "uid=admin,dc=example,dc=com", "uid=alice,dc=example,dc=com", "ou=users,dc=example,dc=com", false},
		{"specific DN case insensitive", "UID=Admin,DC=Example,DC=Com", "uid=admin,dc=example,dc=com", "ou=users,dc=example,dc=com", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := &ACL{Subject: tt.subject}
			result := m.MatchesSubject(rule, tt.bindDN, tt.targetDN)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestMatcherIsImmediateChild(t *testing.T) {
	m := NewMatcher()

	tests := []struct {
		name     string
		parent   string
		target   string
		expected bool
	}{
		{"immediate child", "ou=users,dc=example,dc=com", "uid=alice,ou=users,dc=example,dc=com", true},
		{"not a child - same", "ou=users,dc=example,dc=com", "ou=users,dc=example,dc=com", false},
		{"not a child - grandchild", "dc=example,dc=com", "uid=alice,ou=users,dc=example,dc=com", false},
		{"not a child - sibling", "ou=groups,dc=example,dc=com", "uid=alice,ou=users,dc=example,dc=com", false},
		{"empty parent", "", "uid=alice,dc=example,dc=com", false},
		{"empty target", "ou=users,dc=example,dc=com", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.isImmediateChild(tt.parent, tt.target)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestMatcherIsSubtreeMatch(t *testing.T) {
	m := NewMatcher()

	tests := []struct {
		name     string
		base     string
		target   string
		expected bool
	}{
		{"exact match", "ou=users,dc=example,dc=com", "ou=users,dc=example,dc=com", true},
		{"child match", "ou=users,dc=example,dc=com", "uid=alice,ou=users,dc=example,dc=com", true},
		{"grandchild match", "dc=example,dc=com", "uid=alice,ou=users,dc=example,dc=com", true},
		{"no match - sibling", "ou=groups,dc=example,dc=com", "uid=alice,ou=users,dc=example,dc=com", false},
		{"no match - parent", "uid=alice,ou=users,dc=example,dc=com", "ou=users,dc=example,dc=com", false},
		{"empty base matches all", "", "uid=alice,dc=example,dc=com", true},
		{"empty target no match", "ou=users,dc=example,dc=com", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.isSubtreeMatch(tt.base, tt.target)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestMatcherParseDN(t *testing.T) {
	m := NewMatcher()

	tests := []struct {
		name     string
		dn       string
		expected []string
	}{
		{"simple DN", "uid=alice,ou=users,dc=example,dc=com", []string{"uid=alice", "ou=users", "dc=example", "dc=com"}},
		{"single component", "dc=com", []string{"dc=com"}},
		{"empty DN", "", nil},
		{"with spaces", "uid=alice , ou=users , dc=example , dc=com", []string{"uid=alice", "ou=users", "dc=example", "dc=com"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.ParseDN(tt.dn)
			if len(result) != len(tt.expected) {
				t.Errorf("expected %d components, got %d", len(tt.expected), len(result))
				return
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("component %d: expected %q, got %q", i, tt.expected[i], v)
				}
			}
		})
	}
}

func TestMatcherGetParentDN(t *testing.T) {
	m := NewMatcher()

	tests := []struct {
		name     string
		dn       string
		expected string
	}{
		{"normal DN", "uid=alice,ou=users,dc=example,dc=com", "ou=users,dc=example,dc=com"},
		{"two components", "ou=users,dc=com", "dc=com"},
		{"single component", "dc=com", ""},
		{"empty DN", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.GetParentDN(tt.dn)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestMatcherNormalizeDN(t *testing.T) {
	m := NewMatcher()

	tests := []struct {
		name     string
		dn       string
		expected string
	}{
		{"lowercase", "UID=Alice,OU=Users,DC=Example,DC=Com", "uid=alice,ou=users,dc=example,dc=com"},
		{"trim spaces", "uid=alice , ou=users , dc=example , dc=com", "uid=alice,ou=users,dc=example,dc=com"},
		{"already normalized", "uid=alice,ou=users,dc=example,dc=com", "uid=alice,ou=users,dc=example,dc=com"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.NormalizeDN(tt.dn)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestMatcherMatchesPattern(t *testing.T) {
	m := NewMatcher()

	tests := []struct {
		name     string
		pattern  string
		dn       string
		expected bool
	}{
		{"wildcard all", "*", "uid=alice,ou=users,dc=example,dc=com", true},
		{"exact match", "uid=alice,ou=users,dc=example,dc=com", "uid=alice,ou=users,dc=example,dc=com", true},
		{"rdn wildcard", "uid=*,ou=users,dc=example,dc=com", "uid=alice,ou=users,dc=example,dc=com", true},
		{"rdn wildcard different user", "uid=*,ou=users,dc=example,dc=com", "uid=bob,ou=users,dc=example,dc=com", true},
		{"full rdn wildcard", "*,ou=users,dc=example,dc=com", "uid=alice,ou=users,dc=example,dc=com", true},
		{"no match different length", "uid=*,dc=example,dc=com", "uid=alice,ou=users,dc=example,dc=com", false},
		{"no match different value", "uid=alice,ou=users,dc=example,dc=com", "uid=bob,ou=users,dc=example,dc=com", false},
		{"case insensitive", "UID=*,OU=Users,DC=Example,DC=Com", "uid=alice,ou=users,dc=example,dc=com", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.MatchesPattern(tt.pattern, tt.dn)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}
