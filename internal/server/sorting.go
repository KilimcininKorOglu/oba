// Package server provides the LDAP server implementation.
package server

import (
	"sort"
	"strings"

	"github.com/KilimcininKorOglu/oba/internal/storage"
)

// SortRequestOID is the OID for the Server-Side Sorting Request Control (RFC 2891).
const SortRequestOID = "1.2.840.113556.1.4.473"

// SortResponseOID is the OID for the Server-Side Sorting Response Control (RFC 2891).
const SortResponseOID = "1.2.840.113556.1.4.474"

// SortResult codes as defined in RFC 2891.
const (
	// SortResultSuccess indicates the results are sorted as requested.
	SortResultSuccess = 0
	// SortResultOperationsError indicates an internal error during sorting.
	SortResultOperationsError = 1
	// SortResultTimeLimitExceeded indicates the time limit was exceeded during sorting.
	SortResultTimeLimitExceeded = 3
	// SortResultStrongAuthRequired indicates strong authentication is required.
	SortResultStrongAuthRequired = 8
	// SortResultAdminLimitExceeded indicates an administrative limit was exceeded.
	SortResultAdminLimitExceeded = 11
	// SortResultNoSuchAttribute indicates the sort attribute does not exist.
	SortResultNoSuchAttribute = 16
	// SortResultInappropriateMatching indicates the matching rule is inappropriate.
	SortResultInappropriateMatching = 18
	// SortResultInsufficientAccessRights indicates insufficient access rights.
	SortResultInsufficientAccessRights = 50
	// SortResultBusy indicates the server is too busy to sort.
	SortResultBusy = 51
	// SortResultUnwillingToPerform indicates the server is unwilling to sort.
	SortResultUnwillingToPerform = 53
	// SortResultOther indicates an unspecified error.
	SortResultOther = 80
)

// SortKey represents a single sort key for ordering search results.
type SortKey struct {
	// Attribute is the attribute name to sort by.
	Attribute string

	// OrderingRule is an optional matching rule OID for comparison.
	// If empty, the default ordering rule for the attribute is used.
	OrderingRule string

	// Reverse indicates whether to sort in descending order.
	// If false, results are sorted in ascending order.
	Reverse bool
}

// SortControl represents the Server-Side Sorting Control.
type SortControl struct {
	// Keys is the list of sort keys in priority order.
	// The first key is the primary sort key, the second is secondary, etc.
	Keys []SortKey
}

// NewSortControl creates a new SortControl with the given sort keys.
func NewSortControl(keys ...SortKey) *SortControl {
	return &SortControl{
		Keys: keys,
	}
}

// NewSortKey creates a new SortKey for the given attribute.
func NewSortKey(attribute string) SortKey {
	return SortKey{
		Attribute: attribute,
	}
}

// NewReverseSortKey creates a new SortKey for the given attribute in reverse order.
func NewReverseSortKey(attribute string) SortKey {
	return SortKey{
		Attribute: attribute,
		Reverse:   true,
	}
}

// SortResponse represents the response to a sort request.
type SortResponse struct {
	// ResultCode is the sort result code.
	ResultCode int

	// AttributeType is the attribute that caused the error (if any).
	AttributeType string
}

// SortEntries sorts a slice of storage.Entry according to the sort control.
// Returns the sorted entries (the original slice is modified in place).
func SortEntries(entries []*storage.Entry, ctrl *SortControl) []*storage.Entry {
	if ctrl == nil || len(ctrl.Keys) == 0 || len(entries) <= 1 {
		return entries
	}

	sort.SliceStable(entries, func(i, j int) bool {
		return compareEntries(entries[i], entries[j], ctrl.Keys) < 0
	})

	return entries
}

// SortSearchEntries sorts a slice of SearchEntry according to the sort control.
// Returns the sorted entries (the original slice is modified in place).
func SortSearchEntries(entries []*SearchEntry, ctrl *SortControl) []*SearchEntry {
	if ctrl == nil || len(ctrl.Keys) == 0 || len(entries) <= 1 {
		return entries
	}

	sort.SliceStable(entries, func(i, j int) bool {
		return compareSearchEntries(entries[i], entries[j], ctrl.Keys) < 0
	})

	return entries
}

