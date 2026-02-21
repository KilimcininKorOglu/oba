// Package main provides tests for user management commands.
package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/KilimcininKorOglu/oba/internal/storage"
	"github.com/KilimcininKorOglu/oba/internal/storage/engine"
)

// mockPasswordReader is a mock password reader for testing.
type mockPasswordReader struct {
	passwords []string
	index     int
}

func (m *mockPasswordReader) ReadPassword() (string, error) {
	if m.index >= len(m.passwords) {
		return "", nil
	}
	pw := m.passwords[m.index]
	m.index++
	return pw, nil
}

func TestHashPassword(t *testing.T) {
	password := "testpassword"
	hash := hashPassword(password)

	if !strings.HasPrefix(hash, "{SHA256}") {
		t.Errorf("expected hash to start with {SHA256}, got %s", hash)
	}

	// Same password should produce same hash
	hash2 := hashPassword(password)
	if hash != hash2 {
		t.Errorf("expected same hash for same password")
	}

	// Different password should produce different hash
	hash3 := hashPassword("differentpassword")
	if hash == hash3 {
		t.Errorf("expected different hash for different password")
	}
}

func TestIsUserEntry(t *testing.T) {
	tests := []struct {
		name         string
		objectClass  []string
		expectedUser bool
	}{
		{
			name:         "person",
			objectClass:  []string{"top", "person"},
			expectedUser: true,
		},
		{
			name:         "inetOrgPerson",
			objectClass:  []string{"top", "person", "inetOrgPerson"},
			expectedUser: true,
		},
		{
			name:         "organizationalPerson",
			objectClass:  []string{"top", "person", "organizationalPerson"},
			expectedUser: true,
		},
		{
			name:         "organizationalUnit",
			objectClass:  []string{"top", "organizationalUnit"},
			expectedUser: false,
		},
		{
			name:         "empty",
			objectClass:  []string{},
			expectedUser: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := storage.NewEntry("cn=test,dc=example,dc=com")
			entry.SetStringAttribute(objectClassAttr, tt.objectClass...)

			result := isUserEntry(entry)
			if result != tt.expectedUser {
				t.Errorf("expected isUserEntry=%v, got %v", tt.expectedUser, result)
			}
		})
	}
}

func TestIsAccountLocked(t *testing.T) {
	tests := []struct {
		name       string
		lockedTime string
		expected   bool
	}{
		{
			name:       "locked",
			lockedTime: "20260101000000Z",
			expected:   true,
		},
		{
			name:       "not locked",
			lockedTime: "",
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := storage.NewEntry("cn=test,dc=example,dc=com")
			if tt.lockedTime != "" {
				entry.SetStringAttribute(pwdAccountLockedTimeAttr, tt.lockedTime)
			}

			result := isAccountLocked(entry)
			if result != tt.expected {
				t.Errorf("expected isAccountLocked=%v, got %v", tt.expected, result)
			}
		})
	}
}

// Test help messages for all commands

func TestUserAddCmdImpl_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer
	impl := &userCmdImpl{
		stdout: &stdout,
		stderr: &stderr,
	}

	exitCode := impl.userAddCmdImpl([]string{"-h"})
	if exitCode != 0 {
		t.Errorf("expected exit code 0 for help, got %d", exitCode)
	}

	output := stdout.String()
	if !strings.Contains(output, "Add a new user") {
		t.Errorf("expected help output to contain 'Add a new user'")
	}
}

func TestUserDeleteCmdImpl_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer
	impl := &userCmdImpl{
		stdout: &stdout,
		stderr: &stderr,
	}

	exitCode := impl.userDeleteCmdImpl([]string{"-h"})
	if exitCode != 0 {
		t.Errorf("expected exit code 0 for help, got %d", exitCode)
	}

	if !strings.Contains(stdout.String(), "Delete a user") {
		t.Errorf("expected help output to contain 'Delete a user'")
	}
}

func TestUserListCmdImpl_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer
	impl := &userCmdImpl{
		stdout: &stdout,
		stderr: &stderr,
	}

	exitCode := impl.userListCmdImpl([]string{"-h"})
	if exitCode != 0 {
		t.Errorf("expected exit code 0 for help, got %d", exitCode)
	}

	if !strings.Contains(stdout.String(), "List users") {
		t.Errorf("expected help output to contain 'List users'")
	}
}

