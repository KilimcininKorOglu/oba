// Package logging provides structured logging for the Oba LDAP server.
package logging

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected Level
	}{
		{"debug", LevelDebug},
		{"info", LevelInfo},
		{"warn", LevelWarn},
		{"error", LevelError},
		{"unknown", LevelInfo}, // default
		{"", LevelInfo},        // default
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ParseLevel(tt.input)
			if result != tt.expected {
				t.Errorf("ParseLevel(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestLevelString(t *testing.T) {
	tests := []struct {
		level    Level
		expected string
	}{
		{LevelDebug, "debug"},
		{LevelInfo, "info"},
		{LevelWarn, "warn"},
		{LevelError, "error"},
		{Level(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.level.String()
			if result != tt.expected {
				t.Errorf("Level.String() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestParseFormat(t *testing.T) {
	tests := []struct {
		input    string
		expected Format
	}{
		{"json", FormatJSON},
		{"text", FormatText},
		{"unknown", FormatText}, // default
		{"", FormatText},        // default
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ParseFormat(tt.input)
			if result != tt.expected {
				t.Errorf("ParseFormat(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestLoggerJSON(t *testing.T) {
	var buf bytes.Buffer
	l := &logger{
		level:  LevelDebug,
		format: FormatJSON,
		output: &buf,
		fields: make(map[string]interface{}),
	}

	l.Info("test message", "key1", "value1", "key2", 42)

	output := buf.String()
	if output == "" {
		t.Fatal("Expected output, got empty string")
	}

	var entry map[string]interface{}
	if err := json.Unmarshal([]byte(output), &entry); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	if entry["level"] != "info" {
		t.Errorf("Expected level=info, got %v", entry["level"])
	}
	if entry["msg"] != "test message" {
		t.Errorf("Expected msg='test message', got %v", entry["msg"])
	}
	if entry["key1"] != "value1" {
		t.Errorf("Expected key1=value1, got %v", entry["key1"])
	}
	if entry["key2"] != float64(42) { // JSON numbers are float64
		t.Errorf("Expected key2=42, got %v", entry["key2"])
	}
}

func TestLoggerText(t *testing.T) {
	var buf bytes.Buffer
	l := &logger{
		level:  LevelDebug,
		format: FormatText,
		output: &buf,
		fields: make(map[string]interface{}),
	}

	l.Info("test message", "key1", "value1")

	output := buf.String()
	if !strings.Contains(output, "[info]") {
		t.Errorf("Expected [info] in output, got: %s", output)
	}
	if !strings.Contains(output, "test message") {
		t.Errorf("Expected 'test message' in output, got: %s", output)
	}
	if !strings.Contains(output, "key1=value1") {
		t.Errorf("Expected 'key1=value1' in output, got: %s", output)
	}
}

func TestLoggerLevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	l := &logger{
		level:  LevelWarn,
		format: FormatText,
		output: &buf,
		fields: make(map[string]interface{}),
	}

	l.Debug("debug message")
	l.Info("info message")
	l.Warn("warn message")
	l.Error("error message")

	output := buf.String()
	if strings.Contains(output, "debug message") {
		t.Error("Debug message should be filtered")
	}
	if strings.Contains(output, "info message") {
		t.Error("Info message should be filtered")
	}
	if !strings.Contains(output, "warn message") {
		t.Error("Warn message should be present")
	}
	if !strings.Contains(output, "error message") {
		t.Error("Error message should be present")
	}
}

func TestLoggerWithRequestID(t *testing.T) {
	var buf bytes.Buffer
	l := &logger{
		level:  LevelDebug,
		format: FormatJSON,
		output: &buf,
		fields: make(map[string]interface{}),
	}

	reqLogger := l.WithRequestID("req-123")
	reqLogger.Info("test message")

	var entry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	if entry["request_id"] != "req-123" {
		t.Errorf("Expected request_id=req-123, got %v", entry["request_id"])
	}
}

func TestLoggerWithFields(t *testing.T) {
	var buf bytes.Buffer
	l := &logger{
		level:  LevelDebug,
		format: FormatJSON,
		output: &buf,
		fields: make(map[string]interface{}),
	}

	fieldLogger := l.WithFields("client", "192.168.1.100", "tls", true)
	fieldLogger.Info("test message")

	var entry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	if entry["client"] != "192.168.1.100" {
		t.Errorf("Expected client=192.168.1.100, got %v", entry["client"])
	}
	if entry["tls"] != true {
		t.Errorf("Expected tls=true, got %v", entry["tls"])
	}
}

func TestLoggerCloneIsolation(t *testing.T) {
	var buf bytes.Buffer
	l := &logger{
		level:  LevelDebug,
		format: FormatJSON,
		output: &buf,
		fields: make(map[string]interface{}),
	}

	// Create a child logger with fields
	child := l.WithFields("child_field", "value")

	// Original logger should not have the child's fields
	buf.Reset()
	l.Info("parent message")

	var parentEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &parentEntry); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	if _, ok := parentEntry["child_field"]; ok {
		t.Error("Parent logger should not have child's fields")
	}

	// Child logger should have its fields
	buf.Reset()
	child.Info("child message")

	var childEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &childEntry); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	if childEntry["child_field"] != "value" {
		t.Errorf("Child logger should have its fields, got %v", childEntry["child_field"])
	}
}

func TestNewLogger(t *testing.T) {
	cfg := Config{
		Level:  "debug",
		Format: "json",
		Output: "stdout",
	}

	l := New(cfg)
	if l == nil {
		t.Fatal("New returned nil")
	}
}

func TestNewDefault(t *testing.T) {
	l := NewDefault()
	if l == nil {
		t.Fatal("NewDefault returned nil")
	}
}

func TestNopLogger(t *testing.T) {
	l := NewNop()
	if l == nil {
		t.Fatal("NewNop returned nil")
	}

	// These should not panic
	l.Debug("test")
	l.Info("test")
	l.Warn("test")
	l.Error("test")

	// WithRequestID should return the same nop logger
	l2 := l.WithRequestID("req-123")
	if l2 == nil {
		t.Error("WithRequestID returned nil")
	}

	// WithFields should return the same nop logger
	l3 := l.WithFields("key", "value")
	if l3 == nil {
		t.Error("WithFields returned nil")
	}
}

func TestLoggerAllLevels(t *testing.T) {
	var buf bytes.Buffer
	l := &logger{
		level:  LevelDebug,
		format: FormatJSON,
		output: &buf,
		fields: make(map[string]interface{}),
	}

	tests := []struct {
		logFunc func(string, ...interface{})
		level   string
	}{
		{l.Debug, "debug"},
		{l.Info, "info"},
		{l.Warn, "warn"},
		{l.Error, "error"},
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			buf.Reset()
			tt.logFunc("test message")

			var entry map[string]interface{}
			if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
				t.Fatalf("Failed to parse JSON output: %v", err)
			}

			if entry["level"] != tt.level {
				t.Errorf("Expected level=%s, got %v", tt.level, entry["level"])
			}
		})
	}
}