// compareEntries compares two storage.Entry objects using the sort keys.
// Returns negative if a < b, positive if a > b, and 0 if equal.
func compareEntries(a, b *storage.Entry, keys []SortKey) int {
	for _, key := range keys {
		cmp := compareAttributeWithReverse(a, b, key.Attribute, key.Reverse)
		if cmp == 0 {
			continue
		}
		return cmp
	}
	return 0
}

// compareSearchEntries compares two SearchEntry objects using the sort keys.
// Returns negative if a < b, positive if a > b, and 0 if equal.
func compareSearchEntries(a, b *SearchEntry, keys []SortKey) int {
	for _, key := range keys {
		cmp := compareSearchEntryAttributeWithReverse(a, b, key.Attribute, key.Reverse)
		if cmp == 0 {
			continue
		}
		return cmp
	}
	return 0
}

// compareAttribute compares a single attribute between two storage.Entry objects.
// Returns negative if a < b, positive if a > b, and 0 if equal.
// Missing attributes are sorted after present attributes.
// The reverse parameter indicates if the sort is reversed, which affects value comparison
// but not the handling of missing attributes (missing always sorts last).
func compareAttributeWithReverse(a, b *storage.Entry, attr string, reverse bool) int {
	va := getFirstAttributeValue(a, attr)
	vb := getFirstAttributeValue(b, attr)

	// Handle missing attributes: entries without the attribute sort last
	// This is independent of the reverse flag
	aHas := va != ""
	bHas := vb != ""

	if !aHas && !bHas {
		return 0
	}
	if !aHas {
		return 1 // a sorts after b (missing values always last)
	}
	if !bHas {
		return -1 // a sorts before b (missing values always last)
	}

	cmp := strings.Compare(va, vb)
	if reverse {
		return -cmp
	}
	return cmp
}

// compareSearchEntryAttribute compares a single attribute between two SearchEntry objects.
// Returns negative if a < b, positive if a > b, and 0 if equal.
// Missing attributes are sorted after present attributes.
// The reverse parameter indicates if the sort is reversed, which affects value comparison
// but not the handling of missing attributes (missing always sorts last).
func compareSearchEntryAttributeWithReverse(a, b *SearchEntry, attr string, reverse bool) int {
	va := getSearchEntryFirstValue(a, attr)
	vb := getSearchEntryFirstValue(b, attr)

	// Handle missing attributes: entries without the attribute sort last
	// This is independent of the reverse flag
	aHas := va != ""
	bHas := vb != ""

	if !aHas && !bHas {
		return 0
	}
	if !aHas {
		return 1 // a sorts after b (missing values always last)
	}
	if !bHas {
		return -1 // a sorts before b (missing values always last)
	}

	cmp := strings.Compare(va, vb)
	if reverse {
		return -cmp
	}
	return cmp
}

// getFirstAttributeValue returns the first value of an attribute from a storage.Entry.
// Returns an empty string if the attribute doesn't exist or has no values.
func getFirstAttributeValue(entry *storage.Entry, attr string) string {
	if entry == nil || entry.Attributes == nil {
		return ""
	}

	// Try exact match first
	if values, ok := entry.Attributes[attr]; ok && len(values) > 0 {
		return string(values[0])
	}

	// Try case-insensitive match
	lowerAttr := strings.ToLower(attr)
	for name, values := range entry.Attributes {
		if strings.ToLower(name) == lowerAttr && len(values) > 0 {
			return string(values[0])
		}
	}

	return ""
}

// getSearchEntryFirstValue returns the first value of an attribute from a SearchEntry.
// Returns an empty string if the attribute doesn't exist or has no values.
func getSearchEntryFirstValue(entry *SearchEntry, attr string) string {
	if entry == nil {
		return ""
	}

	lowerAttr := strings.ToLower(attr)
	for _, a := range entry.Attributes {
		if strings.ToLower(a.Type) == lowerAttr && len(a.Values) > 0 {
			return string(a.Values[0])
		}
	}

	return ""
}

// ValidateSortControl validates a sort control and returns a SortResponse.
// Returns nil if the control is valid.
func ValidateSortControl(ctrl *SortControl) *SortResponse {
	if ctrl == nil {
		return nil
	}

	for _, key := range ctrl.Keys {
		if key.Attribute == "" {
			return &SortResponse{
				ResultCode:    SortResultNoSuchAttribute,
				AttributeType: "",
			}
		}
	}

	return nil
}
