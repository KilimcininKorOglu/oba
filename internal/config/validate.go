// Package config provides configuration parsing and management for the Oba LDAP server.
package config

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
)

// ValidationError represents a configuration validation error.
type ValidationError struct {
	Field   string
	Message string
}

// Error implements the error interface.
func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidateConfig validates the configuration and returns a list of validation errors.
// An empty slice indicates the configuration is valid.
func ValidateConfig(config *Config) []error {
	var errs []error

	// Validate server configuration
	errs = append(errs, validateServerConfig(&config.Server)...)

	// Validate directory configuration
	errs = append(errs, validateDirectoryConfig(&config.Directory)...)

	// Validate storage configuration
	errs = append(errs, validateStorageConfig(&config.Storage)...)

	// Validate logging configuration
	errs = append(errs, validateLogConfig(&config.Logging)...)

	// Validate security configuration
	errs = append(errs, validateSecurityConfig(&config.Security)...)

	// Validate ACL configuration
	errs = append(errs, validateACLConfig(&config.ACL)...)

	return errs
}

// validateServerConfig validates server configuration.
func validateServerConfig(config *ServerConfig) []error {
	var errs []error

	// Validate address format
	if config.Address != "" {
		if err := validateAddress(config.Address); err != nil {
			errs = append(errs, ValidationError{
				Field:   "server.address",
				Message: err.Error(),
			})
		}
	}

	// Validate TLS address format
	if config.TLSAddress != "" {
		if err := validateAddress(config.TLSAddress); err != nil {
			errs = append(errs, ValidationError{
				Field:   "server.tlsAddress",
				Message: err.Error(),
			})
		}
	}

	// Validate TLS certificate and key
	if config.TLSCert != "" || config.TLSKey != "" {
		if config.TLSCert == "" {
			errs = append(errs, ValidationError{
				Field:   "server.tlsCert",
				Message: "TLS certificate is required when TLS key is specified",
			})
		}
		if config.TLSKey == "" {
			errs = append(errs, ValidationError{
				Field:   "server.tlsKey",
				Message: "TLS key is required when TLS certificate is specified",
			})
		}
	}

	// Validate max connections
	if config.MaxConnections < 0 {
		errs = append(errs, ValidationError{
			Field:   "server.maxConnections",
			Message: "must be non-negative",
		})
	}

	// Validate timeouts
	if config.ReadTimeout < 0 {
		errs = append(errs, ValidationError{
			Field:   "server.readTimeout",
			Message: "must be non-negative",
		})
	}

	if config.WriteTimeout < 0 {
		errs = append(errs, ValidationError{
			Field:   "server.writeTimeout",
			Message: "must be non-negative",
		})
	}

	return errs
}

// validateDirectoryConfig validates directory configuration.
func validateDirectoryConfig(config *DirectoryConfig) []error {
	var errs []error

	// Validate baseDN format if provided
	if config.BaseDN != "" {
		if err := validateDN(config.BaseDN); err != nil {
			errs = append(errs, ValidationError{
				Field:   "directory.baseDN",
				Message: err.Error(),
			})
		}
	}

	// Validate rootDN format if provided
	if config.RootDN != "" {
		if err := validateDN(config.RootDN); err != nil {
			errs = append(errs, ValidationError{
				Field:   "directory.rootDN",
				Message: err.Error(),
			})
		}
	}

	return errs
}

// validateStorageConfig validates storage configuration.
func validateStorageConfig(config *StorageConfig) []error {
	var errs []error

	// Validate data directory
	if config.DataDir == "" {
		errs = append(errs, ValidationError{
			Field:   "storage.dataDir",
			Message: "data directory is required",
		})
	} else if !filepath.IsAbs(config.DataDir) {
		errs = append(errs, ValidationError{
			Field:   "storage.dataDir",
			Message: "must be an absolute path",
		})
	}

	// Validate WAL directory if specified
	if config.WALDir != "" && !filepath.IsAbs(config.WALDir) {
		errs = append(errs, ValidationError{
			Field:   "storage.walDir",
			Message: "must be an absolute path",
		})
	}

	// Validate page size
	validPageSizes := map[int]bool{4096: true, 8192: true, 16384: true, 32768: true}
	if config.PageSize != 0 && !validPageSizes[config.PageSize] {
		errs = append(errs, ValidationError{
			Field:   "storage.pageSize",
			Message: "must be 4096, 8192, 16384, or 32768",
		})
	}

	// Validate buffer pool size format
	if config.BufferPoolSize != "" {
		if _, err := parseSize(config.BufferPoolSize); err != nil {
			errs = append(errs, ValidationError{
				Field:   "storage.bufferPoolSize",
				Message: err.Error(),
			})
		}
	}

	// Validate checkpoint interval
	if config.CheckpointInterval < 0 {
		errs = append(errs, ValidationError{
			Field:   "storage.checkpointInterval",
			Message: "must be non-negative",
		})
	}

	return errs
}

// validateLogConfig validates logging configuration.
func validateLogConfig(config *LogConfig) []error {
	var errs []error

	// Validate log level
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if config.Level != "" && !validLevels[strings.ToLower(config.Level)] {
		errs = append(errs, ValidationError{
			Field:   "logging.level",
			Message: "must be debug, info, warn, or error",
		})
	}

	// Validate log format
	validFormats := map[string]bool{"text": true, "json": true}
	if config.Format != "" && !validFormats[strings.ToLower(config.Format)] {
		errs = append(errs, ValidationError{
			Field:   "logging.format",
			Message: "must be text or json",
		})
	}

	// Validate output
	if config.Output != "" && config.Output != "stdout" && config.Output != "stderr" {
		// Check if it's a valid file path
		dir := filepath.Dir(config.Output)
		if !filepath.IsAbs(config.Output) {
			errs = append(errs, ValidationError{
				Field:   "logging.output",
				Message: "must be stdout, stderr, or an absolute file path",
			})
		} else if _, err := os.Stat(dir); os.IsNotExist(err) {
			errs = append(errs, ValidationError{
				Field:   "logging.output",
				Message: fmt.Sprintf("directory %s does not exist", dir),
			})
		}
	}

	return errs
}

