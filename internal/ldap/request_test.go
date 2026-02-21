package ldap

import (
	"bytes"
	"testing"

	"github.com/KilimcininKorOglu/oba/internal/ber"
)

// ============================================================================
// BindRequest Tests
// ============================================================================

func TestParseBindRequest_SimpleAuth(t *testing.T) {
	// Create a simple bind request
	encoder := ber.NewBEREncoder(128)

	// version = 3
	encoder.WriteInteger(3)
	// name = "cn=admin,dc=example,dc=com"
	encoder.WriteOctetString([]byte("cn=admin,dc=example,dc=com"))
	// authentication = simple [0] "secret"
	encoder.WriteTaggedValue(AuthSimple, false, []byte("secret"))

	data := encoder.Bytes()

	req, err := ParseBindRequest(data)
	if err != nil {
		t.Fatalf("ParseBindRequest failed: %v", err)
	}

	if req.Version != 3 {
		t.Errorf("Version = %d, want 3", req.Version)
	}

	if req.Name != "cn=admin,dc=example,dc=com" {
		t.Errorf("Name = %q, want %q", req.Name, "cn=admin,dc=example,dc=com")
	}

	if req.AuthMethod != AuthMethodSimple {
		t.Errorf("AuthMethod = %v, want AuthMethodSimple", req.AuthMethod)
	}

	if !bytes.Equal(req.SimplePassword, []byte("secret")) {
		t.Errorf("SimplePassword = %q, want %q", req.SimplePassword, "secret")
	}
}

func TestParseBindRequest_AnonymousBind(t *testing.T) {
	encoder := ber.NewBEREncoder(64)

	// version = 3
	encoder.WriteInteger(3)
	// name = "" (empty)
	encoder.WriteOctetString([]byte(""))
	// authentication = simple [0] "" (empty)
	encoder.WriteTaggedValue(AuthSimple, false, []byte(""))

	data := encoder.Bytes()

	req, err := ParseBindRequest(data)
	if err != nil {
		t.Fatalf("ParseBindRequest failed: %v", err)
	}

	if !req.IsAnonymous() {
		t.Error("Expected anonymous bind")
	}
}

func TestParseBindRequest_SASLAuth(t *testing.T) {
	// Create SASL credentials
	saslEncoder := ber.NewBEREncoder(64)
	saslEncoder.WriteOctetString([]byte("PLAIN"))
	saslEncoder.WriteOctetString([]byte("\x00user\x00password"))
	saslData := saslEncoder.Bytes()

	encoder := ber.NewBEREncoder(128)

	// version = 3
	encoder.WriteInteger(3)
	// name = ""
	encoder.WriteOctetString([]byte(""))
	// authentication = sasl [3] SEQUENCE { mechanism, credentials }
	encoder.WriteTaggedValue(AuthSASL, true, saslData)

	data := encoder.Bytes()

	req, err := ParseBindRequest(data)
	if err != nil {
		t.Fatalf("ParseBindRequest failed: %v", err)
	}

	if req.AuthMethod != AuthMethodSASL {
		t.Errorf("AuthMethod = %v, want AuthMethodSASL", req.AuthMethod)
	}

	if req.SASLCredentials == nil {
		t.Fatal("SASLCredentials is nil")
	}

	if req.SASLCredentials.Mechanism != "PLAIN" {
		t.Errorf("Mechanism = %q, want %q", req.SASLCredentials.Mechanism, "PLAIN")
	}

	if !bytes.Equal(req.SASLCredentials.Credentials, []byte("\x00user\x00password")) {
		t.Errorf("Credentials mismatch")
	}
}

func TestParseBindRequest_InvalidVersion(t *testing.T) {
	encoder := ber.NewBEREncoder(64)

	// version = 128 (out of range)
	encoder.WriteInteger(128)
	encoder.WriteOctetString([]byte(""))
	encoder.WriteTaggedValue(AuthSimple, false, []byte(""))

	data := encoder.Bytes()

	_, err := ParseBindRequest(data)
	if err != ErrInvalidBindVersion {
		t.Errorf("Expected ErrInvalidBindVersion, got %v", err)
	}
}

func TestBindRequest_Encode(t *testing.T) {
	req := &BindRequest{
		Version:        3,
		Name:           "cn=admin,dc=example,dc=com",
		AuthMethod:     AuthMethodSimple,
		SimplePassword: []byte("secret"),
	}

	encoded, err := req.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// Parse it back
	parsed, err := ParseBindRequest(encoded)
	if err != nil {
		t.Fatalf("ParseBindRequest failed: %v", err)
	}

	if parsed.Version != req.Version {
		t.Errorf("Version = %d, want %d", parsed.Version, req.Version)
	}

	if parsed.Name != req.Name {
		t.Errorf("Name = %q, want %q", parsed.Name, req.Name)
	}

	if !bytes.Equal(parsed.SimplePassword, req.SimplePassword) {
		t.Errorf("SimplePassword mismatch")
	}
}

