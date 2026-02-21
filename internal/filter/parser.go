package filter

import (
	"errors"
	"strings"
)

// Parser errors
var (
	ErrEmptyFilter      = errors.New("empty filter")
	ErrInvalidFilter    = errors.New("invalid filter syntax")
	ErrUnbalancedParens = errors.New("unbalanced parentheses")
	ErrMissingAttribute = errors.New("missing attribute name")
	ErrMissingValue     = errors.New("missing filter value")
)

// Parse parses an LDAP filter string into a Filter structure.
// Supports RFC 4515 filter syntax:
//   - (attr=value)     - equality
//   - (attr=*)         - presence
//   - (attr=*val*)     - substring
//   - (attr>=value)    - greater or equal
//   - (attr<=value)    - less or equal
//   - (attr~=value)    - approximate match
//   - (&(f1)(f2)...)   - AND
//   - (|(f1)(f2)...)   - OR
//   - (!(filter))      - NOT
func Parse(filterStr string) (*Filter, error) {
	filterStr = strings.TrimSpace(filterStr)
	if filterStr == "" {
		return nil, ErrEmptyFilter
	}

	return parseFilter(filterStr)
}

func parseFilter(s string) (*Filter, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, ErrEmptyFilter
	}

	// Must start and end with parentheses
	if !strings.HasPrefix(s, "(") || !strings.HasSuffix(s, ")") {
		// Try wrapping simple filters
		if !strings.Contains(s, "(") {
			s = "(" + s + ")"
		} else {
			return nil, ErrInvalidFilter
		}
	}

	// Remove outer parentheses
	inner := s[1 : len(s)-1]
	if inner == "" {
		return nil, ErrEmptyFilter
	}

	// Check for composite filters
	switch inner[0] {
	case '&':
		return parseAndFilter(inner[1:])
	case '|':
		return parseOrFilter(inner[1:])
	case '!':
		return parseNotFilter(inner[1:])
	default:
		return parseSimpleFilter(inner)
	}
}

func parseAndFilter(s string) (*Filter, error) {
	children, err := parseFilterList(s)
	if err != nil {
		return nil, err
	}
	if len(children) == 0 {
		return nil, ErrInvalidFilter
	}
	return NewAndFilter(children...), nil
}

func parseOrFilter(s string) (*Filter, error) {
	children, err := parseFilterList(s)
	if err != nil {
		return nil, err
	}
	if len(children) == 0 {
		return nil, ErrInvalidFilter
	}
	return NewOrFilter(children...), nil
}

func parseNotFilter(s string) (*Filter, error) {
	s = strings.TrimSpace(s)
	child, err := parseFilter(s)
	if err != nil {
		return nil, err
	}
	return NewNotFilter(child), nil
}

func parseFilterList(s string) ([]*Filter, error) {
	var filters []*Filter
	s = strings.TrimSpace(s)

	for len(s) > 0 {
		if s[0] != '(' {
			return nil, ErrInvalidFilter
		}

		// Find matching closing paren
		depth := 0
		end := -1
		for i, c := range s {
			if c == '(' {
				depth++
			} else if c == ')' {
				depth--
				if depth == 0 {
					end = i
					break
				}
			}
		}

		if end == -1 {
			return nil, ErrUnbalancedParens
		}

		filterStr := s[:end+1]
		f, err := parseFilter(filterStr)
		if err != nil {
			return nil, err
		}
		filters = append(filters, f)

		s = strings.TrimSpace(s[end+1:])
	}

	return filters, nil
}

func parseSimpleFilter(s string) (*Filter, error) {
	// Check for different operators
	if idx := strings.Index(s, ">="); idx > 0 {
		attr := strings.TrimSpace(s[:idx])
		value := s[idx+2:]
		if attr == "" {
			return nil, ErrMissingAttribute
		}
		return NewGreaterOrEqualFilter(attr, []byte(value)), nil
	}

	if idx := strings.Index(s, "<="); idx > 0 {
		attr := strings.TrimSpace(s[:idx])
		value := s[idx+2:]
		if attr == "" {
			return nil, ErrMissingAttribute
		}
		return NewLessOrEqualFilter(attr, []byte(value)), nil
	}

	if idx := strings.Index(s, "~="); idx > 0 {
		attr := strings.TrimSpace(s[:idx])
		value := s[idx+2:]
		if attr == "" {
			return nil, ErrMissingAttribute
		}
		return NewApproxMatchFilter(attr, []byte(value)), nil
	}

	// Equality or substring or presence
	idx := strings.Index(s, "=")
	if idx <= 0 {
		return nil, ErrInvalidFilter
	}

	attr := strings.TrimSpace(s[:idx])
	value := s[idx+1:]

	if attr == "" {
		return nil, ErrMissingAttribute
	}

	// Presence filter: (attr=*)
	if value == "*" {
		return NewPresentFilter(attr), nil
	}

	// Check for substring filter
	if strings.Contains(value, "*") {
		return parseSubstringFilter(attr, value)
	}

	// Simple equality
	return NewEqualityFilter(attr, []byte(value)), nil
}

func parseSubstringFilter(attr, value string) (*Filter, error) {
	parts := strings.Split(value, "*")

	sf := &SubstringFilter{
		Attribute: attr,
	}

	for i, part := range parts {
		if part == "" {
			continue
		}

		if i == 0 {
			// First non-empty part is initial
			sf.Initial = []byte(part)
		} else if i == len(parts)-1 {
			// Last non-empty part is final
			sf.Final = []byte(part)
		} else {
			// Middle parts are "any"
			sf.Any = append(sf.Any, []byte(part))
		}
	}

	// Handle edge cases
	if !strings.HasPrefix(value, "*") && len(parts) > 0 && parts[0] != "" {
		sf.Initial = []byte(parts[0])
	}
	if !strings.HasSuffix(value, "*") && len(parts) > 0 && parts[len(parts)-1] != "" {
		sf.Final = []byte(parts[len(parts)-1])
	}

	return NewSubstringFilter(sf), nil
}
