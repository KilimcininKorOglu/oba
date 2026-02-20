// Example password_policy demonstrates password policy configuration and validation.
//
// This example shows how to:
//   - Configure password complexity requirements
//   - Validate passwords against policy
//   - Check password expiration
//   - Handle account lockout
//
// Run with: go run examples/password_policy/main.go
package main

import (
	"fmt"
	"time"

	"github.com/oba-ldap/oba/internal/password"
)

func main() {
	fmt.Println("=== Password Policy Examples ===")
	fmt.Println()

	// Create a password policy
	policy := &password.Policy{
		Enabled:          true,
		MinLength:        8,
		MaxLength:        128,
		RequireUppercase: true,
		RequireLowercase: true,
		RequireDigit:     true,
		RequireSpecial:   true,
		MaxAge:           90 * 24 * time.Hour, // 90 days
		MinAge:           24 * time.Hour,      // 1 day
		HistoryCount:     5,
		MaxFailures:      5,
		LockoutDuration:  15 * time.Minute,
		AllowUserChange:  true,
	}

	fmt.Println("Policy Configuration:")
	fmt.Printf("  Min Length: %d\n", policy.MinLength)
	fmt.Printf("  Require Uppercase: %v\n", policy.RequireUppercase)
	fmt.Printf("  Require Lowercase: %v\n", policy.RequireLowercase)
	fmt.Printf("  Require Digit: %v\n", policy.RequireDigit)
	fmt.Printf("  Require Special: %v\n", policy.RequireSpecial)
	fmt.Printf("  Max Age: %v\n", policy.MaxAge)
	fmt.Printf("  Max Failures: %d\n", policy.MaxFailures)
	fmt.Printf("  Lockout Duration: %v\n", policy.LockoutDuration)
	fmt.Println()

	// Test various passwords
	testPasswords := []string{
		"short",           // Too short
		"alllowercase",    // No uppercase
		"ALLUPPERCASE",    // No lowercase
		"NoDigitsHere!",   // No digit
		"NoSpecial123",    // No special character
		"Valid@Pass123",   // Valid password
		"Another$ecure1",  // Valid password
	}

	fmt.Println("Password Validation:")
	for _, pwd := range testPasswords {
		err := policy.Validate(pwd)
		if err != nil {
			fmt.Printf("  %-20s INVALID: %s\n", pwd, err.Error())
		} else {
			fmt.Printf("  %-20s VALID\n", pwd)
		}
	}
	fmt.Println()

	// Test password expiration
	fmt.Println("Password Expiration:")
	testDates := []struct {
		name string
		date time.Time
	}{
		{"30 days ago", time.Now().Add(-30 * 24 * time.Hour)},
		{"60 days ago", time.Now().Add(-60 * 24 * time.Hour)},
		{"90 days ago", time.Now().Add(-90 * 24 * time.Hour)},
		{"100 days ago", time.Now().Add(-100 * 24 * time.Hour)},
	}

	for _, td := range testDates {
		expired := policy.IsExpired(td.date)
		status := "not expired"
		if expired {
			status = "EXPIRED"
		}
		fmt.Printf("  Changed %s: %s\n", td.name, status)
	}
	fmt.Println()

	// Test account lockout
	fmt.Println("Account Lockout:")
	testLockouts := []struct {
		failures    int
		lastFailure time.Time
	}{
		{3, time.Now().Add(-5 * time.Minute)},  // Under threshold
		{5, time.Now().Add(-5 * time.Minute)},  // At threshold, recent
		{5, time.Now().Add(-20 * time.Minute)}, // At threshold, lockout expired
		{10, time.Now().Add(-1 * time.Minute)}, // Over threshold, recent
	}

	for _, tl := range testLockouts {
		locked := policy.IsLockedOut(tl.failures, tl.lastFailure)
		status := "not locked"
		if locked {
			status = "LOCKED"
		}
		fmt.Printf("  %d failures, last %v ago: %s\n",
			tl.failures,
			time.Since(tl.lastFailure).Round(time.Minute),
			status)
	}
	fmt.Println()

	// Test password change timing
	fmt.Println("Password Change Timing:")
	testChanges := []struct {
		name string
		date time.Time
	}{
		{"12 hours ago", time.Now().Add(-12 * time.Hour)},
		{"25 hours ago", time.Now().Add(-25 * time.Hour)},
	}

	for _, tc := range testChanges {
		canChange := policy.CanChange(tc.date)
		status := "can change"
		if !canChange {
			status = "TOO SOON"
		}
		fmt.Printf("  Changed %s: %s\n", tc.name, status)
	}
	fmt.Println()

	// Demonstrate default policy
	fmt.Println("Default Policy:")
	defaultPolicy := password.DefaultPolicy()
	fmt.Printf("  Min Length: %d\n", defaultPolicy.MinLength)
	fmt.Printf("  Max Age: %v\n", defaultPolicy.MaxAge)
	fmt.Printf("  History Count: %d\n", defaultPolicy.HistoryCount)
}
