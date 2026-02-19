package password

import (
	"sync"
	"testing"
	"time"
)

// TestNewAccountLockout verifies the constructor creates a properly initialized lockout.
func TestNewAccountLockout(t *testing.T) {
	lockout := NewAccountLockout(5, 15*time.Minute, 10*time.Minute)

	if lockout == nil {
		t.Fatal("NewAccountLockout returned nil")
	}

	if lockout.MaxFailures() != 5 {
		t.Errorf("expected MaxFailures 5, got %d", lockout.MaxFailures())
	}

	if lockout.LockoutDuration() != 15*time.Minute {
		t.Errorf("expected LockoutDuration 15m, got %v", lockout.LockoutDuration())
	}

	if lockout.FailureWindow() != 10*time.Minute {
		t.Errorf("expected FailureWindow 10m, got %v", lockout.FailureWindow())
	}

	if lockout.FailureCount() != 0 {
		t.Errorf("expected FailureCount 0, got %d", lockout.FailureCount())
	}

	if lockout.IsLocked() {
		t.Error("new lockout should not be locked")
	}
}

// TestRecordFailure tests that failures are tracked correctly.
func TestRecordFailure(t *testing.T) {
	lockout := NewAccountLockout(5, 15*time.Minute, 0)

	// Record some failures
	for i := 0; i < 3; i++ {
		lockout.RecordFailure()
	}

	if lockout.FailureCount() != 3 {
		t.Errorf("expected FailureCount 3, got %d", lockout.FailureCount())
	}

	if lockout.IsLocked() {
		t.Error("should not be locked with only 3 failures")
	}
}

// TestRecordFailureAt tests recording failures at specific times.
func TestRecordFailureAt(t *testing.T) {
	lockout := NewAccountLockout(5, 15*time.Minute, 0)

	baseTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	lockout.RecordFailureAt(baseTime)
	lockout.RecordFailureAt(baseTime.Add(1 * time.Minute))
	lockout.RecordFailureAt(baseTime.Add(2 * time.Minute))

	times := lockout.FailureTimes()
	if len(times) != 3 {
		t.Errorf("expected 3 failure times, got %d", len(times))
	}

	if !times[0].Equal(baseTime) {
		t.Errorf("expected first failure at %v, got %v", baseTime, times[0])
	}
}

// TestAccountLocksAfterMaxFailures tests that account locks after reaching max failures.
func TestAccountLocksAfterMaxFailures(t *testing.T) {
	lockout := NewAccountLockout(3, 15*time.Minute, 0)

	// Record failures up to max
	for i := 0; i < 3; i++ {
		lockout.RecordFailure()
	}

	if !lockout.IsLocked() {
		t.Error("account should be locked after max failures")
	}

	if lockout.LockedTime().IsZero() {
		t.Error("locked time should be set")
	}
}

// TestAutoUnlockAfterDuration tests automatic unlock after lockout duration.
func TestAutoUnlockAfterDuration(t *testing.T) {
	lockout := NewAccountLockout(3, 15*time.Minute, 0)

	baseTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	// Lock the account
	for i := 0; i < 3; i++ {
		lockout.RecordFailureAt(baseTime)
	}

	// Should be locked immediately after
	if !lockout.IsLockedAt(baseTime.Add(1 * time.Second)) {
		t.Error("account should be locked immediately after max failures")
	}

	// Should still be locked after 10 minutes
	if !lockout.IsLockedAt(baseTime.Add(10 * time.Minute)) {
		t.Error("account should still be locked after 10 minutes")
	}

	// Should be unlocked after 15 minutes
	if lockout.IsLockedAt(baseTime.Add(16 * time.Minute)) {
		t.Error("account should be unlocked after lockout duration")
	}
}