// ============================================================================
// SearchRequest Tests
// ============================================================================

func TestParseSearchRequest_Basic(t *testing.T) {
	encoder := ber.NewBEREncoder(256)

	// baseObject = "dc=example,dc=com"
	encoder.WriteOctetString([]byte("dc=example,dc=com"))
	// scope = wholeSubtree (2)
	encoder.WriteEnumerated(2)
	// derefAliases = neverDerefAliases (0)
	encoder.WriteEnumerated(0)
	// sizeLimit = 100
	encoder.WriteInteger(100)
	// timeLimit = 30
	encoder.WriteInteger(30)
	// typesOnly = FALSE
	encoder.WriteBoolean(false)
	// filter = present "objectClass" [7]
	encoder.WriteTaggedValue(FilterTagPresent, false, []byte("objectClass"))
	// attributes = SEQUENCE OF { "cn", "mail" }
	attrSeqPos := encoder.BeginSequence()
	encoder.WriteOctetString([]byte("cn"))
	encoder.WriteOctetString([]byte("mail"))
	encoder.EndSequence(attrSeqPos)

	data := encoder.Bytes()

	req, err := ParseSearchRequest(data)
	if err != nil {
		t.Fatalf("ParseSearchRequest failed: %v", err)
	}

	if req.BaseObject != "dc=example,dc=com" {
		t.Errorf("BaseObject = %q, want %q", req.BaseObject, "dc=example,dc=com")
	}

	if req.Scope != ScopeWholeSubtree {
		t.Errorf("Scope = %v, want ScopeWholeSubtree", req.Scope)
	}

	if req.DerefAliases != DerefNever {
		t.Errorf("DerefAliases = %v, want DerefNever", req.DerefAliases)
	}

	if req.SizeLimit != 100 {
		t.Errorf("SizeLimit = %d, want 100", req.SizeLimit)
	}

	if req.TimeLimit != 30 {
		t.Errorf("TimeLimit = %d, want 30", req.TimeLimit)
	}

	if req.TypesOnly {
		t.Error("TypesOnly should be false")
	}

	if req.Filter == nil {
		t.Fatal("Filter is nil")
	}

	if req.Filter.Type != FilterTagPresent {
		t.Errorf("Filter.Type = %d, want %d", req.Filter.Type, FilterTagPresent)
	}

	if req.Filter.Attribute != "objectClass" {
		t.Errorf("Filter.Attribute = %q, want %q", req.Filter.Attribute, "objectClass")
	}

	if len(req.Attributes) != 2 {
		t.Errorf("len(Attributes) = %d, want 2", len(req.Attributes))
	}

	if req.Attributes[0] != "cn" || req.Attributes[1] != "mail" {
		t.Errorf("Attributes = %v, want [cn, mail]", req.Attributes)
	}
}

func TestParseSearchRequest_EqualityFilter(t *testing.T) {
	encoder := ber.NewBEREncoder(256)

	encoder.WriteOctetString([]byte("dc=example,dc=com"))
	encoder.WriteEnumerated(2)
	encoder.WriteEnumerated(0)
	encoder.WriteInteger(0)
	encoder.WriteInteger(0)
	encoder.WriteBoolean(false)

	// filter = equality [3] { "uid", "alice" }
	filterEncoder := ber.NewBEREncoder(64)
	filterEncoder.WriteOctetString([]byte("uid"))
	filterEncoder.WriteOctetString([]byte("alice"))
	encoder.WriteTaggedValue(FilterTagEquality, true, filterEncoder.Bytes())

	// empty attributes
	attrSeqPos := encoder.BeginSequence()
	encoder.EndSequence(attrSeqPos)

	data := encoder.Bytes()

	req, err := ParseSearchRequest(data)
	if err != nil {
		t.Fatalf("ParseSearchRequest failed: %v", err)
	}

	if req.Filter.Type != FilterTagEquality {
		t.Errorf("Filter.Type = %d, want %d", req.Filter.Type, FilterTagEquality)
	}

	if req.Filter.Attribute != "uid" {
		t.Errorf("Filter.Attribute = %q, want %q", req.Filter.Attribute, "uid")
	}

	if !bytes.Equal(req.Filter.Value, []byte("alice")) {
		t.Errorf("Filter.Value = %q, want %q", req.Filter.Value, "alice")
	}
}

