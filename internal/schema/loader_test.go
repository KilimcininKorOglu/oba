package schema

import (
	"strings"
	"testing"
)

func TestParseObjectClass(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantOID  string
		wantName string
		wantKind ObjectClassKind
		wantMust []string
		wantMay  []string
		wantSup  string
		wantErr  bool
	}{
		{
			name:     "simple object class",
			input:    "( 2.5.6.0 NAME 'top' ABSTRACT MUST objectClass )",
			wantOID:  "2.5.6.0",
			wantName: "top",
			wantKind: ObjectClassAbstract,
			wantMust: []string{"objectClass"},
			wantMay:  []string{},
		},
		{
			name:     "structural with superior",
			input:    "( 2.5.6.6 NAME 'person' SUP top STRUCTURAL MUST ( sn $ cn ) MAY ( userPassword $ telephoneNumber ) )",
			wantOID:  "2.5.6.6",
			wantName: "person",
			wantKind: ObjectClassStructural,
			wantMust: []string{"sn", "cn"},
			wantMay:  []string{"userPassword", "telephoneNumber"},
			wantSup:  "top",
		},
		{
			name:     "auxiliary class",
			input:    "( 1.3.6.1.4.1.1466.344 NAME 'dcObject' SUP top AUXILIARY MUST dc )",
			wantOID:  "1.3.6.1.4.1.1466.344",
			wantName: "dcObject",
			wantKind: ObjectClassAuxiliary,
			wantMust: []string{"dc"},
			wantMay:  []string{},
			wantSup:  "top",
		},
		{
			name:     "multiple names",
			input:    "( 2.5.6.1 NAME ( 'alias' 'aliasObject' ) SUP top STRUCTURAL MUST aliasedObjectName )",
			wantOID:  "2.5.6.1",
			wantName: "alias",
			wantKind: ObjectClassStructural,
			wantMust: []string{"aliasedObjectName"},
			wantMay:  []string{},
			wantSup:  "top",
		},
		{
			name:    "invalid - no parentheses",
			input:   "2.5.6.0 NAME 'top' ABSTRACT",
			wantErr: true,
		},
		{
			name:    "invalid - empty",
			input:   "()",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oc, err := parseObjectClass(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if oc.OID != tt.wantOID {
				t.Errorf("OID = %q, want %q", oc.OID, tt.wantOID)
			}
			if oc.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", oc.Name, tt.wantName)
			}
			if oc.Kind != tt.wantKind {
				t.Errorf("Kind = %v, want %v", oc.Kind, tt.wantKind)
			}
			if oc.Superior != tt.wantSup {
				t.Errorf("Superior = %q, want %q", oc.Superior, tt.wantSup)
			}
			if !stringSliceEqual(oc.Must, tt.wantMust) {
				t.Errorf("Must = %v, want %v", oc.Must, tt.wantMust)
			}
			if !stringSliceEqual(oc.May, tt.wantMay) {
				t.Errorf("May = %v, want %v", oc.May, tt.wantMay)
			}
		})
	}
}

