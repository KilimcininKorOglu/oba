// Package logging provides structured logging for the Oba LDAP server.
package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// Level represents the logging level.
type Level int

const (
	// LevelDebug is the most verbose level.
	LevelDebug Level = iota
	// LevelInfo is for informational messages.
	LevelInfo
	// LevelWarn is for warning messages.
	LevelWarn
	// LevelError is for error messages.
	LevelError
)

// String returns the string representation of the log level.
func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "debug"
	case LevelInfo:
		return "info"
	case LevelWarn:
		return "warn"
	case LevelError:
		return "error"
	default:
		return "unknown"
	}
}

// ParseLevel parses a string into a Level.
func ParseLevel(s string) Level {
	switch s {
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "warn":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return LevelInfo
	}
}

// Format represents the log output format.
type Format int

const (
	// FormatText outputs logs in human-readable text format.
	FormatText Format = iota
	// FormatJSON outputs logs in JSON format.
	FormatJSON
)

// ParseFormat parses a string into a Format.
func ParseFormat(s string) Format {
	switch s {
	case "json":
		return FormatJSON
	case "text":
		return FormatText
	default:
		return FormatText
	}
}

// Logger is the interface for structured logging.
type Logger interface {
	// Debug logs a debug message with optional key-value pairs.
	Debug(msg string, keysAndValues ...interface{})
	// Info logs an info message with optional key-value pairs.
	Info(msg string, keysAndValues ...interface{})
	// Warn logs a warning message with optional key-value pairs.
	Warn(msg string, keysAndValues ...interface{})
	// Error logs an error message with optional key-value pairs.
	Error(msg string, keysAndValues ...interface{})
	// WithRequestID returns a new logger with the given request ID.
	WithRequestID(requestID string) Logger
	// WithFields returns a new logger with the given fields.
	WithFields(keysAndValues ...interface{}) Logger
}

// logger is the default implementation of Logger.
type logger struct {
	level     Level
	format    Format
	output    io.Writer
	fields    map[string]interface{}
	mu        sync.Mutex
	requestID string
}

// Config holds the logger configuration.
type Config struct {
	Level  string
	Format string
	Output string
}

// New creates a new Logger with the given configuration.
func New(cfg Config) Logger {
	var output io.Writer
	switch cfg.Output {
	case "", "stdout":
		output = os.Stdout
	case "stderr":
		output = os.Stderr
	default:
		// Try to open file, fall back to stdout on error
		f, err := os.OpenFile(cfg.Output, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			output = os.Stdout
		} else {
			output = f
		}
	}

	return &logger{
		level:  ParseLevel(cfg.Level),
		format: ParseFormat(cfg.Format),
		output: output,
		fields: make(map[string]interface{}),
	}
}

// NewDefault creates a new Logger with default settings.
func NewDefault() Logger {
	return &logger{
		level:  LevelInfo,
		format: FormatText,
		output: os.Stdout,
		fields: make(map[string]interface{}),
	}
}

// NewNop creates a no-op logger that discards all output.
func NewNop() Logger {
	return &nopLogger{}
}

// Debug logs a debug message.
func (l *logger) Debug(msg string, keysAndValues ...interface{}) {
	l.log(LevelDebug, msg, keysAndValues...)
}

// Info logs an info message.
func (l *logger) Info(msg string, keysAndValues ...interface{}) {
	l.log(LevelInfo, msg, keysAndValues...)
}

// Warn logs a warning message.
func (l *logger) Warn(msg string, keysAndValues ...interface{}) {
	l.log(LevelWarn, msg, keysAndValues...)
}

// Error logs an error message.
func (l *logger) Error(msg string, keysAndValues ...interface{}) {
	l.log(LevelError, msg, keysAndValues...)
}

// WithRequestID returns a new logger with the given request ID.
func (l *logger) WithRequestID(requestID string) Logger {
	newLogger := l.clone()
	newLogger.requestID = requestID
	return newLogger
}

// WithFields returns a new logger with the given fields.
func (l *logger) WithFields(keysAndValues ...interface{}) Logger {
	newLogger := l.clone()
	for i := 0; i < len(keysAndValues)-1; i += 2 {
		if key, ok := keysAndValues[i].(string); ok {
			newLogger.fields[key] = keysAndValues[i+1]
		}
	}
	return newLogger
}

// clone creates a copy of the logger.
func (l *logger) clone() *logger {
	newFields := make(map[string]interface{}, len(l.fields))
	for k, v := range l.fields {
		newFields[k] = v
	}
	return &logger{
		level:     l.level,
		format:    l.format,
		output:    l.output,
		fields:    newFields,
		requestID: l.requestID,
	}
}

// log writes a log entry.
func (l *logger) log(level Level, msg string, keysAndValues ...interface{}) {
	if level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Build the log entry
	entry := make(map[string]interface{})
	entry["ts"] = time.Now().UTC().Format(time.RFC3339)
	entry["level"] = level.String()
	entry["msg"] = msg

	// Add request ID if present
	if l.requestID != "" {
		entry["request_id"] = l.requestID
	}

	// Add base fields
	for k, v := range l.fields {
		entry[k] = v
	}

	// Add key-value pairs
	for i := 0; i < len(keysAndValues)-1; i += 2 {
		if key, ok := keysAndValues[i].(string); ok {
			entry[key] = keysAndValues[i+1]
		}
	}

	// Format and write
	var output string
	if l.format == FormatJSON {
		data, err := json.Marshal(entry)
		if err != nil {
			output = fmt.Sprintf(`{"ts":"%s","level":"error","msg":"failed to marshal log entry"}`, time.Now().UTC().Format(time.RFC3339))
		} else {
			output = string(data)
		}
	} else {
		output = l.formatText(entry)
	}

	fmt.Fprintln(l.output, output)
}

// formatText formats a log entry as text.
func (l *logger) formatText(entry map[string]interface{}) string {
	ts := entry["ts"]
	level := entry["level"]
	msg := entry["msg"]

	result := fmt.Sprintf("%s [%s] %s", ts, level, msg)

	// Add request ID if present
	if reqID, ok := entry["request_id"]; ok {
		result += fmt.Sprintf(" request_id=%v", reqID)
	}

	// Add other fields
	for k, v := range entry {
		if k == "ts" || k == "level" || k == "msg" || k == "request_id" {
			continue
		}
		result += fmt.Sprintf(" %s=%v", k, v)
	}

	return result
}

// nopLogger is a no-op logger that discards all output.
type nopLogger struct{}

func (n *nopLogger) Debug(_ string, _ ...interface{})          {}
func (n *nopLogger) Info(_ string, _ ...interface{})           {}
func (n *nopLogger) Warn(_ string, _ ...interface{})           {}
func (n *nopLogger) Error(_ string, _ ...interface{})          {}
func (n *nopLogger) WithRequestID(_ string) Logger             { return n }
func (n *nopLogger) WithFields(_ ...interface{}) Logger        { return n }
