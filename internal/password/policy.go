// Package password provides password policy configuration and validation
// for the Oba LDAP server.
package password

import (
	"strings"
	"time"
	"unicode"
)

// Policy defines password complexity rules, expiration settings, and lockout parameters.
type Policy struct {
	// Enabled indicates whether password policy enforcement is active
	Enabled bool

	// MinLength is the minimum required password length
	MinLength int

	// MaxLength is the maximum allowed password length (0 = unlimited)
	MaxLength int

	// RequireUppercase requires at least one uppercase letter
	RequireUppercase bool

	// RequireLowercase requires at least one lowercase letter
	RequireLowercase bool

	// RequireDigit requires at least one numeric digit
	RequireDigit bool

	// RequireSpecial requires at least one special character
	RequireSpecial bool

	// MaxAge is the maximum password age before expiration (0 = never expires)
	MaxAge time.Duration

	// MinAge is the minimum time before a password can be changed
	MinAge time.Duration

	// HistoryCount is the number of old passwords to remember (0 = no history)
	HistoryCount int

	// MaxFailures is the number of failed attempts before lockout (0 = no lockout)
	MaxFailures int

	// LockoutDuration is how long an account stays locked after MaxFailures
	LockoutDuration time.Duration

	// AllowUserChange indicates whether users can change their own password
	AllowUserChange bool

	// MustChangeOnReset requires password change after admin reset
	MustChangeOnReset bool
}

// ValidationError represents a password validation failure.
type ValidationError struct {
	Code    ValidationErrorCode
	Message string
}

// Error implements the error interface.
func (e *ValidationError) Error() string {
	return e.Message
}

// ValidationErrorCode represents specific validation failure types.
type ValidationErrorCode int

const (
	// ErrTooShort indicates password is shorter than MinLength
	ErrTooShort ValidationErrorCode = iota + 1

	// ErrTooLong indicates password exceeds MaxLength
	ErrTooLong

	// ErrNoUppercase indicates missing required uppercase letter
	ErrNoUppercase

	// ErrNoLowercase indicates missing required lowercase letter
	ErrNoLowercase

	// ErrNoDigit indicates missing required digit
	ErrNoDigit

	// ErrNoSpecial indicates missing required special character
	ErrNoSpecial

	// ErrInHistory indicates password was used recently
	ErrInHistory

	// ErrTooSoon indicates password change attempted before MinAge
	ErrTooSoon
)

// DefaultPolicy returns a sensible default password policy.
func DefaultPolicy() *Policy {
	return &Policy{
		Enabled:           true,
		MinLength:         8,
		MaxLength:         128,
		RequireUppercase:  true,
		RequireLowercase:  true,
		RequireDigit:      true,
		RequireSpecial:    false,
		MaxAge:            90 * 24 * time.Hour, // 90 days
		MinAge:            0,
		HistoryCount:      5,
		MaxFailures:       5,
		LockoutDuration:   15 * time.Minute,
		AllowUserChange:   true,
		MustChangeOnReset: true,
	}
}

// DisabledPolicy returns a policy with all enforcement disabled.
func DisabledPolicy() *Policy {
	return &Policy{
		Enabled:         false,
		MinLength:       0,
		MaxLength:       0,
		AllowUserChange: true,
	}
}

// Validate checks if a password meets the policy requirements.
// Returns nil if the password is valid, or a ValidationError describing the failure.
func (p *Policy) Validate(password string) error {
	if !p.Enabled {
		return nil
	}

	// Check minimum length
	if p.MinLength > 0 && len(password) < p.MinLength {
		return &ValidationError{
			Code:    ErrTooShort,
			Message: "password is too short",
		}
	}

	// Check maximum length
	if p.MaxLength > 0 && len(password) > p.MaxLength {
		return &ValidationError{
			Code:    ErrTooLong,
			Message: "password is too long",
		}
	}

	// Check character requirements
	var hasUpper, hasLower, hasDigit, hasSpecial bool

	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		case isSpecialChar(r):
			hasSpecial = true
		}
	}

	if p.RequireUppercase && !hasUpper {
		return &ValidationError{
			Code:    ErrNoUppercase,
			Message: "password must contain at least one uppercase letter",
		}
	}

	if p.RequireLowercase && !hasLower {
		return &ValidationError{
			Code:    ErrNoLowercase,
			Message: "password must contain at least one lowercase letter",
		}
	}

	if p.RequireDigit && !hasDigit {
		return &ValidationError{
			Code:    ErrNoDigit,
			Message: "password must contain at least one digit",
		}
	}

	if p.RequireSpecial && !hasSpecial {
		return &ValidationError{
			Code:    ErrNoSpecial,
			Message: "password must contain at least one special character",
		}
	}

	return nil
}

