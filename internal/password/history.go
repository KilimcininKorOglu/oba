// Package password provides password policy configuration and validation
// for the Oba LDAP server.
package password

import (
	"crypto/subtle"
	"sync"
)

// History tracks previous password hashes to prevent reuse.
// It maintains a fixed-size list of hashed passwords and provides
// constant-time comparison to check if a password was used before.
type History struct {
	mu       sync.RWMutex
	hashes   []string // Previous password hashes (newest first)
	maxCount int      // Maximum number of hashes to retain
}

// NewHistory creates a new password history tracker with the specified
// maximum number of passwords to remember.
// If maxCount is 0 or negative, history tracking is effectively disabled.
func NewHistory(maxCount int) *History {
	if maxCount < 0 {
		maxCount = 0
	}
	return &History{
		hashes:   make([]string, 0, maxCount),
		maxCount: maxCount,
	}
}

// NewHistoryFromHashes creates a History from existing hashes.
// This is useful when loading history from storage.
// The hashes should be ordered from newest to oldest.
func NewHistoryFromHashes(hashes []string, maxCount int) *History {
	if maxCount < 0 {
		maxCount = 0
	}

	h := &History{
		maxCount: maxCount,
	}

	// If maxCount is 0, don't store any hashes
	if maxCount == 0 {
		h.hashes = make([]string, 0)
		return h
	}

	// Copy and truncate if necessary
	if len(hashes) > maxCount {
		h.hashes = make([]string, maxCount)
		copy(h.hashes, hashes[:maxCount])
	} else {
		h.hashes = make([]string, len(hashes))
		copy(h.hashes, hashes)
	}

	return h
}

// Add adds a new password hash to the history.
// The hash is prepended to the list (newest first).
// If the history exceeds maxCount, the oldest hash is removed.
func (h *History) Add(hash string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.maxCount == 0 {
		return
	}

	// Prepend the new hash
	h.hashes = append([]string{hash}, h.hashes...)

	// Trim to maxCount
	if len(h.hashes) > h.maxCount {
		h.hashes = h.hashes[:h.maxCount]
	}
}

// Contains checks if the given hash exists in the history.
// Uses constant-time comparison to prevent timing attacks.
func (h *History) Contains(hash string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, stored := range h.hashes {
		if subtle.ConstantTimeCompare([]byte(hash), []byte(stored)) == 1 {
			return true
		}
	}
	return false
}

// Count returns the number of hashes currently stored.
func (h *History) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return len(h.hashes)
}

// MaxCount returns the maximum number of hashes that can be stored.
func (h *History) MaxCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return h.maxCount
}

// Hashes returns a copy of all stored hashes (newest first).
// This is useful for persisting the history to storage.
func (h *History) Hashes() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make([]string, len(h.hashes))
	copy(result, h.hashes)
	return result
}

// Clear removes all stored hashes.
func (h *History) Clear() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.hashes = make([]string, 0, h.maxCount)
}

// SetMaxCount updates the maximum number of hashes to retain.
// If the new max is smaller than the current count, older hashes are removed.
func (h *History) SetMaxCount(maxCount int) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if maxCount < 0 {
		maxCount = 0
	}

	h.maxCount = maxCount

	// Trim if necessary
	if maxCount == 0 {
		h.hashes = make([]string, 0)
	} else if len(h.hashes) > maxCount {
		h.hashes = h.hashes[:maxCount]
	}
}

// IsEnabled returns true if history tracking is enabled (maxCount > 0).
func (h *History) IsEnabled() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return h.maxCount > 0
}

// Clone creates a deep copy of the history.
func (h *History) Clone() *History {
	h.mu.RLock()
	defer h.mu.RUnlock()

	clone := &History{
		maxCount: h.maxCount,
		hashes:   make([]string, len(h.hashes)),
	}
	copy(clone.hashes, h.hashes)
	return clone
}

// HistoryManager manages password histories for multiple users.
type HistoryManager struct {
	mu              sync.RWMutex
	histories       map[string]*History // DN -> History
	defaultMaxCount int
}

// NewHistoryManager creates a new history manager with the specified
// default maximum count for new histories.
func NewHistoryManager(defaultMaxCount int) *HistoryManager {
	if defaultMaxCount < 0 {
		defaultMaxCount = 0
	}
	return &HistoryManager{
		histories:       make(map[string]*History),
		defaultMaxCount: defaultMaxCount,
	}
}

// GetHistory returns the password history for a user DN.
// If no history exists, a new one is created with the default max count.
func (m *HistoryManager) GetHistory(dn string) *History {
	m.mu.Lock()
	defer m.mu.Unlock()

	normalizedDN := normalizeDN(dn)

	history, exists := m.histories[normalizedDN]
	if !exists {
		history = NewHistory(m.defaultMaxCount)
		m.histories[normalizedDN] = history
	}

	return history
}

// SetHistory sets the password history for a user DN.
// Pass nil to remove the history.
func (m *HistoryManager) SetHistory(dn string, history *History) {
	m.mu.Lock()
	defer m.mu.Unlock()

	normalizedDN := normalizeDN(dn)

	if history == nil {
		delete(m.histories, normalizedDN)
		return
	}

	m.histories[normalizedDN] = history.Clone()
}

// HasHistory checks if a history exists for the given DN.
func (m *HistoryManager) HasHistory(dn string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	normalizedDN := normalizeDN(dn)
	_, exists := m.histories[normalizedDN]
	return exists
}

// RemoveHistory removes the history for a user DN.
func (m *HistoryManager) RemoveHistory(dn string) {
	m.SetHistory(dn, nil)
}

// AddPassword adds a password hash to a user's history.
// Creates a new history if one doesn't exist.
func (m *HistoryManager) AddPassword(dn, hash string) {
	history := m.GetHistory(dn)
	history.Add(hash)
}

// ContainsPassword checks if a password hash exists in a user's history.
func (m *HistoryManager) ContainsPassword(dn, hash string) bool {
	m.mu.RLock()
	normalizedDN := normalizeDN(dn)
	history, exists := m.histories[normalizedDN]
	m.mu.RUnlock()

	if !exists {
		return false
	}

	return history.Contains(hash)
}

// ListUsers returns a list of all DNs that have password histories.
func (m *HistoryManager) ListUsers() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	dns := make([]string, 0, len(m.histories))
	for dn := range m.histories {
		dns = append(dns, dn)
	}
	return dns
}

// UserCount returns the number of users with password histories.
func (m *HistoryManager) UserCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.histories)
}

// ClearAll removes all password histories.
func (m *HistoryManager) ClearAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.histories = make(map[string]*History)
}

// SetDefaultMaxCount updates the default max count for new histories.
// This does not affect existing histories.
func (m *HistoryManager) SetDefaultMaxCount(maxCount int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if maxCount < 0 {
		maxCount = 0
	}
	m.defaultMaxCount = maxCount
}

// DefaultMaxCount returns the default max count for new histories.
func (m *HistoryManager) DefaultMaxCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.defaultMaxCount
}
