// Package server provides the LDAP server implementation.
package server

import (
	"testing"

	"github.com/KilimcininKorOglu/oba/internal/ldap"
	"github.com/KilimcininKorOglu/oba/internal/storage"
)

// TestSortEntries_SingleAttribute tests sorting by a single attribute.
func TestSortEntries_SingleAttribute(t *testing.T) {
	entries := []*storage.Entry{
		createTestEntry("uid=charlie,dc=example,dc=com", "cn", "Charlie"),
		createTestEntry("uid=alice,dc=example,dc=com", "cn", "Alice"),
		createTestEntry("uid=bob,dc=example,dc=com", "cn", "Bob"),
	}

	ctrl := NewSortControl(NewSortKey("cn"))
	result := SortEntries(entries, ctrl)

	expected := []string{"Alice", "Bob", "Charlie"}
	for i, entry := range result {
		cn := getFirstAttributeValue(entry, "cn")
		if cn != expected[i] {
			t.Errorf("Entry %d: expected cn=%s, got cn=%s", i, expected[i], cn)
		}
	}
}

// TestSortEntries_SingleAttributeReverse tests sorting by a single attribute in reverse order.
func TestSortEntries_SingleAttributeReverse(t *testing.T) {
	entries := []*storage.Entry{
		createTestEntry("uid=charlie,dc=example,dc=com", "cn", "Charlie"),
		createTestEntry("uid=alice,dc=example,dc=com", "cn", "Alice"),
		createTestEntry("uid=bob,dc=example,dc=com", "cn", "Bob"),
	}

	ctrl := NewSortControl(NewReverseSortKey("cn"))
	result := SortEntries(entries, ctrl)

	expected := []string{"Charlie", "Bob", "Alice"}
	for i, entry := range result {
		cn := getFirstAttributeValue(entry, "cn")
		if cn != expected[i] {
			t.Errorf("Entry %d: expected cn=%s, got cn=%s", i, expected[i], cn)
		}
	}
}

// TestSortEntries_MultiAttribute tests sorting by multiple attributes.
func TestSortEntries_MultiAttribute(t *testing.T) {
	entries := []*storage.Entry{
		createTestEntryMulti("uid=alice2,dc=example,dc=com", map[string]string{"sn": "Smith", "givenName": "Alice"}),
		createTestEntryMulti("uid=bob,dc=example,dc=com", map[string]string{"sn": "Jones", "givenName": "Bob"}),
		createTestEntryMulti("uid=alice1,dc=example,dc=com", map[string]string{"sn": "Smith", "givenName": "Amy"}),
		createTestEntryMulti("uid=charlie,dc=example,dc=com", map[string]string{"sn": "Brown", "givenName": "Charlie"}),
	}

	// Sort by sn (surname) first, then by givenName
	ctrl := NewSortControl(
		NewSortKey("sn"),
		NewSortKey("givenName"),
	)
	result := SortEntries(entries, ctrl)

	// Expected order: Brown/Charlie, Jones/Bob, Smith/Alice, Smith/Amy
	expectedSn := []string{"Brown", "Jones", "Smith", "Smith"}
	expectedGiven := []string{"Charlie", "Bob", "Alice", "Amy"}

	for i, entry := range result {
		sn := getFirstAttributeValue(entry, "sn")
		given := getFirstAttributeValue(entry, "givenName")
		if sn != expectedSn[i] || given != expectedGiven[i] {
			t.Errorf("Entry %d: expected sn=%s givenName=%s, got sn=%s givenName=%s",
				i, expectedSn[i], expectedGiven[i], sn, given)
		}
	}
}