func TestUserPasswdCmdImpl_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer
	impl := &userCmdImpl{
		stdout: &stdout,
		stderr: &stderr,
	}

	exitCode := impl.userPasswdCmdImpl([]string{"-h"})
	if exitCode != 0 {
		t.Errorf("expected exit code 0 for help, got %d", exitCode)
	}

	if !strings.Contains(stdout.String(), "Change user password") {
		t.Errorf("expected help output to contain 'Change user password'")
	}
}

func TestUserLockCmdImpl_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer
	impl := &userCmdImpl{
		stdout: &stdout,
		stderr: &stderr,
	}

	exitCode := impl.userLockCmdImpl([]string{"-h"})
	if exitCode != 0 {
		t.Errorf("expected exit code 0 for help, got %d", exitCode)
	}

	if !strings.Contains(stdout.String(), "Lock a user account") {
		t.Errorf("expected help output to contain 'Lock a user account'")
	}
}

func TestUserUnlockCmdImpl_Help(t *testing.T) {
	var stdout, stderr bytes.Buffer
	impl := &userCmdImpl{
		stdout: &stdout,
		stderr: &stderr,
	}

	exitCode := impl.userUnlockCmdImpl([]string{"-h"})
	if exitCode != 0 {
		t.Errorf("expected exit code 0 for help, got %d", exitCode)
	}

	if !strings.Contains(stdout.String(), "Unlock a user account") {
		t.Errorf("expected help output to contain 'Unlock a user account'")
	}
}

// Test missing required arguments

func TestUserAddCmdImpl_MissingDN(t *testing.T) {
	var stdout, stderr bytes.Buffer
	impl := &userCmdImpl{
		stdout: &stdout,
		stderr: &stderr,
	}

	exitCode := impl.userAddCmdImpl([]string{})
	if exitCode != 1 {
		t.Errorf("expected exit code 1 for missing DN, got %d", exitCode)
	}

	if !strings.Contains(stderr.String(), "-dn is required") {
		t.Errorf("expected error message about missing DN")
	}
}

func TestUserDeleteCmdImpl_MissingDN(t *testing.T) {
	var stdout, stderr bytes.Buffer
	impl := &userCmdImpl{
		stdout: &stdout,
		stderr: &stderr,
	}

	exitCode := impl.userDeleteCmdImpl([]string{})
	if exitCode != 1 {
		t.Errorf("expected exit code 1 for missing DN, got %d", exitCode)
	}
}

func TestUserPasswdCmdImpl_MissingDN(t *testing.T) {
	var stdout, stderr bytes.Buffer
	impl := &userCmdImpl{
		stdout: &stdout,
		stderr: &stderr,
	}

	exitCode := impl.userPasswdCmdImpl([]string{})
	if exitCode != 1 {
		t.Errorf("expected exit code 1 for missing DN, got %d", exitCode)
	}
}

func TestUserLockCmdImpl_MissingDN(t *testing.T) {
	var stdout, stderr bytes.Buffer
	impl := &userCmdImpl{
		stdout: &stdout,
		stderr: &stderr,
	}

	exitCode := impl.userLockCmdImpl([]string{})
	if exitCode != 1 {
		t.Errorf("expected exit code 1 for missing DN, got %d", exitCode)
	}
}

func TestUserUnlockCmdImpl_MissingDN(t *testing.T) {
	var stdout, stderr bytes.Buffer
	impl := &userCmdImpl{
		stdout: &stdout,
		stderr: &stderr,
	}

	exitCode := impl.userUnlockCmdImpl([]string{})
	if exitCode != 1 {
		t.Errorf("expected exit code 1 for missing DN, got %d", exitCode)
	}
}

// Test successful operations

func TestUserAddCmdImpl_Success(t *testing.T) {
	tmpDir := t.TempDir()

	var stdout, stderr bytes.Buffer
	impl := &userCmdImpl{
		stdout:         &stdout,
		stderr:         &stderr,
		passwordReader: &mockPasswordReader{passwords: []string{}},
		openDB:         engine.Open,
	}

	dn := "uid=testuser,ou=users,dc=example,dc=com"
	exitCode := impl.userAddCmdImpl([]string{"-dn", dn, "-data-dir", tmpDir})
	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d. stderr: %s", exitCode, stderr.String())
	}

	if !strings.Contains(stdout.String(), "User added:") {
		t.Errorf("expected success message, got: %s", stdout.String())
	}

	if !strings.Contains(stdout.String(), dn) {
		t.Errorf("expected DN in output, got: %s", stdout.String())
	}
}

