package server

import (
	"testing"

	"github.com/oba-ldap/oba/internal/ldap"
)

// TestNewRootDSEConfig tests creating a new RootDSEConfig with defaults.
func TestNewRootDSEConfig(t *testing.T) {
	config := NewRootDSEConfig()

	if config == nil {
		t.Fatal("NewRootDSEConfig returned nil")
	}

	if config.VendorName != DefaultVendorName {
		t.Errorf("VendorName = %q, want %q", config.VendorName, DefaultVendorName)
	}

	if config.VendorVersion != DefaultVendorVersion {
		t.Errorf("VendorVersion = %q, want %q", config.VendorVersion, DefaultVendorVersion)
	}

	if len(config.NamingContexts) != 0 {
		t.Errorf("NamingContexts should be empty, got %v", config.NamingContexts)
	}
}

// TestRootDSEConfigChaining tests the builder pattern methods.
func TestRootDSEConfigChaining(t *testing.T) {
	config := NewRootDSEConfig().
		WithNamingContexts("dc=example,dc=com", "dc=test,dc=org").
		WithVendorName("TestVendor").
		WithVendorVersion("1.0.0").
		WithSupportedControls("1.2.3.4", "5.6.7.8").
		WithSupportedFeatures("1.1.1.1")

	if config.VendorName != "TestVendor" {
		t.Errorf("VendorName = %q, want %q", config.VendorName, "TestVendor")
	}

	if config.VendorVersion != "1.0.0" {
		t.Errorf("VendorVersion = %q, want %q", config.VendorVersion, "1.0.0")
	}

	if len(config.NamingContexts) != 2 {
		t.Errorf("NamingContexts length = %d, want 2", len(config.NamingContexts))
	}

	if config.NamingContexts[0] != "dc=example,dc=com" {
		t.Errorf("NamingContexts[0] = %q, want %q", config.NamingContexts[0], "dc=example,dc=com")
	}

	if len(config.SupportedControls) != 2 {
		t.Errorf("SupportedControls length = %d, want 2", len(config.SupportedControls))
	}

	if len(config.SupportedFeatures) != 1 {
		t.Errorf("SupportedFeatures length = %d, want 1", len(config.SupportedFeatures))
	}
}

// TestNewRootDSEProvider tests creating a new RootDSEProvider.
func TestNewRootDSEProvider(t *testing.T) {
	// Test with nil config
	provider := NewRootDSEProvider(nil)
	if provider == nil {
		t.Fatal("NewRootDSEProvider(nil) returned nil")
	}

	// Test with custom config
	config := NewRootDSEConfig().WithVendorName("Custom")
	provider = NewRootDSEProvider(config)
	if provider == nil {
		t.Fatal("NewRootDSEProvider returned nil")
	}
}

// TestRootDSEProvider_GetRootDSE tests getting the Root DSE.
func TestRootDSEProvider_GetRootDSE(t *testing.T) {
	config := NewRootDSEConfig().
		WithNamingContexts("dc=example,dc=com").
		WithVendorName("Oba").
		WithVendorVersion("1.0.0").
		WithSupportedControls("1.2.840.113556.1.4.319").
		WithSupportedFeatures("1.3.6.1.4.1.4203.1.5.1")

	provider := NewRootDSEProvider(config)
	dse := provider.GetRootDSE()

	if dse == nil {
		t.Fatal("GetRootDSE returned nil")
	}

	// Check naming contexts
	if len(dse.NamingContexts) != 1 {
		t.Errorf("NamingContexts length = %d, want 1", len(dse.NamingContexts))
	}
	if dse.NamingContexts[0] != "dc=example,dc=com" {
		t.Errorf("NamingContexts[0] = %q, want %q", dse.NamingContexts[0], "dc=example,dc=com")
	}

	// Check supported LDAP version
	if len(dse.SupportedLDAPVersion) != 1 {
		t.Errorf("SupportedLDAPVersion length = %d, want 1", len(dse.SupportedLDAPVersion))
	}
	if dse.SupportedLDAPVersion[0] != "3" {
		t.Errorf("SupportedLDAPVersion[0] = %q, want %q", dse.SupportedLDAPVersion[0], "3")
	}

	// Check vendor info
	if dse.VendorName != "Oba" {
		t.Errorf("VendorName = %q, want %q", dse.VendorName, "Oba")
	}
	if dse.VendorVersion != "1.0.0" {
		t.Errorf("VendorVersion = %q, want %q", dse.VendorVersion, "1.0.0")
	}

	// Check supported controls
	if len(dse.SupportedControl) != 1 {
		t.Errorf("SupportedControl length = %d, want 1", len(dse.SupportedControl))
	}

	// Check supported features
	if len(dse.SupportedFeatures) != 1 {
		t.Errorf("SupportedFeatures length = %d, want 1", len(dse.SupportedFeatures))
	}
}

