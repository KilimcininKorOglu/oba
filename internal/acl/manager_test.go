package acl

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	t.Run("with file", func(t *testing.T) {
		content := `
version: 1
defaultPolicy: deny
rules:
  - target: "*"
    subject: "cn=admin,dc=example,dc=com"
    rights: [read, write]
`
		tmpFile := createTempACLFile(t, content)
		defer os.Remove(tmpFile)

		m, err := NewManager(&ManagerConfig{
			FilePath: tmpFile,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !m.IsFileMode() {
			t.Error("expected file mode")
		}
		if m.FilePath() != tmpFile {
			t.Errorf("expected file path %q, got %q", tmpFile, m.FilePath())
		}

		config := m.GetConfig()
		if len(config.Rules) != 1 {
			t.Errorf("expected 1 rule, got %d", len(config.Rules))
		}
	})

	t.Run("with embedded config", func(t *testing.T) {
		embeddedConfig := NewConfig()
		embeddedConfig.SetDefaultPolicy("allow")
		embeddedConfig.AddRule(NewACL("*", "*", Read))

		m, err := NewManager(&ManagerConfig{
			EmbeddedConfig: embeddedConfig,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if m.IsFileMode() {
			t.Error("expected embedded mode, not file mode")
		}

		config := m.GetConfig()
		if config.DefaultPolicy != "allow" {
			t.Errorf("expected defaultPolicy 'allow', got %q", config.DefaultPolicy)
		}
	})

	t.Run("with default config", func(t *testing.T) {
		m, err := NewManager(&ManagerConfig{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		config := m.GetConfig()
		if config.DefaultPolicy != "deny" {
			t.Errorf("expected default policy 'deny', got %q", config.DefaultPolicy)
		}
	})

	t.Run("file not found", func(t *testing.T) {
		_, err := NewManager(&ManagerConfig{
			FilePath: "/nonexistent/path/acl.yaml",
		})
		if err == nil {
			t.Error("expected error for nonexistent file")
		}
	})

	t.Run("nil config", func(t *testing.T) {
		m, err := NewManager(nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if m.GetConfig() == nil {
			t.Error("expected non-nil config")
		}
	})
}

func TestManagerReload(t *testing.T) {
	t.Run("successful reload", func(t *testing.T) {
		// Initial config
		content1 := `
version: 1
defaultPolicy: deny
rules:
  - target: "*"
    subject: "cn=admin,dc=example,dc=com"
    rights: [read]
`
		tmpFile := createTempACLFile(t, content1)
		defer os.Remove(tmpFile)

		m, err := NewManager(&ManagerConfig{FilePath: tmpFile})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify initial state
		if len(m.GetConfig().Rules) != 1 {
			t.Errorf("expected 1 rule initially, got %d", len(m.GetConfig().Rules))
		}

		// Update file
		content2 := `
version: 1
defaultPolicy: allow
rules:
  - target: "*"
    subject: "*"
    rights: [read, write]
  - target: "ou=users,dc=example,dc=com"
    subject: "authenticated"
    rights: [search]
`
		if err := os.WriteFile(tmpFile, []byte(content2), 0644); err != nil {
			t.Fatalf("failed to update file: %v", err)
		}

		// Reload
		if err := m.Reload(); err != nil {
			t.Fatalf("reload failed: %v", err)
		}

		// Verify new state
		config := m.GetConfig()
		if config.DefaultPolicy != "allow" {
			t.Errorf("expected defaultPolicy 'allow', got %q", config.DefaultPolicy)
		}
		if len(config.Rules) != 2 {
			t.Errorf("expected 2 rules after reload, got %d", len(config.Rules))
		}

		// Verify stats
		stats := m.Stats()
		if stats.ReloadCount != 1 {
			t.Errorf("expected reload count 1, got %d", stats.ReloadCount)
		}
	})

	t.Run("reload with missing required fields preserves old config", func(t *testing.T) {
		content := `
version: 1
defaultPolicy: deny
rules:
  - target: "*"
    subject: "cn=admin,dc=example,dc=com"
    rights: [read]
`
		tmpFile := createTempACLFile(t, content)
		defer os.Remove(tmpFile)

		m, err := NewManager(&ManagerConfig{FilePath: tmpFile})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Write config with missing required field (no rights)
		invalidContent := `
version: 1
defaultPolicy: deny
rules:
  - target: "*"
    subject: "cn=admin,dc=example,dc=com"
`
		if err := os.WriteFile(tmpFile, []byte(invalidContent), 0644); err != nil {
			t.Fatalf("failed to update file: %v", err)
		}

		// Reload should fail
		err = m.Reload()
		if err == nil {
			t.Error("expected reload to fail with missing rights")
		}

		// Old config should be preserved
		if len(m.GetConfig().Rules) != 1 {
			t.Errorf("expected old config to be preserved with 1 rule, got %d", len(m.GetConfig().Rules))
		}

		// Stats should show error
		stats := m.Stats()
		if stats.LastError == nil {
			t.Error("expected LastError to be set")
		}
	})

	t.Run("reload without file mode", func(t *testing.T) {
		m, err := NewManager(&ManagerConfig{
			EmbeddedConfig: NewConfig(),
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		err = m.Reload()
		if err == nil {
			t.Error("expected error when reloading without file mode")
		}
	})
}

func TestManagerConcurrency(t *testing.T) {
	content := `
version: 1
defaultPolicy: deny
rules:
  - target: "*"
    subject: "cn=admin,dc=example,dc=com"
    rights: [read, write, search]
`
	tmpFile := createTempACLFile(t, content)
	defer os.Remove(tmpFile)

	m, err := NewManager(&ManagerConfig{FilePath: tmpFile})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Run concurrent access
	var wg sync.WaitGroup
	done := make(chan struct{})

	// Readers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
					m.CanRead("cn=admin,dc=example,dc=com", "ou=users,dc=example,dc=com")
					m.CanWrite("cn=admin,dc=example,dc=com", "ou=users,dc=example,dc=com")
					m.GetConfig()
					m.Stats()
				}
			}
		}()
	}

	// Reloader
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 5; i++ {
			select {
			case <-done:
				return
			default:
				m.Reload()
				time.Sleep(10 * time.Millisecond)
			}
		}
	}()

	// Let it run for a bit
	time.Sleep(100 * time.Millisecond)
	close(done)
	wg.Wait()

	// If we get here without deadlock or panic, test passes
}

func TestManagerCheckAccess(t *testing.T) {
	config := NewConfig()
	config.SetDefaultPolicy("deny")
	config.AddRule(NewACL("*", "cn=admin,dc=example,dc=com", All))
	config.AddRule(NewACL("ou=users,dc=example,dc=com", "authenticated", Read|Search))

	m, err := NewManager(&ManagerConfig{EmbeddedConfig: config})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tests := []struct {
		name     string
		bindDN   string
		targetDN string
		check    func(string, string) bool
		expected bool
	}{
		{"admin can read", "cn=admin,dc=example,dc=com", "ou=users,dc=example,dc=com", m.CanRead, true},
		{"admin can write", "cn=admin,dc=example,dc=com", "ou=users,dc=example,dc=com", m.CanWrite, true},
		{"admin can delete", "cn=admin,dc=example,dc=com", "ou=users,dc=example,dc=com", m.CanDelete, true},
		{"user can read", "uid=alice,ou=users,dc=example,dc=com", "ou=users,dc=example,dc=com", m.CanRead, true},
		{"user can search", "uid=alice,ou=users,dc=example,dc=com", "ou=users,dc=example,dc=com", m.CanSearch, true},
		{"user cannot write", "uid=alice,ou=users,dc=example,dc=com", "ou=users,dc=example,dc=com", m.CanWrite, false},
		{"anonymous denied", "", "ou=users,dc=example,dc=com", m.CanRead, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.check(tt.bindDN, tt.targetDN)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestManagerStats(t *testing.T) {
	content := `
version: 1
defaultPolicy: deny
rules:
  - target: "*"
    subject: "*"
    rights: [read]
`
	tmpFile := createTempACLFile(t, content)
	defer os.Remove(tmpFile)

	m, err := NewManager(&ManagerConfig{FilePath: tmpFile})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stats := m.Stats()

	if stats.FilePath != tmpFile {
		t.Errorf("expected file path %q, got %q", tmpFile, stats.FilePath)
	}
	if stats.RuleCount != 1 {
		t.Errorf("expected 1 rule, got %d", stats.RuleCount)
	}
	if stats.DefaultPolicy != "deny" {
		t.Errorf("expected defaultPolicy 'deny', got %q", stats.DefaultPolicy)
	}
	if stats.ReloadCount != 0 {
		t.Errorf("expected reload count 0, got %d", stats.ReloadCount)
	}
	if stats.LastReload.IsZero() {
		t.Error("expected LastReload to be set")
	}
}

func createTempACLFile(t *testing.T, content string) string {
	t.Helper()
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "acl.yaml")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	return tmpFile
}
