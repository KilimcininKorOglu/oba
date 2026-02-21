// Package main provides the entry point for the oba LDAP server CLI.
package main

import (
	"fmt"
	"os"
)

func main() {
	exitCode := run(os.Args)
	os.Exit(exitCode)
}

// run executes the CLI and returns an exit code.
// This is separated from main() to facilitate testing.
func run(args []string) int {
	if len(args) < 2 {
		printUsage(os.Stdout)
		return 1
	}

	switch args[1] {
	case "serve":
		return serveCmd(args[2:])
	case "backup":
		return backupCmd(args[2:])
	case "restore":
		return restoreCmd(args[2:])
	case "user":
		return userCmd(args[2:])
	case "config":
		return configCmd(args[2:])
	case "reload":
		return reloadCmd(args[2:])
	case "version":
		return versionCmd(args[2:])
	case "help", "-h", "--help":
		printUsage(os.Stdout)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", args[1])
		fmt.Fprintln(os.Stderr, "Run 'oba help' for usage.")
		return 1
	}
}