func TestUserAddCmdImpl_WithPassword(t *testing.T) {
	tmpDir := t.TempDir()

	var stdout, stderr bytes.Buffer
	impl := &userCmdImpl{
		stdout:         &stdout,
		stderr:         &stderr,
		passwordReader: &mockPasswordReader{passwords: []string{"testpass123", "testpass123"}},
		openDB:         engine.Open,
	}

	dn := "uid=testuser,ou=users,dc=example,dc=com"
	exitCode := impl.userAddCmdImpl([]string{"-dn", dn, "-password", "-data-dir", tmpDir})
	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d. stderr: %s", exitCode, stderr.String())
	}

	if !strings.Contains(stdout.String(), "User added:") {
		t.Errorf("expected success message, got: %s", stdout.String())
	}
}

func TestUserAddCmdImpl_PasswordMismatch(t *testing.T) {
	tmpDir := t.TempDir()

	var stdout, stderr bytes.Buffer
	impl := &userCmdImpl{
		stdout:         &stdout,
		stderr:         &stderr,
		passwordReader: &mockPasswordReader{passwords: []string{"password1", "password2"}},
		openDB:         engine.Open,
	}

	dn := "uid=testuser,ou=users,dc=example,dc=com"
	exitCode := impl.userAddCmdImpl([]string{"-dn", dn, "-password", "-data-dir", tmpDir})
	if exitCode != 1 {
		t.Errorf("expected exit code 1 for password mismatch, got %d", exitCode)
	}

	if !strings.Contains(stderr.String(), "passwords do not match") {
		t.Errorf("expected password mismatch error")
	}
}

func TestUserDeleteCmdImpl_NotFound(t *testing.T) {
	tmpDir := t.TempDir()

	// Create empty database
	opts := storage.DefaultEngineOptions().
		WithDataDir(tmpDir).
		WithCreateIfNotExists(true)

	db, err := engine.Open(tmpDir, opts)
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	db.Close()

	var stdout, stderr bytes.Buffer
	impl := &userCmdImpl{
		stdout: &stdout,
		stderr: &stderr,
		openDB: engine.Open,
	}

	exitCode := impl.userDeleteCmdImpl([]string{"-dn", "uid=nonexistent,dc=example,dc=com", "-data-dir", tmpDir})
	if exitCode != 1 {
		t.Errorf("expected exit code 1 for not found, got %d", exitCode)
	}

	if !strings.Contains(stderr.String(), "user not found") {
		t.Errorf("expected not found error message")
	}
}

func TestUserPasswdCmdImpl_PasswordMismatch(t *testing.T) {
	tmpDir := t.TempDir()

	// Create database with a user
	opts := storage.DefaultEngineOptions().
		WithDataDir(tmpDir).
		WithCreateIfNotExists(true)

	db, err := engine.Open(tmpDir, opts)
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	db.Close()

	var stdout, stderr bytes.Buffer
	impl := &userCmdImpl{
		stdout:         &stdout,
		stderr:         &stderr,
		passwordReader: &mockPasswordReader{passwords: []string{"password1", "password2"}},
		openDB:         engine.Open,
	}

	exitCode := impl.userPasswdCmdImpl([]string{"-dn", "uid=test,dc=example,dc=com", "-data-dir", tmpDir})
	if exitCode != 1 {
		t.Errorf("expected exit code 1 for password mismatch, got %d", exitCode)
	}

	if !strings.Contains(stderr.String(), "passwords do not match") {
		t.Errorf("expected password mismatch error")
	}
}

func TestUserPasswdCmdImpl_NotFound(t *testing.T) {
	tmpDir := t.TempDir()

	// Create empty database
	opts := storage.DefaultEngineOptions().
		WithDataDir(tmpDir).
		WithCreateIfNotExists(true)

	db, err := engine.Open(tmpDir, opts)
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	db.Close()

	var stdout, stderr bytes.Buffer
	impl := &userCmdImpl{
		stdout:         &stdout,
		stderr:         &stderr,
		passwordReader: &mockPasswordReader{passwords: []string{"newpass", "newpass"}},
		openDB:         engine.Open,
	}

	exitCode := impl.userPasswdCmdImpl([]string{"-dn", "uid=nonexistent,dc=example,dc=com", "-data-dir", tmpDir})
	if exitCode != 1 {
		t.Errorf("expected exit code 1 for not found, got %d", exitCode)
	}

	if !strings.Contains(stderr.String(), "user not found") {
		t.Errorf("expected not found error message")
	}
}