// TestRootDSEProvider_GetRootDSE_WithExtendedDispatcher tests getting extensions from dispatcher.
func TestRootDSEProvider_GetRootDSE_WithExtendedDispatcher(t *testing.T) {
	dispatcher := NewExtendedDispatcher()

	// Register some handlers
	handler := NewWhoAmIHandler()
	if err := dispatcher.Register(handler); err != nil {
		t.Fatalf("Failed to register handler: %v", err)
	}

	config := NewRootDSEConfig().
		WithExtendedDispatcher(dispatcher)

	provider := NewRootDSEProvider(config)
	dse := provider.GetRootDSE()

	// Check that extensions are populated
	if len(dse.SupportedExtension) != 1 {
		t.Errorf("SupportedExtension length = %d, want 1", len(dse.SupportedExtension))
	}

	found := false
	for _, oid := range dse.SupportedExtension {
		if oid == WhoAmIOID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("WhoAmIOID not found in SupportedExtension")
	}
}

// TestRootDSEProvider_GetSearchEntry tests getting the Root DSE as a SearchEntry.
func TestRootDSEProvider_GetSearchEntry(t *testing.T) {
	config := NewRootDSEConfig().
		WithNamingContexts("dc=example,dc=com").
		WithVendorName("Oba").
		WithVendorVersion("1.0.0")

	provider := NewRootDSEProvider(config)
	entry := provider.GetSearchEntry()

	if entry == nil {
		t.Fatal("GetSearchEntry returned nil")
	}

	// Check DN is empty
	if entry.DN != "" {
		t.Errorf("DN = %q, want empty string", entry.DN)
	}

	// Check attributes
	attrMap := make(map[string][][]byte)
	for _, attr := range entry.Attributes {
		attrMap[attr.Type] = attr.Values
	}

	// Check objectClass
	if values, ok := attrMap["objectClass"]; !ok {
		t.Error("objectClass attribute missing")
	} else if len(values) != 1 || string(values[0]) != "top" {
		t.Errorf("objectClass = %v, want [top]", values)
	}

	// Check namingContexts
	if values, ok := attrMap["namingContexts"]; !ok {
		t.Error("namingContexts attribute missing")
	} else if len(values) != 1 || string(values[0]) != "dc=example,dc=com" {
		t.Errorf("namingContexts = %v, want [dc=example,dc=com]", values)
	}

	// Check supportedLDAPVersion
	if values, ok := attrMap["supportedLDAPVersion"]; !ok {
		t.Error("supportedLDAPVersion attribute missing")
	} else if len(values) != 1 || string(values[0]) != "3" {
		t.Errorf("supportedLDAPVersion = %v, want [3]", values)
	}

	// Check vendorName
	if values, ok := attrMap["vendorName"]; !ok {
		t.Error("vendorName attribute missing")
	} else if len(values) != 1 || string(values[0]) != "Oba" {
		t.Errorf("vendorName = %v, want [Oba]", values)
	}

	// Check vendorVersion
	if values, ok := attrMap["vendorVersion"]; !ok {
		t.Error("vendorVersion attribute missing")
	} else if len(values) != 1 || string(values[0]) != "1.0.0" {
		t.Errorf("vendorVersion = %v, want [1.0.0]", values)
	}
}

