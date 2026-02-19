package password

import (
	"testing"
)

// =============================================================================
// History Tests
// =============================================================================

// TestNewHistory tests history creation.
func TestNewHistory(t *testing.T) {
	tests := []struct {
		name     string
		maxCount int
		wantMax  int
	}{
		{"positive max", 5, 5},
		{"zero max", 0, 0},
		{"negative max", -1, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHistory(tt.maxCount)
			if h == nil {
				t.Fatal("NewHistory returned nil")
			}
			if h.MaxCount() != tt.wantMax {
				t.Errorf("MaxCount() = %d, want %d", h.MaxCount(), tt.wantMax)
			}
			if h.Count() != 0 {
				t.Errorf("Count() = %d, want 0", h.Count())
			}
		})
	}
}

// TestNewHistoryFromHashes tests creating history from existing hashes.
func TestNewHistoryFromHashes(t *testing.T) {
	hashes := []string{"hash1", "hash2", "hash3", "hash4", "hash5"}

	tests := []struct {
		name      string
		hashes    []string
		maxCount  int
		wantCount int
	}{
		{"exact fit", hashes, 5, 5},
		{"truncate", hashes, 3, 3},
		{"more space", hashes[:2], 5, 2},
		{"zero max", hashes, 0, 0},
		{"empty hashes", []string{}, 5, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHistoryFromHashes(tt.hashes, tt.maxCount)
			if h.Count() != tt.wantCount {
				t.Errorf("Count() = %d, want %d", h.Count(), tt.wantCount)
			}
		})
	}
}

// TestHistoryAdd tests adding hashes to history.
func TestHistoryAdd(t *testing.T) {
	h := NewHistory(3)

	// Add first hash
	h.Add("hash1")
	if h.Count() != 1 {
		t.Errorf("Count() = %d, want 1", h.Count())
	}

	// Add more hashes
	h.Add("hash2")
	h.Add("hash3")
	if h.Count() != 3 {
		t.Errorf("Count() = %d, want 3", h.Count())
	}

	// Add beyond max - should trim
	h.Add("hash4")
	if h.Count() != 3 {
		t.Errorf("Count() = %d, want 3 (should trim)", h.Count())
	}

	// Verify newest is first
	hashes := h.Hashes()
	if hashes[0] != "hash4" {
		t.Errorf("newest hash should be first, got %s", hashes[0])
	}
}

// TestHistoryAddDisabled tests adding to disabled history.
func TestHistoryAddDisabled(t *testing.T) {
	h := NewHistory(0)
	h.Add("hash1")
	if h.Count() != 0 {
		t.Errorf("disabled history should not store hashes, got count %d", h.Count())
	}
}

// TestHistoryContains tests hash lookup with constant-time comparison.
func TestHistoryContains(t *testing.T) {
	h := NewHistory(5)
	h.Add("hash1")
	h.Add("hash2")
	h.Add("hash3")

	tests := []struct {
		hash string
		want bool
	}{
		{"hash1", true},
		{"hash2", true},
		{"hash3", true},
		{"hash4", false},
		{"", false},
		{"HASH1", false}, // Case sensitive
	}

	for _, tt := range tests {
		t.Run(tt.hash, func(t *testing.T) {
			if got := h.Contains(tt.hash); got != tt.want {
				t.Errorf("Contains(%q) = %v, want %v", tt.hash, got, tt.want)
			}
		})
	}
}

// TestHistoryContainsEmpty tests Contains on empty history.
func TestHistoryContainsEmpty(t *testing.T) {
	h := NewHistory(5)
	if h.Contains("anything") {
		t.Error("empty history should not contain anything")
	}
}

// TestHistoryHashes tests getting all hashes.
func TestHistoryHashes(t *testing.T) {
	h := NewHistory(5)
	h.Add("hash1")
	h.Add("hash2")
	h.Add("hash3")

	hashes := h.Hashes()
	if len(hashes) != 3 {
		t.Errorf("len(Hashes()) = %d, want 3", len(hashes))
	}

	// Verify order (newest first)
	expected := []string{"hash3", "hash2", "hash1"}
	for i, want := range expected {
		if hashes[i] != want {
			t.Errorf("Hashes()[%d] = %s, want %s", i, hashes[i], want)
		}
	}

	// Verify it's a copy
	hashes[0] = "modified"
	if h.Hashes()[0] == "modified" {
		t.Error("Hashes() should return a copy")
	}
}

