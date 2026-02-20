// Package logging provides structured logging for the Oba LDAP server.
package logging

import (
	"strings"
	"testing"
)

func TestGenerateRequestID(t *testing.T) {
	id1 := GenerateRequestID()
	id2 := GenerateRequestID()

	if id1 == "" {
		t.Error("GenerateRequestID returned empty string")
	}

	if id2 == "" {
		t.Error("GenerateRequestID returned empty string")
	}

	// IDs should be unique
	if id1 == id2 {
		t.Errorf("GenerateRequestID returned duplicate IDs: %s", id1)
	}

	// IDs should have the expected format (timestamp-counter-random)
	parts := strings.Split(id1, "-")
	if len(parts) != 3 {
		t.Errorf("Expected 3 parts in request ID, got %d: %s", len(parts), id1)
	}
}

func TestGenerateRequestIDUniqueness(t *testing.T) {
	ids := make(map[string]bool)
	count := 1000

	for i := 0; i < count; i++ {
		id := GenerateRequestID()
		if ids[id] {
			t.Errorf("Duplicate request ID generated: %s", id)
		}
		ids[id] = true
	}

	if len(ids) != count {
		t.Errorf("Expected %d unique IDs, got %d", count, len(ids))
	}
}

func TestFormatCounter(t *testing.T) {
	tests := []struct {
		counter  uint64
		expected string
	}{
		{0, "0000"},
		{1, "0001"},
		{255, "00ff"},
		{256, "0100"},
		{65535, "ffff"},
	}

	for _, tt := range tests {
		result := formatCounter(tt.counter)
		if result != tt.expected {
			t.Errorf("formatCounter(%d) = %s, want %s", tt.counter, result, tt.expected)
		}
	}
}