func TestParseAttributeType(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		wantOID         string
		wantName        string
		wantSyntax      string
		wantSingleValue bool
		wantNoUserMod   bool
		wantUsage       AttributeUsage
		wantSuperior    string
		wantErr         bool
	}{
		{
			name:       "simple attribute",
			input:      "( 2.5.4.3 NAME 'cn' SYNTAX 1.3.6.1.4.1.1466.115.121.1.15 )",
			wantOID:    "2.5.4.3",
			wantName:   "cn",
			wantSyntax: SyntaxDirectoryString,
			wantUsage:  UserApplications,
		},
		{
			name:            "single value attribute",
			input:           "( 2.5.4.6 NAME 'c' SYNTAX 1.3.6.1.4.1.1466.115.121.1.15 SINGLE-VALUE )",
			wantOID:         "2.5.4.6",
			wantName:        "c",
			wantSyntax:      SyntaxDirectoryString,
			wantSingleValue: true,
			wantUsage:       UserApplications,
		},
		{
			name:            "operational attribute",
			input:           "( 2.5.18.1 NAME 'createTimestamp' SYNTAX 1.3.6.1.4.1.1466.115.121.1.24 SINGLE-VALUE NO-USER-MODIFICATION USAGE directoryOperation )",
			wantOID:         "2.5.18.1",
			wantName:        "createTimestamp",
			wantSyntax:      SyntaxGeneralizedTime,
			wantSingleValue: true,
			wantNoUserMod:   true,
			wantUsage:       DirectoryOperation,
		},
		{
			name:         "attribute with superior",
			input:        "( 2.5.4.3 NAME 'cn' SUP name )",
			wantOID:      "2.5.4.3",
			wantName:     "cn",
			wantSuperior: "name",
			wantUsage:    UserApplications,
		},
		{
			name:       "syntax with length constraint",
			input:      "( 2.5.4.3 NAME 'cn' SYNTAX 1.3.6.1.4.1.1466.115.121.1.15{256} )",
			wantOID:    "2.5.4.3",
			wantName:   "cn",
			wantSyntax: SyntaxDirectoryString,
			wantUsage:  UserApplications,
		},
		{
			name:    "invalid - no parentheses",
			input:   "2.5.4.3 NAME 'cn'",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			at, err := parseAttributeType(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if at.OID != tt.wantOID {
				t.Errorf("OID = %q, want %q", at.OID, tt.wantOID)
			}
			if at.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", at.Name, tt.wantName)
			}
			if at.Syntax != tt.wantSyntax {
				t.Errorf("Syntax = %q, want %q", at.Syntax, tt.wantSyntax)
			}
			if at.SingleValue != tt.wantSingleValue {
				t.Errorf("SingleValue = %v, want %v", at.SingleValue, tt.wantSingleValue)
			}
			if at.NoUserMod != tt.wantNoUserMod {
				t.Errorf("NoUserMod = %v, want %v", at.NoUserMod, tt.wantNoUserMod)
			}
			if at.Usage != tt.wantUsage {
				t.Errorf("Usage = %v, want %v", at.Usage, tt.wantUsage)
			}
			if at.Superior != tt.wantSuperior {
				t.Errorf("Superior = %q, want %q", at.Superior, tt.wantSuperior)
			}
		})
	}
}

func TestLoadSchemaFromLDIF(t *testing.T) {
	ldif := `dn: cn=schema
objectClass: top
objectClass: ldapSubentry
objectClass: subschema
attributeTypes: ( 2.5.4.0 NAME 'objectClass' EQUALITY objectIdentifierMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.38 )
attributeTypes: ( 2.5.4.3 NAME ( 'cn' 'commonName' ) SUP name )
attributeTypes: ( 2.5.4.41 NAME 'name' EQUALITY caseIgnoreMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.15 )
objectClasses: ( 2.5.6.0 NAME 'top' ABSTRACT MUST objectClass )
objectClasses: ( 2.5.6.6 NAME 'person' SUP top STRUCTURAL MUST ( sn $ cn ) MAY description )
`

	s, err := LoadSchemaFromLDIF(strings.NewReader(ldif))
	if err != nil {
		t.Fatalf("LoadSchemaFromLDIF failed: %v", err)
	}

	// Check attribute types
	at := s.GetAttributeType("objectClass")
	if at == nil {
		t.Error("objectClass attribute type not found")
	} else if at.OID != "2.5.4.0" {
		t.Errorf("objectClass OID = %q, want %q", at.OID, "2.5.4.0")
	}

	at = s.GetAttributeType("cn")
	if at == nil {
		t.Error("cn attribute type not found")
	} else {
		if at.Superior != "name" {
			t.Errorf("cn Superior = %q, want %q", at.Superior, "name")
		}
	}

	// Check object classes
	oc := s.GetObjectClass("top")
	if oc == nil {
		t.Error("top object class not found")
	} else {
		if oc.Kind != ObjectClassAbstract {
			t.Errorf("top Kind = %v, want %v", oc.Kind, ObjectClassAbstract)
		}
	}

	oc = s.GetObjectClass("person")
	if oc == nil {
		t.Error("person object class not found")
	} else {
		if oc.Superior != "top" {
			t.Errorf("person Superior = %q, want %q", oc.Superior, "top")
		}
		if !stringSliceContains(oc.Must, "sn") || !stringSliceContains(oc.Must, "cn") {
			t.Errorf("person Must = %v, want to contain sn and cn", oc.Must)
		}
	}
}

