package schema

import (
	"errors"
	"strings"
)

// Parser errors
var (
	ErrInvalidObjectClass   = errors.New("invalid object class definition")
	ErrInvalidAttributeType = errors.New("invalid attribute type definition")
	ErrMissingOID           = errors.New("missing OID in definition")
	ErrUnterminatedString   = errors.New("unterminated quoted string")
	ErrUnterminatedParens   = errors.New("unterminated parentheses")
)

// parseObjectClass parses an LDAP object class definition string.
// Format: ( OID NAME 'name' SUP superior KIND MUST (attr1 $ attr2) MAY (attr3) )
func parseObjectClass(s string) (*ObjectClass, error) {
	s = strings.TrimSpace(s)
	if len(s) < 2 || s[0] != '(' || s[len(s)-1] != ')' {
		return nil, ErrInvalidObjectClass
	}

	// Remove outer parentheses
	s = strings.TrimSpace(s[1 : len(s)-1])

	tokens, err := tokenize(s)
	if err != nil {
		return nil, err
	}

	if len(tokens) == 0 {
		return nil, ErrMissingOID
	}

	oc := &ObjectClass{
		OID:  tokens[0],
		Kind: ObjectClassStructural, // Default kind
		Must: []string{},
		May:  []string{},
	}

	i := 1
	for i < len(tokens) {
		keyword := strings.ToUpper(tokens[i])
		switch keyword {
		case "NAME":
			i++
			if i >= len(tokens) {
				return nil, ErrInvalidObjectClass
			}
			names := parseNames(tokens[i])
			if len(names) > 0 {
				oc.Name = names[0]
				oc.Names = names
			}
		case "DESC":
			i++
			if i >= len(tokens) {
				return nil, ErrInvalidObjectClass
			}
			oc.Desc = unquote(tokens[i])
		case "OBSOLETE":
			oc.Obsolete = true
		case "SUP":
			i++
			if i >= len(tokens) {
				return nil, ErrInvalidObjectClass
			}
			oc.Superior = unquote(tokens[i])
		case "ABSTRACT":
			oc.Kind = ObjectClassAbstract
		case "STRUCTURAL":
			oc.Kind = ObjectClassStructural
		case "AUXILIARY":
			oc.Kind = ObjectClassAuxiliary
		case "MUST":
			i++
			if i >= len(tokens) {
				return nil, ErrInvalidObjectClass
			}
			oc.Must = parseAttributeList(tokens[i])
		case "MAY":
			i++
			if i >= len(tokens) {
				return nil, ErrInvalidObjectClass
			}
			oc.May = parseAttributeList(tokens[i])
		}
		i++
	}

	return oc, nil
}

// parseAttributeType parses an LDAP attribute type definition string.
// Format: ( OID NAME 'name' SYNTAX syntaxOID SINGLE-VALUE ... )
func parseAttributeType(s string) (*AttributeType, error) {
	s = strings.TrimSpace(s)
	if len(s) < 2 || s[0] != '(' || s[len(s)-1] != ')' {
		return nil, ErrInvalidAttributeType
	}

	// Remove outer parentheses
	s = strings.TrimSpace(s[1 : len(s)-1])

	tokens, err := tokenize(s)
	if err != nil {
		return nil, err
	}

	if len(tokens) == 0 {
		return nil, ErrMissingOID
	}

	at := &AttributeType{
		OID:   tokens[0],
		Usage: UserApplications, // Default usage
	}

	i := 1
	for i < len(tokens) {
		keyword := strings.ToUpper(tokens[i])
		switch keyword {
		case "NAME":
			i++
			if i >= len(tokens) {
				return nil, ErrInvalidAttributeType
			}
			names := parseNames(tokens[i])
			if len(names) > 0 {
				at.Name = names[0]
				at.Names = names
			}
		case "DESC":
			i++
			if i >= len(tokens) {
				return nil, ErrInvalidAttributeType
			}
			at.Desc = unquote(tokens[i])
		case "OBSOLETE":
			at.Obsolete = true
		case "SUP":
			i++
			if i >= len(tokens) {
				return nil, ErrInvalidAttributeType
			}
			at.Superior = unquote(tokens[i])
		case "EQUALITY":
			i++
			if i >= len(tokens) {
				return nil, ErrInvalidAttributeType
			}
			at.Equality = unquote(tokens[i])
		case "ORDERING":
			i++
			if i >= len(tokens) {
				return nil, ErrInvalidAttributeType
			}
			at.Ordering = unquote(tokens[i])
		case "SUBSTR":
			i++
			if i >= len(tokens) {
				return nil, ErrInvalidAttributeType
			}
			at.Substring = unquote(tokens[i])
		case "SYNTAX":
			i++
			if i >= len(tokens) {
				return nil, ErrInvalidAttributeType
			}
			// Syntax may include length constraint like "1.3.6.1.4.1.1466.115.121.1.15{256}"
			at.Syntax = parseSyntaxOID(tokens[i])
		case "SINGLE-VALUE":
			at.SingleValue = true
		case "COLLECTIVE":
			at.Collective = true
		case "NO-USER-MODIFICATION":
			at.NoUserMod = true
		case "USAGE":
			i++
			if i >= len(tokens) {
				return nil, ErrInvalidAttributeType
			}
			at.Usage = parseUsage(tokens[i])
		}
		i++
	}

	return at, nil
}