func TestUserLockCmdImpl_NotFound(t *testing.T) {
	tmpDir := t.TempDir()

	// Create empty database
	opts := storage.DefaultEngineOptions().
		WithDataDir(tmpDir).
		WithCreateIfNotExists(true)

	db, err := engine.Open(tmpDir, opts)
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	db.Close()

	var stdout, stderr bytes.Buffer
	impl := &userCmdImpl{
		stdout: &stdout,
		stderr: &stderr,
		openDB: engine.Open,
	}

	exitCode := impl.userLockCmdImpl([]string{"-dn", "uid=nonexistent,dc=example,dc=com", "-data-dir", tmpDir})
	if exitCode != 1 {
		t.Errorf("expected exit code 1 for not found, got %d", exitCode)
	}

	if !strings.Contains(stderr.String(), "user not found") {
		t.Errorf("expected not found error message")
	}
}

func TestUserUnlockCmdImpl_NotFound(t *testing.T) {
	tmpDir := t.TempDir()

	// Create empty database
	opts := storage.DefaultEngineOptions().
		WithDataDir(tmpDir).
		WithCreateIfNotExists(true)

	db, err := engine.Open(tmpDir, opts)
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	db.Close()

	var stdout, stderr bytes.Buffer
	impl := &userCmdImpl{
		stdout: &stdout,
		stderr: &stderr,
		openDB: engine.Open,
	}

	exitCode := impl.userUnlockCmdImpl([]string{"-dn", "uid=nonexistent,dc=example,dc=com", "-data-dir", tmpDir})
	if exitCode != 1 {
		t.Errorf("expected exit code 1 for not found, got %d", exitCode)
	}

	if !strings.Contains(stderr.String(), "user not found") {
		t.Errorf("expected not found error message")
	}
}

// Integration tests for the main command functions

func TestUserCmd_Help(t *testing.T) {
	exitCode := userCmd([]string{"-h"})
	if exitCode != 0 {
		t.Errorf("expected exit code 0 for help, got %d", exitCode)
	}
}

func TestUserCmd_UnknownSubcommand(t *testing.T) {
	exitCode := userCmd([]string{"unknown"})
	if exitCode != 1 {
		t.Errorf("expected exit code 1 for unknown subcommand, got %d", exitCode)
	}
}

func TestUserCmd_Add_MissingDN(t *testing.T) {
	exitCode := userCmd([]string{"add"})
	if exitCode != 1 {
		t.Errorf("expected exit code 1 for missing DN, got %d", exitCode)
	}
}

func TestUserCmd_Delete_MissingDN(t *testing.T) {
	exitCode := userCmd([]string{"delete"})
	if exitCode != 1 {
		t.Errorf("expected exit code 1 for missing DN, got %d", exitCode)
	}
}

func TestUserCmd_Passwd_MissingDN(t *testing.T) {
	exitCode := userCmd([]string{"passwd"})
	if exitCode != 1 {
		t.Errorf("expected exit code 1 for missing DN, got %d", exitCode)
	}
}

func TestUserCmd_Lock_MissingDN(t *testing.T) {
	exitCode := userCmd([]string{"lock"})
	if exitCode != 1 {
		t.Errorf("expected exit code 1 for missing DN, got %d", exitCode)
	}
}

func TestUserCmd_Unlock_MissingDN(t *testing.T) {
	exitCode := userCmd([]string{"unlock"})
	if exitCode != 1 {
		t.Errorf("expected exit code 1 for missing DN, got %d", exitCode)
	}
}

// Test readPasswordFromStdin
func TestReadPasswordFromStdin(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple password",
			input:    "password123\n",
			expected: "password123",
		},
		{
			name:     "password with CRLF",
			input:    "password123\r\n",
			expected: "password123",
		},
		{
			name:     "empty password",
			input:    "\n",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			result, err := readPasswordFromStdin(reader)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// Test database path handling
func TestUserCmdImpl_InvalidDataDir(t *testing.T) {
	var stdout, stderr bytes.Buffer
	impl := &userCmdImpl{
		stdout: &stdout,
		stderr: &stderr,
		openDB: engine.Open,
	}

	// Use a path that doesn't exist and can't be created
	invalidPath := filepath.Join("/nonexistent", "path", "that", "cannot", "exist")

	exitCode := impl.userListCmdImpl([]string{"-data-dir", invalidPath})
	if exitCode != 1 {
		t.Errorf("expected exit code 1 for invalid data dir, got %d", exitCode)
	}

	if !strings.Contains(stderr.String(), "failed to open database") {
		t.Errorf("expected database open error message")
	}
}
