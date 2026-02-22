// Package backend provides the LDAP backend interface tests.
package backend

import (
	"regexp"
	"strings"
	"testing"
	"time"
)

// TestGenerateUUID tests UUID generation.
func TestGenerateUUID(t *testing.T) {
	uuid := GenerateUUID()

	// Check UUID format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	uuidRegex := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	if !uuidRegex.MatchString(uuid) {
		t.Errorf("UUID format invalid: %s", uuid)
	}

	// Check that UUIDs are unique
	uuid2 := GenerateUUID()
	if uuid == uuid2 {
		t.Error("UUIDs should be unique")
	}
}

// TestGenerateUUIDVersion tests that generated UUIDs are version 4.
func TestGenerateUUIDVersion(t *testing.T) {
	for i := 0; i < 100; i++ {
		uuid := GenerateUUID()
		parts := strings.Split(uuid, "-")
		if len(parts) != 5 {
			t.Fatalf("UUID should have 5 parts: %s", uuid)
		}

		// Check version 4 (third part starts with 4)
		if parts[2][0] != '4' {
			t.Errorf("UUID version should be 4, got: %s", uuid)
		}

		// Check variant (fourth part starts with 8, 9, a, or b)
		firstChar := parts[3][0]
		if firstChar != '8' && firstChar != '9' && firstChar != 'a' && firstChar != 'b' {
			t.Errorf("UUID variant should be RFC 4122, got: %s", uuid)
		}
	}
}

// TestFormatTimestamp tests timestamp formatting.
func TestFormatTimestamp(t *testing.T) {
	tests := []struct {
		name     string
		time     time.Time
		expected string
	}{
		{
			name:     "basic timestamp",
			time:     time.Date(2026, 2, 18, 10, 30, 0, 0, time.UTC),
			expected: "20260218103000Z",
		},
		{
			name:     "with seconds",
			time:     time.Date(2026, 2, 18, 15, 30, 45, 0, time.UTC),
			expected: "20260218153045Z",
		},
		{
			name:     "midnight",
			time:     time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			expected: "20260101000000Z",
		},
		{
			name:     "end of day",
			time:     time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC),
			expected: "20261231235959Z",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatTimestamp(tt.time)
			if result != tt.expected {
				t.Errorf("FormatTimestamp() = %s, expected %s", result, tt.expected)
			}
		})
	}
}

// TestFormatTimestampNonUTC tests that non-UTC times are converted to UTC.
func TestFormatTimestampNonUTC(t *testing.T) {
	// Create a time in a non-UTC timezone
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip("Could not load timezone")
	}

	// 10:30 AM EST = 15:30 UTC
	localTime := time.Date(2026, 2, 18, 10, 30, 0, 0, loc)
	result := FormatTimestamp(localTime)

	// Should be converted to UTC
	if !strings.HasSuffix(result, "Z") {
		t.Errorf("Timestamp should end with Z: %s", result)
	}
}

