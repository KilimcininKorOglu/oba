package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
)

// Version information - these can be set at build time using ldflags.
// Example: go build -ldflags "-X main.version=1.0.0 -X main.commit=abc123"
var (
	version   = "1.0.1"
	commit    = "unknown"
	buildDate = "unknown"
)

// versionCmd handles the version command.
func versionCmd(args []string) int {
	fs := flag.NewFlagSet("version", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	short := fs.Bool("short", false, "Show only version number")
	help := fs.Bool("h", false, "Show help message")
	helpLong := fs.Bool("help", false, "Show help message")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if *help || *helpLong {
		printVersionUsage(os.Stdout)
		return 0
	}

	if *short {
		fmt.Println(version)
		return 0
	}

	fmt.Printf("oba version %s\n", version)
	fmt.Printf("  Commit:     %s\n", commit)
	fmt.Printf("  Built:      %s\n", buildDate)
	fmt.Printf("  Go version: %s\n", runtime.Version())
	fmt.Printf("  OS/Arch:    %s/%s\n", runtime.GOOS, runtime.GOARCH)

	return 0
}

// GetVersion returns the current version string.
func GetVersion() string {
	return version
}

// GetCommit returns the current commit hash.
func GetCommit() string {
	return commit
}

// GetBuildDate returns the build date.
func GetBuildDate() string {
	return buildDate
}
