// Package index provides indexing implementations for ObaDB.
package index

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/oba-ldap/oba/internal/storage"
	"github.com/oba-ldap/oba/internal/storage/btree"
)

// TestGenerateNgrams tests the n-gram generation function.
func TestGenerateNgrams(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		n        int
		expected []string
	}{
		{
			name:     "empty string",
			input:    "",
			n:        3,
			expected: nil,
		},
		{
			name:     "string shorter than n",
			input:    "ab",
			n:        3,
			expected: []string{"ab"},
		},
		{
			name:     "string equal to n",
			input:    "abc",
			n:        3,
			expected: []string{"abc"},
		},
		{
			name:     "standard string",
			input:    "admin",
			n:        3,
			expected: []string{"adm", "dmi", "min"},
		},
		{
			name:     "uppercase converted to lowercase",
			input:    "ADMIN",
			n:        3,
			expected: []string{"adm", "dmi", "min"},
		},
		{
			name:     "mixed case",
			input:    "AdMiN",
			n:        3,
			expected: []string{"adm", "dmi", "min"},
		},
		{
			name:     "longer string",
			input:    "administrator",
			n:        3,
			expected: []string{"adm", "dmi", "min", "ini", "nis", "ist", "str", "tra", "rat", "ato", "tor"},
		},
		{
			name:     "with spaces",
			input:    "john doe",
			n:        3,
			expected: []string{"joh", "ohn", "hn ", "n d", " do", "doe"},
		},
		{
			name:     "n-gram size 2",
			input:    "test",
			n:        2,
			expected: []string{"te", "es", "st"},
		},
		{
			name:     "n-gram size 4",
			input:    "testing",
			n:        4,
			expected: []string{"test", "esti", "stin", "ting"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateNgrams(tt.input, tt.n)
			if !stringSliceEqual(result, tt.expected) {
				t.Errorf("GenerateNgrams(%q, %d) = %v, want %v", tt.input, tt.n, result, tt.expected)
			}
		})
	}
}

// TestGenerateNgramsDefault tests the default n-gram generation.
func TestGenerateNgramsDefault(t *testing.T) {
	result := GenerateNgramsDefault("admin")
	expected := []string{"adm", "dmi", "min"}
	if !stringSliceEqual(result, expected) {
		t.Errorf("GenerateNgramsDefault(\"admin\") = %v, want %v", result, expected)
	}
}

// TestGenerateUniqueNgrams tests unique n-gram generation.
func TestGenerateUniqueNgrams(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		n        int
		expected []string
	}{
		{
			name:     "no duplicates",
			input:    "admin",
			n:        3,
			expected: []string{"adm", "dmi", "min"},
		},
		{
			name:     "with duplicates",
			input:    "banana",
			n:        2,
			expected: []string{"ba", "an", "na"}, // "an" and "na" appear twice but only once in result
		},
		{
			name:     "all same",
			input:    "aaaa",
			n:        2,
			expected: []string{"aa"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateUniqueNgrams(tt.input, tt.n)
			if !stringSliceEqual(result, tt.expected) {
				t.Errorf("GenerateUniqueNgrams(%q, %d) = %v, want %v", tt.input, tt.n, result, tt.expected)
			}
		})
	}
}

// TestExtractSearchableNgrams tests n-gram extraction from patterns.
func TestExtractSearchableNgrams(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		n        int
		expected []string
	}{
		{
			name:     "no wildcards",
			pattern:  "admin",
			n:        3,
			expected: []string{"adm", "dmi", "min"},
		},
		{
			name:     "leading wildcard",
			pattern:  "*admin",
			n:        3,
			expected: []string{"adm", "dmi", "min"},
		},
		{
			name:     "trailing wildcard",
			pattern:  "admin*",
			n:        3,
			expected: []string{"adm", "dmi", "min"},
		},
		{
			name:     "both wildcards",
			pattern:  "*admin*",
			n:        3,
			expected: []string{"adm", "dmi", "min"},
		},
		{
			name:     "internal wildcard",
			pattern:  "ad*in",
			n:        2,
			expected: []string{"ad", "in"},
		},
		{
			name:     "only wildcards",
			pattern:  "***",
			n:        3,
			expected: nil,
		},
		{
			name:     "pattern too short",
			pattern:  "*ab*",
			n:        3,
			expected: []string{"ab"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractSearchableNgrams(tt.pattern, tt.n)
			if !stringSliceEqual(result, tt.expected) {
				t.Errorf("ExtractSearchableNgrams(%q, %d) = %v, want %v", tt.pattern, tt.n, result, tt.expected)
			}
		})
	}
}

