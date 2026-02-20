// Package index provides indexing implementations for ObaDB.
package index

import (
	"errors"
	"strings"
	"sync"

	"github.com/oba-ldap/oba/internal/storage"
	"github.com/oba-ldap/oba/internal/storage/btree"
)

// SubstringIndex errors.
var (
	ErrIndexNotInitialized = errors.New("substring index not initialized")
	ErrInvalidPageManager  = errors.New("invalid page manager")
	ErrEmptyValue          = errors.New("value cannot be empty")
	ErrEmptyPattern        = errors.New("pattern cannot be empty")
)

// SubstringIndex provides efficient substring search using n-gram indexing.
// It indexes attribute values by their n-grams, allowing fast lookups for
// wildcard patterns like (cn=*admin*) without full table scans.
type SubstringIndex struct {
	tree        *btree.BPlusTree
	pageManager *storage.PageManager
	ngramSize   int
	mu          sync.RWMutex
}

// NewSubstringIndex creates a new SubstringIndex with the given PageManager.
// Uses the default NgramSize for n-gram generation.
func NewSubstringIndex(pm *storage.PageManager) (*SubstringIndex, error) {
	return NewSubstringIndexWithSize(pm, NgramSize)
}

// NewSubstringIndexWithSize creates a new SubstringIndex with a custom n-gram size.
func NewSubstringIndexWithSize(pm *storage.PageManager, ngramSize int) (*SubstringIndex, error) {
	if pm == nil {
		return nil, ErrInvalidPageManager
	}

	if ngramSize <= 0 {
		ngramSize = NgramSize
	}

	tree, err := btree.NewBPlusTree(pm, 0)
	if err != nil {
		return nil, err
	}

	return &SubstringIndex{
		tree:        tree,
		pageManager: pm,
		ngramSize:   ngramSize,
	}, nil
}

// NewSubstringIndexWithRoot creates a SubstringIndex loading from an existing root page.
func NewSubstringIndexWithRoot(pm *storage.PageManager, rootPageID storage.PageID, ngramSize int) (*SubstringIndex, error) {
	if pm == nil {
		return nil, ErrInvalidPageManager
	}

	if ngramSize <= 0 {
		ngramSize = NgramSize
	}

	tree, err := btree.NewBPlusTreeWithRoot(pm, rootPageID, 0)
	if err != nil {
		return nil, err
	}

	return &SubstringIndex{
		tree:        tree,
		pageManager: pm,
		ngramSize:   ngramSize,
	}, nil
}

// Index adds a value to the substring index with the given entry reference.
// The value is broken into n-grams, and each n-gram is indexed pointing to the entry.
func (si *SubstringIndex) Index(value string, ref btree.EntryRef) error {
	if len(value) == 0 {
		return ErrEmptyValue
	}

	si.mu.Lock()
	defer si.mu.Unlock()

	// Generate unique n-grams to avoid duplicate index entries for the same value
	ngrams := GenerateUniqueNgrams(value, si.ngramSize)
	if len(ngrams) == 0 {
		return nil
	}

	// Index each n-gram
	for _, ngram := range ngrams {
		if err := si.tree.Insert([]byte(ngram), ref); err != nil {
			return err
		}
	}

	return nil
}

// Remove removes a value from the substring index for the given entry reference.
// This removes all n-gram entries associated with the value and entry reference.
func (si *SubstringIndex) Remove(value string, ref btree.EntryRef) error {
	if len(value) == 0 {
		return ErrEmptyValue
	}

	si.mu.Lock()
	defer si.mu.Unlock()

	// Generate the same n-grams that were indexed
	ngrams := GenerateUniqueNgrams(value, si.ngramSize)
	if len(ngrams) == 0 {
		return nil
	}

	// Remove each n-gram entry
	for _, ngram := range ngrams {
		if err := si.tree.Delete([]byte(ngram), ref); err != nil {
			// Ignore not found errors during removal
			if err != btree.ErrKeyNotFound {
				return err
			}
		}
	}

	return nil
}

// Search finds all entry references that may contain the given pattern.
// The pattern can include wildcards (*) for substring matching.
// Returns candidate entries that must be verified by the evaluator for false positives.
func (si *SubstringIndex) Search(pattern string) ([]btree.EntryRef, error) {
	if len(pattern) == 0 {
		return nil, ErrEmptyPattern
	}

	si.mu.RLock()
	defer si.mu.RUnlock()

	// Extract searchable n-grams from the pattern
	ngrams := ExtractSearchableNgrams(pattern, si.ngramSize)
	if len(ngrams) == 0 {
		// No n-grams could be extracted (pattern too short or all wildcards)
		// Return nil to indicate a full scan is needed
		return nil, nil
	}

	// Search for the first n-gram
	results, err := si.tree.Search([]byte(ngrams[0]))
	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return nil, nil
	}

	// Intersect results from all n-grams
	for _, ngram := range ngrams[1:] {
		ngramResults, err := si.tree.Search([]byte(ngram))
		if err != nil {
			return nil, err
		}

		results = intersectRefs(results, ngramResults)
		if len(results) == 0 {
			return nil, nil
		}
	}

	return results, nil
}

