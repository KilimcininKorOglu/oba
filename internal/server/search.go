// Package server provides the LDAP server implementation.
package server

import (
	"strings"

	"github.com/KilimcininKorOglu/oba/internal/acl"
	"github.com/KilimcininKorOglu/oba/internal/filter"
	"github.com/KilimcininKorOglu/oba/internal/ldap"
	"github.com/KilimcininKorOglu/oba/internal/storage"
)

// SearchBackend defines the interface for search operations.
// It extends the basic Backend interface with search capabilities.
type SearchBackend interface {
	Backend
	// SearchByDN searches for entries by DN with the given scope.
	// Returns an iterator over matching entries.
	SearchByDN(baseDN string, scope storage.Scope) storage.Iterator
}

// SearchConfig holds configuration for the search handler.
type SearchConfig struct {
	// Backend is the directory backend for entry lookups.
	Backend Backend
	// SearchBackend is the directory backend for search operations.
	// If nil, Backend is used (if it implements SearchBackend).
	SearchBackend SearchBackend
	// MaxSizeLimit is the maximum number of entries to return (0 = unlimited).
	MaxSizeLimit int
	// MaxTimeLimit is the maximum time limit in seconds (0 = unlimited).
	MaxTimeLimit int
	// DefaultSizeLimit is the default size limit if client doesn't specify one.
	DefaultSizeLimit int
	// DefaultTimeLimit is the default time limit if client doesn't specify one.
	DefaultTimeLimit int
	// ACLEvaluator is the ACL evaluator for access control checks.
	// If nil, no ACL checks are performed.
	ACLEvaluator *acl.Evaluator
}

// NewSearchConfig creates a new SearchConfig with default settings.
func NewSearchConfig() *SearchConfig {
	return &SearchConfig{
		MaxSizeLimit:     1000,
		MaxTimeLimit:     60,
		DefaultSizeLimit: 100,
		DefaultTimeLimit: 30,
	}
}

// SearchHandlerImpl implements the search operation handler.
type SearchHandlerImpl struct {
	config           *SearchConfig
	evaluator        *filter.Evaluator
	oneLevelSearcher *OneLevelSearcher
	subtreeSearcher  *SubtreeSearcher
}

// NewSearchHandler creates a new search handler with the given configuration.
func NewSearchHandler(config *SearchConfig) *SearchHandlerImpl {
	if config == nil {
		config = NewSearchConfig()
	}

	handler := &SearchHandlerImpl{
		config:    config,
		evaluator: filter.NewEvaluator(nil),
	}

	// Initialize searchers if SearchBackend is available
	if config.SearchBackend != nil {
		handler.oneLevelSearcher = NewOneLevelSearcher(config.SearchBackend)
		handler.subtreeSearcher = NewSubtreeSearcher(config.SearchBackend)
	} else if sb, ok := config.Backend.(SearchBackend); ok {
		handler.oneLevelSearcher = NewOneLevelSearcher(sb)
		handler.subtreeSearcher = NewSubtreeSearcher(sb)
	}

	return handler
}

// Handle processes a search request and returns the result.
// It implements the SearchHandler function signature.
func (h *SearchHandlerImpl) Handle(conn *Connection, req *ldap.SearchRequest) *SearchResult {
	// Validate the request
	if err := h.validateRequest(req); err != nil {
		return &SearchResult{
			OperationResult: OperationResult{
				ResultCode:        ldap.ResultProtocolError,
				DiagnosticMessage: err.Error(),
			},
		}
	}

	// Check if backend is configured
	if h.config.Backend == nil {
		return &SearchResult{
			OperationResult: OperationResult{
				ResultCode:        ldap.ResultOperationsError,
				DiagnosticMessage: "backend not configured",
			},
		}
	}

	// Check ACL search permission on the base DN
	if h.config.ACLEvaluator != nil {
		bindDN := ""
		if conn != nil {
			bindDN = conn.BindDN()
		}
		if !h.config.ACLEvaluator.CanSearch(bindDN, req.BaseObject) {
			return &SearchResult{
				OperationResult: OperationResult{
					ResultCode:        ldap.ResultInsufficientAccessRights,
					DiagnosticMessage: "insufficient access rights",
				},
			}
		}
	}

	// Dispatch based on search scope
	var result *SearchResult
	switch req.Scope {
	case ldap.ScopeBaseObject:
		result = h.searchBase(conn, req)
	case ldap.ScopeSingleLevel:
		result = h.searchOneLevel(conn, req)
	case ldap.ScopeWholeSubtree:
		result = h.searchSubtree(conn, req)
	default:
		return &SearchResult{
			OperationResult: OperationResult{
				ResultCode:        ldap.ResultProtocolError,
				DiagnosticMessage: "invalid search scope",
			},
		}
	}

	// Filter attributes based on read permission
	if h.config.ACLEvaluator != nil && result != nil && result.Entries != nil {
		bindDN := ""
		if conn != nil {
			bindDN = conn.BindDN()
		}
		result.Entries = h.filterEntriesByACL(bindDN, result.Entries)
	}

	return result
}

