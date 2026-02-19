// Package password provides password policy configuration and validation
// for the Oba LDAP server.
package password

import (
	"strings"
	"unicode"
)

// Validator provides password complexity validation against a policy.
// It wraps a Policy and provides additional validation capabilities
// including password history checking.
type Validator struct {
	policy *Policy
}

// NewValidator creates a new password validator with the given policy.
// If policy is nil, a default policy is used.
func NewValidator(policy *Policy) *Validator {
	if policy == nil {
		policy = DefaultPolicy()
	}
	return &Validator{
		policy: policy.Clone(),
	}
}

// Validate checks if a password meets all policy requirements.
// Returns nil if the password is valid, or a ValidationError describing the failure.
func (v *Validator) Validate(password string) error {
	if !v.policy.Enabled {
		return nil
	}

	// Check minimum length
	if v.policy.MinLength > 0 && len(password) < v.policy.MinLength {
		return &ValidationError{
			Code:    ErrTooShort,
			Message: "password is too short",
		}
	}

	// Check maximum length
	if v.policy.MaxLength > 0 && len(password) > v.policy.MaxLength {
		return &ValidationError{
			Code:    ErrTooLong,
			Message: "password is too long",
		}
	}

	// Check character class requirements
	var hasUpper, hasLower, hasDigit, hasSpecial bool

	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		case isSpecialCharacter(r):
			hasSpecial = true
		}
	}

	if v.policy.RequireUppercase && !hasUpper {
		return &ValidationError{
			Code:    ErrNoUppercase,
			Message: "password must contain at least one uppercase letter",
		}
	}

	if v.policy.RequireLowercase && !hasLower {
		return &ValidationError{
			Code:    ErrNoLowercase,
			Message: "password must contain at least one lowercase letter",
		}
	}

	if v.policy.RequireDigit && !hasDigit {
		return &ValidationError{
			Code:    ErrNoDigit,
			Message: "password must contain at least one digit",
		}
	}

	if v.policy.RequireSpecial && !hasSpecial {
		return &ValidationError{
			Code:    ErrNoSpecial,
			Message: "password must contain at least one special character",
		}
	}

	return nil
}

// ValidateWithHistory checks if a password meets policy requirements
// and is not in the provided password history.
// The history parameter should contain hashed passwords from previous uses.
// The hashFunc parameter is used to hash the new password for comparison.
func (v *Validator) ValidateWithHistory(password string, history []string, hashFunc func(string) string) error {
	// First validate against basic policy requirements
	if err := v.Validate(password); err != nil {
		return err
	}

	// Check password history if enabled
	if v.policy.HistoryCount > 0 && len(history) > 0 && hashFunc != nil {
		hashedPassword := hashFunc(password)

		// Only check up to HistoryCount entries
		checkCount := len(history)
		if checkCount > v.policy.HistoryCount {
			checkCount = v.policy.HistoryCount
		}

		for i := 0; i < checkCount; i++ {
			if history[i] == hashedPassword {
				return &ValidationError{
					Code:    ErrInHistory,
					Message: "password was used recently",
				}
			}
		}
	}

	return nil
}

// Policy returns a copy of the validator's policy.
func (v *Validator) Policy() *Policy {
	return v.policy.Clone()
}

// SetPolicy updates the validator's policy.
// If policy is nil, a default policy is used.
func (v *Validator) SetPolicy(policy *Policy) {
	if policy == nil {
		v.policy = DefaultPolicy()
		return
	}
	v.policy = policy.Clone()
}

// IsEnabled returns whether password validation is enabled.
func (v *Validator) IsEnabled() bool {
	return v.policy.Enabled
}

// MinLength returns the minimum required password length.
func (v *Validator) MinLength() int {
	return v.policy.MinLength
}

// MaxLength returns the maximum allowed password length.
func (v *Validator) MaxLength() int {
	return v.policy.MaxLength
}

// RequiresUppercase returns whether uppercase letters are required.
func (v *Validator) RequiresUppercase() bool {
	return v.policy.RequireUppercase
}

// RequiresLowercase returns whether lowercase letters are required.
func (v *Validator) RequiresLowercase() bool {
	return v.policy.RequireLowercase
}

