// Package server provides the LDAP server implementation.
package server

import (
	"strings"
	"testing"
)

// TestVerifyPassword tests password verification for various schemes.
func TestVerifyPassword(t *testing.T) {
	tests := []struct {
		name      string
		plaintext string
		stored    string
		wantErr   error
	}{
		{
			name:      "cleartext match",
			plaintext: "secret123",
			stored:    "{CLEARTEXT}secret123",
			wantErr:   nil,
		},
		{
			name:      "cleartext mismatch",
			plaintext: "wrongpassword",
			stored:    "{CLEARTEXT}secret123",
			wantErr:   ErrPasswordMismatch,
		},
		{
			name:      "empty stored password",
			plaintext: "anypassword",
			stored:    "",
			wantErr:   ErrInvalidPasswordFormat,
		},
		{
			name:      "no scheme prefix match",
			plaintext: "plaintext",
			stored:    "plaintext",
			wantErr:   nil,
		},
		{
			name:      "no scheme prefix mismatch",
			plaintext: "wrong",
			stored:    "plaintext",
			wantErr:   ErrPasswordMismatch,
		},
		{
			name:      "unsupported scheme",
			plaintext: "password",
			stored:    "{MD5}somebase64hash",
			wantErr:   ErrUnsupportedScheme,
		},
		{
			name:      "case insensitive scheme",
			plaintext: "secret123",
			stored:    "{cleartext}secret123",
			wantErr:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := VerifyPassword(tt.plaintext, tt.stored)
			if err != tt.wantErr {
				t.Errorf("VerifyPassword() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestVerifyPassword_SHA256 tests SHA256 password verification.
func TestVerifyPassword_SHA256(t *testing.T) {
	// Create a SHA256 hash
	hashed, err := HashPassword("testpassword", SchemeSHA256)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	tests := []struct {
		name      string
		plaintext string
		wantErr   error
	}{
		{
			name:      "correct password",
			plaintext: "testpassword",
			wantErr:   nil,
		},
		{
			name:      "wrong password",
			plaintext: "wrongpassword",
			wantErr:   ErrPasswordMismatch,
		},
		{
			name:      "empty password",
			plaintext: "",
			wantErr:   ErrPasswordMismatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := VerifyPassword(tt.plaintext, hashed)
			if err != tt.wantErr {
				t.Errorf("VerifyPassword() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestVerifyPassword_SSHA256 tests salted SHA256 password verification.
func TestVerifyPassword_SSHA256(t *testing.T) {
	// Create a SSHA256 hash
	hashed, err := HashPassword("testpassword", SchemeSSHA256)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	tests := []struct {
		name      string
		plaintext string
		wantErr   error
	}{
		{
			name:      "correct password",
			plaintext: "testpassword",
			wantErr:   nil,
		},
		{
			name:      "wrong password",
			plaintext: "wrongpassword",
			wantErr:   ErrPasswordMismatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := VerifyPassword(tt.plaintext, hashed)
			if err != tt.wantErr {
				t.Errorf("VerifyPassword() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestVerifyPassword_SHA512 tests SHA512 password verification.
func TestVerifyPassword_SHA512(t *testing.T) {
	// Create a SHA512 hash
	hashed, err := HashPassword("testpassword", SchemeSHA512)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	tests := []struct {
		name      string
		plaintext string
		wantErr   error
	}{
		{
			name:      "correct password",
			plaintext: "testpassword",
			wantErr:   nil,
		},
		{
			name:      "wrong password",
			plaintext: "wrongpassword",
			wantErr:   ErrPasswordMismatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := VerifyPassword(tt.plaintext, hashed)
			if err != tt.wantErr {
				t.Errorf("VerifyPassword() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestVerifyPassword_SSHA512 tests salted SHA512 password verification.
func TestVerifyPassword_SSHA512(t *testing.T) {
	// Create a SSHA512 hash
	hashed, err := HashPassword("testpassword", SchemeSSHA512)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	tests := []struct {
		name      string
		plaintext string
		wantErr   error
	}{
		{
			name:      "correct password",
			plaintext: "testpassword",
			wantErr:   nil,
		},
		{
			name:      "wrong password",
			plaintext: "wrongpassword",
			wantErr:   ErrPasswordMismatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := VerifyPassword(tt.plaintext, hashed)
			if err != tt.wantErr {
				t.Errorf("VerifyPassword() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestVerifyPassword_InvalidFormat tests invalid password format handling.
func TestVerifyPassword_InvalidFormat(t *testing.T) {
	tests := []struct {
		name    string
		stored  string
		wantErr error
	}{
		{
			name:    "SHA256 invalid base64",
			stored:  "{SHA256}not-valid-base64!!!",
			wantErr: ErrInvalidPasswordFormat,
		},
		{
			name:    "SHA256 wrong hash length",
			stored:  "{SHA256}dG9vc2hvcnQ=", // "tooshort" in base64
			wantErr: ErrInvalidPasswordFormat,
		},
		{
			name:    "SSHA256 too short",
			stored:  "{SSHA256}dG9vc2hvcnQ=", // "tooshort" in base64
			wantErr: ErrInvalidPasswordFormat,
		},
		{
			name:    "SHA512 invalid base64",
			stored:  "{SHA512}not-valid-base64!!!",
			wantErr: ErrInvalidPasswordFormat,
		},
		{
			name:    "SHA512 wrong hash length",
			stored:  "{SHA512}dG9vc2hvcnQ=", // "tooshort" in base64
			wantErr: ErrInvalidPasswordFormat,
		},
		{
			name:    "SSHA512 too short",
			stored:  "{SSHA512}dG9vc2hvcnQ=", // "tooshort" in base64
			wantErr: ErrInvalidPasswordFormat,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := VerifyPassword("anypassword", tt.stored)
			if err != tt.wantErr {
				t.Errorf("VerifyPassword() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestHashPassword tests password hashing for various schemes.
func TestHashPassword(t *testing.T) {
	tests := []struct {
		name       string
		plaintext  string
		scheme     string
		wantPrefix string
		wantErr    error
	}{
		{
			name:       "cleartext",
			plaintext:  "password123",
			scheme:     SchemeCleartext,
			wantPrefix: SchemeCleartext,
			wantErr:    nil,
		},
		{
			name:       "SHA256",
			plaintext:  "password123",
			scheme:     SchemeSHA256,
			wantPrefix: SchemeSHA256,
			wantErr:    nil,
		},
		{
			name:       "SSHA256",
			plaintext:  "password123",
			scheme:     SchemeSSHA256,
			wantPrefix: SchemeSSHA256,
			wantErr:    nil,
		},
		{
			name:       "SHA512",
			plaintext:  "password123",
			scheme:     SchemeSHA512,
			wantPrefix: SchemeSHA512,
			wantErr:    nil,
		},
		{
			name:       "SSHA512",
			plaintext:  "password123",
			scheme:     SchemeSSHA512,
			wantPrefix: SchemeSSHA512,
			wantErr:    nil,
		},
		{
			name:       "unsupported scheme",
			plaintext:  "password123",
			scheme:     "{MD5}",
			wantPrefix: "",
			wantErr:    ErrUnsupportedScheme,
		},
		{
			name:       "lowercase scheme",
			plaintext:  "password123",
			scheme:     "{sha256}",
			wantPrefix: SchemeSHA256,
			wantErr:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hashed, err := HashPassword(tt.plaintext, tt.scheme)

			if err != tt.wantErr {
				t.Errorf("HashPassword() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr != nil {
				return
			}

			if !strings.HasPrefix(hashed, tt.wantPrefix) {
				t.Errorf("HashPassword() = %v, want prefix %v", hashed, tt.wantPrefix)
			}

			// Verify the hash can be verified
			err = VerifyPassword(tt.plaintext, hashed)
			if err != nil {
				t.Errorf("VerifyPassword() failed for hashed password: %v", err)
			}
		})
	}
}

// TestHashPassword_Roundtrip tests that hashed passwords can be verified.
func TestHashPassword_Roundtrip(t *testing.T) {
	schemes := []string{
		SchemeCleartext,
		SchemeSHA256,
		SchemeSSHA256,
		SchemeSHA512,
		SchemeSSHA512,
	}

	passwords := []string{
		"simple",
		"with spaces",
		"with!special@chars#",
		"unicode: 日本語",
		"",
		"a",
		strings.Repeat("long", 100),
	}

	for _, scheme := range schemes {
		for _, password := range passwords {
			t.Run(scheme+"_"+password[:min(10, len(password))], func(t *testing.T) {
				hashed, err := HashPassword(password, scheme)
				if err != nil {
					t.Fatalf("HashPassword() error = %v", err)
				}

				err = VerifyPassword(password, hashed)
				if err != nil {
					t.Errorf("VerifyPassword() error = %v", err)
				}

				// Verify wrong password fails
				if password != "" {
					err = VerifyPassword(password+"wrong", hashed)
					if err != ErrPasswordMismatch {
						t.Errorf("VerifyPassword() with wrong password error = %v, want %v", err, ErrPasswordMismatch)
					}
				}
			})
		}
	}
}

// TestGenerateSaltSecure tests secure salt generation.
func TestGenerateSaltSecure(t *testing.T) {
	lengths := []int{8, 16, 32, 64}

	for _, length := range lengths {
		t.Run(string(rune('0'+length)), func(t *testing.T) {
			salt, err := GenerateSaltSecure(length)
			if err != nil {
				t.Fatalf("GenerateSaltSecure() error = %v", err)
			}

			if len(salt) != length {
				t.Errorf("GenerateSaltSecure() length = %v, want %v", len(salt), length)
			}
		})
	}
}

// TestPasswordAttribute tests the password attribute constant.
func TestPasswordAttribute(t *testing.T) {
	if PasswordAttribute != "userPassword" {
		t.Errorf("PasswordAttribute = %v, want userPassword", PasswordAttribute)
	}
}

// TestSchemeConstants tests that scheme constants are properly defined.
func TestSchemeConstants(t *testing.T) {
	schemes := map[string]string{
		"SSHA":      SchemeSSHA,
		"SHA":       SchemeSHA,
		"SSHA256":   SchemeSSHA256,
		"SHA256":    SchemeSHA256,
		"SSHA512":   SchemeSSHA512,
		"SHA512":    SchemeSHA512,
		"CLEARTEXT": SchemeCleartext,
	}

	for name, scheme := range schemes {
		if !strings.HasPrefix(scheme, "{") || !strings.HasSuffix(scheme, "}") {
			t.Errorf("Scheme %s = %v, should be wrapped in braces", name, scheme)
		}
	}
}

// min returns the minimum of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
