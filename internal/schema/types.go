// Package schema provides LDAP schema data structures including object classes,
// attribute types, syntaxes, and matching rules.
package schema

// Schema represents the complete LDAP schema containing all definitions
// for object classes, attribute types, syntaxes, and matching rules.
type Schema struct {
	ObjectClasses  map[string]*ObjectClass
	AttributeTypes map[string]*AttributeType
	Syntaxes       map[string]*Syntax
	MatchingRules  map[string]*MatchingRule
}

// NewSchema creates a new empty Schema with initialized maps.
func NewSchema() *Schema {
	return &Schema{
		ObjectClasses:  make(map[string]*ObjectClass),
		AttributeTypes: make(map[string]*AttributeType),
		Syntaxes:       make(map[string]*Syntax),
		MatchingRules:  make(map[string]*MatchingRule),
	}
}

// MatchingRule defines how attribute values are compared for equality,
// ordering, and substring matching operations.
type MatchingRule struct {
	OID         string
	Name        string
	Names       []string // Aliases
	Description string
	Syntax      string // Syntax OID this rule applies to
	Obsolete    bool
}

// NewMatchingRule creates a new MatchingRule with the given OID and name.
func NewMatchingRule(oid, name string) *MatchingRule {
	return &MatchingRule{
		OID:   oid,
		Name:  name,
		Names: []string{name},
	}
}

// GetObjectClass retrieves an object class by name or OID.
// Returns nil if not found.
func (s *Schema) GetObjectClass(nameOrOID string) *ObjectClass {
	if oc, ok := s.ObjectClasses[nameOrOID]; ok {
		return oc
	}
	// Search by alias
	for _, oc := range s.ObjectClasses {
		for _, alias := range oc.Names {
			if alias == nameOrOID {
				return oc
			}
		}
	}
	return nil
}

// GetAttributeType retrieves an attribute type by name or OID.
// Returns nil if not found.
func (s *Schema) GetAttributeType(nameOrOID string) *AttributeType {
	if at, ok := s.AttributeTypes[nameOrOID]; ok {
		return at
	}
	// Search by alias
	for _, at := range s.AttributeTypes {
		for _, alias := range at.Names {
			if alias == nameOrOID {
				return at
			}
		}
	}
	return nil
}

// GetSyntax retrieves a syntax by OID.
// Returns nil if not found.
func (s *Schema) GetSyntax(oid string) *Syntax {
	return s.Syntaxes[oid]
}

// GetMatchingRule retrieves a matching rule by name or OID.
// Returns nil if not found.
func (s *Schema) GetMatchingRule(nameOrOID string) *MatchingRule {
	if mr, ok := s.MatchingRules[nameOrOID]; ok {
		return mr
	}
	// Search by alias
	for _, mr := range s.MatchingRules {
		for _, alias := range mr.Names {
			if alias == nameOrOID {
				return mr
			}
		}
	}
	return nil
}

// AddObjectClass adds an object class to the schema.
// It registers the object class by both OID and primary name.
func (s *Schema) AddObjectClass(oc *ObjectClass) {
	if oc.OID != "" {
		s.ObjectClasses[oc.OID] = oc
	}
	if oc.Name != "" {
		s.ObjectClasses[oc.Name] = oc
	}
}

// AddAttributeType adds an attribute type to the schema.
// It registers the attribute type by both OID and primary name.
func (s *Schema) AddAttributeType(at *AttributeType) {
	if at.OID != "" {
		s.AttributeTypes[at.OID] = at
	}
	if at.Name != "" {
		s.AttributeTypes[at.Name] = at
	}
}

// AddSyntax adds a syntax to the schema by its OID.
func (s *Schema) AddSyntax(syn *Syntax) {
	if syn.OID != "" {
		s.Syntaxes[syn.OID] = syn
	}
}

// AddMatchingRule adds a matching rule to the schema.
// It registers the matching rule by both OID and primary name.
func (s *Schema) AddMatchingRule(mr *MatchingRule) {
	if mr.OID != "" {
		s.MatchingRules[mr.OID] = mr
	}
	if mr.Name != "" {
		s.MatchingRules[mr.Name] = mr
	}
}
