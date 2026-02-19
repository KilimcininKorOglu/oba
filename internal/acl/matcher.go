// Package acl provides Access Control List (ACL) data structures and evaluation
// for the Oba LDAP server.
package acl

import (
	"strings"
)

// Matcher provides DN and subject matching functionality for ACL evaluation.
type Matcher struct{}

// NewMatcher creates a new Matcher instance.
func NewMatcher() *Matcher {
	return &Matcher{}
}

// MatchesTarget checks if the target DN matches the ACL rule's target pattern.
func (m *Matcher) MatchesTarget(rule *ACL, targetDN string) bool {
	// Normalize DNs for comparison (case-insensitive)
	ruleTarget := strings.ToLower(rule.Target)
	target := strings.ToLower(targetDN)

	// Wildcard matches everything
	if ruleTarget == "*" {
		return true
	}

	switch rule.Scope {
	case ScopeBase:
		// Exact match only
		return target == ruleTarget

	case ScopeOne:
		// Target must be an immediate child of the rule target
		return m.isImmediateChild(ruleTarget, target)

	case ScopeSubtree:
		// Target must be the rule target or a descendant
		return m.isSubtreeMatch(ruleTarget, target)

	default:
		return false
	}
}

// MatchesSubject checks if the bind DN matches the ACL rule's subject.
func (m *Matcher) MatchesSubject(rule *ACL, bindDN, targetDN string) bool {
	subject := strings.ToLower(rule.Subject)
	bind := strings.ToLower(bindDN)

	switch subject {
	case "anonymous":
		// Matches only unauthenticated users
		return bindDN == ""

	case "authenticated":
		// Matches any authenticated user
		return bindDN != ""

	case "self":
		// Matches when the bind DN equals the target DN
		return bindDN != "" && strings.EqualFold(bindDN, targetDN)

	case "*":
		// Matches everyone (anonymous and authenticated)
		return true

	default:
		// Exact DN match
		return bind == subject
	}
}

// isImmediateChild checks if target is an immediate child of parent.
// For example: "uid=alice,ou=users,dc=example,dc=com" is an immediate child of "ou=users,dc=example,dc=com"
func (m *Matcher) isImmediateChild(parent, target string) bool {
	if parent == "" || target == "" {
		return false
	}

	// Target must end with the parent DN
	if !strings.HasSuffix(target, parent) {
		return false
	}

	// Get the prefix (the part before the parent)
	prefix := strings.TrimSuffix(target, parent)
	if prefix == "" {
		// Target equals parent, not a child
		return false
	}

	// Remove trailing comma from prefix
	prefix = strings.TrimSuffix(prefix, ",")
	if prefix == "" {
		return false
	}

	// Prefix should be a single RDN (no commas)
	// This means target is an immediate child
	return !strings.Contains(prefix, ",")
}

// isSubtreeMatch checks if target is equal to or a descendant of base.
func (m *Matcher) isSubtreeMatch(base, target string) bool {
	if base == "" {
		return true // Empty base matches everything
	}

	if target == "" {
		return false
	}

	// Exact match
	if target == base {
		return true
	}

	// Target must end with ",base" to be a descendant
	suffix := "," + base
	return strings.HasSuffix(target, suffix)
}

// ParseDN splits a DN into its RDN components.
// For example: "uid=alice,ou=users,dc=example,dc=com" returns
// ["uid=alice", "ou=users", "dc=example", "dc=com"]
func (m *Matcher) ParseDN(dn string) []string {
	if dn == "" {
		return nil
	}

	// Simple split by comma - doesn't handle escaped commas
	// For a production implementation, proper DN parsing would be needed
	parts := strings.Split(dn, ",")
	result := make([]string, 0, len(parts))

	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}

// GetParentDN returns the parent DN of the given DN.
// For example: "uid=alice,ou=users,dc=example,dc=com" returns "ou=users,dc=example,dc=com"
func (m *Matcher) GetParentDN(dn string) string {
	if dn == "" {
		return ""
	}

	idx := strings.Index(dn, ",")
	if idx == -1 {
		return "" // Root DN has no parent
	}

	return dn[idx+1:]
}

// NormalizeDN normalizes a DN for comparison by converting to lowercase
// and trimming whitespace around components.
func (m *Matcher) NormalizeDN(dn string) string {
	if dn == "" {
		return ""
	}

	parts := m.ParseDN(dn)
	normalized := make([]string, len(parts))

	for i, part := range parts {
		normalized[i] = strings.ToLower(strings.TrimSpace(part))
	}

	return strings.Join(normalized, ",")
}

// MatchesPattern checks if a DN matches a pattern with wildcards.
// Supports "*" as a wildcard for any single RDN component.
// For example: "uid=*,ou=users,dc=example,dc=com" matches "uid=alice,ou=users,dc=example,dc=com"
func (m *Matcher) MatchesPattern(pattern, dn string) bool {
	if pattern == "*" {
		return true
	}

	patternParts := m.ParseDN(strings.ToLower(pattern))
	dnParts := m.ParseDN(strings.ToLower(dn))

	if len(patternParts) != len(dnParts) {
		return false
	}

	for i, patternPart := range patternParts {
		if patternPart == "*" {
			continue // Wildcard matches any RDN
		}

		// Check for attribute=* pattern (e.g., "uid=*")
		if strings.HasSuffix(patternPart, "=*") {
			attrName := strings.TrimSuffix(patternPart, "=*")
			dnAttr := strings.SplitN(dnParts[i], "=", 2)
			if len(dnAttr) < 2 || dnAttr[0] != attrName {
				return false
			}
			continue
		}

		if patternPart != dnParts[i] {
			return false
		}
	}

	return true
}
