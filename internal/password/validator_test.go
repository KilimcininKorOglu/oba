package password

import (
	"testing"
)

// TestNewValidator verifies the constructor creates a properly initialized validator.
func TestNewValidator(t *testing.T) {
	// Test with nil policy
	v := NewValidator(nil)
	if v == nil {
		t.Fatal("NewValidator returned nil")
	}

	if !v.IsEnabled() {
		t.Error("default validator should be enabled")
	}

	if v.MinLength() != 8 {
		t.Errorf("expected MinLength 8, got %d", v.MinLength())
	}

	// Test with custom policy
	policy := &Policy{
		Enabled:   true,
		MinLength: 12,
		MaxLength: 64,
	}
	v2 := NewValidator(policy)

	if v2.MinLength() != 12 {
		t.Errorf("expected MinLength 12, got %d", v2.MinLength())
	}

	if v2.MaxLength() != 64 {
		t.Errorf("expected MaxLength 64, got %d", v2.MaxLength())
	}
}

// TestValidatorValidate tests the main validation method.
func TestValidatorValidate(t *testing.T) {
	tests := []struct {
		name     string
		policy   *Policy
		password string
		wantErr  ValidationErrorCode
	}{
		{
			name: "disabled policy accepts anything",
			policy: &Policy{
				Enabled: false,
			},
			password: "",
			wantErr:  0,
		},
		{
			name: "too short",
			policy: &Policy{
				Enabled:   true,
				MinLength: 8,
			},
			password: "short",
			wantErr:  ErrTooShort,
		},
		{
			name: "too long",
			policy: &Policy{
				Enabled:   true,
				MaxLength: 10,
			},
			password: "thispasswordistoolong",
			wantErr:  ErrTooLong,
		},
		{
			name: "missing uppercase",
			policy: &Policy{
				Enabled:          true,
				RequireUppercase: true,
			},
			password: "lowercase123",
			wantErr:  ErrNoUppercase,
		},
		{
			name: "missing lowercase",
			policy: &Policy{
				Enabled:          true,
				RequireLowercase: true,
			},
			password: "UPPERCASE123",
			wantErr:  ErrNoLowercase,
		},
		{
			name: "missing digit",
			policy: &Policy{
				Enabled:      true,
				RequireDigit: true,
			},
			password: "NoDigitsHere",
			wantErr:  ErrNoDigit,
		},
		{
			name: "missing special",
			policy: &Policy{
				Enabled:        true,
				RequireSpecial: true,
			},
			password: "NoSpecial123",
			wantErr:  ErrNoSpecial,
		},
		{
			name: "valid password with all requirements",
			policy: &Policy{
				Enabled:          true,
				MinLength:        8,
				MaxLength:        128,
				RequireUppercase: true,
				RequireLowercase: true,
				RequireDigit:     true,
				RequireSpecial:   true,
			},
			password: "ValidPass123!",
			wantErr:  0,
		},
		{
			name: "valid password minimal requirements",
			policy: &Policy{
				Enabled:   true,
				MinLength: 4,
			},
			password: "test",
			wantErr:  0,
		},
		{
			name: "exact minimum length",
			policy: &Policy{
				Enabled:   true,
				MinLength: 8,
			},
			password: "exactly8",
			wantErr:  0,
		},
		{
			name: "exact maximum length",
			policy: &Policy{
				Enabled:   true,
				MaxLength: 8,
			},
			password: "exactly8",
			wantErr:  0,
		},
		{
			name: "empty password with no requirements",
			policy: &Policy{
				Enabled: true,
			},
			password: "",
			wantErr:  0,
		},
		{
			name: "unicode uppercase",
			policy: &Policy{
				Enabled:          true,
				RequireUppercase: true,
			},
			password: "Ülowercase",
			wantErr:  0,
		},
		{
			name: "unicode lowercase",
			policy: &Policy{
				Enabled:          true,
				RequireLowercase: true,
			},
			password: "UPPERCASEü",
			wantErr:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewValidator(tt.policy)
			err := v.Validate(tt.password)

			if tt.wantErr == 0 {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
				return
			}

			if err == nil {
				t.Errorf("expected error code %d, got nil", tt.wantErr)
				return
			}

			validationErr, ok := err.(*ValidationError)
			if !ok {
				t.Errorf("expected ValidationError, got %T", err)
				return
			}

			if validationErr.Code != tt.wantErr {
				t.Errorf("expected error code %d, got %d", tt.wantErr, validationErr.Code)
			}
		})
	}
}

