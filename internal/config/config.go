// Package config provides configuration parsing and management for the Oba LDAP server.
package config

import (
	"path/filepath"
	"time"
)

// Config holds the complete server configuration.
type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Directory DirectoryConfig `yaml:"directory"`
	Storage   StorageConfig   `yaml:"storage"`
	Logging   LogConfig       `yaml:"logging"`
	Security  SecurityConfig  `yaml:"security"`
	ACL       ACLConfig       `yaml:"acl"`
}

// ResolvePaths resolves relative paths in the configuration to absolute paths.
// This should be called after loading the configuration and before using it.
func (c *Config) ResolvePaths() error {
	var err error

	// Resolve data directory
	if c.Storage.DataDir != "" {
		c.Storage.DataDir, err = filepath.Abs(c.Storage.DataDir)
		if err != nil {
			return err
		}
	}

	// Resolve WAL directory
	if c.Storage.WALDir != "" {
		c.Storage.WALDir, err = filepath.Abs(c.Storage.WALDir)
		if err != nil {
			return err
		}
	}

	// Resolve TLS certificate paths
	if c.Server.TLSCert != "" {
		c.Server.TLSCert, err = filepath.Abs(c.Server.TLSCert)
		if err != nil {
			return err
		}
	}

	if c.Server.TLSKey != "" {
		c.Server.TLSKey, err = filepath.Abs(c.Server.TLSKey)
		if err != nil {
			return err
		}
	}

	return nil
}

// ServerConfig holds server-related configuration.
type ServerConfig struct {
	Address        string        `yaml:"address"`
	TLSAddress     string        `yaml:"tlsAddress"`
	TLSCert        string        `yaml:"tlsCert"`
	TLSKey         string        `yaml:"tlsKey"`
	MaxConnections int           `yaml:"maxConnections"`
	ReadTimeout    time.Duration `yaml:"readTimeout"`
	WriteTimeout   time.Duration `yaml:"writeTimeout"`
}

// DirectoryConfig holds directory-related configuration.
type DirectoryConfig struct {
	BaseDN       string `yaml:"baseDN"`
	RootDN       string `yaml:"rootDN"`
	RootPassword string `yaml:"rootPassword"`
}

// StorageConfig holds storage engine configuration.
type StorageConfig struct {
	DataDir            string        `yaml:"dataDir"`
	WALDir             string        `yaml:"walDir"`
	PageSize           int           `yaml:"pageSize"`
	BufferPoolSize     string        `yaml:"bufferPoolSize"`
	CheckpointInterval time.Duration `yaml:"checkpointInterval"`
}

// LogConfig holds logging configuration.
type LogConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
	Output string `yaml:"output"`
}

// SecurityConfig holds security-related configuration.
type SecurityConfig struct {
	PasswordPolicy PasswordPolicyConfig `yaml:"passwordPolicy"`
	RateLimit      RateLimitConfig      `yaml:"rateLimit"`
}

// PasswordPolicyConfig holds password policy configuration.
type PasswordPolicyConfig struct {
	Enabled          bool          `yaml:"enabled"`
	MinLength        int           `yaml:"minLength"`
	RequireUppercase bool          `yaml:"requireUppercase"`
	RequireLowercase bool          `yaml:"requireLowercase"`
	RequireDigit     bool          `yaml:"requireDigit"`
	RequireSpecial   bool          `yaml:"requireSpecial"`
	MaxAge           time.Duration `yaml:"maxAge"`
	HistoryCount     int           `yaml:"historyCount"`
}

// RateLimitConfig holds rate limiting configuration.
type RateLimitConfig struct {
	Enabled         bool          `yaml:"enabled"`
	MaxAttempts     int           `yaml:"maxAttempts"`
	LockoutDuration time.Duration `yaml:"lockoutDuration"`
}

// ACLConfig holds access control list configuration.
type ACLConfig struct {
	DefaultPolicy string        `yaml:"defaultPolicy"`
	Rules         []ACLRuleConfig `yaml:"rules"`
}

// ACLRuleConfig holds a single ACL rule configuration.
type ACLRuleConfig struct {
	Target     string   `yaml:"target"`
	Subject    string   `yaml:"subject"`
	Rights     []string `yaml:"rights"`
	Attributes []string `yaml:"attributes"`
}
