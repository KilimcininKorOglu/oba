package acl

import (
	"testing"
)

func TestRightString(t *testing.T) {
	tests := []struct {
		right    Right
		expected string
	}{
		{Read, "read"},
		{Write, "write"},
		{Add, "add"},
		{Delete, "delete"},
		{Search, "search"},
		{Compare, "compare"},
		{All, "all"},
		{Right(0), "unknown"},
		{Right(128), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.right.String()
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestRightHas(t *testing.T) {
	tests := []struct {
		name     string
		right    Right
		check    Right
		expected bool
	}{
		{"read has read", Read, Read, true},
		{"all has read", All, Read, true},
		{"all has write", All, Write, true},
		{"read|write has read", Read | Write, Read, true},
		{"read|write has write", Read | Write, Write, true},
		{"read does not have write", Read, Write, false},
		{"read does not have delete", Read, Delete, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.right.Has(tt.check)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestScopeString(t *testing.T) {
	tests := []struct {
		scope    Scope
		expected string
	}{
		{ScopeBase, "base"},
		{ScopeOne, "one"},
		{ScopeSubtree, "subtree"},
		{Scope(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.scope.String()
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestNewACL(t *testing.T) {
	acl := NewACL("ou=users,dc=example,dc=com", "uid=admin,dc=example,dc=com", Read|Write)

	if acl.Target != "ou=users,dc=example,dc=com" {
		t.Errorf("expected target 'ou=users,dc=example,dc=com', got %q", acl.Target)
	}
	if acl.Subject != "uid=admin,dc=example,dc=com" {
		t.Errorf("expected subject 'uid=admin,dc=example,dc=com', got %q", acl.Subject)
	}
	if acl.Rights != Read|Write {
		t.Errorf("expected rights Read|Write, got %v", acl.Rights)
	}
	if acl.Scope != ScopeSubtree {
		t.Errorf("expected default scope ScopeSubtree, got %v", acl.Scope)
	}
	if acl.Deny {
		t.Error("expected Deny to be false by default")
	}
	if acl.Attributes != nil {
		t.Error("expected Attributes to be nil by default")
	}
}

func TestACLChaining(t *testing.T) {
	acl := NewACL("*", "*", Read).
		WithScope(ScopeOne).
		WithAttributes("cn", "mail").
		WithDeny(true)

	if acl.Scope != ScopeOne {
		t.Errorf("expected scope ScopeOne, got %v", acl.Scope)
	}
	if len(acl.Attributes) != 2 {
		t.Errorf("expected 2 attributes, got %d", len(acl.Attributes))
	}
	if !acl.Deny {
		t.Error("expected Deny to be true")
	}
}

func TestACLAppliesToAttribute(t *testing.T) {
	tests := []struct {
		name       string
		attributes []string
		attr       string
		expected   bool
	}{
		{"empty applies to all", nil, "cn", true},
		{"empty applies to any", nil, "userPassword", true},
		{"specific matches", []string{"cn", "mail"}, "cn", true},
		{"specific matches second", []string{"cn", "mail"}, "mail", true},
		{"specific no match", []string{"cn", "mail"}, "userPassword", false},
		{"wildcard matches all", []string{"*"}, "anything", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			acl := &ACL{Attributes: tt.attributes}
			result := acl.AppliesToAttribute(tt.attr)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestNewConfig(t *testing.T) {
	config := NewConfig()

	if config.DefaultPolicy != "deny" {
		t.Errorf("expected default policy 'deny', got %q", config.DefaultPolicy)
	}
	if config.Rules == nil {
		t.Error("expected Rules to be initialized")
	}
	if len(config.Rules) != 0 {
		t.Errorf("expected 0 rules, got %d", len(config.Rules))
	}
}

func TestConfigAddRule(t *testing.T) {
	config := NewConfig()
	rule1 := NewACL("*", "*", Read)
	rule2 := NewACL("*", "authenticated", Write)

	config.AddRule(rule1)
	config.AddRule(rule2)

	if len(config.Rules) != 2 {
		t.Errorf("expected 2 rules, got %d", len(config.Rules))
	}
	if config.Rules[0] != rule1 {
		t.Error("first rule mismatch")
	}
	if config.Rules[1] != rule2 {
		t.Error("second rule mismatch")
	}
}

func TestConfigSetDefaultPolicy(t *testing.T) {
	config := NewConfig()

	config.SetDefaultPolicy("allow")
	if config.DefaultPolicy != "allow" {
		t.Errorf("expected 'allow', got %q", config.DefaultPolicy)
	}

	config.SetDefaultPolicy("deny")
	if config.DefaultPolicy != "deny" {
		t.Errorf("expected 'deny', got %q", config.DefaultPolicy)
	}
}

func TestConfigIsDefaultAllow(t *testing.T) {
	config := NewConfig()

	if config.IsDefaultAllow() {
		t.Error("expected IsDefaultAllow to be false for 'deny'")
	}

	config.SetDefaultPolicy("allow")
	if !config.IsDefaultAllow() {
		t.Error("expected IsDefaultAllow to be true for 'allow'")
	}
}

func TestNewAccessContext(t *testing.T) {
	ctx := NewAccessContext("uid=alice,dc=example,dc=com", "ou=users,dc=example,dc=com", Read)

	if ctx.BindDN != "uid=alice,dc=example,dc=com" {
		t.Errorf("expected bindDN 'uid=alice,dc=example,dc=com', got %q", ctx.BindDN)
	}
	if ctx.TargetDN != "ou=users,dc=example,dc=com" {
		t.Errorf("expected targetDN 'ou=users,dc=example,dc=com', got %q", ctx.TargetDN)
	}
	if ctx.Operation != Read {
		t.Errorf("expected operation Read, got %v", ctx.Operation)
	}
	if ctx.Attributes != nil {
		t.Error("expected Attributes to be nil by default")
	}
}

func TestAccessContextWithAttributes(t *testing.T) {
	ctx := NewAccessContext("", "", Read).WithAttributes("cn", "mail", "uid")

	if len(ctx.Attributes) != 3 {
		t.Errorf("expected 3 attributes, got %d", len(ctx.Attributes))
	}
}

func TestAccessContextIsAnonymous(t *testing.T) {
	t.Run("anonymous", func(t *testing.T) {
		ctx := NewAccessContext("", "ou=users,dc=example,dc=com", Read)
		if !ctx.IsAnonymous() {
			t.Error("expected IsAnonymous to be true for empty bindDN")
		}
	})

	t.Run("authenticated", func(t *testing.T) {
		ctx := NewAccessContext("uid=alice,dc=example,dc=com", "ou=users,dc=example,dc=com", Read)
		if ctx.IsAnonymous() {
			t.Error("expected IsAnonymous to be false for non-empty bindDN")
		}
	})
}

func TestAccessContextIsSelf(t *testing.T) {
	t.Run("self", func(t *testing.T) {
		ctx := NewAccessContext("uid=alice,dc=example,dc=com", "uid=alice,dc=example,dc=com", Read)
		if !ctx.IsSelf() {
			t.Error("expected IsSelf to be true when bindDN equals targetDN")
		}
	})

	t.Run("not self", func(t *testing.T) {
		ctx := NewAccessContext("uid=alice,dc=example,dc=com", "uid=bob,dc=example,dc=com", Read)
		if ctx.IsSelf() {
			t.Error("expected IsSelf to be false when bindDN differs from targetDN")
		}
	})

	t.Run("anonymous not self", func(t *testing.T) {
		ctx := NewAccessContext("", "uid=alice,dc=example,dc=com", Read)
		if ctx.IsSelf() {
			t.Error("expected IsSelf to be false for anonymous")
		}
	})
}

func TestEntryOperations(t *testing.T) {
	entry := NewEntry("uid=alice,dc=example,dc=com")

	t.Run("set and get attribute", func(t *testing.T) {
		entry.SetAttribute("cn", "Alice Smith")
		values := entry.GetAttribute("cn")
		if len(values) != 1 || values[0] != "Alice Smith" {
			t.Errorf("expected ['Alice Smith'], got %v", values)
		}
	})

	t.Run("set multiple values", func(t *testing.T) {
		entry.SetAttribute("mail", "alice@example.com", "alice.smith@example.com")
		values := entry.GetAttribute("mail")
		if len(values) != 2 {
			t.Errorf("expected 2 values, got %d", len(values))
		}
	})

	t.Run("has attribute", func(t *testing.T) {
		if !entry.HasAttribute("cn") {
			t.Error("expected HasAttribute to return true for 'cn'")
		}
		if entry.HasAttribute("nonexistent") {
			t.Error("expected HasAttribute to return false for 'nonexistent'")
		}
	})

	t.Run("get nonexistent attribute", func(t *testing.T) {
		values := entry.GetAttribute("nonexistent")
		if values != nil {
			t.Errorf("expected nil for nonexistent attribute, got %v", values)
		}
	})
}

func TestEntryClone(t *testing.T) {
	original := NewEntry("uid=alice,dc=example,dc=com")
	original.SetAttribute("cn", "Alice Smith")
	original.SetAttribute("mail", "alice@example.com")

	clone := original.Clone()

	t.Run("clone has same DN", func(t *testing.T) {
		if clone.DN != original.DN {
			t.Errorf("expected DN %q, got %q", original.DN, clone.DN)
		}
	})

	t.Run("clone has same attributes", func(t *testing.T) {
		if !clone.HasAttribute("cn") || !clone.HasAttribute("mail") {
			t.Error("clone should have same attributes")
		}
	})

	t.Run("clone is independent", func(t *testing.T) {
		clone.SetAttribute("cn", "Modified")
		if original.GetAttribute("cn")[0] == "Modified" {
			t.Error("modifying clone should not affect original")
		}
	})

	t.Run("nil clone returns nil", func(t *testing.T) {
		var nilEntry *Entry
		if nilEntry.Clone() != nil {
			t.Error("cloning nil should return nil")
		}
	})
}

func TestRightConstants(t *testing.T) {
	// Verify rights are distinct bit flags
	rights := []Right{Read, Write, Add, Delete, Search, Compare}
	for i, r1 := range rights {
		for j, r2 := range rights {
			if i != j && r1&r2 != 0 {
				t.Errorf("rights %v and %v should not overlap", r1, r2)
			}
		}
	}

	// Verify All contains all rights
	for _, r := range rights {
		if All&r == 0 {
			t.Errorf("All should contain %v", r)
		}
	}
}