func TestParseSearchRequest_AndFilter(t *testing.T) {
	encoder := ber.NewBEREncoder(256)

	encoder.WriteOctetString([]byte("dc=example,dc=com"))
	encoder.WriteEnumerated(2)
	encoder.WriteEnumerated(0)
	encoder.WriteInteger(0)
	encoder.WriteInteger(0)
	encoder.WriteBoolean(false)

	// filter = AND [0] { equality(uid=alice), present(mail) }
	andEncoder := ber.NewBEREncoder(128)

	// equality filter
	eqEncoder := ber.NewBEREncoder(64)
	eqEncoder.WriteOctetString([]byte("uid"))
	eqEncoder.WriteOctetString([]byte("alice"))
	andEncoder.WriteTaggedValue(FilterTagEquality, true, eqEncoder.Bytes())

	// present filter
	andEncoder.WriteTaggedValue(FilterTagPresent, false, []byte("mail"))

	encoder.WriteTaggedValue(FilterTagAnd, true, andEncoder.Bytes())

	// empty attributes
	attrSeqPos := encoder.BeginSequence()
	encoder.EndSequence(attrSeqPos)

	data := encoder.Bytes()

	req, err := ParseSearchRequest(data)
	if err != nil {
		t.Fatalf("ParseSearchRequest failed: %v", err)
	}

	if req.Filter.Type != FilterTagAnd {
		t.Errorf("Filter.Type = %d, want %d", req.Filter.Type, FilterTagAnd)
	}

	if len(req.Filter.Children) != 2 {
		t.Fatalf("len(Filter.Children) = %d, want 2", len(req.Filter.Children))
	}

	if req.Filter.Children[0].Type != FilterTagEquality {
		t.Errorf("Children[0].Type = %d, want %d", req.Filter.Children[0].Type, FilterTagEquality)
	}

	if req.Filter.Children[1].Type != FilterTagPresent {
		t.Errorf("Children[1].Type = %d, want %d", req.Filter.Children[1].Type, FilterTagPresent)
	}
}

func TestParseSearchRequest_SubstringFilter(t *testing.T) {
	encoder := ber.NewBEREncoder(256)

	encoder.WriteOctetString([]byte("dc=example,dc=com"))
	encoder.WriteEnumerated(2)
	encoder.WriteEnumerated(0)
	encoder.WriteInteger(0)
	encoder.WriteInteger(0)
	encoder.WriteBoolean(false)

	// filter = substring [4] { type="cn", substrings={ initial="Jo", any="hn", final="Doe" } }
	subEncoder := ber.NewBEREncoder(128)
	subEncoder.WriteOctetString([]byte("cn"))

	// substrings SEQUENCE
	subSeqPos := subEncoder.BeginSequence()
	subEncoder.WriteTaggedValue(SubstringInitial, false, []byte("Jo"))
	subEncoder.WriteTaggedValue(SubstringAny, false, []byte("hn"))
	subEncoder.WriteTaggedValue(SubstringFinal, false, []byte("Doe"))
	subEncoder.EndSequence(subSeqPos)

	encoder.WriteTaggedValue(FilterTagSubstrings, true, subEncoder.Bytes())

	// empty attributes
	attrSeqPos := encoder.BeginSequence()
	encoder.EndSequence(attrSeqPos)

	data := encoder.Bytes()

	req, err := ParseSearchRequest(data)
	if err != nil {
		t.Fatalf("ParseSearchRequest failed: %v", err)
	}

	if req.Filter.Type != FilterTagSubstrings {
		t.Errorf("Filter.Type = %d, want %d", req.Filter.Type, FilterTagSubstrings)
	}

	if req.Filter.Attribute != "cn" {
		t.Errorf("Filter.Attribute = %q, want %q", req.Filter.Attribute, "cn")
	}

	if req.Filter.Substrings == nil {
		t.Fatal("Filter.Substrings is nil")
	}

	if !bytes.Equal(req.Filter.Substrings.Initial, []byte("Jo")) {
		t.Errorf("Substrings.Initial = %q, want %q", req.Filter.Substrings.Initial, "Jo")
	}

	if len(req.Filter.Substrings.Any) != 1 || !bytes.Equal(req.Filter.Substrings.Any[0], []byte("hn")) {
		t.Errorf("Substrings.Any = %v, want [[hn]]", req.Filter.Substrings.Any)
	}

	if !bytes.Equal(req.Filter.Substrings.Final, []byte("Doe")) {
		t.Errorf("Substrings.Final = %q, want %q", req.Filter.Substrings.Final, "Doe")
	}
}

func TestParseSearchRequest_InvalidScope(t *testing.T) {
	encoder := ber.NewBEREncoder(256)

	encoder.WriteOctetString([]byte("dc=example,dc=com"))
	encoder.WriteEnumerated(5) // Invalid scope
	encoder.WriteEnumerated(0)
	encoder.WriteInteger(0)
	encoder.WriteInteger(0)
	encoder.WriteBoolean(false)
	encoder.WriteTaggedValue(FilterTagPresent, false, []byte("objectClass"))
	attrSeqPos := encoder.BeginSequence()
	encoder.EndSequence(attrSeqPos)

	data := encoder.Bytes()

	_, err := ParseSearchRequest(data)
	if err != ErrInvalidSearchScope {
		t.Errorf("Expected ErrInvalidSearchScope, got %v", err)
	}
}

