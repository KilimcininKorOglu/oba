// Package index provides indexing implementations for ObaDB.
package index

import (
	"strings"
	"unicode"
)

// NgramSize is the default size of n-grams used for substring indexing.
const NgramSize = 3

// GenerateNgrams generates n-grams of the specified size from the input string.
// The input is normalized to lowercase for case-insensitive matching.
// If the string is shorter than n, returns a slice containing the entire string.
// Returns an empty slice for empty strings.
func GenerateNgrams(s string, n int) []string {
	if len(s) == 0 {
		return nil
	}

	// Normalize to lowercase for case-insensitive matching
	s = strings.ToLower(s)

	// If string is shorter than n, return the whole string as a single n-gram
	if len(s) < n {
		return []string{s}
	}

	// Generate n-grams
	ngrams := make([]string, 0, len(s)-n+1)
	for i := 0; i <= len(s)-n; i++ {
		ngrams = append(ngrams, s[i:i+n])
	}

	return ngrams
}

// GenerateNgramsDefault generates n-grams using the default NgramSize.
func GenerateNgramsDefault(s string) []string {
	return GenerateNgrams(s, NgramSize)
}

// GenerateUniqueNgrams generates unique n-grams from the input string.
// This is useful when indexing to avoid duplicate entries.
func GenerateUniqueNgrams(s string, n int) []string {
	ngrams := GenerateNgrams(s, n)
	if len(ngrams) == 0 {
		return nil
	}

	// Use a map to track unique n-grams while preserving order
	seen := make(map[string]struct{}, len(ngrams))
	unique := make([]string, 0, len(ngrams))

	for _, ngram := range ngrams {
		if _, exists := seen[ngram]; !exists {
			seen[ngram] = struct{}{}
			unique = append(unique, ngram)
		}
	}

	return unique
}

// GenerateUniqueNgramsDefault generates unique n-grams using the default NgramSize.
func GenerateUniqueNgramsDefault(s string) []string {
	return GenerateUniqueNgrams(s, NgramSize)
}

// NormalizeForNgram normalizes a string for n-gram generation.
// It converts to lowercase and removes leading/trailing whitespace.
func NormalizeForNgram(s string) string {
	return strings.TrimSpace(strings.ToLower(s))
}

// ExtractSearchableNgrams extracts n-grams from a search pattern.
// It handles wildcard patterns by extracting n-grams from non-wildcard portions.
// For example, "*admin*" would extract n-grams from "admin".
func ExtractSearchableNgrams(pattern string, n int) []string {
	// Remove leading and trailing wildcards
	pattern = strings.Trim(pattern, "*")

	if len(pattern) == 0 {
		return nil
	}

	// Split by internal wildcards and generate n-grams for each part
	parts := strings.Split(pattern, "*")
	var allNgrams []string

	for _, part := range parts {
		if len(part) > 0 {
			ngrams := GenerateNgrams(part, n)
			allNgrams = append(allNgrams, ngrams...)
		}
	}

	return allNgrams
}

// ExtractSearchableNgramsDefault extracts searchable n-grams using the default NgramSize.
func ExtractSearchableNgramsDefault(pattern string) []string {
	return ExtractSearchableNgrams(pattern, NgramSize)
}

// IsValidNgramCandidate checks if a string is a valid candidate for n-gram indexing.
// Returns false for strings that are too short or contain only whitespace.
func IsValidNgramCandidate(s string, n int) bool {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return false
	}
	// Even short strings can be indexed (they become single n-grams)
	return true
}

// CountNgrams returns the number of n-grams that would be generated for a string.
// This is useful for estimating index size.
func CountNgrams(s string, n int) int {
	s = strings.ToLower(s)
	if len(s) == 0 {
		return 0
	}
	if len(s) < n {
		return 1
	}
	return len(s) - n + 1
}

// NgramOverlap calculates the number of common n-grams between two strings.
// This can be used for similarity scoring.
func NgramOverlap(s1, s2 string, n int) int {
	ngrams1 := GenerateUniqueNgrams(s1, n)
	ngrams2 := GenerateUniqueNgrams(s2, n)

	if len(ngrams1) == 0 || len(ngrams2) == 0 {
		return 0
	}

	// Create a set from the first string's n-grams
	set1 := make(map[string]struct{}, len(ngrams1))
	for _, ngram := range ngrams1 {
		set1[ngram] = struct{}{}
	}

	// Count overlapping n-grams
	overlap := 0
	for _, ngram := range ngrams2 {
		if _, exists := set1[ngram]; exists {
			overlap++
		}
	}

	return overlap
}

// ContainsNonAlphanumeric checks if a string contains non-alphanumeric characters.
// This can be useful for deciding whether to apply special handling.
func ContainsNonAlphanumeric(s string) bool {
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return true
		}
	}
	return false
}
