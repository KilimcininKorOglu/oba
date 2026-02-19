// Package radix provides a radix tree implementation optimized for DN (Distinguished Name)
// components in LDAP directory structures.
package radix

import (
	"errors"
	"strings"
)

// DN parsing errors.
var (
	ErrEmptyDN           = errors.New("DN cannot be empty")
	ErrInvalidDN         = errors.New("invalid DN format")
	ErrInvalidRDN        = errors.New("invalid RDN format")
	ErrEmptyRDNComponent = errors.New("empty RDN component")
)

// ParseDN parses a Distinguished Name string into its RDN (Relative Distinguished Name) components.
// The components are returned in reverse order (leaf first) for radix tree traversal.
//
// Example:
//
//	"uid=alice,ou=users,dc=example,dc=com" -> ["dc=com", "dc=example", "ou=users", "uid=alice"]
//
// This ordering allows efficient tree traversal from root to leaf.
func ParseDN(dn string) ([]string, error) {
	if dn == "" {
		return nil, ErrEmptyDN
	}

	// Trim whitespace
	dn = strings.TrimSpace(dn)
	if dn == "" {
		return nil, ErrEmptyDN
	}

	// Split by comma, handling escaped commas
	components := splitDN(dn)
	if len(components) == 0 {
		return nil, ErrInvalidDN
	}

	// Validate and normalize each component
	result := make([]string, len(components))
	for i, comp := range components {
		normalized, err := normalizeRDN(comp)
		if err != nil {
			return nil, err
		}
		// Store in reverse order (leaf first becomes last)
		result[len(components)-1-i] = normalized
	}

	return result, nil
}

// ParseDNForward parses a DN and returns components in forward order (root first).
// This is useful for display purposes.
//
// Example:
//
//	"uid=alice,ou=users,dc=example,dc=com" -> ["uid=alice", "ou=users", "dc=example", "dc=com"]
func ParseDNForward(dn string) ([]string, error) {
	if dn == "" {
		return nil, ErrEmptyDN
	}

	dn = strings.TrimSpace(dn)
	if dn == "" {
		return nil, ErrEmptyDN
	}

	components := splitDN(dn)
	if len(components) == 0 {
		return nil, ErrInvalidDN
	}

	result := make([]string, len(components))
	for i, comp := range components {
		normalized, err := normalizeRDN(comp)
		if err != nil {
			return nil, err
		}
		result[i] = normalized
	}

	return result, nil
}

// splitDN splits a DN string by commas, handling escaped commas.
func splitDN(dn string) []string {
	var components []string
	var current strings.Builder
	escaped := false

	for i := 0; i < len(dn); i++ {
		c := dn[i]

		if escaped {
			current.WriteByte(c)
			escaped = false
			continue
		}

		if c == '\\' {
			current.WriteByte(c)
			escaped = true
			continue
		}

		if c == ',' {
			comp := strings.TrimSpace(current.String())
			if comp != "" {
				components = append(components, comp)
			}
			current.Reset()
			continue
		}

		current.WriteByte(c)
	}

	// Add the last component
	comp := strings.TrimSpace(current.String())
	if comp != "" {
		components = append(components, comp)
	}

	return components
}

// normalizeRDN normalizes an RDN component.
// It validates the format and normalizes case for attribute types.
func normalizeRDN(rdn string) (string, error) {
	rdn = strings.TrimSpace(rdn)
	if rdn == "" {
		return "", ErrEmptyRDNComponent
	}

	// Find the equals sign
	eqIdx := strings.Index(rdn, "=")
	if eqIdx == -1 {
		return "", ErrInvalidRDN
	}

	attrType := strings.TrimSpace(rdn[:eqIdx])
	attrValue := strings.TrimSpace(rdn[eqIdx+1:])

	if attrType == "" {
		return "", ErrInvalidRDN
	}

	// Normalize attribute type to lowercase
	attrType = strings.ToLower(attrType)

	// Reconstruct the RDN
	return attrType + "=" + attrValue, nil
}

