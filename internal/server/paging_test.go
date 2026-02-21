// Package server provides the LDAP server implementation.
package server

import (
	"bytes"
	"testing"
	"time"

	"github.com/KilimcininKorOglu/oba/internal/ldap"
)

// TestParsePagedResultsControl tests parsing of PagedResultsControl.
func TestParsePagedResultsControl(t *testing.T) {
	tests := []struct {
		name       string
		control    ldap.Control
		wantSize   int32
		wantCookie []byte
		wantCrit   bool
		wantNil    bool
		wantErr    bool
	}{
		{
			name: "valid control with size and empty cookie",
			control: ldap.Control{
				OID:         PagedResultsOID,
				Criticality: true,
				Value:       []byte{0x30, 0x05, 0x02, 0x01, 0x64, 0x04, 0x00}, // SEQUENCE { INTEGER 100, OCTET STRING "" }
			},
			wantSize:   100,
			wantCookie: []byte{},
			wantCrit:   true,
		},
		{
			name: "valid control with size and cookie",
			control: ldap.Control{
				OID:         PagedResultsOID,
				Criticality: false,
				Value:       []byte{0x30, 0x09, 0x02, 0x01, 0x32, 0x04, 0x04, 0x01, 0x02, 0x03, 0x04}, // SEQUENCE { INTEGER 50, OCTET STRING "\x01\x02\x03\x04" }
			},
			wantSize:   50,
			wantCookie: []byte{0x01, 0x02, 0x03, 0x04},
			wantCrit:   false,
		},
		{
			name: "control with empty value",
			control: ldap.Control{
				OID:         PagedResultsOID,
				Criticality: false,
				Value:       nil,
			},
			wantSize:   0,
			wantCookie: nil,
			wantCrit:   false,
		},
		{
			name: "different OID - not paged results",
			control: ldap.Control{
				OID:         "1.2.3.4.5",
				Criticality: false,
				Value:       []byte{0x30, 0x05, 0x02, 0x01, 0x64, 0x04, 0x00},
			},
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prc, err := ParsePagedResultsControl(tt.control)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.wantNil {
				if prc != nil {
					t.Error("expected nil, got non-nil")
				}
				return
			}

			if prc == nil {
				t.Error("expected non-nil, got nil")
				return
			}

			if prc.Size != tt.wantSize {
				t.Errorf("Size = %d, want %d", prc.Size, tt.wantSize)
			}

			if !bytes.Equal(prc.Cookie, tt.wantCookie) {
				t.Errorf("Cookie = %v, want %v", prc.Cookie, tt.wantCookie)
			}

			if prc.Criticality != tt.wantCrit {
				t.Errorf("Criticality = %v, want %v", prc.Criticality, tt.wantCrit)
			}
		})
	}
}

// TestPagedResultsControl_Encode tests encoding of PagedResultsControl.
func TestPagedResultsControl_Encode(t *testing.T) {
	tests := []struct {
		name    string
		control *PagedResultsControl
		wantErr bool
	}{
		{
			name: "encode with size and empty cookie",
			control: &PagedResultsControl{
				Size:        100,
				Cookie:      []byte{},
				Criticality: true,
			},
		},
		{
			name: "encode with size and cookie",
			control: &PagedResultsControl{
				Size:        50,
				Cookie:      []byte{0x01, 0x02, 0x03, 0x04},
				Criticality: false,
			},
		},
		{
			name: "encode with zero size",
			control: &PagedResultsControl{
				Size:        0,
				Cookie:      nil,
				Criticality: false,
			},
		},
		{
			name: "encode with large size",
			control: &PagedResultsControl{
				Size:        1000000,
				Cookie:      []byte("test-cookie-data"),
				Criticality: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := tt.control.Encode()

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Verify we can parse it back
			ctrl := ldap.Control{
				OID:         PagedResultsOID,
				Criticality: tt.control.Criticality,
				Value:       encoded,
			}

			parsed, err := ParsePagedResultsControl(ctrl)
			if err != nil {
				t.Errorf("failed to parse encoded control: %v", err)
				return
			}

			if parsed.Size != tt.control.Size {
				t.Errorf("Size = %d, want %d", parsed.Size, tt.control.Size)
			}

			if !bytes.Equal(parsed.Cookie, tt.control.Cookie) {
				t.Errorf("Cookie = %v, want %v", parsed.Cookie, tt.control.Cookie)
			}
		})
	}
}