func TestLoadSchemaFromLDIFWithContinuation(t *testing.T) {
	ldif := `dn: cn=schema
attributeTypes: ( 2.5.4.3 NAME 'cn' DESC 'Common name'
 EQUALITY caseIgnoreMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.15 )
`

	s, err := LoadSchemaFromLDIF(strings.NewReader(ldif))
	if err != nil {
		t.Fatalf("LoadSchemaFromLDIF failed: %v", err)
	}

	at := s.GetAttributeType("cn")
	if at == nil {
		t.Fatal("cn attribute type not found")
	}
	if at.Desc != "Common name" {
		t.Errorf("cn Desc = %q, want %q", at.Desc, "Common name")
	}
}

func TestLoadDefaultSchema(t *testing.T) {
	s := LoadDefaultSchema()

	// Check that essential object classes exist
	essentialOCs := []string{"top", "person", "organizationalPerson", "organization", "organizationalUnit", "domain", "dcObject", "groupOfNames"}
	for _, name := range essentialOCs {
		if s.GetObjectClass(name) == nil {
			t.Errorf("essential object class %q not found", name)
		}
	}

	// Check that essential attribute types exist
	essentialATs := []string{"objectClass", "cn", "sn", "uid", "dc", "ou", "o", "description", "userPassword", "member"}
	for _, name := range essentialATs {
		if s.GetAttributeType(name) == nil {
			t.Errorf("essential attribute type %q not found", name)
		}
	}

	// Check operational attributes
	operationalATs := []string{"createTimestamp", "modifyTimestamp", "creatorsName", "modifiersName", "entryUUID"}
	for _, name := range operationalATs {
		at := s.GetAttributeType(name)
		if at == nil {
			t.Errorf("operational attribute %q not found", name)
		} else if !at.IsOperational() {
			t.Errorf("attribute %q should be operational", name)
		}
	}

	// Check matching rules
	essentialMRs := []string{"caseIgnoreMatch", "distinguishedNameMatch", "integerMatch", "booleanMatch"}
	for _, name := range essentialMRs {
		if s.GetMatchingRule(name) == nil {
			t.Errorf("essential matching rule %q not found", name)
		}
	}

	// Check syntaxes
	essentialSyntaxes := []string{SyntaxDirectoryString, SyntaxDN, SyntaxInteger, SyntaxBoolean}
	for _, oid := range essentialSyntaxes {
		if s.GetSyntax(oid) == nil {
			t.Errorf("essential syntax %q not found", oid)
		}
	}
}

func TestObjectClassInheritance(t *testing.T) {
	s := LoadDefaultSchema()

	// Test person inherits from top
	person := s.GetObjectClass("person")
	if person == nil {
		t.Fatal("person object class not found")
	}
	if person.Superior != "top" {
		t.Errorf("person Superior = %q, want %q", person.Superior, "top")
	}

	// Test organizationalPerson inherits from person
	orgPerson := s.GetObjectClass("organizationalPerson")
	if orgPerson == nil {
		t.Fatal("organizationalPerson object class not found")
	}
	if orgPerson.Superior != "person" {
		t.Errorf("organizationalPerson Superior = %q, want %q", orgPerson.Superior, "person")
	}

	// Test GetAllMustAttributes includes inherited attributes
	mustAttrs := s.GetAllMustAttributes("person")
	if !stringSliceContains(mustAttrs, "objectClass") {
		t.Error("person should inherit objectClass from top")
	}
	if !stringSliceContains(mustAttrs, "sn") || !stringSliceContains(mustAttrs, "cn") {
		t.Error("person should have sn and cn as MUST attributes")
	}
}

