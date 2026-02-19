package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	t.Run("server defaults", func(t *testing.T) {
		if config.Server.Address != ":389" {
			t.Errorf("expected address ':389', got %q", config.Server.Address)
		}
		if config.Server.TLSAddress != ":636" {
			t.Errorf("expected TLS address ':636', got %q", config.Server.TLSAddress)
		}
		if config.Server.MaxConnections != 10000 {
			t.Errorf("expected max connections 10000, got %d", config.Server.MaxConnections)
		}
		if config.Server.ReadTimeout != 30*time.Second {
			t.Errorf("expected read timeout 30s, got %v", config.Server.ReadTimeout)
		}
		if config.Server.WriteTimeout != 30*time.Second {
			t.Errorf("expected write timeout 30s, got %v", config.Server.WriteTimeout)
		}
	})

	t.Run("storage defaults", func(t *testing.T) {
		if config.Storage.DataDir != "/var/lib/oba" {
			t.Errorf("expected data dir '/var/lib/oba', got %q", config.Storage.DataDir)
		}
		if config.Storage.PageSize != 4096 {
			t.Errorf("expected page size 4096, got %d", config.Storage.PageSize)
		}
		if config.Storage.BufferPoolSize != "256MB" {
			t.Errorf("expected buffer pool size '256MB', got %q", config.Storage.BufferPoolSize)
		}
		if config.Storage.CheckpointInterval != 5*time.Minute {
			t.Errorf("expected checkpoint interval 5m, got %v", config.Storage.CheckpointInterval)
		}
	})

	t.Run("logging defaults", func(t *testing.T) {
		if config.Logging.Level != "info" {
			t.Errorf("expected log level 'info', got %q", config.Logging.Level)
		}
		if config.Logging.Format != "json" {
			t.Errorf("expected log format 'json', got %q", config.Logging.Format)
		}
		if config.Logging.Output != "stdout" {
			t.Errorf("expected log output 'stdout', got %q", config.Logging.Output)
		}
	})

	t.Run("acl defaults", func(t *testing.T) {
		if config.ACL.DefaultPolicy != "deny" {
			t.Errorf("expected default policy 'deny', got %q", config.ACL.DefaultPolicy)
		}
	})
}