// TestParseTimestamp tests timestamp parsing.
func TestParseTimestamp(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected time.Time
	}{
		{
			name:     "basic timestamp",
			input:    "20260218103000Z",
			expected: time.Date(2026, 2, 18, 10, 30, 0, 0, time.UTC),
		},
		{
			name:     "with seconds",
			input:    "20260218153045Z",
			expected: time.Date(2026, 2, 18, 15, 30, 45, 0, time.UTC),
		},
		{
			name:     "invalid format",
			input:    "invalid",
			expected: time.Time{},
		},
		{
			name:     "empty string",
			input:    "",
			expected: time.Time{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseTimestamp(tt.input)
			if !result.Equal(tt.expected) {
				t.Errorf("ParseTimestamp(%s) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

// TestSetOperationalAttrsAdd tests setting operational attributes for add operations.
func TestSetOperationalAttrsAdd(t *testing.T) {
	entry := NewEntry("uid=test,ou=users,dc=example,dc=com")
	bindDN := "cn=admin,dc=example,dc=com"

	SetOperationalAttrs(entry, OpAdd, bindDN)

	// Check createTimestamp is set
	createTimestamp := entry.GetFirstAttribute(AttrCreateTimestamp)
	if createTimestamp == "" {
		t.Error("createTimestamp should be set")
	}

	// Check modifyTimestamp is set
	modifyTimestamp := entry.GetFirstAttribute(AttrModifyTimestamp)
	if modifyTimestamp == "" {
		t.Error("modifyTimestamp should be set")
	}

	// Check creatorsName is set
	creatorsName := entry.GetFirstAttribute(AttrCreatorsName)
	if creatorsName != bindDN {
		t.Errorf("creatorsName = %s, expected %s", creatorsName, bindDN)
	}

	// Check modifiersName is set
	modifiersName := entry.GetFirstAttribute(AttrModifiersName)
	if modifiersName != bindDN {
		t.Errorf("modifiersName = %s, expected %s", modifiersName, bindDN)
	}

	// Check entryUUID is set
	entryUUID := entry.GetFirstAttribute(AttrEntryUUID)
	if entryUUID == "" {
		t.Error("entryUUID should be set")
	}

	// Check entryDN is set
	entryDN := entry.GetFirstAttribute(AttrEntryDN)
	if entryDN != entry.DN {
		t.Errorf("entryDN = %s, expected %s", entryDN, entry.DN)
	}
}

// TestSetOperationalAttrsModify tests setting operational attributes for modify operations.
func TestSetOperationalAttrsModify(t *testing.T) {
	entry := NewEntry("uid=test,ou=users,dc=example,dc=com")
	bindDN := "cn=admin,dc=example,dc=com"

	// First add the entry
	SetOperationalAttrs(entry, OpAdd, "cn=creator,dc=example,dc=com")

	// Store original values
	originalCreateTimestamp := entry.GetFirstAttribute(AttrCreateTimestamp)
	originalCreatorsName := entry.GetFirstAttribute(AttrCreatorsName)
	originalEntryUUID := entry.GetFirstAttribute(AttrEntryUUID)

	// Wait a bit to ensure different timestamp
	time.Sleep(10 * time.Millisecond)

	// Now modify the entry
	SetOperationalAttrs(entry, OpModify, bindDN)

	// Check createTimestamp is unchanged
	createTimestamp := entry.GetFirstAttribute(AttrCreateTimestamp)
	if createTimestamp != originalCreateTimestamp {
		t.Error("createTimestamp should not change on modify")
	}

	// Check creatorsName is unchanged
	creatorsName := entry.GetFirstAttribute(AttrCreatorsName)
	if creatorsName != originalCreatorsName {
		t.Error("creatorsName should not change on modify")
	}

	// Check entryUUID is unchanged
	entryUUID := entry.GetFirstAttribute(AttrEntryUUID)
	if entryUUID != originalEntryUUID {
		t.Error("entryUUID should not change on modify")
	}

	// Check modifyTimestamp is updated
	modifyTimestamp := entry.GetFirstAttribute(AttrModifyTimestamp)
	if modifyTimestamp == "" {
		t.Error("modifyTimestamp should be set")
	}

	// Check modifiersName is updated
	modifiersName := entry.GetFirstAttribute(AttrModifiersName)
	if modifiersName != bindDN {
		t.Errorf("modifiersName = %s, expected %s", modifiersName, bindDN)
	}
}

// TestSetOperationalAttrsNilEntry tests that nil entry is handled gracefully.
func TestSetOperationalAttrsNilEntry(t *testing.T) {
	// Should not panic
	SetOperationalAttrs(nil, OpAdd, "cn=admin,dc=example,dc=com")
}

// TestSetSubordinateAttrs tests setting subordinate attributes.
func TestSetSubordinateAttrs(t *testing.T) {
	tests := []struct {
		name            string
		hasSubordinates bool
		numSubordinates int
		expectedHas     string
		expectedNum     string
	}{
		{
			name:            "no subordinates",
			hasSubordinates: false,
			numSubordinates: 0,
			expectedHas:     "FALSE",
			expectedNum:     "0",
		},
		{
			name:            "has subordinates",
			hasSubordinates: true,
			numSubordinates: 5,
			expectedHas:     "TRUE",
			expectedNum:     "5",
		},
		{
			name:            "many subordinates",
			hasSubordinates: true,
			numSubordinates: 100,
			expectedHas:     "TRUE",
			expectedNum:     "100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := NewEntry("ou=test,dc=example,dc=com")
			SetSubordinateAttrs(entry, tt.hasSubordinates, tt.numSubordinates)

			hasSubordinates := entry.GetFirstAttribute(AttrHasSubordinates)
			if hasSubordinates != tt.expectedHas {
				t.Errorf("hasSubordinates = %s, expected %s", hasSubordinates, tt.expectedHas)
			}

			numSubordinates := entry.GetFirstAttribute(AttrNumSubordinates)
			if numSubordinates != tt.expectedNum {
				t.Errorf("numSubordinates = %s, expected %s", numSubordinates, tt.expectedNum)
			}
		})
	}
}

// TestSetSubordinateAttrsNilEntry tests that nil entry is handled gracefully.
func TestSetSubordinateAttrsNilEntry(t *testing.T) {
	// Should not panic
	SetSubordinateAttrs(nil, true, 5)
}

// TestOperationalAttrConstants tests that operational attribute constants are correct.
func TestOperationalAttrConstants(t *testing.T) {
	// Verify constants match expected LDAP attribute names
	tests := []struct {
		constant string
		expected string
	}{
		{AttrCreateTimestamp, "createTimestamp"},
		{AttrModifyTimestamp, "modifyTimestamp"},
		{AttrCreatorsName, "creatorsName"},
		{AttrModifiersName, "modifiersName"},
		{AttrEntryDN, "entryDN"},
		{AttrEntryUUID, "entryUUID"},
		{AttrSubschemaSubentry, "subschemaSubentry"},
		{AttrHasSubordinates, "hasSubordinates"},
		{AttrNumSubordinates, "numSubordinates"},
	}

	for _, tt := range tests {
		if tt.constant != tt.expected {
			t.Errorf("Constant value = %s, expected %s", tt.constant, tt.expected)
		}
	}
}

// TestTimestampRoundTrip tests that timestamps can be formatted and parsed back.
func TestTimestampRoundTrip(t *testing.T) {
	original := time.Date(2026, 2, 18, 10, 30, 45, 0, time.UTC)
	formatted := FormatTimestamp(original)
	parsed := ParseTimestamp(formatted)

	if !parsed.Equal(original) {
		t.Errorf("Round trip failed: original=%v, parsed=%v", original, parsed)
	}
}

// TestUUIDUniqueness tests that generated UUIDs are unique.
func TestUUIDUniqueness(t *testing.T) {
	uuids := make(map[string]bool)
	count := 1000

	for i := 0; i < count; i++ {
		uuid := GenerateUUID()
		if uuids[uuid] {
			t.Errorf("Duplicate UUID generated: %s", uuid)
		}
		uuids[uuid] = true
	}

	if len(uuids) != count {
		t.Errorf("Expected %d unique UUIDs, got %d", count, len(uuids))
	}
}

// TestAddWithBindDNSetsOperationalAttrs tests that AddWithBindDN sets operational attributes.
func TestAddWithBindDNSetsOperationalAttrs(t *testing.T) {
	engine := newMockStorageEngine()
	backend := NewBackend(engine, nil)

	entry := NewEntry("uid=test,ou=users,dc=example,dc=com")
	entry.SetAttribute("objectClass", "inetOrgPerson")
	entry.SetAttribute("uid", "test")
	entry.SetAttribute("cn", "Test User")
	entry.SetAttribute("sn", "User")

	bindDN := "cn=admin,dc=example,dc=com"

	err := backend.AddWithBindDN(entry, bindDN)
	if err != nil {
		t.Fatalf("AddWithBindDN failed: %v", err)
	}

	// Retrieve the entry from storage
	storedEntry, ok := engine.entries["uid=test,ou=users,dc=example,dc=com"]
	if !ok {
		t.Fatal("Entry not found in storage")
	}

	// Check createTimestamp is set
	createTimestamp := storedEntry.GetAttribute("createtimestamp")
	if len(createTimestamp) == 0 {
		t.Error("createTimestamp should be set")
	}

	// Check modifyTimestamp is set
	modifyTimestamp := storedEntry.GetAttribute("modifytimestamp")
	if len(modifyTimestamp) == 0 {
		t.Error("modifyTimestamp should be set")
	}

	// Check creatorsName is set
	creatorsName := storedEntry.GetAttribute("creatorsname")
	if len(creatorsName) == 0 || string(creatorsName[0]) != bindDN {
		t.Errorf("creatorsName = %v, expected %s", creatorsName, bindDN)
	}

	// Check modifiersName is set
	modifiersName := storedEntry.GetAttribute("modifiersname")
	if len(modifiersName) == 0 || string(modifiersName[0]) != bindDN {
		t.Errorf("modifiersName = %v, expected %s", modifiersName, bindDN)
	}

	// Check entryUUID is set
	entryUUID := storedEntry.GetAttribute("entryuuid")
	if len(entryUUID) == 0 {
		t.Error("entryUUID should be set")
	}

	// Check entryDN is set
	entryDN := storedEntry.GetAttribute("entrydn")
	if len(entryDN) == 0 || string(entryDN[0]) != "uid=test,ou=users,dc=example,dc=com" {
		t.Errorf("entryDN = %v, expected uid=test,ou=users,dc=example,dc=com", entryDN)
	}
}

// TestModifyWithBindDNSetsOperationalAttrs tests that ModifyWithBindDN sets operational attributes.
func TestModifyWithBindDNSetsOperationalAttrs(t *testing.T) {
	engine := newMockStorageEngine()
	backend := NewBackend(engine, nil)

	// First add an entry
	entry := NewEntry("uid=test,ou=users,dc=example,dc=com")
	entry.SetAttribute("objectClass", "inetOrgPerson")
	entry.SetAttribute("uid", "test")
	entry.SetAttribute("cn", "Test User")
	entry.SetAttribute("sn", "User")

	creatorDN := "cn=creator,dc=example,dc=com"
	err := backend.AddWithBindDN(entry, creatorDN)
	if err != nil {
		t.Fatalf("AddWithBindDN failed: %v", err)
	}

	// Get original values
	storedEntry, _ := engine.entries["uid=test,ou=users,dc=example,dc=com"]
	originalCreateTimestamp := string(storedEntry.GetAttribute("createtimestamp")[0])
	originalCreatorsName := string(storedEntry.GetAttribute("creatorsname")[0])
	originalEntryUUID := string(storedEntry.GetAttribute("entryuuid")[0])

	// Wait a bit to ensure different timestamp
	time.Sleep(10 * time.Millisecond)

	// Now modify the entry
	modifierDN := "cn=modifier,dc=example,dc=com"
	changes := []Modification{
		{Type: ModReplace, Attribute: "cn", Values: []string{"Modified User"}},
	}

	err = backend.ModifyWithBindDN("uid=test,ou=users,dc=example,dc=com", changes, modifierDN)
	if err != nil {
		t.Fatalf("ModifyWithBindDN failed: %v", err)
	}

	// Retrieve the modified entry
	modifiedEntry, _ := engine.entries["uid=test,ou=users,dc=example,dc=com"]

	// Check createTimestamp is unchanged
	createTimestamp := string(modifiedEntry.GetAttribute("createtimestamp")[0])
	if createTimestamp != originalCreateTimestamp {
		t.Error("createTimestamp should not change on modify")
	}

	// Check creatorsName is unchanged
	creatorsName := string(modifiedEntry.GetAttribute("creatorsname")[0])
	if creatorsName != originalCreatorsName {
		t.Error("creatorsName should not change on modify")
	}

	// Check entryUUID is unchanged
	entryUUID := string(modifiedEntry.GetAttribute("entryuuid")[0])
	if entryUUID != originalEntryUUID {
		t.Error("entryUUID should not change on modify")
	}

	// Check modifyTimestamp is updated
	modifyTimestamp := modifiedEntry.GetAttribute("modifytimestamp")
	if len(modifyTimestamp) == 0 {
		t.Error("modifyTimestamp should be set")
	}

	// Check modifiersName is updated
	modifiersName := string(modifiedEntry.GetAttribute("modifiersname")[0])
	if modifiersName != modifierDN {
		t.Errorf("modifiersName = %s, expected %s", modifiersName, modifierDN)
	}
}
