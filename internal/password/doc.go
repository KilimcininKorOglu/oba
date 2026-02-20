// Package password provides password policy configuration and validation
// for the Oba LDAP server.
//
// # Overview
//
// The password package implements password policy enforcement as commonly
// used in LDAP directories. It provides:
//
//   - Password complexity requirements
//   - Password expiration and history
//   - Account lockout after failed attempts
//   - Password change restrictions
//
// # Password Policy
//
// Create a policy with complexity requirements:
//
//	policy := &password.Policy{
//	    Enabled:          true,
//	    MinLength:        8,
//	    MaxLength:        128,
//	    RequireUppercase: true,
//	    RequireLowercase: true,
//	    RequireDigit:     true,
//	    RequireSpecial:   false,
//	    MaxAge:           90 * 24 * time.Hour, // 90 days
//	    HistoryCount:     5,
//	    MaxFailures:      5,
//	    LockoutDuration:  15 * time.Minute,
//	}
//
// Or use defaults:
//
//	policy := password.DefaultPolicy()
//
// # Password Validation
//
// Validate passwords against policy:
//
//	if err := policy.Validate("MyP@ssw0rd"); err != nil {
//	    if verr, ok := err.(*password.ValidationError); ok {
//	        switch verr.Code {
//	        case password.ErrTooShort:
//	            // Password too short
//	        case password.ErrNoUppercase:
//	            // Missing uppercase letter
//	        case password.ErrNoDigit:
//	            // Missing digit
//	        }
//	    }
//	}
//
// # Password Expiration
//
// Check if a password has expired:
//
//	lastChanged := time.Now().Add(-100 * 24 * time.Hour) // 100 days ago
//
//	if policy.IsExpired(lastChanged) {
//	    // Password has expired, require change
//	}
//
// # Account Lockout
//
// Check if an account is locked:
//
//	failureCount := 5
//	lastFailure := time.Now().Add(-5 * time.Minute)
//
//	if policy.IsLockedOut(failureCount, lastFailure) {
//	    // Account is locked
//	}
//
// # Password History
//
// The HistoryManager tracks previous passwords:
//
//	manager := password.NewHistoryManager(policy.HistoryCount)
//
//	// Check if password was used recently
//	if manager.Contains(hashedPassword) {
//	    // Password was used before
//	}
//
//	// Add new password to history
//	manager.Add(hashedPassword)
//
// # Policy Attributes
//
// Standard LDAP password policy attributes:
//
//   - pwdChangedTime: Last password change timestamp
//   - pwdAccountLockedTime: Account lock timestamp
//   - pwdFailureTime: Failed attempt timestamps
//   - pwdHistory: Previous password hashes
//   - pwdMustChange: User must change password on login
//
// # Policy Merging
//
// Merge per-user overrides with global policy:
//
//	globalPolicy := password.DefaultPolicy()
//	userOverride := &password.Policy{
//	    MinLength: 12, // Stricter requirement for this user
//	}
//
//	effectivePolicy := globalPolicy.Merge(userOverride)
package password