// TestSortEntries_MultiAttributeMixedOrder tests multi-attribute sorting with mixed order.
func TestSortEntries_MultiAttributeMixedOrder(t *testing.T) {
	entries := []*storage.Entry{
		createTestEntryMulti("uid=1,dc=example,dc=com", map[string]string{"department": "Engineering", "seniority": "3"}),
		createTestEntryMulti("uid=2,dc=example,dc=com", map[string]string{"department": "Engineering", "seniority": "1"}),
		createTestEntryMulti("uid=3,dc=example,dc=com", map[string]string{"department": "Sales", "seniority": "2"}),
		createTestEntryMulti("uid=4,dc=example,dc=com", map[string]string{"department": "Engineering", "seniority": "2"}),
	}

	// Sort by department ascending, then by seniority descending
	ctrl := NewSortControl(
		NewSortKey("department"),
		NewReverseSortKey("seniority"),
	)
	result := SortEntries(entries, ctrl)

	// Expected order: Engineering/3, Engineering/2, Engineering/1, Sales/2
	expectedDept := []string{"Engineering", "Engineering", "Engineering", "Sales"}
	expectedSeniority := []string{"3", "2", "1", "2"}

	for i, entry := range result {
		dept := getFirstAttributeValue(entry, "department")
		seniority := getFirstAttributeValue(entry, "seniority")
		if dept != expectedDept[i] || seniority != expectedSeniority[i] {
			t.Errorf("Entry %d: expected dept=%s seniority=%s, got dept=%s seniority=%s",
				i, expectedDept[i], expectedSeniority[i], dept, seniority)
		}
	}
}

// TestSortEntries_MissingAttributes tests handling of missing attributes.
func TestSortEntries_MissingAttributes(t *testing.T) {
	entries := []*storage.Entry{
		createTestEntry("uid=alice,dc=example,dc=com", "cn", "Alice"),
		createTestEntryMulti("uid=bob,dc=example,dc=com", map[string]string{"uid": "bob"}), // No cn attribute
		createTestEntry("uid=charlie,dc=example,dc=com", "cn", "Charlie"),
	}

	ctrl := NewSortControl(NewSortKey("cn"))
	result := SortEntries(entries, ctrl)

	// Entries with missing attribute should sort last
	if getFirstAttributeValue(result[0], "cn") != "Alice" {
		t.Errorf("Expected Alice first, got %s", getFirstAttributeValue(result[0], "cn"))
	}
	if getFirstAttributeValue(result[1], "cn") != "Charlie" {
		t.Errorf("Expected Charlie second, got %s", getFirstAttributeValue(result[1], "cn"))
	}
	if getFirstAttributeValue(result[2], "cn") != "" {
		t.Errorf("Expected entry without cn last, got %s", getFirstAttributeValue(result[2], "cn"))
	}
}

// TestSortEntries_MissingAttributesReverse tests missing attributes with reverse order.
func TestSortEntries_MissingAttributesReverse(t *testing.T) {
	entries := []*storage.Entry{
		createTestEntry("uid=alice,dc=example,dc=com", "cn", "Alice"),
		createTestEntryMulti("uid=bob,dc=example,dc=com", map[string]string{"uid": "bob"}), // No cn attribute
		createTestEntry("uid=charlie,dc=example,dc=com", "cn", "Charlie"),
	}

	ctrl := NewSortControl(NewReverseSortKey("cn"))
	result := SortEntries(entries, ctrl)

	// In reverse order: Charlie, Alice, then missing
	if getFirstAttributeValue(result[0], "cn") != "Charlie" {
		t.Errorf("Expected Charlie first, got %s", getFirstAttributeValue(result[0], "cn"))
	}
	if getFirstAttributeValue(result[1], "cn") != "Alice" {
		t.Errorf("Expected Alice second, got %s", getFirstAttributeValue(result[1], "cn"))
	}
	if getFirstAttributeValue(result[2], "cn") != "" {
		t.Errorf("Expected entry without cn last, got %s", getFirstAttributeValue(result[2], "cn"))
	}
}

// TestSortEntries_AllMissingAttributes tests when all entries are missing the sort attribute.
func TestSortEntries_AllMissingAttributes(t *testing.T) {
	entries := []*storage.Entry{
		createTestEntry("uid=alice,dc=example,dc=com", "uid", "alice"),
		createTestEntry("uid=bob,dc=example,dc=com", "uid", "bob"),
		createTestEntry("uid=charlie,dc=example,dc=com", "uid", "charlie"),
	}

	ctrl := NewSortControl(NewSortKey("nonexistent"))
	result := SortEntries(entries, ctrl)

	// Order should be preserved (stable sort)
	if len(result) != 3 {
		t.Errorf("Expected 3 entries, got %d", len(result))
	}
}

