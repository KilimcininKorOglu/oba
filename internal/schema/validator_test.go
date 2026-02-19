package schema

import (
	"testing"
)

// setupTestSchema creates a test schema with common object classes and attributes.
func setupTestSchema() *Schema {
	s := NewSchema()

	// Add syntaxes with validators
	s.AddSyntax(NewSyntaxWithValidator(SyntaxDirectoryString, "Directory String", ValidateDirectoryString))
	s.AddSyntax(NewSyntaxWithValidator(SyntaxInteger, "INTEGER", ValidateInteger))
	s.AddSyntax(NewSyntaxWithValidator(SyntaxBoolean, "Boolean", ValidateBoolean))
	s.AddSyntax(NewSyntaxWithValidator(SyntaxOID, "OID", ValidateDirectoryString))
	s.AddSyntax(NewSyntaxWithValidator(SyntaxOctetString, "Octet String", ValidateOctetString))

	// Add attribute types
	objectClass := NewAttributeType("2.5.4.0", "objectClass")
	objectClass.Syntax = SyntaxOID
	s.AddAttributeType(objectClass)

	cn := NewAttributeType("2.5.4.3", "cn")
	cn.Syntax = SyntaxDirectoryString
	s.AddAttributeType(cn)

	sn := NewAttributeType("2.5.4.4", "sn")
	sn.Syntax = SyntaxDirectoryString
	s.AddAttributeType(sn)

	uid := NewAttributeType("0.9.2342.19200300.100.1.1", "uid")
	uid.Syntax = SyntaxDirectoryString
	uid.SingleValue = true
	s.AddAttributeType(uid)

	mail := NewAttributeType("0.9.2342.19200300.100.1.3", "mail")
	mail.Syntax = SyntaxDirectoryString
	s.AddAttributeType(mail)

	description := NewAttributeType("2.5.4.13", "description")
	description.Syntax = SyntaxDirectoryString
	s.AddAttributeType(description)

	userPassword := NewAttributeType("2.5.4.35", "userPassword")
	userPassword.Syntax = SyntaxOctetString
	s.AddAttributeType(userPassword)

	telephoneNumber := NewAttributeType("2.5.4.20", "telephoneNumber")
	telephoneNumber.Syntax = SyntaxDirectoryString
	s.AddAttributeType(telephoneNumber)

	// Operational attributes
	createTimestamp := NewAttributeType("2.5.18.1", "createTimestamp")
	createTimestamp.Syntax = SyntaxDirectoryString
	createTimestamp.SingleValue = true
	createTimestamp.NoUserMod = true
	createTimestamp.Usage = DirectoryOperation
	s.AddAttributeType(createTimestamp)

	modifyTimestamp := NewAttributeType("2.5.18.2", "modifyTimestamp")
	modifyTimestamp.Syntax = SyntaxDirectoryString
	modifyTimestamp.SingleValue = true
	modifyTimestamp.NoUserMod = true
	modifyTimestamp.Usage = DirectoryOperation
	s.AddAttributeType(modifyTimestamp)

	// Add object classes
	top := NewObjectClass("2.5.6.0", "top")
	top.Kind = ObjectClassAbstract
	top.Must = []string{"objectClass"}
	s.AddObjectClass(top)

	person := NewObjectClass("2.5.6.6", "person")
	person.Kind = ObjectClassStructural
	person.Superior = "top"
	person.Must = []string{"sn", "cn"}
	person.May = []string{"userPassword", "telephoneNumber", "description"}
	s.AddObjectClass(person)

	organizationalPerson := NewObjectClass("2.5.6.7", "organizationalPerson")
	organizationalPerson.Kind = ObjectClassStructural
	organizationalPerson.Superior = "person"
	organizationalPerson.May = []string{"uid", "mail"}
	s.AddObjectClass(organizationalPerson)

	inetOrgPerson := NewObjectClass("2.16.840.1.113730.3.2.2", "inetOrgPerson")
	inetOrgPerson.Kind = ObjectClassStructural
	inetOrgPerson.Superior = "organizationalPerson"
	inetOrgPerson.May = []string{"mail", "uid"}
	s.AddObjectClass(inetOrgPerson)

	// Auxiliary object class
	simpleSecurityObject := NewObjectClass("0.9.2342.19200300.100.4.19", "simpleSecurityObject")
	simpleSecurityObject.Kind = ObjectClassAuxiliary
	simpleSecurityObject.Superior = "top"
	simpleSecurityObject.Must = []string{"userPassword"}
	s.AddObjectClass(simpleSecurityObject)

	return s
}

