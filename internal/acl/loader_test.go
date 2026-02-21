package acl

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFromFile(t *testing.T) {
	t.Run("valid file", func(t *testing.T) {
		content := `
version: 1
defaultPolicy: "deny"
rules:
  - target: "*"
    subject: "cn=admin,dc=example,dc=com"
    rights: ["read", "write", "add", "delete"]
`
		tmpFile := createTempFile(t, content)
		defer os.Remove(tmpFile)

		config, err := LoadFromFile(tmpFile)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if config.DefaultPolicy != "deny" {
			t.Errorf("expected defaultPolicy 'deny', got %q", config.DefaultPolicy)
		}
		if len(config.Rules) != 1 {
			t.Errorf("expected 1 rule, got %d", len(config.Rules))
		}
	})

	t.Run("file not found", func(t *testing.T) {
		_, err := LoadFromFile("/nonexistent/path/acl.yaml")
		if err != ErrFileNotFound {
			t.Errorf("expected ErrFileNotFound, got %v", err)
		}
	})
}

func TestParseACLYAML(t *testing.T) {
	t.Run("minimal config", func(t *testing.T) {
		yaml := `
defaultPolicy: allow
rules:
  - target: "*"
    subject: "*"
    rights: [read]
`
		config, err := ParseACLYAML([]byte(yaml))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if config.DefaultPolicy != "allow" {
			t.Errorf("expected defaultPolicy 'allow', got %q", config.DefaultPolicy)
		}
		if len(config.Rules) != 1 {
			t.Errorf("expected 1 rule, got %d", len(config.Rules))
		}
	})

	t.Run("full config", func(t *testing.T) {
		yaml := `
version: 1
defaultPolicy: "deny"
rules:
  - target: "*"
    subject: "cn=admin,dc=example,dc=com"
    rights: ["read", "write", "add", "delete", "search", "compare"]
  - target: "ou=users,dc=example,dc=com"
    subject: "authenticated"
    scope: "subtree"
    rights:
      - read
      - search
  - target: "ou=users,dc=example,dc=com"
    subject: "self"
    rights: [read, write]
    attributes: [userPassword, mail, telephoneNumber]
  - target: "ou=sensitive,dc=example,dc=com"
    subject: "anonymous"
    rights: [read]
    deny: true
`
		config, err := ParseACLYAML([]byte(yaml))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(config.Rules) != 4 {
			t.Fatalf("expected 4 rules, got %d", len(config.Rules))
		}

		// Check first rule (admin)
		rule0 := config.Rules[0]
		if rule0.Target != "*" {
			t.Errorf("rule 0: expected target '*', got %q", rule0.Target)
		}
		if rule0.Subject != "cn=admin,dc=example,dc=com" {
			t.Errorf("rule 0: expected subject 'cn=admin,dc=example,dc=com', got %q", rule0.Subject)
		}
		if rule0.Rights != (Read | Write | Add | Delete | Search | Compare) {
			t.Errorf("rule 0: unexpected rights %v", rule0.Rights)
		}

		// Check second rule (authenticated)
		rule1 := config.Rules[1]
		if rule1.Scope != ScopeSubtree {
			t.Errorf("rule 1: expected scope ScopeSubtree, got %v", rule1.Scope)
		}
		if rule1.Rights != (Read | Search) {
			t.Errorf("rule 1: expected rights Read|Search, got %v", rule1.Rights)
		}

		// Check third rule (self with attributes)
		rule2 := config.Rules[2]
		if len(rule2.Attributes) != 3 {
			t.Errorf("rule 2: expected 3 attributes, got %d", len(rule2.Attributes))
		}

		// Check fourth rule (deny)
		rule3 := config.Rules[3]
		if !rule3.Deny {
			t.Error("rule 3: expected Deny to be true")
		}
	})

	t.Run("inline arrays", func(t *testing.T) {
		yaml := `
defaultPolicy: deny
rules:
  - target: "*"
    subject: "*"
    rights: [read, write, search]
    attributes: [cn, mail, uid]
`
		config, err := ParseACLYAML([]byte(yaml))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		rule := config.Rules[0]
		if rule.Rights != (Read | Write | Search) {
			t.Errorf("expected rights Read|Write|Search, got %v", rule.Rights)
		}
		if len(rule.Attributes) != 3 {
			t.Errorf("expected 3 attributes, got %d", len(rule.Attributes))
		}
	})

	t.Run("comments ignored", func(t *testing.T) {
		yaml := `
# This is a comment
version: 1
# Another comment
defaultPolicy: deny
rules:
  # Rule comment
  - target: "*"
    subject: "*"
    rights: [read]
`
		config, err := ParseACLYAML([]byte(yaml))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(config.Rules) != 1 {
			t.Errorf("expected 1 rule, got %d", len(config.Rules))
		}
	})
}