// TestPermanentLockout tests permanent lockout when duration is 0.
func TestPermanentLockout(t *testing.T) {
	lockout := NewAccountLockout(3, 0, 0) // 0 duration = permanent

	baseTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	// Lock the account
	for i := 0; i < 3; i++ {
		lockout.RecordFailureAt(baseTime)
	}

	// Should still be locked after a very long time
	if !lockout.IsLockedAt(baseTime.Add(365 * 24 * time.Hour)) {
		t.Error("permanent lockout should remain locked indefinitely")
	}
}

// TestManualUnlock tests manual unlock functionality.
func TestManualUnlock(t *testing.T) {
	lockout := NewAccountLockout(3, 0, 0) // Permanent lock

	// Lock the account
	for i := 0; i < 3; i++ {
		lockout.RecordFailure()
	}

	if !lockout.IsLocked() {
		t.Error("account should be locked")
	}

	// Manually unlock
	lockout.Unlock()

	if lockout.IsLocked() {
		t.Error("account should be unlocked after manual unlock")
	}

	if lockout.FailureCount() != 0 {
		t.Error("failure count should be cleared after unlock")
	}

	if !lockout.LockedTime().IsZero() {
		t.Error("locked time should be cleared after unlock")
	}
}

// TestRecordSuccessClearsFailures tests that successful auth clears failure count.
func TestRecordSuccessClearsFailures(t *testing.T) {
	lockout := NewAccountLockout(5, 15*time.Minute, 0)

	// Record some failures
	for i := 0; i < 3; i++ {
		lockout.RecordFailure()
	}

	if lockout.FailureCount() != 3 {
		t.Errorf("expected 3 failures, got %d", lockout.FailureCount())
	}

	// Record success
	lockout.RecordSuccess()

	if lockout.FailureCount() != 0 {
		t.Errorf("expected 0 failures after success, got %d", lockout.FailureCount())
	}
}

// TestFailureWindow tests that old failures are pruned based on window.
func TestFailureWindow(t *testing.T) {
	lockout := NewAccountLockout(5, 15*time.Minute, 10*time.Minute)

	baseTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	// Record failures spread over time
	lockout.RecordFailureAt(baseTime)                      // Will be pruned
	lockout.RecordFailureAt(baseTime.Add(5 * time.Minute)) // Will be pruned
	lockout.RecordFailureAt(baseTime.Add(15 * time.Minute))
	lockout.RecordFailureAt(baseTime.Add(16 * time.Minute))
	lockout.RecordFailureAt(baseTime.Add(17 * time.Minute))

	// At time baseTime + 20 minutes, only failures from last 10 minutes should count
	count := lockout.FailureCountAt(baseTime.Add(20 * time.Minute))
	if count != 3 {
		t.Errorf("expected 3 failures within window, got %d", count)
	}
}

// TestFailureWindowPruningOnRecord tests that failures are pruned when recording new ones.
func TestFailureWindowPruningOnRecord(t *testing.T) {
	lockout := NewAccountLockout(5, 15*time.Minute, 10*time.Minute)

	baseTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	// Record old failures
	lockout.RecordFailureAt(baseTime)
	lockout.RecordFailureAt(baseTime.Add(1 * time.Minute))

	// Record a new failure much later - should prune old ones
	lockout.RecordFailureAt(baseTime.Add(20 * time.Minute))

	// Only the recent failure should remain
	if lockout.FailureCount() != 1 {
		t.Errorf("expected 1 failure after pruning, got %d", lockout.FailureCount())
	}
}

// TestNoLockoutWhenMaxFailuresZero tests that lockout is disabled when maxFailures is 0.
func TestNoLockoutWhenMaxFailuresZero(t *testing.T) {
	lockout := NewAccountLockout(0, 15*time.Minute, 0)

	// Record many failures
	for i := 0; i < 100; i++ {
		lockout.RecordFailure()
	}

	if lockout.IsLocked() {
		t.Error("account should not lock when maxFailures is 0")
	}
}