func TestNewValidator(t *testing.T) {
	s := setupTestSchema()
	v := NewValidator(s)

	if v == nil {
		t.Fatal("NewValidator returned nil")
	}

	if v.GetSchema() != s {
		t.Error("GetSchema returned wrong schema")
	}
}

func TestValidateEntry_MissingObjectClass(t *testing.T) {
	s := setupTestSchema()
	v := NewValidator(s)

	entry := NewEntry("cn=test,dc=example,dc=com")
	entry.SetStringAttribute("cn", "test")

	err := v.ValidateEntry(entry)
	if err == nil {
		t.Fatal("expected error for missing objectClass")
	}

	ve, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	if ve.Code != ErrObjectClassViolation {
		t.Errorf("expected ErrObjectClassViolation, got %d", ve.Code)
	}
}

func TestValidateEntry_UnknownObjectClass(t *testing.T) {
	s := setupTestSchema()
	v := NewValidator(s)

	entry := NewEntry("cn=test,dc=example,dc=com")
	entry.SetStringAttribute("objectClass", "unknownClass")

	err := v.ValidateEntry(entry)
	if err == nil {
		t.Fatal("expected error for unknown objectClass")
	}

	ve, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	if ve.Code != ErrObjectClassViolation {
		t.Errorf("expected ErrObjectClassViolation, got %d", ve.Code)
	}

	if ve.Attr != "unknownClass" {
		t.Errorf("expected attr 'unknownClass', got '%s'", ve.Attr)
	}
}

func TestValidateEntry_NoStructuralObjectClass(t *testing.T) {
	s := setupTestSchema()
	v := NewValidator(s)

	entry := NewEntry("cn=test,dc=example,dc=com")
	entry.SetStringAttribute("objectClass", "top") // top is abstract

	err := v.ValidateEntry(entry)
	if err == nil {
		t.Fatal("expected error for no structural objectClass")
	}

	ve, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	if ve.Code != ErrObjectClassViolation {
		t.Errorf("expected ErrObjectClassViolation, got %d", ve.Code)
	}
}

func TestValidateEntry_MissingRequiredAttribute(t *testing.T) {
	s := setupTestSchema()
	v := NewValidator(s)

	entry := NewEntry("cn=test,dc=example,dc=com")
	entry.SetStringAttribute("objectClass", "person")
	entry.SetStringAttribute("cn", "test")
	// Missing 'sn' which is required by person

	err := v.ValidateEntry(entry)
	if err == nil {
		t.Fatal("expected error for missing required attribute")
	}

	ve, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	if ve.Code != ErrMissingRequiredAttribute {
		t.Errorf("expected ErrMissingRequiredAttribute, got %d", ve.Code)
	}

	if ve.Attr != "sn" {
		t.Errorf("expected attr 'sn', got '%s'", ve.Attr)
	}
}

func TestValidateEntry_UndefinedAttribute(t *testing.T) {
	s := setupTestSchema()
	v := NewValidator(s)

	entry := NewEntry("cn=test,dc=example,dc=com")
	entry.SetStringAttribute("objectClass", "person")
	entry.SetStringAttribute("cn", "test")
	entry.SetStringAttribute("sn", "Test")
	entry.SetStringAttribute("unknownAttr", "value") // Not allowed by person

	err := v.ValidateEntry(entry)
	if err == nil {
		t.Fatal("expected error for undefined attribute")
	}

	ve, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	if ve.Code != ErrUndefinedAttributeType {
		t.Errorf("expected ErrUndefinedAttributeType, got %d", ve.Code)
	}

	if ve.Attr != "unknownAttr" {
		t.Errorf("expected attr 'unknownAttr', got '%s'", ve.Attr)
	}
}

