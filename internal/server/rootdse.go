// Package server provides the LDAP server implementation.
package server

import (
	"sort"

	"github.com/oba-ldap/oba/internal/ldap"
)

// RootDSE represents the Root DSA-Specific Entry that provides information
// about the server's capabilities. This includes supported controls,
// extended operations, LDAP versions, and naming contexts.
//
// The Root DSE is accessed by searching with:
// - Base DN: "" (empty string)
// - Scope: base
// - Filter: (objectClass=*)
type RootDSE struct {
	// NamingContexts contains the base DNs served by this server.
	NamingContexts []string

	// SupportedLDAPVersion contains the LDAP protocol versions supported.
	SupportedLDAPVersion []string

	// SupportedControl contains the OIDs of supported LDAP controls.
	SupportedControl []string

	// SupportedExtension contains the OIDs of supported extended operations.
	SupportedExtension []string

	// SupportedFeatures contains the OIDs of supported LDAP features.
	SupportedFeatures []string

	// VendorName is the name of the LDAP server vendor.
	VendorName string

	// VendorVersion is the version of the LDAP server.
	VendorVersion string
}

// RootDSEConfig holds configuration for the Root DSE.
type RootDSEConfig struct {
	// NamingContexts contains the base DNs served by this server.
	NamingContexts []string

	// VendorName is the name of the LDAP server vendor.
	VendorName string

	// VendorVersion is the version of the LDAP server.
	VendorVersion string

	// ExtendedDispatcher provides the list of supported extended operations.
	ExtendedDispatcher *ExtendedDispatcher

	// SupportedControls contains the OIDs of supported LDAP controls.
	SupportedControls []string

	// SupportedFeatures contains the OIDs of supported LDAP features.
	SupportedFeatures []string
}

// DefaultVendorName is the default vendor name for the Oba LDAP server.
const DefaultVendorName = "Oba"

// DefaultVendorVersion is the default vendor version (can be overridden at build time).
const DefaultVendorVersion = "dev"

// NewRootDSEConfig creates a new RootDSEConfig with default settings.
func NewRootDSEConfig() *RootDSEConfig {
	return &RootDSEConfig{
		NamingContexts: []string{},
		VendorName:     DefaultVendorName,
		VendorVersion:  DefaultVendorVersion,
	}
}

// WithNamingContexts sets the naming contexts (base DNs).
func (c *RootDSEConfig) WithNamingContexts(contexts ...string) *RootDSEConfig {
	c.NamingContexts = contexts
	return c
}

// WithVendorName sets the vendor name.
func (c *RootDSEConfig) WithVendorName(name string) *RootDSEConfig {
	c.VendorName = name
	return c
}

// WithVendorVersion sets the vendor version.
func (c *RootDSEConfig) WithVendorVersion(version string) *RootDSEConfig {
	c.VendorVersion = version
	return c
}

// WithExtendedDispatcher sets the extended dispatcher for supported extensions.
func (c *RootDSEConfig) WithExtendedDispatcher(dispatcher *ExtendedDispatcher) *RootDSEConfig {
	c.ExtendedDispatcher = dispatcher
	return c
}

// WithSupportedControls sets the supported control OIDs.
func (c *RootDSEConfig) WithSupportedControls(controls ...string) *RootDSEConfig {
	c.SupportedControls = controls
	return c
}

// WithSupportedFeatures sets the supported feature OIDs.
func (c *RootDSEConfig) WithSupportedFeatures(features ...string) *RootDSEConfig {
	c.SupportedFeatures = features
	return c
}

// RootDSEProvider generates the Root DSE entry based on configuration.
type RootDSEProvider struct {
	config *RootDSEConfig
}

// NewRootDSEProvider creates a new RootDSEProvider with the given configuration.
func NewRootDSEProvider(config *RootDSEConfig) *RootDSEProvider {
	if config == nil {
		config = NewRootDSEConfig()
	}
	return &RootDSEProvider{
		config: config,
	}
}

// GetRootDSE returns the Root DSE entry.
func (p *RootDSEProvider) GetRootDSE() *RootDSE {
	dse := &RootDSE{
		NamingContexts:       make([]string, len(p.config.NamingContexts)),
		SupportedLDAPVersion: []string{"3"},
		VendorName:           p.config.VendorName,
		VendorVersion:        p.config.VendorVersion,
	}

	// Copy naming contexts
	copy(dse.NamingContexts, p.config.NamingContexts)

	// Get supported extensions from dispatcher
	if p.config.ExtendedDispatcher != nil {
		dse.SupportedExtension = p.config.ExtendedDispatcher.SupportedOIDs()
	}

	// Copy supported controls
	if len(p.config.SupportedControls) > 0 {
		dse.SupportedControl = make([]string, len(p.config.SupportedControls))
		copy(dse.SupportedControl, p.config.SupportedControls)
		sort.Strings(dse.SupportedControl)
	}

	// Copy supported features
	if len(p.config.SupportedFeatures) > 0 {
		dse.SupportedFeatures = make([]string, len(p.config.SupportedFeatures))
		copy(dse.SupportedFeatures, p.config.SupportedFeatures)
		sort.Strings(dse.SupportedFeatures)
	}

	return dse
}