// ============================================================================
// AddRequest Tests
// ============================================================================

func TestParseAddRequest_Basic(t *testing.T) {
	encoder := ber.NewBEREncoder(256)

	// entry DN
	encoder.WriteOctetString([]byte("uid=alice,ou=users,dc=example,dc=com"))

	// attributes SEQUENCE
	attrListPos := encoder.BeginSequence()

	// First attribute: objectClass
	attr1Pos := encoder.BeginSequence()
	encoder.WriteOctetString([]byte("objectClass"))
	valSet1Pos := encoder.BeginSet()
	encoder.WriteOctetString([]byte("inetOrgPerson"))
	encoder.WriteOctetString([]byte("organizationalPerson"))
	encoder.WriteOctetString([]byte("person"))
	encoder.EndSet(valSet1Pos)
	encoder.EndSequence(attr1Pos)

	// Second attribute: cn
	attr2Pos := encoder.BeginSequence()
	encoder.WriteOctetString([]byte("cn"))
	valSet2Pos := encoder.BeginSet()
	encoder.WriteOctetString([]byte("Alice Smith"))
	encoder.EndSet(valSet2Pos)
	encoder.EndSequence(attr2Pos)

	// Third attribute: uid
	attr3Pos := encoder.BeginSequence()
	encoder.WriteOctetString([]byte("uid"))
	valSet3Pos := encoder.BeginSet()
	encoder.WriteOctetString([]byte("alice"))
	encoder.EndSet(valSet3Pos)
	encoder.EndSequence(attr3Pos)

	encoder.EndSequence(attrListPos)

	data := encoder.Bytes()

	req, err := ParseAddRequest(data)
	if err != nil {
		t.Fatalf("ParseAddRequest failed: %v", err)
	}

	if req.Entry != "uid=alice,ou=users,dc=example,dc=com" {
		t.Errorf("Entry = %q, want %q", req.Entry, "uid=alice,ou=users,dc=example,dc=com")
	}

	if len(req.Attributes) != 3 {
		t.Fatalf("len(Attributes) = %d, want 3", len(req.Attributes))
	}

	// Check objectClass attribute
	objClass := req.GetAttribute("objectClass")
	if objClass == nil {
		t.Fatal("objectClass attribute not found")
	}
	if len(objClass.Values) != 3 {
		t.Errorf("len(objectClass.Values) = %d, want 3", len(objClass.Values))
	}

	// Check cn attribute
	cn := req.GetAttribute("cn")
	if cn == nil {
		t.Fatal("cn attribute not found")
	}
	if !bytes.Equal(cn.Values[0], []byte("Alice Smith")) {
		t.Errorf("cn value = %q, want %q", cn.Values[0], "Alice Smith")
	}

	// Check uid attribute
	uid := req.GetAttribute("uid")
	if uid == nil {
		t.Fatal("uid attribute not found")
	}
	if !bytes.Equal(uid.Values[0], []byte("alice")) {
		t.Errorf("uid value = %q, want %q", uid.Values[0], "alice")
	}
}

func TestAddRequest_Encode(t *testing.T) {
	req := &AddRequest{
		Entry: "uid=bob,ou=users,dc=example,dc=com",
		Attributes: []Attribute{
			{
				Type:   "objectClass",
				Values: [][]byte{[]byte("person")},
			},
			{
				Type:   "cn",
				Values: [][]byte{[]byte("Bob Jones")},
			},
		},
	}

	encoded, err := req.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// Parse it back
	parsed, err := ParseAddRequest(encoded)
	if err != nil {
		t.Fatalf("ParseAddRequest failed: %v", err)
	}

	if parsed.Entry != req.Entry {
		t.Errorf("Entry = %q, want %q", parsed.Entry, req.Entry)
	}

	if len(parsed.Attributes) != len(req.Attributes) {
		t.Errorf("len(Attributes) = %d, want %d", len(parsed.Attributes), len(req.Attributes))
	}
}