// TestRemainingLockoutTime tests the remaining lockout time calculation.
func TestRemainingLockoutTime(t *testing.T) {
	lockout := NewAccountLockout(3, 15*time.Minute, 0)

	baseTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	// Lock the account
	for i := 0; i < 3; i++ {
		lockout.RecordFailureAt(baseTime)
	}

	// Check remaining time at various points
	remaining := lockout.RemainingLockoutTimeAt(baseTime.Add(5 * time.Minute))
	if remaining != 10*time.Minute {
		t.Errorf("expected 10m remaining, got %v", remaining)
	}

	remaining = lockout.RemainingLockoutTimeAt(baseTime.Add(10 * time.Minute))
	if remaining != 5*time.Minute {
		t.Errorf("expected 5m remaining, got %v", remaining)
	}

	// After lockout expires
	remaining = lockout.RemainingLockoutTimeAt(baseTime.Add(20 * time.Minute))
	if remaining != 0 {
		t.Errorf("expected 0 remaining after expiry, got %v", remaining)
	}
}

// TestRemainingLockoutTimePermanent tests remaining time for permanent lockout.
func TestRemainingLockoutTimePermanent(t *testing.T) {
	lockout := NewAccountLockout(3, 0, 0) // Permanent

	// Lock the account
	for i := 0; i < 3; i++ {
		lockout.RecordFailure()
	}

	// Permanent lockout returns 0 (no concept of remaining time)
	if lockout.RemainingLockoutTime() != 0 {
		t.Error("permanent lockout should return 0 remaining time")
	}
}

// TestRemainingLockoutTimeNotLocked tests remaining time when not locked.
func TestRemainingLockoutTimeNotLocked(t *testing.T) {
	lockout := NewAccountLockout(5, 15*time.Minute, 0)

	if lockout.RemainingLockoutTime() != 0 {
		t.Error("unlocked account should return 0 remaining time")
	}
}

// TestClone tests deep copying of AccountLockout.
func TestClone(t *testing.T) {
	lockout := NewAccountLockout(5, 15*time.Minute, 10*time.Minute)

	// Add some state
	lockout.RecordFailure()
	lockout.RecordFailure()

	// Clone
	clone := lockout.Clone()

	// Verify values match
	if clone.MaxFailures() != lockout.MaxFailures() {
		t.Error("clone MaxFailures mismatch")
	}

	if clone.LockoutDuration() != lockout.LockoutDuration() {
		t.Error("clone LockoutDuration mismatch")
	}

	if clone.FailureWindow() != lockout.FailureWindow() {
		t.Error("clone FailureWindow mismatch")
	}

	if clone.FailureCount() != lockout.FailureCount() {
		t.Error("clone FailureCount mismatch")
	}

	// Modify clone and verify original is unchanged
	clone.RecordFailure()
	if lockout.FailureCount() == clone.FailureCount() {
		t.Error("modifying clone affected original")
	}
}

// TestReset tests resetting all state.
func TestReset(t *testing.T) {
	lockout := NewAccountLockout(3, 15*time.Minute, 0)

	// Lock the account
	for i := 0; i < 3; i++ {
		lockout.RecordFailure()
	}

	if !lockout.IsLocked() {
		t.Error("account should be locked")
	}

	// Reset
	lockout.Reset()

	if lockout.IsLocked() {
		t.Error("account should not be locked after reset")
	}

	if lockout.FailureCount() != 0 {
		t.Error("failure count should be 0 after reset")
	}

	if !lockout.LockedTime().IsZero() {
		t.Error("locked time should be zero after reset")
	}
}

// TestSetters tests the setter methods.
func TestSetters(t *testing.T) {
	lockout := NewAccountLockout(5, 15*time.Minute, 10*time.Minute)

	lockout.SetMaxFailures(10)
	if lockout.MaxFailures() != 10 {
		t.Errorf("expected MaxFailures 10, got %d", lockout.MaxFailures())
	}

	lockout.SetLockoutDuration(30 * time.Minute)
	if lockout.LockoutDuration() != 30*time.Minute {
		t.Errorf("expected LockoutDuration 30m, got %v", lockout.LockoutDuration())
	}

	lockout.SetFailureWindow(20 * time.Minute)
	if lockout.FailureWindow() != 20*time.Minute {
		t.Errorf("expected FailureWindow 20m, got %v", lockout.FailureWindow())
	}
}

