package schema

import (
	"bufio"
	"errors"
	"io"
	"os"
	"strings"
)

// Loader errors
var (
	ErrSchemaFileNotFound = errors.New("schema file not found")
	ErrInvalidLDIF        = errors.New("invalid LDIF format")
	ErrInheritanceCycle   = errors.New("inheritance cycle detected")
)

// LoadSchema loads a schema from an LDIF file at the given path.
func LoadSchema(path string) (*Schema, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrSchemaFileNotFound
		}
		return nil, err
	}
	defer file.Close()

	return LoadSchemaFromLDIF(file)
}

// LoadSchemaFromLDIF loads a schema from an LDIF-formatted reader.
// The LDIF format for schema entries follows RFC 4512.
//
// Example LDIF schema entry:
//
//	dn: cn=schema
//	objectClass: top
//	objectClass: ldapSubentry
//	objectClass: subschema
//	objectClasses: ( 2.5.6.0 NAME 'top' ABSTRACT MUST objectClass )
//	attributeTypes: ( 2.5.4.0 NAME 'objectClass' EQUALITY objectIdentifierMatch ... )
func LoadSchemaFromLDIF(r io.Reader) (*Schema, error) {
	s := NewSchema()

	scanner := bufio.NewScanner(r)
	var currentAttr string
	var currentValue strings.Builder

	processValue := func() error {
		if currentAttr == "" {
			return nil
		}

		value := strings.TrimSpace(currentValue.String())
		if value == "" {
			return nil
		}

		var err error
		switch strings.ToLower(currentAttr) {
		case "attributetypes":
			var at *AttributeType
			at, err = parseAttributeType(value)
			if err == nil {
				s.AddAttributeType(at)
			}
		case "objectclasses":
			var oc *ObjectClass
			oc, err = parseObjectClass(value)
			if err == nil {
				s.AddObjectClass(oc)
			}
		case "matchingrules":
			var mr *MatchingRule
			mr, err = parseMatchingRule(value)
			if err == nil {
				s.AddMatchingRule(mr)
			}
		case "ldapsyntaxes":
			var syn *Syntax
			syn, err = parseSyntaxDef(value)
			if err == nil {
				s.AddSyntax(syn)
			}
		}

		currentAttr = ""
		currentValue.Reset()
		return err
	}

	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			if err := processValue(); err != nil {
				return nil, err
			}
			continue
		}

		// Handle line continuation (line starting with space)
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			// Add a space before continuation to preserve token separation
			currentValue.WriteString(" ")
			currentValue.WriteString(strings.TrimLeft(line, " \t"))
			continue
		}

		// Process previous attribute before starting new one
		if err := processValue(); err != nil {
			return nil, err
		}

		// Parse attribute: value
		colonIdx := strings.Index(line, ":")
		if colonIdx == -1 {
			continue
		}

		attrName := strings.TrimSpace(line[:colonIdx])
		attrValue := strings.TrimSpace(line[colonIdx+1:])

		// Handle base64 encoded values (attribute:: value)
		if strings.HasSuffix(attrName, ":") {
			attrName = strings.TrimSuffix(attrName, ":")
			// Base64 decoding would go here if needed
		}

		currentAttr = attrName
		currentValue.WriteString(attrValue)
	}

	// Process last attribute
	if err := processValue(); err != nil {
		return nil, err
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Resolve inheritance
	if err := resolveObjectClassInheritance(s); err != nil {
		return nil, err
	}
	if err := resolveAttributeTypeInheritance(s); err != nil {
		return nil, err
	}

	return s, nil
}

// LoadDefaultSchema loads the built-in default schema with standard
// LDAP object classes and attribute types.
func LoadDefaultSchema() *Schema {
	s := NewSchema()

	// Load in order: syntaxes, matching rules, attribute types, object classes
	// This ensures dependencies are available when needed
	_ = loadDefaultSyntaxes(s)
	_ = loadDefaultMatchingRules(s)
	_ = loadDefaultAttributeTypes(s)
	_ = loadDefaultObjectClasses(s)

	// Resolve inheritance
	_ = resolveObjectClassInheritance(s)
	_ = resolveAttributeTypeInheritance(s)

	return s
}

