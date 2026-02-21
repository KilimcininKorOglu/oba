// Package main provides user management commands for the oba LDAP server.
package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/KilimcininKorOglu/oba/internal/storage"
	"github.com/KilimcininKorOglu/oba/internal/storage/engine"
)

const (
	// defaultDataDir is the default data directory for the database.
	defaultDataDir = "/var/lib/oba"

	// pwdAccountLockedTimeAttr is the attribute for account lock time.
	pwdAccountLockedTimeAttr = "pwdaccountlockedtime"

	// userPasswordAttr is the attribute for user password.
	userPasswordAttr = "userpassword"

	// objectClassAttr is the attribute for object class.
	objectClassAttr = "objectclass"
)

// passwordReader is an interface for reading passwords.
// This allows for testing without actual terminal input.
type passwordReader interface {
	ReadPassword() (string, error)
}

// termPasswordReader reads passwords from the terminal.
type termPasswordReader struct {
	stdin io.Reader
}

func (t *termPasswordReader) ReadPassword() (string, error) {
	return readPasswordFromStdin(t.stdin)
}

// readPasswordFromStdin reads a password from stdin.
// For simplicity, this reads until newline. In production, you would
// disable echo using terminal-specific syscalls.
func readPasswordFromStdin(r io.Reader) (string, error) {
	reader := bufio.NewReader(r)
	password, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	// Trim newline
	if len(password) > 0 && password[len(password)-1] == '\n' {
		password = password[:len(password)-1]
	}
	if len(password) > 0 && password[len(password)-1] == '\r' {
		password = password[:len(password)-1]
	}
	return password, nil
}

// userCmdImpl handles the user command with dependency injection for testing.
type userCmdImpl struct {
	stdout         io.Writer
	stderr         io.Writer
	stdin          io.Reader
	passwordReader passwordReader
	openDB         func(path string, opts storage.EngineOptions) (*engine.ObaDB, error)
}

// newUserCmdImpl creates a new userCmdImpl with default dependencies.
func newUserCmdImpl() *userCmdImpl {
	return &userCmdImpl{
		stdout:         os.Stdout,
		stderr:         os.Stderr,
		stdin:          os.Stdin,
		passwordReader: &termPasswordReader{stdin: os.Stdin},
		openDB:         engine.Open,
	}
}

// promptPassword prompts for a password and returns it.
func (u *userCmdImpl) promptPassword(prompt string) (string, error) {
	fmt.Fprint(u.stdout, prompt)
	password, err := u.passwordReader.ReadPassword()
	if err != nil {
		return "", err
	}
	return password, nil
}

// hashPassword hashes a password using SHA256.
// In a production system, you would use bcrypt or argon2.
func hashPassword(password string) string {
	hash := sha256.Sum256([]byte(password))
	return "{SHA256}" + hex.EncodeToString(hash[:])
}

