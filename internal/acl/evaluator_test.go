package acl

import (
	"testing"
)

func TestNewEvaluator(t *testing.T) {
	t.Run("with nil config", func(t *testing.T) {
		e := NewEvaluator(nil)
		if e == nil {
			t.Fatal("expected non-nil evaluator")
		}
		if e.config == nil {
			t.Fatal("expected non-nil config")
		}
		if e.config.DefaultPolicy != "deny" {
			t.Errorf("expected default policy 'deny', got %q", e.config.DefaultPolicy)
		}
	})

	t.Run("with custom config", func(t *testing.T) {
		config := NewConfig()
		config.SetDefaultPolicy("allow")

		e := NewEvaluator(config)
		if e.config.DefaultPolicy != "allow" {
			t.Errorf("expected default policy 'allow', got %q", e.config.DefaultPolicy)
		}
	})
}

func TestCheckAccess_DefaultPolicy(t *testing.T) {
	t.Run("default deny", func(t *testing.T) {
		config := NewConfig()
		config.SetDefaultPolicy("deny")
		e := NewEvaluator(config)

		ctx := NewAccessContext("uid=alice,dc=example,dc=com", "ou=users,dc=example,dc=com", Read)
		if e.CheckAccess(ctx) {
			t.Error("expected access denied with default deny policy")
		}
	})

	t.Run("default allow", func(t *testing.T) {
		config := NewConfig()
		config.SetDefaultPolicy("allow")
		e := NewEvaluator(config)

		ctx := NewAccessContext("uid=alice,dc=example,dc=com", "ou=users,dc=example,dc=com", Read)
		if !e.CheckAccess(ctx) {
			t.Error("expected access allowed with default allow policy")
		}
	})

	t.Run("nil context with default deny", func(t *testing.T) {
		config := NewConfig()
		config.SetDefaultPolicy("deny")
		e := NewEvaluator(config)

		if e.CheckAccess(nil) {
			t.Error("expected access denied for nil context with default deny")
		}
	})

	t.Run("nil context with default allow", func(t *testing.T) {
		config := NewConfig()
		config.SetDefaultPolicy("allow")
		e := NewEvaluator(config)

		if !e.CheckAccess(nil) {
			t.Error("expected access allowed for nil context with default allow")
		}
	})
}

func TestCheckAccess_FirstMatchWins(t *testing.T) {
	config := NewConfig()
	config.SetDefaultPolicy("deny")

	// First rule: deny all to anonymous
	denyRule := NewACL("*", "anonymous", All).WithDeny(true)
	config.AddRule(denyRule)

	// Second rule: allow read to everyone
	allowRule := NewACL("*", "*", Read)
	config.AddRule(allowRule)

	e := NewEvaluator(config)

	t.Run("anonymous denied by first rule", func(t *testing.T) {
		ctx := NewAccessContext("", "ou=users,dc=example,dc=com", Read)
		if e.CheckAccess(ctx) {
			t.Error("expected anonymous access denied by first rule")
		}
	})

	t.Run("authenticated allowed by second rule", func(t *testing.T) {
		ctx := NewAccessContext("uid=alice,dc=example,dc=com", "ou=users,dc=example,dc=com", Read)
		if !e.CheckAccess(ctx) {
			t.Error("expected authenticated access allowed by second rule")
		}
	})
}

