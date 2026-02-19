package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestRun_NoArgs(t *testing.T) {
	exitCode := run([]string{"oba"})
	if exitCode != 1 {
		t.Errorf("expected exit code 1 for no args, got %d", exitCode)
	}
}

func TestRun_Help(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"help command", []string{"oba", "help"}},
		{"short flag", []string{"oba", "-h"}},
		{"long flag", []string{"oba", "--help"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exitCode := run(tt.args)
			if exitCode != 0 {
				t.Errorf("expected exit code 0 for help, got %d", exitCode)
			}
		})
	}
}

func TestRun_UnknownCommand(t *testing.T) {
	exitCode := run([]string{"oba", "unknown"})
	if exitCode != 1 {
		t.Errorf("expected exit code 1 for unknown command, got %d", exitCode)
	}
}

func TestRun_Version(t *testing.T) {
	exitCode := run([]string{"oba", "version"})
	if exitCode != 0 {
		t.Errorf("expected exit code 0 for version, got %d", exitCode)
	}
}

func TestRun_VersionShort(t *testing.T) {
	exitCode := run([]string{"oba", "version", "-short"})
	if exitCode != 0 {
		t.Errorf("expected exit code 0 for version -short, got %d", exitCode)
	}
}

func TestRun_VersionHelp(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"short flag", []string{"oba", "version", "-h"}},
		{"long flag", []string{"oba", "version", "-help"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exitCode := run(tt.args)
			if exitCode != 0 {
				t.Errorf("expected exit code 0 for version help, got %d", exitCode)
			}
		})
	}
}

func TestRun_Serve(t *testing.T) {
	exitCode := run([]string{"oba", "serve"})
	if exitCode != 0 {
		t.Errorf("expected exit code 0 for serve, got %d", exitCode)
	}
}

func TestRun_ServeHelp(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"short flag", []string{"oba", "serve", "-h"}},
		{"long flag", []string{"oba", "serve", "-help"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exitCode := run(tt.args)
			if exitCode != 0 {
				t.Errorf("expected exit code 0 for serve help, got %d", exitCode)
			}
		})
	}
}

func TestRun_ServeWithOptions(t *testing.T) {
	exitCode := run([]string{"oba", "serve", "-address", ":1389", "-tls-address", ":1636"})
	if exitCode != 0 {
		t.Errorf("expected exit code 0 for serve with options, got %d", exitCode)
	}
}

func TestRun_Backup(t *testing.T) {
	// Without required -output flag
	exitCode := run([]string{"oba", "backup"})
	if exitCode != 1 {
		t.Errorf("expected exit code 1 for backup without output, got %d", exitCode)
	}
}

func TestRun_BackupWithOutput(t *testing.T) {
	exitCode := run([]string{"oba", "backup", "-output", "/tmp/backup.bak"})
	if exitCode != 0 {
		t.Errorf("expected exit code 0 for backup with output, got %d", exitCode)
	}
}

func TestRun_BackupHelp(t *testing.T) {
	exitCode := run([]string{"oba", "backup", "-h"})
	if exitCode != 0 {
		t.Errorf("expected exit code 0 for backup help, got %d", exitCode)
	}
}

func TestRun_Restore(t *testing.T) {
	// Without required -input flag
	exitCode := run([]string{"oba", "restore"})
	if exitCode != 1 {
		t.Errorf("expected exit code 1 for restore without input, got %d", exitCode)
	}
}

func TestRun_RestoreWithInput(t *testing.T) {
	exitCode := run([]string{"oba", "restore", "-input", "/tmp/backup.bak"})
	if exitCode != 0 {
		t.Errorf("expected exit code 0 for restore with input, got %d", exitCode)
	}
}

func TestRun_RestoreHelp(t *testing.T) {
	exitCode := run([]string{"oba", "restore", "-h"})
	if exitCode != 0 {
		t.Errorf("expected exit code 0 for restore help, got %d", exitCode)
	}
}

func TestRun_User(t *testing.T) {
	exitCode := run([]string{"oba", "user"})
	if exitCode != 0 {
		t.Errorf("expected exit code 0 for user (shows help), got %d", exitCode)
	}
}

func TestRun_UserHelp(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"help subcommand", []string{"oba", "user", "help"}},
		{"short flag", []string{"oba", "user", "-h"}},
		{"long flag", []string{"oba", "user", "--help"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exitCode := run(tt.args)
			if exitCode != 0 {
				t.Errorf("expected exit code 0 for user help, got %d", exitCode)
			}
		})
	}
}

