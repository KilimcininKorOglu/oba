package schema

import "testing"

func TestNewSchema(t *testing.T) {
	s := NewSchema()

	if s.ObjectClasses == nil {
		t.Error("ObjectClasses map should be initialized")
	}
	if s.AttributeTypes == nil {
		t.Error("AttributeTypes map should be initialized")
	}
	if s.Syntaxes == nil {
		t.Error("Syntaxes map should be initialized")
	}
	if s.MatchingRules == nil {
		t.Error("MatchingRules map should be initialized")
	}
}

func TestNewMatchingRule(t *testing.T) {
	mr := NewMatchingRule("2.5.13.2", "caseIgnoreMatch")

	if mr.OID != "2.5.13.2" {
		t.Errorf("expected OID '2.5.13.2', got '%s'", mr.OID)
	}
	if mr.Name != "caseIgnoreMatch" {
		t.Errorf("expected Name 'caseIgnoreMatch', got '%s'", mr.Name)
	}
	if len(mr.Names) != 1 || mr.Names[0] != "caseIgnoreMatch" {
		t.Error("Names should contain the primary name")
	}
}

func TestSchemaAddAndGetObjectClass(t *testing.T) {
	s := NewSchema()
	oc := NewObjectClass("2.5.6.6", "person")
	oc.Names = []string{"person", "Person"}

	s.AddObjectClass(oc)

	// Get by OID
	result := s.GetObjectClass("2.5.6.6")
	if result == nil {
		t.Error("should find object class by OID")
	}

	// Get by name
	result = s.GetObjectClass("person")
	if result == nil {
		t.Error("should find object class by name")
	}

	// Get by alias
	result = s.GetObjectClass("Person")
	if result == nil {
		t.Error("should find object class by alias")
	}

	// Not found
	result = s.GetObjectClass("nonexistent")
	if result != nil {
		t.Error("should return nil for nonexistent object class")
	}
}

func TestSchemaAddAndGetAttributeType(t *testing.T) {
	s := NewSchema()
	at := NewAttributeType("2.5.4.3", "cn")
	at.Names = []string{"cn", "commonName"}

	s.AddAttributeType(at)

	// Get by OID
	result := s.GetAttributeType("2.5.4.3")
	if result == nil {
		t.Error("should find attribute type by OID")
	}

	// Get by name
	result = s.GetAttributeType("cn")
	if result == nil {
		t.Error("should find attribute type by name")
	}

	// Get by alias
	result = s.GetAttributeType("commonName")
	if result == nil {
		t.Error("should find attribute type by alias")
	}

	// Not found
	result = s.GetAttributeType("nonexistent")
	if result != nil {
		t.Error("should return nil for nonexistent attribute type")
	}
}

func TestSchemaAddAndGetSyntax(t *testing.T) {
	s := NewSchema()
	syn := NewSyntax(SyntaxDirectoryString, "Directory String")

	s.AddSyntax(syn)

	// Get by OID
	result := s.GetSyntax(SyntaxDirectoryString)
	if result == nil {
		t.Error("should find syntax by OID")
	}

	// Not found
	result = s.GetSyntax("nonexistent")
	if result != nil {
		t.Error("should return nil for nonexistent syntax")
	}
}

func TestSchemaAddAndGetMatchingRule(t *testing.T) {
	s := NewSchema()
	mr := NewMatchingRule("2.5.13.2", "caseIgnoreMatch")
	mr.Names = []string{"caseIgnoreMatch", "CaseIgnoreMatch"}

	s.AddMatchingRule(mr)

	// Get by OID
	result := s.GetMatchingRule("2.5.13.2")
	if result == nil {
		t.Error("should find matching rule by OID")
	}

	// Get by name
	result = s.GetMatchingRule("caseIgnoreMatch")
	if result == nil {
		t.Error("should find matching rule by name")
	}

	// Get by alias
	result = s.GetMatchingRule("CaseIgnoreMatch")
	if result == nil {
		t.Error("should find matching rule by alias")
	}

	// Not found
	result = s.GetMatchingRule("nonexistent")
	if result != nil {
		t.Error("should return nil for nonexistent matching rule")
	}
}

func TestSchemaEmptyOIDOrName(t *testing.T) {
	s := NewSchema()

	// Object class with empty OID
	oc := &ObjectClass{Name: "test"}
	s.AddObjectClass(oc)
	if s.GetObjectClass("test") == nil {
		t.Error("should add object class by name when OID is empty")
	}

	// Attribute type with empty name
	at := &AttributeType{OID: "1.2.3.4"}
	s.AddAttributeType(at)
	if s.GetAttributeType("1.2.3.4") == nil {
		t.Error("should add attribute type by OID when name is empty")
	}

	// Syntax with empty OID should not be added
	syn := &Syntax{Description: "test"}
	s.AddSyntax(syn)
	if len(s.Syntaxes) != 0 {
		t.Error("should not add syntax with empty OID")
	}
}
