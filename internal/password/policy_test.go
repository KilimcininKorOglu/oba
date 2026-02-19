package password

import (
	"testing"
	"time"
)

// TestDefaultPolicy verifies the default policy has sensible values.
func TestDefaultPolicy(t *testing.T) {
	p := DefaultPolicy()

	if !p.Enabled {
		t.Error("default policy should be enabled")
	}

	if p.MinLength != 8 {
		t.Errorf("expected MinLength 8, got %d", p.MinLength)
	}

	if p.MaxLength != 128 {
		t.Errorf("expected MaxLength 128, got %d", p.MaxLength)
	}

	if !p.RequireUppercase {
		t.Error("default policy should require uppercase")
	}

	if !p.RequireLowercase {
		t.Error("default policy should require lowercase")
	}

	if !p.RequireDigit {
		t.Error("default policy should require digit")
	}

	if p.RequireSpecial {
		t.Error("default policy should not require special by default")
	}

	if p.MaxAge != 90*24*time.Hour {
		t.Errorf("expected MaxAge 90 days, got %v", p.MaxAge)
	}

	if p.HistoryCount != 5 {
		t.Errorf("expected HistoryCount 5, got %d", p.HistoryCount)
	}

	if p.MaxFailures != 5 {
		t.Errorf("expected MaxFailures 5, got %d", p.MaxFailures)
	}

	if p.LockoutDuration != 15*time.Minute {
		t.Errorf("expected LockoutDuration 15m, got %v", p.LockoutDuration)
	}

	if !p.AllowUserChange {
		t.Error("default policy should allow user change")
	}

	if !p.MustChangeOnReset {
		t.Error("default policy should require change on reset")
	}
}

// TestDisabledPolicy verifies the disabled policy.
func TestDisabledPolicy(t *testing.T) {
	p := DisabledPolicy()

	if p.Enabled {
		t.Error("disabled policy should not be enabled")
	}

	if p.MinLength != 0 {
		t.Errorf("expected MinLength 0, got %d", p.MinLength)
	}

	if !p.AllowUserChange {
		t.Error("disabled policy should allow user change")
	}
}