// TestPagedResultsControl_ToLDAPControl tests conversion to ldap.Control.
func TestPagedResultsControl_ToLDAPControl(t *testing.T) {
	prc := &PagedResultsControl{
		Size:        100,
		Cookie:      []byte("test-cookie"),
		Criticality: true,
	}

	ctrl, err := prc.ToLDAPControl()
	if err != nil {
		t.Fatalf("ToLDAPControl failed: %v", err)
	}

	if ctrl.OID != PagedResultsOID {
		t.Errorf("OID = %s, want %s", ctrl.OID, PagedResultsOID)
	}

	if ctrl.Criticality != true {
		t.Error("Criticality = false, want true")
	}

	if len(ctrl.Value) == 0 {
		t.Error("Value is empty")
	}
}

// TestFindPagedResultsControl tests finding PagedResultsControl in a slice.
func TestFindPagedResultsControl(t *testing.T) {
	tests := []struct {
		name     string
		controls []ldap.Control
		wantNil  bool
		wantSize int32
	}{
		{
			name:     "empty controls",
			controls: nil,
			wantNil:  true,
		},
		{
			name: "no paged results control",
			controls: []ldap.Control{
				{OID: "1.2.3.4.5", Value: []byte{0x01}},
				{OID: "2.3.4.5.6", Value: []byte{0x02}},
			},
			wantNil: true,
		},
		{
			name: "paged results control present",
			controls: []ldap.Control{
				{OID: "1.2.3.4.5", Value: []byte{0x01}},
				{OID: PagedResultsOID, Value: []byte{0x30, 0x05, 0x02, 0x01, 0x64, 0x04, 0x00}},
			},
			wantNil:  false,
			wantSize: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prc, err := FindPagedResultsControl(tt.controls)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.wantNil {
				if prc != nil {
					t.Error("expected nil, got non-nil")
				}
				return
			}

			if prc == nil {
				t.Error("expected non-nil, got nil")
				return
			}

			if prc.Size != tt.wantSize {
				t.Errorf("Size = %d, want %d", prc.Size, tt.wantSize)
			}
		})
	}
}

// TestPagedSearchManager_CreateState tests creating paged search states.
func TestPagedSearchManager_CreateState(t *testing.T) {
	mgr := NewPagedSearchManager(nil)

	results := []*SearchEntry{
		{DN: "uid=alice,ou=users,dc=example,dc=com"},
		{DN: "uid=bob,ou=users,dc=example,dc=com"},
		{DN: "uid=charlie,ou=users,dc=example,dc=com"},
	}

	cookieID, err := mgr.CreateState(
		"ou=users,dc=example,dc=com",
		ldap.ScopeWholeSubtree,
		"(objectClass=*)",
		[]string{"cn", "mail"},
		false,
		results,
	)

	if err != nil {
		t.Fatalf("CreateState failed: %v", err)
	}

	if cookieID == "" {
		t.Error("cookieID is empty")
	}

	// Verify state was created
	state, err := mgr.GetState(cookieID)
	if err != nil {
		t.Fatalf("GetState failed: %v", err)
	}

	if state.BaseDN != "ou=users,dc=example,dc=com" {
		t.Errorf("BaseDN = %s, want ou=users,dc=example,dc=com", state.BaseDN)
	}

	if state.TotalCount != 3 {
		t.Errorf("TotalCount = %d, want 3", state.TotalCount)
	}

	if state.Position != 0 {
		t.Errorf("Position = %d, want 0", state.Position)
	}
}

// TestPagedSearchManager_GetState tests retrieving paged search states.
func TestPagedSearchManager_GetState(t *testing.T) {
	mgr := NewPagedSearchManager(nil)

	// Test getting non-existent state
	_, err := mgr.GetState("non-existent")
	if err != ErrInvalidCookie {
		t.Errorf("expected ErrInvalidCookie, got %v", err)
	}

	// Create a state
	results := []*SearchEntry{{DN: "uid=test,dc=example,dc=com"}}
	cookieID, _ := mgr.CreateState("dc=example,dc=com", ldap.ScopeBaseObject, "(cn=*)", nil, false, results)

	// Get the state
	state, err := mgr.GetState(cookieID)
	if err != nil {
		t.Fatalf("GetState failed: %v", err)
	}

	if state.ID != cookieID {
		t.Errorf("ID = %s, want %s", state.ID, cookieID)
	}
}