// IsExpired checks if a password has expired based on the last change time.
func (p *Policy) IsExpired(lastChanged time.Time) bool {
	if !p.Enabled || p.MaxAge == 0 {
		return false
	}
	return time.Since(lastChanged) > p.MaxAge
}

// CanChange checks if enough time has passed since the last password change.
func (p *Policy) CanChange(lastChanged time.Time) bool {
	if !p.Enabled || p.MinAge == 0 {
		return true
	}
	return time.Since(lastChanged) >= p.MinAge
}

// IsLockedOut checks if an account should be locked based on failure count and time.
func (p *Policy) IsLockedOut(failureCount int, lastFailure time.Time) bool {
	if !p.Enabled || p.MaxFailures == 0 {
		return false
	}

	if failureCount < p.MaxFailures {
		return false
	}

	// Check if lockout duration has passed
	if p.LockoutDuration > 0 && time.Since(lastFailure) > p.LockoutDuration {
		return false
	}

	return true
}

// Clone creates a deep copy of the policy.
func (p *Policy) Clone() *Policy {
	if p == nil {
		return nil
	}
	clone := *p
	return &clone
}

// Merge applies non-zero values from another policy to this one.
// This is useful for applying per-user overrides to a global policy.
// Note: Boolean fields from the override are only applied if they are true,
// since Go cannot distinguish between "not set" and "set to false".
func (p *Policy) Merge(override *Policy) *Policy {
	if override == nil {
		return p.Clone()
	}

	result := p.Clone()

	// Only override if the override policy is enabled
	if override.Enabled {
		result.Enabled = override.Enabled
	}

	if override.MinLength > 0 {
		result.MinLength = override.MinLength
	}

	if override.MaxLength > 0 {
		result.MaxLength = override.MaxLength
	}

	// Boolean fields: only override if the override value is true
	// This allows user policies to add requirements but not remove them
	if override.RequireUppercase {
		result.RequireUppercase = true
	}

	if override.RequireLowercase {
		result.RequireLowercase = true
	}

	if override.RequireDigit {
		result.RequireDigit = true
	}

	if override.RequireSpecial {
		result.RequireSpecial = true
	}

	// AllowUserChange and MustChangeOnReset: only override if explicitly set
	// These are typically set at the global level
	if override.AllowUserChange {
		result.AllowUserChange = true
	}

	if override.MustChangeOnReset {
		result.MustChangeOnReset = true
	}

	if override.MaxAge > 0 {
		result.MaxAge = override.MaxAge
	}

	if override.MinAge > 0 {
		result.MinAge = override.MinAge
	}

	if override.HistoryCount > 0 {
		result.HistoryCount = override.HistoryCount
	}

	if override.MaxFailures > 0 {
		result.MaxFailures = override.MaxFailures
	}

	if override.LockoutDuration > 0 {
		result.LockoutDuration = override.LockoutDuration
	}

	return result
}

// isSpecialChar checks if a rune is a special character.
func isSpecialChar(r rune) bool {
	specialChars := "!@#$%^&*()_+-=[]{}|;':\",./<>?`~"
	return strings.ContainsRune(specialChars, r)
}

// PolicyAttribute represents LDAP password policy attributes stored on entries.
type PolicyAttribute string

const (
	// AttrPwdPolicySubentry is the DN of the applicable password policy
	AttrPwdPolicySubentry PolicyAttribute = "pwdPolicySubentry"

	// AttrPwdChangedTime is the timestamp of the last password change
	AttrPwdChangedTime PolicyAttribute = "pwdChangedTime"

	// AttrPwdAccountLockedTime is the timestamp when the account was locked
	AttrPwdAccountLockedTime PolicyAttribute = "pwdAccountLockedTime"

	// AttrPwdFailureTime contains timestamps of failed authentication attempts
	AttrPwdFailureTime PolicyAttribute = "pwdFailureTime"

	// AttrPwdHistory contains hashes of previous passwords
	AttrPwdHistory PolicyAttribute = "pwdHistory"

	// AttrPwdGraceUseTime contains timestamps of grace login uses
	AttrPwdGraceUseTime PolicyAttribute = "pwdGraceUseTime"

	// AttrPwdReset indicates the password was reset by an administrator
	AttrPwdReset PolicyAttribute = "pwdReset"

	// AttrPwdMustChange indicates the user must change password on next login
	AttrPwdMustChange PolicyAttribute = "pwdMustChange"
)

// String returns the string representation of the policy attribute.
func (a PolicyAttribute) String() string {
	return string(a)
}

// AllPolicyAttributes returns all defined password policy attributes.
func AllPolicyAttributes() []PolicyAttribute {
	return []PolicyAttribute{
		AttrPwdPolicySubentry,
		AttrPwdChangedTime,
		AttrPwdAccountLockedTime,
		AttrPwdFailureTime,
		AttrPwdHistory,
		AttrPwdGraceUseTime,
		AttrPwdReset,
		AttrPwdMustChange,
	}
}