func TestAddRequest_GetAttributeStringValues(t *testing.T) {
	req := &AddRequest{
		Entry: "uid=test,dc=example,dc=com",
		Attributes: []Attribute{
			{
				Type:   "mail",
				Values: [][]byte{[]byte("test@example.com"), []byte("test2@example.com")},
			},
		},
	}

	values := req.GetAttributeStringValues("mail")
	if len(values) != 2 {
		t.Fatalf("len(values) = %d, want 2", len(values))
	}

	if values[0] != "test@example.com" {
		t.Errorf("values[0] = %q, want %q", values[0], "test@example.com")
	}

	if values[1] != "test2@example.com" {
		t.Errorf("values[1] = %q, want %q", values[1], "test2@example.com")
	}

	// Test non-existent attribute
	nilValues := req.GetAttributeStringValues("nonexistent")
	if nilValues != nil {
		t.Errorf("Expected nil for non-existent attribute, got %v", nilValues)
	}
}

// ============================================================================
// DeleteRequest Tests
// ============================================================================

func TestParseDeleteRequest_Basic(t *testing.T) {
	dn := "uid=alice,ou=users,dc=example,dc=com"
	data := []byte(dn)

	req, err := ParseDeleteRequest(data)
	if err != nil {
		t.Fatalf("ParseDeleteRequest failed: %v", err)
	}

	if req.DN != dn {
		t.Errorf("DN = %q, want %q", req.DN, dn)
	}
}

func TestDeleteRequest_Encode(t *testing.T) {
	req := &DeleteRequest{
		DN: "uid=bob,ou=users,dc=example,dc=com",
	}

	encoded, err := req.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// Parse it back
	parsed, err := ParseDeleteRequest(encoded)
	if err != nil {
		t.Fatalf("ParseDeleteRequest failed: %v", err)
	}

	if parsed.DN != req.DN {
		t.Errorf("DN = %q, want %q", parsed.DN, req.DN)
	}
}

func TestDeleteRequest_Validate(t *testing.T) {
	// Valid request
	req := &DeleteRequest{DN: "uid=alice,dc=example,dc=com"}
	if err := req.Validate(); err != nil {
		t.Errorf("Validate failed for valid request: %v", err)
	}

	// Empty DN
	req = &DeleteRequest{DN: ""}
	if err := req.Validate(); err != ErrEmptyDeleteDN {
		t.Errorf("Expected ErrEmptyDeleteDN, got %v", err)
	}
}

// ============================================================================
// UnbindRequest Tests
// ============================================================================

func TestParseUnbindRequest(t *testing.T) {
	// UnbindRequest is NULL, so data should be empty
	req, err := ParseUnbindRequest([]byte{})
	if err != nil {
		t.Fatalf("ParseUnbindRequest failed: %v", err)
	}

	if req == nil {
		t.Fatal("req is nil")
	}
}

func TestUnbindRequest_Encode(t *testing.T) {
	req := &UnbindRequest{}

	encoded, err := req.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	if len(encoded) != 0 {
		t.Errorf("len(encoded) = %d, want 0", len(encoded))
	}
}

// ============================================================================
// ModifyRequest Tests
// ============================================================================

func TestParseModifyRequest_Basic(t *testing.T) {
	encoder := ber.NewBEREncoder(256)

	// object DN
	encoder.WriteOctetString([]byte("uid=alice,ou=users,dc=example,dc=com"))

	// changes SEQUENCE
	changesPos := encoder.BeginSequence()

	// First change: add mail
	change1Pos := encoder.BeginSequence()
	encoder.WriteEnumerated(int64(ModifyOperationAdd))
	// modification (PartialAttribute)
	mod1Pos := encoder.BeginSequence()
	encoder.WriteOctetString([]byte("mail"))
	valSet1Pos := encoder.BeginSet()
	encoder.WriteOctetString([]byte("alice@example.com"))
	encoder.EndSet(valSet1Pos)
	encoder.EndSequence(mod1Pos)
	encoder.EndSequence(change1Pos)

	// Second change: replace description
	change2Pos := encoder.BeginSequence()
	encoder.WriteEnumerated(int64(ModifyOperationReplace))
	mod2Pos := encoder.BeginSequence()
	encoder.WriteOctetString([]byte("description"))
	valSet2Pos := encoder.BeginSet()
	encoder.WriteOctetString([]byte("Updated description"))
	encoder.EndSet(valSet2Pos)
	encoder.EndSequence(mod2Pos)
	encoder.EndSequence(change2Pos)

	// Third change: delete telephoneNumber
	change3Pos := encoder.BeginSequence()
	encoder.WriteEnumerated(int64(ModifyOperationDelete))
	mod3Pos := encoder.BeginSequence()
	encoder.WriteOctetString([]byte("telephoneNumber"))
	valSet3Pos := encoder.BeginSet()
	encoder.EndSet(valSet3Pos)
	encoder.EndSequence(mod3Pos)
	encoder.EndSequence(change3Pos)

	encoder.EndSequence(changesPos)

	data := encoder.Bytes()

	req, err := ParseModifyRequest(data)
	if err != nil {
		t.Fatalf("ParseModifyRequest failed: %v", err)
	}

	if req.Object != "uid=alice,ou=users,dc=example,dc=com" {
		t.Errorf("Object = %q, want %q", req.Object, "uid=alice,ou=users,dc=example,dc=com")
	}

	if len(req.Changes) != 3 {
		t.Fatalf("len(Changes) = %d, want 3", len(req.Changes))
	}

	// Check first change (add)
	if req.Changes[0].Operation != ModifyOperationAdd {
		t.Errorf("Changes[0].Operation = %v, want Add", req.Changes[0].Operation)
	}
	if req.Changes[0].Attribute.Type != "mail" {
		t.Errorf("Changes[0].Attribute.Type = %q, want %q", req.Changes[0].Attribute.Type, "mail")
	}
	if !bytes.Equal(req.Changes[0].Attribute.Values[0], []byte("alice@example.com")) {
		t.Errorf("Changes[0].Attribute.Values[0] = %q, want %q", req.Changes[0].Attribute.Values[0], "alice@example.com")
	}

	// Check second change (replace)
	if req.Changes[1].Operation != ModifyOperationReplace {
		t.Errorf("Changes[1].Operation = %v, want Replace", req.Changes[1].Operation)
	}
	if req.Changes[1].Attribute.Type != "description" {
		t.Errorf("Changes[1].Attribute.Type = %q, want %q", req.Changes[1].Attribute.Type, "description")
	}

	// Check third change (delete)
	if req.Changes[2].Operation != ModifyOperationDelete {
		t.Errorf("Changes[2].Operation = %v, want Delete", req.Changes[2].Operation)
	}
	if req.Changes[2].Attribute.Type != "telephoneNumber" {
		t.Errorf("Changes[2].Attribute.Type = %q, want %q", req.Changes[2].Attribute.Type, "telephoneNumber")
	}
}