func TestParseConfig(t *testing.T) {
	t.Run("empty config uses defaults", func(t *testing.T) {
		config, err := ParseConfig([]byte(""))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if config.Server.Address != ":389" {
			t.Errorf("expected default address ':389', got %q", config.Server.Address)
		}
	})

	t.Run("parse server config", func(t *testing.T) {
		yaml := `
server:
  address: ":1389"
  tlsAddress: ":1636"
  maxConnections: 5000
  readTimeout: 60s
  writeTimeout: 45s
`
		config, err := ParseConfig([]byte(yaml))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if config.Server.Address != ":1389" {
			t.Errorf("expected address ':1389', got %q", config.Server.Address)
		}
		if config.Server.TLSAddress != ":1636" {
			t.Errorf("expected TLS address ':1636', got %q", config.Server.TLSAddress)
		}
		if config.Server.MaxConnections != 5000 {
			t.Errorf("expected max connections 5000, got %d", config.Server.MaxConnections)
		}
		if config.Server.ReadTimeout != 60*time.Second {
			t.Errorf("expected read timeout 60s, got %v", config.Server.ReadTimeout)
		}
		if config.Server.WriteTimeout != 45*time.Second {
			t.Errorf("expected write timeout 45s, got %v", config.Server.WriteTimeout)
		}
	})

	t.Run("parse directory config", func(t *testing.T) {
		yaml := `
directory:
  baseDN: "dc=example,dc=com"
  rootDN: "cn=admin,dc=example,dc=com"
  rootPassword: "secret"
`
		config, err := ParseConfig([]byte(yaml))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if config.Directory.BaseDN != "dc=example,dc=com" {
			t.Errorf("expected baseDN 'dc=example,dc=com', got %q", config.Directory.BaseDN)
		}
		if config.Directory.RootDN != "cn=admin,dc=example,dc=com" {
			t.Errorf("expected rootDN 'cn=admin,dc=example,dc=com', got %q", config.Directory.RootDN)
		}
		if config.Directory.RootPassword != "secret" {
			t.Errorf("expected rootPassword 'secret', got %q", config.Directory.RootPassword)
		}
	})

	t.Run("parse storage config", func(t *testing.T) {
		yaml := `
storage:
  dataDir: "/data/oba"
  walDir: "/data/oba/wal"
  pageSize: 8192
  bufferPoolSize: "512MB"
  checkpointInterval: 10m
`
		config, err := ParseConfig([]byte(yaml))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if config.Storage.DataDir != "/data/oba" {
			t.Errorf("expected dataDir '/data/oba', got %q", config.Storage.DataDir)
		}
		if config.Storage.WALDir != "/data/oba/wal" {
			t.Errorf("expected walDir '/data/oba/wal', got %q", config.Storage.WALDir)
		}
		if config.Storage.PageSize != 8192 {
			t.Errorf("expected pageSize 8192, got %d", config.Storage.PageSize)
		}
		if config.Storage.BufferPoolSize != "512MB" {
			t.Errorf("expected bufferPoolSize '512MB', got %q", config.Storage.BufferPoolSize)
		}
		if config.Storage.CheckpointInterval != 10*time.Minute {
			t.Errorf("expected checkpointInterval 10m, got %v", config.Storage.CheckpointInterval)
		}
	})

	t.Run("parse logging config", func(t *testing.T) {
		yaml := `
logging:
  level: "debug"
  format: "text"
  output: "/var/log/oba.log"
`
		config, err := ParseConfig([]byte(yaml))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if config.Logging.Level != "debug" {
			t.Errorf("expected level 'debug', got %q", config.Logging.Level)
		}
		if config.Logging.Format != "text" {
			t.Errorf("expected format 'text', got %q", config.Logging.Format)
		}
		if config.Logging.Output != "/var/log/oba.log" {
			t.Errorf("expected output '/var/log/oba.log', got %q", config.Logging.Output)
		}
	})

	t.Run("parse security config", func(t *testing.T) {
		yaml := `
security:
  passwordPolicy:
    enabled: true
    minLength: 12
    requireUppercase: true
    requireLowercase: true
    requireDigit: true
    requireSpecial: true
    maxAge: 90d
    historyCount: 5
  rateLimit:
    enabled: true
    maxAttempts: 3
    lockoutDuration: 30m
`
		config, err := ParseConfig([]byte(yaml))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !config.Security.PasswordPolicy.Enabled {
			t.Error("expected password policy enabled")
		}
		if config.Security.PasswordPolicy.MinLength != 12 {
			t.Errorf("expected minLength 12, got %d", config.Security.PasswordPolicy.MinLength)
		}
		if !config.Security.PasswordPolicy.RequireSpecial {
			t.Error("expected requireSpecial true")
		}
		if config.Security.PasswordPolicy.MaxAge != 90*24*time.Hour {
			t.Errorf("expected maxAge 90d, got %v", config.Security.PasswordPolicy.MaxAge)
		}
		if config.Security.PasswordPolicy.HistoryCount != 5 {
			t.Errorf("expected historyCount 5, got %d", config.Security.PasswordPolicy.HistoryCount)
		}
		if !config.Security.RateLimit.Enabled {
			t.Error("expected rate limit enabled")
		}
		if config.Security.RateLimit.MaxAttempts != 3 {
			t.Errorf("expected maxAttempts 3, got %d", config.Security.RateLimit.MaxAttempts)
		}
		if config.Security.RateLimit.LockoutDuration != 30*time.Minute {
			t.Errorf("expected lockoutDuration 30m, got %v", config.Security.RateLimit.LockoutDuration)
		}
	})

	t.Run("parse acl config", func(t *testing.T) {
		yaml := `
acl:
  defaultPolicy: "allow"
`
		config, err := ParseConfig([]byte(yaml))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if config.ACL.DefaultPolicy != "allow" {
			t.Errorf("expected defaultPolicy 'allow', got %q", config.ACL.DefaultPolicy)
		}
	})

	t.Run("parse quoted values", func(t *testing.T) {
		yaml := `
directory:
  baseDN: "dc=example,dc=com"
  rootPassword: 'single-quoted'
`
		config, err := ParseConfig([]byte(yaml))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if config.Directory.BaseDN != "dc=example,dc=com" {
			t.Errorf("expected baseDN 'dc=example,dc=com', got %q", config.Directory.BaseDN)
		}
		if config.Directory.RootPassword != "single-quoted" {
			t.Errorf("expected rootPassword 'single-quoted', got %q", config.Directory.RootPassword)
		}
	})

	t.Run("skip comments", func(t *testing.T) {
		yaml := `
# This is a comment
server:
  # Another comment
  address: ":1389"
`
		config, err := ParseConfig([]byte(yaml))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if config.Server.Address != ":1389" {
			t.Errorf("expected address ':1389', got %q", config.Server.Address)
		}
	})

	t.Run("partial config merges with defaults", func(t *testing.T) {
		yaml := `
server:
  address: ":1389"
`
		config, err := ParseConfig([]byte(yaml))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Overridden value
		if config.Server.Address != ":1389" {
			t.Errorf("expected address ':1389', got %q", config.Server.Address)
		}
		// Default value preserved
		if config.Server.TLSAddress != ":636" {
			t.Errorf("expected default TLS address ':636', got %q", config.Server.TLSAddress)
		}
		// Other sections use defaults
		if config.Logging.Level != "info" {
			t.Errorf("expected default log level 'info', got %q", config.Logging.Level)
		}
	})
}

