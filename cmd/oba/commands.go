// Package main provides CLI commands for the oba LDAP server.
package main

import (
	"flag"
	"fmt"
	"os"
)

// backupCmd handles the backup command.
func backupCmd(args []string) int {
	fs := flag.NewFlagSet("backup", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	output := fs.String("output", "", "Output file path")
	compress := fs.Bool("compress", false, "Compress backup file")
	incremental := fs.Bool("incremental", false, "Create incremental backup")
	format := fs.String("format", "native", "Backup format: native, ldif")
	help := fs.Bool("h", false, "Show help message")
	helpLong := fs.Bool("help", false, "Show help message")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if *help || *helpLong {
		printBackupUsage(os.Stdout)
		return 0
	}

	if *output == "" {
		fmt.Fprintln(os.Stderr, "Error: -output is required")
		return 1
	}

	// TODO: Implement backup logic
	fmt.Printf("Creating backup...\n")
	fmt.Printf("  Output:      %s\n", *output)
	fmt.Printf("  Compress:    %v\n", *compress)
	fmt.Printf("  Incremental: %v\n", *incremental)
	fmt.Printf("  Format:      %s\n", *format)

	fmt.Println("Backup implementation pending...")
	return 0
}

// restoreCmd handles the restore command.
func restoreCmd(args []string) int {
	fs := flag.NewFlagSet("restore", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	input := fs.String("input", "", "Input backup file path")
	verify := fs.Bool("verify", false, "Verify checksums before restore")
	format := fs.String("format", "native", "Backup format: native, ldif")
	help := fs.Bool("h", false, "Show help message")
	helpLong := fs.Bool("help", false, "Show help message")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if *help || *helpLong {
		printRestoreUsage(os.Stdout)
		return 0
	}

	if *input == "" {
		fmt.Fprintln(os.Stderr, "Error: -input is required")
		return 1
	}

	// TODO: Implement restore logic
	fmt.Printf("Restoring from backup...\n")
	fmt.Printf("  Input:  %s\n", *input)
	fmt.Printf("  Verify: %v\n", *verify)
	fmt.Printf("  Format: %s\n", *format)

	fmt.Println("Restore implementation pending...")
	return 0
}

// userCmd handles the user command.
func userCmd(args []string) int {
	if len(args) == 0 {
		printUserUsage(os.Stdout)
		return 0
	}

	// Check for help flags
	if args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		printUserUsage(os.Stdout)
		return 0
	}

	switch args[0] {
	case "add":
		return userAddCmd(args[1:])
	case "delete":
		return userDeleteCmd(args[1:])
	case "passwd":
		return userPasswdCmd(args[1:])
	case "list":
		return userListCmd(args[1:])
	case "lock":
		return userLockCmd(args[1:])
	case "unlock":
		return userUnlockCmd(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "Unknown user subcommand: %s\n", args[0])
		fmt.Fprintln(os.Stderr, "Run 'oba user help' for usage.")
		return 1
	}
}

// userAddCmd handles the user add subcommand.
func userAddCmd(args []string) int {
	impl := newUserCmdImpl()
	return impl.userAddCmdImpl(args)
}

// userDeleteCmd handles the user delete subcommand.
func userDeleteCmd(args []string) int {
	impl := newUserCmdImpl()
	return impl.userDeleteCmdImpl(args)
}

// userPasswdCmd handles the user passwd subcommand.
func userPasswdCmd(args []string) int {
	impl := newUserCmdImpl()
	return impl.userPasswdCmdImpl(args)
}

// userListCmd handles the user list subcommand.
func userListCmd(args []string) int {
	impl := newUserCmdImpl()
	return impl.userListCmdImpl(args)
}

// userLockCmd handles the user lock subcommand.
func userLockCmd(args []string) int {
	impl := newUserCmdImpl()
	return impl.userLockCmdImpl(args)
}

// userUnlockCmd handles the user unlock subcommand.
func userUnlockCmd(args []string) int {
	impl := newUserCmdImpl()
	return impl.userUnlockCmdImpl(args)
}

// valueOrDefault returns the value if non-empty, otherwise returns the default.
func valueOrDefault(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}