func TestAttributeTypeInheritance(t *testing.T) {
	s := LoadDefaultSchema()

	// cn inherits from name
	cn := s.GetAttributeType("cn")
	if cn == nil {
		t.Fatal("cn attribute type not found")
	}
	if cn.Superior != "name" {
		t.Errorf("cn Superior = %q, want %q", cn.Superior, "name")
	}

	// cn should inherit syntax from name
	effectiveSyntax := s.GetEffectiveSyntax("cn")
	if effectiveSyntax != SyntaxDirectoryString {
		t.Errorf("cn effective syntax = %q, want %q", effectiveSyntax, SyntaxDirectoryString)
	}

	// cn should inherit equality matching from name
	effectiveEquality := s.GetEffectiveEqualityMatch("cn")
	if effectiveEquality != "caseIgnoreMatch" {
		t.Errorf("cn effective equality = %q, want %q", effectiveEquality, "caseIgnoreMatch")
	}
}

func TestGetAllMustAttributes(t *testing.T) {
	s := LoadDefaultSchema()

	// organizationalPerson should have MUST from person and top
	mustAttrs := s.GetAllMustAttributes("organizationalPerson")

	// From top
	if !stringSliceContains(mustAttrs, "objectClass") {
		t.Error("should include objectClass from top")
	}

	// From person
	if !stringSliceContains(mustAttrs, "sn") {
		t.Error("should include sn from person")
	}
	if !stringSliceContains(mustAttrs, "cn") {
		t.Error("should include cn from person")
	}
}

func TestGetAllMayAttributes(t *testing.T) {
	s := LoadDefaultSchema()

	// person should have MAY attributes
	mayAttrs := s.GetAllMayAttributes("person")

	if !stringSliceContains(mayAttrs, "userPassword") {
		t.Error("person should have userPassword as MAY attribute")
	}
	if !stringSliceContains(mayAttrs, "telephoneNumber") {
		t.Error("person should have telephoneNumber as MAY attribute")
	}
}

func TestLoadSchemaFileNotFound(t *testing.T) {
	_, err := LoadSchema("/nonexistent/path/schema.ldif")
	if err != ErrSchemaFileNotFound {
		t.Errorf("expected ErrSchemaFileNotFound, got %v", err)
	}
}

func TestParseMatchingRule(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantOID    string
		wantName   string
		wantSyntax string
		wantErr    bool
	}{
		{
			name:       "simple matching rule",
			input:      "( 2.5.13.2 NAME 'caseIgnoreMatch' SYNTAX 1.3.6.1.4.1.1466.115.121.1.15 )",
			wantOID:    "2.5.13.2",
			wantName:   "caseIgnoreMatch",
			wantSyntax: SyntaxDirectoryString,
		},
		{
			name:    "invalid - no parentheses",
			input:   "2.5.13.2 NAME 'caseIgnoreMatch'",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mr, err := parseMatchingRule(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if mr.OID != tt.wantOID {
				t.Errorf("OID = %q, want %q", mr.OID, tt.wantOID)
			}
			if mr.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", mr.Name, tt.wantName)
			}
			if mr.Syntax != tt.wantSyntax {
				t.Errorf("Syntax = %q, want %q", mr.Syntax, tt.wantSyntax)
			}
		})
	}
}

func TestParseSyntaxDef(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantOID  string
		wantDesc string
		wantErr  bool
	}{
		{
			name:     "simple syntax",
			input:    "( 1.3.6.1.4.1.1466.115.121.1.15 DESC 'Directory String' )",
			wantOID:  SyntaxDirectoryString,
			wantDesc: "Directory String",
		},
		{
			name:    "invalid - no parentheses",
			input:   "1.3.6.1.4.1.1466.115.121.1.15 DESC 'Directory String'",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			syn, err := parseSyntaxDef(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if syn.OID != tt.wantOID {
				t.Errorf("OID = %q, want %q", syn.OID, tt.wantOID)
			}
			if syn.Description != tt.wantDesc {
				t.Errorf("Description = %q, want %q", syn.Description, tt.wantDesc)
			}
		})
	}
}

