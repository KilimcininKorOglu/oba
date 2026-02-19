package schema

import "testing"

func TestObjectClassKindString(t *testing.T) {
	tests := []struct {
		kind     ObjectClassKind
		expected string
	}{
		{ObjectClassAbstract, "ABSTRACT"},
		{ObjectClassStructural, "STRUCTURAL"},
		{ObjectClassAuxiliary, "AUXILIARY"},
		{ObjectClassKind(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		result := tt.kind.String()
		if result != tt.expected {
			t.Errorf("ObjectClassKind(%d).String() = %s, want %s", tt.kind, result, tt.expected)
		}
	}
}

func TestNewObjectClass(t *testing.T) {
	oc := NewObjectClass("2.5.6.6", "person")

	if oc.OID != "2.5.6.6" {
		t.Errorf("expected OID '2.5.6.6', got '%s'", oc.OID)
	}
	if oc.Name != "person" {
		t.Errorf("expected Name 'person', got '%s'", oc.Name)
	}
	if len(oc.Names) != 1 || oc.Names[0] != "person" {
		t.Error("Names should contain the primary name")
	}
	if oc.Kind != ObjectClassStructural {
		t.Error("default Kind should be ObjectClassStructural")
	}
	if oc.Must == nil {
		t.Error("Must should be initialized")
	}
	if oc.May == nil {
		t.Error("May should be initialized")
	}
}

func TestObjectClassKindChecks(t *testing.T) {
	oc := NewObjectClass("2.5.6.6", "person")

	// Default is structural
	if !oc.IsStructural() {
		t.Error("default should be structural")
	}
	if oc.IsAbstract() {
		t.Error("should not be abstract")
	}
	if oc.IsAuxiliary() {
		t.Error("should not be auxiliary")
	}

	// Test abstract
	oc.Kind = ObjectClassAbstract
	if !oc.IsAbstract() {
		t.Error("should be abstract")
	}
	if oc.IsStructural() {
		t.Error("should not be structural")
	}

	// Test auxiliary
	oc.Kind = ObjectClassAuxiliary
	if !oc.IsAuxiliary() {
		t.Error("should be auxiliary")
	}
	if oc.IsStructural() {
		t.Error("should not be structural")
	}
}

func TestObjectClassMustAttributes(t *testing.T) {
	oc := NewObjectClass("2.5.6.6", "person")
	oc.Must = []string{"cn", "sn"}

	if !oc.HasMustAttribute("cn") {
		t.Error("should have 'cn' as must attribute")
	}
	if !oc.HasMustAttribute("sn") {
		t.Error("should have 'sn' as must attribute")
	}
	if oc.HasMustAttribute("mail") {
		t.Error("should not have 'mail' as must attribute")
	}
}

func TestObjectClassMayAttributes(t *testing.T) {
	oc := NewObjectClass("2.5.6.6", "person")
	oc.May = []string{"mail", "telephoneNumber"}

	if !oc.HasMayAttribute("mail") {
		t.Error("should have 'mail' as may attribute")
	}
	if !oc.HasMayAttribute("telephoneNumber") {
		t.Error("should have 'telephoneNumber' as may attribute")
	}
	if oc.HasMayAttribute("cn") {
		t.Error("should not have 'cn' as may attribute")
	}
}

func TestObjectClassAllowsAttribute(t *testing.T) {
	oc := NewObjectClass("2.5.6.6", "person")
	oc.Must = []string{"cn", "sn"}
	oc.May = []string{"mail", "telephoneNumber"}

	// Must attributes are allowed
	if !oc.AllowsAttribute("cn") {
		t.Error("should allow 'cn'")
	}

	// May attributes are allowed
	if !oc.AllowsAttribute("mail") {
		t.Error("should allow 'mail'")
	}

	// Unknown attributes are not allowed
	if oc.AllowsAttribute("unknown") {
		t.Error("should not allow 'unknown'")
	}
}

func TestObjectClassAddMustAttribute(t *testing.T) {
	oc := NewObjectClass("2.5.6.6", "person")

	oc.AddMustAttribute("cn")
	if !oc.HasMustAttribute("cn") {
		t.Error("should have added 'cn'")
	}

	// Adding duplicate should not create duplicate
	oc.AddMustAttribute("cn")
	count := 0
	for _, attr := range oc.Must {
		if attr == "cn" {
			count++
		}
	}
	if count != 1 {
		t.Error("should not add duplicate must attribute")
	}
}

func TestObjectClassAddMayAttribute(t *testing.T) {
	oc := NewObjectClass("2.5.6.6", "person")

	oc.AddMayAttribute("mail")
	if !oc.HasMayAttribute("mail") {
		t.Error("should have added 'mail'")
	}

	// Adding duplicate should not create duplicate
	oc.AddMayAttribute("mail")
	count := 0
	for _, attr := range oc.May {
		if attr == "mail" {
			count++
		}
	}
	if count != 1 {
		t.Error("should not add duplicate may attribute")
	}
}

func TestObjectClassAddName(t *testing.T) {
	oc := NewObjectClass("2.5.6.6", "person")

	oc.AddName("Person")
	if len(oc.Names) != 2 {
		t.Error("should have 2 names")
	}

	// Adding duplicate should not create duplicate
	oc.AddName("person")
	if len(oc.Names) != 2 {
		t.Error("should not add duplicate name")
	}
}