// TestRootDSEProvider_GetSearchEntry_WithExtensions tests SearchEntry with extensions.
func TestRootDSEProvider_GetSearchEntry_WithExtensions(t *testing.T) {
	dispatcher := NewExtendedDispatcher()
	handler := NewWhoAmIHandler()
	if err := dispatcher.Register(handler); err != nil {
		t.Fatalf("Failed to register handler: %v", err)
	}

	config := NewRootDSEConfig().
		WithExtendedDispatcher(dispatcher).
		WithSupportedControls("1.2.840.113556.1.4.319")

	provider := NewRootDSEProvider(config)
	entry := provider.GetSearchEntry()

	attrMap := make(map[string][][]byte)
	for _, attr := range entry.Attributes {
		attrMap[attr.Type] = attr.Values
	}

	// Check supportedExtension
	if values, ok := attrMap["supportedExtension"]; !ok {
		t.Error("supportedExtension attribute missing")
	} else {
		found := false
		for _, v := range values {
			if string(v) == WhoAmIOID {
				found = true
				break
			}
		}
		if !found {
			t.Error("WhoAmIOID not found in supportedExtension")
		}
	}

	// Check supportedControl
	if values, ok := attrMap["supportedControl"]; !ok {
		t.Error("supportedControl attribute missing")
	} else if len(values) != 1 || string(values[0]) != "1.2.840.113556.1.4.319" {
		t.Errorf("supportedControl = %v, want [1.2.840.113556.1.4.319]", values)
	}
}

