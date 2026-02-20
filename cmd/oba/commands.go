// Package main provides CLI commands for the oba LDAP server.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/oba-ldap/oba/internal/backup"
	"github.com/oba-ldap/oba/internal/storage"
)

// backupCmd handles the backup command.
func backupCmd(args []string) int {
	fs := flag.NewFlagSet("backup", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	output := fs.String("output", "", "Output file path")
	dataDir := fs.String("data-dir", "", "Data directory path")
	compress := fs.Bool("compress", false, "Compress backup file")
	incremental := fs.Bool("incremental", false, "Create incremental backup")
	format := fs.String("format", "native", "Backup format: native, ldif")
	baseDN := fs.String("base-dn", "", "Base DN for LDIF export (optional)")
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

	if *dataDir == "" {
		fmt.Fprintln(os.Stderr, "Error: -data-dir is required")
		return 1
	}

	// Open page manager for backup
	pmOpts := storage.Options{
		PageSize:    4096,
		ReadOnly:    true,
		CreateIfNew: false,
	}
	dataFile := *dataDir + "/data.oba"
	pm, err := storage.OpenPageManager(dataFile, pmOpts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening data file: %v\n", err)
		return 1
	}
	defer pm.Close()

	// Create backup manager
	bm := backup.NewBackupManager(pm)

	// Configure backup options
	backupOpts := &backup.BackupOptions{
		OutputPath:  *output,
		Compress:    *compress,
		Incremental: *incremental,
		Format:      backup.BackupFormat(*format),
		BaseDN:      *baseDN,
	}

	fmt.Printf("Creating backup...\n")
	fmt.Printf("  Output:      %s\n", *output)
	fmt.Printf("  Data Dir:    %s\n", *dataDir)
	fmt.Printf("  Compress:    %v\n", *compress)
	fmt.Printf("  Incremental: %v\n", *incremental)
	fmt.Printf("  Format:      %s\n", *format)

	startTime := time.Now()
	stats, err := bm.Backup(backupOpts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Backup failed: %v\n", err)
		return 1
	}

	fmt.Printf("\nBackup completed successfully!\n")
	fmt.Printf("  Total pages: %d\n", stats.TotalPages)
	fmt.Printf("  Total bytes: %d\n", stats.TotalBytes)
	if *compress && stats.CompressedBytes > 0 {
		fmt.Printf("  Compressed:  %d (%.1f%% reduction)\n", stats.CompressedBytes, stats.CompressionRatio()*100)
	}
	fmt.Printf("  Duration:    %v\n", time.Since(startTime).Round(time.Millisecond))

	return 0
}

// restoreCmd handles the restore command.
func restoreCmd(args []string) int {
	fs := flag.NewFlagSet("restore", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	input := fs.String("input", "", "Input backup file path")
	dataDir := fs.String("data-dir", "", "Target data directory path")
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

	if *dataDir == "" {
		fmt.Fprintln(os.Stderr, "Error: -data-dir is required")
		return 1
	}

	// Create restore manager
	rm := backup.NewRestoreManager(*dataDir)

	// Configure restore options
	opts := &backup.RestoreOptions{
		InputPath: *input,
		Verify:    *verify,
		Format:    backup.BackupFormat(*format),
		DataDir:   *dataDir,
	}

	fmt.Printf("Restoring from backup...\n")
	fmt.Printf("  Input:    %s\n", *input)
	fmt.Printf("  Data Dir: %s\n", *dataDir)
	fmt.Printf("  Verify:   %v\n", *verify)
	fmt.Printf("  Format:   %s\n", *format)

	startTime := time.Now()
	stats, err := rm.Restore(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Restore failed: %v\n", err)
		return 1
	}

	fmt.Printf("\nRestore completed successfully!\n")
	fmt.Printf("  Backup type:     %s\n", stats.BackupType)
	fmt.Printf("  Total pages:     %d\n", stats.TotalPages)
	fmt.Printf("  Total bytes:     %d\n", stats.TotalBytes)
	fmt.Printf("  Backups applied: %d\n", stats.BackupsApplied)
	fmt.Printf("  Duration:        %v\n", time.Since(startTime).Round(time.Millisecond))

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
