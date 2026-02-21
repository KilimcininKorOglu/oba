// Package filter provides LDAP search filter data structures and evaluation
// for the Oba LDAP server.
package filter

import (
	"github.com/KilimcininKorOglu/oba/internal/storage/index"
)

// QueryPlan represents an optimized execution plan for a filter query.
// It determines whether to use an index for efficient lookup or fall back
// to a full scan with filter evaluation.
type QueryPlan struct {
	// UseIndex indicates whether an index should be used for this query.
	UseIndex bool

	// IndexAttr is the attribute name to use for index lookup.
	// Only valid when UseIndex is true.
	IndexAttr string

	// IndexType is the type of index to use (equality, presence, substring).
	// Only valid when UseIndex is true.
	IndexType index.IndexType

	// IndexLookup is the key to look up in the index.
	// For equality filters, this is the exact value.
	// For substring filters, this may be a prefix or n-gram.
	// Only valid when UseIndex is true.
	IndexLookup []byte

	// SubstringPattern holds the original substring filter for post-filtering.
	// Only valid for substring index lookups.
	SubstringPattern *SubstringFilter

	// PostFilter is the filter to apply after index lookup.
	// This handles conditions that couldn't be satisfied by the index alone.
	// May be nil if the index fully satisfies the query.
	PostFilter *Filter

	// EstimatedCost is the estimated cost of executing this plan.
	// Lower values indicate more efficient plans.
	// Cost units are arbitrary but consistent for comparison.
	EstimatedCost int

	// OriginalFilter is the original filter that was optimized.
	// Kept for reference and debugging.
	OriginalFilter *Filter
}

// Cost constants for query planning.
const (
	// CostFullScan is the base cost for a full table scan.
	CostFullScan = 10000

	// CostIndexLookup is the base cost for an index lookup.
	CostIndexLookup = 10

	// CostPostFilter is the additional cost per post-filter condition.
	CostPostFilter = 100

	// CostOrUnion is the cost multiplier for OR filter unions.
	CostOrUnion = 50

	// CostSubstringIndex is the cost for substring index lookup.
	// Higher than equality because it may return false positives.
	CostSubstringIndex = 50

	// CostPresenceIndex is the cost for presence index lookup.
	CostPresenceIndex = 30
)

// NewFullScanPlan creates a query plan that performs a full scan.
// This is used when no suitable index is available.
func NewFullScanPlan(filter *Filter) *QueryPlan {
	return &QueryPlan{
		UseIndex:       false,
		PostFilter:     filter,
		EstimatedCost:  CostFullScan,
		OriginalFilter: filter,
	}
}

// NewIndexPlan creates a query plan that uses an index lookup.
func NewIndexPlan(attr string, indexType index.IndexType, lookup []byte, postFilter *Filter, cost int, original *Filter) *QueryPlan {
	return &QueryPlan{
		UseIndex:       true,
		IndexAttr:      attr,
		IndexType:      indexType,
		IndexLookup:    lookup,
		PostFilter:     postFilter,
		EstimatedCost:  cost,
		OriginalFilter: original,
	}
}

// NewSubstringIndexPlan creates a query plan for substring index lookup.
func NewSubstringIndexPlan(attr string, lookup []byte, pattern *SubstringFilter, postFilter *Filter, cost int, original *Filter) *QueryPlan {
	return &QueryPlan{
		UseIndex:         true,
		IndexAttr:        attr,
		IndexType:        index.IndexSubstring,
		IndexLookup:      lookup,
		SubstringPattern: pattern,
		PostFilter:       postFilter,
		EstimatedCost:    cost,
		OriginalFilter:   original,
	}
}

// IsFullScan returns true if this plan requires a full table scan.
func (p *QueryPlan) IsFullScan() bool {
	return !p.UseIndex
}

// HasPostFilter returns true if post-filtering is required after index lookup.
func (p *QueryPlan) HasPostFilter() bool {
	return p.PostFilter != nil
}

// String returns a human-readable description of the query plan.
func (p *QueryPlan) String() string {
	if !p.UseIndex {
		return "FULL_SCAN"
	}

	result := "INDEX_LOOKUP(" + p.IndexAttr + ", " + p.IndexType.String() + ")"
	if p.PostFilter != nil {
		result += " + POST_FILTER"
	}
	return result
}
