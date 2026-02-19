package schema

import "testing"

func TestAttributeUsageString(t *testing.T) {
	tests := []struct {
		usage    AttributeUsage
		expected string
	}{
		{UserApplications, "userApplications"},
		{DirectoryOperation, "directoryOperation"},
		{DistributedOperation, "distributedOperation"},
		{DSAOperation, "dSAOperation"},
		{AttributeUsage(99), "unknown"},
	}

	for _, tt := range tests {
		result := tt.usage.String()
		if result != tt.expected {
			t.Errorf("AttributeUsage(%d).String() = %s, want %s", tt.usage, result, tt.expected)
		}
	}
}

func TestAttributeUsageIsOperational(t *testing.T) {
	tests := []struct {
		usage      AttributeUsage
		operational bool
	}{
		{UserApplications, false},
		{DirectoryOperation, true},
		{DistributedOperation, true},
		{DSAOperation, true},
	}

	for _, tt := range tests {
		result := tt.usage.IsOperational()
		if result != tt.operational {
			t.Errorf("AttributeUsage(%d).IsOperational() = %v, want %v", tt.usage, result, tt.operational)
		}
	}
}

func TestNewAttributeType(t *testing.T) {
	at := NewAttributeType("2.5.4.3", "cn")

	if at.OID != "2.5.4.3" {
		t.Errorf("expected OID '2.5.4.3', got '%s'", at.OID)
	}
	if at.Name != "cn" {
		t.Errorf("expected Name 'cn', got '%s'", at.Name)
	}
	if len(at.Names) != 1 || at.Names[0] != "cn" {
		t.Error("Names should contain the primary name")
	}
	if at.Usage != UserApplications {
		t.Error("default Usage should be UserApplications")
	}
}

func TestAttributeTypeIsUserAttribute(t *testing.T) {
	at := NewAttributeType("2.5.4.3", "cn")

	// Default is user attribute
	if !at.IsUserAttribute() {
		t.Error("default should be user attribute")
	}

	// NoUserMod makes it not a user attribute
	at.NoUserMod = true
	if at.IsUserAttribute() {
		t.Error("NoUserMod should make it not a user attribute")
	}

	// Operational usage makes it not a user attribute
	at.NoUserMod = false
	at.Usage = DirectoryOperation
	if at.IsUserAttribute() {
		t.Error("operational usage should make it not a user attribute")
	}
}

func TestAttributeTypeIsOperational(t *testing.T) {
	at := NewAttributeType("2.5.4.3", "cn")

	if at.IsOperational() {
		t.Error("default should not be operational")
	}

	at.Usage = DirectoryOperation
	if !at.IsOperational() {
		t.Error("DirectoryOperation should be operational")
	}
}

func TestAttributeTypeIsReadOnly(t *testing.T) {
	at := NewAttributeType("2.5.4.3", "cn")

	if at.IsReadOnly() {
		t.Error("default should not be read-only")
	}

	at.NoUserMod = true
	if !at.IsReadOnly() {
		t.Error("NoUserMod should make it read-only")
	}
}

func TestAttributeTypeSingleMultiValued(t *testing.T) {
	at := NewAttributeType("2.5.4.3", "cn")

	// Default is multi-valued
	if at.IsSingleValued() {
		t.Error("default should not be single-valued")
	}
	if !at.IsMultiValued() {
		t.Error("default should be multi-valued")
	}

	at.SingleValue = true
	if !at.IsSingleValued() {
		t.Error("should be single-valued")
	}
	if at.IsMultiValued() {
		t.Error("should not be multi-valued")
	}
}

func TestAttributeTypeMatchingRules(t *testing.T) {
	at := NewAttributeType("2.5.4.3", "cn")

	// No matching rules by default
	if at.HasEqualityMatching() {
		t.Error("should not have equality matching by default")
	}
	if at.HasOrderingMatching() {
		t.Error("should not have ordering matching by default")
	}
	if at.HasSubstringMatching() {
		t.Error("should not have substring matching by default")
	}

	// Set matching rules
	at.SetMatchingRules("caseIgnoreMatch", "caseIgnoreOrderingMatch", "caseIgnoreSubstringsMatch")

	if !at.HasEqualityMatching() {
		t.Error("should have equality matching")
	}
	if !at.HasOrderingMatching() {
		t.Error("should have ordering matching")
	}
	if !at.HasSubstringMatching() {
		t.Error("should have substring matching")
	}
}

func TestAttributeTypeAddName(t *testing.T) {
	at := NewAttributeType("2.5.4.3", "cn")

	at.AddName("commonName")
	if len(at.Names) != 2 {
		t.Error("should have 2 names")
	}

	// Adding duplicate should not create duplicate
	at.AddName("cn")
	if len(at.Names) != 2 {
		t.Error("should not add duplicate name")
	}
}

func TestAttributeTypeSetSyntax(t *testing.T) {
	at := NewAttributeType("2.5.4.3", "cn")

	at.SetSyntax(SyntaxDirectoryString)
	if at.Syntax != SyntaxDirectoryString {
		t.Errorf("expected syntax '%s', got '%s'", SyntaxDirectoryString, at.Syntax)
	}
}