func TestEnvironmentVariableSubstitution(t *testing.T) {
	t.Run("simple substitution", func(t *testing.T) {
		os.Setenv("TEST_OBA_ADDRESS", ":2389")
		defer os.Unsetenv("TEST_OBA_ADDRESS")

		yaml := `
server:
  address: "${TEST_OBA_ADDRESS}"
`
		config, err := ParseConfig([]byte(yaml))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if config.Server.Address != ":2389" {
			t.Errorf("expected address ':2389', got %q", config.Server.Address)
		}
	})

	t.Run("substitution with default value", func(t *testing.T) {
		// Ensure the variable is not set
		os.Unsetenv("TEST_OBA_MISSING")

		yaml := `
server:
  address: "${TEST_OBA_MISSING:-:3389}"
`
		config, err := ParseConfig([]byte(yaml))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if config.Server.Address != ":3389" {
			t.Errorf("expected address ':3389', got %q", config.Server.Address)
		}
	})

	t.Run("substitution with default when var is set", func(t *testing.T) {
		os.Setenv("TEST_OBA_SET", ":4389")
		defer os.Unsetenv("TEST_OBA_SET")

		yaml := `
server:
  address: "${TEST_OBA_SET:-:5389}"
`
		config, err := ParseConfig([]byte(yaml))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if config.Server.Address != ":4389" {
			t.Errorf("expected address ':4389', got %q", config.Server.Address)
		}
	})

	t.Run("multiple substitutions", func(t *testing.T) {
		os.Setenv("TEST_OBA_ADDR", ":6389")
		os.Setenv("TEST_OBA_TLS", ":6636")
		defer os.Unsetenv("TEST_OBA_ADDR")
		defer os.Unsetenv("TEST_OBA_TLS")

		yaml := `
server:
  address: "${TEST_OBA_ADDR}"
  tlsAddress: "${TEST_OBA_TLS}"
`
		config, err := ParseConfig([]byte(yaml))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if config.Server.Address != ":6389" {
			t.Errorf("expected address ':6389', got %q", config.Server.Address)
		}
		if config.Server.TLSAddress != ":6636" {
			t.Errorf("expected TLS address ':6636', got %q", config.Server.TLSAddress)
		}
	})

	t.Run("unset variable becomes empty", func(t *testing.T) {
		os.Unsetenv("TEST_OBA_UNSET")

		yaml := `
directory:
  rootPassword: "${TEST_OBA_UNSET}"
`
		config, err := ParseConfig([]byte(yaml))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Empty value should not override default (which is also empty)
		if config.Directory.RootPassword != "" {
			t.Errorf("expected empty rootPassword, got %q", config.Directory.RootPassword)
		}
	})
}