func TestModifyRequest_Encode(t *testing.T) {
	req := &ModifyRequest{
		Object: "uid=bob,ou=users,dc=example,dc=com",
		Changes: []Modification{
			{
				Operation: ModifyOperationAdd,
				Attribute: Attribute{
					Type:   "mail",
					Values: [][]byte{[]byte("bob@example.com")},
				},
			},
			{
				Operation: ModifyOperationReplace,
				Attribute: Attribute{
					Type:   "cn",
					Values: [][]byte{[]byte("Robert Jones")},
				},
			},
		},
	}

	encoded, err := req.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// Parse it back
	parsed, err := ParseModifyRequest(encoded)
	if err != nil {
		t.Fatalf("ParseModifyRequest failed: %v", err)
	}

	if parsed.Object != req.Object {
		t.Errorf("Object = %q, want %q", parsed.Object, req.Object)
	}

	if len(parsed.Changes) != len(req.Changes) {
		t.Errorf("len(Changes) = %d, want %d", len(parsed.Changes), len(req.Changes))
	}

	for i := range req.Changes {
		if parsed.Changes[i].Operation != req.Changes[i].Operation {
			t.Errorf("Changes[%d].Operation = %v, want %v", i, parsed.Changes[i].Operation, req.Changes[i].Operation)
		}
		if parsed.Changes[i].Attribute.Type != req.Changes[i].Attribute.Type {
			t.Errorf("Changes[%d].Attribute.Type = %q, want %q", i, parsed.Changes[i].Attribute.Type, req.Changes[i].Attribute.Type)
		}
	}
}

func TestModifyRequest_Validate(t *testing.T) {
	// Valid request
	req := &ModifyRequest{
		Object: "uid=alice,dc=example,dc=com",
		Changes: []Modification{
			{
				Operation: ModifyOperationAdd,
				Attribute: Attribute{Type: "mail", Values: [][]byte{[]byte("test@example.com")}},
			},
		},
	}
	if err := req.Validate(); err != nil {
		t.Errorf("Validate failed for valid request: %v", err)
	}

	// Empty object
	req = &ModifyRequest{
		Object: "",
		Changes: []Modification{
			{Operation: ModifyOperationAdd, Attribute: Attribute{Type: "mail"}},
		},
	}
	if err := req.Validate(); err != ErrEmptyModifyObject {
		t.Errorf("Expected ErrEmptyModifyObject, got %v", err)
	}

	// Empty changes
	req = &ModifyRequest{
		Object:  "uid=alice,dc=example,dc=com",
		Changes: []Modification{},
	}
	if err := req.Validate(); err != ErrEmptyModifications {
		t.Errorf("Expected ErrEmptyModifications, got %v", err)
	}

	// Invalid operation
	req = &ModifyRequest{
		Object: "uid=alice,dc=example,dc=com",
		Changes: []Modification{
			{Operation: ModifyOperation(99), Attribute: Attribute{Type: "mail"}},
		},
	}
	if err := req.Validate(); err != ErrInvalidModifyOperation {
		t.Errorf("Expected ErrInvalidModifyOperation, got %v", err)
	}
}