// TestCountNgrams tests n-gram counting.
func TestCountNgrams(t *testing.T) {
	tests := []struct {
		input    string
		n        int
		expected int
	}{
		{"", 3, 0},
		{"ab", 3, 1},
		{"abc", 3, 1},
		{"admin", 3, 3},
		{"administrator", 3, 11},
	}

	for _, tt := range tests {
		result := CountNgrams(tt.input, tt.n)
		if result != tt.expected {
			t.Errorf("CountNgrams(%q, %d) = %d, want %d", tt.input, tt.n, result, tt.expected)
		}
	}
}

// TestNgramOverlap tests n-gram overlap calculation.
func TestNgramOverlap(t *testing.T) {
	tests := []struct {
		s1       string
		s2       string
		n        int
		expected int
	}{
		{"admin", "admin", 3, 3},
		{"admin", "administrator", 3, 3},
		{"hello", "world", 3, 0},
		{"", "admin", 3, 0},
		{"admin", "", 3, 0},
	}

	for _, tt := range tests {
		result := NgramOverlap(tt.s1, tt.s2, tt.n)
		if result != tt.expected {
			t.Errorf("NgramOverlap(%q, %q, %d) = %d, want %d", tt.s1, tt.s2, tt.n, result, tt.expected)
		}
	}
}

// TestMatchesPattern tests wildcard pattern matching.
func TestMatchesPattern(t *testing.T) {
	tests := []struct {
		value    string
		pattern  string
		expected bool
	}{
		// Exact match
		{"admin", "admin", true},
		{"admin", "ADMIN", true}, // Case insensitive
		{"ADMIN", "admin", true},

		// Leading wildcard
		{"administrator", "*admin", false},
		{"superadmin", "*admin", true},
		{"admin", "*admin", true},

		// Trailing wildcard
		{"administrator", "admin*", true},
		{"admin", "admin*", true},
		{"superadmin", "admin*", false},

		// Both wildcards (substring)
		{"administrator", "*admin*", true},
		{"superadmin", "*admin*", true},
		{"superadministrator", "*admin*", true},
		{"hello", "*admin*", false},

		// Internal wildcard
		{"administrator", "admin*tor", true},
		{"admin123tor", "admin*tor", true},
		{"admintor", "admin*tor", true},
		{"adminator", "admin*tor", true}, // "admin" + "ator" ends with "tor"

		// Multiple wildcards
		{"administrator", "*ad*in*", true},
		{"abcdefghij", "*c*f*i*", true},

		// Empty cases
		{"", "*", true},
		{"admin", "*", true},
		{"", "admin", false},

		// Question mark wildcard
		{"admin", "adm?n", true},
		{"adman", "adm?n", true},
		{"admiin", "adm?n", false},
	}

	for _, tt := range tests {
		t.Run(tt.value+"_"+tt.pattern, func(t *testing.T) {
			result := MatchesPattern(tt.value, tt.pattern)
			if result != tt.expected {
				t.Errorf("MatchesPattern(%q, %q) = %v, want %v", tt.value, tt.pattern, result, tt.expected)
			}
		})
	}
}

// TestFilterByPattern tests filtering values by pattern.
func TestFilterByPattern(t *testing.T) {
	values := []string{"admin", "administrator", "superadmin", "user", "guest"}

	tests := []struct {
		pattern  string
		expected []string
	}{
		{"*admin*", []string{"admin", "administrator", "superadmin"}},
		{"admin*", []string{"admin", "administrator"}},
		{"*admin", []string{"admin", "superadmin"}},
		{"user", []string{"user"}},
		{"*xyz*", nil},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			result := FilterByPattern(values, tt.pattern)
			if !stringSliceEqual(result, tt.expected) {
				t.Errorf("FilterByPattern(%v, %q) = %v, want %v", values, tt.pattern, result, tt.expected)
			}
		})
	}
}