// TestValidatorValidateWithHistory tests password history checking.
func TestValidatorValidateWithHistory(t *testing.T) {
	// Simple hash function for testing
	hashFunc := func(s string) string {
		return "hash:" + s
	}

	tests := []struct {
		name     string
		policy   *Policy
		password string
		history  []string
		wantErr  ValidationErrorCode
	}{
		{
			name: "password not in history",
			policy: &Policy{
				Enabled:      true,
				HistoryCount: 5,
			},
			password: "newpassword",
			history:  []string{"hash:oldpass1", "hash:oldpass2", "hash:oldpass3"},
			wantErr:  0,
		},
		{
			name: "password in history",
			policy: &Policy{
				Enabled:      true,
				HistoryCount: 5,
			},
			password: "oldpass2",
			history:  []string{"hash:oldpass1", "hash:oldpass2", "hash:oldpass3"},
			wantErr:  ErrInHistory,
		},
		{
			name: "password in history but beyond count",
			policy: &Policy{
				Enabled:      true,
				HistoryCount: 2,
			},
			password: "oldpass3",
			history:  []string{"hash:oldpass1", "hash:oldpass2", "hash:oldpass3"},
			wantErr:  0, // Only checks first 2 entries
		},
		{
			name: "history disabled",
			policy: &Policy{
				Enabled:      true,
				HistoryCount: 0,
			},
			password: "oldpass1",
			history:  []string{"hash:oldpass1"},
			wantErr:  0,
		},
		{
			name: "empty history",
			policy: &Policy{
				Enabled:      true,
				HistoryCount: 5,
			},
			password: "anypassword",
			history:  []string{},
			wantErr:  0,
		},
		{
			name: "nil history",
			policy: &Policy{
				Enabled:      true,
				HistoryCount: 5,
			},
			password: "anypassword",
			history:  nil,
			wantErr:  0,
		},
		{
			name: "basic validation fails before history check",
			policy: &Policy{
				Enabled:   true,
				MinLength: 10,
			},
			password: "short",
			history:  []string{"hash:short"},
			wantErr:  ErrTooShort,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewValidator(tt.policy)
			err := v.ValidateWithHistory(tt.password, tt.history, hashFunc)

			if tt.wantErr == 0 {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
				return
			}

			if err == nil {
				t.Errorf("expected error code %d, got nil", tt.wantErr)
				return
			}

			validationErr, ok := err.(*ValidationError)
			if !ok {
				t.Errorf("expected ValidationError, got %T", err)
				return
			}

			if validationErr.Code != tt.wantErr {
				t.Errorf("expected error code %d, got %d", tt.wantErr, validationErr.Code)
			}
		})
	}
}

// TestValidatorValidateWithHistoryNilHashFunc tests behavior with nil hash function.
func TestValidatorValidateWithHistoryNilHashFunc(t *testing.T) {
	v := NewValidator(&Policy{
		Enabled:      true,
		HistoryCount: 5,
	})

	// Should not panic and should pass validation
	err := v.ValidateWithHistory("password", []string{"hash:password"}, nil)
	if err != nil {
		t.Errorf("expected no error with nil hash func, got %v", err)
	}
}

