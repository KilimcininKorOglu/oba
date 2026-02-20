// Package server provides the LDAP server implementation.
package server

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"sync"
	"time"

	"github.com/oba-ldap/oba/internal/ldap"
)

// Paging errors.
var (
	// ErrInvalidCookie is returned when the paging cookie is invalid or expired.
	ErrInvalidCookie = errors.New("server: invalid or expired paging cookie")
	// ErrCookieMismatch is returned when the cookie doesn't match the search parameters.
	ErrCookieMismatch = errors.New("server: cookie does not match search parameters")
)

// PagedSearchState holds the state for a paged search operation.
// This state is stored server-side and referenced by an opaque cookie.
type PagedSearchState struct {
	// ID is the unique identifier for this search state.
	ID string
	// BaseDN is the base DN of the search.
	BaseDN string
	// Scope is the search scope.
	Scope ldap.SearchScope
	// Filter is the search filter (stored as string representation for comparison).
	FilterStr string
	// Attributes is the list of requested attributes.
	Attributes []string
	// TypesOnly indicates whether only attribute types should be returned.
	TypesOnly bool
	// Position is the current position in the result set.
	Position int
	// TotalCount is the total number of matching entries.
	TotalCount int
	// Results holds all matching entries for this search.
	Results []*SearchEntry
	// CreatedAt is when this state was created.
	CreatedAt time.Time
	// LastAccessed is when this state was last accessed.
	LastAccessed time.Time
}

// PagedSearchManager manages paged search states.
type PagedSearchManager struct {
	// states maps cookie IDs to search states.
	states map[string]*PagedSearchState
	// mu protects concurrent access to states.
	mu sync.RWMutex
	// stateTimeout is how long a state is valid.
	stateTimeout time.Duration
	// maxStates is the maximum number of concurrent paged searches.
	maxStates int
}

// PagedSearchManagerConfig holds configuration for the PagedSearchManager.
type PagedSearchManagerConfig struct {
	// StateTimeout is how long a paged search state is valid (default: 5 minutes).
	StateTimeout time.Duration
	// MaxStates is the maximum number of concurrent paged searches (default: 1000).
	MaxStates int
}

// DefaultPagedSearchManagerConfig returns the default configuration.
func DefaultPagedSearchManagerConfig() *PagedSearchManagerConfig {
	return &PagedSearchManagerConfig{
		StateTimeout: 5 * time.Minute,
		MaxStates:    1000,
	}
}

// NewPagedSearchManager creates a new PagedSearchManager.
func NewPagedSearchManager(config *PagedSearchManagerConfig) *PagedSearchManager {
	if config == nil {
		config = DefaultPagedSearchManagerConfig()
	}

	mgr := &PagedSearchManager{
		states:       make(map[string]*PagedSearchState),
		stateTimeout: config.StateTimeout,
		maxStates:    config.MaxStates,
	}

	// Start cleanup goroutine
	go mgr.cleanupLoop()

	return mgr
}

// CreateState creates a new paged search state and returns its cookie.
func (m *PagedSearchManager) CreateState(
	baseDN string,
	scope ldap.SearchScope,
	filterStr string,
	attributes []string,
	typesOnly bool,
	results []*SearchEntry,
) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if we've reached the maximum number of states
	if len(m.states) >= m.maxStates {
		// Try to clean up expired states first
		m.cleanupExpiredLocked()
		if len(m.states) >= m.maxStates {
			return "", errors.New("server: maximum number of paged searches reached")
		}
	}

	// Generate a unique ID
	id, err := generateCookieID()
	if err != nil {
		return "", err
	}

	now := time.Now()
	state := &PagedSearchState{
		ID:           id,
		BaseDN:       baseDN,
		Scope:        scope,
		FilterStr:    filterStr,
		Attributes:   attributes,
		TypesOnly:    typesOnly,
		Position:     0,
		TotalCount:   len(results),
		Results:      results,
		CreatedAt:    now,
		LastAccessed: now,
	}

	m.states[id] = state

	return id, nil
}

// GetState retrieves a paged search state by its cookie ID.
func (m *PagedSearchManager) GetState(cookieID string) (*PagedSearchState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, ok := m.states[cookieID]
	if !ok {
		return nil, ErrInvalidCookie
	}

	// Check if state has expired
	if time.Since(state.LastAccessed) > m.stateTimeout {
		delete(m.states, cookieID)
		return nil, ErrInvalidCookie
	}

	// Update last accessed time
	state.LastAccessed = time.Now()

	return state, nil
}

// UpdatePosition updates the position in a paged search state.
func (m *PagedSearchManager) UpdatePosition(cookieID string, newPosition int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, ok := m.states[cookieID]
	if !ok {
		return ErrInvalidCookie
	}

	state.Position = newPosition
	state.LastAccessed = time.Now()

	return nil
}

