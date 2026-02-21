// Package server provides the LDAP server implementation.
package server

import (
	"github.com/KilimcininKorOglu/oba/internal/filter"
	"github.com/KilimcininKorOglu/oba/internal/ldap"
	"github.com/KilimcininKorOglu/oba/internal/storage"
)

// BaseSearcher handles base scope search operations.
// Base scope search returns only the entry specified by the base DN.
type BaseSearcher struct {
	backend   Backend
	evaluator *filter.Evaluator
}

// NewBaseSearcher creates a new BaseSearcher with the given backend.
func NewBaseSearcher(backend Backend) *BaseSearcher {
	return &BaseSearcher{
		backend:   backend,
		evaluator: filter.NewEvaluator(nil),
	}
}

// Search performs a base scope search operation.
// It looks up the entry by base DN, evaluates the filter, and returns
// the entry if it matches.
func (s *BaseSearcher) Search(req *ldap.SearchRequest) *SearchResult {
	// Look up the entry by DN
	entry, err := s.backend.GetEntry(req.BaseObject)
	if err != nil {
		return &SearchResult{
			OperationResult: OperationResult{
				ResultCode:        ldap.ResultOperationsError,
				DiagnosticMessage: "internal error during search",
			},
		}
	}

	// Entry not found
	if entry == nil {
		return &SearchResult{
			OperationResult: OperationResult{
				ResultCode: ldap.ResultNoSuchObject,
				MatchedDN:  findMatchedDNForBase(req.BaseObject),
			},
		}
	}

	// Evaluate the filter against the entry
	if !s.matchesFilter(entry, req.Filter) {
		// Filter doesn't match - return success with no entries
		return &SearchResult{
			OperationResult: OperationResult{
				ResultCode: ldap.ResultSuccess,
			},
			Entries: nil,
		}
	}

	// Build the search result entry with attribute selection
	searchEntry := buildSearchEntryFromStorage(entry, req.Attributes, req.TypesOnly)

	return &SearchResult{
		OperationResult: OperationResult{
			ResultCode: ldap.ResultSuccess,
		},
		Entries: []*SearchEntry{searchEntry},
	}
}

// matchesFilter evaluates the search filter against an entry.
// Returns true if the filter matches or if no filter is specified.
func (s *BaseSearcher) matchesFilter(entry *storage.Entry, searchFilter *ldap.SearchFilter) bool {
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

// storageToFilterEntry converts a storage.Entry to a filter.Entry.
func storageToFilterEntry(entry *storage.Entry) *filter.Entry {
	filterEntry := filter.NewEntry(entry.DN)
	for name, values := range entry.Attributes {
		filterEntry.Attributes[name] = values
	}
	return filterEntry
}

// ldapFilterToFilter converts an ldap.SearchFilter to a filter.Filter.
func ldapFilterToFilter(sf *ldap.SearchFilter) *filter.Filter {
	if sf == nil {
		return nil
	}

	switch sf.Type {
	case ldap.FilterTagAnd:
		children := make([]*filter.Filter, 0, len(sf.Children))
		for _, child := range sf.Children {
			if converted := ldapFilterToFilter(child); converted != nil {
				children = append(children, converted)
			}
		}
		return filter.NewAndFilter(children...)

	case ldap.FilterTagOr:
		children := make([]*filter.Filter, 0, len(sf.Children))
		for _, child := range sf.Children {
			if converted := ldapFilterToFilter(child); converted != nil {
				children = append(children, converted)
			}
		}
		return filter.NewOrFilter(children...)

	case ldap.FilterTagNot:
		child := ldapFilterToFilter(sf.Child)
		if child == nil {
			return nil
		}
		return filter.NewNotFilter(child)

	case ldap.FilterTagEquality:
		return filter.NewEqualityFilter(sf.Attribute, sf.Value)

	case ldap.FilterTagSubstrings:
		if sf.Substrings == nil {
			return nil
		}
		return filter.NewSubstringFilter(&filter.SubstringFilter{
			Attribute: sf.Attribute,
			Initial:   sf.Substrings.Initial,
			Any:       sf.Substrings.Any,
			Final:     sf.Substrings.Final,
		})

	case ldap.FilterTagPresent:
		return filter.NewPresentFilter(sf.Attribute)

	case ldap.FilterTagGreaterOrEqual:
		return filter.NewGreaterOrEqualFilter(sf.Attribute, sf.Value)

	case ldap.FilterTagLessOrEqual:
		return filter.NewLessOrEqualFilter(sf.Attribute, sf.Value)

	case ldap.FilterTagApproxMatch:
		return filter.NewApproxMatchFilter(sf.Attribute, sf.Value)

	default:
		return nil
	}
}

// buildSearchEntryFromStorage builds a SearchEntry from a storage.Entry.
func buildSearchEntryFromStorage(entry *storage.Entry, requestedAttrs []string, typesOnly bool) *SearchEntry {
	searchEntry := &SearchEntry{
		DN: entry.DN,
	}

	// Select attributes based on the request
	selectedAttrs := SelectAttributes(entry, requestedAttrs)

	// Build the attribute list
	for name, values := range selectedAttrs {
		attr := ldap.Attribute{
			Type: name,
		}

		if !typesOnly {
			// Include attribute values
			attr.Values = values
		}
		// If typesOnly is true, Values remains nil (empty)

		searchEntry.Attributes = append(searchEntry.Attributes, attr)
	}

	return searchEntry
}

// findMatchedDNForBase finds the longest existing parent DN for error reporting.
// This is used when the base DN doesn't exist to return the closest ancestor.
func findMatchedDNForBase(dn string) string {
	// In a full implementation, this would traverse up the DN tree
	// to find the closest existing ancestor.
	// For now, return empty string as we don't have access to the full tree.
	return ""
}
