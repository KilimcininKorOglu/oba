// Package server provides the LDAP server implementation.
package server

import (
	"time"

	"github.com/oba-ldap/oba/internal/filter"
	"github.com/oba-ldap/oba/internal/ldap"
	"github.com/oba-ldap/oba/internal/storage"
)

// SubtreeSearcher handles subtree scope search operations.
// Subtree scope search returns the base entry and all its descendants.
type SubtreeSearcher struct {
	backend   SearchBackend
	evaluator *filter.Evaluator
}

// NewSubtreeSearcher creates a new SubtreeSearcher with the given backend.
func NewSubtreeSearcher(backend SearchBackend) *SubtreeSearcher {
	return &SubtreeSearcher{
		backend:   backend,
		evaluator: filter.NewEvaluator(nil),
	}
}

// Search performs a subtree scope search operation.
// It iterates over the base entry and all its descendants, evaluates the filter,
// and returns matching entries.
func (s *SubtreeSearcher) Search(req *ldap.SearchRequest, config *SearchConfig) *SearchResult {
	// Get iterator for subtree scope
	iter := s.backend.SearchByDN(req.BaseObject, storage.ScopeSubtree)
	if iter == nil {
		return &SearchResult{
			OperationResult: OperationResult{
				ResultCode:        ldap.ResultOperationsError,
				DiagnosticMessage: "failed to create search iterator",
			},
		}
	}
	defer iter.Close()

	// Check for iterator error
	if err := iter.Error(); err != nil {
		return &SearchResult{
			OperationResult: OperationResult{
				ResultCode:        ldap.ResultNoSuchObject,
				DiagnosticMessage: "base object not found",
			},
		}
	}

	// Process results with limits
	return s.processResults(req, config, iter)
}

// processResults iterates over entries and applies filter, size limit, and time limit.
func (s *SubtreeSearcher) processResults(req *ldap.SearchRequest, config *SearchConfig, iter storage.Iterator) *SearchResult {
	var entries []*SearchEntry
	count := 0

	// Calculate effective limits
	sizeLimit := req.SizeLimit
	if sizeLimit == 0 && config != nil {
		sizeLimit = config.DefaultSizeLimit
	}
	if config != nil && config.MaxSizeLimit > 0 && (sizeLimit == 0 || sizeLimit > config.MaxSizeLimit) {
		sizeLimit = config.MaxSizeLimit
	}

	timeLimit := req.TimeLimit
	if timeLimit == 0 && config != nil {
		timeLimit = config.DefaultTimeLimit
	}
	if config != nil && config.MaxTimeLimit > 0 && (timeLimit == 0 || timeLimit > config.MaxTimeLimit) {
		timeLimit = config.MaxTimeLimit
	}

	// Set deadline if time limit is specified
	var deadline time.Time
	if timeLimit > 0 {
		deadline = time.Now().Add(time.Duration(timeLimit) * time.Second)
	}

	// Iterate over entries
	for iter.Next() {
		// Check size limit
		if sizeLimit > 0 && count >= sizeLimit {
			return &SearchResult{
				OperationResult: OperationResult{
					ResultCode: ldap.ResultSizeLimitExceeded,
				},
				Entries: entries,
			}
		}

		// Check time limit
		if timeLimit > 0 && time.Now().After(deadline) {
			return &SearchResult{
				OperationResult: OperationResult{
					ResultCode: ldap.ResultTimeLimitExceeded,
				},
				Entries: entries,
			}
		}

		entry := iter.Entry()
		if entry == nil {
			continue
		}

		// Evaluate filter
		if !s.matchesFilter(entry, req.Filter) {
			continue
		}

		// Build search entry with attribute selection
		searchEntry := buildSearchEntryFromStorage(entry, req.Attributes, req.TypesOnly)
		entries = append(entries, searchEntry)
		count++
	}

	// Check for iteration error
	if err := iter.Error(); err != nil {
		return &SearchResult{
			OperationResult: OperationResult{
				ResultCode:        ldap.ResultOperationsError,
				DiagnosticMessage: "error during search iteration",
			},
			Entries: entries,
		}
	}

	return &SearchResult{
		OperationResult: OperationResult{
			ResultCode: ldap.ResultSuccess,
		},
		Entries: entries,
	}
}

// matchesFilter evaluates the search filter against an entry.
// Returns true if the filter matches or if no filter is specified.
func (s *SubtreeSearcher) matchesFilter(entry *storage.Entry, searchFilter *ldap.SearchFilter) bool {
	// No filter means match everything
	if searchFilter == nil {
		return true
	}

	// Convert storage.Entry to filter.Entry
	filterEntry := storageToFilterEntry(entry)

	// Convert ldap.SearchFilter to filter.Filter
	f := ldapFilterToFilter(searchFilter)
	if f == nil {
		// If conversion fails, treat as no filter (match everything)
		return true
	}

	return s.evaluator.Evaluate(f, filterEntry)
}