// TestSortEntries_EmptySlice tests sorting an empty slice.
func TestSortEntries_EmptySlice(t *testing.T) {
	entries := []*storage.Entry{}
	ctrl := NewSortControl(NewSortKey("cn"))
	result := SortEntries(entries, ctrl)

	if len(result) != 0 {
		t.Errorf("Expected empty result, got %d entries", len(result))
	}
}

// TestSortEntries_SingleEntry tests sorting a single entry.
func TestSortEntries_SingleEntry(t *testing.T) {
	entries := []*storage.Entry{
		createTestEntry("uid=alice,dc=example,dc=com", "cn", "Alice"),
	}

	ctrl := NewSortControl(NewSortKey("cn"))
	result := SortEntries(entries, ctrl)

	if len(result) != 1 {
		t.Errorf("Expected 1 entry, got %d", len(result))
	}
	if getFirstAttributeValue(result[0], "cn") != "Alice" {
		t.Errorf("Expected Alice, got %s", getFirstAttributeValue(result[0], "cn"))
	}
}

// TestSortEntries_NilControl tests sorting with nil control.
func TestSortEntries_NilControl(t *testing.T) {
	entries := []*storage.Entry{
		createTestEntry("uid=charlie,dc=example,dc=com", "cn", "Charlie"),
		createTestEntry("uid=alice,dc=example,dc=com", "cn", "Alice"),
	}

	result := SortEntries(entries, nil)

	// Order should be preserved
	if getFirstAttributeValue(result[0], "cn") != "Charlie" {
		t.Errorf("Expected Charlie first (unchanged), got %s", getFirstAttributeValue(result[0], "cn"))
	}
}

// TestSortEntries_EmptyKeys tests sorting with empty keys.
func TestSortEntries_EmptyKeys(t *testing.T) {
	entries := []*storage.Entry{
		createTestEntry("uid=charlie,dc=example,dc=com", "cn", "Charlie"),
		createTestEntry("uid=alice,dc=example,dc=com", "cn", "Alice"),
	}

	ctrl := &SortControl{Keys: []SortKey{}}
	result := SortEntries(entries, ctrl)

	// Order should be preserved
	if getFirstAttributeValue(result[0], "cn") != "Charlie" {
		t.Errorf("Expected Charlie first (unchanged), got %s", getFirstAttributeValue(result[0], "cn"))
	}
}

// TestSortEntries_CaseInsensitiveAttribute tests case-insensitive attribute lookup.
func TestSortEntries_CaseInsensitiveAttribute(t *testing.T) {
	entries := []*storage.Entry{
		createTestEntry("uid=charlie,dc=example,dc=com", "CN", "Charlie"),
		createTestEntry("uid=alice,dc=example,dc=com", "cn", "Alice"),
		createTestEntry("uid=bob,dc=example,dc=com", "Cn", "Bob"),
	}

	ctrl := NewSortControl(NewSortKey("cn"))
	result := SortEntries(entries, ctrl)

	expected := []string{"Alice", "Bob", "Charlie"}
	for i, entry := range result {
		cn := getFirstAttributeValue(entry, "cn")
		if cn != expected[i] {
			t.Errorf("Entry %d: expected cn=%s, got cn=%s", i, expected[i], cn)
		}
	}
}

// TestSortSearchEntries_SingleAttribute tests sorting SearchEntry by a single attribute.
func TestSortSearchEntries_SingleAttribute(t *testing.T) {
	entries := []*SearchEntry{
		createTestSearchEntry("uid=charlie,dc=example,dc=com", "cn", "Charlie"),
		createTestSearchEntry("uid=alice,dc=example,dc=com", "cn", "Alice"),
		createTestSearchEntry("uid=bob,dc=example,dc=com", "cn", "Bob"),
	}

	ctrl := NewSortControl(NewSortKey("cn"))
	result := SortSearchEntries(entries, ctrl)

	expected := []string{"Alice", "Bob", "Charlie"}
	for i, entry := range result {
		cn := getSearchEntryFirstValue(entry, "cn")
		if cn != expected[i] {
			t.Errorf("Entry %d: expected cn=%s, got cn=%s", i, expected[i], cn)
		}
	}
}

