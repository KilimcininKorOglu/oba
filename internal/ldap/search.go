// Package ldap implements LDAP protocol message parsing and encoding
// as specified in RFC 4511.
package ldap

import (
	"errors"

	"github.com/KilimcininKorOglu/oba/internal/ber"
)

// SearchScope represents the scope of an LDAP search operation
type SearchScope int

const (
	// ScopeBaseObject searches only the base object
	ScopeBaseObject SearchScope = 0
	// ScopeSingleLevel searches one level below the base object
	ScopeSingleLevel SearchScope = 1
	// ScopeWholeSubtree searches the entire subtree
	ScopeWholeSubtree SearchScope = 2
)

// String returns the string representation of the search scope
func (s SearchScope) String() string {
	switch s {
	case ScopeBaseObject:
		return "BaseObject"
	case ScopeSingleLevel:
		return "SingleLevel"
	case ScopeWholeSubtree:
		return "WholeSubtree"
	default:
		return "Unknown"
	}
}

// DerefAliases represents how aliases should be dereferenced during search
type DerefAliases int

const (
	// DerefNever never dereferences aliases
	DerefNever DerefAliases = 0
	// DerefInSearching dereferences aliases when searching subordinates
	DerefInSearching DerefAliases = 1
	// DerefFindingBaseObj dereferences aliases when finding the base object
	DerefFindingBaseObj DerefAliases = 2
	// DerefAlways always dereferences aliases
	DerefAlways DerefAliases = 3
)

// String returns the string representation of the deref aliases setting
func (d DerefAliases) String() string {
	switch d {
	case DerefNever:
		return "NeverDerefAliases"
	case DerefInSearching:
		return "DerefInSearching"
	case DerefFindingBaseObj:
		return "DerefFindingBaseObj"
	case DerefAlways:
		return "DerefAlways"
	default:
		return "Unknown"
	}
}

// Filter tag numbers (context-specific) per RFC 4511
const (
	FilterTagAnd             = 0  // [0] SET OF filter
	FilterTagOr              = 1  // [1] SET OF filter
	FilterTagNot             = 2  // [2] Filter
	FilterTagEquality        = 3  // [3] AttributeValueAssertion
	FilterTagSubstrings      = 4  // [4] SubstringFilter
	FilterTagGreaterOrEqual  = 5  // [5] AttributeValueAssertion
	FilterTagLessOrEqual     = 6  // [6] AttributeValueAssertion
	FilterTagPresent         = 7  // [7] AttributeDescription
	FilterTagApproxMatch     = 8  // [8] AttributeValueAssertion
	FilterTagExtensibleMatch = 9  // [9] MatchingRuleAssertion
)

// Substring filter component tags
const (
	SubstringInitial = 0 // [0] initial
	SubstringAny     = 1 // [1] any
	SubstringFinal   = 2 // [2] final
)

// SearchFilter represents an LDAP search filter
type SearchFilter struct {
	// Type is the filter type tag
	Type int
	// Attribute is the attribute name (for comparison filters)
	Attribute string
	// Value is the assertion value (for comparison filters)
	Value []byte
	// Children contains sub-filters (for AND/OR)
	Children []*SearchFilter
	// Child contains the negated filter (for NOT)
	Child *SearchFilter
	// Substrings contains substring components (for substring filter)
	Substrings *SubstringComponents
	// ExtensibleMatch contains extensible match components
	ExtensibleMatch *ExtensibleMatchComponents
}

// SubstringComponents represents the components of a substring filter
type SubstringComponents struct {
	// Initial is the initial substring (before first *)
	Initial []byte
	// Any contains middle substrings (between *s)
	Any [][]byte
	// Final is the final substring (after last *)
	Final []byte
}

// ExtensibleMatchComponents represents the components of an extensible match filter
type ExtensibleMatchComponents struct {
	// MatchingRule is the OID of the matching rule (optional)
	MatchingRule string
	// Type is the attribute type (optional)
	Type string
	// MatchValue is the assertion value
	MatchValue []byte
	// DNAttributes if true, also match against DN attributes
	DNAttributes bool
}

// SearchRequest represents an LDAP Search Request
// SearchRequest ::= [APPLICATION 3] SEQUENCE {
//
//	baseObject      LDAPDN,
//	scope           ENUMERATED { baseObject(0), singleLevel(1), wholeSubtree(2) },
//	derefAliases    ENUMERATED { neverDerefAliases(0), derefInSearching(1),
//	                             derefFindingBaseObj(2), derefAlways(3) },
//	sizeLimit       INTEGER (0 .. maxInt),
//	timeLimit       INTEGER (0 .. maxInt),
//	typesOnly       BOOLEAN,
//	filter          Filter,
//	attributes      AttributeSelection
//
// }
type SearchRequest struct {
	// BaseObject is the base DN for the search
	BaseObject string
	// Scope is the search scope
	Scope SearchScope
	// DerefAliases specifies how aliases should be dereferenced
	DerefAliases DerefAliases
	// SizeLimit is the maximum number of entries to return (0 = no limit)
	SizeLimit int
	// TimeLimit is the maximum time in seconds (0 = no limit)
	TimeLimit int
	// TypesOnly if true, only attribute types are returned (no values)
	TypesOnly bool
	// Filter is the search filter
	Filter *SearchFilter
	// Attributes is the list of attributes to return (empty = all user attributes)
	Attributes []string
}