// resolveObjectClassInheritance resolves the inheritance chain for all object classes.
// It collects MUST and MAY attributes from all superiors.
func resolveObjectClassInheritance(s *Schema) error {
	resolved := make(map[string]bool)

	var resolve func(oc *ObjectClass, visited map[string]bool) error
	resolve = func(oc *ObjectClass, visited map[string]bool) error {
		if oc == nil {
			return nil
		}

		key := oc.OID
		if key == "" {
			key = oc.Name
		}

		if resolved[key] {
			return nil
		}

		if visited[key] {
			return ErrInheritanceCycle
		}
		visited[key] = true

		if oc.Superior != "" {
			sup := s.GetObjectClass(oc.Superior)
			if sup != nil {
				if err := resolve(sup, visited); err != nil {
					return err
				}
			}
		}

		resolved[key] = true
		return nil
	}

	for _, oc := range s.ObjectClasses {
		visited := make(map[string]bool)
		if err := resolve(oc, visited); err != nil {
			return err
		}
	}

	return nil
}

// resolveAttributeTypeInheritance resolves the inheritance chain for all attribute types.
// It inherits syntax and matching rules from superiors.
func resolveAttributeTypeInheritance(s *Schema) error {
	resolved := make(map[string]bool)

	var resolve func(at *AttributeType, visited map[string]bool) error
	resolve = func(at *AttributeType, visited map[string]bool) error {
		if at == nil {
			return nil
		}

		key := at.OID
		if key == "" {
			key = at.Name
		}

		if resolved[key] {
			return nil
		}

		if visited[key] {
			return ErrInheritanceCycle
		}
		visited[key] = true

		if at.Superior != "" {
			sup := s.GetAttributeType(at.Superior)
			if sup != nil {
				if err := resolve(sup, visited); err != nil {
					return err
				}

				// Inherit properties from superior if not set
				if at.Syntax == "" {
					at.Syntax = sup.Syntax
				}
				if at.Equality == "" {
					at.Equality = sup.Equality
				}
				if at.Ordering == "" {
					at.Ordering = sup.Ordering
				}
				if at.Substring == "" {
					at.Substring = sup.Substring
				}
			}
		}

		resolved[key] = true
		return nil
	}

	for _, at := range s.AttributeTypes {
		visited := make(map[string]bool)
		if err := resolve(at, visited); err != nil {
			return err
		}
	}

	return nil
}

// GetAllMustAttributes returns all required attributes for an object class,
// including those inherited from superiors.
func (s *Schema) GetAllMustAttributes(ocName string) []string {
	oc := s.GetObjectClass(ocName)
	if oc == nil {
		return nil
	}

	seen := make(map[string]bool)
	var result []string

	var collect func(oc *ObjectClass)
	collect = func(oc *ObjectClass) {
		if oc == nil {
			return
		}

		// Collect from superior first
		if oc.Superior != "" {
			sup := s.GetObjectClass(oc.Superior)
			collect(sup)
		}

		// Add this class's MUST attributes
		for _, attr := range oc.Must {
			if !seen[attr] {
				seen[attr] = true
				result = append(result, attr)
			}
		}
	}

	collect(oc)
	return result
}

// GetAllMayAttributes returns all optional attributes for an object class,
// including those inherited from superiors.
func (s *Schema) GetAllMayAttributes(ocName string) []string {
	oc := s.GetObjectClass(ocName)
	if oc == nil {
		return nil
	}

	seen := make(map[string]bool)
	var result []string

	var collect func(oc *ObjectClass)
	collect = func(oc *ObjectClass) {
		if oc == nil {
			return
		}

		// Collect from superior first
		if oc.Superior != "" {
			sup := s.GetObjectClass(oc.Superior)
			collect(sup)
		}

		// Add this class's MAY attributes
		for _, attr := range oc.May {
			if !seen[attr] {
				seen[attr] = true
				result = append(result, attr)
			}
		}
	}

	collect(oc)
	return result
}

// GetEffectiveSyntax returns the effective syntax OID for an attribute type,
// resolving inheritance if necessary.
func (s *Schema) GetEffectiveSyntax(atName string) string {
	at := s.GetAttributeType(atName)
	if at == nil {
		return ""
	}

	// Walk up the inheritance chain
	for at != nil {
		if at.Syntax != "" {
			return at.Syntax
		}
		if at.Superior == "" {
			break
		}
		at = s.GetAttributeType(at.Superior)
	}

	return ""
}

// GetEffectiveEqualityMatch returns the effective equality matching rule
// for an attribute type, resolving inheritance if necessary.
func (s *Schema) GetEffectiveEqualityMatch(atName string) string {
	at := s.GetAttributeType(atName)
	if at == nil {
		return ""
	}

	// Walk up the inheritance chain
	for at != nil {
		if at.Equality != "" {
			return at.Equality
		}
		if at.Superior == "" {
			break
		}
		at = s.GetAttributeType(at.Superior)
	}

	return ""
}
