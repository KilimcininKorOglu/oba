// Package config provides configuration parsing and management for the Oba LDAP server.
//
// # Overview
//
// The config package handles loading, parsing, and validating server configuration
// from YAML files and environment variables. It supports:
//
//   - YAML configuration files
//   - Environment variable overrides
//   - Default values for all settings
//   - Configuration validation
//
// # Configuration Structure
//
// The main Config struct contains all server settings:
//
//	type Config struct {
//	    Server    ServerConfig    // Network settings
//	    Directory DirectoryConfig // LDAP directory settings
//	    Storage   StorageConfig   // Storage engine settings
//	    Logging   LogConfig       // Logging settings
//	    Security  SecurityConfig  // Security settings
//	    ACL       ACLConfig       // Access control settings
//	}
//
// # Loading Configuration
//
// Load configuration from a YAML file:
//
//	cfg, err := config.Load("/etc/oba/config.yaml")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// Or use defaults:
//
//	cfg := config.Default()
//
// # Environment Variables
//
// Configuration values can be overridden with environment variables using
// the pattern OBA_<SECTION>_<KEY>:
//
//	OBA_SERVER_ADDRESS=:1389
//	OBA_DIRECTORY_ROOTPASSWORD=secret
//	OBA_LOGGING_LEVEL=debug
//
// # Example Configuration
//
// A typical configuration file:
//
//	server:
//	  address: ":389"
//	  tlsAddress: ":636"
//	  tlsCert: "/etc/oba/certs/server.crt"
//	  tlsKey: "/etc/oba/certs/server.key"
//	  maxConnections: 10000
//	  readTimeout: 30s
//	  writeTimeout: 30s
//
//	directory:
//	  baseDN: "dc=example,dc=com"
//	  rootDN: "cn=admin,dc=example,dc=com"
//	  rootPassword: "${OBA_ROOT_PASSWORD}"
//
//	storage:
//	  dataDir: "/var/lib/oba"
//	  walDir: "/var/lib/oba/wal"
//	  pageSize: 4096
//	  bufferPoolSize: "256MB"
//	  checkpointInterval: 5m
//
//	logging:
//	  level: "info"
//	  format: "json"
//	  output: "/var/log/oba/oba.log"
//
//	security:
//	  passwordPolicy:
//	    enabled: true
//	    minLength: 8
//	    requireUppercase: true
//	    requireLowercase: true
//	    requireDigit: true
//
//	acl:
//	  defaultPolicy: "deny"
//	  rules:
//	    - target: "*"
//	      subject: "cn=admin,dc=example,dc=com"
//	      rights: ["read", "write", "add", "delete"]
package config