// SearchSubstring searches for entries containing the given substring.
// This is a convenience method for patterns like *substring*.
func (si *SubstringIndex) SearchSubstring(substring string) ([]btree.EntryRef, error) {
	return si.Search("*" + substring + "*")
}

// SearchPrefix searches for entries starting with the given prefix.
// This is a convenience method for patterns like prefix*.
func (si *SubstringIndex) SearchPrefix(prefix string) ([]btree.EntryRef, error) {
	return si.Search(prefix + "*")
}

// SearchSuffix searches for entries ending with the given suffix.
// This is a convenience method for patterns like *suffix.
func (si *SubstringIndex) SearchSuffix(suffix string) ([]btree.EntryRef, error) {
	return si.Search("*" + suffix)
}

// Root returns the root page ID of the underlying B+ tree.
func (si *SubstringIndex) Root() storage.PageID {
	return si.tree.Root()
}

// NgramSize returns the n-gram size used by this index.
func (si *SubstringIndex) NgramSize() int {
	return si.ngramSize
}

// IsEmpty returns true if the index has no entries.
func (si *SubstringIndex) IsEmpty() bool {
	return si.tree.IsEmpty()
}

// Stats returns statistics about the underlying B+ tree.
func (si *SubstringIndex) Stats() (btree.TreeStats, error) {
	return si.tree.Stats()
}

// intersectRefs returns the intersection of two entry reference slices.
// The result contains only references that appear in both slices.
func intersectRefs(a, b []btree.EntryRef) []btree.EntryRef {
	if len(a) == 0 || len(b) == 0 {
		return nil
	}

	// Use the smaller slice for the set
	if len(a) > len(b) {
		a, b = b, a
	}

	// Create a set from the smaller slice
	set := make(map[btree.EntryRef]struct{}, len(a))
	for _, ref := range a {
		set[ref] = struct{}{}
	}

	// Find intersection
	result := make([]btree.EntryRef, 0, len(a))
	for _, ref := range b {
		if _, exists := set[ref]; exists {
			result = append(result, ref)
			// Remove from set to avoid duplicates in result
			delete(set, ref)
		}
	}

	return result
}

// MatchesPattern checks if a value matches a wildcard pattern.
// This is used by the evaluator to filter false positives from index results.
// Supports * as a wildcard that matches any sequence of characters.
func MatchesPattern(value, pattern string) bool {
	// Normalize both to lowercase for case-insensitive matching
	value = strings.ToLower(value)
	pattern = strings.ToLower(pattern)

	return matchWildcard(value, pattern)
}

// matchWildcard performs wildcard pattern matching.
// Uses dynamic programming for efficient matching.
func matchWildcard(value, pattern string) bool {
	v := len(value)
	p := len(pattern)

	// dp[i][j] = true if value[0:i] matches pattern[0:j]
	dp := make([][]bool, v+1)
	for i := range dp {
		dp[i] = make([]bool, p+1)
	}

	// Empty pattern matches empty value
	dp[0][0] = true

	// Handle patterns starting with *
	for j := 1; j <= p; j++ {
		if pattern[j-1] == '*' {
			dp[0][j] = dp[0][j-1]
		}
	}

	// Fill the DP table
	for i := 1; i <= v; i++ {
		for j := 1; j <= p; j++ {
			if pattern[j-1] == '*' {
				// * can match zero characters (dp[i][j-1]) or one+ characters (dp[i-1][j])
				dp[i][j] = dp[i][j-1] || dp[i-1][j]
			} else if pattern[j-1] == '?' || value[i-1] == pattern[j-1] {
				// ? matches any single character, or exact character match
				dp[i][j] = dp[i-1][j-1]
			}
		}
	}

	return dp[v][p]
}

// FilterByPattern filters a slice of values, returning only those that match the pattern.
// This is useful for post-processing index results.
func FilterByPattern(values []string, pattern string) []string {
	if len(values) == 0 {
		return nil
	}

	result := make([]string, 0, len(values))
	for _, value := range values {
		if MatchesPattern(value, pattern) {
			result = append(result, value)
		}
	}

	return result
}