func TestValidateEntry_SingleValueViolation(t *testing.T) {
	s := setupTestSchema()
	v := NewValidator(s)

	entry := NewEntry("uid=test,dc=example,dc=com")
	entry.SetStringAttribute("objectClass", "inetOrgPerson")
	entry.SetStringAttribute("cn", "test")
	entry.SetStringAttribute("sn", "Test")
	entry.SetStringAttribute("uid", "test1", "test2") // uid is single-value

	err := v.ValidateEntry(entry)
	if err == nil {
		t.Fatal("expected error for single-value violation")
	}

	ve, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	if ve.Code != ErrSingleValueViolation {
		t.Errorf("expected ErrSingleValueViolation, got %d", ve.Code)
	}

	if ve.Attr != "uid" {
		t.Errorf("expected attr 'uid', got '%s'", ve.Attr)
	}
}

func TestValidateEntry_ValidPerson(t *testing.T) {
	s := setupTestSchema()
	v := NewValidator(s)

	entry := NewEntry("cn=John Doe,dc=example,dc=com")
	entry.SetStringAttribute("objectClass", "person")
	entry.SetStringAttribute("cn", "John Doe")
	entry.SetStringAttribute("sn", "Doe")
	entry.SetStringAttribute("description", "A test person")

	err := v.ValidateEntry(entry)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateEntry_ValidInetOrgPerson(t *testing.T) {
	s := setupTestSchema()
	v := NewValidator(s)

	entry := NewEntry("uid=jdoe,dc=example,dc=com")
	entry.SetStringAttribute("objectClass", "inetOrgPerson")
	entry.SetStringAttribute("cn", "John Doe")
	entry.SetStringAttribute("sn", "Doe")
	entry.SetStringAttribute("uid", "jdoe")
	entry.SetStringAttribute("mail", "jdoe@example.com")

	err := v.ValidateEntry(entry)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateEntry_MultipleObjectClasses(t *testing.T) {
	s := setupTestSchema()
	v := NewValidator(s)

	entry := NewEntry("cn=admin,dc=example,dc=com")
	entry.SetStringAttribute("objectClass", "person", "simpleSecurityObject")
	entry.SetStringAttribute("cn", "admin")
	entry.SetStringAttribute("sn", "Admin")
	entry.SetStringAttribute("userPassword", "secret")

	err := v.ValidateEntry(entry)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateEntry_AuxiliaryWithoutStructural(t *testing.T) {
	s := setupTestSchema()
	v := NewValidator(s)

	entry := NewEntry("cn=test,dc=example,dc=com")
	entry.SetStringAttribute("objectClass", "simpleSecurityObject") // auxiliary only
	entry.SetStringAttribute("userPassword", "secret")

	err := v.ValidateEntry(entry)
	if err == nil {
		t.Fatal("expected error for auxiliary without structural")
	}

	ve, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	if ve.Code != ErrObjectClassViolation {
		t.Errorf("expected ErrObjectClassViolation, got %d", ve.Code)
	}
}

func TestValidateEntry_OperationalAttributeAllowed(t *testing.T) {
	s := setupTestSchema()
	v := NewValidator(s)

	entry := NewEntry("cn=test,dc=example,dc=com")
	entry.SetStringAttribute("objectClass", "person")
	entry.SetStringAttribute("cn", "test")
	entry.SetStringAttribute("sn", "Test")
	entry.SetStringAttribute("createTimestamp", "20260219120000Z") // operational

	err := v.ValidateEntry(entry)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateEntry_NilEntry(t *testing.T) {
	s := setupTestSchema()
	v := NewValidator(s)

	err := v.ValidateEntry(nil)
	if err == nil {
		t.Fatal("expected error for nil entry")
	}

	ve, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	if ve.Code != ErrObjectClassViolation {
		t.Errorf("expected ErrObjectClassViolation, got %d", ve.Code)
	}
}

func TestValidateModification_AddAttribute(t *testing.T) {
	s := setupTestSchema()
	v := NewValidator(s)

	entry := NewEntry("cn=test,dc=example,dc=com")
	entry.SetStringAttribute("objectClass", "person")
	entry.SetStringAttribute("cn", "test")
	entry.SetStringAttribute("sn", "Test")

	mods := []Modification{
		*NewStringModification(ModAdd, "description", "A description"),
	}

	err := v.ValidateModification(entry, mods)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateModification_AddDisallowedAttribute(t *testing.T) {
	s := setupTestSchema()
	v := NewValidator(s)

	entry := NewEntry("cn=test,dc=example,dc=com")
	entry.SetStringAttribute("objectClass", "person")
	entry.SetStringAttribute("cn", "test")
	entry.SetStringAttribute("sn", "Test")

	mods := []Modification{
		*NewStringModification(ModAdd, "unknownAttr", "value"),
	}

	err := v.ValidateModification(entry, mods)
	if err == nil {
		t.Fatal("expected error for adding disallowed attribute")
	}

	ve, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	if ve.Code != ErrUndefinedAttributeType {
		t.Errorf("expected ErrUndefinedAttributeType, got %d", ve.Code)
	}
}

func TestValidateModification_DeleteRequiredAttribute(t *testing.T) {
	s := setupTestSchema()
	v := NewValidator(s)

	entry := NewEntry("cn=test,dc=example,dc=com")
	entry.SetStringAttribute("objectClass", "person")
	entry.SetStringAttribute("cn", "test")
	entry.SetStringAttribute("sn", "Test")

	mods := []Modification{
		*NewStringModification(ModDelete, "sn"), // sn is required
	}

	err := v.ValidateModification(entry, mods)
	if err == nil {
		t.Fatal("expected error for deleting required attribute")
	}

	ve, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	if ve.Code != ErrMissingRequiredAttribute {
		t.Errorf("expected ErrMissingRequiredAttribute, got %d", ve.Code)
	}
}

func TestValidateModification_ReplaceAttribute(t *testing.T) {
	s := setupTestSchema()
	v := NewValidator(s)

	entry := NewEntry("cn=test,dc=example,dc=com")
	entry.SetStringAttribute("objectClass", "person")
	entry.SetStringAttribute("cn", "test")
	entry.SetStringAttribute("sn", "Test")
	entry.SetStringAttribute("description", "Old description")

	mods := []Modification{
		*NewStringModification(ModReplace, "description", "New description"),
	}

	err := v.ValidateModification(entry, mods)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateModification_SingleValueViolation(t *testing.T) {
	s := setupTestSchema()
	v := NewValidator(s)

	entry := NewEntry("uid=test,dc=example,dc=com")
	entry.SetStringAttribute("objectClass", "inetOrgPerson")
	entry.SetStringAttribute("cn", "test")
	entry.SetStringAttribute("sn", "Test")
	entry.SetStringAttribute("uid", "test")

	mods := []Modification{
		*NewStringModification(ModAdd, "uid", "test2"), // uid is single-value
	}

	err := v.ValidateModification(entry, mods)
	if err == nil {
		t.Fatal("expected error for single-value violation")
	}

	ve, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	if ve.Code != ErrSingleValueViolation {
		t.Errorf("expected ErrSingleValueViolation, got %d", ve.Code)
	}
}

func TestValidateModification_NoUserModification(t *testing.T) {
	s := setupTestSchema()
	v := NewValidator(s)

	entry := NewEntry("cn=test,dc=example,dc=com")
	entry.SetStringAttribute("objectClass", "person")
	entry.SetStringAttribute("cn", "test")
	entry.SetStringAttribute("sn", "Test")

	mods := []Modification{
		*NewStringModification(ModReplace, "createTimestamp", "20260219120000Z"),
	}

	err := v.ValidateModification(entry, mods)
	if err == nil {
		t.Fatal("expected error for modifying read-only attribute")
	}

	ve, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	if ve.Code != ErrNoUserModification {
		t.Errorf("expected ErrNoUserModification, got %d", ve.Code)
	}
}

func TestValidateModification_DeleteSpecificValue(t *testing.T) {
	s := setupTestSchema()
	v := NewValidator(s)

	entry := NewEntry("cn=test,dc=example,dc=com")
	entry.SetStringAttribute("objectClass", "person")
	entry.SetStringAttribute("cn", "test")
	entry.SetStringAttribute("sn", "Test")
	entry.SetStringAttribute("telephoneNumber", "111-1111", "222-2222")

	mods := []Modification{
		*NewStringModification(ModDelete, "telephoneNumber", "111-1111"),
	}

	err := v.ValidateModification(entry, mods)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateModification_NilEntry(t *testing.T) {
	s := setupTestSchema()
	v := NewValidator(s)

	mods := []Modification{
		*NewStringModification(ModAdd, "cn", "test"),
	}

	err := v.ValidateModification(nil, mods)
	if err == nil {
		t.Fatal("expected error for nil entry")
	}
}

func TestValidateModification_EmptyMods(t *testing.T) {
	s := setupTestSchema()
	v := NewValidator(s)

	entry := NewEntry("cn=test,dc=example,dc=com")
	entry.SetStringAttribute("objectClass", "person")
	entry.SetStringAttribute("cn", "test")
	entry.SetStringAttribute("sn", "Test")

	err := v.ValidateModification(entry, []Modification{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidationError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *ValidationError
		expected string
	}{
		{
			name:     "with attribute",
			err:      NewValidationErrorWithAttr(ErrMissingRequiredAttribute, "missing required attribute", "cn"),
			expected: "missing required attribute: cn",
		},
		{
			name:     "without attribute",
			err:      NewValidationError(ErrObjectClassViolation, "objectClass required"),
			expected: "objectClass required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.expected {
				t.Errorf("Error() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestEntry_Methods(t *testing.T) {
	entry := NewEntry("cn=test,dc=example,dc=com")

	// Test SetStringAttribute and GetAll
	entry.SetStringAttribute("cn", "test1", "test2")
	values := entry.GetAll("cn")
	if len(values) != 2 {
		t.Errorf("expected 2 values, got %d", len(values))
	}
	if values[0] != "test1" || values[1] != "test2" {
		t.Errorf("unexpected values: %v", values)
	}

	// Test Has
	if !entry.Has("cn") {
		t.Error("expected Has('cn') to return true")
	}
	if entry.Has("nonexistent") {
		t.Error("expected Has('nonexistent') to return false")
	}

	// Test GetAttribute
	byteValues := entry.GetAttribute("cn")
	if len(byteValues) != 2 {
		t.Errorf("expected 2 byte values, got %d", len(byteValues))
	}

	// Test Clone
	clone := entry.Clone()
	if clone.DN != entry.DN {
		t.Error("clone DN mismatch")
	}
	if len(clone.Attributes) != len(entry.Attributes) {
		t.Error("clone attributes count mismatch")
	}

	// Modify clone and verify original is unchanged
	clone.SetStringAttribute("cn", "modified")
	if entry.GetAll("cn")[0] == "modified" {
		t.Error("modifying clone affected original")
	}

	// Test Clone nil
	var nilEntry *Entry
	if nilEntry.Clone() != nil {
		t.Error("Clone of nil should return nil")
	}
}

func TestModification_Constructors(t *testing.T) {
	// Test NewModification
	mod := NewModification(ModAdd, "cn", []byte("test"))
	if mod.Type != ModAdd {
		t.Errorf("expected ModAdd, got %d", mod.Type)
	}
	if mod.Attr != "cn" {
		t.Errorf("expected 'cn', got '%s'", mod.Attr)
	}
	if len(mod.Values) != 1 || string(mod.Values[0]) != "test" {
		t.Errorf("unexpected values: %v", mod.Values)
	}

	// Test NewStringModification
	strMod := NewStringModification(ModReplace, "sn", "Test", "Test2")
	if strMod.Type != ModReplace {
		t.Errorf("expected ModReplace, got %d", strMod.Type)
	}
	if len(strMod.Values) != 2 {
		t.Errorf("expected 2 values, got %d", len(strMod.Values))
	}
}

func TestBytesEqual(t *testing.T) {
	tests := []struct {
		name     string
		a        []byte
		b        []byte
		expected bool
	}{
		{"equal", []byte("test"), []byte("test"), true},
		{"different length", []byte("test"), []byte("tests"), false},
		{"different content", []byte("test"), []byte("tess"), false},
		{"empty", []byte{}, []byte{}, true},
		{"nil and empty", nil, []byte{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := bytesEqual(tt.a, tt.b); got != tt.expected {
				t.Errorf("bytesEqual(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.expected)
			}
		})
	}
}

func TestValidateEntry_CaseInsensitiveAttributeCheck(t *testing.T) {
	s := setupTestSchema()
	v := NewValidator(s)

	// Test with different case for required attribute
	entry := NewEntry("cn=test,dc=example,dc=com")
	entry.SetStringAttribute("objectClass", "person")
	entry.SetStringAttribute("CN", "test") // uppercase
	entry.SetStringAttribute("SN", "Test") // uppercase

	err := v.ValidateEntry(entry)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateModification_ReplaceWithEmpty(t *testing.T) {
	s := setupTestSchema()
	v := NewValidator(s)

	entry := NewEntry("cn=test,dc=example,dc=com")
	entry.SetStringAttribute("objectClass", "person")
	entry.SetStringAttribute("cn", "test")
	entry.SetStringAttribute("sn", "Test")
	entry.SetStringAttribute("description", "A description")

	// Replace with empty values = delete
	mods := []Modification{
		{Type: ModReplace, Attr: "description", Values: nil},
	}

	err := v.ValidateModification(entry, mods)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateModification_MultipleModifications(t *testing.T) {
	s := setupTestSchema()
	v := NewValidator(s)

	entry := NewEntry("cn=test,dc=example,dc=com")
	entry.SetStringAttribute("objectClass", "person")
	entry.SetStringAttribute("cn", "test")
	entry.SetStringAttribute("sn", "Test")

	mods := []Modification{
		*NewStringModification(ModAdd, "description", "A description"),
		*NewStringModification(ModAdd, "telephoneNumber", "111-1111"),
		*NewStringModification(ModReplace, "sn", "NewTest"),
	}

	err := v.ValidateModification(entry, mods)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateEntry_InheritedAttributes(t *testing.T) {
	s := setupTestSchema()
	v := NewValidator(s)

	// inetOrgPerson inherits from organizationalPerson -> person -> top
	// So it should require cn and sn from person
	entry := NewEntry("uid=jdoe,dc=example,dc=com")
	entry.SetStringAttribute("objectClass", "inetOrgPerson")
	entry.SetStringAttribute("cn", "John Doe")
	// Missing sn - should fail

	err := v.ValidateEntry(entry)
	if err == nil {
		t.Fatal("expected error for missing inherited required attribute")
	}

	ve, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	if ve.Code != ErrMissingRequiredAttribute {
		t.Errorf("expected ErrMissingRequiredAttribute, got %d", ve.Code)
	}
}

func TestValidateEntry_InheritedMayAttributes(t *testing.T) {
	s := setupTestSchema()
	v := NewValidator(s)

	// inetOrgPerson should allow description from person (inherited)
	entry := NewEntry("uid=jdoe,dc=example,dc=com")
	entry.SetStringAttribute("objectClass", "inetOrgPerson")
	entry.SetStringAttribute("cn", "John Doe")
	entry.SetStringAttribute("sn", "Doe")
	entry.SetStringAttribute("description", "A test user") // from person

	err := v.ValidateEntry(entry)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
