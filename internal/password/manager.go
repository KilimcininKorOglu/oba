// Package password provides password policy configuration and validation
// for the Oba LDAP server.
package password

import (
	"strings"
	"sync"
)

// Manager handles password policy management with support for
// global policy and per-user overrides.
type Manager struct {
	mu           sync.RWMutex
	globalPolicy *Policy
	userPolicies map[string]*Policy // DN -> policy
}

// NewManager creates a new password policy manager with the given global policy.
// If global is nil, a default policy is used.
func NewManager(global *Policy) *Manager {
	if global == nil {
		global = DefaultPolicy()
	}
	return &Manager{
		globalPolicy: global.Clone(),
		userPolicies: make(map[string]*Policy),
	}
}

// GetPolicy returns the effective password policy for a given DN.
// If a user-specific policy exists, it is merged with the global policy.
// The user policy values override the global policy values.
func (m *Manager) GetPolicy(dn string) *Policy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	normalizedDN := normalizeDN(dn)

	userPolicy, exists := m.userPolicies[normalizedDN]
	if !exists {
		return m.globalPolicy.Clone()
	}

	return m.globalPolicy.Merge(userPolicy)
}

// SetUserPolicy sets a password policy override for a specific user DN.
// Pass nil to remove the user-specific policy.
func (m *Manager) SetUserPolicy(dn string, policy *Policy) {
	m.mu.Lock()
	defer m.mu.Unlock()

	normalizedDN := normalizeDN(dn)

	if policy == nil {
		delete(m.userPolicies, normalizedDN)
		return
	}

	m.userPolicies[normalizedDN] = policy.Clone()
}

// GetUserPolicy returns the user-specific policy override for a DN.
// Returns nil if no user-specific policy exists.
func (m *Manager) GetUserPolicy(dn string) *Policy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	normalizedDN := normalizeDN(dn)

	policy, exists := m.userPolicies[normalizedDN]
	if !exists {
		return nil
	}

	return policy.Clone()
}

// HasUserPolicy checks if a user-specific policy exists for the given DN.
func (m *Manager) HasUserPolicy(dn string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	normalizedDN := normalizeDN(dn)
	_, exists := m.userPolicies[normalizedDN]
	return exists
}

// RemoveUserPolicy removes the user-specific policy for a DN.
func (m *Manager) RemoveUserPolicy(dn string) {
	m.SetUserPolicy(dn, nil)
}

// GetGlobalPolicy returns a copy of the global password policy.
func (m *Manager) GetGlobalPolicy() *Policy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.globalPolicy.Clone()
}

// SetGlobalPolicy updates the global password policy.
func (m *Manager) SetGlobalPolicy(policy *Policy) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if policy == nil {
		m.globalPolicy = DefaultPolicy()
		return
	}

	m.globalPolicy = policy.Clone()
}

// ListUserPolicies returns a list of all DNs that have user-specific policies.
func (m *Manager) ListUserPolicies() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	dns := make([]string, 0, len(m.userPolicies))
	for dn := range m.userPolicies {
		dns = append(dns, dn)
	}
	return dns
}

// UserPolicyCount returns the number of user-specific policies.
func (m *Manager) UserPolicyCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.userPolicies)
}

// ValidatePassword validates a password against the effective policy for a DN.
func (m *Manager) ValidatePassword(dn, password string) error {
	policy := m.GetPolicy(dn)
	return policy.Validate(password)
}

// ClearUserPolicies removes all user-specific policies.
func (m *Manager) ClearUserPolicies() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.userPolicies = make(map[string]*Policy)
}

// normalizeDN normalizes a DN for consistent map lookups.
// It converts to lowercase and trims whitespace.
func normalizeDN(dn string) string {
	// Normalize by converting to lowercase and trimming spaces
	// This ensures consistent lookups regardless of case variations
	return strings.ToLower(strings.TrimSpace(dn))
}