// DeleteState removes a paged search state.
func (m *PagedSearchManager) DeleteState(cookieID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.states, cookieID)
}

// cleanupLoop periodically cleans up expired states.
func (m *PagedSearchManager) cleanupLoop() {
	ticker := time.NewTicker(m.stateTimeout / 2)
	defer ticker.Stop()

	for range ticker.C {
		m.mu.Lock()
		m.cleanupExpiredLocked()
		m.mu.Unlock()
	}
}

// cleanupExpiredLocked removes expired states. Must be called with lock held.
func (m *PagedSearchManager) cleanupExpiredLocked() {
	now := time.Now()
	for id, state := range m.states {
		if now.Sub(state.LastAccessed) > m.stateTimeout {
			delete(m.states, id)
		}
	}
}

// generateCookieID generates a unique cookie ID.
func generateCookieID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// EncodeCookie encodes a cookie ID into a cookie byte slice.
// The cookie format is: [version(1)] [id_length(2)] [id(n)]
func EncodeCookie(cookieID string) []byte {
	idBytes := []byte(cookieID)
	cookie := make([]byte, 3+len(idBytes))
	cookie[0] = 1 // Version 1
	binary.BigEndian.PutUint16(cookie[1:3], uint16(len(idBytes)))
	copy(cookie[3:], idBytes)
	return cookie
}

// DecodeCookie decodes a cookie byte slice into a cookie ID.
func DecodeCookie(cookie []byte) (string, error) {
	if len(cookie) < 3 {
		return "", ErrInvalidCookie
	}

	version := cookie[0]
	if version != 1 {
		return "", ErrInvalidCookie
	}

	idLen := binary.BigEndian.Uint16(cookie[1:3])
	if len(cookie) < 3+int(idLen) {
		return "", ErrInvalidCookie
	}

	return string(cookie[3 : 3+idLen]), nil
}

// ValidateSearchParameters checks if the search parameters match the stored state.
func (s *PagedSearchState) ValidateSearchParameters(
	baseDN string,
	scope ldap.SearchScope,
	filterStr string,
	attributes []string,
	typesOnly bool,
) error {
	if s.BaseDN != baseDN {
		return ErrCookieMismatch
	}
	if s.Scope != scope {
		return ErrCookieMismatch
	}
	if s.FilterStr != filterStr {
		return ErrCookieMismatch
	}
	if s.TypesOnly != typesOnly {
		return ErrCookieMismatch
	}
	// Note: We don't validate attributes as clients may request different attributes
	// for different pages (though this is unusual).

	return nil
}

// GetPage returns a page of results starting from the current position.
// Returns the entries for this page and whether there are more results.
func (s *PagedSearchState) GetPage(pageSize int) ([]*SearchEntry, bool) {
	if s.Position >= len(s.Results) {
		return nil, false
	}

	end := s.Position + pageSize
	if end > len(s.Results) {
		end = len(s.Results)
	}

	entries := s.Results[s.Position:end]
	hasMore := end < len(s.Results)

	return entries, hasMore
}

// filterToString converts a search filter to a string for comparison.
// This is a simple representation for state validation.
func filterToString(f *ldap.SearchFilter) string {
	if f == nil {
		return "(objectClass=*)"
	}

	switch f.Type {
	case ldap.FilterTagAnd:
		result := "(&"
		for _, child := range f.Children {
			result += filterToString(child)
		}
		return result + ")"

	case ldap.FilterTagOr:
		result := "(|"
		for _, child := range f.Children {
			result += filterToString(child)
		}
		return result + ")"

	case ldap.FilterTagNot:
		return "(!" + filterToString(f.Child) + ")"

	case ldap.FilterTagEquality:
		return "(" + f.Attribute + "=" + string(f.Value) + ")"

	case ldap.FilterTagPresent:
		return "(" + f.Attribute + "=*)"

	case ldap.FilterTagSubstrings:
		result := "(" + f.Attribute + "="
		if f.Substrings != nil {
			if len(f.Substrings.Initial) > 0 {
				result += string(f.Substrings.Initial)
			}
			result += "*"
			for _, any := range f.Substrings.Any {
				result += string(any) + "*"
			}
			if len(f.Substrings.Final) > 0 {
				result += string(f.Substrings.Final)
			}
		}
		return result + ")"

	case ldap.FilterTagGreaterOrEqual:
		return "(" + f.Attribute + ">=" + string(f.Value) + ")"

	case ldap.FilterTagLessOrEqual:
		return "(" + f.Attribute + "<=" + string(f.Value) + ")"

	case ldap.FilterTagApproxMatch:
		return "(" + f.Attribute + "~=" + string(f.Value) + ")"

	default:
		return "(unknown)"
	}
}

// FilterToString exports the filterToString function for use in handlers.
func FilterToString(f *ldap.SearchFilter) string {
	return filterToString(f)
}