// JoinDN joins DN components into a DN string.
// Components should be in forward order (leaf first in LDAP convention).
//
// Example:
//
//	["uid=alice", "ou=users", "dc=example", "dc=com"] -> "uid=alice,ou=users,dc=example,dc=com"
func JoinDN(components []string) string {
	return strings.Join(components, ",")
}

// JoinDNReverse joins DN components in reverse order.
// Components should be in reverse order (root first).
//
// Example:
//
//	["dc=com", "dc=example", "ou=users", "uid=alice"] -> "uid=alice,ou=users,dc=example,dc=com"
func JoinDNReverse(components []string) string {
	if len(components) == 0 {
		return ""
	}

	reversed := make([]string, len(components))
	for i, comp := range components {
		reversed[len(components)-1-i] = comp
	}
	return strings.Join(reversed, ",")
}

// GetParentDN returns the parent DN of the given DN.
// Returns empty string if the DN has no parent (is a root entry).
//
// Example:
//
//	"uid=alice,ou=users,dc=example,dc=com" -> "ou=users,dc=example,dc=com"
func GetParentDN(dn string) (string, error) {
	components, err := ParseDNForward(dn)
	if err != nil {
		return "", err
	}

	if len(components) <= 1 {
		return "", nil
	}

	return JoinDN(components[1:]), nil
}

// GetRDN returns the RDN (first component) of the given DN.
//
// Example:
//
//	"uid=alice,ou=users,dc=example,dc=com" -> "uid=alice"
func GetRDN(dn string) (string, error) {
	components, err := ParseDNForward(dn)
	if err != nil {
		return "", err
	}

	if len(components) == 0 {
		return "", ErrInvalidDN
	}

	return components[0], nil
}

// IsDescendantOf checks if childDN is a descendant of parentDN.
//
// Example:
//
//	IsDescendantOf("uid=alice,ou=users,dc=example,dc=com", "dc=example,dc=com") -> true
func IsDescendantOf(childDN, parentDN string) (bool, error) {
	childComps, err := ParseDN(childDN)
	if err != nil {
		return false, err
	}

	parentComps, err := ParseDN(parentDN)
	if err != nil {
		return false, err
	}

	// Child must have more components than parent
	if len(childComps) <= len(parentComps) {
		return false, nil
	}

	// Check if parent components match the beginning of child components
	for i, comp := range parentComps {
		if !strings.EqualFold(childComps[i], comp) {
			return false, nil
		}
	}

	return true, nil
}

// IsDirectChildOf checks if childDN is a direct child of parentDN.
//
// Example:
//
//	IsDirectChildOf("ou=users,dc=example,dc=com", "dc=example,dc=com") -> true
//	IsDirectChildOf("uid=alice,ou=users,dc=example,dc=com", "dc=example,dc=com") -> false
func IsDirectChildOf(childDN, parentDN string) (bool, error) {
	childComps, err := ParseDN(childDN)
	if err != nil {
		return false, err
	}

	parentComps, err := ParseDN(parentDN)
	if err != nil {
		return false, err
	}

	// Child must have exactly one more component than parent
	if len(childComps) != len(parentComps)+1 {
		return false, nil
	}

	// Check if parent components match
	for i, comp := range parentComps {
		if !strings.EqualFold(childComps[i], comp) {
			return false, nil
		}
	}

	return true, nil
}

// NormalizeDN normalizes a DN string by parsing and rejoining it.
// This ensures consistent formatting.
func NormalizeDN(dn string) (string, error) {
	components, err := ParseDNForward(dn)
	if err != nil {
		return "", err
	}
	return JoinDN(components), nil
}

// CompareDN compares two DNs for equality (case-insensitive for attribute types).
func CompareDN(dn1, dn2 string) (bool, error) {
	norm1, err := NormalizeDN(dn1)
	if err != nil {
		return false, err
	}

	norm2, err := NormalizeDN(dn2)
	if err != nil {
		return false, err
	}

	return strings.EqualFold(norm1, norm2), nil
}

// DNDepth returns the number of components in a DN.
//
// Example:
//
//	DNDepth("uid=alice,ou=users,dc=example,dc=com") -> 4
func DNDepth(dn string) (int, error) {
	components, err := ParseDN(dn)
	if err != nil {
		return 0, err
	}
	return len(components), nil
}