// userAddCmdImpl handles the user add subcommand.
func (u *userCmdImpl) userAddCmdImpl(args []string) int {
	fs := flag.NewFlagSet("user add", flag.ContinueOnError)
	fs.SetOutput(u.stderr)

	dn := fs.String("dn", "", "User DN (required)")
	password := fs.Bool("password", false, "Prompt for password")
	dataDir := fs.String("data-dir", defaultDataDir, "Data directory")
	help := fs.Bool("h", false, "Show help message")
	helpLong := fs.Bool("help", false, "Show help message")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if *help || *helpLong {
		fmt.Fprintln(u.stdout, "Add a new user")
		fmt.Fprintln(u.stdout)
		fmt.Fprintln(u.stdout, "Usage:")
		fmt.Fprintln(u.stdout, "  oba user add [options]")
		fmt.Fprintln(u.stdout)
		fmt.Fprintln(u.stdout, "Options:")
		fmt.Fprintln(u.stdout, "  -dn string")
		fmt.Fprintln(u.stdout, "        User DN (required)")
		fmt.Fprintln(u.stdout, "  -password")
		fmt.Fprintln(u.stdout, "        Prompt for password")
		fmt.Fprintln(u.stdout, "  -data-dir string")
		fmt.Fprintf(u.stdout, "        Data directory (default %q)\n", defaultDataDir)
		return 0
	}

	if *dn == "" {
		fmt.Fprintln(u.stderr, "Error: -dn is required")
		return 1
	}

	// Get password if requested
	var pw string
	if *password {
		var err error
		pw, err = u.promptPassword("Enter password: ")
		if err != nil {
			fmt.Fprintf(u.stderr, "Error reading password: %v\n", err)
			return 1
		}

		confirm, err := u.promptPassword("Confirm password: ")
		if err != nil {
			fmt.Fprintf(u.stderr, "Error reading password: %v\n", err)
			return 1
		}

		if pw != confirm {
			fmt.Fprintln(u.stderr, "Error: passwords do not match")
			return 1
		}
	}

	// Open database
	opts := storage.DefaultEngineOptions().
		WithDataDir(*dataDir).
		WithCreateIfNotExists(true)

	db, err := u.openDB(*dataDir, opts)
	if err != nil {
		fmt.Fprintf(u.stderr, "Error: failed to open database: %v\n", err)
		return 1
	}
	defer db.Close()

	// Begin transaction
	txIface, err := db.Begin()
	if err != nil {
		fmt.Fprintf(u.stderr, "Error: failed to begin transaction: %v\n", err)
		return 1
	}

	// Create entry
	entry := storage.NewEntry(*dn)
	entry.SetStringAttribute(objectClassAttr, "top", "person", "inetOrgPerson")

	if pw != "" {
		entry.SetStringAttribute(userPasswordAttr, hashPassword(pw))
	}

	// Put entry
	if err := db.Put(txIface, entry); err != nil {
		db.Rollback(txIface)
		if err == engine.ErrEntryExists {
			fmt.Fprintf(u.stderr, "Error: user already exists: %s\n", *dn)
		} else {
			fmt.Fprintf(u.stderr, "Error: failed to add user: %v\n", err)
		}
		return 1
	}

	// Commit transaction
	if err := db.Commit(txIface); err != nil {
		fmt.Fprintf(u.stderr, "Error: failed to commit transaction: %v\n", err)
		return 1
	}

	fmt.Fprintf(u.stdout, "User added: %s\n", *dn)
	return 0
}

// userDeleteCmdImpl handles the user delete subcommand.
func (u *userCmdImpl) userDeleteCmdImpl(args []string) int {
	fs := flag.NewFlagSet("user delete", flag.ContinueOnError)
	fs.SetOutput(u.stderr)

	dn := fs.String("dn", "", "User DN (required)")
	dataDir := fs.String("data-dir", defaultDataDir, "Data directory")
	help := fs.Bool("h", false, "Show help message")
	helpLong := fs.Bool("help", false, "Show help message")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if *help || *helpLong {
		fmt.Fprintln(u.stdout, "Delete a user")
		fmt.Fprintln(u.stdout)
		fmt.Fprintln(u.stdout, "Usage:")
		fmt.Fprintln(u.stdout, "  oba user delete [options]")
		fmt.Fprintln(u.stdout)
		fmt.Fprintln(u.stdout, "Options:")
		fmt.Fprintln(u.stdout, "  -dn string")
		fmt.Fprintln(u.stdout, "        User DN (required)")
		fmt.Fprintln(u.stdout, "  -data-dir string")
		fmt.Fprintf(u.stdout, "        Data directory (default %q)\n", defaultDataDir)
		return 0
	}

	if *dn == "" {
		fmt.Fprintln(u.stderr, "Error: -dn is required")
		return 1
	}

	// Open database
	opts := storage.DefaultEngineOptions().
		WithDataDir(*dataDir).
		WithCreateIfNotExists(false)

	db, err := u.openDB(*dataDir, opts)
	if err != nil {
		fmt.Fprintf(u.stderr, "Error: failed to open database: %v\n", err)
		return 1
	}
	defer db.Close()

	// Begin transaction
	txIface, err := db.Begin()
	if err != nil {
		fmt.Fprintf(u.stderr, "Error: failed to begin transaction: %v\n", err)
		return 1
	}

	// Delete entry
	if err := db.Delete(txIface, *dn); err != nil {
		db.Rollback(txIface)
		if err == engine.ErrEntryNotFound {
			fmt.Fprintf(u.stderr, "Error: user not found: %s\n", *dn)
		} else {
			fmt.Fprintf(u.stderr, "Error: failed to delete user: %v\n", err)
		}
		return 1
	}

	// Commit transaction
	if err := db.Commit(txIface); err != nil {
		fmt.Fprintf(u.stderr, "Error: failed to commit transaction: %v\n", err)
		return 1
	}

	fmt.Fprintf(u.stdout, "User deleted: %s\n", *dn)
	return 0
}