func TestParseRights(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected Right
		wantErr  bool
	}{
		{"single read", []string{"read"}, Read, false},
		{"single write", []string{"write"}, Write, false},
		{"multiple", []string{"read", "write", "search"}, Read | Write | Search, false},
		{"all", []string{"all"}, All, false},
		{"case insensitive", []string{"READ", "Write", "SEARCH"}, Read | Write | Search, false},
		{"with spaces", []string{" read ", " write "}, Read | Write, false},
		{"invalid", []string{"invalid"}, 0, true},
		{"mixed valid invalid", []string{"read", "invalid"}, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseRights(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestParseScope(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected Scope
		wantErr  bool
	}{
		{"base", "base", ScopeBase, false},
		{"one", "one", ScopeOne, false},
		{"onelevel", "onelevel", ScopeOne, false},
		{"sub", "sub", ScopeSubtree, false},
		{"subtree", "subtree", ScopeSubtree, false},
		{"empty", "", ScopeSubtree, false},
		{"case insensitive", "BASE", ScopeBase, false},
		{"with spaces", " sub ", ScopeSubtree, false},
		{"invalid", "invalid", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseScope(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestConvertFileConfig(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		fc := &FileConfig{
			Version:       1,
			DefaultPolicy: "deny",
			Rules: []FileRuleConfig{
				{
					Target:  "*",
					Subject: "cn=admin,dc=example,dc=com",
					Rights:  []string{"read", "write"},
				},
			},
		}

		config, err := convertFileConfig(fc)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if config.DefaultPolicy != "deny" {
			t.Errorf("expected defaultPolicy 'deny', got %q", config.DefaultPolicy)
		}
		if len(config.Rules) != 1 {
			t.Errorf("expected 1 rule, got %d", len(config.Rules))
		}
	})

	t.Run("invalid version", func(t *testing.T) {
		fc := &FileConfig{
			Version:       0,
			DefaultPolicy: "deny",
			Rules:         []FileRuleConfig{},
		}

		_, err := convertFileConfig(fc)
		if err == nil {
			t.Error("expected error for invalid version")
		}
	})

	t.Run("invalid policy", func(t *testing.T) {
		fc := &FileConfig{
			Version:       1,
			DefaultPolicy: "invalid",
			Rules:         []FileRuleConfig{},
		}

		_, err := convertFileConfig(fc)
		if err == nil {
			t.Error("expected error for invalid policy")
		}
	})

	t.Run("missing target", func(t *testing.T) {
		fc := &FileConfig{
			Version:       1,
			DefaultPolicy: "deny",
			Rules: []FileRuleConfig{
				{Subject: "*", Rights: []string{"read"}},
			},
		}

		_, err := convertFileConfig(fc)
		if err == nil {
			t.Error("expected error for missing target")
		}
	})

	t.Run("missing subject", func(t *testing.T) {
		fc := &FileConfig{
			Version:       1,
			DefaultPolicy: "deny",
			Rules: []FileRuleConfig{
				{Target: "*", Rights: []string{"read"}},
			},
		}

		_, err := convertFileConfig(fc)
		if err == nil {
			t.Error("expected error for missing subject")
		}
	})

	t.Run("missing rights", func(t *testing.T) {
		fc := &FileConfig{
			Version:       1,
			DefaultPolicy: "deny",
			Rules: []FileRuleConfig{
				{Target: "*", Subject: "*"},
			},
		}

		_, err := convertFileConfig(fc)
		if err == nil {
			t.Error("expected error for missing rights")
		}
	})
}

func TestEnvironmentVariableSubstitution(t *testing.T) {
	os.Setenv("TEST_ACL_SUBJECT", "cn=testuser,dc=example,dc=com")
	defer os.Unsetenv("TEST_ACL_SUBJECT")

	yaml := `
defaultPolicy: deny
rules:
  - target: "*"
    subject: "${TEST_ACL_SUBJECT}"
    rights: [read]
`
	config, err := ParseACLYAML([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if config.Rules[0].Subject != "cn=testuser,dc=example,dc=com" {
		t.Errorf("expected subject from env var, got %q", config.Rules[0].Subject)
	}
}

func TestEnvironmentVariableWithDefault(t *testing.T) {
	os.Unsetenv("NONEXISTENT_VAR")

	yaml := `
defaultPolicy: ${NONEXISTENT_VAR:-deny}
rules:
  - target: "*"
    subject: "*"
    rights: [read]
`
	config, err := ParseACLYAML([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if config.DefaultPolicy != "deny" {
		t.Errorf("expected default value 'deny', got %q", config.DefaultPolicy)
	}
}

func createTempFile(t *testing.T, content string) string {
	t.Helper()
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "acl.yaml")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	return tmpFile
}