// Errors for SearchRequest parsing
var (
	// ErrInvalidSearchScope is returned when the search scope is invalid
	ErrInvalidSearchScope = errors.New("ldap: invalid search scope")
	// ErrInvalidDerefAliases is returned when the deref aliases value is invalid
	ErrInvalidDerefAliases = errors.New("ldap: invalid deref aliases value")
	// ErrInvalidFilter is returned when the filter is malformed
	ErrInvalidFilter = errors.New("ldap: invalid search filter")
	// ErrInvalidSubstringFilter is returned when a substring filter is malformed
	ErrInvalidSubstringFilter = errors.New("ldap: invalid substring filter")
)

// ParseSearchRequest parses a SearchRequest from raw operation data.
// The data should be the contents of the APPLICATION 3 tag (without the tag and length).
func ParseSearchRequest(data []byte) (*SearchRequest, error) {
	if len(data) == 0 {
		return nil, NewParseError(0, "empty search request data", nil)
	}

	decoder := ber.NewBERDecoder(data)
	req := &SearchRequest{}

	// Read baseObject (LDAPDN - OCTET STRING)
	baseBytes, err := decoder.ReadOctetString()
	if err != nil {
		return nil, NewParseError(decoder.Offset(), "failed to read baseObject", err)
	}
	req.BaseObject = string(baseBytes)

	// Read scope (ENUMERATED)
	scope, err := decoder.ReadEnumerated()
	if err != nil {
		return nil, NewParseError(decoder.Offset(), "failed to read scope", err)
	}
	if scope < 0 || scope > 2 {
		return nil, ErrInvalidSearchScope
	}
	req.Scope = SearchScope(scope)

	// Read derefAliases (ENUMERATED)
	deref, err := decoder.ReadEnumerated()
	if err != nil {
		return nil, NewParseError(decoder.Offset(), "failed to read derefAliases", err)
	}
	if deref < 0 || deref > 3 {
		return nil, ErrInvalidDerefAliases
	}
	req.DerefAliases = DerefAliases(deref)

	// Read sizeLimit (INTEGER)
	sizeLimit, err := decoder.ReadInteger()
	if err != nil {
		return nil, NewParseError(decoder.Offset(), "failed to read sizeLimit", err)
	}
	req.SizeLimit = int(sizeLimit)

	// Read timeLimit (INTEGER)
	timeLimit, err := decoder.ReadInteger()
	if err != nil {
		return nil, NewParseError(decoder.Offset(), "failed to read timeLimit", err)
	}
	req.TimeLimit = int(timeLimit)

	// Read typesOnly (BOOLEAN)
	typesOnly, err := decoder.ReadBoolean()
	if err != nil {
		return nil, NewParseError(decoder.Offset(), "failed to read typesOnly", err)
	}
	req.TypesOnly = typesOnly

	// Read filter (context-specific tagged)
	filter, err := parseSearchFilter(decoder)
	if err != nil {
		return nil, NewParseError(decoder.Offset(), "failed to read filter", err)
	}
	req.Filter = filter

	// Read attributes (SEQUENCE OF AttributeDescription)
	attrSeqLen, err := decoder.ExpectSequence()
	if err != nil {
		return nil, NewParseError(decoder.Offset(), "failed to read attributes sequence", err)
	}

	attrEnd := decoder.Offset() + attrSeqLen
	var attributes []string
	for decoder.Offset() < attrEnd && decoder.Remaining() > 0 {
		attrBytes, err := decoder.ReadOctetString()
		if err != nil {
			return nil, NewParseError(decoder.Offset(), "failed to read attribute", err)
		}
		attributes = append(attributes, string(attrBytes))
	}
	req.Attributes = attributes

	return req, nil
}