// TestHistoryClear tests clearing history.
func TestHistoryClear(t *testing.T) {
	h := NewHistory(5)
	h.Add("hash1")
	h.Add("hash2")

	h.Clear()

	if h.Count() != 0 {
		t.Errorf("Count() after Clear() = %d, want 0", h.Count())
	}
	if h.Contains("hash1") {
		t.Error("history should not contain hash1 after Clear()")
	}
}

// TestHistorySetMaxCount tests changing max count.
func TestHistorySetMaxCount(t *testing.T) {
	h := NewHistory(5)
	h.Add("hash1")
	h.Add("hash2")
	h.Add("hash3")
	h.Add("hash4")
	h.Add("hash5")

	// Reduce max count
	h.SetMaxCount(3)
	if h.Count() != 3 {
		t.Errorf("Count() after SetMaxCount(3) = %d, want 3", h.Count())
	}
	if h.MaxCount() != 3 {
		t.Errorf("MaxCount() = %d, want 3", h.MaxCount())
	}

	// Verify oldest were removed
	if h.Contains("hash1") || h.Contains("hash2") {
		t.Error("oldest hashes should be removed")
	}

	// Set to zero
	h.SetMaxCount(0)
	if h.Count() != 0 {
		t.Errorf("Count() after SetMaxCount(0) = %d, want 0", h.Count())
	}

	// Negative should become zero
	h.SetMaxCount(-5)
	if h.MaxCount() != 0 {
		t.Errorf("MaxCount() after SetMaxCount(-5) = %d, want 0", h.MaxCount())
	}
}

// TestHistoryIsEnabled tests enabled check.
func TestHistoryIsEnabled(t *testing.T) {
	enabled := NewHistory(5)
	if !enabled.IsEnabled() {
		t.Error("history with maxCount > 0 should be enabled")
	}

	disabled := NewHistory(0)
	if disabled.IsEnabled() {
		t.Error("history with maxCount = 0 should be disabled")
	}
}

// TestHistoryClone tests cloning history.
func TestHistoryClone(t *testing.T) {
	original := NewHistory(5)
	original.Add("hash1")
	original.Add("hash2")

	clone := original.Clone()

	// Verify values match
	if clone.Count() != original.Count() {
		t.Error("clone count mismatch")
	}
	if clone.MaxCount() != original.MaxCount() {
		t.Error("clone maxCount mismatch")
	}

	// Modify clone and verify original unchanged
	clone.Add("hash3")
	if original.Count() == clone.Count() {
		t.Error("modifying clone affected original")
	}
}

// =============================================================================
// HistoryManager Tests
// =============================================================================

// TestNewHistoryManager tests manager creation.
func TestNewHistoryManager(t *testing.T) {
	m := NewHistoryManager(5)
	if m == nil {
		t.Fatal("NewHistoryManager returned nil")
	}
	if m.DefaultMaxCount() != 5 {
		t.Errorf("DefaultMaxCount() = %d, want 5", m.DefaultMaxCount())
	}
	if m.UserCount() != 0 {
		t.Errorf("UserCount() = %d, want 0", m.UserCount())
	}
}

// TestHistoryManagerGetHistory tests getting user history.
func TestHistoryManagerGetHistory(t *testing.T) {
	m := NewHistoryManager(5)
	dn := "uid=alice,ou=users,dc=example,dc=com"

	// Get creates new history
	h := m.GetHistory(dn)
	if h == nil {
		t.Fatal("GetHistory returned nil")
	}
	if h.MaxCount() != 5 {
		t.Errorf("MaxCount() = %d, want 5", h.MaxCount())
	}

	// Same DN returns same history
	h.Add("hash1")
	h2 := m.GetHistory(dn)
	if !h2.Contains("hash1") {
		t.Error("GetHistory should return same history instance")
	}
}

// TestHistoryManagerSetHistory tests setting user history.
func TestHistoryManagerSetHistory(t *testing.T) {
	m := NewHistoryManager(5)
	dn := "uid=bob,dc=example,dc=com"

	h := NewHistory(10)
	h.Add("custom_hash")

	m.SetHistory(dn, h)

	retrieved := m.GetHistory(dn)
	if !retrieved.Contains("custom_hash") {
		t.Error("SetHistory should store the history")
	}

	// Set nil removes
	m.SetHistory(dn, nil)
	if m.HasHistory(dn) {
		t.Error("SetHistory(nil) should remove history")
	}
}