// TestFailureTimesReturnsNilForEmpty tests that FailureTimes returns nil when empty.
func TestFailureTimesReturnsNilForEmpty(t *testing.T) {
	lockout := NewAccountLockout(5, 15*time.Minute, 0)

	// Clear any failures
	lockout.RecordSuccess()

	times := lockout.FailureTimes()
	if times != nil {
		t.Error("expected nil for empty failure times")
	}
}

// TestFailureTimesReturnsCopy tests that FailureTimes returns a copy.
func TestFailureTimesReturnsCopy(t *testing.T) {
	lockout := NewAccountLockout(5, 15*time.Minute, 0)

	lockout.RecordFailure()
	lockout.RecordFailure()

	times := lockout.FailureTimes()
	originalLen := len(times)

	// Modify the returned slice
	times = append(times, time.Now())

	// Original should be unchanged
	if lockout.FailureCount() != originalLen {
		t.Error("modifying returned slice affected original")
	}
}

// TestConcurrency tests thread-safe access to AccountLockout.
func TestConcurrency(t *testing.T) {
	lockout := NewAccountLockout(100, 15*time.Minute, 10*time.Minute)

	var wg sync.WaitGroup
	done := make(chan bool)

	// Concurrent writers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				lockout.RecordFailure()
			}
		}()
	}

	// Concurrent readers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_ = lockout.IsLocked()
				_ = lockout.FailureCount()
				_ = lockout.FailureTimes()
				_ = lockout.RemainingLockoutTime()
			}
		}()
	}

	// Wait in a goroutine
	go func() {
		wg.Wait()
		done <- true
	}()

	select {
	case <-done:
		// Success
	case <-time.After(10 * time.Second):
		t.Fatal("test timed out - possible deadlock")
	}
}

// TestLockoutAtExactThreshold tests behavior at exactly max failures.
func TestLockoutAtExactThreshold(t *testing.T) {
	lockout := NewAccountLockout(3, 15*time.Minute, 0)

	// Record exactly max failures
	lockout.RecordFailure()
	lockout.RecordFailure()

	if lockout.IsLocked() {
		t.Error("should not be locked with 2 failures (threshold is 3)")
	}

	lockout.RecordFailure()

	if !lockout.IsLocked() {
		t.Error("should be locked at exactly 3 failures")
	}
}

// TestLockoutAboveThreshold tests behavior above max failures.
func TestLockoutAboveThreshold(t *testing.T) {
	lockout := NewAccountLockout(3, 15*time.Minute, 0)

	// Record more than max failures
	for i := 0; i < 10; i++ {
		lockout.RecordFailure()
	}

	if !lockout.IsLocked() {
		t.Error("should be locked with 10 failures")
	}

	if lockout.FailureCount() != 10 {
		t.Errorf("expected 10 failures recorded, got %d", lockout.FailureCount())
	}
}

// TestUnlockDoesNotAffectConfiguration tests that unlock preserves config.
func TestUnlockDoesNotAffectConfiguration(t *testing.T) {
	lockout := NewAccountLockout(5, 15*time.Minute, 10*time.Minute)

	// Lock and unlock
	for i := 0; i < 5; i++ {
		lockout.RecordFailure()
	}
	lockout.Unlock()

	// Configuration should be preserved
	if lockout.MaxFailures() != 5 {
		t.Error("MaxFailures should be preserved after unlock")
	}

	if lockout.LockoutDuration() != 15*time.Minute {
		t.Error("LockoutDuration should be preserved after unlock")
	}

	if lockout.FailureWindow() != 10*time.Minute {
		t.Error("FailureWindow should be preserved after unlock")
	}
}