// Helper function to compare string slices.
func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// setupTestPageManager creates a temporary PageManager for testing.
func setupTestPageManager(t *testing.T) (*storage.PageManager, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "substring_index_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	dataFile := filepath.Join(tmpDir, "test.oba")
	opts := storage.DefaultOptions()
	opts.InitialPages = 100
	pm, err := storage.OpenPageManager(dataFile, opts)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to create PageManager: %v", err)
	}

	cleanup := func() {
		pm.Close()
		os.RemoveAll(tmpDir)
	}

	return pm, cleanup
}

// TestSubstringIndexCreation tests creating a new SubstringIndex.
func TestSubstringIndexCreation(t *testing.T) {
	pm, cleanup := setupTestPageManager(t)
	defer cleanup()

	si, err := NewSubstringIndex(pm)
	if err != nil {
		t.Fatalf("NewSubstringIndex failed: %v", err)
	}

	if si == nil {
		t.Fatal("SubstringIndex is nil")
	}

	if si.NgramSize() != NgramSize {
		t.Errorf("NgramSize() = %d, want %d", si.NgramSize(), NgramSize)
	}

	if !si.IsEmpty() {
		t.Error("New index should be empty")
	}
}

// TestSubstringIndexWithCustomSize tests creating index with custom n-gram size.
func TestSubstringIndexWithCustomSize(t *testing.T) {
	pm, cleanup := setupTestPageManager(t)
	defer cleanup()

	customSize := 4
	si, err := NewSubstringIndexWithSize(pm, customSize)
	if err != nil {
		t.Fatalf("NewSubstringIndexWithSize failed: %v", err)
	}

	if si.NgramSize() != customSize {
		t.Errorf("NgramSize() = %d, want %d", si.NgramSize(), customSize)
	}
}

// TestSubstringIndexNilPageManager tests error handling for nil PageManager.
func TestSubstringIndexNilPageManager(t *testing.T) {
	_, err := NewSubstringIndex(nil)
	if err != ErrInvalidPageManager {
		t.Errorf("Expected ErrInvalidPageManager, got %v", err)
	}
}

// TestSubstringIndexBasicOperations tests basic index and search operations.
func TestSubstringIndexBasicOperations(t *testing.T) {
	pm, cleanup := setupTestPageManager(t)
	defer cleanup()

	si, err := NewSubstringIndex(pm)
	if err != nil {
		t.Fatalf("NewSubstringIndex failed: %v", err)
	}

	// Index some values
	ref1 := btree.EntryRef{PageID: 1, SlotID: 0}
	ref2 := btree.EntryRef{PageID: 1, SlotID: 1}
	ref3 := btree.EntryRef{PageID: 2, SlotID: 0}

	if err := si.Index("administrator", ref1); err != nil {
		t.Fatalf("Index failed: %v", err)
	}
	if err := si.Index("superadmin", ref2); err != nil {
		t.Fatalf("Index failed: %v", err)
	}
	if err := si.Index("guest", ref3); err != nil {
		t.Fatalf("Index failed: %v", err)
	}

	// Search for substring
	results, err := si.Search("*admin*")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Should find both admin-related entries
	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	// Verify the results contain the expected refs
	foundRef1 := false
	foundRef2 := false
	for _, ref := range results {
		if ref == ref1 {
			foundRef1 = true
		}
		if ref == ref2 {
			foundRef2 = true
		}
	}
	if !foundRef1 || !foundRef2 {
		t.Errorf("Expected to find ref1 and ref2 in results, got %v", results)
	}
}