// RequiresDigit returns whether digits are required.
func (v *Validator) RequiresDigit() bool {
	return v.policy.RequireDigit
}

// RequiresSpecial returns whether special characters are required.
func (v *Validator) RequiresSpecial() bool {
	return v.policy.RequireSpecial
}

// HistoryCount returns the number of previous passwords to check.
func (v *Validator) HistoryCount() int {
	return v.policy.HistoryCount
}

// CheckLength validates only the password length requirements.
// Returns nil if length is valid, or a ValidationError if not.
func (v *Validator) CheckLength(password string) error {
	if !v.policy.Enabled {
		return nil
	}

	if v.policy.MinLength > 0 && len(password) < v.policy.MinLength {
		return &ValidationError{
			Code:    ErrTooShort,
			Message: "password is too short",
		}
	}

	if v.policy.MaxLength > 0 && len(password) > v.policy.MaxLength {
		return &ValidationError{
			Code:    ErrTooLong,
			Message: "password is too long",
		}
	}

	return nil
}

// CheckCharacterClasses validates only the character class requirements.
// Returns nil if all required character classes are present, or a ValidationError if not.
func (v *Validator) CheckCharacterClasses(password string) error {
	if !v.policy.Enabled {
		return nil
	}

	var hasUpper, hasLower, hasDigit, hasSpecial bool

	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		case isSpecialCharacter(r):
			hasSpecial = true
		}
	}

	if v.policy.RequireUppercase && !hasUpper {
		return &ValidationError{
			Code:    ErrNoUppercase,
			Message: "password must contain at least one uppercase letter",
		}
	}

	if v.policy.RequireLowercase && !hasLower {
		return &ValidationError{
			Code:    ErrNoLowercase,
			Message: "password must contain at least one lowercase letter",
		}
	}

	if v.policy.RequireDigit && !hasDigit {
		return &ValidationError{
			Code:    ErrNoDigit,
			Message: "password must contain at least one digit",
		}
	}

	if v.policy.RequireSpecial && !hasSpecial {
		return &ValidationError{
			Code:    ErrNoSpecial,
			Message: "password must contain at least one special character",
		}
	}

	return nil
}

// GetAllErrors returns all validation errors for a password.
// Unlike Validate which returns on the first error, this method
// collects all validation failures for comprehensive feedback.
func (v *Validator) GetAllErrors(password string) []ValidationError {
	if !v.policy.Enabled {
		return nil
	}

	var errors []ValidationError

	// Check length
	if v.policy.MinLength > 0 && len(password) < v.policy.MinLength {
		errors = append(errors, ValidationError{
			Code:    ErrTooShort,
			Message: "password is too short",
		})
	}

	if v.policy.MaxLength > 0 && len(password) > v.policy.MaxLength {
		errors = append(errors, ValidationError{
			Code:    ErrTooLong,
			Message: "password is too long",
		})
	}

	// Check character classes
	var hasUpper, hasLower, hasDigit, hasSpecial bool

	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		case isSpecialCharacter(r):
			hasSpecial = true
		}
	}

	if v.policy.RequireUppercase && !hasUpper {
		errors = append(errors, ValidationError{
			Code:    ErrNoUppercase,
			Message: "password must contain at least one uppercase letter",
		})
	}

	if v.policy.RequireLowercase && !hasLower {
		errors = append(errors, ValidationError{
			Code:    ErrNoLowercase,
			Message: "password must contain at least one lowercase letter",
		})
	}

	if v.policy.RequireDigit && !hasDigit {
		errors = append(errors, ValidationError{
			Code:    ErrNoDigit,
			Message: "password must contain at least one digit",
		})
	}

	if v.policy.RequireSpecial && !hasSpecial {
		errors = append(errors, ValidationError{
			Code:    ErrNoSpecial,
			Message: "password must contain at least one special character",
		})
	}

	return errors
}

// isSpecialCharacter checks if a rune is a special character.
func isSpecialCharacter(r rune) bool {
	specialChars := "!@#$%^&*()_+-=[]{}|;':\",./<>?`~"
	return strings.ContainsRune(specialChars, r)
}