func TestModifyRequest_AddModification(t *testing.T) {
	req := &ModifyRequest{
		Object: "uid=alice,dc=example,dc=com",
	}

	req.AddModification(ModifyOperationAdd, "mail", []byte("alice@example.com"))
	req.AddStringModification(ModifyOperationReplace, "cn", "Alice Smith")

	if len(req.Changes) != 2 {
		t.Fatalf("len(Changes) = %d, want 2", len(req.Changes))
	}

	if req.Changes[0].Operation != ModifyOperationAdd {
		t.Errorf("Changes[0].Operation = %v, want Add", req.Changes[0].Operation)
	}

	if req.Changes[1].Operation != ModifyOperationReplace {
		t.Errorf("Changes[1].Operation = %v, want Replace", req.Changes[1].Operation)
	}

	if !bytes.Equal(req.Changes[1].Attribute.Values[0], []byte("Alice Smith")) {
		t.Errorf("Changes[1].Attribute.Values[0] = %q, want %q", req.Changes[1].Attribute.Values[0], "Alice Smith")
	}
}

// ============================================================================
// AbandonRequest Tests
// ============================================================================

func TestParseAbandonRequest_Basic(t *testing.T) {
	// MessageID = 42 encoded as raw integer bytes
	data := []byte{42}

	req, err := ParseAbandonRequest(data)
	if err != nil {
		t.Fatalf("ParseAbandonRequest failed: %v", err)
	}

	if req.MessageID != 42 {
		t.Errorf("MessageID = %d, want 42", req.MessageID)
	}
}

func TestParseAbandonRequest_LargeMessageID(t *testing.T) {
	// MessageID = 1000 (0x03E8) encoded as raw integer bytes
	data := []byte{0x03, 0xE8}

	req, err := ParseAbandonRequest(data)
	if err != nil {
		t.Fatalf("ParseAbandonRequest failed: %v", err)
	}

	if req.MessageID != 1000 {
		t.Errorf("MessageID = %d, want 1000", req.MessageID)
	}
}

// ============================================================================
// ModifyDNRequest Tests
// ============================================================================

func TestParseModifyDNRequest_BasicRename(t *testing.T) {
	encoder := ber.NewBEREncoder(128)

	// entry = "uid=alice,ou=users,dc=example,dc=com"
	encoder.WriteOctetString([]byte("uid=alice,ou=users,dc=example,dc=com"))
	// newrdn = "uid=alice2"
	encoder.WriteOctetString([]byte("uid=alice2"))
	// deleteoldrdn = true
	encoder.WriteBoolean(true)

	data := encoder.Bytes()

	req, err := ParseModifyDNRequest(data)
	if err != nil {
		t.Fatalf("ParseModifyDNRequest failed: %v", err)
	}

	if req.Entry != "uid=alice,ou=users,dc=example,dc=com" {
		t.Errorf("Entry = %q, want %q", req.Entry, "uid=alice,ou=users,dc=example,dc=com")
	}

	if req.NewRDN != "uid=alice2" {
		t.Errorf("NewRDN = %q, want %q", req.NewRDN, "uid=alice2")
	}

	if !req.DeleteOldRDN {
		t.Error("DeleteOldRDN = false, want true")
	}

	if req.NewSuperior != "" {
		t.Errorf("NewSuperior = %q, want empty", req.NewSuperior)
	}
}

func TestParseModifyDNRequest_WithNewSuperior(t *testing.T) {
	encoder := ber.NewBEREncoder(256)

	// entry = "uid=alice,ou=users,dc=example,dc=com"
	encoder.WriteOctetString([]byte("uid=alice,ou=users,dc=example,dc=com"))
	// newrdn = "uid=alice"
	encoder.WriteOctetString([]byte("uid=alice"))
	// deleteoldrdn = false
	encoder.WriteBoolean(false)
	// newSuperior [0] = "ou=people,dc=example,dc=com"
	encoder.WriteTaggedValue(0, false, []byte("ou=people,dc=example,dc=com"))

	data := encoder.Bytes()

	req, err := ParseModifyDNRequest(data)
	if err != nil {
		t.Fatalf("ParseModifyDNRequest failed: %v", err)
	}

	if req.Entry != "uid=alice,ou=users,dc=example,dc=com" {
		t.Errorf("Entry = %q, want %q", req.Entry, "uid=alice,ou=users,dc=example,dc=com")
	}

	if req.NewRDN != "uid=alice" {
		t.Errorf("NewRDN = %q, want %q", req.NewRDN, "uid=alice")
	}

	if req.DeleteOldRDN {
		t.Error("DeleteOldRDN = true, want false")
	}

	if req.NewSuperior != "ou=people,dc=example,dc=com" {
		t.Errorf("NewSuperior = %q, want %q", req.NewSuperior, "ou=people,dc=example,dc=com")
	}
}