// TestIsRootDSESearch tests the IsRootDSESearch function.
func TestIsRootDSESearch(t *testing.T) {
	tests := []struct {
		name       string
		baseObject string
		scope      ldap.SearchScope
		want       bool
	}{
		{
			name:       "Valid Root DSE search",
			baseObject: "",
			scope:      ldap.ScopeBaseObject,
			want:       true,
		},
		{
			name:       "Non-empty base DN",
			baseObject: "dc=example,dc=com",
			scope:      ldap.ScopeBaseObject,
			want:       false,
		},
		{
			name:       "Single level scope",
			baseObject: "",
			scope:      ldap.ScopeSingleLevel,
			want:       false,
		},
		{
			name:       "Subtree scope",
			baseObject: "",
			scope:      ldap.ScopeWholeSubtree,
			want:       false,
		},
		{
			name:       "Non-empty base with subtree",
			baseObject: "dc=example,dc=com",
			scope:      ldap.ScopeWholeSubtree,
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &ldap.SearchRequest{
				BaseObject: tt.baseObject,
				Scope:      tt.scope,
			}

			got := IsRootDSESearch(req)
			if got != tt.want {
				t.Errorf("IsRootDSESearch() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestFilterRootDSEAttributes tests attribute filtering.
func TestFilterRootDSEAttributes(t *testing.T) {
	// Create a sample entry
	entry := &SearchEntry{
		DN: "",
		Attributes: []ldap.Attribute{
			{Type: "objectClass", Values: [][]byte{[]byte("top")}},
			{Type: "namingContexts", Values: [][]byte{[]byte("dc=example,dc=com")}},
			{Type: "supportedLDAPVersion", Values: [][]byte{[]byte("3")}},
			{Type: "vendorName", Values: [][]byte{[]byte("Oba")}},
		},
	}

	tests := []struct {
		name           string
		requestedAttrs []string
		typesOnly      bool
		wantAttrs      []string
	}{
		{
			name:           "No attributes requested - return all",
			requestedAttrs: nil,
			typesOnly:      false,
			wantAttrs:      []string{"objectClass", "namingContexts", "supportedLDAPVersion", "vendorName"},
		},
		{
			name:           "Specific attribute requested",
			requestedAttrs: []string{"vendorName"},
			typesOnly:      false,
			wantAttrs:      []string{"vendorName"},
		},
		{
			name:           "Multiple specific attributes",
			requestedAttrs: []string{"vendorName", "namingContexts"},
			typesOnly:      false,
			wantAttrs:      []string{"namingContexts", "vendorName"},
		},
		{
			name:           "Case insensitive attribute",
			requestedAttrs: []string{"VENDORNAME"},
			typesOnly:      false,
			wantAttrs:      []string{"vendorName"},
		},
		{
			name:           "All operational attributes (+)",
			requestedAttrs: []string{"+"},
			typesOnly:      false,
			wantAttrs:      []string{"objectClass", "namingContexts", "supportedLDAPVersion", "vendorName"},
		},
		{
			name:           "All user attributes (*) - only objectClass",
			requestedAttrs: []string{"*"},
			typesOnly:      false,
			wantAttrs:      []string{"objectClass"},
		},
		{
			name:           "Both * and +",
			requestedAttrs: []string{"*", "+"},
			typesOnly:      false,
			wantAttrs:      []string{"objectClass", "namingContexts", "supportedLDAPVersion", "vendorName"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FilterRootDSEAttributes(entry, tt.requestedAttrs, tt.typesOnly)

			if len(result.Attributes) != len(tt.wantAttrs) {
				t.Errorf("got %d attributes, want %d", len(result.Attributes), len(tt.wantAttrs))
				return
			}

			// Check that all expected attributes are present
			gotAttrs := make(map[string]bool)
			for _, attr := range result.Attributes {
				gotAttrs[attr.Type] = true
			}

			for _, want := range tt.wantAttrs {
				if !gotAttrs[want] {
					t.Errorf("missing attribute %q", want)
				}
			}
		})
	}
}

// TestFilterRootDSEAttributes_TypesOnly tests typesOnly filtering.
func TestFilterRootDSEAttributes_TypesOnly(t *testing.T) {
	entry := &SearchEntry{
		DN: "",
		Attributes: []ldap.Attribute{
			{Type: "objectClass", Values: [][]byte{[]byte("top")}},
			{Type: "vendorName", Values: [][]byte{[]byte("Oba")}},
		},
	}

	result := FilterRootDSEAttributes(entry, nil, true)

	for _, attr := range result.Attributes {
		if attr.Values != nil {
			t.Errorf("attribute %q has values when typesOnly=true", attr.Type)
		}
	}
}

// TestRootDSEProvider_MultipleNamingContexts tests multiple naming contexts.
func TestRootDSEProvider_MultipleNamingContexts(t *testing.T) {
	config := NewRootDSEConfig().
		WithNamingContexts("dc=example,dc=com", "dc=test,dc=org", "o=company")

	provider := NewRootDSEProvider(config)
	dse := provider.GetRootDSE()

	if len(dse.NamingContexts) != 3 {
		t.Errorf("NamingContexts length = %d, want 3", len(dse.NamingContexts))
	}

	entry := provider.GetSearchEntry()
	attrMap := make(map[string][][]byte)
	for _, attr := range entry.Attributes {
		attrMap[attr.Type] = attr.Values
	}

	if values, ok := attrMap["namingContexts"]; !ok {
		t.Error("namingContexts attribute missing")
	} else if len(values) != 3 {
		t.Errorf("namingContexts has %d values, want 3", len(values))
	}
}

// TestRootDSEProvider_EmptyConfig tests provider with empty config.
func TestRootDSEProvider_EmptyConfig(t *testing.T) {
	config := NewRootDSEConfig()
	provider := NewRootDSEProvider(config)
	dse := provider.GetRootDSE()

	// Should still have LDAP version
	if len(dse.SupportedLDAPVersion) != 1 || dse.SupportedLDAPVersion[0] != "3" {
		t.Error("SupportedLDAPVersion should always be [3]")
	}

	// Should have default vendor info
	if dse.VendorName != DefaultVendorName {
		t.Errorf("VendorName = %q, want %q", dse.VendorName, DefaultVendorName)
	}
}

// TestRootDSEProvider_SortedControls tests that controls are sorted.
func TestRootDSEProvider_SortedControls(t *testing.T) {
	config := NewRootDSEConfig().
		WithSupportedControls("9.9.9.9", "1.1.1.1", "5.5.5.5")

	provider := NewRootDSEProvider(config)
	dse := provider.GetRootDSE()

	if len(dse.SupportedControl) != 3 {
		t.Fatalf("SupportedControl length = %d, want 3", len(dse.SupportedControl))
	}

	// Check sorted order
	if dse.SupportedControl[0] != "1.1.1.1" {
		t.Errorf("SupportedControl[0] = %q, want %q", dse.SupportedControl[0], "1.1.1.1")
	}
	if dse.SupportedControl[1] != "5.5.5.5" {
		t.Errorf("SupportedControl[1] = %q, want %q", dse.SupportedControl[1], "5.5.5.5")
	}
	if dse.SupportedControl[2] != "9.9.9.9" {
		t.Errorf("SupportedControl[2] = %q, want %q", dse.SupportedControl[2], "9.9.9.9")
	}
}

// TestNormalizeAttrName tests attribute name normalization.
func TestNormalizeAttrName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"objectClass", "objectclass"},
		{"OBJECTCLASS", "objectclass"},
		{"namingContexts", "namingcontexts"},
		{"vendorName", "vendorname"},
		{"already-lower", "already-lower"},
		{"MixedCase123", "mixedcase123"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeAttrName(tt.input)
			if got != tt.want {
				t.Errorf("normalizeAttrName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestDefaultConstants tests the default constants.
func TestDefaultConstants(t *testing.T) {
	if DefaultVendorName != "Oba" {
		t.Errorf("DefaultVendorName = %q, want %q", DefaultVendorName, "Oba")
	}

	if DefaultVendorVersion != "dev" {
		t.Errorf("DefaultVendorVersion = %q, want %q", DefaultVendorVersion, "dev")
	}
}

// TestRootDSEProvider_GetSearchEntry_NoNamingContexts tests entry without naming contexts.
func TestRootDSEProvider_GetSearchEntry_NoNamingContexts(t *testing.T) {
	config := NewRootDSEConfig()
	provider := NewRootDSEProvider(config)
	entry := provider.GetSearchEntry()

	// Check that namingContexts is not present when empty
	for _, attr := range entry.Attributes {
		if attr.Type == "namingContexts" {
			t.Error("namingContexts should not be present when empty")
		}
	}
}

// TestRootDSEProvider_GetSearchEntry_NoExtensions tests entry without extensions.
func TestRootDSEProvider_GetSearchEntry_NoExtensions(t *testing.T) {
	config := NewRootDSEConfig()
	provider := NewRootDSEProvider(config)
	entry := provider.GetSearchEntry()

	// Check that supportedExtension is not present when empty
	for _, attr := range entry.Attributes {
		if attr.Type == "supportedExtension" {
			t.Error("supportedExtension should not be present when empty")
		}
	}
}

// TestRootDSEProvider_GetSearchEntry_NoControls tests entry without controls.
func TestRootDSEProvider_GetSearchEntry_NoControls(t *testing.T) {
	config := NewRootDSEConfig()
	provider := NewRootDSEProvider(config)
	entry := provider.GetSearchEntry()

	// Check that supportedControl is not present when empty
	for _, attr := range entry.Attributes {
		if attr.Type == "supportedControl" {
			t.Error("supportedControl should not be present when empty")
		}
	}
}

// TestRootDSEProvider_GetSearchEntry_NoFeatures tests entry without features.
func TestRootDSEProvider_GetSearchEntry_NoFeatures(t *testing.T) {
	config := NewRootDSEConfig()
	provider := NewRootDSEProvider(config)
	entry := provider.GetSearchEntry()

	// Check that supportedFeatures is not present when empty
	for _, attr := range entry.Attributes {
		if attr.Type == "supportedFeatures" {
			t.Error("supportedFeatures should not be present when empty")
		}
	}
}

// TestRootDSE_ImmutableNamingContexts tests that naming contexts are copied.
func TestRootDSE_ImmutableNamingContexts(t *testing.T) {
	contexts := []string{"dc=example,dc=com"}
	config := NewRootDSEConfig().WithNamingContexts(contexts...)

	provider := NewRootDSEProvider(config)
	dse := provider.GetRootDSE()

	// Modify the original slice
	contexts[0] = "dc=modified,dc=com"

	// The DSE should not be affected
	if dse.NamingContexts[0] == "dc=modified,dc=com" {
		t.Error("NamingContexts should be a copy, not a reference")
	}
}

// TestFilterRootDSEAttributes_EmptyEntry tests filtering an empty entry.
func TestFilterRootDSEAttributes_EmptyEntry(t *testing.T) {
	entry := &SearchEntry{
		DN:         "",
		Attributes: []ldap.Attribute{},
	}

	result := FilterRootDSEAttributes(entry, []string{"vendorName"}, false)

	if len(result.Attributes) != 0 {
		t.Errorf("expected 0 attributes, got %d", len(result.Attributes))
	}
}

// TestFilterRootDSEAttributes_NonExistentAttribute tests requesting non-existent attribute.
func TestFilterRootDSEAttributes_NonExistentAttribute(t *testing.T) {
	entry := &SearchEntry{
		DN: "",
		Attributes: []ldap.Attribute{
			{Type: "objectClass", Values: [][]byte{[]byte("top")}},
		},
	}

	result := FilterRootDSEAttributes(entry, []string{"nonExistent"}, false)

	if len(result.Attributes) != 0 {
		t.Errorf("expected 0 attributes, got %d", len(result.Attributes))
	}
}