// TestHistoryManagerHasHistory tests checking history existence.
func TestHistoryManagerHasHistory(t *testing.T) {
	m := NewHistoryManager(5)
	dn := "uid=charlie,dc=example,dc=com"

	if m.HasHistory(dn) {
		t.Error("HasHistory should return false for non-existent")
	}

	m.GetHistory(dn) // Creates history

	if !m.HasHistory(dn) {
		t.Error("HasHistory should return true after GetHistory")
	}
}

// TestHistoryManagerAddPassword tests adding passwords.
func TestHistoryManagerAddPassword(t *testing.T) {
	m := NewHistoryManager(5)
	dn := "uid=dave,dc=example,dc=com"

	m.AddPassword(dn, "hash1")
	m.AddPassword(dn, "hash2")

	if !m.ContainsPassword(dn, "hash1") {
		t.Error("ContainsPassword should return true for hash1")
	}
	if !m.ContainsPassword(dn, "hash2") {
		t.Error("ContainsPassword should return true for hash2")
	}
}

// TestHistoryManagerContainsPassword tests password lookup.
func TestHistoryManagerContainsPassword(t *testing.T) {
	m := NewHistoryManager(5)
	dn := "uid=eve,dc=example,dc=com"

	// Non-existent user
	if m.ContainsPassword(dn, "hash1") {
		t.Error("ContainsPassword should return false for non-existent user")
	}

	m.AddPassword(dn, "hash1")

	if !m.ContainsPassword(dn, "hash1") {
		t.Error("ContainsPassword should return true for existing hash")
	}
	if m.ContainsPassword(dn, "hash2") {
		t.Error("ContainsPassword should return false for non-existent hash")
	}
}

// TestHistoryManagerListUsers tests listing users.
func TestHistoryManagerListUsers(t *testing.T) {
	m := NewHistoryManager(5)

	dns := []string{
		"uid=user1,dc=example,dc=com",
		"uid=user2,dc=example,dc=com",
		"uid=user3,dc=example,dc=com",
	}

	for _, dn := range dns {
		m.AddPassword(dn, "hash")
	}

	users := m.ListUsers()
	if len(users) != 3 {
		t.Errorf("ListUsers() returned %d users, want 3", len(users))
	}
}

// TestHistoryManagerClearAll tests clearing all histories.
func TestHistoryManagerClearAll(t *testing.T) {
	m := NewHistoryManager(5)

	m.AddPassword("uid=user1,dc=example,dc=com", "hash1")
	m.AddPassword("uid=user2,dc=example,dc=com", "hash2")

	m.ClearAll()

	if m.UserCount() != 0 {
		t.Errorf("UserCount() after ClearAll() = %d, want 0", m.UserCount())
	}
}

// TestHistoryManagerDNCaseInsensitivity tests case-insensitive DN handling.
func TestHistoryManagerDNCaseInsensitivity(t *testing.T) {
	m := NewHistoryManager(5)

	dn := "uid=Alice,ou=Users,dc=Example,dc=COM"
	m.AddPassword(dn, "hash1")

	variations := []string{
		"uid=alice,ou=users,dc=example,dc=com",
		"UID=ALICE,OU=USERS,DC=EXAMPLE,DC=COM",
		"  uid=alice,ou=users,dc=example,dc=com  ",
	}

	for _, v := range variations {
		if !m.ContainsPassword(v, "hash1") {
			t.Errorf("ContainsPassword should work for DN variation: %s", v)
		}
	}
}

// TestHistoryManagerConcurrency tests concurrent access.
func TestHistoryManagerConcurrency(t *testing.T) {
	m := NewHistoryManager(10)
	done := make(chan bool)

	// Concurrent writers
	for i := 0; i < 10; i++ {
		go func(id int) {
			dn := "uid=user" + string(rune('0'+id)) + ",dc=example,dc=com"
			for j := 0; j < 100; j++ {
				m.AddPassword(dn, "hash"+string(rune('0'+j)))
			}
			done <- true
		}(i)
	}

	// Concurrent readers
	for i := 0; i < 10; i++ {
		go func(id int) {
			dn := "uid=user" + string(rune('0'+id)) + ",dc=example,dc=com"
			for j := 0; j < 100; j++ {
				_ = m.ContainsPassword(dn, "hash"+string(rune('0'+j)))
				_ = m.HasHistory(dn)
				_ = m.ListUsers()
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 20; i++ {
		<-done
	}
}
