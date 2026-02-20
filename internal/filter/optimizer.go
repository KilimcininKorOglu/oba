// Package filter provides LDAP search filter data structures and evaluation
// for the Oba LDAP server.
package filter

import (
	"strings"

	"github.com/oba-ldap/oba/internal/storage/index"
)

// Optimizer analyzes filters and creates optimized query plans using available indexes.
// It determines the most efficient way to execute a filter query by considering
// available indexes and estimating execution costs.
type Optimizer struct {
	indexManager *index.IndexManager
}

// NewOptimizer creates a new Optimizer with the given IndexManager.
// The IndexManager is used to check which indexes are available for optimization.
func NewOptimizer(im *index.IndexManager) *Optimizer {
	return &Optimizer{
		indexManager: im,
	}
}

// Optimize analyzes a filter and returns an optimized query plan.
// It considers available indexes and selects the most efficient execution strategy.
func (o *Optimizer) Optimize(filter *Filter) *QueryPlan {
	if filter == nil {
		return NewFullScanPlan(nil)
	}

	// If no index manager, fall back to full scan
	if o.indexManager == nil {
		return NewFullScanPlan(filter)
	}

	switch filter.Type {
	case FilterEquality:
		return o.optimizeEquality(filter)
	case FilterPresent:
		return o.optimizePresence(filter)
	case FilterSubstring:
		return o.optimizeSubstring(filter)
	case FilterAnd:
		return o.optimizeAnd(filter)
	case FilterOr:
		return o.optimizeOr(filter)
	case FilterNot:
		return o.optimizeNot(filter)
	case FilterGreaterOrEqual, FilterLessOrEqual:
		return o.optimizeRange(filter)
	default:
		return NewFullScanPlan(filter)
	}
}

// optimizeEquality optimizes an equality filter (attr=value).
// Uses an equality index if available.
func (o *Optimizer) optimizeEquality(filter *Filter) *QueryPlan {
	attr := normalizeAttr(filter.Attribute)

	// Check if we have an equality index for this attribute
	idx, exists := o.indexManager.GetIndex(attr)
	if exists && idx.Type == index.IndexEquality {
		return NewIndexPlan(
			attr,
			index.IndexEquality,
			filter.Value,
			nil, // No post-filter needed for exact equality match
			CostIndexLookup,
			filter,
		)
	}

	// No index available, fall back to full scan
	return NewFullScanPlan(filter)
}

// optimizePresence optimizes a presence filter (attr=*).
// Uses a presence index if available.
func (o *Optimizer) optimizePresence(filter *Filter) *QueryPlan {
	attr := normalizeAttr(filter.Attribute)

	// Check if we have a presence index for this attribute
	idx, exists := o.indexManager.GetIndex(attr)
	if exists && idx.Type == index.IndexPresence {
		return NewIndexPlan(
			attr,
			index.IndexPresence,
			index.PresenceMarker,
			nil,
			CostPresenceIndex,
			filter,
		)
	}

	// Check if we have an equality index - we can use it for presence
	// by scanning all keys (less efficient but still better than full scan)
	if exists && idx.Type == index.IndexEquality {
		// For equality index, we can't directly check presence
		// Fall back to full scan
		return NewFullScanPlan(filter)
	}

	return NewFullScanPlan(filter)
}

// optimizeSubstring optimizes a substring filter (attr=*value*).
// Uses a substring index if available and the pattern has searchable components.
func (o *Optimizer) optimizeSubstring(filter *Filter) *QueryPlan {
	if filter.Substring == nil {
		return NewFullScanPlan(filter)
	}

	attr := normalizeAttr(filter.Substring.Attribute)

	// Check if we have a substring index for this attribute
	idx, exists := o.indexManager.GetIndex(attr)
	if !exists || idx.Type != index.IndexSubstring {
		return NewFullScanPlan(filter)
	}

	// Extract the best searchable component from the substring filter
	lookup := o.extractSubstringLookup(filter.Substring)
	if lookup == nil {
		// No searchable component (e.g., pattern too short)
		return NewFullScanPlan(filter)
	}

	// Substring index returns candidates that need post-filtering
	return NewSubstringIndexPlan(
		attr,
		lookup,
		filter.Substring,
		filter, // Post-filter to verify actual match
		CostSubstringIndex,
		filter,
	)
}

// extractSubstringLookup extracts the best lookup key from a substring filter.
// Returns the longest component that can be used for index lookup.
func (o *Optimizer) extractSubstringLookup(sf *SubstringFilter) []byte {
	// Prefer initial (prefix) as it's most selective
	if len(sf.Initial) >= 3 {
		return sf.Initial
	}

	// Try any middle components
	for _, any := range sf.Any {
		if len(any) >= 3 {
			return any
		}
	}

	// Try final (suffix)
	if len(sf.Final) >= 3 {
		return sf.Final
	}

	return nil
}