// GetSearchEntry returns the Root DSE as a SearchEntry for search responses.
func (p *RootDSEProvider) GetSearchEntry() *SearchEntry {
	dse := p.GetRootDSE()

	entry := &SearchEntry{
		DN:         "",
		Attributes: []ldap.Attribute{},
	}

	// Add objectClass
	entry.Attributes = append(entry.Attributes, ldap.Attribute{
		Type:   "objectClass",
		Values: [][]byte{[]byte("top")},
	})

	// Add namingContexts
	if len(dse.NamingContexts) > 0 {
		values := make([][]byte, len(dse.NamingContexts))
		for i, nc := range dse.NamingContexts {
			values[i] = []byte(nc)
		}
		entry.Attributes = append(entry.Attributes, ldap.Attribute{
			Type:   "namingContexts",
			Values: values,
		})
	}

	// Add supportedLDAPVersion
	if len(dse.SupportedLDAPVersion) > 0 {
		values := make([][]byte, len(dse.SupportedLDAPVersion))
		for i, v := range dse.SupportedLDAPVersion {
			values[i] = []byte(v)
		}
		entry.Attributes = append(entry.Attributes, ldap.Attribute{
			Type:   "supportedLDAPVersion",
			Values: values,
		})
	}

	// Add supportedExtension
	if len(dse.SupportedExtension) > 0 {
		values := make([][]byte, len(dse.SupportedExtension))
		for i, oid := range dse.SupportedExtension {
			values[i] = []byte(oid)
		}
		entry.Attributes = append(entry.Attributes, ldap.Attribute{
			Type:   "supportedExtension",
			Values: values,
		})
	}

	// Add supportedControl
	if len(dse.SupportedControl) > 0 {
		values := make([][]byte, len(dse.SupportedControl))
		for i, oid := range dse.SupportedControl {
			values[i] = []byte(oid)
		}
		entry.Attributes = append(entry.Attributes, ldap.Attribute{
			Type:   "supportedControl",
			Values: values,
		})
	}

	// Add supportedFeatures
	if len(dse.SupportedFeatures) > 0 {
		values := make([][]byte, len(dse.SupportedFeatures))
		for i, oid := range dse.SupportedFeatures {
			values[i] = []byte(oid)
		}
		entry.Attributes = append(entry.Attributes, ldap.Attribute{
			Type:   "supportedFeatures",
			Values: values,
		})
	}

	// Add vendorName
	if dse.VendorName != "" {
		entry.Attributes = append(entry.Attributes, ldap.Attribute{
			Type:   "vendorName",
			Values: [][]byte{[]byte(dse.VendorName)},
		})
	}

	// Add vendorVersion
	if dse.VendorVersion != "" {
		entry.Attributes = append(entry.Attributes, ldap.Attribute{
			Type:   "vendorVersion",
			Values: [][]byte{[]byte(dse.VendorVersion)},
		})
	}

	return entry
}

// IsRootDSESearch checks if the search request is for the Root DSE.
// A Root DSE search has:
// - Empty base DN
// - Base scope
// - Filter that matches (typically objectClass=* or objectClass=top)
func IsRootDSESearch(req *ldap.SearchRequest) bool {
	// Check for empty base DN
	if req.BaseObject != "" {
		return false
	}

	// Check for base scope
	if req.Scope != ldap.ScopeBaseObject {
		return false
	}

	return true
}

// FilterRootDSEAttributes filters the Root DSE entry attributes based on
// the requested attributes in the search request.
func FilterRootDSEAttributes(entry *SearchEntry, requestedAttrs []string, typesOnly bool) *SearchEntry {
	if len(requestedAttrs) == 0 {
		// Return all attributes if none specified
		if typesOnly {
			return filterTypesOnly(entry)
		}
		return entry
	}

	// Check for special selectors
	hasAllUser := false
	hasAllOp := false
	specificAttrs := make(map[string]bool)

	for _, attr := range requestedAttrs {
		switch attr {
		case "*":
			hasAllUser = true
		case "+":
			hasAllOp = true
		default:
			specificAttrs[normalizeAttrName(attr)] = true
		}
	}

	// Root DSE attributes are considered operational
	// If "*" is requested without "+", return no attributes
	// If "+" is requested, return all operational attributes
	// If specific attributes are requested, return those

	result := &SearchEntry{
		DN:         entry.DN,
		Attributes: []ldap.Attribute{},
	}

	for _, attr := range entry.Attributes {
		attrName := normalizeAttrName(attr.Type)

		// Check if this attribute should be included
		include := false

		if hasAllOp {
			// "+" includes all operational attributes (Root DSE attrs are operational)
			include = true
		}

		if hasAllUser && isUserAttribute(attrName) {
			include = true
		}

		if specificAttrs[attrName] {
			include = true
		}

		if include {
			if typesOnly {
				result.Attributes = append(result.Attributes, ldap.Attribute{
					Type:   attr.Type,
					Values: nil,
				})
			} else {
				result.Attributes = append(result.Attributes, attr)
			}
		}
	}

	return result
}

// filterTypesOnly returns a copy of the entry with only attribute types (no values).
func filterTypesOnly(entry *SearchEntry) *SearchEntry {
	result := &SearchEntry{
		DN:         entry.DN,
		Attributes: make([]ldap.Attribute, len(entry.Attributes)),
	}

	for i, attr := range entry.Attributes {
		result.Attributes[i] = ldap.Attribute{
			Type:   attr.Type,
			Values: nil,
		}
	}

	return result
}

// normalizeAttrName normalizes an attribute name for comparison.
func normalizeAttrName(name string) string {
	// Simple lowercase normalization
	result := make([]byte, len(name))
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c >= 'A' && c <= 'Z' {
			result[i] = c + 32
		} else {
			result[i] = c
		}
	}
	return string(result)
}

// isUserAttribute checks if an attribute is a user attribute (not operational).
// For Root DSE, most attributes are operational, but objectClass is considered user.
func isUserAttribute(name string) bool {
	return name == "objectclass"
}
