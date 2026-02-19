package filter

import (
	"bytes"
	"strings"
)

// matchEquality performs case-insensitive equality matching between two byte slices.
// This is the default matching behavior for string attributes in LDAP.
func matchEquality(a, b []byte) bool {
	return bytes.EqualFold(a, b)
}

// matchEqualityExact performs exact (case-sensitive) equality matching.
func matchEqualityExact(a, b []byte) bool {
	return bytes.Equal(a, b)
}

// matchSubstring checks if a value matches a substring filter pattern.
// The pattern consists of optional initial, any (middle), and final components.
func matchSubstring(value []byte, initial []byte, any [][]byte, final []byte) bool {
	// Convert to lowercase for case-insensitive matching
	valueLower := bytes.ToLower(value)
	pos := 0

	// Check initial substring
	if len(initial) > 0 {
		initialLower := bytes.ToLower(initial)
		if !bytes.HasPrefix(valueLower, initialLower) {
			return false
		}
		pos = len(initial)
	}

	// Check middle substrings (any)
	for _, substr := range any {
		if len(substr) == 0 {
			continue
		}
		substrLower := bytes.ToLower(substr)
		idx := bytes.Index(valueLower[pos:], substrLower)
		if idx < 0 {
			return false
		}
		pos += idx + len(substr)
	}

	// Check final substring
	if len(final) > 0 {
		finalLower := bytes.ToLower(final)
		if !bytes.HasSuffix(valueLower[pos:], finalLower) {
			return false
		}
	}

	return true
}

// matchGreaterOrEqual performs case-insensitive greater-or-equal comparison.
// For string values, this uses lexicographic ordering.
func matchGreaterOrEqual(value, threshold []byte) bool {
	valueLower := bytes.ToLower(value)
	thresholdLower := bytes.ToLower(threshold)
	return bytes.Compare(valueLower, thresholdLower) >= 0
}

// matchLessOrEqual performs case-insensitive less-or-equal comparison.
// For string values, this uses lexicographic ordering.
func matchLessOrEqual(value, threshold []byte) bool {
	valueLower := bytes.ToLower(value)
	thresholdLower := bytes.ToLower(threshold)
	return bytes.Compare(valueLower, thresholdLower) <= 0
}

// matchApprox performs approximate matching.
// This implementation uses a simplified approach based on normalized comparison.
func matchApprox(a, b []byte) bool {
	// Normalize both values: lowercase and collapse whitespace
	aNorm := normalizeForApprox(a)
	bNorm := normalizeForApprox(b)
	return bytes.Equal(aNorm, bNorm)
}

// normalizeForApprox normalizes a value for approximate matching.
// It converts to lowercase and collapses multiple whitespace to single space.
func normalizeForApprox(value []byte) []byte {
	// Convert to lowercase string for easier manipulation
	s := strings.ToLower(string(value))

	// Collapse whitespace
	var result strings.Builder
	inWhitespace := false
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			if !inWhitespace {
				result.WriteRune(' ')
				inWhitespace = true
			}
		} else {
			result.WriteRune(r)
			inWhitespace = false
		}
	}

	// Trim leading/trailing whitespace
	return []byte(strings.TrimSpace(result.String()))
}

// normalizeAttributeName normalizes an attribute name for case-insensitive lookup.
func normalizeAttributeName(name string) string {
	return strings.ToLower(name)
}