// searchOneLevel performs a one-level scope search (returns immediate children).
func (h *SearchHandlerImpl) searchOneLevel(conn *Connection, req *ldap.SearchRequest) *SearchResult {
	if h.oneLevelSearcher == nil {
		return &SearchResult{
			OperationResult: OperationResult{
				ResultCode:        ldap.ResultUnwillingToPerform,
				DiagnosticMessage: "single level search not configured",
			},
		}
	}
	return h.oneLevelSearcher.Search(req, h.config)
}

// searchSubtree performs a subtree scope search (returns base and all descendants).
func (h *SearchHandlerImpl) searchSubtree(conn *Connection, req *ldap.SearchRequest) *SearchResult {
	if h.subtreeSearcher == nil {
		return &SearchResult{
			OperationResult: OperationResult{
				ResultCode:        ldap.ResultUnwillingToPerform,
				DiagnosticMessage: "subtree search not configured",
			},
		}
	}
	return h.subtreeSearcher.Search(req, h.config)
}

// validateRequest validates the search request parameters.
func (h *SearchHandlerImpl) validateRequest(req *ldap.SearchRequest) error {
	// Base DN can be empty (root DSE search)
	// Scope is validated by the parser

	// Validate size limit
	if req.SizeLimit < 0 {
		return ldap.ErrInvalidSearchScope
	}

	// Validate time limit
	if req.TimeLimit < 0 {
		return ldap.ErrInvalidSearchScope
	}

	return nil
}

// searchBase performs a base scope search (returns single entry matching base DN).
func (h *SearchHandlerImpl) searchBase(conn *Connection, req *ldap.SearchRequest) *SearchResult {
	// Look up the entry by DN
	entry, err := h.config.Backend.GetEntry(req.BaseObject)
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
				MatchedDN:  findMatchedDN(req.BaseObject),
			},
		}
	}

	// Convert storage.Entry to filter.Entry for filter evaluation
	filterEntry := convertToFilterEntry(entry)

	// Convert ldap.SearchFilter to filter.Filter
	f := convertSearchFilter(req.Filter)

	// Evaluate the filter against the entry
	if f != nil && !h.evaluator.Evaluate(f, filterEntry) {
		// Filter doesn't match - return success with no entries
		return &SearchResult{
			OperationResult: OperationResult{
				ResultCode: ldap.ResultSuccess,
			},
			Entries: nil,
		}
	}

	// Build the search result entry with attribute selection
	searchEntry := buildSearchEntry(entry, req.Attributes, req.TypesOnly)

	return &SearchResult{
		OperationResult: OperationResult{
			ResultCode: ldap.ResultSuccess,
		},
		Entries: []*SearchEntry{searchEntry},
	}
}

// convertToFilterEntry converts a storage.Entry to a filter.Entry.
func convertToFilterEntry(entry *storage.Entry) *filter.Entry {
	filterEntry := filter.NewEntry(entry.DN)
	for name, values := range entry.Attributes {
		filterEntry.Attributes[name] = values
	}
	return filterEntry
}