func TestLoadConfig(t *testing.T) {
	t.Run("load from file", func(t *testing.T) {
		// Create a temporary config file
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.yaml")

		yaml := `
server:
  address: ":7389"
  maxConnections: 2000
logging:
  level: "warn"
`
		if err := os.WriteFile(configPath, []byte(yaml), 0644); err != nil {
			t.Fatalf("failed to write config file: %v", err)
		}

		config, err := LoadConfig(configPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if config.Server.Address != ":7389" {
			t.Errorf("expected address ':7389', got %q", config.Server.Address)
		}
		if config.Server.MaxConnections != 2000 {
			t.Errorf("expected max connections 2000, got %d", config.Server.MaxConnections)
		}
		if config.Logging.Level != "warn" {
			t.Errorf("expected log level 'warn', got %q", config.Logging.Level)
		}
	})

	t.Run("file not found", func(t *testing.T) {
		_, err := LoadConfig("/nonexistent/path/config.yaml")
		if err != ErrFileNotFound {
			t.Errorf("expected ErrFileNotFound, got %v", err)
		}
	})
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
		hasError bool
	}{
		{"30s", 30 * time.Second, false},
		{"5m", 5 * time.Minute, false},
		{"1h", 1 * time.Hour, false},
		{"90d", 90 * 24 * time.Hour, false},
		{"1h30m", 90 * time.Minute, false},
		{"", 0, false},
		{"invalid", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := parseDuration(tt.input)
			if tt.hasError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("expected %v, got %v", tt.expected, result)
				}
			}
		})
	}
}