func TestTokenize(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr bool
	}{
		{
			name:  "simple tokens",
			input: "NAME 'test' SYNTAX 1.2.3",
			want:  []string{"NAME", "'test'", "SYNTAX", "1.2.3"},
		},
		{
			name:  "parenthesized list",
			input: "MUST ( attr1 $ attr2 )",
			want:  []string{"MUST", " attr1 $ attr2 "},
		},
		{
			name:    "unterminated quote",
			input:   "NAME 'test",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tokenize(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(got) != len(tt.want) {
				t.Errorf("got %d tokens, want %d: %v vs %v", len(got), len(tt.want), got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("token[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestParseNames(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"'cn'", []string{"cn"}},
		{"'cn' 'commonName'", []string{"cn", "commonName"}},
		{"cn", []string{"cn"}},
	}

	for _, tt := range tests {
		got := parseNames(tt.input)
		if !stringSliceEqual(got, tt.want) {
			t.Errorf("parseNames(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestParseAttributeList(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"attr1", []string{"attr1"}},
		{"attr1 $ attr2", []string{"attr1", "attr2"}},
		{" attr1 $ attr2 $ attr3 ", []string{"attr1", "attr2", "attr3"}},
	}

	for _, tt := range tests {
		got := parseAttributeList(tt.input)
		if !stringSliceEqual(got, tt.want) {
			t.Errorf("parseAttributeList(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestUnquote(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"'test'", "test"},
		{"test", "test"},
		{"''", ""},
		{" 'test' ", "test"},
	}

	for _, tt := range tests {
		got := unquote(tt.input)
		if got != tt.want {
			t.Errorf("unquote(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseSyntaxOID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"1.3.6.1.4.1.1466.115.121.1.15", "1.3.6.1.4.1.1466.115.121.1.15"},
		{"1.3.6.1.4.1.1466.115.121.1.15{256}", "1.3.6.1.4.1.1466.115.121.1.15"},
		{"'1.3.6.1.4.1.1466.115.121.1.15{256}'", "1.3.6.1.4.1.1466.115.121.1.15"},
	}

	for _, tt := range tests {
		got := parseSyntaxOID(tt.input)
		if got != tt.want {
			t.Errorf("parseSyntaxOID(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseUsage(t *testing.T) {
	tests := []struct {
		input string
		want  AttributeUsage
	}{
		{"userApplications", UserApplications},
		{"directoryOperation", DirectoryOperation},
		{"distributedOperation", DistributedOperation},
		{"dSAOperation", DSAOperation},
		{"unknown", UserApplications},
	}

	for _, tt := range tests {
		got := parseUsage(tt.input)
		if got != tt.want {
			t.Errorf("parseUsage(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestSchemaLookupByAlias(t *testing.T) {
	s := LoadDefaultSchema()

	// Test attribute type alias lookup
	at := s.GetAttributeType("commonName")
	if at == nil {
		t.Error("should find cn by alias commonName")
	} else if at.Name != "cn" {
		t.Errorf("expected primary name 'cn', got %q", at.Name)
	}

	// Test object class lookup by OID
	oc := s.GetObjectClass("2.5.6.6")
	if oc == nil {
		t.Error("should find person by OID")
	} else if oc.Name != "person" {
		t.Errorf("expected name 'person', got %q", oc.Name)
	}
}

func TestNonexistentSchemaElements(t *testing.T) {
	s := LoadDefaultSchema()

	if s.GetObjectClass("nonexistent") != nil {
		t.Error("should return nil for nonexistent object class")
	}
	if s.GetAttributeType("nonexistent") != nil {
		t.Error("should return nil for nonexistent attribute type")
	}
	if s.GetMatchingRule("nonexistent") != nil {
		t.Error("should return nil for nonexistent matching rule")
	}
	if s.GetSyntax("nonexistent") != nil {
		t.Error("should return nil for nonexistent syntax")
	}
	if s.GetAllMustAttributes("nonexistent") != nil {
		t.Error("should return nil for nonexistent object class MUST attrs")
	}
	if s.GetAllMayAttributes("nonexistent") != nil {
		t.Error("should return nil for nonexistent object class MAY attrs")
	}
	if s.GetEffectiveSyntax("nonexistent") != "" {
		t.Error("should return empty string for nonexistent attribute syntax")
	}
	if s.GetEffectiveEqualityMatch("nonexistent") != "" {
		t.Error("should return empty string for nonexistent attribute equality")
	}
}

// Helper functions

func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func stringSliceContains(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}