func TestRun_UserUnknownSubcommand(t *testing.T) {
	exitCode := run([]string{"oba", "user", "unknown"})
	if exitCode != 1 {
		t.Errorf("expected exit code 1 for unknown user subcommand, got %d", exitCode)
	}
}

func TestRun_UserAdd(t *testing.T) {
	// Without required -dn flag
	exitCode := run([]string{"oba", "user", "add"})
	if exitCode != 1 {
		t.Errorf("expected exit code 1 for user add without dn, got %d", exitCode)
	}
}

func TestRun_UserAddWithDN(t *testing.T) {
	exitCode := run([]string{"oba", "user", "add", "-dn", "uid=test,ou=users,dc=example,dc=com"})
	if exitCode != 0 {
		t.Errorf("expected exit code 0 for user add with dn, got %d", exitCode)
	}
}

func TestRun_UserAddHelp(t *testing.T) {
	exitCode := run([]string{"oba", "user", "add", "-h"})
	if exitCode != 0 {
		t.Errorf("expected exit code 0 for user add help, got %d", exitCode)
	}
}

func TestRun_UserDelete(t *testing.T) {
	// Without required -dn flag
	exitCode := run([]string{"oba", "user", "delete"})
	if exitCode != 1 {
		t.Errorf("expected exit code 1 for user delete without dn, got %d", exitCode)
	}
}

func TestRun_UserDeleteWithDN(t *testing.T) {
	exitCode := run([]string{"oba", "user", "delete", "-dn", "uid=test,ou=users,dc=example,dc=com"})
	if exitCode != 0 {
		t.Errorf("expected exit code 0 for user delete with dn, got %d", exitCode)
	}
}

func TestRun_UserPasswd(t *testing.T) {
	// Without required -dn flag
	exitCode := run([]string{"oba", "user", "passwd"})
	if exitCode != 1 {
		t.Errorf("expected exit code 1 for user passwd without dn, got %d", exitCode)
	}
}

func TestRun_UserPasswdWithDN(t *testing.T) {
	exitCode := run([]string{"oba", "user", "passwd", "-dn", "uid=test,ou=users,dc=example,dc=com"})
	if exitCode != 0 {
		t.Errorf("expected exit code 0 for user passwd with dn, got %d", exitCode)
	}
}

func TestRun_UserList(t *testing.T) {
	exitCode := run([]string{"oba", "user", "list"})
	if exitCode != 0 {
		t.Errorf("expected exit code 0 for user list, got %d", exitCode)
	}
}

func TestRun_UserListWithBase(t *testing.T) {
	exitCode := run([]string{"oba", "user", "list", "-base", "ou=users,dc=example,dc=com"})
	if exitCode != 0 {
		t.Errorf("expected exit code 0 for user list with base, got %d", exitCode)
	}
}

func TestRun_UserLock(t *testing.T) {
	// Without required -dn flag
	exitCode := run([]string{"oba", "user", "lock"})
	if exitCode != 1 {
		t.Errorf("expected exit code 1 for user lock without dn, got %d", exitCode)
	}
}

func TestRun_UserLockWithDN(t *testing.T) {
	exitCode := run([]string{"oba", "user", "lock", "-dn", "uid=test,ou=users,dc=example,dc=com"})
	if exitCode != 0 {
		t.Errorf("expected exit code 0 for user lock with dn, got %d", exitCode)
	}
}

func TestRun_UserUnlock(t *testing.T) {
	// Without required -dn flag
	exitCode := run([]string{"oba", "user", "unlock"})
	if exitCode != 1 {
		t.Errorf("expected exit code 1 for user unlock without dn, got %d", exitCode)
	}
}

func TestRun_UserUnlockWithDN(t *testing.T) {
	exitCode := run([]string{"oba", "user", "unlock", "-dn", "uid=test,ou=users,dc=example,dc=com"})
	if exitCode != 0 {
		t.Errorf("expected exit code 0 for user unlock with dn, got %d", exitCode)
	}
}

func TestRun_Config(t *testing.T) {
	exitCode := run([]string{"oba", "config"})
	if exitCode != 0 {
		t.Errorf("expected exit code 0 for config (shows help), got %d", exitCode)
	}
}

func TestRun_ConfigHelp(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"help subcommand", []string{"oba", "config", "help"}},
		{"short flag", []string{"oba", "config", "-h"}},
		{"long flag", []string{"oba", "config", "--help"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exitCode := run(tt.args)
			if exitCode != 0 {
				t.Errorf("expected exit code 0 for config help, got %d", exitCode)
			}
		})
	}
}