func TestParseModifyDNRequest_DeleteOldRDNFalse(t *testing.T) {
	encoder := ber.NewBEREncoder(128)

	// entry = "cn=admin,dc=example,dc=com"
	encoder.WriteOctetString([]byte("cn=admin,dc=example,dc=com"))
	// newrdn = "cn=administrator"
	encoder.WriteOctetString([]byte("cn=administrator"))
	// deleteoldrdn = false
	encoder.WriteBoolean(false)

	data := encoder.Bytes()

	req, err := ParseModifyDNRequest(data)
	if err != nil {
		t.Fatalf("ParseModifyDNRequest failed: %v", err)
	}

	if req.DeleteOldRDN {
		t.Error("DeleteOldRDN = true, want false")
	}
}

func TestParseModifyDNRequest_EmptyData(t *testing.T) {
	_, err := ParseModifyDNRequest([]byte{})
	if err == nil {
		t.Error("Expected error for empty data")
	}
}

func TestModifyDNRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     *ModifyDNRequest
		wantErr bool
	}{
		{
			name: "valid request",
			req: &ModifyDNRequest{
				Entry:        "uid=alice,ou=users,dc=example,dc=com",
				NewRDN:       "uid=alice2",
				DeleteOldRDN: true,
			},
			wantErr: false,
		},
		{
			name: "empty entry",
			req: &ModifyDNRequest{
				Entry:        "",
				NewRDN:       "uid=alice2",
				DeleteOldRDN: true,
			},
			wantErr: true,
		},
		{
			name: "empty new RDN",
			req: &ModifyDNRequest{
				Entry:        "uid=alice,ou=users,dc=example,dc=com",
				NewRDN:       "",
				DeleteOldRDN: true,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestModifyDNRequest_HasNewSuperior(t *testing.T) {
	tests := []struct {
		name        string
		newSuperior string
		want        bool
	}{
		{
			name:        "with new superior",
			newSuperior: "ou=people,dc=example,dc=com",
			want:        true,
		},
		{
			name:        "without new superior",
			newSuperior: "",
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &ModifyDNRequest{
				Entry:       "uid=alice,ou=users,dc=example,dc=com",
				NewRDN:      "uid=alice2",
				NewSuperior: tt.newSuperior,
			}

			if got := req.HasNewSuperior(); got != tt.want {
				t.Errorf("HasNewSuperior() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestModifyDNRequest_Encode(t *testing.T) {
	req := &ModifyDNRequest{
		Entry:        "uid=alice,ou=users,dc=example,dc=com",
		NewRDN:       "uid=alice2",
		DeleteOldRDN: true,
	}

	data, err := req.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// Parse it back
	parsed, err := ParseModifyDNRequest(data)
	if err != nil {
		t.Fatalf("ParseModifyDNRequest failed: %v", err)
	}

	if parsed.Entry != req.Entry {
		t.Errorf("Entry = %q, want %q", parsed.Entry, req.Entry)
	}

	if parsed.NewRDN != req.NewRDN {
		t.Errorf("NewRDN = %q, want %q", parsed.NewRDN, req.NewRDN)
	}

	if parsed.DeleteOldRDN != req.DeleteOldRDN {
		t.Errorf("DeleteOldRDN = %v, want %v", parsed.DeleteOldRDN, req.DeleteOldRDN)
	}
}

func TestModifyDNRequest_EncodeWithNewSuperior(t *testing.T) {
	req := &ModifyDNRequest{
		Entry:        "uid=alice,ou=users,dc=example,dc=com",
		NewRDN:       "uid=alice",
		DeleteOldRDN: false,
		NewSuperior:  "ou=people,dc=example,dc=com",
	}

	data, err := req.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// Parse it back
	parsed, err := ParseModifyDNRequest(data)
	if err != nil {
		t.Fatalf("ParseModifyDNRequest failed: %v", err)
	}

	if parsed.Entry != req.Entry {
		t.Errorf("Entry = %q, want %q", parsed.Entry, req.Entry)
	}

	if parsed.NewRDN != req.NewRDN {
		t.Errorf("NewRDN = %q, want %q", parsed.NewRDN, req.NewRDN)
	}

	if parsed.DeleteOldRDN != req.DeleteOldRDN {
		t.Errorf("DeleteOldRDN = %v, want %v", parsed.DeleteOldRDN, req.DeleteOldRDN)
	}

	if parsed.NewSuperior != req.NewSuperior {
		t.Errorf("NewSuperior = %q, want %q", parsed.NewSuperior, req.NewSuperior)
	}
}
