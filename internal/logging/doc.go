// Package logging provides structured logging for the Oba LDAP server.
//
// # Overview
//
// The logging package provides a structured logging interface with support for:
//
//   - Multiple log levels (debug, info, warn, error)
//   - Text and JSON output formats
//   - Request ID tracking for distributed tracing
//   - Field-based contextual logging
//
// # Creating a Logger
//
// Create a logger with configuration:
//
//	logger := logging.New(logging.Config{
//	    Level:  "info",
//	    Format: "json",
//	    Output: "/var/log/oba/oba.log",
//	})
//
// Or use defaults:
//
//	logger := logging.NewDefault() // Info level, text format, stdout
//
// For testing, use a no-op logger:
//
//	logger := logging.NewNop()
//
// # Log Levels
//
// Four log levels are supported:
//
//	logger.Debug("detailed debugging info", "key", "value")
//	logger.Info("informational message", "key", "value")
//	logger.Warn("warning message", "key", "value")
//	logger.Error("error message", "key", "value")
//
// Parse level from string:
//
//	level := logging.ParseLevel("debug") // Returns LevelDebug
//
// # Structured Logging
//
// Add key-value pairs to log entries:
//
//	logger.Info("bind successful",
//	    "dn", "uid=alice,ou=users,dc=example,dc=com",
//	    "client", "192.168.1.100:54321",
//	    "duration_ms", 2,
//	)
//
// Output (JSON format):
//
//	{
//	    "ts": "2026-02-18T10:30:00Z",
//	    "level": "info",
//	    "msg": "bind successful",
//	    "dn": "uid=alice,ou=users,dc=example,dc=com",
//	    "client": "192.168.1.100:54321",
//	    "duration_ms": 2
//	}
//
// # Request ID Tracking
//
// Add request ID for tracing:
//
//	requestID := logging.GenerateRequestID()
//	connLogger := logger.WithRequestID(requestID)
//
//	connLogger.Info("processing request") // Includes request_id field
//
// # Contextual Fields
//
// Create loggers with persistent fields:
//
//	connLogger := logger.WithFields(
//	    "client", conn.RemoteAddr().String(),
//	    "connection_id", connID,
//	)
//
//	// All subsequent logs include these fields
//	connLogger.Info("bind request received")
//	connLogger.Info("bind successful")
//
// # Output Formats
//
// Text format (human-readable):
//
//	2026-02-18T10:30:00Z [info] bind successful dn=uid=alice,... duration_ms=2
//
// JSON format (machine-parseable):
//
//	{"ts":"2026-02-18T10:30:00Z","level":"info","msg":"bind successful",...}
//
// # Output Destinations
//
// Configure output destination:
//
//	logging.Config{Output: "stdout"}           // Standard output
//	logging.Config{Output: "stderr"}           // Standard error
//	logging.Config{Output: "/var/log/oba.log"} // File path
package logging
