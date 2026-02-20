package main

import (
	"fmt"
	"io"
)

// printUsage prints the main usage information to the given writer.
func printUsage(w io.Writer) {
	fmt.Fprint(w, `oba - Zero-dependency LDAP server

Usage:
  oba <command> [options]

Commands:
  serve       Start the LDAP server
  backup      Create database backup
  restore     Restore from backup
  user        User management
  config      Configuration management
  version     Show version information

Use "oba <command> -h" for more information about a command.
`)
}

// printServeUsage prints the serve command usage.
func printServeUsage(w io.Writer) {
	fmt.Fprint(w, `Start the LDAP server

Usage:
  oba serve [options]

Options:
  -config string
        Path to configuration file
  -address string
        Listen address (overrides config, default ":389")
  -tls-address string
        TLS listen address (overrides config, default ":636")
  -data-dir string
        Data directory path (overrides config, default "/var/lib/oba")
  -log-level string
        Log level: debug, info, warn, error (overrides config)
  -h, -help
        Show this help message

Environment Variables:
  OBA_SERVER_ADDRESS       Override server listen address
  OBA_SERVER_TLS_ADDRESS   Override TLS listen address
  OBA_STORAGE_DATA_DIR     Override data directory path
  OBA_LOGGING_LEVEL        Override log level
`)
}

// printBackupUsage prints the backup command usage.
func printBackupUsage(w io.Writer) {
	fmt.Fprint(w, `Create database backup

Usage:
  oba backup [options]

Options:
  -output string
        Output file path (required)
  -compress
        Compress backup file
  -incremental
        Create incremental backup
  -format string
        Backup format: native, ldif (default "native")
  -h, -help
        Show this help message
`)
}

// printRestoreUsage prints the restore command usage.
func printRestoreUsage(w io.Writer) {
	fmt.Fprint(w, `Restore from backup

Usage:
  oba restore [options]

Options:
  -input string
        Input backup file path (required)
  -verify
        Verify checksums before restore
  -format string
        Backup format: native, ldif (default "native")
  -h, -help
        Show this help message
`)
}

// printUserUsage prints the user command usage.
func printUserUsage(w io.Writer) {
	fmt.Fprint(w, `User management

Usage:
  oba user <subcommand> [options]

Subcommands:
  add         Add a new user
  delete      Delete a user
  passwd      Change user password
  list        List users
  lock        Lock a user account
  unlock      Unlock a user account

Use "oba user <subcommand> -h" for more information.
`)
}

// printConfigUsage prints the config command usage.
func printConfigUsage(w io.Writer) {
	fmt.Fprint(w, `Configuration management

Usage:
  oba config <subcommand> [options]

Subcommands:
  validate    Validate configuration file
  init        Generate default configuration
  show        Show effective configuration

Use "oba config <subcommand> -h" for more information.
`)
}

// printVersionUsage prints the version command usage.
func printVersionUsage(w io.Writer) {
	fmt.Fprint(w, `Show version information

Usage:
  oba version [options]

Options:
  -short
        Show only version number
  -h, -help
        Show this help message
`)
}
