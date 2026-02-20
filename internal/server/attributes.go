// Package server provides the LDAP server implementation.
package server

import (
	"strings"

	"github.com/oba-ldap/oba/internal/storage"
)

// AttributeSelector handles attribute selection for search results.
// It implements the LDAP attribute selection rules per RFC 4511.
type AttributeSelector struct {
	// requestedAttrs is the list of requested attributes from the search request.
	requestedAttrs []string
	// hasAllUser indicates if "*" was requested (all user attributes).
	hasAllUser bool
	// hasAllOp indicates if "+" was requested (all operational attributes).
	hasAllOp bool
	// specificAttrs contains specifically requested attribute names.
	specificAttrs []string
}

// NewAttributeSelector creates a new AttributeSelector from the requested attributes.
func NewAttributeSelector(requestedAttrs []string) *AttributeSelector {
	selector := &AttributeSelector{
		requestedAttrs: requestedAttrs,
		specificAttrs:  make([]string, 0),
	}

	// Parse the requested attributes
	for _, attr := range requestedAttrs {
		switch strings.ToLower(strings.TrimSpace(attr)) {
		case "*":
			selector.hasAllUser = true
		case "+":
			selector.hasAllOp = true
		case "1.1":
			// Special case: "1.1" means no attributes
			// Don't add to specificAttrs
		default:
			if attr != "" {
				selector.specificAttrs = append(selector.specificAttrs, attr)
			}
		}
	}

	return selector
}

// Select selects attributes from an entry based on the selector configuration.
func (s *AttributeSelector) Select(entry *storage.Entry) map[string][][]byte {
	if entry == nil || entry.Attributes == nil {
		return make(map[string][][]byte)
	}

	// Special case: if "1.1" was the only attribute requested, return no attributes
	if len(s.requestedAttrs) == 1 && strings.TrimSpace(s.requestedAttrs[0]) == "1.1" {
		return make(map[string][][]byte)
	}

	// If no attributes requested, return all user attributes (default behavior)
	if len(s.requestedAttrs) == 0 {
		return s.selectAllUserAttributes(entry)
	}

	result := make(map[string][][]byte)

	// If "*" is requested, include all user attributes
	if s.hasAllUser {
		for name, values := range entry.Attributes {
			if !IsOperationalAttribute(name) {
				result[name] = values
			}
		}
	}

	// If "+" is requested, include all operational attributes
	if s.hasAllOp {
		for name, values := range entry.Attributes {
			if IsOperationalAttribute(name) {
				result[name] = values
			}
		}
	}

	// Add specifically requested attributes
	for _, attrName := range s.specificAttrs {
		s.addAttribute(result, entry, attrName)
	}

	return result
}

// selectAllUserAttributes returns all non-operational attributes from an entry.
func (s *AttributeSelector) selectAllUserAttributes(entry *storage.Entry) map[string][][]byte {
	result := make(map[string][][]byte)
	for name, values := range entry.Attributes {
		if !IsOperationalAttribute(name) {
			result[name] = values
		}
	}
	return result
}

// addAttribute adds an attribute to the result if it exists in the entry.
// Uses case-insensitive matching.
func (s *AttributeSelector) addAttribute(result map[string][][]byte, entry *storage.Entry, attrName string) {
	// Case-insensitive attribute lookup
	for name, values := range entry.Attributes {
		if strings.EqualFold(name, attrName) {
			result[name] = values
			return
		}
	}
}

// SelectAttributes is a convenience function that selects attributes from an entry.
// This is the main entry point for attribute selection.
func SelectAttributes(entry *storage.Entry, requestedAttrs []string) map[string][][]byte {
	selector := NewAttributeSelector(requestedAttrs)
	return selector.Select(entry)
}

// IsOperationalAttribute checks if an attribute is an operational attribute.
// Operational attributes are maintained by the server and are not returned
// by default in search results.
func IsOperationalAttribute(name string) bool {
	// List of common operational attributes per RFC 4512
	operationalAttrs := map[string]bool{
		// Timestamps
		"createtimestamp": true,
		"modifytimestamp": true,

		// Creator/modifier
		"creatorsname": true,
		"modifiersname": true,

		// Entry metadata
		"entrydn":            true,
		"entryuuid":          true,
		"subschemasubentry":  true,
		"hassubordinates":    true,
		"numsubordinates":    true,
		"structuralobjectclass": true,

		// DSA-specific
		"namingcontexts":       true,
		"supportedcontrol":     true,
		"supportedextension":   true,
		"supportedfeatures":    true,
		"supportedldapversion": true,
		"supportedsaslmechanisms": true,

		// Password policy
		"pwdchangedtime":       true,
		"pwdaccountlockedtime": true,
		"pwdfailuretime":       true,
		"pwdhistory":           true,
		"pwdgraceuseTime":      true,
		"pwdreset":             true,
		"pwdmustchange":        true,
		"pwdpolicysubentry":    true,
	}

	return operationalAttrs[strings.ToLower(name)]
}

// FilterAttributeValues filters attribute values based on typesOnly flag.
// If typesOnly is true, returns empty values slice.
func FilterAttributeValues(values [][]byte, typesOnly bool) [][]byte {
	if typesOnly {
		return nil
	}
	return values
}

// NormalizeAttributeName normalizes an attribute name for comparison.
// Converts to lowercase and trims whitespace.
func NormalizeAttributeName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// AttributeExists checks if an attribute exists in an entry (case-insensitive).
func AttributeExists(entry *storage.Entry, attrName string) bool {
	if entry == nil || entry.Attributes == nil {
		return false
	}

	normalizedName := NormalizeAttributeName(attrName)
	for name := range entry.Attributes {
		if NormalizeAttributeName(name) == normalizedName {
			return true
		}
	}
	return false
}

// GetAttributeValues retrieves attribute values from an entry (case-insensitive).
func GetAttributeValues(entry *storage.Entry, attrName string) [][]byte {
	if entry == nil || entry.Attributes == nil {
		return nil
	}

	normalizedName := NormalizeAttributeName(attrName)
	for name, values := range entry.Attributes {
		if NormalizeAttributeName(name) == normalizedName {
			return values
		}
	}
	return nil
}
