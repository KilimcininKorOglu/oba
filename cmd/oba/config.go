package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/oba-ldap/oba/internal/config"
)

// configCmd handles the config command.
func configCmd(args []string) int {
	if len(args) == 0 {
		printConfigUsage(os.Stdout)
		return 0
	}

	// Check for help flags
	if args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		printConfigUsage(os.Stdout)
		return 0
	}

	switch args[0] {
	case "validate":
		return configValidateCmd(args[1:])
	case "init":
		return configInitCmd(args[1:])
	case "show":
		return configShowCmd(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "Unknown config subcommand: %s\n", args[0])
		fmt.Fprintln(os.Stderr, "Run 'oba config help' for usage.")
		return 1
	}
}

// configValidateCmd handles the config validate subcommand.
func configValidateCmd(args []string) int {
	fs := flag.NewFlagSet("config validate", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	configFile := fs.String("config", "", "Path to configuration file")
	help := fs.Bool("h", false, "Show help message")
	helpLong := fs.Bool("help", false, "Show help message")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if *help || *helpLong {
		fmt.Println("Validate configuration file")
		fmt.Println()
		fmt.Println("Usage:")
		fmt.Println("  oba config validate [options]")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  -config string")
		fmt.Println("        Path to configuration file (required)")
		return 0
	}

	if *configFile == "" {
		fmt.Fprintln(os.Stderr, "Error: -config is required")
		return 1
	}

	// Load the configuration file
	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid configuration: %v\n", err)
		return 1
	}

	// Validate the configuration
	errs := config.ValidateConfig(cfg)
	if len(errs) > 0 {
		fmt.Fprintln(os.Stderr, "Configuration errors:")
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "  - %s\n", e)
		}
		return 1
	}

	fmt.Println("Configuration is valid")
	return 0
}