// TestSortSearchEntries_Reverse tests sorting SearchEntry in reverse order.
func TestSortSearchEntries_Reverse(t *testing.T) {
	entries := []*SearchEntry{
		createTestSearchEntry("uid=charlie,dc=example,dc=com", "cn", "Charlie"),
		createTestSearchEntry("uid=alice,dc=example,dc=com", "cn", "Alice"),
		createTestSearchEntry("uid=bob,dc=example,dc=com", "cn", "Bob"),
	}

	ctrl := NewSortControl(NewReverseSortKey("cn"))
	result := SortSearchEntries(entries, ctrl)

	expected := []string{"Charlie", "Bob", "Alice"}
	for i, entry := range result {
		cn := getSearchEntryFirstValue(entry, "cn")
		if cn != expected[i] {
			t.Errorf("Entry %d: expected cn=%s, got cn=%s", i, expected[i], cn)
		}
	}
}

// TestSortSearchEntries_MissingAttributes tests SearchEntry with missing attributes.
func TestSortSearchEntries_MissingAttributes(t *testing.T) {
	entries := []*SearchEntry{
		createTestSearchEntry("uid=alice,dc=example,dc=com", "cn", "Alice"),
		createTestSearchEntry("uid=bob,dc=example,dc=com", "uid", "bob"), // No cn
		createTestSearchEntry("uid=charlie,dc=example,dc=com", "cn", "Charlie"),
	}

	ctrl := NewSortControl(NewSortKey("cn"))
	result := SortSearchEntries(entries, ctrl)

	// Entries with missing attribute should sort last
	if getSearchEntryFirstValue(result[0], "cn") != "Alice" {
		t.Errorf("Expected Alice first, got %s", getSearchEntryFirstValue(result[0], "cn"))
	}
	if getSearchEntryFirstValue(result[1], "cn") != "Charlie" {
		t.Errorf("Expected Charlie second, got %s", getSearchEntryFirstValue(result[1], "cn"))
	}
	if getSearchEntryFirstValue(result[2], "cn") != "" {
		t.Errorf("Expected entry without cn last")
	}
}

// TestSortSearchEntries_NilControl tests SearchEntry sorting with nil control.
func TestSortSearchEntries_NilControl(t *testing.T) {
	entries := []*SearchEntry{
		createTestSearchEntry("uid=charlie,dc=example,dc=com", "cn", "Charlie"),
		createTestSearchEntry("uid=alice,dc=example,dc=com", "cn", "Alice"),
	}

	result := SortSearchEntries(entries, nil)

	// Order should be preserved
	if getSearchEntryFirstValue(result[0], "cn") != "Charlie" {
		t.Errorf("Expected Charlie first (unchanged), got %s", getSearchEntryFirstValue(result[0], "cn"))
	}
}

// TestValidateSortControl tests sort control validation.
func TestValidateSortControl(t *testing.T) {
	tests := []struct {
		name       string
		ctrl       *SortControl
		wantNil    bool
		wantResult int
	}{
		{
			name:    "nil control",
			ctrl:    nil,
			wantNil: true,
		},
		{
			name:    "valid control",
			ctrl:    NewSortControl(NewSortKey("cn")),
			wantNil: true,
		},
		{
			name:    "valid multi-key control",
			ctrl:    NewSortControl(NewSortKey("sn"), NewSortKey("givenName")),
			wantNil: true,
		},
		{
			name:       "empty attribute",
			ctrl:       NewSortControl(SortKey{Attribute: ""}),
			wantNil:    false,
			wantResult: SortResultNoSuchAttribute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateSortControl(tt.ctrl)
			if tt.wantNil {
				if result != nil {
					t.Errorf("Expected nil result, got %+v", result)
				}
			} else {
				if result == nil {
					t.Error("Expected non-nil result")
				} else if result.ResultCode != tt.wantResult {
					t.Errorf("Expected result code %d, got %d", tt.wantResult, result.ResultCode)
				}
			}
		})
	}
}