// TestPagedSearchManager_UpdatePosition tests updating position in paged search.
func TestPagedSearchManager_UpdatePosition(t *testing.T) {
	mgr := NewPagedSearchManager(nil)

	results := make([]*SearchEntry, 100)
	for i := 0; i < 100; i++ {
		results[i] = &SearchEntry{DN: "uid=user" + string(rune('0'+i%10)) + ",dc=example,dc=com"}
	}

	cookieID, _ := mgr.CreateState("dc=example,dc=com", ldap.ScopeWholeSubtree, "(objectClass=*)", nil, false, results)

	// Update position
	err := mgr.UpdatePosition(cookieID, 50)
	if err != nil {
		t.Fatalf("UpdatePosition failed: %v", err)
	}

	// Verify position was updated
	state, _ := mgr.GetState(cookieID)
	if state.Position != 50 {
		t.Errorf("Position = %d, want 50", state.Position)
	}

	// Test updating non-existent state
	err = mgr.UpdatePosition("non-existent", 10)
	if err != ErrInvalidCookie {
		t.Errorf("expected ErrInvalidCookie, got %v", err)
	}
}

// TestPagedSearchManager_DeleteState tests deleting paged search states.
func TestPagedSearchManager_DeleteState(t *testing.T) {
	mgr := NewPagedSearchManager(nil)

	results := []*SearchEntry{{DN: "uid=test,dc=example,dc=com"}}
	cookieID, _ := mgr.CreateState("dc=example,dc=com", ldap.ScopeBaseObject, "(cn=*)", nil, false, results)

	// Delete the state
	mgr.DeleteState(cookieID)

	// Verify state was deleted
	_, err := mgr.GetState(cookieID)
	if err != ErrInvalidCookie {
		t.Errorf("expected ErrInvalidCookie, got %v", err)
	}
}