// TestValidatorCheckLength tests length-only validation.
func TestValidatorCheckLength(t *testing.T) {
	v := NewValidator(&Policy{
		Enabled:   true,
		MinLength: 8,
		MaxLength: 20,
	})

	tests := []struct {
		password string
		wantErr  ValidationErrorCode
	}{
		{"short", ErrTooShort},
		{"exactly8", 0},
		{"validlength", 0},
		{"exactly20characters!", 0},
		{"thispasswordiswaytoolong", ErrTooLong},
	}

	for _, tt := range tests {
		err := v.CheckLength(tt.password)

		if tt.wantErr == 0 {
			if err != nil {
				t.Errorf("CheckLength(%q): expected no error, got %v", tt.password, err)
			}
			continue
		}

		if err == nil {
			t.Errorf("CheckLength(%q): expected error code %d, got nil", tt.password, tt.wantErr)
			continue
		}

		validationErr, ok := err.(*ValidationError)
		if !ok {
			t.Errorf("CheckLength(%q): expected ValidationError, got %T", tt.password, err)
			continue
		}

		if validationErr.Code != tt.wantErr {
			t.Errorf("CheckLength(%q): expected error code %d, got %d", tt.password, tt.wantErr, validationErr.Code)
		}
	}
}

// TestValidatorCheckLengthDisabled tests length check when policy is disabled.
func TestValidatorCheckLengthDisabled(t *testing.T) {
	v := NewValidator(&Policy{
		Enabled:   false,
		MinLength: 100,
	})

	err := v.CheckLength("x")
	if err != nil {
		t.Errorf("expected no error when disabled, got %v", err)
	}
}

// TestValidatorCheckCharacterClasses tests character class validation.
func TestValidatorCheckCharacterClasses(t *testing.T) {
	v := NewValidator(&Policy{
		Enabled:          true,
		RequireUppercase: true,
		RequireLowercase: true,
		RequireDigit:     true,
		RequireSpecial:   true,
	})

	tests := []struct {
		password string
		wantErr  ValidationErrorCode
	}{
		{"lowercase123!", ErrNoUppercase},
		{"UPPERCASE123!", ErrNoLowercase},
		{"NoDigitsHere!", ErrNoDigit},
		{"NoSpecial123", ErrNoSpecial},
		{"ValidPass123!", 0},
	}

	for _, tt := range tests {
		err := v.CheckCharacterClasses(tt.password)

		if tt.wantErr == 0 {
			if err != nil {
				t.Errorf("CheckCharacterClasses(%q): expected no error, got %v", tt.password, err)
			}
			continue
		}

		if err == nil {
			t.Errorf("CheckCharacterClasses(%q): expected error code %d, got nil", tt.password, tt.wantErr)
			continue
		}

		validationErr, ok := err.(*ValidationError)
		if !ok {
			t.Errorf("CheckCharacterClasses(%q): expected ValidationError, got %T", tt.password, err)
			continue
		}

		if validationErr.Code != tt.wantErr {
			t.Errorf("CheckCharacterClasses(%q): expected error code %d, got %d", tt.password, tt.wantErr, validationErr.Code)
		}
	}
}

// TestValidatorCheckCharacterClassesDisabled tests character check when policy is disabled.
func TestValidatorCheckCharacterClassesDisabled(t *testing.T) {
	v := NewValidator(&Policy{
		Enabled:          false,
		RequireUppercase: true,
		RequireLowercase: true,
		RequireDigit:     true,
		RequireSpecial:   true,
	})

	err := v.CheckCharacterClasses("x")
	if err != nil {
		t.Errorf("expected no error when disabled, got %v", err)
	}
}

// TestValidatorGetAllErrors tests collecting all validation errors.
func TestValidatorGetAllErrors(t *testing.T) {
	v := NewValidator(&Policy{
		Enabled:          true,
		MinLength:        10,
		MaxLength:        5, // Intentionally conflicting for testing
		RequireUppercase: true,
		RequireLowercase: true,
		RequireDigit:     true,
		RequireSpecial:   true,
	})

	// Password that fails multiple checks
	errors := v.GetAllErrors("x")

	if len(errors) == 0 {
		t.Error("expected multiple errors, got none")
	}

	// Check that we got expected errors
	hasShort := false
	hasNoUpper := false
	hasNoDigit := false
	hasNoSpecial := false

	for _, e := range errors {
		switch e.Code {
		case ErrTooShort:
			hasShort = true
		case ErrNoUppercase:
			hasNoUpper = true
		case ErrNoDigit:
			hasNoDigit = true
		case ErrNoSpecial:
			hasNoSpecial = true
		}
	}

	if !hasShort {
		t.Error("expected ErrTooShort in errors")
	}

	if !hasNoUpper {
		t.Error("expected ErrNoUppercase in errors")
	}

	if !hasNoDigit {
		t.Error("expected ErrNoDigit in errors")
	}

	if !hasNoSpecial {
		t.Error("expected ErrNoSpecial in errors")
	}

	// "x" is lowercase, so should not have ErrNoLowercase
	for _, e := range errors {
		if e.Code == ErrNoLowercase {
			t.Error("should not have ErrNoLowercase for 'x'")
		}
	}
}

