// Package password provides password policy configuration and validation
// for the Oba LDAP server.
package password

import (
	"sync"
	"time"
)

// Expiration tracks password age and expiration status.
// It supports grace logins for expired passwords and provides
// methods to check expiration status and remaining time.
type Expiration struct {
	mu          sync.RWMutex
	changedTime time.Time     // When the password was last changed
	maxAge      time.Duration // Maximum password age (0 = never expires)
	graceLogins int           // Number of grace logins allowed after expiration
	graceUsed   int           // Number of grace logins already used
}

// NewExpiration creates a new password expiration tracker.
// changedTime is when the password was last changed.
// maxAge is the maximum password age (0 means never expires).
// graceLogins is the number of grace logins allowed after expiration.
func NewExpiration(changedTime time.Time, maxAge time.Duration, graceLogins int) *Expiration {
	if graceLogins < 0 {
		graceLogins = 0
	}
	return &Expiration{
		changedTime: changedTime,
		maxAge:      maxAge,
		graceLogins: graceLogins,
		graceUsed:   0,
	}
}

// NewExpirationWithGraceUsed creates an Expiration with pre-used grace logins.
// This is useful when loading state from storage.
func NewExpirationWithGraceUsed(changedTime time.Time, maxAge time.Duration, graceLogins, graceUsed int) *Expiration {
	if graceLogins < 0 {
		graceLogins = 0
	}
	if graceUsed < 0 {
		graceUsed = 0
	}
	if graceUsed > graceLogins {
		graceUsed = graceLogins
	}
	return &Expiration{
		changedTime: changedTime,
		maxAge:      maxAge,
		graceLogins: graceLogins,
		graceUsed:   graceUsed,
	}
}

// IsExpired checks if the password has exceeded its maximum age.
// Returns false if maxAge is 0 (never expires).
func (e *Expiration) IsExpired() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.isExpiredLocked()
}

// isExpiredLocked is the internal version without locking.
func (e *Expiration) isExpiredLocked() bool {
	if e.maxAge == 0 {
		return false
	}
	return time.Since(e.changedTime) > e.maxAge
}

// IsExpiredAt checks if the password would be expired at the given time.
func (e *Expiration) IsExpiredAt(t time.Time) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.maxAge == 0 {
		return false
	}
	return t.Sub(e.changedTime) > e.maxAge
}

// DaysUntilExpiry returns the number of days until the password expires.
// Returns -1 if the password never expires (maxAge == 0).
// Returns 0 if already expired.
func (e *Expiration) DaysUntilExpiry() int {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.maxAge == 0 {
		return -1 // Never expires
	}

	remaining := e.maxAge - time.Since(e.changedTime)
	if remaining <= 0 {
		return 0
	}

	return int(remaining.Hours() / 24)
}

// TimeUntilExpiry returns the duration until the password expires.
// Returns -1 if the password never expires (maxAge == 0).
// Returns 0 if already expired.
func (e *Expiration) TimeUntilExpiry() time.Duration {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.maxAge == 0 {
		return -1 // Never expires
	}

	remaining := e.maxAge - time.Since(e.changedTime)
	if remaining <= 0 {
		return 0
	}

	return remaining
}

// ExpiryTime returns the time when the password will expire.
// Returns zero time if the password never expires.
func (e *Expiration) ExpiryTime() time.Time {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.maxAge == 0 {
		return time.Time{}
	}

	return e.changedTime.Add(e.maxAge)
}

// ChangedTime returns when the password was last changed.
func (e *Expiration) ChangedTime() time.Time {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.changedTime
}

// MaxAge returns the maximum password age.
func (e *Expiration) MaxAge() time.Duration {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.maxAge
}

// SetChangedTime updates the password change time.
// This also resets the grace login counter.
func (e *Expiration) SetChangedTime(t time.Time) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.changedTime = t
	e.graceUsed = 0
}

// SetMaxAge updates the maximum password age.
func (e *Expiration) SetMaxAge(maxAge time.Duration) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.maxAge = maxAge
}

// GraceLogins returns the total number of grace logins allowed.
func (e *Expiration) GraceLogins() int {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.graceLogins
}

// GraceLoginsRemaining returns the number of grace logins still available.
func (e *Expiration) GraceLoginsRemaining() int {
	e.mu.RLock()
	defer e.mu.RUnlock()

	remaining := e.graceLogins - e.graceUsed
	if remaining < 0 {
		return 0
	}
	return remaining
}

