// Package filter provides LDAP search filter data structures and evaluation
// for the Oba LDAP server.
package filter

// FilterType represents the type of LDAP filter operation.
type FilterType int

const (
	// FilterAnd represents an AND filter (&).
	FilterAnd FilterType = iota
	// FilterOr represents an OR filter (|).
	FilterOr
	// FilterNot represents a NOT filter (!).
	FilterNot
	// FilterEquality represents an equality filter (attr=value).
	FilterEquality
	// FilterSubstring represents a substring filter (attr=*value*).
	FilterSubstring
	// FilterGreaterOrEqual represents a greater-or-equal filter (attr>=value).
	FilterGreaterOrEqual
	// FilterLessOrEqual represents a less-or-equal filter (attr<=value).
	FilterLessOrEqual
	// FilterPresent represents a presence filter (attr=*).
	FilterPresent
	// FilterApproxMatch represents an approximate match filter (attr~=value).
	FilterApproxMatch
	// FilterExtensibleMatch represents an extensible match filter.
	FilterExtensibleMatch
)

// String returns the string representation of the FilterType.
func (ft FilterType) String() string {
	switch ft {
	case FilterAnd:
		return "AND"
	case FilterOr:
		return "OR"
	case FilterNot:
		return "NOT"
	case FilterEquality:
		return "EQUALITY"
	case FilterSubstring:
		return "SUBSTRING"
	case FilterGreaterOrEqual:
		return "GREATER_OR_EQUAL"
	case FilterLessOrEqual:
		return "LESS_OR_EQUAL"
	case FilterPresent:
		return "PRESENT"
	case FilterApproxMatch:
		return "APPROX_MATCH"
	case FilterExtensibleMatch:
		return "EXTENSIBLE_MATCH"
	default:
		return "UNKNOWN"
	}
}

// Filter represents an LDAP search filter.
type Filter struct {
	Type      FilterType
	Attribute string
	Value     []byte
	Children  []*Filter        // For AND/OR filters
	Child     *Filter          // For NOT filter
	Substring *SubstringFilter // For substring filters
}

// SubstringFilter represents the components of a substring filter.
type SubstringFilter struct {
	Attribute string
	Initial   []byte   // Initial substring (before first *)
	Any       [][]byte // Middle substrings (between *s)
	Final     []byte   // Final substring (after last *)
}

// NewAndFilter creates a new AND filter with the given children.
func NewAndFilter(children ...*Filter) *Filter {
	return &Filter{
		Type:     FilterAnd,
		Children: children,
	}
}

// NewOrFilter creates a new OR filter with the given children.
func NewOrFilter(children ...*Filter) *Filter {
	return &Filter{
		Type:     FilterOr,
		Children: children,
	}
}

// NewNotFilter creates a new NOT filter with the given child.
func NewNotFilter(child *Filter) *Filter {
	return &Filter{
		Type:  FilterNot,
		Child: child,
	}
}

// NewEqualityFilter creates a new equality filter.
func NewEqualityFilter(attribute string, value []byte) *Filter {
	return &Filter{
		Type:      FilterEquality,
		Attribute: attribute,
		Value:     value,
	}
}

// NewSubstringFilter creates a new substring filter.
func NewSubstringFilter(sf *SubstringFilter) *Filter {
	return &Filter{
		Type:      FilterSubstring,
		Attribute: sf.Attribute,
		Substring: sf,
	}
}

// NewPresentFilter creates a new presence filter.
func NewPresentFilter(attribute string) *Filter {
	return &Filter{
		Type:      FilterPresent,
		Attribute: attribute,
	}
}

// NewGreaterOrEqualFilter creates a new greater-or-equal filter.
func NewGreaterOrEqualFilter(attribute string, value []byte) *Filter {
	return &Filter{
		Type:      FilterGreaterOrEqual,
		Attribute: attribute,
		Value:     value,
	}
}

// NewLessOrEqualFilter creates a new less-or-equal filter.
func NewLessOrEqualFilter(attribute string, value []byte) *Filter {
	return &Filter{
		Type:      FilterLessOrEqual,
		Attribute: attribute,
		Value:     value,
	}
}

// NewApproxMatchFilter creates a new approximate match filter.
func NewApproxMatchFilter(attribute string, value []byte) *Filter {
	return &Filter{
		Type:      FilterApproxMatch,
		Attribute: attribute,
		Value:     value,
	}
}

// Entry represents an LDAP entry for filter evaluation.
// This is a simplified interface to avoid circular dependencies.
type Entry struct {
	DN         string
	Attributes map[string][][]byte
}

// NewEntry creates a new Entry with the given DN.
func NewEntry(dn string) *Entry {
	return &Entry{
		DN:         dn,
		Attributes: make(map[string][][]byte),
	}
}

// SetAttribute sets an attribute value on the entry.
func (e *Entry) SetAttribute(name string, values ...[]byte) {
	e.Attributes[name] = values
}

// SetStringAttribute sets a string attribute value on the entry.
func (e *Entry) SetStringAttribute(name string, values ...string) {
	byteValues := make([][]byte, len(values))
	for i, v := range values {
		byteValues[i] = []byte(v)
	}
	e.Attributes[name] = byteValues
}

// GetAttribute returns the values for an attribute.
func (e *Entry) GetAttribute(name string) [][]byte {
	return e.Attributes[name]
}

// HasAttribute checks if the entry has the given attribute.
func (e *Entry) HasAttribute(name string) bool {
	_, ok := e.Attributes[name]
	return ok
}

// Clone creates a deep copy of the entry.
func (e *Entry) Clone() *Entry {
	if e == nil {
		return nil
	}

	clone := &Entry{
		DN:         e.DN,
		Attributes: make(map[string][][]byte, len(e.Attributes)),
	}

	for k, v := range e.Attributes {
		values := make([][]byte, len(v))
		for i, val := range v {
			values[i] = make([]byte, len(val))
			copy(values[i], val)
		}
		clone.Attributes[k] = values
	}

	return clone
}