// parseSearchFilter parses a search filter from the decoder
func parseSearchFilter(decoder *ber.BERDecoder) (*SearchFilter, error) {
	// Read the filter using ReadTaggedValue which handles context-specific tags
	tagNum, constructed, filterData, err := decoder.ReadTaggedValue()
	if err != nil {
		return nil, err
	}

	filter := &SearchFilter{Type: tagNum}

	switch tagNum {
	case FilterTagAnd, FilterTagOr:
		// SET OF Filter
		if !constructed {
			return nil, NewParseError(decoder.Offset(), "AND/OR filter must be constructed", ErrInvalidFilter)
		}
		subDecoder := ber.NewBERDecoder(filterData)
		var children []*SearchFilter
		for subDecoder.Remaining() > 0 {
			child, err := parseSearchFilter(subDecoder)
			if err != nil {
				return nil, err
			}
			children = append(children, child)
		}
		filter.Children = children

	case FilterTagNot:
		// Filter
		if !constructed {
			return nil, NewParseError(decoder.Offset(), "NOT filter must be constructed", ErrInvalidFilter)
		}
		subDecoder := ber.NewBERDecoder(filterData)
		child, err := parseSearchFilter(subDecoder)
		if err != nil {
			return nil, err
		}
		filter.Child = child

	case FilterTagEquality, FilterTagGreaterOrEqual, FilterTagLessOrEqual, FilterTagApproxMatch:
		// AttributeValueAssertion ::= SEQUENCE { attributeDesc, assertionValue }
		if !constructed {
			return nil, NewParseError(decoder.Offset(), "comparison filter must be constructed", ErrInvalidFilter)
		}
		subDecoder := ber.NewBERDecoder(filterData)

		attrBytes, err := subDecoder.ReadOctetString()
		if err != nil {
			return nil, NewParseError(decoder.Offset(), "failed to read filter attribute", err)
		}
		filter.Attribute = string(attrBytes)

		valueBytes, err := subDecoder.ReadOctetString()
		if err != nil {
			return nil, NewParseError(decoder.Offset(), "failed to read filter value", err)
		}
		filter.Value = valueBytes

	case FilterTagSubstrings:
		// SubstringFilter ::= SEQUENCE { type, substrings SEQUENCE OF choice }
		if !constructed {
			return nil, NewParseError(decoder.Offset(), "substring filter must be constructed", ErrInvalidFilter)
		}
		subDecoder := ber.NewBERDecoder(filterData)
		substrings, attr, err := parseSubstringFilter(subDecoder)
		if err != nil {
			return nil, err
		}
		filter.Attribute = attr
		filter.Substrings = substrings

	case FilterTagPresent:
		// AttributeDescription (OCTET STRING, but primitive with context tag)
		if constructed {
			return nil, NewParseError(decoder.Offset(), "present filter must be primitive", ErrInvalidFilter)
		}
		filter.Attribute = string(filterData)

	case FilterTagExtensibleMatch:
		// MatchingRuleAssertion
		if !constructed {
			return nil, NewParseError(decoder.Offset(), "extensible match filter must be constructed", ErrInvalidFilter)
		}
		subDecoder := ber.NewBERDecoder(filterData)
		extMatch, err := parseExtensibleMatch(subDecoder)
		if err != nil {
			return nil, err
		}
		filter.ExtensibleMatch = extMatch

	default:
		return nil, NewParseError(decoder.Offset(), "unknown filter type", ErrInvalidFilter)
	}

	return filter, nil
}

// parseSubstringFilter parses a substring filter
func parseSubstringFilter(decoder *ber.BERDecoder) (*SubstringComponents, string, error) {
	// Read attribute type (OCTET STRING)
	attrBytes, err := decoder.ReadOctetString()
	if err != nil {
		return nil, "", NewParseError(decoder.Offset(), "failed to read substring attribute", err)
	}
	attr := string(attrBytes)

	// Read substrings SEQUENCE
	subSeqLen, err := decoder.ExpectSequence()
	if err != nil {
		return nil, "", NewParseError(decoder.Offset(), "failed to read substrings sequence", err)
	}

	subEnd := decoder.Offset() + subSeqLen
	components := &SubstringComponents{}

	for decoder.Offset() < subEnd {
		// Read context-specific tag
		tagNum, _, value, err := decoder.ReadTaggedValue()
		if err != nil {
			return nil, "", NewParseError(decoder.Offset(), "failed to read substring component", err)
		}

		switch tagNum {
		case SubstringInitial:
			components.Initial = value
		case SubstringAny:
			components.Any = append(components.Any, value)
		case SubstringFinal:
			components.Final = value
		default:
			return nil, "", NewParseError(decoder.Offset(), "unknown substring component tag", ErrInvalidSubstringFilter)
		}
	}

	return components, attr, nil
}

// Extensible match component tags
const (
	ExtMatchMatchingRule = 1 // [1] matchingRule
	ExtMatchType         = 2 // [2] type
	ExtMatchMatchValue   = 3 // [3] matchValue
	ExtMatchDNAttributes = 4 // [4] dnAttributes
)

// parseExtensibleMatch parses an extensible match filter
func parseExtensibleMatch(decoder *ber.BERDecoder) (*ExtensibleMatchComponents, error) {
	components := &ExtensibleMatchComponents{}

	for decoder.Remaining() > 0 {
		tagNum, _, value, err := decoder.ReadTaggedValue()
		if err != nil {
			return nil, NewParseError(decoder.Offset(), "failed to read extensible match component", err)
		}

		switch tagNum {
		case ExtMatchMatchingRule:
			components.MatchingRule = string(value)
		case ExtMatchType:
			components.Type = string(value)
		case ExtMatchMatchValue:
			components.MatchValue = value
		case ExtMatchDNAttributes:
			// Boolean encoded as single byte
			if len(value) > 0 && value[0] != 0 {
				components.DNAttributes = true
			}
		}
	}

	return components, nil
}