// TestValidatorGetAllErrorsDisabled tests GetAllErrors when policy is disabled.
func TestValidatorGetAllErrorsDisabled(t *testing.T) {
	v := NewValidator(&Policy{
		Enabled:          false,
		MinLength:        100,
		RequireUppercase: true,
	})

	errors := v.GetAllErrors("x")
	if errors != nil {
		t.Errorf("expected nil errors when disabled, got %v", errors)
	}
}

// TestValidatorGetAllErrorsValidPassword tests GetAllErrors with valid password.
func TestValidatorGetAllErrorsValidPassword(t *testing.T) {
	v := NewValidator(&Policy{
		Enabled:          true,
		MinLength:        8,
		RequireUppercase: true,
		RequireLowercase: true,
		RequireDigit:     true,
		RequireSpecial:   true,
	})

	errors := v.GetAllErrors("ValidPass123!")
	if len(errors) != 0 {
		t.Errorf("expected no errors for valid password, got %v", errors)
	}
}

// TestValidatorPolicy tests getting and setting policy.
func TestValidatorPolicy(t *testing.T) {
	original := &Policy{
		Enabled:   true,
		MinLength: 10,
	}

	v := NewValidator(original)

	// Get policy
	policy := v.Policy()
	if policy.MinLength != 10 {
		t.Errorf("expected MinLength 10, got %d", policy.MinLength)
	}

	// Modify returned policy should not affect validator
	policy.MinLength = 20
	if v.MinLength() != 10 {
		t.Error("modifying returned policy affected validator")
	}

	// Set new policy
	v.SetPolicy(&Policy{
		Enabled:   true,
		MinLength: 15,
	})

	if v.MinLength() != 15 {
		t.Errorf("expected MinLength 15 after SetPolicy, got %d", v.MinLength())
	}

	// Set nil policy should use default
	v.SetPolicy(nil)
	if v.MinLength() != 8 {
		t.Errorf("expected default MinLength 8 after SetPolicy(nil), got %d", v.MinLength())
	}
}

// TestValidatorGetters tests all getter methods.
func TestValidatorGetters(t *testing.T) {
	policy := &Policy{
		Enabled:          true,
		MinLength:        10,
		MaxLength:        50,
		RequireUppercase: true,
		RequireLowercase: true,
		RequireDigit:     true,
		RequireSpecial:   true,
		HistoryCount:     5,
	}

	v := NewValidator(policy)

	if !v.IsEnabled() {
		t.Error("expected IsEnabled true")
	}

	if v.MinLength() != 10 {
		t.Errorf("expected MinLength 10, got %d", v.MinLength())
	}

	if v.MaxLength() != 50 {
		t.Errorf("expected MaxLength 50, got %d", v.MaxLength())
	}

	if !v.RequiresUppercase() {
		t.Error("expected RequiresUppercase true")
	}

	if !v.RequiresLowercase() {
		t.Error("expected RequiresLowercase true")
	}

	if !v.RequiresDigit() {
		t.Error("expected RequiresDigit true")
	}

	if !v.RequiresSpecial() {
		t.Error("expected RequiresSpecial true")
	}

	if v.HistoryCount() != 5 {
		t.Errorf("expected HistoryCount 5, got %d", v.HistoryCount())
	}
}