// optimizeAnd optimizes an AND filter by selecting the best index.
// Strategy: Use the most selective index, post-filter the rest.
func (o *Optimizer) optimizeAnd(filter *Filter) *QueryPlan {
	if len(filter.Children) == 0 {
		return NewFullScanPlan(filter)
	}

	// Find the child with the lowest cost index
	var bestPlan *QueryPlan
	var bestChildIdx int = -1

	for i, child := range filter.Children {
		plan := o.Optimize(child)
		if plan.UseIndex {
			if bestPlan == nil || plan.EstimatedCost < bestPlan.EstimatedCost {
				bestPlan = plan
				bestChildIdx = i
			}
		}
	}

	// If no index can be used, fall back to full scan
	if bestPlan == nil {
		return NewFullScanPlan(filter)
	}

	// Build post-filter from remaining children
	remainingChildren := make([]*Filter, 0, len(filter.Children)-1)
	for i, child := range filter.Children {
		if i != bestChildIdx {
			remainingChildren = append(remainingChildren, child)
		}
	}

	var postFilter *Filter
	if len(remainingChildren) == 1 {
		postFilter = remainingChildren[0]
	} else if len(remainingChildren) > 1 {
		postFilter = NewAndFilter(remainingChildren...)
	}

	// Calculate total cost
	cost := bestPlan.EstimatedCost
	if postFilter != nil {
		cost += o.estimateCost(postFilter)
	}

	return &QueryPlan{
		UseIndex:         true,
		IndexAttr:        bestPlan.IndexAttr,
		IndexType:        bestPlan.IndexType,
		IndexLookup:      bestPlan.IndexLookup,
		SubstringPattern: bestPlan.SubstringPattern,
		PostFilter:       postFilter,
		EstimatedCost:    cost,
		OriginalFilter:   filter,
	}
}

// optimizeOr optimizes an OR filter.
// Strategy: If all children can use indexes, union the results.
// Otherwise, fall back to full scan.
func (o *Optimizer) optimizeOr(filter *Filter) *QueryPlan {
	if len(filter.Children) == 0 {
		return NewFullScanPlan(filter)
	}

	// Check if all children can use indexes
	allIndexed := true
	totalCost := 0

	for _, child := range filter.Children {
		plan := o.Optimize(child)
		if !plan.UseIndex {
			allIndexed = false
			break
		}
		totalCost += plan.EstimatedCost
	}

	// If not all children can use indexes, fall back to full scan
	// OR filters require all branches to be indexed for efficient execution
	if !allIndexed {
		return NewFullScanPlan(filter)
	}

	// All children can use indexes - return a plan indicating OR union
	// The executor will need to union results from each child's index lookup
	// For now, we return a full scan plan with the original filter
	// as the post-filter, since OR union execution is complex
	// and requires special handling in the executor

	// Calculate cost with union overhead
	cost := totalCost + CostOrUnion*len(filter.Children)

	// For OR filters, we return a plan that indicates the filter
	// can potentially use indexes, but the executor needs to handle
	// the union logic. We mark it as a full scan with the cost
	// reflecting the potential optimization.
	return &QueryPlan{
		UseIndex:       false,
		PostFilter:     filter,
		EstimatedCost:  cost,
		OriginalFilter: filter,
	}
}

// optimizeNot optimizes a NOT filter.
// NOT filters generally cannot use indexes efficiently.
func (o *Optimizer) optimizeNot(filter *Filter) *QueryPlan {
	// NOT filters require scanning all entries and excluding matches
	// This is inherently a full scan operation
	return NewFullScanPlan(filter)
}

// optimizeRange optimizes a range filter (>=, <=).
// Range filters can use B+ tree range scans if an index exists.
func (o *Optimizer) optimizeRange(filter *Filter) *QueryPlan {
	attr := normalizeAttr(filter.Attribute)

	// Check if we have an equality index (B+ tree supports range scans)
	idx, exists := o.indexManager.GetIndex(attr)
	if exists && idx.Type == index.IndexEquality {
		// B+ tree indexes support range scans
		return NewIndexPlan(
			attr,
			index.IndexEquality,
			filter.Value,
			filter, // Post-filter to verify range condition
			CostIndexLookup*2, // Range scans are more expensive
			filter,
		)
	}

	return NewFullScanPlan(filter)
}

// estimateCost estimates the cost of evaluating a filter without an index.
func (o *Optimizer) estimateCost(filter *Filter) int {
	if filter == nil {
		return 0
	}

	switch filter.Type {
	case FilterAnd:
		cost := 0
		for _, child := range filter.Children {
			cost += o.estimateCost(child)
		}
		return cost
	case FilterOr:
		cost := 0
		for _, child := range filter.Children {
			cost += o.estimateCost(child)
		}
		return cost
	case FilterNot:
		return o.estimateCost(filter.Child)
	case FilterEquality:
		return CostPostFilter
	case FilterPresent:
		return CostPostFilter / 2 // Presence checks are cheap
	case FilterSubstring:
		return CostPostFilter * 2 // Substring matching is expensive
	case FilterGreaterOrEqual, FilterLessOrEqual:
		return CostPostFilter
	case FilterApproxMatch:
		return CostPostFilter * 3 // Approximate matching is very expensive
	default:
		return CostPostFilter
	}
}

// canUseIndex checks if a filter can use an index and returns the index details.
// Returns the attribute name, index type, and whether an index can be used.
func (o *Optimizer) canUseIndex(filter *Filter) (string, index.IndexType, bool) {
	if filter == nil || o.indexManager == nil {
		return "", 0, false
	}

	var attr string
	switch filter.Type {
	case FilterEquality:
		attr = normalizeAttr(filter.Attribute)
		idx, exists := o.indexManager.GetIndex(attr)
		if exists && idx.Type == index.IndexEquality {
			return attr, index.IndexEquality, true
		}
	case FilterPresent:
		attr = normalizeAttr(filter.Attribute)
		idx, exists := o.indexManager.GetIndex(attr)
		if exists && idx.Type == index.IndexPresence {
			return attr, index.IndexPresence, true
		}
	case FilterSubstring:
		if filter.Substring != nil {
			attr = normalizeAttr(filter.Substring.Attribute)
			idx, exists := o.indexManager.GetIndex(attr)
			if exists && idx.Type == index.IndexSubstring {
				return attr, index.IndexSubstring, true
			}
		}
	}

	return "", 0, false
}

// normalizeAttr normalizes an attribute name for index lookup.
func normalizeAttr(attr string) string {
	return strings.ToLower(strings.TrimSpace(attr))
}