func TestRun_ConfigUnknownSubcommand(t *testing.T) {
	exitCode := run([]string{"oba", "config", "unknown"})
	if exitCode != 1 {
		t.Errorf("expected exit code 1 for unknown config subcommand, got %d", exitCode)
	}
}

func TestRun_ConfigValidate(t *testing.T) {
	// Without required -config flag
	exitCode := run([]string{"oba", "config", "validate"})
	if exitCode != 1 {
		t.Errorf("expected exit code 1 for config validate without config, got %d", exitCode)
	}
}

func TestRun_ConfigValidateWithConfig(t *testing.T) {
	// Create a temporary valid config file
	tmpDir := t.TempDir()
	configPath := tmpDir + "/config.yaml"

	validConfig := `
server:
  address: ":389"

storage:
  dataDir: "/var/lib/oba"
`
	if err := os.WriteFile(configPath, []byte(validConfig), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	exitCode := run([]string{"oba", "config", "validate", "-config", configPath})
	if exitCode != 0 {
		t.Errorf("expected exit code 0 for config validate with config, got %d", exitCode)
	}
}

func TestRun_ConfigInit(t *testing.T) {
	exitCode := run([]string{"oba", "config", "init"})
	if exitCode != 0 {
		t.Errorf("expected exit code 0 for config init, got %d", exitCode)
	}
}

func TestRun_ConfigShow(t *testing.T) {
	exitCode := run([]string{"oba", "config", "show"})
	if exitCode != 0 {
		t.Errorf("expected exit code 0 for config show, got %d", exitCode)
	}
}

func TestPrintUsage(t *testing.T) {
	var buf bytes.Buffer
	printUsage(&buf)

	output := buf.String()

	// Check for expected content
	expectedStrings := []string{
		"oba - Zero-dependency LDAP server",
		"Usage:",
		"oba <command> [options]",
		"Commands:",
		"serve",
		"backup",
		"restore",
		"user",
		"config",
		"version",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("expected usage to contain %q", expected)
		}
	}
}

func TestPrintServeUsage(t *testing.T) {
	var buf bytes.Buffer
	printServeUsage(&buf)

	output := buf.String()

	expectedStrings := []string{
		"Start the LDAP server",
		"-config",
		"-address",
		"-tls-address",
		"-data-dir",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("expected serve usage to contain %q", expected)
		}
	}
}

func TestPrintBackupUsage(t *testing.T) {
	var buf bytes.Buffer
	printBackupUsage(&buf)

	output := buf.String()

	expectedStrings := []string{
		"Create database backup",
		"-output",
		"-compress",
		"-incremental",
		"-format",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("expected backup usage to contain %q", expected)
		}
	}
}

func TestPrintRestoreUsage(t *testing.T) {
	var buf bytes.Buffer
	printRestoreUsage(&buf)

	output := buf.String()

	expectedStrings := []string{
		"Restore from backup",
		"-input",
		"-verify",
		"-format",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("expected restore usage to contain %q", expected)
		}
	}
}

func TestPrintUserUsage(t *testing.T) {
	var buf bytes.Buffer
	printUserUsage(&buf)

	output := buf.String()

	expectedStrings := []string{
		"User management",
		"add",
		"delete",
		"passwd",
		"list",
		"lock",
		"unlock",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("expected user usage to contain %q", expected)
		}
	}
}

func TestPrintConfigUsage(t *testing.T) {
	var buf bytes.Buffer
	printConfigUsage(&buf)

	output := buf.String()

	expectedStrings := []string{
		"Configuration management",
		"validate",
		"init",
		"show",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("expected config usage to contain %q", expected)
		}
	}
}

func TestPrintVersionUsage(t *testing.T) {
	var buf bytes.Buffer
	printVersionUsage(&buf)

	output := buf.String()

	expectedStrings := []string{
		"Show version information",
		"-short",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("expected version usage to contain %q", expected)
		}
	}
}

func TestValueOrDefault(t *testing.T) {
	tests := []struct {
		name         string
		value        string
		defaultValue string
		expected     string
	}{
		{"empty value", "", "default", "default"},
		{"non-empty value", "value", "default", "value"},
		{"both empty", "", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := valueOrDefault(tt.value, tt.defaultValue)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestGetVersion(t *testing.T) {
	v := GetVersion()
	if v == "" {
		t.Error("expected non-empty version")
	}
}

func TestGetCommit(t *testing.T) {
	c := GetCommit()
	if c == "" {
		t.Error("expected non-empty commit")
	}
}

func TestGetBuildDate(t *testing.T) {
	d := GetBuildDate()
	if d == "" {
		t.Error("expected non-empty build date")
	}
}
