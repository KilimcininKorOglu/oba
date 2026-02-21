// Package config provides configuration parsing and management for the Oba LDAP server.
package config

import "time"

// DefaultConfig returns a Config with sensible default values.
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Address:        ":389",
			TLSAddress:     ":636",
			TLSCert:        "",
			TLSKey:         "",
			MaxConnections: 10000,
			ReadTimeout:    30 * time.Second,
			WriteTimeout:   30 * time.Second,
		},
		Directory: DirectoryConfig{
			BaseDN:       "",
			RootDN:       "",
			RootPassword: "",
		},
		Storage: StorageConfig{
			DataDir:            "/var/lib/oba",
			WALDir:             "",
			PageSize:           4096,
			BufferPoolSize:     "256MB",
			CheckpointInterval: 5 * time.Minute,
			CacheSize:          10000,
		},
		Logging: LogConfig{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		},
		Security: SecurityConfig{
			PasswordPolicy: PasswordPolicyConfig{
				Enabled:          false,
				MinLength:        8,
				RequireUppercase: true,
				RequireLowercase: true,
				RequireDigit:     true,
				RequireSpecial:   false,
				MaxAge:           0,
				HistoryCount:     0,
			},
			RateLimit: RateLimitConfig{
				Enabled:         false,
				MaxAttempts:     5,
				LockoutDuration: 15 * time.Minute,
			},
		},
		ACL: ACLConfig{
			DefaultPolicy: "deny",
			Rules:         nil,
		},
		REST: RESTConfig{
			Enabled:     false,
			Address:     ":8080",
			TLSAddress:  "",
			JWTSecret:   "",
			TokenTTL:    24 * time.Hour,
			RateLimit:   100,
			CORSOrigins: []string{"*"},
		},
	}
}