// TestPagedSearchManager_StateExpiration tests state expiration.
func TestPagedSearchManager_StateExpiration(t *testing.T) {
	config := &PagedSearchManagerConfig{
		StateTimeout: 50 * time.Millisecond,
		MaxStates:    100,
	}
	mgr := NewPagedSearchManager(config)

	results := []*SearchEntry{{DN: "uid=test,dc=example,dc=com"}}
	cookieID, _ := mgr.CreateState("dc=example,dc=com", ldap.ScopeBaseObject, "(cn=*)", nil, false, results)

	// State should be accessible immediately
	_, err := mgr.GetState(cookieID)
	if err != nil {
		t.Fatalf("GetState failed: %v", err)
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// State should be expired
	_, err = mgr.GetState(cookieID)
	if err != ErrInvalidCookie {
		t.Errorf("expected ErrInvalidCookie after expiration, got %v", err)
	}
}

// TestPagedSearchManager_MaxStates tests maximum states limit.
func TestPagedSearchManager_MaxStates(t *testing.T) {
	config := &PagedSearchManagerConfig{
		StateTimeout: 5 * time.Minute,
		MaxStates:    5,
	}
	mgr := NewPagedSearchManager(config)

	results := []*SearchEntry{{DN: "uid=test,dc=example,dc=com"}}

	// Create max states
	for i := 0; i < 5; i++ {
		_, err := mgr.CreateState("dc=example,dc=com", ldap.ScopeBaseObject, "(cn=*)", nil, false, results)
		if err != nil {
			t.Fatalf("CreateState %d failed: %v", i, err)
		}
	}

	// Next create should fail
	_, err := mgr.CreateState("dc=example,dc=com", ldap.ScopeBaseObject, "(cn=*)", nil, false, results)
	if err == nil {
		t.Error("expected error when exceeding max states, got nil")
	}
}

// TestPagedSearchState_ValidateSearchParameters tests parameter validation.
func TestPagedSearchState_ValidateSearchParameters(t *testing.T) {
	state := &PagedSearchState{
		BaseDN:    "ou=users,dc=example,dc=com",
		Scope:     ldap.ScopeWholeSubtree,
		FilterStr: "(objectClass=*)",
		TypesOnly: false,
	}

	tests := []struct {
		name      string
		baseDN    string
		scope     ldap.SearchScope
		filterStr string
		typesOnly bool
		wantErr   bool
	}{
		{
			name:      "matching parameters",
			baseDN:    "ou=users,dc=example,dc=com",
			scope:     ldap.ScopeWholeSubtree,
			filterStr: "(objectClass=*)",
			typesOnly: false,
			wantErr:   false,
		},
		{
			name:      "different baseDN",
			baseDN:    "ou=groups,dc=example,dc=com",
			scope:     ldap.ScopeWholeSubtree,
			filterStr: "(objectClass=*)",
			typesOnly: false,
			wantErr:   true,
		},
		{
			name:      "different scope",
			baseDN:    "ou=users,dc=example,dc=com",
			scope:     ldap.ScopeSingleLevel,
			filterStr: "(objectClass=*)",
			typesOnly: false,
			wantErr:   true,
		},
		{
			name:      "different filter",
			baseDN:    "ou=users,dc=example,dc=com",
			scope:     ldap.ScopeWholeSubtree,
			filterStr: "(cn=*)",
			typesOnly: false,
			wantErr:   true,
		},
		{
			name:      "different typesOnly",
			baseDN:    "ou=users,dc=example,dc=com",
			scope:     ldap.ScopeWholeSubtree,
			filterStr: "(objectClass=*)",
			typesOnly: true,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := state.ValidateSearchParameters(tt.baseDN, tt.scope, tt.filterStr, nil, tt.typesOnly)
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestPagedSearchState_GetPage tests getting pages of results.
func TestPagedSearchState_GetPage(t *testing.T) {
	// Create 25 test entries
	results := make([]*SearchEntry, 25)
	for i := 0; i < 25; i++ {
		results[i] = &SearchEntry{DN: "uid=user" + string(rune('a'+i)) + ",dc=example,dc=com"}
	}

	state := &PagedSearchState{
		Results:    results,
		TotalCount: 25,
		Position:   0,
	}

	tests := []struct {
		name        string
		position    int
		pageSize    int
		wantCount   int
		wantHasMore bool
	}{
		{
			name:        "first page of 10",
			position:    0,
			pageSize:    10,
			wantCount:   10,
			wantHasMore: true,
		},
		{
			name:        "second page of 10",
			position:    10,
			pageSize:    10,
			wantCount:   10,
			wantHasMore: true,
		},
		{
			name:        "last page of 10",
			position:    20,
			pageSize:    10,
			wantCount:   5,
			wantHasMore: false,
		},
		{
			name:        "page size larger than remaining",
			position:    20,
			pageSize:    100,
			wantCount:   5,
			wantHasMore: false,
		},
		{
			name:        "position at end",
			position:    25,
			pageSize:    10,
			wantCount:   0,
			wantHasMore: false,
		},
		{
			name:        "position beyond end",
			position:    30,
			pageSize:    10,
			wantCount:   0,
			wantHasMore: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state.Position = tt.position
			entries, hasMore := state.GetPage(tt.pageSize)

			if len(entries) != tt.wantCount {
				t.Errorf("got %d entries, want %d", len(entries), tt.wantCount)
			}

			if hasMore != tt.wantHasMore {
				t.Errorf("hasMore = %v, want %v", hasMore, tt.wantHasMore)
			}
		})
	}
}

// TestEncodeCookie tests cookie encoding.
func TestEncodeCookie(t *testing.T) {
	cookieID := "test-cookie-id-12345"
	encoded := EncodeCookie(cookieID)

	// Verify format
	if encoded[0] != 1 {
		t.Errorf("version = %d, want 1", encoded[0])
	}

	// Decode and verify
	decoded, err := DecodeCookie(encoded)
	if err != nil {
		t.Fatalf("DecodeCookie failed: %v", err)
	}

	if decoded != cookieID {
		t.Errorf("decoded = %s, want %s", decoded, cookieID)
	}
}

// TestDecodeCookie tests cookie decoding.
func TestDecodeCookie(t *testing.T) {
	tests := []struct {
		name    string
		cookie  []byte
		want    string
		wantErr bool
	}{
		{
			name:    "valid cookie",
			cookie:  []byte{0x01, 0x00, 0x04, 't', 'e', 's', 't'},
			want:    "test",
			wantErr: false,
		},
		{
			name:    "empty cookie",
			cookie:  []byte{},
			wantErr: true,
		},
		{
			name:    "too short cookie",
			cookie:  []byte{0x01, 0x00},
			wantErr: true,
		},
		{
			name:    "invalid version",
			cookie:  []byte{0x02, 0x00, 0x04, 't', 'e', 's', 't'},
			wantErr: true,
		},
		{
			name:    "truncated data",
			cookie:  []byte{0x01, 0x00, 0x10, 't', 'e', 's', 't'},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decoded, err := DecodeCookie(tt.cookie)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if decoded != tt.want {
				t.Errorf("decoded = %s, want %s", decoded, tt.want)
			}
		})
	}
}

// TestFilterToString tests filter to string conversion.
func TestFilterToString(t *testing.T) {
	tests := []struct {
		name   string
		filter *ldap.SearchFilter
		want   string
	}{
		{
			name:   "nil filter",
			filter: nil,
			want:   "(objectClass=*)",
		},
		{
			name: "equality filter",
			filter: &ldap.SearchFilter{
				Type:      ldap.FilterTagEquality,
				Attribute: "uid",
				Value:     []byte("alice"),
			},
			want: "(uid=alice)",
		},
		{
			name: "presence filter",
			filter: &ldap.SearchFilter{
				Type:      ldap.FilterTagPresent,
				Attribute: "mail",
			},
			want: "(mail=*)",
		},
		{
			name: "AND filter",
			filter: &ldap.SearchFilter{
				Type: ldap.FilterTagAnd,
				Children: []*ldap.SearchFilter{
					{Type: ldap.FilterTagEquality, Attribute: "uid", Value: []byte("alice")},
					{Type: ldap.FilterTagPresent, Attribute: "mail"},
				},
			},
			want: "(&(uid=alice)(mail=*))",
		},
		{
			name: "OR filter",
			filter: &ldap.SearchFilter{
				Type: ldap.FilterTagOr,
				Children: []*ldap.SearchFilter{
					{Type: ldap.FilterTagEquality, Attribute: "uid", Value: []byte("alice")},
					{Type: ldap.FilterTagEquality, Attribute: "uid", Value: []byte("bob")},
				},
			},
			want: "(|(uid=alice)(uid=bob))",
		},
		{
			name: "NOT filter",
			filter: &ldap.SearchFilter{
				Type: ldap.FilterTagNot,
				Child: &ldap.SearchFilter{
					Type:      ldap.FilterTagEquality,
					Attribute: "uid",
					Value:     []byte("alice"),
				},
			},
			want: "(!(uid=alice))",
		},
		{
			name: "substring filter",
			filter: &ldap.SearchFilter{
				Type:      ldap.FilterTagSubstrings,
				Attribute: "cn",
				Substrings: &ldap.SubstringComponents{
					Initial: []byte("Al"),
					Any:     [][]byte{[]byte("ic")},
					Final:   []byte("e"),
				},
			},
			want: "(cn=Al*ic*e)",
		},
		{
			name: "greater or equal filter",
			filter: &ldap.SearchFilter{
				Type:      ldap.FilterTagGreaterOrEqual,
				Attribute: "age",
				Value:     []byte("18"),
			},
			want: "(age>=18)",
		},
		{
			name: "less or equal filter",
			filter: &ldap.SearchFilter{
				Type:      ldap.FilterTagLessOrEqual,
				Attribute: "age",
				Value:     []byte("65"),
			},
			want: "(age<=65)",
		},
		{
			name: "approx match filter",
			filter: &ldap.SearchFilter{
				Type:      ldap.FilterTagApproxMatch,
				Attribute: "cn",
				Value:     []byte("John"),
			},
			want: "(cn~=John)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterToString(tt.filter)
			if got != tt.want {
				t.Errorf("FilterToString() = %s, want %s", got, tt.want)
			}
		})
	}
}

// TestPagedSearchRoundTrip tests a complete paging round trip.
func TestPagedSearchRoundTrip(t *testing.T) {
	mgr := NewPagedSearchManager(nil)

	// Create 100 test entries
	results := make([]*SearchEntry, 100)
	for i := 0; i < 100; i++ {
		results[i] = &SearchEntry{DN: "uid=user" + string(rune('0'+i/10)) + string(rune('0'+i%10)) + ",dc=example,dc=com"}
	}

	// Create initial state
	cookieID, err := mgr.CreateState(
		"dc=example,dc=com",
		ldap.ScopeWholeSubtree,
		"(objectClass=*)",
		[]string{"cn"},
		false,
		results,
	)
	if err != nil {
		t.Fatalf("CreateState failed: %v", err)
	}

	// Encode cookie
	cookie := EncodeCookie(cookieID)

	// Simulate paging through results
	pageSize := 25
	totalRetrieved := 0
	pageCount := 0

	for {
		// Decode cookie
		decodedID, err := DecodeCookie(cookie)
		if err != nil {
			t.Fatalf("DecodeCookie failed: %v", err)
		}

		// Get state
		state, err := mgr.GetState(decodedID)
		if err != nil {
			t.Fatalf("GetState failed: %v", err)
		}

		// Get page
		entries, hasMore := state.GetPage(pageSize)
		totalRetrieved += len(entries)
		pageCount++

		// Update position
		newPosition := state.Position + len(entries)
		if err := mgr.UpdatePosition(decodedID, newPosition); err != nil {
			t.Fatalf("UpdatePosition failed: %v", err)
		}

		if !hasMore {
			break
		}

		// Re-encode cookie for next iteration
		cookie = EncodeCookie(decodedID)
	}

	if totalRetrieved != 100 {
		t.Errorf("totalRetrieved = %d, want 100", totalRetrieved)
	}

	if pageCount != 4 {
		t.Errorf("pageCount = %d, want 4", pageCount)
	}
}