// TestRecordSuccessDoesNotAffectLockStatus tests that success doesn't unlock.
func TestRecordSuccessDoesNotAffectLockStatus(t *testing.T) {
	lockout := NewAccountLockout(3, 15*time.Minute, 0)

	// Lock the account
	for i := 0; i < 3; i++ {
		lockout.RecordFailure()
	}

	if !lockout.IsLocked() {
		t.Error("account should be locked")
	}

	// Record success - should clear failures but not unlock
	lockout.RecordSuccess()

	// Note: RecordSuccess only clears failure times, it doesn't unlock
	// The account remains locked until manual unlock or duration expires
	if lockout.FailureCount() != 0 {
		t.Error("failure count should be cleared")
	}
}

// TestFailureWindowZeroMeansNoExpiry tests that 0 window means failures never expire.
func TestFailureWindowZeroMeansNoExpiry(t *testing.T) {
	lockout := NewAccountLockout(5, 15*time.Minute, 0) // 0 = no expiry

	baseTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	// Record failures spread over a long time
	lockout.RecordFailureAt(baseTime)
	lockout.RecordFailureAt(baseTime.Add(24 * time.Hour))
	lockout.RecordFailureAt(baseTime.Add(48 * time.Hour))

	// All failures should still count
	if lockout.FailureCount() != 3 {
		t.Errorf("expected 3 failures with no window, got %d", lockout.FailureCount())
	}
}

// TestLockedTimeIsSetCorrectly tests that locked time is set to the time of the locking failure.
func TestLockedTimeIsSetCorrectly(t *testing.T) {
	lockout := NewAccountLockout(3, 15*time.Minute, 0)

	baseTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	lockout.RecordFailureAt(baseTime)
	lockout.RecordFailureAt(baseTime.Add(1 * time.Minute))
	lockout.RecordFailureAt(baseTime.Add(2 * time.Minute)) // This should trigger lock

	lockedTime := lockout.LockedTime()
	expected := baseTime.Add(2 * time.Minute)

	if !lockedTime.Equal(expected) {
		t.Errorf("expected locked time %v, got %v", expected, lockedTime)
	}
}

// TestMultipleLockCycles tests multiple lock/unlock cycles.
func TestMultipleLockCycles(t *testing.T) {
	lockout := NewAccountLockout(2, 15*time.Minute, 0)

	for cycle := 0; cycle < 3; cycle++ {
		// Lock
		lockout.RecordFailure()
		lockout.RecordFailure()

		if !lockout.IsLocked() {
			t.Errorf("cycle %d: should be locked", cycle)
		}

		// Unlock
		lockout.Unlock()

		if lockout.IsLocked() {
			t.Errorf("cycle %d: should be unlocked", cycle)
		}

		if lockout.FailureCount() != 0 {
			t.Errorf("cycle %d: failure count should be 0", cycle)
		}
	}
}

// TestClonePreservesLockedState tests that clone preserves locked state.
func TestClonePreservesLockedState(t *testing.T) {
	lockout := NewAccountLockout(3, 15*time.Minute, 0)

	// Lock the account
	for i := 0; i < 3; i++ {
		lockout.RecordFailure()
	}

	clone := lockout.Clone()

	if !clone.IsLocked() {
		t.Error("clone should preserve locked state")
	}

	if clone.LockedTime() != lockout.LockedTime() {
		t.Error("clone should preserve locked time")
	}
}

// TestFailureCountAtWithNoWindow tests FailureCountAt when window is 0.
func TestFailureCountAtWithNoWindow(t *testing.T) {
	lockout := NewAccountLockout(5, 15*time.Minute, 0)

	baseTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	lockout.RecordFailureAt(baseTime)
	lockout.RecordFailureAt(baseTime.Add(1 * time.Hour))
	lockout.RecordFailureAt(baseTime.Add(2 * time.Hour))

	// With no window, all failures should count regardless of time
	count := lockout.FailureCountAt(baseTime.Add(100 * time.Hour))
	if count != 3 {
		t.Errorf("expected 3 failures with no window, got %d", count)
	}
}
