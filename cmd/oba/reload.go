// Package main provides the reload command for the oba LDAP server.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"syscall"
)

// reloadCmd handles the reload command.
func reloadCmd(args []string) int {
	fs := flag.NewFlagSet("reload", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	pidFile := fs.String("pid-file", "/var/run/oba.pid", "Path to PID file")
	help := fs.Bool("h", false, "Show help message")
	helpLong := fs.Bool("help", false, "Show help message")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if *help || *helpLong {
		printReloadUsage(os.Stdout)
		return 0
	}

	// Get component to reload
	remaining := fs.Args()
	if len(remaining) < 1 {
		fmt.Fprintln(os.Stderr, "Error: component name required")
		fmt.Fprintln(os.Stderr, "Usage: oba reload <component>")
		fmt.Fprintln(os.Stderr, "Components: acl")
		return 1
	}

	component := remaining[0]

	switch component {
	case "acl":
		return reloadACL(*pidFile)
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown component: %s\n", component)
		fmt.Fprintln(os.Stderr, "Supported components: acl")
		return 1
	}
}

// reloadACL sends SIGHUP to the oba process to reload ACL.
func reloadACL(pidFile string) int {
	// Check environment variable override
	if envPid := os.Getenv("OBA_PID_FILE"); envPid != "" {
		pidFile = envPid
	}

	// Read PID from file
	data, err := os.ReadFile(pidFile)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Error: PID file not found: %s\n", pidFile)
			fmt.Fprintln(os.Stderr, "Is the oba server running?")
		} else {
			fmt.Fprintf(os.Stderr, "Error: failed to read PID file: %v\n", err)
		}
		return 1
	}

	// Parse PID
	pidStr := string(data)
	// Trim whitespace
	for len(pidStr) > 0 && (pidStr[len(pidStr)-1] == '\n' || pidStr[len(pidStr)-1] == '\r' || pidStr[len(pidStr)-1] == ' ') {
		pidStr = pidStr[:len(pidStr)-1]
	}

	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid PID in %s: %s\n", pidFile, pidStr)
		return 1
	}

	// Find process
	process, err := os.FindProcess(pid)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to find process %d: %v\n", pid, err)
		return 1
	}

	// Send SIGHUP
	if err := process.Signal(syscall.SIGHUP); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to send SIGHUP to process %d: %v\n", pid, err)
		return 1
	}

	fmt.Printf("Sent SIGHUP to oba process (PID %d)\n", pid)
	fmt.Println("Check server logs for reload status")
	return 0
}

// printReloadUsage prints the reload command usage.
func printReloadUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: oba reload <component> [options]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Reload server configuration without restart.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Components:")
	fmt.Fprintln(w, "  acl         Reload ACL configuration from file")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Options:")
	fmt.Fprintln(w, "  -pid-file   Path to PID file (default: /var/run/oba.pid)")
	fmt.Fprintln(w, "  -h, -help   Show this help message")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Environment Variables:")
	fmt.Fprintln(w, "  OBA_PID_FILE  Override PID file path")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintln(w, "  oba reload acl")
	fmt.Fprintln(w, "  oba reload acl -pid-file /tmp/oba.pid")
}