// TestNewSortControl tests SortControl creation.
func TestNewSortControl(t *testing.T) {
	ctrl := NewSortControl(
		NewSortKey("cn"),
		NewReverseSortKey("sn"),
	)

	if len(ctrl.Keys) != 2 {
		t.Errorf("Expected 2 keys, got %d", len(ctrl.Keys))
	}
	if ctrl.Keys[0].Attribute != "cn" {
		t.Errorf("Expected first key attribute 'cn', got '%s'", ctrl.Keys[0].Attribute)
	}
	if ctrl.Keys[0].Reverse {
		t.Error("Expected first key not reversed")
	}
	if ctrl.Keys[1].Attribute != "sn" {
		t.Errorf("Expected second key attribute 'sn', got '%s'", ctrl.Keys[1].Attribute)
	}
	if !ctrl.Keys[1].Reverse {
		t.Error("Expected second key reversed")
	}
}

// TestSortKey_OrderingRule tests SortKey with ordering rule.
func TestSortKey_OrderingRule(t *testing.T) {
	key := SortKey{
		Attribute:    "cn",
		OrderingRule: "2.5.13.3", // caseIgnoreOrderingMatch
		Reverse:      false,
	}

	if key.Attribute != "cn" {
		t.Errorf("Expected attribute 'cn', got '%s'", key.Attribute)
	}
	if key.OrderingRule != "2.5.13.3" {
		t.Errorf("Expected ordering rule '2.5.13.3', got '%s'", key.OrderingRule)
	}
}

// TestSortResultConstants tests that sort result constants are defined correctly.
func TestSortResultConstants(t *testing.T) {
	// Verify RFC 2891 result codes
	if SortResultSuccess != 0 {
		t.Errorf("SortResultSuccess should be 0, got %d", SortResultSuccess)
	}
	if SortResultOperationsError != 1 {
		t.Errorf("SortResultOperationsError should be 1, got %d", SortResultOperationsError)
	}
	if SortResultNoSuchAttribute != 16 {
		t.Errorf("SortResultNoSuchAttribute should be 16, got %d", SortResultNoSuchAttribute)
	}
}

// TestSortOIDConstants tests that OID constants are defined correctly.
func TestSortOIDConstants(t *testing.T) {
	if SortRequestOID != "1.2.840.113556.1.4.473" {
		t.Errorf("SortRequestOID incorrect: %s", SortRequestOID)
	}
	if SortResponseOID != "1.2.840.113556.1.4.474" {
		t.Errorf("SortResponseOID incorrect: %s", SortResponseOID)
	}
}

// TestSortEntries_StableSort tests that sorting is stable.
func TestSortEntries_StableSort(t *testing.T) {
	// Create entries with same sort key value but different DNs
	entries := []*storage.Entry{
		createTestEntryMulti("uid=1,dc=example,dc=com", map[string]string{"department": "Engineering", "uid": "1"}),
		createTestEntryMulti("uid=2,dc=example,dc=com", map[string]string{"department": "Engineering", "uid": "2"}),
		createTestEntryMulti("uid=3,dc=example,dc=com", map[string]string{"department": "Engineering", "uid": "3"}),
	}

	ctrl := NewSortControl(NewSortKey("department"))
	result := SortEntries(entries, ctrl)

	// Order should be preserved for equal elements (stable sort)
	expectedUIDs := []string{"1", "2", "3"}
	for i, entry := range result {
		uid := getFirstAttributeValue(entry, "uid")
		if uid != expectedUIDs[i] {
			t.Errorf("Entry %d: expected uid=%s, got uid=%s (stable sort violated)", i, expectedUIDs[i], uid)
		}
	}
}

// Helper functions for creating test entries

func createTestEntry(dn, attr, value string) *storage.Entry {
	entry := storage.NewEntry(dn)
	entry.SetStringAttribute(attr, value)
	return entry
}

func createTestEntryMulti(dn string, attrs map[string]string) *storage.Entry {
	entry := storage.NewEntry(dn)
	for k, v := range attrs {
		entry.SetStringAttribute(k, v)
	}
	return entry
}

func createTestSearchEntry(dn, attr, value string) *SearchEntry {
	return &SearchEntry{
		DN: dn,
		Attributes: []ldap.Attribute{
			{
				Type:   attr,
				Values: [][]byte{[]byte(value)},
			},
		},
	}
}