// TestValidatorSpecialCharacters tests various special characters.
func TestValidatorSpecialCharacters(t *testing.T) {
	v := NewValidator(&Policy{
		Enabled:        true,
		RequireSpecial: true,
	})

	validPasswords := []string{
		"Password!",
		"Password@",
		"Password#",
		"Password$",
		"Password%",
		"Password^",
		"Password&",
		"Password*",
		"Password(",
		"Password)",
		"Password_",
		"Password+",
		"Password-",
		"Password=",
		"Password[",
		"Password]",
		"Password{",
		"Password}",
		"Password|",
		"Password;",
		"Password'",
		"Password:",
		"Password\"",
		"Password,",
		"Password.",
		"Password/",
		"Password<",
		"Password>",
		"Password?",
		"Password`",
		"Password~",
	}

	for _, pwd := range validPasswords {
		err := v.Validate(pwd)
		if err != nil {
			t.Errorf("password %q should be valid: %v", pwd, err)
		}
	}

	invalidPasswords := []string{
		"Password123",
		"NoSpecialHere",
		"JustLetters",
		"12345678",
	}

	for _, pwd := range invalidPasswords {
		err := v.Validate(pwd)
		if err == nil {
			t.Errorf("password %q should be invalid (no special char)", pwd)
		}
	}
}

// TestValidatorErrorMessages tests that error messages are clear.
func TestValidatorErrorMessages(t *testing.T) {
	tests := []struct {
		policy      *Policy
		password    string
		wantMessage string
	}{
		{
			policy:      &Policy{Enabled: true, MinLength: 10},
			password:    "short",
			wantMessage: "password is too short",
		},
		{
			policy:      &Policy{Enabled: true, MaxLength: 5},
			password:    "toolongpassword",
			wantMessage: "password is too long",
		},
		{
			policy:      &Policy{Enabled: true, RequireUppercase: true},
			password:    "lowercase",
			wantMessage: "password must contain at least one uppercase letter",
		},
		{
			policy:      &Policy{Enabled: true, RequireLowercase: true},
			password:    "UPPERCASE",
			wantMessage: "password must contain at least one lowercase letter",
		},
		{
			policy:      &Policy{Enabled: true, RequireDigit: true},
			password:    "nodigits",
			wantMessage: "password must contain at least one digit",
		},
		{
			policy:      &Policy{Enabled: true, RequireSpecial: true},
			password:    "nospecial123",
			wantMessage: "password must contain at least one special character",
		},
	}

	for _, tt := range tests {
		v := NewValidator(tt.policy)
		err := v.Validate(tt.password)

		if err == nil {
			t.Errorf("expected error for password %q", tt.password)
			continue
		}

		if err.Error() != tt.wantMessage {
			t.Errorf("expected message %q, got %q", tt.wantMessage, err.Error())
		}
	}
}

// TestValidatorHistoryErrorMessage tests the history error message.
func TestValidatorHistoryErrorMessage(t *testing.T) {
	v := NewValidator(&Policy{
		Enabled:      true,
		HistoryCount: 5,
	})

	hashFunc := func(s string) string { return "hash:" + s }
	history := []string{"hash:oldpassword"}

	err := v.ValidateWithHistory("oldpassword", history, hashFunc)
	if err == nil {
		t.Fatal("expected error for password in history")
	}

	if err.Error() != "password was used recently" {
		t.Errorf("expected message 'password was used recently', got %q", err.Error())
	}
}

// TestValidatorPolicyIsolation tests that validator policy is isolated from original.
func TestValidatorPolicyIsolation(t *testing.T) {
	original := &Policy{
		Enabled:   true,
		MinLength: 8,
	}

	v := NewValidator(original)

	// Modify original
	original.MinLength = 20

	// Validator should not be affected
	if v.MinLength() != 8 {
		t.Error("validator should be isolated from original policy changes")
	}
}

// TestValidatorZeroMaxLength tests that zero MaxLength means unlimited.
func TestValidatorZeroMaxLength(t *testing.T) {
	v := NewValidator(&Policy{
		Enabled:   true,
		MaxLength: 0, // Unlimited
	})

	// Very long password should be valid
	longPassword := ""
	for i := 0; i < 1000; i++ {
		longPassword += "a"
	}

	err := v.Validate(longPassword)
	if err != nil {
		t.Errorf("expected no error for long password with unlimited max, got %v", err)
	}
}

