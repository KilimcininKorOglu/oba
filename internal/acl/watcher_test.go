package acl

import (
	"os"
	"testing"
	"time"
)

func TestFileWatcher(t *testing.T) {
	t.Run("detects file change and reloads", func(t *testing.T) {
		// Create initial ACL file
		content := `
version: 1
defaultPolicy: deny
rules:
  - target: "*"
    subject: "cn=admin,dc=test,dc=com"
    rights: [read]
`
		tmpFile := createTempACLFile(t, content)
		defer os.Remove(tmpFile)

		// Create manager
		manager, err := NewManager(&ManagerConfig{FilePath: tmpFile})
		if err != nil {
			t.Fatalf("failed to create manager: %v", err)
		}

		initialRules := len(manager.GetConfig().Rules)
		if initialRules != 1 {
			t.Fatalf("expected 1 rule, got %d", initialRules)
		}

		// Create watcher with fast polling for test
		watcher, err := NewFileWatcher(&WatcherConfig{
			FilePath:     tmpFile,
			Manager:      manager,
			PollInterval: 10 * time.Millisecond,
			Debounce:     20 * time.Millisecond,
		})
		if err != nil {
			t.Fatalf("failed to create watcher: %v", err)
		}

		watcher.Start()
		defer watcher.Stop()

		// Wait for watcher to start
		time.Sleep(50 * time.Millisecond)

		// Update file with new rule
		newContent := `
version: 1
defaultPolicy: deny
rules:
  - target: "*"
    subject: "cn=admin,dc=test,dc=com"
    rights: [read]
  - target: "dc=test,dc=com"
    subject: "authenticated"
    rights: [search]
`
		if err := os.WriteFile(tmpFile, []byte(newContent), 0644); err != nil {
			t.Fatalf("failed to update file: %v", err)
		}

		// Wait for detection and reload
		time.Sleep(100 * time.Millisecond)

		// Check reload happened
		newRules := len(manager.GetConfig().Rules)
		if newRules != 2 {
			t.Errorf("expected 2 rules after reload, got %d", newRules)
		}

		stats := manager.Stats()
		if stats.ReloadCount < 1 {
			t.Errorf("expected at least 1 reload, got %d", stats.ReloadCount)
		}
	})

	t.Run("debounce prevents multiple reloads", func(t *testing.T) {
		content := `
version: 1
defaultPolicy: deny
rules:
  - target: "*"
    subject: "cn=admin,dc=test,dc=com"
    rights: [read]
`
		tmpFile := createTempACLFile(t, content)
		defer os.Remove(tmpFile)

		manager, err := NewManager(&ManagerConfig{FilePath: tmpFile})
		if err != nil {
			t.Fatalf("failed to create manager: %v", err)
		}

		watcher, err := NewFileWatcher(&WatcherConfig{
			FilePath:     tmpFile,
			Manager:      manager,
			PollInterval: 10 * time.Millisecond,
			Debounce:     100 * time.Millisecond,
		})
		if err != nil {
			t.Fatalf("failed to create watcher: %v", err)
		}

		watcher.Start()
		defer watcher.Stop()

		time.Sleep(30 * time.Millisecond)

		// Rapid file updates (simulating partial writes)
		for i := 0; i < 5; i++ {
			newContent := `
version: 1
defaultPolicy: deny
rules:
  - target: "*"
    subject: "cn=admin,dc=test,dc=com"
    rights: [read, write]
`
			if err := os.WriteFile(tmpFile, []byte(newContent), 0644); err != nil {
				t.Fatalf("failed to update file: %v", err)
			}
			time.Sleep(10 * time.Millisecond)
		}

		// Wait for debounce to complete
		time.Sleep(200 * time.Millisecond)

		// Should have only 1 reload due to debounce
		stats := manager.Stats()
		if stats.ReloadCount != 1 {
			t.Errorf("expected 1 reload due to debounce, got %d", stats.ReloadCount)
		}
	})

	t.Run("stop prevents further reloads", func(t *testing.T) {
		content := `
version: 1
defaultPolicy: deny
rules:
  - target: "*"
    subject: "cn=admin,dc=test,dc=com"
    rights: [read]
`
		tmpFile := createTempACLFile(t, content)
		defer os.Remove(tmpFile)

		manager, err := NewManager(&ManagerConfig{FilePath: tmpFile})
		if err != nil {
			t.Fatalf("failed to create manager: %v", err)
		}

		watcher, err := NewFileWatcher(&WatcherConfig{
			FilePath:     tmpFile,
			Manager:      manager,
			PollInterval: 10 * time.Millisecond,
			Debounce:     20 * time.Millisecond,
		})
		if err != nil {
			t.Fatalf("failed to create watcher: %v", err)
		}

		watcher.Start()
		time.Sleep(30 * time.Millisecond)

		// Stop watcher
		watcher.Stop()

		if watcher.IsRunning() {
			t.Error("watcher should not be running after Stop()")
		}

		// Update file after stop
		newContent := `
version: 1
defaultPolicy: deny
rules:
  - target: "*"
    subject: "cn=admin,dc=test,dc=com"
    rights: [read, write, delete]
`
		if err := os.WriteFile(tmpFile, []byte(newContent), 0644); err != nil {
			t.Fatalf("failed to update file: %v", err)
		}

		time.Sleep(100 * time.Millisecond)

		// Should still have original config (no reload after stop)
		stats := manager.Stats()
		if stats.ReloadCount != 0 {
			t.Errorf("expected 0 reloads after stop, got %d", stats.ReloadCount)
		}
	})
}

func TestFileWatcherErrors(t *testing.T) {
	t.Run("missing file path", func(t *testing.T) {
		_, err := NewFileWatcher(&WatcherConfig{
			Manager: &Manager{},
		})
		if err != ErrNoFilePath {
			t.Errorf("expected ErrNoFilePath, got %v", err)
		}
	})

	t.Run("missing manager", func(t *testing.T) {
		_, err := NewFileWatcher(&WatcherConfig{
			FilePath: "/tmp/test.yaml",
		})
		if err != ErrInvalidConfig {
			t.Errorf("expected ErrInvalidConfig, got %v", err)
		}
	})

	t.Run("non-existent file", func(t *testing.T) {
		_, err := NewFileWatcher(&WatcherConfig{
			FilePath: "/tmp/non-existent-acl-file.yaml",
			Manager:  &Manager{},
		})
		if err == nil {
			t.Error("expected error for non-existent file")
		}
	})
}