// TestSubstringIndexSearchSubstring tests the SearchSubstring convenience method.
func TestSubstringIndexSearchSubstring(t *testing.T) {
	pm, cleanup := setupTestPageManager(t)
	defer cleanup()

	si, err := NewSubstringIndex(pm)
	if err != nil {
		t.Fatalf("NewSubstringIndex failed: %v", err)
	}

	ref := btree.EntryRef{PageID: 1, SlotID: 0}
	if err := si.Index("administrator", ref); err != nil {
		t.Fatalf("Index failed: %v", err)
	}

	results, err := si.SearchSubstring("admin")
	if err != nil {
		t.Fatalf("SearchSubstring failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}
}

// TestSubstringIndexEmptyValue tests error handling for empty values.
func TestSubstringIndexEmptyValue(t *testing.T) {
	pm, cleanup := setupTestPageManager(t)
	defer cleanup()

	si, err := NewSubstringIndex(pm)
	if err != nil {
		t.Fatalf("NewSubstringIndex failed: %v", err)
	}

	ref := btree.EntryRef{PageID: 1, SlotID: 0}
	err = si.Index("", ref)
	if err != ErrEmptyValue {
		t.Errorf("Expected ErrEmptyValue, got %v", err)
	}
}

// TestSubstringIndexEmptyPattern tests error handling for empty patterns.
func TestSubstringIndexEmptyPattern(t *testing.T) {
	pm, cleanup := setupTestPageManager(t)
	defer cleanup()

	si, err := NewSubstringIndex(pm)
	if err != nil {
		t.Fatalf("NewSubstringIndex failed: %v", err)
	}

	_, err = si.Search("")
	if err != ErrEmptyPattern {
		t.Errorf("Expected ErrEmptyPattern, got %v", err)
	}
}

// TestSubstringIndexNoResults tests search with no matching results.
func TestSubstringIndexNoResults(t *testing.T) {
	pm, cleanup := setupTestPageManager(t)
	defer cleanup()

	si, err := NewSubstringIndex(pm)
	if err != nil {
		t.Fatalf("NewSubstringIndex failed: %v", err)
	}

	ref := btree.EntryRef{PageID: 1, SlotID: 0}
	if err := si.Index("administrator", ref); err != nil {
		t.Fatalf("Index failed: %v", err)
	}

	results, err := si.Search("*xyz*")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if results != nil && len(results) != 0 {
		t.Errorf("Expected no results, got %v", results)
	}
}

// TestSubstringIndexShortPattern tests search with pattern shorter than n-gram size.
func TestSubstringIndexShortPattern(t *testing.T) {
	pm, cleanup := setupTestPageManager(t)
	defer cleanup()

	si, err := NewSubstringIndex(pm)
	if err != nil {
		t.Fatalf("NewSubstringIndex failed: %v", err)
	}

	ref := btree.EntryRef{PageID: 1, SlotID: 0}
	if err := si.Index("ab", ref); err != nil {
		t.Fatalf("Index failed: %v", err)
	}

	// Search for the short value
	results, err := si.Search("*ab*")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}
}

// TestSubstringIndexCaseInsensitive tests case-insensitive matching.
func TestSubstringIndexCaseInsensitive(t *testing.T) {
	pm, cleanup := setupTestPageManager(t)
	defer cleanup()

	si, err := NewSubstringIndex(pm)
	if err != nil {
		t.Fatalf("NewSubstringIndex failed: %v", err)
	}

	ref := btree.EntryRef{PageID: 1, SlotID: 0}
	if err := si.Index("Administrator", ref); err != nil {
		t.Fatalf("Index failed: %v", err)
	}

	// Search with different case
	results, err := si.Search("*ADMIN*")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 result for case-insensitive search, got %d", len(results))
	}
}

// TestSubstringIndexMultipleValues tests indexing multiple values with same n-grams.
func TestSubstringIndexMultipleValues(t *testing.T) {
	pm, cleanup := setupTestPageManager(t)
	defer cleanup()

	si, err := NewSubstringIndex(pm)
	if err != nil {
		t.Fatalf("NewSubstringIndex failed: %v", err)
	}

	// Index multiple values that share n-grams
	refs := []btree.EntryRef{
		{PageID: 1, SlotID: 0},
		{PageID: 1, SlotID: 1},
		{PageID: 1, SlotID: 2},
		{PageID: 2, SlotID: 0},
	}

	values := []string{"admin", "administrator", "sysadmin", "guest"}

	for i, value := range values {
		if err := si.Index(value, refs[i]); err != nil {
			t.Fatalf("Index failed for %s: %v", value, err)
		}
	}

	// Search for "admin" substring
	results, err := si.Search("*admin*")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Should find 3 entries (admin, administrator, sysadmin)
	if len(results) != 3 {
		t.Errorf("Expected 3 results, got %d: %v", len(results), results)
	}
}

