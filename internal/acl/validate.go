package acl

import (
	"errors"
	"fmt"
)

// Validation errors.
var (
	ErrNoFilePath    = errors.New("file path is required")
	ErrInvalidConfig = errors.New("invalid configuration")
)

// ValidateConfig validates an ACL configuration.
// Returns a slice of errors found during validation.
func ValidateConfig(config *Config) []error {
	var errs []error

	if config == nil {
		return []error{fmt.Errorf("config is nil")}
	}

	// Validate default policy
	policy := config.DefaultPolicy
	if policy != "" && policy != "allow" && policy != "deny" {
		errs = append(errs, fmt.Errorf("invalid defaultPolicy: %s (must be allow or deny)", policy))
	}

	// Validate rules
	for i, rule := range config.Rules {
		if rule == nil {
			errs = append(errs, fmt.Errorf("rule %d: is nil", i))
			continue
		}

		if rule.Target == "" {
			errs = append(errs, fmt.Errorf("rule %d: target is required", i))
		}

		if rule.Subject == "" {
			errs = append(errs, fmt.Errorf("rule %d: subject is required", i))
		}

		if rule.Rights == 0 {
			errs = append(errs, fmt.Errorf("rule %d: at least one right is required", i))
		}

		// Validate scope
		if rule.Scope < ScopeBase || rule.Scope > ScopeSubtree {
			errs = append(errs, fmt.Errorf("rule %d: invalid scope %d", i, rule.Scope))
		}
	}

	return errs
}