func TestParseBool(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"true", true},
		{"True", true},
		{"TRUE", true},
		{"yes", true},
		{"Yes", true},
		{"1", true},
		{"on", true},
		{"false", false},
		{"False", false},
		{"no", false},
		{"0", false},
		{"off", false},
		{"", false},
		{"invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseBool(tt.input)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestNestedStructures(t *testing.T) {
	yaml := `
server:
  address: ":8389"
  tlsAddress: ":8636"
  maxConnections: 1000
  readTimeout: 15s
  writeTimeout: 20s
directory:
  baseDN: "dc=test,dc=com"
  rootDN: "cn=admin,dc=test,dc=com"
storage:
  dataDir: "/tmp/oba"
  pageSize: 4096
logging:
  level: "debug"
  format: "text"
security:
  passwordPolicy:
    enabled: true
    minLength: 10
  rateLimit:
    enabled: true
    maxAttempts: 5
acl:
  defaultPolicy: "deny"
`
	config, err := ParseConfig([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify all nested values
	if config.Server.Address != ":8389" {
		t.Errorf("server.address: expected ':8389', got %q", config.Server.Address)
	}
	if config.Server.TLSAddress != ":8636" {
		t.Errorf("server.tlsAddress: expected ':8636', got %q", config.Server.TLSAddress)
	}
	if config.Server.MaxConnections != 1000 {
		t.Errorf("server.maxConnections: expected 1000, got %d", config.Server.MaxConnections)
	}
	if config.Server.ReadTimeout != 15*time.Second {
		t.Errorf("server.readTimeout: expected 15s, got %v", config.Server.ReadTimeout)
	}
	if config.Server.WriteTimeout != 20*time.Second {
		t.Errorf("server.writeTimeout: expected 20s, got %v", config.Server.WriteTimeout)
	}
	if config.Directory.BaseDN != "dc=test,dc=com" {
		t.Errorf("directory.baseDN: expected 'dc=test,dc=com', got %q", config.Directory.BaseDN)
	}
	if config.Directory.RootDN != "cn=admin,dc=test,dc=com" {
		t.Errorf("directory.rootDN: expected 'cn=admin,dc=test,dc=com', got %q", config.Directory.RootDN)
	}
	if config.Storage.DataDir != "/tmp/oba" {
		t.Errorf("storage.dataDir: expected '/tmp/oba', got %q", config.Storage.DataDir)
	}
	if config.Storage.PageSize != 4096 {
		t.Errorf("storage.pageSize: expected 4096, got %d", config.Storage.PageSize)
	}
	if config.Logging.Level != "debug" {
		t.Errorf("logging.level: expected 'debug', got %q", config.Logging.Level)
	}
	if config.Logging.Format != "text" {
		t.Errorf("logging.format: expected 'text', got %q", config.Logging.Format)
	}
	if !config.Security.PasswordPolicy.Enabled {
		t.Error("security.passwordPolicy.enabled: expected true")
	}
	if config.Security.PasswordPolicy.MinLength != 10 {
		t.Errorf("security.passwordPolicy.minLength: expected 10, got %d", config.Security.PasswordPolicy.MinLength)
	}
	if !config.Security.RateLimit.Enabled {
		t.Error("security.rateLimit.enabled: expected true")
	}
	if config.Security.RateLimit.MaxAttempts != 5 {
		t.Errorf("security.rateLimit.maxAttempts: expected 5, got %d", config.Security.RateLimit.MaxAttempts)
	}
	if config.ACL.DefaultPolicy != "deny" {
		t.Errorf("acl.defaultPolicy: expected 'deny', got %q", config.ACL.DefaultPolicy)
	}
}

func TestInvalidYAML(t *testing.T) {
	t.Run("missing colon", func(t *testing.T) {
		yaml := `
server
  address: ":389"
`
		_, err := ParseConfig([]byte(yaml))
		if err == nil {
			t.Error("expected error for invalid YAML")
		}
	})

	t.Run("invalid number", func(t *testing.T) {
		yaml := `
server:
  maxConnections: not-a-number
`
		_, err := ParseConfig([]byte(yaml))
		if err != ErrInvalidNumber {
			t.Errorf("expected ErrInvalidNumber, got %v", err)
		}
	})

	t.Run("invalid duration", func(t *testing.T) {
		yaml := `
server:
  readTimeout: invalid-duration
`
		_, err := ParseConfig([]byte(yaml))
		if err != ErrInvalidDuration {
			t.Errorf("expected ErrInvalidDuration, got %v", err)
		}
	})
}

func TestCompleteConfigExample(t *testing.T) {
	// Test a complete configuration similar to the PRD example
	yaml := `
server:
  address: ":389"
  tlsAddress: ":636"
  tlsCert: "/etc/oba/certs/server.crt"
  tlsKey: "/etc/oba/certs/server.key"
  maxConnections: 10000
  readTimeout: 30s
  writeTimeout: 30s

directory:
  baseDN: "dc=example,dc=com"
  rootDN: "cn=admin,dc=example,dc=com"
  rootPassword: "admin-secret"

storage:
  dataDir: "/var/lib/oba"
  walDir: "/var/lib/oba/wal"
  pageSize: 4096
  bufferPoolSize: "256MB"
  checkpointInterval: 5m

logging:
  level: "info"
  format: "json"
  output: "/var/log/oba/oba.log"

security:
  passwordPolicy:
    enabled: true
    minLength: 8
    requireUppercase: true
    requireLowercase: true
    requireDigit: true
    requireSpecial: false
    maxAge: 90d
    historyCount: 5
  rateLimit:
    enabled: true
    maxAttempts: 5
    lockoutDuration: 15m

acl:
  defaultPolicy: "deny"
`
	config, err := ParseConfig([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify key values
	if config.Server.Address != ":389" {
		t.Errorf("server.address mismatch")
	}
	if config.Server.TLSCert != "/etc/oba/certs/server.crt" {
		t.Errorf("server.tlsCert mismatch")
	}
	if config.Directory.BaseDN != "dc=example,dc=com" {
		t.Errorf("directory.baseDN mismatch")
	}
	if config.Storage.WALDir != "/var/lib/oba/wal" {
		t.Errorf("storage.walDir mismatch")
	}
	if config.Logging.Output != "/var/log/oba/oba.log" {
		t.Errorf("logging.output mismatch")
	}
	if config.Security.PasswordPolicy.MaxAge != 90*24*time.Hour {
		t.Errorf("security.passwordPolicy.maxAge mismatch: got %v", config.Security.PasswordPolicy.MaxAge)
	}
	if config.Security.RateLimit.LockoutDuration != 15*time.Minute {
		t.Errorf("security.rateLimit.lockoutDuration mismatch")
	}
}

func TestSubstituteEnvVars(t *testing.T) {
	t.Run("substitute single var", func(t *testing.T) {
		os.Setenv("TEST_VAR", "value")
		defer os.Unsetenv("TEST_VAR")

		input := []byte("key: ${TEST_VAR}")
		result := substituteEnvVars(input)
		expected := "key: value"
		if string(result) != expected {
			t.Errorf("expected %q, got %q", expected, string(result))
		}
	})

	t.Run("substitute with default", func(t *testing.T) {
		os.Unsetenv("TEST_MISSING")

		input := []byte("key: ${TEST_MISSING:-default}")
		result := substituteEnvVars(input)
		expected := "key: default"
		if string(result) != expected {
			t.Errorf("expected %q, got %q", expected, string(result))
		}
	})

	t.Run("no substitution needed", func(t *testing.T) {
		input := []byte("key: value")
		result := substituteEnvVars(input)
		if string(result) != string(input) {
			t.Errorf("expected %q, got %q", string(input), string(result))
		}
	})
}