// tokenize splits a schema definition into tokens, handling quoted strings and parentheses.
func tokenize(s string) ([]string, error) {
	var tokens []string
	var current strings.Builder
	inQuote := false
	parenDepth := 0

	for i := 0; i < len(s); i++ {
		ch := s[i]

		if inQuote {
			current.WriteByte(ch)
			if ch == '\'' {
				inQuote = false
			}
			continue
		}

		switch ch {
		case '\'':
			inQuote = true
			current.WriteByte(ch)
		case '(':
			if parenDepth > 0 {
				current.WriteByte(ch)
			}
			parenDepth++
		case ')':
			parenDepth--
			if parenDepth > 0 {
				current.WriteByte(ch)
			} else if parenDepth == 0 && current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		case ' ', '\t', '\n', '\r':
			if parenDepth > 0 {
				current.WriteByte(ch)
			} else if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		case '$':
			if parenDepth > 0 {
				current.WriteByte(ch)
			}
		default:
			current.WriteByte(ch)
		}
	}

	if inQuote {
		return nil, ErrUnterminatedString
	}
	if parenDepth != 0 {
		return nil, ErrUnterminatedParens
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens, nil
}

// parseNames parses a NAME value which can be a single quoted string or a list.
// Examples: 'cn' or ( 'cn' 'commonName' )
func parseNames(s string) []string {
	s = strings.TrimSpace(s)

	// Check if it's a list (starts with content from parentheses)
	if strings.Contains(s, "'") {
		var names []string
		inQuote := false
		var current strings.Builder

		for i := 0; i < len(s); i++ {
			ch := s[i]
			if ch == '\'' {
				if inQuote {
					if current.Len() > 0 {
						names = append(names, current.String())
						current.Reset()
					}
				}
				inQuote = !inQuote
			} else if inQuote {
				current.WriteByte(ch)
			}
		}
		return names
	}

	// Single unquoted name
	return []string{s}
}

// parseAttributeList parses a list of attribute names.
// Examples: attr1 or ( attr1 $ attr2 $ attr3 )
func parseAttributeList(s string) []string {
	s = strings.TrimSpace(s)

	// Split by $ separator
	parts := strings.Split(s, "$")
	var attrs []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		part = unquote(part)
		if part != "" {
			attrs = append(attrs, part)
		}
	}
	return attrs
}

// unquote removes surrounding single quotes from a string.
func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '\'' && s[len(s)-1] == '\'' {
		return s[1 : len(s)-1]
	}
	return s
}

// parseSyntaxOID extracts the OID from a syntax specification.
// Handles length constraints like "1.3.6.1.4.1.1466.115.121.1.15{256}"
func parseSyntaxOID(s string) string {
	s = unquote(s)
	if idx := strings.Index(s, "{"); idx != -1 {
		return s[:idx]
	}
	return s
}

// parseUsage parses an attribute usage value.
func parseUsage(s string) AttributeUsage {
	switch strings.ToLower(unquote(s)) {
	case "userapplications":
		return UserApplications
	case "directoryoperation":
		return DirectoryOperation
	case "distributedoperation":
		return DistributedOperation
	case "dsaoperation":
		return DSAOperation
	default:
		return UserApplications
	}
}

// parseMatchingRule parses an LDAP matching rule definition string.
// Format: ( OID NAME 'name' SYNTAX syntaxOID )
func parseMatchingRule(s string) (*MatchingRule, error) {
	s = strings.TrimSpace(s)
	if len(s) < 2 || s[0] != '(' || s[len(s)-1] != ')' {
		return nil, errors.New("invalid matching rule definition")
	}

	// Remove outer parentheses
	s = strings.TrimSpace(s[1 : len(s)-1])

	tokens, err := tokenize(s)
	if err != nil {
		return nil, err
	}

	if len(tokens) == 0 {
		return nil, ErrMissingOID
	}

	mr := &MatchingRule{
		OID: tokens[0],
	}

	i := 1
	for i < len(tokens) {
		keyword := strings.ToUpper(tokens[i])
		switch keyword {
		case "NAME":
			i++
			if i >= len(tokens) {
				return nil, errors.New("invalid matching rule definition")
			}
			names := parseNames(tokens[i])
			if len(names) > 0 {
				mr.Name = names[0]
				mr.Names = names
			}
		case "DESC":
			i++
			if i >= len(tokens) {
				return nil, errors.New("invalid matching rule definition")
			}
			mr.Description = unquote(tokens[i])
		case "OBSOLETE":
			mr.Obsolete = true
		case "SYNTAX":
			i++
			if i >= len(tokens) {
				return nil, errors.New("invalid matching rule definition")
			}
			mr.Syntax = parseSyntaxOID(tokens[i])
		}
		i++
	}

	return mr, nil
}

// parseSyntaxDef parses an LDAP syntax definition string.
// Format: ( OID DESC 'description' )
func parseSyntaxDef(s string) (*Syntax, error) {
	s = strings.TrimSpace(s)
	if len(s) < 2 || s[0] != '(' || s[len(s)-1] != ')' {
		return nil, errors.New("invalid syntax definition")
	}

	// Remove outer parentheses
	s = strings.TrimSpace(s[1 : len(s)-1])

	tokens, err := tokenize(s)
	if err != nil {
		return nil, err
	}

	if len(tokens) == 0 {
		return nil, ErrMissingOID
	}

	syn := &Syntax{
		OID: tokens[0],
	}

	i := 1
	for i < len(tokens) {
		keyword := strings.ToUpper(tokens[i])
		switch keyword {
		case "DESC":
			i++
			if i >= len(tokens) {
				return nil, errors.New("invalid syntax definition")
			}
			syn.Description = unquote(tokens[i])
		}
		i++
	}

	return syn, nil
}
