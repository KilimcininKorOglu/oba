package filter

import (
	"github.com/KilimcininKorOglu/oba/internal/schema"
)

// Evaluator evaluates LDAP search filters against entries.
type Evaluator struct {
	schema *schema.Schema
}

// NewEvaluator creates a new filter evaluator with the given schema.
// The schema is used for attribute syntax matching. If nil, default
// case-insensitive string matching is used.
func NewEvaluator(s *schema.Schema) *Evaluator {
	return &Evaluator{
		schema: s,
	}
}

// Evaluate tests whether an entry matches a filter.
// Returns true if the entry matches the filter, false otherwise.
func (e *Evaluator) Evaluate(filter *Filter, entry *Entry) bool {
	if filter == nil || entry == nil {
		return false
	}

	switch filter.Type {
	case FilterAnd:
		return e.evaluateAnd(filter, entry)
	case FilterOr:
		return e.evaluateOr(filter, entry)
	case FilterNot:
		return e.evaluateNot(filter, entry)
	case FilterEquality:
		return e.evaluateEquality(filter.Attribute, filter.Value, entry)
	case FilterSubstring:
		return e.evaluateSubstring(filter.Substring, entry)
	case FilterPresent:
		return e.evaluatePresent(filter.Attribute, entry)
	case FilterGreaterOrEqual:
		return e.evaluateGreaterOrEqual(filter.Attribute, filter.Value, entry)
	case FilterLessOrEqual:
		return e.evaluateLessOrEqual(filter.Attribute, filter.Value, entry)
	case FilterApproxMatch:
		return e.evaluateApproxMatch(filter.Attribute, filter.Value, entry)
	default:
		return false
	}
}

// evaluateAnd evaluates an AND filter.
// Returns true only if all children match.
func (e *Evaluator) evaluateAnd(filter *Filter, entry *Entry) bool {
	// Empty AND filter matches everything (vacuous truth)
	if len(filter.Children) == 0 {
		return true
	}

	for _, child := range filter.Children {
		if !e.Evaluate(child, entry) {
			return false
		}
	}
	return true
}

// evaluateOr evaluates an OR filter.
// Returns true if any child matches.
func (e *Evaluator) evaluateOr(filter *Filter, entry *Entry) bool {
	// Empty OR filter matches nothing
	if len(filter.Children) == 0 {
		return false
	}

	for _, child := range filter.Children {
		if e.Evaluate(child, entry) {
			return true
		}
	}
	return false
}

// evaluateNot evaluates a NOT filter.
// Returns the negation of the child filter result.
func (e *Evaluator) evaluateNot(filter *Filter, entry *Entry) bool {
	if filter.Child == nil {
		return false
	}
	return !e.Evaluate(filter.Child, entry)
}

// evaluateEquality tests if an entry has an attribute with the given value.
// Uses case-insensitive matching for string attributes.
func (e *Evaluator) evaluateEquality(attr string, value []byte, entry *Entry) bool {
	values := e.getAttributeValues(attr, entry)
	if values == nil {
		return false
	}

	for _, v := range values {
		if matchEquality(v, value) {
			return true
		}
	}
	return false
}

// evaluateSubstring tests if an entry has an attribute matching the substring pattern.
func (e *Evaluator) evaluateSubstring(sf *SubstringFilter, entry *Entry) bool {
	if sf == nil {
		return false
	}

	values := e.getAttributeValues(sf.Attribute, entry)
	if values == nil {
		return false
	}

	for _, v := range values {
		if matchSubstring(v, sf.Initial, sf.Any, sf.Final) {
			return true
		}
	}
	return false
}

// evaluatePresent tests if an entry has the specified attribute.
func (e *Evaluator) evaluatePresent(attr string, entry *Entry) bool {
	values := e.getAttributeValues(attr, entry)
	return values != nil && len(values) > 0
}

// evaluateGreaterOrEqual tests if an entry has an attribute >= the given value.
func (e *Evaluator) evaluateGreaterOrEqual(attr string, value []byte, entry *Entry) bool {
	values := e.getAttributeValues(attr, entry)
	if values == nil {
		return false
	}

	for _, v := range values {
		if matchGreaterOrEqual(v, value) {
			return true
		}
	}
	return false
}

// evaluateLessOrEqual tests if an entry has an attribute <= the given value.
func (e *Evaluator) evaluateLessOrEqual(attr string, value []byte, entry *Entry) bool {
	values := e.getAttributeValues(attr, entry)
	if values == nil {
		return false
	}

	for _, v := range values {
		if matchLessOrEqual(v, value) {
			return true
		}
	}
	return false
}

// evaluateApproxMatch tests if an entry has an attribute approximately matching the value.
func (e *Evaluator) evaluateApproxMatch(attr string, value []byte, entry *Entry) bool {
	values := e.getAttributeValues(attr, entry)
	if values == nil {
		return false
	}

	for _, v := range values {
		if matchApprox(v, value) {
			return true
		}
	}
	return false
}

// getAttributeValues retrieves attribute values from an entry.
// Performs case-insensitive attribute name lookup.
func (e *Evaluator) getAttributeValues(attr string, entry *Entry) [][]byte {
	// First try exact match
	if values, ok := entry.Attributes[attr]; ok {
		return values
	}

	// Try case-insensitive match
	attrLower := normalizeAttributeName(attr)
	for name, values := range entry.Attributes {
		if normalizeAttributeName(name) == attrLower {
			return values
		}
	}

	return nil
}

// GetSchema returns the evaluator's schema.
func (e *Evaluator) GetSchema() *schema.Schema {
	return e.schema
}

// SetSchema sets the evaluator's schema.
func (e *Evaluator) SetSchema(s *schema.Schema) {
	e.schema = s
}