// TestIntersectRefs tests the intersection function.
func TestIntersectRefs(t *testing.T) {
	tests := []struct {
		name     string
		a        []btree.EntryRef
		b        []btree.EntryRef
		expected int
	}{
		{
			name:     "empty slices",
			a:        nil,
			b:        nil,
			expected: 0,
		},
		{
			name:     "one empty",
			a:        []btree.EntryRef{{PageID: 1, SlotID: 0}},
			b:        nil,
			expected: 0,
		},
		{
			name: "no overlap",
			a:    []btree.EntryRef{{PageID: 1, SlotID: 0}},
			b:    []btree.EntryRef{{PageID: 2, SlotID: 0}},
			expected: 0,
		},
		{
			name: "full overlap",
			a:    []btree.EntryRef{{PageID: 1, SlotID: 0}, {PageID: 1, SlotID: 1}},
			b:    []btree.EntryRef{{PageID: 1, SlotID: 0}, {PageID: 1, SlotID: 1}},
			expected: 2,
		},
		{
			name: "partial overlap",
			a:    []btree.EntryRef{{PageID: 1, SlotID: 0}, {PageID: 1, SlotID: 1}},
			b:    []btree.EntryRef{{PageID: 1, SlotID: 1}, {PageID: 2, SlotID: 0}},
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := intersectRefs(tt.a, tt.b)
			if len(result) != tt.expected {
				t.Errorf("intersectRefs() returned %d results, want %d", len(result), tt.expected)
			}
		})
	}
}

// TestSubstringIndexStats tests the Stats method.
func TestSubstringIndexStats(t *testing.T) {
	pm, cleanup := setupTestPageManager(t)
	defer cleanup()

	si, err := NewSubstringIndex(pm)
	if err != nil {
		t.Fatalf("NewSubstringIndex failed: %v", err)
	}

	// Index some values
	for i := 0; i < 10; i++ {
		ref := btree.EntryRef{PageID: storage.PageID(i), SlotID: 0}
		if err := si.Index("value"+string(rune('a'+i)), ref); err != nil {
			t.Fatalf("Index failed: %v", err)
		}
	}

	stats, err := si.Stats()
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}

	if stats.TotalKeys == 0 {
		t.Error("Expected non-zero TotalKeys")
	}
}

// TestSubstringIndexRoot tests the Root method.
func TestSubstringIndexRoot(t *testing.T) {
	pm, cleanup := setupTestPageManager(t)
	defer cleanup()

	si, err := NewSubstringIndex(pm)
	if err != nil {
		t.Fatalf("NewSubstringIndex failed: %v", err)
	}

	root := si.Root()
	if root == 0 {
		t.Error("Expected non-zero root page ID")
	}
}

// BenchmarkGenerateNgrams benchmarks n-gram generation.
func BenchmarkGenerateNgrams(b *testing.B) {
	input := "administrator"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GenerateNgrams(input, 3)
	}
}

// BenchmarkMatchesPattern benchmarks pattern matching.
func BenchmarkMatchesPattern(b *testing.B) {
	value := "administrator"
	pattern := "*admin*"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		MatchesPattern(value, pattern)
	}
}

// BenchmarkSubstringIndexSearch benchmarks index search.
func BenchmarkSubstringIndexSearch(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "substring_bench")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dataFile := filepath.Join(tmpDir, "bench.oba")
	opts := storage.DefaultOptions()
	opts.InitialPages = 1000
	pm, err := storage.OpenPageManager(dataFile, opts)
	if err != nil {
		b.Fatalf("Failed to create PageManager: %v", err)
	}
	defer pm.Close()

	si, err := NewSubstringIndex(pm)
	if err != nil {
		b.Fatalf("NewSubstringIndex failed: %v", err)
	}

	// Index 1000 values
	for i := 0; i < 1000; i++ {
		ref := btree.EntryRef{PageID: storage.PageID(i), SlotID: 0}
		value := "user" + string(rune('a'+i%26)) + "admin" + string(rune('0'+i%10))
		if err := si.Index(value, ref); err != nil {
			b.Fatalf("Index failed: %v", err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		si.Search("*admin*")
	}
}