// configInitCmd handles the config init subcommand.
func configInitCmd(args []string) int {
	fs := flag.NewFlagSet("config init", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	help := fs.Bool("h", false, "Show help message")
	helpLong := fs.Bool("help", false, "Show help message")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if *help || *helpLong {
		fmt.Println("Generate default configuration")
		fmt.Println()
		fmt.Println("Usage:")
		fmt.Println("  oba config init")
		fmt.Println()
		fmt.Println("Outputs default configuration to stdout in YAML format.")
		return 0
	}

	// Get default configuration
	cfg := config.DefaultConfig()

	// Marshal to YAML
	yaml := marshalConfigToYAML(cfg)
	fmt.Print(yaml)

	return 0
}

// configShowCmd handles the config show subcommand.
func configShowCmd(args []string) int {
	fs := flag.NewFlagSet("config show", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	configFile := fs.String("config", "", "Path to configuration file")
	format := fs.String("format", "yaml", "Output format (yaml, json)")
	help := fs.Bool("h", false, "Show help message")
	helpLong := fs.Bool("help", false, "Show help message")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if *help || *helpLong {
		fmt.Println("Show effective configuration")
		fmt.Println()
		fmt.Println("Usage:")
		fmt.Println("  oba config show [options]")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  -config string")
		fmt.Println("        Path to configuration file")
		fmt.Println("  -format string")
		fmt.Println("        Output format: yaml, json (default \"yaml\")")
		return 0
	}

	var cfg *config.Config
	var err error

	if *configFile != "" {
		cfg, err = config.LoadConfig(*configFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
			return 1
		}
	} else {
		cfg = config.DefaultConfig()
	}

	// Apply environment variable overrides
	applyEnvOverrides(cfg)

	// Output in requested format
	switch strings.ToLower(*format) {
	case "json":
		data, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to marshal config: %v\n", err)
			return 1
		}
		fmt.Println(string(data))
	default:
		yaml := marshalConfigToYAML(cfg)
		fmt.Print(yaml)
	}

	return 0
}

// applyEnvOverrides applies environment variable overrides to the configuration.
// Environment variables follow the pattern OBA_<SECTION>_<KEY>.
func applyEnvOverrides(cfg *config.Config) {
	// Server overrides
	if v := os.Getenv("OBA_SERVER_ADDRESS"); v != "" {
		cfg.Server.Address = v
	}
	if v := os.Getenv("OBA_SERVER_TLS_ADDRESS"); v != "" {
		cfg.Server.TLSAddress = v
	}
	if v := os.Getenv("OBA_SERVER_TLS_CERT"); v != "" {
		cfg.Server.TLSCert = v
	}
	if v := os.Getenv("OBA_SERVER_TLS_KEY"); v != "" {
		cfg.Server.TLSKey = v
	}

	// Directory overrides
	if v := os.Getenv("OBA_DIRECTORY_BASE_DN"); v != "" {
		cfg.Directory.BaseDN = v
	}
	if v := os.Getenv("OBA_DIRECTORY_ROOT_DN"); v != "" {
		cfg.Directory.RootDN = v
	}
	if v := os.Getenv("OBA_DIRECTORY_ROOT_PASSWORD"); v != "" {
		cfg.Directory.RootPassword = v
	}

	// Storage overrides
	if v := os.Getenv("OBA_STORAGE_DATA_DIR"); v != "" {
		cfg.Storage.DataDir = v
	}
	if v := os.Getenv("OBA_STORAGE_WAL_DIR"); v != "" {
		cfg.Storage.WALDir = v
	}

	// Logging overrides
	if v := os.Getenv("OBA_LOGGING_LEVEL"); v != "" {
		cfg.Logging.Level = v
	}
	if v := os.Getenv("OBA_LOGGING_FORMAT"); v != "" {
		cfg.Logging.Format = v
	}
	if v := os.Getenv("OBA_LOGGING_OUTPUT"); v != "" {
		cfg.Logging.Output = v
	}
}

// marshalConfigToYAML converts a Config to YAML format.
// This is a simple implementation since we can't use external YAML libraries.
func marshalConfigToYAML(cfg *config.Config) string {
	var sb strings.Builder

	sb.WriteString("# Oba LDAP Server Configuration\n")
	sb.WriteString("# Generated by: oba config init\n\n")

	// Server section
	sb.WriteString("server:\n")
	sb.WriteString(fmt.Sprintf("  address: %q\n", cfg.Server.Address))
	sb.WriteString(fmt.Sprintf("  tlsAddress: %q\n", cfg.Server.TLSAddress))
	if cfg.Server.TLSCert != "" {
		sb.WriteString(fmt.Sprintf("  tlsCert: %q\n", cfg.Server.TLSCert))
	}
	if cfg.Server.TLSKey != "" {
		sb.WriteString(fmt.Sprintf("  tlsKey: %q\n", cfg.Server.TLSKey))
	}
	sb.WriteString(fmt.Sprintf("  maxConnections: %d\n", cfg.Server.MaxConnections))
	sb.WriteString(fmt.Sprintf("  readTimeout: %s\n", formatDuration(cfg.Server.ReadTimeout)))
	sb.WriteString(fmt.Sprintf("  writeTimeout: %s\n", formatDuration(cfg.Server.WriteTimeout)))
	sb.WriteString("\n")

	// Directory section
	sb.WriteString("directory:\n")
	sb.WriteString(fmt.Sprintf("  baseDN: %q\n", cfg.Directory.BaseDN))
	sb.WriteString(fmt.Sprintf("  rootDN: %q\n", cfg.Directory.RootDN))
	sb.WriteString(fmt.Sprintf("  rootPassword: %q\n", cfg.Directory.RootPassword))
	sb.WriteString("\n")

	// Storage section
	sb.WriteString("storage:\n")
	sb.WriteString(fmt.Sprintf("  dataDir: %q\n", cfg.Storage.DataDir))
	if cfg.Storage.WALDir != "" {
		sb.WriteString(fmt.Sprintf("  walDir: %q\n", cfg.Storage.WALDir))
	}
	sb.WriteString(fmt.Sprintf("  pageSize: %d\n", cfg.Storage.PageSize))
	sb.WriteString(fmt.Sprintf("  bufferPoolSize: %q\n", cfg.Storage.BufferPoolSize))
	sb.WriteString(fmt.Sprintf("  checkpointInterval: %s\n", formatDuration(cfg.Storage.CheckpointInterval)))
	sb.WriteString("\n")

	// Logging section
	sb.WriteString("logging:\n")
	sb.WriteString(fmt.Sprintf("  level: %q\n", cfg.Logging.Level))
	sb.WriteString(fmt.Sprintf("  format: %q\n", cfg.Logging.Format))
	sb.WriteString(fmt.Sprintf("  output: %q\n", cfg.Logging.Output))
	sb.WriteString("\n")

	// Security section
	sb.WriteString("security:\n")
	sb.WriteString("  passwordPolicy:\n")
	sb.WriteString(fmt.Sprintf("    enabled: %t\n", cfg.Security.PasswordPolicy.Enabled))
	sb.WriteString(fmt.Sprintf("    minLength: %d\n", cfg.Security.PasswordPolicy.MinLength))
	sb.WriteString(fmt.Sprintf("    requireUppercase: %t\n", cfg.Security.PasswordPolicy.RequireUppercase))
	sb.WriteString(fmt.Sprintf("    requireLowercase: %t\n", cfg.Security.PasswordPolicy.RequireLowercase))
	sb.WriteString(fmt.Sprintf("    requireDigit: %t\n", cfg.Security.PasswordPolicy.RequireDigit))
	sb.WriteString(fmt.Sprintf("    requireSpecial: %t\n", cfg.Security.PasswordPolicy.RequireSpecial))
	if cfg.Security.PasswordPolicy.MaxAge > 0 {
		sb.WriteString(fmt.Sprintf("    maxAge: %s\n", formatDuration(cfg.Security.PasswordPolicy.MaxAge)))
	}
	sb.WriteString(fmt.Sprintf("    historyCount: %d\n", cfg.Security.PasswordPolicy.HistoryCount))
	sb.WriteString("  rateLimit:\n")
	sb.WriteString(fmt.Sprintf("    enabled: %t\n", cfg.Security.RateLimit.Enabled))
	sb.WriteString(fmt.Sprintf("    maxAttempts: %d\n", cfg.Security.RateLimit.MaxAttempts))
	sb.WriteString(fmt.Sprintf("    lockoutDuration: %s\n", formatDuration(cfg.Security.RateLimit.LockoutDuration)))
	sb.WriteString("\n")

	// ACL section
	sb.WriteString("acl:\n")
	sb.WriteString(fmt.Sprintf("  defaultPolicy: %q\n", cfg.ACL.DefaultPolicy))
	if len(cfg.ACL.Rules) > 0 {
		sb.WriteString("  rules:\n")
		for _, rule := range cfg.ACL.Rules {
			sb.WriteString(fmt.Sprintf("    - target: %q\n", rule.Target))
			sb.WriteString(fmt.Sprintf("      subject: %q\n", rule.Subject))
			if len(rule.Rights) > 0 {
				sb.WriteString("      rights:\n")
				for _, right := range rule.Rights {
					sb.WriteString(fmt.Sprintf("        - %q\n", right))
				}
			}
			if len(rule.Attributes) > 0 {
				sb.WriteString("      attributes:\n")
				for _, attr := range rule.Attributes {
					sb.WriteString(fmt.Sprintf("        - %q\n", attr))
				}
			}
		}
	}

	return sb.String()
}

// formatDuration formats a duration for YAML output.
func formatDuration(d time.Duration) string {
	if d == 0 {
		return "0s"
	}

	// Convert to days if applicable
	days := d / (24 * time.Hour)
	if days > 0 && d%(24*time.Hour) == 0 {
		return fmt.Sprintf("%dd", days)
	}

	// Use standard duration format
	return d.String()
}