// GraceLoginsUsed returns the number of grace logins already used.
func (e *Expiration) GraceLoginsUsed() int {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.graceUsed
}

// SetGraceLogins updates the total number of grace logins allowed.
func (e *Expiration) SetGraceLogins(count int) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if count < 0 {
		count = 0
	}
	e.graceLogins = count
}

// UseGraceLogin consumes one grace login if available.
// Returns true if a grace login was successfully used.
// Returns false if no grace logins are available or password is not expired.
func (e *Expiration) UseGraceLogin() bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Grace logins only apply when password is expired
	if !e.isExpiredLocked() {
		return false
	}

	if e.graceUsed >= e.graceLogins {
		return false
	}

	e.graceUsed++
	return true
}

// CanUseGraceLogin checks if a grace login can be used.
// Returns true if the password is expired and grace logins are available.
func (e *Expiration) CanUseGraceLogin() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if !e.isExpiredLocked() {
		return false
	}

	return e.graceUsed < e.graceLogins
}

// ResetGraceLogins resets the grace login counter to zero.
func (e *Expiration) ResetGraceLogins() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.graceUsed = 0
}

// IsFullyExpired returns true if the password is expired and
// all grace logins have been exhausted.
func (e *Expiration) IsFullyExpired() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if !e.isExpiredLocked() {
		return false
	}

	return e.graceUsed >= e.graceLogins
}

// Clone creates a deep copy of the expiration.
func (e *Expiration) Clone() *Expiration {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return &Expiration{
		changedTime: e.changedTime,
		maxAge:      e.maxAge,
		graceLogins: e.graceLogins,
		graceUsed:   e.graceUsed,
	}
}

// ExpirationStatus represents the current state of password expiration.
type ExpirationStatus int

const (
	// StatusValid indicates the password is not expired.
	StatusValid ExpirationStatus = iota

	// StatusExpiring indicates the password will expire soon (within warning period).
	StatusExpiring

	// StatusExpiredGrace indicates the password is expired but grace logins are available.
	StatusExpiredGrace

	// StatusExpiredLocked indicates the password is expired and no grace logins remain.
	StatusExpiredLocked
)

// String returns a string representation of the expiration status.
func (s ExpirationStatus) String() string {
	switch s {
	case StatusValid:
		return "valid"
	case StatusExpiring:
		return "expiring"
	case StatusExpiredGrace:
		return "expired_grace"
	case StatusExpiredLocked:
		return "expired_locked"
	default:
		return "unknown"
	}
}

// Status returns the current expiration status with an optional warning period.
// If warningPeriod is > 0, StatusExpiring is returned when the password
// will expire within that duration.
func (e *Expiration) Status(warningPeriod time.Duration) ExpirationStatus {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.maxAge == 0 {
		return StatusValid
	}

	elapsed := time.Since(e.changedTime)

	// Check if expired
	if elapsed > e.maxAge {
		if e.graceUsed < e.graceLogins {
			return StatusExpiredGrace
		}
		return StatusExpiredLocked
	}

	// Check if expiring soon
	if warningPeriod > 0 {
		remaining := e.maxAge - elapsed
		if remaining <= warningPeriod {
			return StatusExpiring
		}
	}

	return StatusValid
}

// ExpirationManager manages password expiration for multiple users.
type ExpirationManager struct {
	mu                 sync.RWMutex
	expirations        map[string]*Expiration // DN -> Expiration
	defaultMaxAge      time.Duration
	defaultGraceLogins int
}

// NewExpirationManager creates a new expiration manager.
func NewExpirationManager(defaultMaxAge time.Duration, defaultGraceLogins int) *ExpirationManager {
	if defaultGraceLogins < 0 {
		defaultGraceLogins = 0
	}
	return &ExpirationManager{
		expirations:        make(map[string]*Expiration),
		defaultMaxAge:      defaultMaxAge,
		defaultGraceLogins: defaultGraceLogins,
	}
}

// GetExpiration returns the expiration tracker for a user DN.
// If no expiration exists, returns nil.
func (m *ExpirationManager) GetExpiration(dn string) *Expiration {
	m.mu.RLock()
	defer m.mu.RUnlock()

	normalizedDN := normalizeDN(dn)
	return m.expirations[normalizedDN]
}