// convertSearchFilter converts an ldap.SearchFilter to a filter.Filter.
func convertSearchFilter(sf *ldap.SearchFilter) *filter.Filter {
	if sf == nil {
		return nil
	}

	switch sf.Type {
	case ldap.FilterTagAnd:
		children := make([]*filter.Filter, len(sf.Children))
		for i, child := range sf.Children {
			children[i] = convertSearchFilter(child)
		}
		return filter.NewAndFilter(children...)

	case ldap.FilterTagOr:
		children := make([]*filter.Filter, len(sf.Children))
		for i, child := range sf.Children {
			children[i] = convertSearchFilter(child)
		}
		return filter.NewOrFilter(children...)

	case ldap.FilterTagNot:
		return filter.NewNotFilter(convertSearchFilter(sf.Child))

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

// findMatchedDN finds the longest existing parent DN for error reporting.
// For now, returns empty string as we don't have access to the full tree.
func findMatchedDN(dn string) string {
	// In a full implementation, this would traverse up the DN tree
	// to find the closest existing ancestor.
	// For now, return empty string.
	return ""
}

// buildSearchEntry builds a SearchEntry from a storage.Entry with attribute selection.
func buildSearchEntry(entry *storage.Entry, requestedAttrs []string, typesOnly bool) *SearchEntry {
	searchEntry := &SearchEntry{
		DN: entry.DN,
	}

	// Select attributes based on the request
	selectedAttrs := selectAttributes(entry, requestedAttrs)

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

// selectAttributes selects attributes from an entry based on the requested attribute list.
func selectAttributes(entry *storage.Entry, requestedAttrs []string) map[string][][]byte {
	// If no attributes requested, return all user attributes
	if len(requestedAttrs) == 0 {
		return entry.Attributes
	}

	// Check for special attribute selectors
	hasAllUser := false
	hasAllOp := false
	specificAttrs := make([]string, 0, len(requestedAttrs))

	for _, attr := range requestedAttrs {
		switch strings.ToLower(attr) {
		case "*":
			hasAllUser = true
		case "+":
			hasAllOp = true
		default:
			specificAttrs = append(specificAttrs, attr)
		}
	}

	result := make(map[string][][]byte)

	// If "*" is requested, include all user attributes
	if hasAllUser {
		for name, values := range entry.Attributes {
			if !isOperationalAttribute(name) {
				result[name] = values
			}
		}
	}

	// If "+" is requested, include all operational attributes
	if hasAllOp {
		for name, values := range entry.Attributes {
			if isOperationalAttribute(name) {
				result[name] = values
			}
		}
	}

	// Add specifically requested attributes
	for _, attrName := range specificAttrs {
		// Case-insensitive attribute lookup
		for name, values := range entry.Attributes {
			if strings.EqualFold(name, attrName) {
				result[name] = values
				break
			}
		}
	}

	return result
}

// isOperationalAttribute checks if an attribute is an operational attribute.
func isOperationalAttribute(name string) bool {
	// List of common operational attributes
	operationalAttrs := map[string]bool{
		"createtimestamp":       true,
		"modifytimestamp":       true,
		"creatorsname":          true,
		"modifiersname":         true,
		"entrydn":               true,
		"entryuuid":             true,
		"subschemasubentry":     true,
		"hassubordinates":       true,
		"numsubordinates":       true,
		"structuralobjectclass": true,
	}

	return operationalAttrs[strings.ToLower(name)]
}

// CreateSearchHandler creates a SearchHandler function from a SearchHandlerImpl.
// This allows the SearchHandlerImpl to be used with the Handler's SetSearchHandler method.
func CreateSearchHandler(impl *SearchHandlerImpl) SearchHandler {
	return func(conn *Connection, req *ldap.SearchRequest) *SearchResult {
		return impl.Handle(conn, req)
	}
}

// filterEntriesByACL filters search result entries based on read permissions.
// It removes entries the user cannot read and filters attributes within each entry.
func (h *SearchHandlerImpl) filterEntriesByACL(bindDN string, entries []*SearchEntry) []*SearchEntry {
	if h.config.ACLEvaluator == nil {
		return entries
	}

	filtered := make([]*SearchEntry, 0, len(entries))
	for _, entry := range entries {
		// Check if user can read this entry
		if !h.config.ACLEvaluator.CanRead(bindDN, entry.DN) {
			continue
		}

		// Filter attributes based on read permission
		filteredEntry := h.filterEntryAttributes(bindDN, entry)
		if filteredEntry != nil {
			filtered = append(filtered, filteredEntry)
		}
	}

	return filtered
}

// filterEntryAttributes filters attributes in a search entry based on read permissions.
func (h *SearchHandlerImpl) filterEntryAttributes(bindDN string, entry *SearchEntry) *SearchEntry {
	if h.config.ACLEvaluator == nil || entry == nil {
		return entry
	}

	// Create access context for attribute filtering
	ctx := &acl.AccessContext{
		BindDN:    bindDN,
		TargetDN:  entry.DN,
		Operation: acl.Read,
	}

	filteredAttrs := make([]ldap.Attribute, 0, len(entry.Attributes))
	for _, attr := range entry.Attributes {
		if h.config.ACLEvaluator.CheckAttributeAccess(ctx, attr.Type) {
			filteredAttrs = append(filteredAttrs, attr)
		}
	}

	return &SearchEntry{
		DN:         entry.DN,
		Attributes: filteredAttrs,
	}
}