// validateSecurityConfig validates security configuration.
func validateSecurityConfig(config *SecurityConfig) []error {
	var errs []error

	// Validate password policy
	errs = append(errs, validatePasswordPolicyConfig(&config.PasswordPolicy)...)

	// Validate rate limit
	errs = append(errs, validateRateLimitConfig(&config.RateLimit)...)

	return errs
}

// validatePasswordPolicyConfig validates password policy configuration.
func validatePasswordPolicyConfig(config *PasswordPolicyConfig) []error {
	var errs []error

	if config.Enabled {
		if config.MinLength < 1 {
			errs = append(errs, ValidationError{
				Field:   "security.passwordPolicy.minLength",
				Message: "must be at least 1 when password policy is enabled",
			})
		}

		if config.HistoryCount < 0 {
			errs = append(errs, ValidationError{
				Field:   "security.passwordPolicy.historyCount",
				Message: "must be non-negative",
			})
		}

		if config.MaxAge < 0 {
			errs = append(errs, ValidationError{
				Field:   "security.passwordPolicy.maxAge",
				Message: "must be non-negative",
			})
		}
	}

	return errs
}

// validateRateLimitConfig validates rate limit configuration.
func validateRateLimitConfig(config *RateLimitConfig) []error {
	var errs []error

	if config.Enabled {
		if config.MaxAttempts < 1 {
			errs = append(errs, ValidationError{
				Field:   "security.rateLimit.maxAttempts",
				Message: "must be at least 1 when rate limiting is enabled",
			})
		}

		if config.LockoutDuration <= 0 {
			errs = append(errs, ValidationError{
				Field:   "security.rateLimit.lockoutDuration",
				Message: "must be positive when rate limiting is enabled",
			})
		}
	}

	return errs
}

// validateACLConfig validates ACL configuration.
func validateACLConfig(config *ACLConfig) []error {
	var errs []error

	// Validate default policy
	validPolicies := map[string]bool{"allow": true, "deny": true}
	if config.DefaultPolicy != "" && !validPolicies[strings.ToLower(config.DefaultPolicy)] {
		errs = append(errs, ValidationError{
			Field:   "acl.defaultPolicy",
			Message: "must be allow or deny",
		})
	}

	// Validate rules
	for i, rule := range config.Rules {
		if rule.Target == "" {
			errs = append(errs, ValidationError{
				Field:   fmt.Sprintf("acl.rules[%d].target", i),
				Message: "target is required",
			})
		}

		if rule.Subject == "" {
			errs = append(errs, ValidationError{
				Field:   fmt.Sprintf("acl.rules[%d].subject", i),
				Message: "subject is required",
			})
		}

		if len(rule.Rights) == 0 {
			errs = append(errs, ValidationError{
				Field:   fmt.Sprintf("acl.rules[%d].rights", i),
				Message: "at least one right is required",
			})
		}

		// Validate rights
		validRights := map[string]bool{
			"read": true, "write": true, "add": true,
			"delete": true, "search": true, "compare": true,
		}
		for _, right := range rule.Rights {
			if !validRights[strings.ToLower(right)] {
				errs = append(errs, ValidationError{
					Field:   fmt.Sprintf("acl.rules[%d].rights", i),
					Message: fmt.Sprintf("invalid right: %s", right),
				})
			}
		}
	}

	return errs
}

// validateAddress validates a network address in host:port format.
func validateAddress(addr string) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("invalid address format: %v", err)
	}

	// Validate host if not empty
	if host != "" && host != "localhost" {
		if ip := net.ParseIP(host); ip == nil {
			// Not an IP, could be a hostname - that's okay
		}
	}

	// Validate port
	if port == "" {
		return fmt.Errorf("port is required")
	}

	return nil
}

// validateDN validates a distinguished name format.
func validateDN(dn string) error {
	if dn == "" {
		return nil
	}

	// Basic DN validation: should contain at least one RDN
	parts := strings.Split(dn, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if !strings.Contains(part, "=") {
			return fmt.Errorf("invalid RDN format: %s", part)
		}
	}

	return nil
}

// parseSize parses a size string like "256MB" or "1GB".
func parseSize(s string) (int64, error) {
	s = strings.TrimSpace(strings.ToUpper(s))
	if s == "" {
		return 0, nil
	}

	multipliers := map[string]int64{
		"B":  1,
		"KB": 1024,
		"MB": 1024 * 1024,
		"GB": 1024 * 1024 * 1024,
		"TB": 1024 * 1024 * 1024 * 1024,
	}

	for suffix, mult := range multipliers {
		if strings.HasSuffix(s, suffix) {
			numStr := strings.TrimSuffix(s, suffix)
			var num int64
			if _, err := fmt.Sscanf(numStr, "%d", &num); err != nil {
				return 0, fmt.Errorf("invalid size format: %s", s)
			}
			return num * mult, nil
		}
	}

	// Try parsing as plain number
	var num int64
	if _, err := fmt.Sscanf(s, "%d", &num); err != nil {
		return 0, fmt.Errorf("invalid size format: %s", s)
	}
	return num, nil
}