func TestCheckAccess_SubjectMatching(t *testing.T) {
	tests := []struct {
		name     string
		subject  string
		bindDN   string
		targetDN string
		expected bool
	}{
		{
			name:     "anonymous matches empty bindDN",
			subject:  "anonymous",
			bindDN:   "",
			targetDN: "ou=users,dc=example,dc=com",
			expected: true,
		},
		{
			name:     "anonymous does not match authenticated",
			subject:  "anonymous",
			bindDN:   "uid=alice,dc=example,dc=com",
			targetDN: "ou=users,dc=example,dc=com",
			expected: false,
		},
		{
			name:     "authenticated matches non-empty bindDN",
			subject:  "authenticated",
			bindDN:   "uid=alice,dc=example,dc=com",
			targetDN: "ou=users,dc=example,dc=com",
			expected: true,
		},
		{
			name:     "authenticated does not match anonymous",
			subject:  "authenticated",
			bindDN:   "",
			targetDN: "ou=users,dc=example,dc=com",
			expected: false,
		},
		{
			name:     "self matches when bindDN equals targetDN",
			subject:  "self",
			bindDN:   "uid=alice,dc=example,dc=com",
			targetDN: "uid=alice,dc=example,dc=com",
			expected: true,
		},
		{
			name:     "self does not match different DNs",
			subject:  "self",
			bindDN:   "uid=alice,dc=example,dc=com",
			targetDN: "uid=bob,dc=example,dc=com",
			expected: false,
		},
		{
			name:     "self does not match anonymous",
			subject:  "self",
			bindDN:   "",
			targetDN: "uid=alice,dc=example,dc=com",
			expected: false,
		},
		{
			name:     "wildcard matches everyone",
			subject:  "*",
			bindDN:   "uid=alice,dc=example,dc=com",
			targetDN: "ou=users,dc=example,dc=com",
			expected: true,
		},
		{
			name:     "wildcard matches anonymous",
			subject:  "*",
			bindDN:   "",
			targetDN: "ou=users,dc=example,dc=com",
			expected: true,
		},
		{
			name:     "specific DN matches exact bindDN",
			subject:  "uid=admin,dc=example,dc=com",
			bindDN:   "uid=admin,dc=example,dc=com",
			targetDN: "ou=users,dc=example,dc=com",
			expected: true,
		},
		{
			name:     "specific DN does not match different bindDN",
			subject:  "uid=admin,dc=example,dc=com",
			bindDN:   "uid=alice,dc=example,dc=com",
			targetDN: "ou=users,dc=example,dc=com",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := NewConfig()
			config.SetDefaultPolicy("deny")

			rule := NewACL("*", tt.subject, Read)
			config.AddRule(rule)

			e := NewEvaluator(config)
			ctx := NewAccessContext(tt.bindDN, tt.targetDN, Read)

			result := e.CheckAccess(ctx)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestCheckAccess_TargetMatching(t *testing.T) {
	tests := []struct {
		name     string
		target   string
		scope    Scope
		targetDN string
		expected bool
	}{
		{
			name:     "wildcard target matches any DN",
			target:   "*",
			scope:    ScopeSubtree,
			targetDN: "uid=alice,ou=users,dc=example,dc=com",
			expected: true,
		},
		{
			name:     "base scope exact match",
			target:   "ou=users,dc=example,dc=com",
			scope:    ScopeBase,
			targetDN: "ou=users,dc=example,dc=com",
			expected: true,
		},
		{
			name:     "base scope no match for child",
			target:   "ou=users,dc=example,dc=com",
			scope:    ScopeBase,
			targetDN: "uid=alice,ou=users,dc=example,dc=com",
			expected: false,
		},
		{
			name:     "one scope matches immediate child",
			target:   "ou=users,dc=example,dc=com",
			scope:    ScopeOne,
			targetDN: "uid=alice,ou=users,dc=example,dc=com",
			expected: true,
		},
		{
			name:     "one scope does not match grandchild",
			target:   "dc=example,dc=com",
			scope:    ScopeOne,
			targetDN: "uid=alice,ou=users,dc=example,dc=com",
			expected: false,
		},
		{
			name:     "subtree scope matches exact",
			target:   "ou=users,dc=example,dc=com",
			scope:    ScopeSubtree,
			targetDN: "ou=users,dc=example,dc=com",
			expected: true,
		},
		{
			name:     "subtree scope matches child",
			target:   "ou=users,dc=example,dc=com",
			scope:    ScopeSubtree,
			targetDN: "uid=alice,ou=users,dc=example,dc=com",
			expected: true,
		},
		{
			name:     "subtree scope matches grandchild",
			target:   "dc=example,dc=com",
			scope:    ScopeSubtree,
			targetDN: "uid=alice,ou=users,dc=example,dc=com",
			expected: true,
		},
		{
			name:     "subtree scope does not match sibling",
			target:   "ou=groups,dc=example,dc=com",
			scope:    ScopeSubtree,
			targetDN: "uid=alice,ou=users,dc=example,dc=com",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := NewConfig()
			config.SetDefaultPolicy("deny")

			rule := NewACL(tt.target, "*", Read).WithScope(tt.scope)
			config.AddRule(rule)

			e := NewEvaluator(config)
			ctx := NewAccessContext("uid=admin,dc=example,dc=com", tt.targetDN, Read)

			result := e.CheckAccess(ctx)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestCheckAccess_RightsMatching(t *testing.T) {
	config := NewConfig()
	config.SetDefaultPolicy("deny")

	// Allow read and search only
	rule := NewACL("*", "*", Read|Search)
	config.AddRule(rule)

	e := NewEvaluator(config)
	bindDN := "uid=alice,dc=example,dc=com"
	targetDN := "ou=users,dc=example,dc=com"

	tests := []struct {
		operation Right
		expected  bool
	}{
		{Read, true},
		{Search, true},
		{Write, false},
		{Add, false},
		{Delete, false},
		{Compare, false},
	}

	for _, tt := range tests {
		t.Run(tt.operation.String(), func(t *testing.T) {
			ctx := NewAccessContext(bindDN, targetDN, tt.operation)
			result := e.CheckAccess(ctx)
			if result != tt.expected {
				t.Errorf("expected %v for %s, got %v", tt.expected, tt.operation, result)
			}
		})
	}
}

func TestCheckAccess_DenyRules(t *testing.T) {
	config := NewConfig()
	config.SetDefaultPolicy("allow")

	// Deny write to users subtree
	denyRule := NewACL("ou=users,dc=example,dc=com", "*", Write).WithDeny(true)
	config.AddRule(denyRule)

	e := NewEvaluator(config)
	bindDN := "uid=admin,dc=example,dc=com"

	t.Run("write denied in users subtree", func(t *testing.T) {
		ctx := NewAccessContext(bindDN, "uid=alice,ou=users,dc=example,dc=com", Write)
		if e.CheckAccess(ctx) {
			t.Error("expected write denied in users subtree")
		}
	})

	t.Run("read allowed in users subtree", func(t *testing.T) {
		ctx := NewAccessContext(bindDN, "uid=alice,ou=users,dc=example,dc=com", Read)
		if !e.CheckAccess(ctx) {
			t.Error("expected read allowed in users subtree")
		}
	})

	t.Run("write allowed outside users subtree", func(t *testing.T) {
		ctx := NewAccessContext(bindDN, "cn=admins,ou=groups,dc=example,dc=com", Write)
		if !e.CheckAccess(ctx) {
			t.Error("expected write allowed outside users subtree")
		}
	})
}

func TestCheckAttributeAccess(t *testing.T) {
	config := NewConfig()
	config.SetDefaultPolicy("deny")

	// Allow read of specific attributes only
	rule := NewACL("*", "*", Read).WithAttributes("cn", "mail", "uid")
	config.AddRule(rule)

	e := NewEvaluator(config)
	ctx := NewAccessContext("uid=alice,dc=example,dc=com", "uid=bob,dc=example,dc=com", Read)

	tests := []struct {
		attr     string
		expected bool
	}{
		{"cn", true},
		{"mail", true},
		{"uid", true},
		{"userPassword", false},
		{"telephoneNumber", false},
	}

	for _, tt := range tests {
		t.Run(tt.attr, func(t *testing.T) {
			result := e.CheckAttributeAccess(ctx, tt.attr)
			if result != tt.expected {
				t.Errorf("expected %v for attribute %s, got %v", tt.expected, tt.attr, result)
			}
		})
	}
}

func TestFilterAttributes(t *testing.T) {
	config := NewConfig()
	config.SetDefaultPolicy("deny")

	// Allow read of specific attributes only
	rule := NewACL("*", "*", Read).WithAttributes("cn", "mail")
	config.AddRule(rule)

	e := NewEvaluator(config)

	entry := NewEntry("uid=alice,dc=example,dc=com")
	entry.SetAttribute("cn", "Alice Smith")
	entry.SetAttribute("mail", "alice@example.com")
	entry.SetAttribute("userPassword", "secret")
	entry.SetAttribute("telephoneNumber", "555-1234")

	ctx := NewAccessContext("uid=bob,dc=example,dc=com", entry.DN, Read)
	filtered := e.FilterAttributes(ctx, entry)

	t.Run("allowed attributes present", func(t *testing.T) {
		if !filtered.HasAttribute("cn") {
			t.Error("expected cn attribute to be present")
		}
		if !filtered.HasAttribute("mail") {
			t.Error("expected mail attribute to be present")
		}
	})

	t.Run("denied attributes removed", func(t *testing.T) {
		if filtered.HasAttribute("userPassword") {
			t.Error("expected userPassword attribute to be removed")
		}
		if filtered.HasAttribute("telephoneNumber") {
			t.Error("expected telephoneNumber attribute to be removed")
		}
	})

	t.Run("original entry unchanged", func(t *testing.T) {
		if !entry.HasAttribute("userPassword") {
			t.Error("original entry should still have userPassword")
		}
	})

	t.Run("nil entry returns nil", func(t *testing.T) {
		result := e.FilterAttributes(ctx, nil)
		if result != nil {
			t.Error("expected nil result for nil entry")
		}
	})
}

func TestFilterAttributeList(t *testing.T) {
	config := NewConfig()
	config.SetDefaultPolicy("deny")

	rule := NewACL("*", "*", Read).WithAttributes("cn", "mail", "uid")
	config.AddRule(rule)

	e := NewEvaluator(config)
	ctx := NewAccessContext("uid=alice,dc=example,dc=com", "uid=bob,dc=example,dc=com", Read)

	attrs := []string{"cn", "mail", "userPassword", "telephoneNumber"}
	filtered := e.FilterAttributeList(ctx, attrs)

	if len(filtered) != 2 {
		t.Errorf("expected 2 filtered attributes, got %d", len(filtered))
	}

	// Check that only allowed attributes are present
	allowed := map[string]bool{"cn": true, "mail": true}
	for _, attr := range filtered {
		if !allowed[attr] {
			t.Errorf("unexpected attribute %s in filtered list", attr)
		}
	}
}

func TestConvenienceMethods(t *testing.T) {
	config := NewConfig()
	config.SetDefaultPolicy("deny")

	// Admin can do everything
	adminRule := NewACL("*", "uid=admin,dc=example,dc=com", All)
	config.AddRule(adminRule)

	// Users can read
	userRule := NewACL("*", "authenticated", Read|Search)
	config.AddRule(userRule)

	e := NewEvaluator(config)

	adminDN := "uid=admin,dc=example,dc=com"
	userDN := "uid=alice,dc=example,dc=com"
	targetDN := "ou=users,dc=example,dc=com"

	t.Run("admin can read", func(t *testing.T) {
		if !e.CanRead(adminDN, targetDN) {
			t.Error("expected admin can read")
		}
	})

	t.Run("admin can write", func(t *testing.T) {
		if !e.CanWrite(adminDN, targetDN) {
			t.Error("expected admin can write")
		}
	})

	t.Run("admin can add", func(t *testing.T) {
		if !e.CanAdd(adminDN, targetDN) {
			t.Error("expected admin can add")
		}
	})

	t.Run("admin can delete", func(t *testing.T) {
		if !e.CanDelete(adminDN, targetDN) {
			t.Error("expected admin can delete")
		}
	})

	t.Run("user can read", func(t *testing.T) {
		if !e.CanRead(userDN, targetDN) {
			t.Error("expected user can read")
		}
	})

	t.Run("user can search", func(t *testing.T) {
		if !e.CanSearch(userDN, targetDN) {
			t.Error("expected user can search")
		}
	})

	t.Run("user cannot write", func(t *testing.T) {
		if e.CanWrite(userDN, targetDN) {
			t.Error("expected user cannot write")
		}
	})

	t.Run("user cannot delete", func(t *testing.T) {
		if e.CanDelete(userDN, targetDN) {
			t.Error("expected user cannot delete")
		}
	})
}

func TestSelfAccess(t *testing.T) {
	config := NewConfig()
	config.SetDefaultPolicy("deny")

	// Users can modify their own entry
	selfRule := NewACL("*", "self", Read|Write)
	config.AddRule(selfRule)

	e := NewEvaluator(config)
	aliceDN := "uid=alice,ou=users,dc=example,dc=com"
	bobDN := "uid=bob,ou=users,dc=example,dc=com"

	t.Run("alice can read own entry", func(t *testing.T) {
		if !e.CanRead(aliceDN, aliceDN) {
			t.Error("expected alice can read own entry")
		}
	})

	t.Run("alice can write own entry", func(t *testing.T) {
		if !e.CanWrite(aliceDN, aliceDN) {
			t.Error("expected alice can write own entry")
		}
	})

	t.Run("alice cannot read bob's entry", func(t *testing.T) {
		if e.CanRead(aliceDN, bobDN) {
			t.Error("expected alice cannot read bob's entry")
		}
	})

	t.Run("alice cannot write bob's entry", func(t *testing.T) {
		if e.CanWrite(aliceDN, bobDN) {
			t.Error("expected alice cannot write bob's entry")
		}
	})
}

func TestComplexACLScenario(t *testing.T) {
	config := NewConfig()
	config.SetDefaultPolicy("deny")

	// Rule 1: Deny anonymous access to sensitive subtree
	config.AddRule(NewACL("ou=sensitive,dc=example,dc=com", "anonymous", All).WithDeny(true))

	// Rule 2: Admin has full access everywhere
	config.AddRule(NewACL("*", "uid=admin,dc=example,dc=com", All))

	// Rule 3: Users can read and search in users subtree
	config.AddRule(NewACL("ou=users,dc=example,dc=com", "authenticated", Read|Search))

	// Rule 4: Users can modify their own entry
	config.AddRule(NewACL("ou=users,dc=example,dc=com", "self", Write))

	// Rule 5: Everyone can read public info
	config.AddRule(NewACL("ou=public,dc=example,dc=com", "*", Read))

	e := NewEvaluator(config)

	adminDN := "uid=admin,dc=example,dc=com"
	aliceDN := "uid=alice,ou=users,dc=example,dc=com"

	tests := []struct {
		name      string
		bindDN    string
		targetDN  string
		operation Right
		expected  bool
	}{
		// Anonymous tests
		{"anonymous denied sensitive", "", "cn=secret,ou=sensitive,dc=example,dc=com", Read, false},
		{"anonymous can read public", "", "cn=info,ou=public,dc=example,dc=com", Read, true},
		{"anonymous cannot read users", "", "uid=alice,ou=users,dc=example,dc=com", Read, false},

		// Admin tests
		{"admin can read sensitive", adminDN, "cn=secret,ou=sensitive,dc=example,dc=com", Read, true},
		{"admin can write sensitive", adminDN, "cn=secret,ou=sensitive,dc=example,dc=com", Write, true},
		{"admin can delete users", adminDN, "uid=alice,ou=users,dc=example,dc=com", Delete, true},

		// User tests
		{"alice can read users", aliceDN, "uid=bob,ou=users,dc=example,dc=com", Read, true},
		{"alice can search users", aliceDN, "ou=users,dc=example,dc=com", Search, true},
		{"alice cannot write bob", aliceDN, "uid=bob,ou=users,dc=example,dc=com", Write, false},
		{"alice can write self", aliceDN, aliceDN, Write, true},
		{"alice cannot delete self", aliceDN, aliceDN, Delete, false},
		{"alice can read public", aliceDN, "cn=info,ou=public,dc=example,dc=com", Read, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewAccessContext(tt.bindDN, tt.targetDN, tt.operation)
			result := e.CheckAccess(ctx)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestCaseInsensitiveMatching(t *testing.T) {
	config := NewConfig()
	config.SetDefaultPolicy("deny")

	rule := NewACL("OU=Users,DC=Example,DC=Com", "UID=Admin,DC=Example,DC=Com", Read)
	config.AddRule(rule)

	e := NewEvaluator(config)

	tests := []struct {
		name     string
		bindDN   string
		targetDN string
		expected bool
	}{
		{"lowercase match", "uid=admin,dc=example,dc=com", "ou=users,dc=example,dc=com", true},
		{"uppercase match", "UID=ADMIN,DC=EXAMPLE,DC=COM", "OU=USERS,DC=EXAMPLE,DC=COM", true},
		{"mixed case match", "Uid=Admin,Dc=Example,Dc=Com", "Ou=Users,Dc=Example,Dc=Com", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewAccessContext(tt.bindDN, tt.targetDN, Read)
			result := e.CheckAccess(ctx)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGetSetConfig(t *testing.T) {
	e := NewEvaluator(nil)

	t.Run("get config", func(t *testing.T) {
		config := e.GetConfig()
		if config == nil {
			t.Error("expected non-nil config")
		}
	})

	t.Run("set config", func(t *testing.T) {
		newConfig := NewConfig()
		newConfig.SetDefaultPolicy("allow")
		e.SetConfig(newConfig)

		if e.GetConfig().DefaultPolicy != "allow" {
			t.Error("expected config to be updated")
		}
	})

	t.Run("set nil config", func(t *testing.T) {
		e.SetConfig(nil)
		if e.GetConfig() == nil {
			t.Error("expected non-nil config after setting nil")
		}
	})
}
