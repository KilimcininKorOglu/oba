package password

import (
	"testing"
	"time"
)

// =============================================================================
// Expiration Tests
// =============================================================================

// TestNewExpiration tests expiration creation.
func TestNewExpiration(t *testing.T) {
	now := time.Now()
	maxAge := 90 * 24 * time.Hour
	graceLogins := 3

	e := NewExpiration(now, maxAge, graceLogins)
	if e == nil {
		t.Fatal("NewExpiration returned nil")
	}

	if !e.ChangedTime().Equal(now) {
		t.Error("ChangedTime mismatch")
	}
	if e.MaxAge() != maxAge {
		t.Errorf("MaxAge() = %v, want %v", e.MaxAge(), maxAge)
	}
	if e.GraceLogins() != graceLogins {
		t.Errorf("GraceLogins() = %d, want %d", e.GraceLogins(), graceLogins)
	}
	if e.GraceLoginsUsed() != 0 {
		t.Errorf("GraceLoginsUsed() = %d, want 0", e.GraceLoginsUsed())
	}
}

// TestNewExpirationNegativeGrace tests negative grace logins.
func TestNewExpirationNegativeGrace(t *testing.T) {
	e := NewExpiration(time.Now(), time.Hour, -5)
	if e.GraceLogins() != 0 {
		t.Errorf("negative grace logins should become 0, got %d", e.GraceLogins())
	}
}

// TestNewExpirationWithGraceUsed tests creating with pre-used grace logins.
func TestNewExpirationWithGraceUsed(t *testing.T) {
	e := NewExpirationWithGraceUsed(time.Now(), time.Hour, 5, 3)
	if e.GraceLoginsUsed() != 3 {
		t.Errorf("GraceLoginsUsed() = %d, want 3", e.GraceLoginsUsed())
	}
	if e.GraceLoginsRemaining() != 2 {
		t.Errorf("GraceLoginsRemaining() = %d, want 2", e.GraceLoginsRemaining())
	}

	// Test clamping
	e2 := NewExpirationWithGraceUsed(time.Now(), time.Hour, 3, 10)
	if e2.GraceLoginsUsed() != 3 {
		t.Errorf("GraceLoginsUsed() should be clamped to max, got %d", e2.GraceLoginsUsed())
	}
}