// userListCmdImpl handles the user list subcommand.
func (u *userCmdImpl) userListCmdImpl(args []string) int {
	fs := flag.NewFlagSet("user list", flag.ContinueOnError)
	fs.SetOutput(u.stderr)

	base := fs.String("base", "", "Base DN for search")
	dataDir := fs.String("data-dir", defaultDataDir, "Data directory")
	help := fs.Bool("h", false, "Show help message")
	helpLong := fs.Bool("help", false, "Show help message")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if *help || *helpLong {
		fmt.Fprintln(u.stdout, "List users")
		fmt.Fprintln(u.stdout)
		fmt.Fprintln(u.stdout, "Usage:")
		fmt.Fprintln(u.stdout, "  oba user list [options]")
		fmt.Fprintln(u.stdout)
		fmt.Fprintln(u.stdout, "Options:")
		fmt.Fprintln(u.stdout, "  -base string")
		fmt.Fprintln(u.stdout, "        Base DN for search")
		fmt.Fprintln(u.stdout, "  -data-dir string")
		fmt.Fprintf(u.stdout, "        Data directory (default %q)\n", defaultDataDir)
		return 0
	}

	// Open database
	opts := storage.DefaultEngineOptions().
		WithDataDir(*dataDir).
		WithCreateIfNotExists(false)

	db, err := u.openDB(*dataDir, opts)
	if err != nil {
		fmt.Fprintf(u.stderr, "Error: failed to open database: %v\n", err)
		return 1
	}
	defer db.Close()

	// Search for entries
	iter := db.SearchByDN(nil, *base, storage.ScopeSubtree)
	defer iter.Close()

	count := 0
	for iter.Next() {
		entry := iter.Entry()
		if entry == nil {
			continue
		}

		// Check if entry is a user (has person or inetOrgPerson objectClass)
		if isUserEntry(entry) {
			// Check if account is locked
			locked := isAccountLocked(entry)
			status := ""
			if locked {
				status = " [LOCKED]"
			}
			fmt.Fprintf(u.stdout, "%s%s\n", entry.DN, status)
			count++
		}
	}

	if err := iter.Error(); err != nil {
		fmt.Fprintf(u.stderr, "Error: search failed: %v\n", err)
		return 1
	}

	fmt.Fprintf(u.stdout, "\nTotal: %d user(s)\n", count)
	return 0
}

// isUserEntry checks if an entry is a user entry.
func isUserEntry(entry *storage.Entry) bool {
	objectClasses := entry.GetAttribute(objectClassAttr)
	for _, oc := range objectClasses {
		ocStr := string(oc)
		if ocStr == "person" || ocStr == "inetOrgPerson" || ocStr == "organizationalPerson" {
			return true
		}
	}
	return false
}

// isAccountLocked checks if an account is locked.
func isAccountLocked(entry *storage.Entry) bool {
	lockedTime := entry.GetAttribute(pwdAccountLockedTimeAttr)
	return len(lockedTime) > 0 && len(lockedTime[0]) > 0
}

