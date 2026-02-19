// Package password provides password policy configuration and validation
// for the Oba LDAP server.
package password

import (
	"sync"
	"time"
)

// AccountLockout tracks failed authentication attempts and manages account lockout state.
// It provides thread-safe operations for recording failures, checking lock status,
// and managing automatic unlock after a configurable duration.
type AccountLockout struct {
	mu              sync.RWMutex
	failureTimes    []time.Time
	lockedTime      time.Time
	maxFailures     int
	lockoutDuration time.Duration
	failureWindow   time.Duration
}

// NewAccountLockout creates a new AccountLockout with the specified configuration.
// maxFailures: number of failures before lockout (0 = no lockout)
// lockoutDuration: how long the account stays locked (0 = permanent until manual unlock)
// failureWindow: time window for counting failures (0 = failures never expire)
func NewAccountLockout(maxFailures int, lockoutDuration, failureWindow time.Duration) *AccountLockout {
	return &AccountLockout{
		failureTimes:    make([]time.Time, 0),
		maxFailures:     maxFailures,
		lockoutDuration: lockoutDuration,
		failureWindow:   failureWindow,
	}
}

// RecordFailure records a failed authentication attempt.
// It removes old failures outside the failure window and checks if the account
// should be locked based on the number of recent failures.
func (l *AccountLockout) RecordFailure() {
	l.RecordFailureAt(time.Now())
}

// RecordFailureAt records a failed authentication attempt at a specific time.
// This is useful for testing and for replaying events.
func (l *AccountLockout) RecordFailureAt(now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.failureTimes = append(l.failureTimes, now)

	// Remove old failures outside the window
	if l.failureWindow > 0 {
		cutoff := now.Add(-l.failureWindow)
		l.pruneFailuresBeforeLocked(cutoff)
	}

	// Check if should lock
	if l.maxFailures > 0 && len(l.failureTimes) >= l.maxFailures {
		l.lockedTime = now
	}
}

// pruneFailuresBeforeLocked removes failures before the cutoff time.
// Must be called with lock held.
func (l *AccountLockout) pruneFailuresBeforeLocked(cutoff time.Time) {
	for len(l.failureTimes) > 0 && l.failureTimes[0].Before(cutoff) {
		l.failureTimes = l.failureTimes[1:]
	}
}

// IsLocked returns true if the account is currently locked.
// An account is locked if:
// - It has been locked (lockedTime is set)
// - AND either lockoutDuration is 0 (permanent lock) OR the lockout duration hasn't passed
func (l *AccountLockout) IsLocked() bool {
	return l.IsLockedAt(time.Now())
}

// IsLockedAt checks if the account is locked at a specific time.
// This is useful for testing and for checking historical state.
func (l *AccountLockout) IsLockedAt(now time.Time) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if l.lockedTime.IsZero() {
		return false
	}

	// Permanent lock if duration is 0
	if l.lockoutDuration == 0 {
		return true
	}

	// Check if lockout duration has passed
	return now.Sub(l.lockedTime) < l.lockoutDuration
}

// Unlock manually unlocks the account and clears all failure history.
func (l *AccountLockout) Unlock() {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.lockedTime = time.Time{}
	l.failureTimes = nil
}

// RecordSuccess records a successful authentication and clears all failure history.
// This should be called after a successful login to reset the failure counter.
func (l *AccountLockout) RecordSuccess() {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.failureTimes = nil
}

// FailureCount returns the current number of recorded failures within the window.
func (l *AccountLockout) FailureCount() int {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return len(l.failureTimes)
}

// FailureCountAt returns the number of failures at a specific time,
// considering the failure window.
func (l *AccountLockout) FailureCountAt(now time.Time) int {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if l.failureWindow == 0 {
		return len(l.failureTimes)
	}

	cutoff := now.Add(-l.failureWindow)
	count := 0
	for _, t := range l.failureTimes {
		if !t.Before(cutoff) {
			count++
		}
	}
	return count
}

// LockedTime returns the time when the account was locked.
// Returns zero time if the account has never been locked.
func (l *AccountLockout) LockedTime() time.Time {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return l.lockedTime
}

// FailureTimes returns a copy of the recorded failure times.
func (l *AccountLockout) FailureTimes() []time.Time {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if l.failureTimes == nil {
		return nil
	}

	result := make([]time.Time, len(l.failureTimes))
	copy(result, l.failureTimes)
	return result
}

// MaxFailures returns the maximum number of failures before lockout.
func (l *AccountLockout) MaxFailures() int {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return l.maxFailures
}

// LockoutDuration returns the lockout duration.
func (l *AccountLockout) LockoutDuration() time.Duration {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return l.lockoutDuration
}

// FailureWindow returns the failure window duration.
func (l *AccountLockout) FailureWindow() time.Duration {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return l.failureWindow
}

// SetMaxFailures updates the maximum number of failures before lockout.
func (l *AccountLockout) SetMaxFailures(maxFailures int) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.maxFailures = maxFailures
}

// SetLockoutDuration updates the lockout duration.
func (l *AccountLockout) SetLockoutDuration(duration time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.lockoutDuration = duration
}

// SetFailureWindow updates the failure window duration.
func (l *AccountLockout) SetFailureWindow(window time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.failureWindow = window
}

// RemainingLockoutTime returns the remaining time until the account is automatically unlocked.
// Returns 0 if the account is not locked or if the lockout is permanent.
func (l *AccountLockout) RemainingLockoutTime() time.Duration {
	return l.RemainingLockoutTimeAt(time.Now())
}

// RemainingLockoutTimeAt returns the remaining lockout time at a specific time.
func (l *AccountLockout) RemainingLockoutTimeAt(now time.Time) time.Duration {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if l.lockedTime.IsZero() {
		return 0
	}

	if l.lockoutDuration == 0 {
		return 0 // Permanent lock, no remaining time concept
	}

	elapsed := now.Sub(l.lockedTime)
	if elapsed >= l.lockoutDuration {
		return 0
	}

	return l.lockoutDuration - elapsed
}

// Clone creates a deep copy of the AccountLockout.
func (l *AccountLockout) Clone() *AccountLockout {
	l.mu.RLock()
	defer l.mu.RUnlock()

	clone := &AccountLockout{
		lockedTime:      l.lockedTime,
		maxFailures:     l.maxFailures,
		lockoutDuration: l.lockoutDuration,
		failureWindow:   l.failureWindow,
	}

	if l.failureTimes != nil {
		clone.failureTimes = make([]time.Time, len(l.failureTimes))
		copy(clone.failureTimes, l.failureTimes)
	}

	return clone
}

// Reset clears all state, including lock status and failure history.
func (l *AccountLockout) Reset() {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.lockedTime = time.Time{}
	l.failureTimes = nil
}