// TestExpirationIsExpired tests expiration checking.
func TestExpirationIsExpired(t *testing.T) {
	tests := []struct {
		name        string
		changedTime time.Time
		maxAge      time.Duration
		want        bool
	}{
		{
			name:        "not expired",
			changedTime: time.Now().Add(-30 * 24 * time.Hour),
			maxAge:      90 * 24 * time.Hour,
			want:        false,
		},
		{
			name:        "expired",
			changedTime: time.Now().Add(-100 * 24 * time.Hour),
			maxAge:      90 * 24 * time.Hour,
			want:        true,
		},
		{
			name:        "never expires (zero maxAge)",
			changedTime: time.Now().Add(-365 * 24 * time.Hour),
			maxAge:      0,
			want:        false,
		},
		{
			name:        "just expired",
			changedTime: time.Now().Add(-91 * 24 * time.Hour),
			maxAge:      90 * 24 * time.Hour,
			want:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewExpiration(tt.changedTime, tt.maxAge, 0)
			if got := e.IsExpired(); got != tt.want {
				t.Errorf("IsExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestExpirationIsExpiredAt tests expiration at specific time.
func TestExpirationIsExpiredAt(t *testing.T) {
	changedTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	maxAge := 30 * 24 * time.Hour
	e := NewExpiration(changedTime, maxAge, 0)

	// Before expiry
	beforeExpiry := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)
	if e.IsExpiredAt(beforeExpiry) {
		t.Error("should not be expired before maxAge")
	}

	// After expiry
	afterExpiry := time.Date(2025, 2, 15, 0, 0, 0, 0, time.UTC)
	if !e.IsExpiredAt(afterExpiry) {
		t.Error("should be expired after maxAge")
	}
}

// TestExpirationDaysUntilExpiry tests days calculation.
func TestExpirationDaysUntilExpiry(t *testing.T) {
	tests := []struct {
		name        string
		changedTime time.Time
		maxAge      time.Duration
		wantDays    int
	}{
		{
			name:        "never expires",
			changedTime: time.Now(),
			maxAge:      0,
			wantDays:    -1,
		},
		{
			name:        "already expired",
			changedTime: time.Now().Add(-100 * 24 * time.Hour),
			maxAge:      90 * 24 * time.Hour,
			wantDays:    0,
		},
		{
			name:        "60 days remaining",
			changedTime: time.Now().Add(-30 * 24 * time.Hour),
			maxAge:      90 * 24 * time.Hour,
			wantDays:    60,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewExpiration(tt.changedTime, tt.maxAge, 0)
			got := e.DaysUntilExpiry()
			// Allow 1 day tolerance for timing
			if tt.wantDays == -1 || tt.wantDays == 0 {
				if got != tt.wantDays {
					t.Errorf("DaysUntilExpiry() = %d, want %d", got, tt.wantDays)
				}
			} else {
				diff := got - tt.wantDays
				if diff < -1 || diff > 1 {
					t.Errorf("DaysUntilExpiry() = %d, want ~%d", got, tt.wantDays)
				}
			}
		})
	}
}

// TestExpirationTimeUntilExpiry tests duration calculation.
func TestExpirationTimeUntilExpiry(t *testing.T) {
	// Never expires
	e1 := NewExpiration(time.Now(), 0, 0)
	if e1.TimeUntilExpiry() != -1 {
		t.Errorf("TimeUntilExpiry() for never-expires = %v, want -1", e1.TimeUntilExpiry())
	}

	// Already expired
	e2 := NewExpiration(time.Now().Add(-100*24*time.Hour), 90*24*time.Hour, 0)
	if e2.TimeUntilExpiry() != 0 {
		t.Errorf("TimeUntilExpiry() for expired = %v, want 0", e2.TimeUntilExpiry())
	}

	// Not expired
	e3 := NewExpiration(time.Now(), 90*24*time.Hour, 0)
	remaining := e3.TimeUntilExpiry()
	if remaining <= 0 || remaining > 90*24*time.Hour {
		t.Errorf("TimeUntilExpiry() = %v, expected positive value <= 90 days", remaining)
	}
}

// TestExpirationExpiryTime tests expiry time calculation.
func TestExpirationExpiryTime(t *testing.T) {
	changedTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	maxAge := 30 * 24 * time.Hour

	e := NewExpiration(changedTime, maxAge, 0)
	expiryTime := e.ExpiryTime()

	expected := time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC)
	if !expiryTime.Equal(expected) {
		t.Errorf("ExpiryTime() = %v, want %v", expiryTime, expected)
	}

	// Never expires
	e2 := NewExpiration(changedTime, 0, 0)
	if !e2.ExpiryTime().IsZero() {
		t.Error("ExpiryTime() for never-expires should be zero")
	}
}

// TestExpirationSetChangedTime tests updating change time.
func TestExpirationSetChangedTime(t *testing.T) {
	e := NewExpirationWithGraceUsed(time.Now().Add(-100*24*time.Hour), 90*24*time.Hour, 5, 3)

	// Should be expired with grace used
	if !e.IsExpired() {
		t.Error("should be expired initially")
	}
	if e.GraceLoginsUsed() != 3 {
		t.Error("grace logins used should be 3")
	}

	// Update change time
	newTime := time.Now()
	e.SetChangedTime(newTime)

	// Should no longer be expired and grace reset
	if e.IsExpired() {
		t.Error("should not be expired after SetChangedTime")
	}
	if e.GraceLoginsUsed() != 0 {
		t.Error("grace logins should be reset after SetChangedTime")
	}
}

// TestExpirationGraceLogins tests grace login functionality.
func TestExpirationGraceLogins(t *testing.T) {
	// Create expired password with 3 grace logins
	e := NewExpiration(time.Now().Add(-100*24*time.Hour), 90*24*time.Hour, 3)

	if !e.IsExpired() {
		t.Fatal("password should be expired")
	}

	// Use grace logins
	for i := 0; i < 3; i++ {
		if !e.CanUseGraceLogin() {
			t.Errorf("should be able to use grace login %d", i+1)
		}
		if !e.UseGraceLogin() {
			t.Errorf("UseGraceLogin() should succeed for login %d", i+1)
		}
		if e.GraceLoginsUsed() != i+1 {
			t.Errorf("GraceLoginsUsed() = %d, want %d", e.GraceLoginsUsed(), i+1)
		}
	}

	// No more grace logins
	if e.CanUseGraceLogin() {
		t.Error("should not be able to use grace login after exhausted")
	}
	if e.UseGraceLogin() {
		t.Error("UseGraceLogin() should fail when exhausted")
	}
}

// TestExpirationGraceLoginNotExpired tests grace login on non-expired password.
func TestExpirationGraceLoginNotExpired(t *testing.T) {
	e := NewExpiration(time.Now(), 90*24*time.Hour, 3)

	if e.IsExpired() {
		t.Fatal("password should not be expired")
	}

	if e.CanUseGraceLogin() {
		t.Error("should not be able to use grace login when not expired")
	}
	if e.UseGraceLogin() {
		t.Error("UseGraceLogin() should fail when not expired")
	}
}

// TestExpirationIsFullyExpired tests fully expired check.
func TestExpirationIsFullyExpired(t *testing.T) {
	// Not expired
	e1 := NewExpiration(time.Now(), 90*24*time.Hour, 3)
	if e1.IsFullyExpired() {
		t.Error("not expired password should not be fully expired")
	}

	// Expired with grace remaining
	e2 := NewExpiration(time.Now().Add(-100*24*time.Hour), 90*24*time.Hour, 3)
	if e2.IsFullyExpired() {
		t.Error("expired with grace remaining should not be fully expired")
	}

	// Fully expired
	e3 := NewExpirationWithGraceUsed(time.Now().Add(-100*24*time.Hour), 90*24*time.Hour, 3, 3)
	if !e3.IsFullyExpired() {
		t.Error("expired with no grace remaining should be fully expired")
	}

	// Expired with zero grace logins
	e4 := NewExpiration(time.Now().Add(-100*24*time.Hour), 90*24*time.Hour, 0)
	if !e4.IsFullyExpired() {
		t.Error("expired with zero grace logins should be fully expired")
	}
}

// TestExpirationStatus tests status calculation.
func TestExpirationStatus(t *testing.T) {
	warningPeriod := 14 * 24 * time.Hour

	tests := []struct {
		name        string
		changedTime time.Time
		maxAge      time.Duration
		graceLogins int
		graceUsed   int
		want        ExpirationStatus
	}{
		{
			name:        "valid (never expires)",
			changedTime: time.Now().Add(-365 * 24 * time.Hour),
			maxAge:      0,
			graceLogins: 0,
			graceUsed:   0,
			want:        StatusValid,
		},
		{
			name:        "valid (not expiring soon)",
			changedTime: time.Now().Add(-30 * 24 * time.Hour),
			maxAge:      90 * 24 * time.Hour,
			graceLogins: 3,
			graceUsed:   0,
			want:        StatusValid,
		},
		{
			name:        "expiring soon",
			changedTime: time.Now().Add(-80 * 24 * time.Hour),
			maxAge:      90 * 24 * time.Hour,
			graceLogins: 3,
			graceUsed:   0,
			want:        StatusExpiring,
		},
		{
			name:        "expired with grace",
			changedTime: time.Now().Add(-100 * 24 * time.Hour),
			maxAge:      90 * 24 * time.Hour,
			graceLogins: 3,
			graceUsed:   1,
			want:        StatusExpiredGrace,
		},
		{
			name:        "expired locked",
			changedTime: time.Now().Add(-100 * 24 * time.Hour),
			maxAge:      90 * 24 * time.Hour,
			graceLogins: 3,
			graceUsed:   3,
			want:        StatusExpiredLocked,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewExpirationWithGraceUsed(tt.changedTime, tt.maxAge, tt.graceLogins, tt.graceUsed)
			got := e.Status(warningPeriod)
			if got != tt.want {
				t.Errorf("Status() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestExpirationStatusString tests status string conversion.
func TestExpirationStatusString(t *testing.T) {
	tests := []struct {
		status ExpirationStatus
		want   string
	}{
		{StatusValid, "valid"},
		{StatusExpiring, "expiring"},
		{StatusExpiredGrace, "expired_grace"},
		{StatusExpiredLocked, "expired_locked"},
		{ExpirationStatus(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.status.String(); got != tt.want {
			t.Errorf("ExpirationStatus(%d).String() = %s, want %s", tt.status, got, tt.want)
		}
	}
}

// TestExpirationClone tests cloning expiration.
func TestExpirationClone(t *testing.T) {
	// Create an expired password so UseGraceLogin will work
	original := NewExpirationWithGraceUsed(time.Now().Add(-100*24*time.Hour), 90*24*time.Hour, 5, 2)

	clone := original.Clone()

	// Verify values match
	if !clone.ChangedTime().Equal(original.ChangedTime()) {
		t.Error("clone ChangedTime mismatch")
	}
	if clone.MaxAge() != original.MaxAge() {
		t.Error("clone MaxAge mismatch")
	}
	if clone.GraceLogins() != original.GraceLogins() {
		t.Error("clone GraceLogins mismatch")
	}
	if clone.GraceLoginsUsed() != original.GraceLoginsUsed() {
		t.Error("clone GraceLoginsUsed mismatch")
	}

	// Modify clone and verify original unchanged
	clone.UseGraceLogin()
	if original.GraceLoginsUsed() == clone.GraceLoginsUsed() {
		t.Error("modifying clone affected original")
	}
}

// TestExpirationResetGraceLogins tests resetting grace logins.
func TestExpirationResetGraceLogins(t *testing.T) {
	e := NewExpirationWithGraceUsed(time.Now().Add(-100*24*time.Hour), 90*24*time.Hour, 5, 3)

	if e.GraceLoginsUsed() != 3 {
		t.Fatal("initial grace used should be 3")
	}

	e.ResetGraceLogins()

	if e.GraceLoginsUsed() != 0 {
		t.Errorf("GraceLoginsUsed() after reset = %d, want 0", e.GraceLoginsUsed())
	}
}

// =============================================================================
// ExpirationManager Tests
// =============================================================================

// TestNewExpirationManager tests manager creation.
func TestNewExpirationManager(t *testing.T) {
	m := NewExpirationManager(90*24*time.Hour, 3)
	if m == nil {
		t.Fatal("NewExpirationManager returned nil")
	}
	if m.DefaultMaxAge() != 90*24*time.Hour {
		t.Errorf("DefaultMaxAge() = %v, want 90 days", m.DefaultMaxAge())
	}
	if m.DefaultGraceLogins() != 3 {
		t.Errorf("DefaultGraceLogins() = %d, want 3", m.DefaultGraceLogins())
	}
	if m.UserCount() != 0 {
		t.Errorf("UserCount() = %d, want 0", m.UserCount())
	}
}

// TestExpirationManagerGetExpiration tests getting user expiration.
func TestExpirationManagerGetExpiration(t *testing.T) {
	m := NewExpirationManager(90*24*time.Hour, 3)
	dn := "uid=alice,dc=example,dc=com"

	// Non-existent returns nil
	if m.GetExpiration(dn) != nil {
		t.Error("GetExpiration should return nil for non-existent")
	}

	// GetOrCreate creates new
	e := m.GetOrCreateExpiration(dn)
	if e == nil {
		t.Fatal("GetOrCreateExpiration returned nil")
	}

	// Now GetExpiration returns it
	if m.GetExpiration(dn) == nil {
		t.Error("GetExpiration should return expiration after GetOrCreate")
	}
}

// TestExpirationManagerSetExpiration tests setting user expiration.
func TestExpirationManagerSetExpiration(t *testing.T) {
	m := NewExpirationManager(90*24*time.Hour, 3)
	dn := "uid=bob,dc=example,dc=com"

	e := NewExpiration(time.Now().Add(-50*24*time.Hour), 60*24*time.Hour, 5)
	m.SetExpiration(dn, e)

	retrieved := m.GetExpiration(dn)
	if retrieved == nil {
		t.Fatal("SetExpiration should store the expiration")
	}
	if retrieved.MaxAge() != 60*24*time.Hour {
		t.Error("stored expiration has wrong MaxAge")
	}

	// Set nil removes
	m.SetExpiration(dn, nil)
	if m.HasExpiration(dn) {
		t.Error("SetExpiration(nil) should remove expiration")
	}
}

// TestExpirationManagerIsExpired tests checking expiration.
func TestExpirationManagerIsExpired(t *testing.T) {
	m := NewExpirationManager(90*24*time.Hour, 3)
	dn := "uid=charlie,dc=example,dc=com"

	// Non-existent user
	if m.IsExpired(dn) {
		t.Error("IsExpired should return false for non-existent user")
	}

	// Not expired
	m.RecordPasswordChange(dn, time.Now())
	if m.IsExpired(dn) {
		t.Error("IsExpired should return false for fresh password")
	}

	// Expired
	m.RecordPasswordChange(dn, time.Now().Add(-100*24*time.Hour))
	if !m.IsExpired(dn) {
		t.Error("IsExpired should return true for old password")
	}
}

// TestExpirationManagerIsFullyExpired tests fully expired check.
func TestExpirationManagerIsFullyExpired(t *testing.T) {
	m := NewExpirationManager(90*24*time.Hour, 3)
	dn := "uid=dave,dc=example,dc=com"

	// Non-existent user
	if m.IsFullyExpired(dn) {
		t.Error("IsFullyExpired should return false for non-existent user")
	}

	// Expired with grace
	e := NewExpiration(time.Now().Add(-100*24*time.Hour), 90*24*time.Hour, 3)
	m.SetExpiration(dn, e)
	if m.IsFullyExpired(dn) {
		t.Error("IsFullyExpired should return false when grace available")
	}

	// Fully expired
	e2 := NewExpirationWithGraceUsed(time.Now().Add(-100*24*time.Hour), 90*24*time.Hour, 3, 3)
	m.SetExpiration(dn, e2)
	if !m.IsFullyExpired(dn) {
		t.Error("IsFullyExpired should return true when grace exhausted")
	}
}

// TestExpirationManagerRecordPasswordChange tests recording changes.
func TestExpirationManagerRecordPasswordChange(t *testing.T) {
	m := NewExpirationManager(90*24*time.Hour, 3)
	dn := "uid=eve,dc=example,dc=com"

	// First change creates expiration
	changeTime := time.Now()
	m.RecordPasswordChange(dn, changeTime)

	e := m.GetExpiration(dn)
	if e == nil {
		t.Fatal("RecordPasswordChange should create expiration")
	}
	if !e.ChangedTime().Equal(changeTime) {
		t.Error("ChangedTime mismatch")
	}

	// Second change updates existing
	newChangeTime := time.Now().Add(time.Hour)
	m.RecordPasswordChange(dn, newChangeTime)

	e = m.GetExpiration(dn)
	if !e.ChangedTime().Equal(newChangeTime) {
		t.Error("RecordPasswordChange should update ChangedTime")
	}
}

// TestExpirationManagerUseGraceLogin tests using grace logins.
func TestExpirationManagerUseGraceLogin(t *testing.T) {
	m := NewExpirationManager(90*24*time.Hour, 3)
	dn := "uid=frank,dc=example,dc=com"

	// Non-existent user
	if m.UseGraceLogin(dn) {
		t.Error("UseGraceLogin should return false for non-existent user")
	}

	// Expired user with grace
	e := NewExpiration(time.Now().Add(-100*24*time.Hour), 90*24*time.Hour, 3)
	m.SetExpiration(dn, e)

	if !m.UseGraceLogin(dn) {
		t.Error("UseGraceLogin should succeed for expired user with grace")
	}
}

// TestExpirationManagerGetExpiredUsers tests getting expired users.
func TestExpirationManagerGetExpiredUsers(t *testing.T) {
	m := NewExpirationManager(90*24*time.Hour, 3)

	// Add some users
	m.RecordPasswordChange("uid=fresh,dc=example,dc=com", time.Now())
	m.RecordPasswordChange("uid=old1,dc=example,dc=com", time.Now().Add(-100*24*time.Hour))
	m.RecordPasswordChange("uid=old2,dc=example,dc=com", time.Now().Add(-100*24*time.Hour))

	expired := m.GetExpiredUsers()
	if len(expired) != 2 {
		t.Errorf("GetExpiredUsers() returned %d users, want 2", len(expired))
	}
}

// TestExpirationManagerGetFullyExpiredUsers tests getting fully expired users.
func TestExpirationManagerGetFullyExpiredUsers(t *testing.T) {
	m := NewExpirationManager(90*24*time.Hour, 3)

	// Fresh user
	m.RecordPasswordChange("uid=fresh,dc=example,dc=com", time.Now())

	// Expired with grace
	e1 := NewExpiration(time.Now().Add(-100*24*time.Hour), 90*24*time.Hour, 3)
	m.SetExpiration("uid=grace,dc=example,dc=com", e1)

	// Fully expired
	e2 := NewExpirationWithGraceUsed(time.Now().Add(-100*24*time.Hour), 90*24*time.Hour, 3, 3)
	m.SetExpiration("uid=locked,dc=example,dc=com", e2)

	fullyExpired := m.GetFullyExpiredUsers()
	if len(fullyExpired) != 1 {
		t.Errorf("GetFullyExpiredUsers() returned %d users, want 1", len(fullyExpired))
	}
}

// TestExpirationManagerListUsers tests listing users.
func TestExpirationManagerListUsers(t *testing.T) {
	m := NewExpirationManager(90*24*time.Hour, 3)

	dns := []string{
		"uid=user1,dc=example,dc=com",
		"uid=user2,dc=example,dc=com",
		"uid=user3,dc=example,dc=com",
	}

	for _, dn := range dns {
		m.RecordPasswordChange(dn, time.Now())
	}

	users := m.ListUsers()
	if len(users) != 3 {
		t.Errorf("ListUsers() returned %d users, want 3", len(users))
	}
}

// TestExpirationManagerClearAll tests clearing all expirations.
func TestExpirationManagerClearAll(t *testing.T) {
	m := NewExpirationManager(90*24*time.Hour, 3)

	m.RecordPasswordChange("uid=user1,dc=example,dc=com", time.Now())
	m.RecordPasswordChange("uid=user2,dc=example,dc=com", time.Now())

	m.ClearAll()

	if m.UserCount() != 0 {
		t.Errorf("UserCount() after ClearAll() = %d, want 0", m.UserCount())
	}
}

// TestExpirationManagerDNCaseInsensitivity tests case-insensitive DN handling.
func TestExpirationManagerDNCaseInsensitivity(t *testing.T) {
	m := NewExpirationManager(90*24*time.Hour, 3)

	dn := "uid=Alice,ou=Users,dc=Example,dc=COM"
	m.RecordPasswordChange(dn, time.Now())

	variations := []string{
		"uid=alice,ou=users,dc=example,dc=com",
		"UID=ALICE,OU=USERS,DC=EXAMPLE,DC=COM",
		"  uid=alice,ou=users,dc=example,dc=com  ",
	}

	for _, v := range variations {
		if !m.HasExpiration(v) {
			t.Errorf("HasExpiration should work for DN variation: %s", v)
		}
	}
}

// TestExpirationManagerConcurrency tests concurrent access.
func TestExpirationManagerConcurrency(t *testing.T) {
	m := NewExpirationManager(90*24*time.Hour, 3)
	done := make(chan bool)

	// Concurrent writers
	for i := 0; i < 10; i++ {
		go func(id int) {
			dn := "uid=user" + string(rune('0'+id)) + ",dc=example,dc=com"
			for j := 0; j < 100; j++ {
				m.RecordPasswordChange(dn, time.Now())
			}
			done <- true
		}(i)
	}

	// Concurrent readers
	for i := 0; i < 10; i++ {
		go func(id int) {
			dn := "uid=user" + string(rune('0'+id)) + ",dc=example,dc=com"
			for j := 0; j < 100; j++ {
				_ = m.IsExpired(dn)
				_ = m.HasExpiration(dn)
				_ = m.ListUsers()
				_ = m.GetExpiredUsers()
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 20; i++ {
		<-done
	}
}