// userPasswdCmdImpl handles the user passwd subcommand.
func (u *userCmdImpl) userPasswdCmdImpl(args []string) int {
	fs := flag.NewFlagSet("user passwd", flag.ContinueOnError)
	fs.SetOutput(u.stderr)

	dn := fs.String("dn", "", "User DN (required)")
	dataDir := fs.String("data-dir", defaultDataDir, "Data directory")
	help := fs.Bool("h", false, "Show help message")
	helpLong := fs.Bool("help", false, "Show help message")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if *help || *helpLong {
		fmt.Fprintln(u.stdout, "Change user password")
		fmt.Fprintln(u.stdout)
		fmt.Fprintln(u.stdout, "Usage:")
		fmt.Fprintln(u.stdout, "  oba user passwd [options]")
		fmt.Fprintln(u.stdout)
		fmt.Fprintln(u.stdout, "Options:")
		fmt.Fprintln(u.stdout, "  -dn string")
		fmt.Fprintln(u.stdout, "        User DN (required)")
		fmt.Fprintln(u.stdout, "  -data-dir string")
		fmt.Fprintf(u.stdout, "        Data directory (default %q)\n", defaultDataDir)
		return 0
	}

	if *dn == "" {
		fmt.Fprintln(u.stderr, "Error: -dn is required")
		return 1
	}

	// Prompt for new password
	pw, err := u.promptPassword("Enter new password: ")
	if err != nil {
		fmt.Fprintf(u.stderr, "Error reading password: %v\n", err)
		return 1
	}

	confirm, err := u.promptPassword("Confirm new password: ")
	if err != nil {
		fmt.Fprintf(u.stderr, "Error reading password: %v\n", err)
		return 1
	}

	if pw != confirm {
		fmt.Fprintln(u.stderr, "Error: passwords do not match")
		return 1
	}

	// Open database
	opts := storage.DefaultEngineOptions().
		WithDataDir(*dataDir).
		WithCreateIfNotExists(false)

	db, err := u.openDB(*dataDir, opts)
	if err != nil {
		fmt.Fprintf(u.stderr, "Error: failed to open database: %v\n", err)
		return 1
	}
	defer db.Close()

	// Begin transaction
	txIface, err := db.Begin()
	if err != nil {
		fmt.Fprintf(u.stderr, "Error: failed to begin transaction: %v\n", err)
		return 1
	}

	// Get existing entry
	entry, err := db.Get(txIface, *dn)
	if err != nil {
		db.Rollback(txIface)
		if err == engine.ErrEntryNotFound {
			fmt.Fprintf(u.stderr, "Error: user not found: %s\n", *dn)
		} else {
			fmt.Fprintf(u.stderr, "Error: failed to get user: %v\n", err)
		}
		return 1
	}

	// Update password
	entry.SetStringAttribute(userPasswordAttr, hashPassword(pw))

	// Put updated entry
	if err := db.Put(txIface, entry); err != nil {
		db.Rollback(txIface)
		fmt.Fprintf(u.stderr, "Error: failed to update password: %v\n", err)
		return 1
	}

	// Commit transaction
	if err := db.Commit(txIface); err != nil {
		fmt.Fprintf(u.stderr, "Error: failed to commit transaction: %v\n", err)
		return 1
	}

	fmt.Fprintf(u.stdout, "Password changed for: %s\n", *dn)
	return 0
}

// userLockCmdImpl handles the user lock subcommand.
func (u *userCmdImpl) userLockCmdImpl(args []string) int {
	fs := flag.NewFlagSet("user lock", flag.ContinueOnError)
	fs.SetOutput(u.stderr)

	dn := fs.String("dn", "", "User DN (required)")
	dataDir := fs.String("data-dir", defaultDataDir, "Data directory")
	help := fs.Bool("h", false, "Show help message")
	helpLong := fs.Bool("help", false, "Show help message")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if *help || *helpLong {
		fmt.Fprintln(u.stdout, "Lock a user account")
		fmt.Fprintln(u.stdout)
		fmt.Fprintln(u.stdout, "Usage:")
		fmt.Fprintln(u.stdout, "  oba user lock [options]")
		fmt.Fprintln(u.stdout)
		fmt.Fprintln(u.stdout, "Options:")
		fmt.Fprintln(u.stdout, "  -dn string")
		fmt.Fprintln(u.stdout, "        User DN (required)")
		fmt.Fprintln(u.stdout, "  -data-dir string")
		fmt.Fprintf(u.stdout, "        Data directory (default %q)\n", defaultDataDir)
		return 0
	}

	if *dn == "" {
		fmt.Fprintln(u.stderr, "Error: -dn is required")
		return 1
	}

	// Open database
	opts := storage.DefaultEngineOptions().
		WithDataDir(*dataDir).
		WithCreateIfNotExists(false)

	db, err := u.openDB(*dataDir, opts)
	if err != nil {
		fmt.Fprintf(u.stderr, "Error: failed to open database: %v\n", err)
		return 1
	}
	defer db.Close()

	// Begin transaction
	txIface, err := db.Begin()
	if err != nil {
		fmt.Fprintf(u.stderr, "Error: failed to begin transaction: %v\n", err)
		return 1
	}

	// Get existing entry
	entry, err := db.Get(txIface, *dn)
	if err != nil {
		db.Rollback(txIface)
		if err == engine.ErrEntryNotFound {
			fmt.Fprintf(u.stderr, "Error: user not found: %s\n", *dn)
		} else {
			fmt.Fprintf(u.stderr, "Error: failed to get user: %v\n", err)
		}
		return 1
	}

	// Check if already locked
	if isAccountLocked(entry) {
		db.Rollback(txIface)
		fmt.Fprintf(u.stderr, "Error: user is already locked: %s\n", *dn)
		return 1
	}

	// Set lock time (LDAP generalized time format)
	lockTime := time.Now().UTC().Format("20060102150405Z")
	entry.SetStringAttribute(pwdAccountLockedTimeAttr, lockTime)

	// Put updated entry
	if err := db.Put(txIface, entry); err != nil {
		db.Rollback(txIface)
		fmt.Fprintf(u.stderr, "Error: failed to lock user: %v\n", err)
		return 1
	}

	// Commit transaction
	if err := db.Commit(txIface); err != nil {
		fmt.Fprintf(u.stderr, "Error: failed to commit transaction: %v\n", err)
		return 1
	}

	fmt.Fprintf(u.stdout, "User locked: %s\n", *dn)
	return 0
}