// TestPolicyValidate tests password validation.
func TestPolicyValidate(t *testing.T) {
	tests := []struct {
		name     string
		policy   *Policy
		password string
		wantErr  ValidationErrorCode
	}{
		{
			name:     "disabled policy accepts anything",
			policy:   DisabledPolicy(),
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.policy.Validate(tt.password)

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

// TestPolicyIsExpired tests password expiration checking.
func TestPolicyIsExpired(t *testing.T) {
	tests := []struct {
		name        string
		policy      *Policy
		lastChanged time.Time
		want        bool
	}{
		{
			name:        "disabled policy never expires",
			policy:      DisabledPolicy(),
			lastChanged: time.Now().Add(-365 * 24 * time.Hour),
			want:        false,
		},
		{
			name: "zero MaxAge never expires",
			policy: &Policy{
				Enabled: true,
				MaxAge:  0,
			},
			lastChanged: time.Now().Add(-365 * 24 * time.Hour),
			want:        false,
		},
		{
			name: "not expired",
			policy: &Policy{
				Enabled: true,
				MaxAge:  90 * 24 * time.Hour,
			},
			lastChanged: time.Now().Add(-30 * 24 * time.Hour),
			want:        false,
		},
		{
			name: "expired",
			policy: &Policy{
				Enabled: true,
				MaxAge:  90 * 24 * time.Hour,
			},
			lastChanged: time.Now().Add(-100 * 24 * time.Hour),
			want:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.policy.IsExpired(tt.lastChanged)
			if got != tt.want {
				t.Errorf("IsExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestPolicyCanChange tests minimum age checking.
func TestPolicyCanChange(t *testing.T) {
	tests := []struct {
		name        string
		policy      *Policy
		lastChanged time.Time
		want        bool
	}{
		{
			name:        "disabled policy always allows change",
			policy:      DisabledPolicy(),
			lastChanged: time.Now(),
			want:        true,
		},
		{
			name: "zero MinAge always allows change",
			policy: &Policy{
				Enabled: true,
				MinAge:  0,
			},
			lastChanged: time.Now(),
			want:        true,
		},
		{
			name: "can change after MinAge",
			policy: &Policy{
				Enabled: true,
				MinAge:  24 * time.Hour,
			},
			lastChanged: time.Now().Add(-48 * time.Hour),
			want:        true,
		},
		{
			name: "cannot change before MinAge",
			policy: &Policy{
				Enabled: true,
				MinAge:  24 * time.Hour,
			},
			lastChanged: time.Now().Add(-1 * time.Hour),
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.policy.CanChange(tt.lastChanged)
			if got != tt.want {
				t.Errorf("CanChange() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestPolicyIsLockedOut tests account lockout checking.
func TestPolicyIsLockedOut(t *testing.T) {
	tests := []struct {
		name         string
		policy       *Policy
		failureCount int
		lastFailure  time.Time
		want         bool
	}{
		{
			name:         "disabled policy never locks",
			policy:       DisabledPolicy(),
			failureCount: 100,
			lastFailure:  time.Now(),
			want:         false,
		},
		{
			name: "zero MaxFailures never locks",
			policy: &Policy{
				Enabled:     true,
				MaxFailures: 0,
			},
			failureCount: 100,
			lastFailure:  time.Now(),
			want:         false,
		},
		{
			name: "not locked under threshold",
			policy: &Policy{
				Enabled:     true,
				MaxFailures: 5,
			},
			failureCount: 3,
			lastFailure:  time.Now(),
			want:         false,
		},
		{
			name: "locked at threshold",
			policy: &Policy{
				Enabled:         true,
				MaxFailures:     5,
				LockoutDuration: 15 * time.Minute,
			},
			failureCount: 5,
			lastFailure:  time.Now(),
			want:         true,
		},
		{
			name: "locked above threshold",
			policy: &Policy{
				Enabled:         true,
				MaxFailures:     5,
				LockoutDuration: 15 * time.Minute,
			},
			failureCount: 10,
			lastFailure:  time.Now(),
			want:         true,
		},
		{
			name: "unlocked after duration",
			policy: &Policy{
				Enabled:         true,
				MaxFailures:     5,
				LockoutDuration: 15 * time.Minute,
			},
			failureCount: 10,
			lastFailure:  time.Now().Add(-30 * time.Minute),
			want:         false,
		},
		{
			name: "permanent lockout with zero duration",
			policy: &Policy{
				Enabled:         true,
				MaxFailures:     5,
				LockoutDuration: 0,
			},
			failureCount: 5,
			lastFailure:  time.Now().Add(-24 * time.Hour),
			want:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.policy.IsLockedOut(tt.failureCount, tt.lastFailure)
			if got != tt.want {
				t.Errorf("IsLockedOut() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestPolicyClone tests policy cloning.
func TestPolicyClone(t *testing.T) {
	original := DefaultPolicy()
	clone := original.Clone()

	// Verify values are equal
	if clone.MinLength != original.MinLength {
		t.Error("clone MinLength mismatch")
	}

	// Modify clone and verify original is unchanged
	clone.MinLength = 100
	if original.MinLength == 100 {
		t.Error("modifying clone affected original")
	}

	// Test nil clone
	var nilPolicy *Policy
	if nilPolicy.Clone() != nil {
		t.Error("nil clone should return nil")
	}
}

// TestPolicyMerge tests policy merging.
func TestPolicyMerge(t *testing.T) {
	base := &Policy{
		Enabled:          true,
		MinLength:        8,
		MaxLength:        128,
		RequireUppercase: true,
		RequireLowercase: true,
		RequireDigit:     true,
		RequireSpecial:   false,
		MaxAge:           90 * 24 * time.Hour,
		HistoryCount:     5,
		MaxFailures:      5,
		LockoutDuration:  15 * time.Minute,
		AllowUserChange:  true,
	}

	override := &Policy{
		Enabled:        true,
		MinLength:      12,
		RequireSpecial: true,
		MaxFailures:    3,
	}

	merged := base.Merge(override)

	// Check overridden values
	if merged.MinLength != 12 {
		t.Errorf("expected MinLength 12, got %d", merged.MinLength)
	}

	if !merged.RequireSpecial {
		t.Error("expected RequireSpecial true")
	}

	if merged.MaxFailures != 3 {
		t.Errorf("expected MaxFailures 3, got %d", merged.MaxFailures)
	}

	// Check preserved values
	if merged.MaxLength != 128 {
		t.Errorf("expected MaxLength 128, got %d", merged.MaxLength)
	}

	if merged.HistoryCount != 5 {
		t.Errorf("expected HistoryCount 5, got %d", merged.HistoryCount)
	}

	// Test merge with nil
	mergedNil := base.Merge(nil)
	if mergedNil.MinLength != base.MinLength {
		t.Error("merge with nil should return clone of base")
	}
}

// TestValidationError tests the ValidationError type.
func TestValidationError(t *testing.T) {
	err := &ValidationError{
		Code:    ErrTooShort,
		Message: "password is too short",
	}

	if err.Error() != "password is too short" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

// TestPolicyAttributes tests policy attribute constants.
func TestPolicyAttributes(t *testing.T) {
	attrs := AllPolicyAttributes()

	if len(attrs) != 8 {
		t.Errorf("expected 8 policy attributes, got %d", len(attrs))
	}

	// Verify string conversion
	if AttrPwdChangedTime.String() != "pwdChangedTime" {
		t.Errorf("unexpected attribute string: %s", AttrPwdChangedTime.String())
	}
}

// TestNewManager tests manager creation.
func TestNewManager(t *testing.T) {
	// Test with nil policy
	m := NewManager(nil)
	if m == nil {
		t.Fatal("NewManager returned nil")
	}

	policy := m.GetGlobalPolicy()
	if !policy.Enabled {
		t.Error("default global policy should be enabled")
	}

	// Test with custom policy
	custom := &Policy{
		Enabled:   true,
		MinLength: 12,
	}
	m2 := NewManager(custom)
	policy2 := m2.GetGlobalPolicy()
	if policy2.MinLength != 12 {
		t.Errorf("expected MinLength 12, got %d", policy2.MinLength)
	}
}

// TestManagerGetPolicy tests getting effective policy.
func TestManagerGetPolicy(t *testing.T) {
	global := &Policy{
		Enabled:          true,
		MinLength:        8,
		RequireUppercase: true,
		RequireLowercase: true,
	}

	m := NewManager(global)

	// Test without user policy
	dn := "uid=alice,ou=users,dc=example,dc=com"
	policy := m.GetPolicy(dn)

	if policy.MinLength != 8 {
		t.Errorf("expected MinLength 8, got %d", policy.MinLength)
	}

	// Set user policy
	userPolicy := &Policy{
		Enabled:   true,
		MinLength: 12,
	}
	m.SetUserPolicy(dn, userPolicy)

	// Test with user policy
	policy = m.GetPolicy(dn)
	if policy.MinLength != 12 {
		t.Errorf("expected MinLength 12, got %d", policy.MinLength)
	}

	// Verify global values are preserved
	if !policy.RequireUppercase {
		t.Error("expected RequireUppercase from global policy")
	}
}

// TestManagerSetUserPolicy tests setting user policies.
func TestManagerSetUserPolicy(t *testing.T) {
	m := NewManager(nil)
	dn := "uid=bob,ou=users,dc=example,dc=com"

	// Set policy
	policy := &Policy{
		Enabled:   true,
		MinLength: 16,
	}
	m.SetUserPolicy(dn, policy)

	if !m.HasUserPolicy(dn) {
		t.Error("expected user policy to exist")
	}

	// Remove policy
	m.SetUserPolicy(dn, nil)

	if m.HasUserPolicy(dn) {
		t.Error("expected user policy to be removed")
	}
}

// TestManagerGetUserPolicy tests getting user-specific policy.
func TestManagerGetUserPolicy(t *testing.T) {
	m := NewManager(nil)
	dn := "uid=charlie,ou=users,dc=example,dc=com"

	// No user policy
	if m.GetUserPolicy(dn) != nil {
		t.Error("expected nil for non-existent user policy")
	}

	// Set and get
	policy := &Policy{
		Enabled:   true,
		MinLength: 20,
	}
	m.SetUserPolicy(dn, policy)

	retrieved := m.GetUserPolicy(dn)
	if retrieved == nil {
		t.Fatal("expected user policy")
	}

	if retrieved.MinLength != 20 {
		t.Errorf("expected MinLength 20, got %d", retrieved.MinLength)
	}
}

// TestManagerRemoveUserPolicy tests removing user policies.
func TestManagerRemoveUserPolicy(t *testing.T) {
	m := NewManager(nil)
	dn := "uid=dave,ou=users,dc=example,dc=com"

	policy := &Policy{Enabled: true, MinLength: 10}
	m.SetUserPolicy(dn, policy)

	if !m.HasUserPolicy(dn) {
		t.Error("expected user policy to exist")
	}

	m.RemoveUserPolicy(dn)

	if m.HasUserPolicy(dn) {
		t.Error("expected user policy to be removed")
	}
}

// TestManagerSetGlobalPolicy tests updating global policy.
func TestManagerSetGlobalPolicy(t *testing.T) {
	m := NewManager(nil)

	newPolicy := &Policy{
		Enabled:   true,
		MinLength: 15,
	}
	m.SetGlobalPolicy(newPolicy)

	global := m.GetGlobalPolicy()
	if global.MinLength != 15 {
		t.Errorf("expected MinLength 15, got %d", global.MinLength)
	}

	// Test with nil
	m.SetGlobalPolicy(nil)
	global = m.GetGlobalPolicy()
	if global.MinLength != 8 {
		t.Errorf("expected default MinLength 8, got %d", global.MinLength)
	}
}

// TestManagerListUserPolicies tests listing user policies.
func TestManagerListUserPolicies(t *testing.T) {
	m := NewManager(nil)

	dns := []string{
		"uid=user1,ou=users,dc=example,dc=com",
		"uid=user2,ou=users,dc=example,dc=com",
		"uid=user3,ou=users,dc=example,dc=com",
	}

	for _, dn := range dns {
		m.SetUserPolicy(dn, &Policy{Enabled: true, MinLength: 10})
	}

	list := m.ListUserPolicies()
	if len(list) != 3 {
		t.Errorf("expected 3 user policies, got %d", len(list))
	}
}

// TestManagerUserPolicyCount tests counting user policies.
func TestManagerUserPolicyCount(t *testing.T) {
	m := NewManager(nil)

	if m.UserPolicyCount() != 0 {
		t.Error("expected 0 user policies initially")
	}

	m.SetUserPolicy("uid=test,dc=example,dc=com", &Policy{Enabled: true})

	if m.UserPolicyCount() != 1 {
		t.Errorf("expected 1 user policy, got %d", m.UserPolicyCount())
	}
}

// TestManagerValidatePassword tests password validation through manager.
func TestManagerValidatePassword(t *testing.T) {
	m := NewManager(&Policy{
		Enabled:   true,
		MinLength: 8,
	})

	dn := "uid=test,dc=example,dc=com"

	// Valid password
	err := m.ValidatePassword(dn, "validpassword")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Invalid password
	err = m.ValidatePassword(dn, "short")
	if err == nil {
		t.Error("expected error for short password")
	}
}

// TestManagerClearUserPolicies tests clearing all user policies.
func TestManagerClearUserPolicies(t *testing.T) {
	m := NewManager(nil)

	for i := 0; i < 5; i++ {
		m.SetUserPolicy("uid=user"+string(rune('0'+i))+",dc=example,dc=com", &Policy{Enabled: true})
	}

	if m.UserPolicyCount() != 5 {
		t.Errorf("expected 5 user policies, got %d", m.UserPolicyCount())
	}

	m.ClearUserPolicies()

	if m.UserPolicyCount() != 0 {
		t.Errorf("expected 0 user policies after clear, got %d", m.UserPolicyCount())
	}
}

// TestManagerDNCaseInsensitivity tests that DN lookups are case-insensitive.
func TestManagerDNCaseInsensitivity(t *testing.T) {
	m := NewManager(nil)

	dn := "uid=Alice,ou=Users,dc=Example,dc=COM"
	policy := &Policy{Enabled: true, MinLength: 15}

	m.SetUserPolicy(dn, policy)

	// Test various case variations
	variations := []string{
		"uid=alice,ou=users,dc=example,dc=com",
		"UID=ALICE,OU=USERS,DC=EXAMPLE,DC=COM",
		"Uid=Alice,Ou=Users,Dc=Example,Dc=Com",
		"  uid=alice,ou=users,dc=example,dc=com  ",
	}

	for _, v := range variations {
		if !m.HasUserPolicy(v) {
			t.Errorf("expected to find policy for DN variation: %s", v)
		}

		p := m.GetPolicy(v)
		if p.MinLength != 15 {
			t.Errorf("expected MinLength 15 for DN %s, got %d", v, p.MinLength)
		}
	}
}

// TestManagerConcurrency tests concurrent access to the manager.
func TestManagerConcurrency(t *testing.T) {
	m := NewManager(nil)
	done := make(chan bool)

	// Concurrent writers
	for i := 0; i < 10; i++ {
		go func(id int) {
			dn := "uid=user" + string(rune('0'+id)) + ",dc=example,dc=com"
			for j := 0; j < 100; j++ {
				m.SetUserPolicy(dn, &Policy{Enabled: true, MinLength: j})
			}
			done <- true
		}(i)
	}

	// Concurrent readers
	for i := 0; i < 10; i++ {
		go func(id int) {
			dn := "uid=user" + string(rune('0'+id)) + ",dc=example,dc=com"
			for j := 0; j < 100; j++ {
				_ = m.GetPolicy(dn)
				_ = m.HasUserPolicy(dn)
				_ = m.ListUserPolicies()
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 20; i++ {
		<-done
	}
}

// TestSpecialCharacters tests special character detection.
func TestSpecialCharacters(t *testing.T) {
	policy := &Policy{
		Enabled:        true,
		RequireSpecial: true,
	}

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
		err := policy.Validate(pwd)
		if err != nil {
			t.Errorf("password %q should be valid: %v", pwd, err)
		}
	}

	invalidPasswords := []string{
		"Password123",
		"NoSpecialHere",
		"JustLetters",
	}

	for _, pwd := range invalidPasswords {
		err := policy.Validate(pwd)
		if err == nil {
			t.Errorf("password %q should be invalid (no special char)", pwd)
		}
	}
}