// GetOrCreateExpiration returns the expiration tracker for a user DN.
// If no expiration exists, a new one is created with the current time
// and default settings.
func (m *ExpirationManager) GetOrCreateExpiration(dn string) *Expiration {
	m.mu.Lock()
	defer m.mu.Unlock()

	normalizedDN := normalizeDN(dn)

	exp, exists := m.expirations[normalizedDN]
	if !exists {
		exp = NewExpiration(time.Now(), m.defaultMaxAge, m.defaultGraceLogins)
		m.expirations[normalizedDN] = exp
	}

	return exp
}

// SetExpiration sets the expiration tracker for a user DN.
// Pass nil to remove the expiration.
func (m *ExpirationManager) SetExpiration(dn string, exp *Expiration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	normalizedDN := normalizeDN(dn)

	if exp == nil {
		delete(m.expirations, normalizedDN)
		return
	}

	m.expirations[normalizedDN] = exp.Clone()
}

// HasExpiration checks if an expiration exists for the given DN.
func (m *ExpirationManager) HasExpiration(dn string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	normalizedDN := normalizeDN(dn)
	_, exists := m.expirations[normalizedDN]
	return exists
}

// RemoveExpiration removes the expiration for a user DN.
func (m *ExpirationManager) RemoveExpiration(dn string) {
	m.SetExpiration(dn, nil)
}

// IsExpired checks if a user's password is expired.
// Returns false if no expiration exists for the user.
func (m *ExpirationManager) IsExpired(dn string) bool {
	exp := m.GetExpiration(dn)
	if exp == nil {
		return false
	}
	return exp.IsExpired()
}

// IsFullyExpired checks if a user's password is expired with no grace logins remaining.
// Returns false if no expiration exists for the user.
func (m *ExpirationManager) IsFullyExpired(dn string) bool {
	exp := m.GetExpiration(dn)
	if exp == nil {
		return false
	}
	return exp.IsFullyExpired()
}

// RecordPasswordChange records a password change for a user.
// Creates a new expiration if one doesn't exist.
func (m *ExpirationManager) RecordPasswordChange(dn string, changedTime time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()

	normalizedDN := normalizeDN(dn)

	exp, exists := m.expirations[normalizedDN]
	if !exists {
		exp = NewExpiration(changedTime, m.defaultMaxAge, m.defaultGraceLogins)
		m.expirations[normalizedDN] = exp
	} else {
		exp.SetChangedTime(changedTime)
	}
}

// UseGraceLogin attempts to use a grace login for a user.
// Returns true if successful, false if no grace logins available.
func (m *ExpirationManager) UseGraceLogin(dn string) bool {
	exp := m.GetExpiration(dn)
	if exp == nil {
		return false
	}
	return exp.UseGraceLogin()
}

// ListUsers returns a list of all DNs that have expiration tracking.
func (m *ExpirationManager) ListUsers() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	dns := make([]string, 0, len(m.expirations))
	for dn := range m.expirations {
		dns = append(dns, dn)
	}
	return dns
}

// UserCount returns the number of users with expiration tracking.
func (m *ExpirationManager) UserCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.expirations)
}

// ClearAll removes all expiration tracking.
func (m *ExpirationManager) ClearAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.expirations = make(map[string]*Expiration)
}

// SetDefaultMaxAge updates the default max age for new expirations.
func (m *ExpirationManager) SetDefaultMaxAge(maxAge time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.defaultMaxAge = maxAge
}

// DefaultMaxAge returns the default max age for new expirations.
func (m *ExpirationManager) DefaultMaxAge() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.defaultMaxAge
}

// SetDefaultGraceLogins updates the default grace logins for new expirations.
func (m *ExpirationManager) SetDefaultGraceLogins(count int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if count < 0 {
		count = 0
	}
	m.defaultGraceLogins = count
}

// DefaultGraceLogins returns the default grace logins for new expirations.
func (m *ExpirationManager) DefaultGraceLogins() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.defaultGraceLogins
}

// GetExpiredUsers returns a list of DNs with expired passwords.
func (m *ExpirationManager) GetExpiredUsers() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var expired []string
	for dn, exp := range m.expirations {
		if exp.IsExpired() {
			expired = append(expired, dn)
		}
	}
	return expired
}

// GetFullyExpiredUsers returns a list of DNs with fully expired passwords
// (expired and no grace logins remaining).
func (m *ExpirationManager) GetFullyExpiredUsers() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var expired []string
	for dn, exp := range m.expirations {
		if exp.IsFullyExpired() {
			expired = append(expired, dn)
		}
	}
	return expired
}