// TestValidatorZeroMinLength tests that zero MinLength means no minimum.
func TestValidatorZeroMinLength(t *testing.T) {
	v := NewValidator(&Policy{
		Enabled:   true,
		MinLength: 0, // No minimum
	})

	err := v.Validate("")
	if err != nil {
		t.Errorf("expected no error for empty password with no minimum, got %v", err)
	}
}

// TestValidatorValidationOrder tests that validation checks happen in expected order.
func TestValidatorValidationOrder(t *testing.T) {
	v := NewValidator(&Policy{
		Enabled:          true,
		MinLength:        10,
		RequireUppercase: true,
	})

	// Password is too short AND missing uppercase
	// Should fail on length first
	err := v.Validate("short")
	if err == nil {
		t.Fatal("expected error")
	}

	validationErr := err.(*ValidationError)
	if validationErr.Code != ErrTooShort {
		t.Errorf("expected ErrTooShort first, got %d", validationErr.Code)
	}
}

// TestValidatorEmptyPassword tests validation of empty password.
func TestValidatorEmptyPassword(t *testing.T) {
	// With requirements
	v := NewValidator(&Policy{
		Enabled:          true,
		MinLength:        8,
		RequireUppercase: true,
	})

	err := v.Validate("")
	if err == nil {
		t.Error("expected error for empty password with requirements")
	}

	// Without requirements
	v2 := NewValidator(&Policy{
		Enabled: true,
	})

	err = v2.Validate("")
	if err != nil {
		t.Errorf("expected no error for empty password without requirements, got %v", err)
	}
}

// TestValidatorWhitespacePassword tests passwords with whitespace.
func TestValidatorWhitespacePassword(t *testing.T) {
	v := NewValidator(&Policy{
		Enabled:   true,
		MinLength: 8,
	})

	// Whitespace counts toward length
	err := v.Validate("        ") // 8 spaces
	if err != nil {
		t.Errorf("expected no error for 8 spaces, got %v", err)
	}

	err = v.Validate("pass word") // 9 chars with space
	if err != nil {
		t.Errorf("expected no error for 'pass word', got %v", err)
	}
}

// TestValidatorNumericPassword tests all-numeric passwords.
func TestValidatorNumericPassword(t *testing.T) {
	v := NewValidator(&Policy{
		Enabled:      true,
		MinLength:    8,
		RequireDigit: true,
	})

	err := v.Validate("12345678")
	if err != nil {
		t.Errorf("expected no error for numeric password, got %v", err)
	}
}

// TestValidatorMixedRequirements tests various combinations of requirements.
func TestValidatorMixedRequirements(t *testing.T) {
	tests := []struct {
		name     string
		policy   *Policy
		password string
		valid    bool
	}{
		{
			name: "upper and digit only",
			policy: &Policy{
				Enabled:          true,
				RequireUppercase: true,
				RequireDigit:     true,
			},
			password: "UPPER123",
			valid:    true,
		},
		{
			name: "lower and special only",
			policy: &Policy{
				Enabled:          true,
				RequireLowercase: true,
				RequireSpecial:   true,
			},
			password: "lower!",
			valid:    true,
		},
		{
			name: "all four classes",
			policy: &Policy{
				Enabled:          true,
				RequireUppercase: true,
				RequireLowercase: true,
				RequireDigit:     true,
				RequireSpecial:   true,
			},
			password: "Aa1!",
			valid:    true,
		},
		{
			name: "length and upper",
			policy: &Policy{
				Enabled:          true,
				MinLength:        8,
				RequireUppercase: true,
			},
			password: "Longpassword",
			valid:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewValidator(tt.policy)
			err := v.Validate(tt.password)

			if tt.valid && err != nil {
				t.Errorf("expected valid, got error: %v", err)
			}

			if !tt.valid && err == nil {
				t.Error("expected invalid, got no error")
			}
		})
	}
}