// userUnlockCmdImpl handles the user unlock subcommand.
func (u *userCmdImpl) userUnlockCmdImpl(args []string) int {
	fs := flag.NewFlagSet("user unlock", flag.ContinueOnError)
	fs.SetOutput(u.stderr)

	dn := fs.String("dn", "", "User DN (required)")
	dataDir := fs.String("data-dir", defaultDataDir, "Data directory")
	help := fs.Bool("h", false, "Show help message")
	helpLong := fs.Bool("help", false, "Show help message")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if *help || *helpLong {
		fmt.Fprintln(u.stdout, "Unlock a user account")
		fmt.Fprintln(u.stdout)
		fmt.Fprintln(u.stdout, "Usage:")
		fmt.Fprintln(u.stdout, "  oba user unlock [options]")
		fmt.Fprintln(u.stdout)
		fmt.Fprintln(u.stdout, "Options:")
		fmt.Fprintln(u.stdout, "  -dn string")
		fmt.Fprintln(u.stdout, "        User DN (required)")
		fmt.Fprintln(u.stdout, "  -data-dir string")
		fmt.Fprintf(u.stdout, "        Data directory (default %q)\n", defaultDataDir)
		return 0
	}

	if *dn == "" {
		fmt.Fprintln(u.stderr, "Error: -dn is required")
		return 1
	}

	// Open database
	opts := storage.DefaultEngineOptions().
		WithDataDir(*dataDir).
		WithCreateIfNotExists(false)

	db, err := u.openDB(*dataDir, opts)
	if err != nil {
		fmt.Fprintf(u.stderr, "Error: failed to open database: %v\n", err)
		return 1
	}
	defer db.Close()

	// Begin transaction
	txIface, err := db.Begin()
	if err != nil {
		fmt.Fprintf(u.stderr, "Error: failed to begin transaction: %v\n", err)
		return 1
	}

	// Get existing entry
	entry, err := db.Get(txIface, *dn)
	if err != nil {
		db.Rollback(txIface)
		if err == engine.ErrEntryNotFound {
			fmt.Fprintf(u.stderr, "Error: user not found: %s\n", *dn)
		} else {
			fmt.Fprintf(u.stderr, "Error: failed to get user: %v\n", err)
		}
		return 1
	}

	// Check if not locked
	if !isAccountLocked(entry) {
		db.Rollback(txIface)
		fmt.Fprintf(u.stderr, "Error: user is not locked: %s\n", *dn)
		return 1
	}

	// Remove lock time attribute
	delete(entry.Attributes, pwdAccountLockedTimeAttr)

	// Put updated entry
	if err := db.Put(txIface, entry); err != nil {
		db.Rollback(txIface)
		fmt.Fprintf(u.stderr, "Error: failed to unlock user: %v\n", err)
		return 1
	}

	// Commit transaction
	if err := db.Commit(txIface); err != nil {
		fmt.Fprintf(u.stderr, "Error: failed to commit transaction: %v\n", err)
		return 1
	}

	fmt.Fprintf(u.stdout, "User unlocked: %s\n", *dn)
	return 0
}
