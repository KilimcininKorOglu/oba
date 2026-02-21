package radix

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/KilimcininKorOglu/oba/internal/storage"
)

// Helper function to create a temporary page manager for testing.
func createTestPageManager(t *testing.T) (*storage.PageManager, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "radix_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tmpDir, "test.db")
	opts := storage.DefaultOptions()
	opts.CreateIfNew = true

	pm, err := storage.OpenPageManager(dbPath, opts)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to create page manager: %v", err)
	}

	cleanup := func() {
		pm.Close()
		os.RemoveAll(tmpDir)
	}

	return pm, cleanup
}

// =============================================================================
// DN Parsing Tests
// =============================================================================

func TestParseDN(t *testing.T) {
	tests := []struct {
		name     string
		dn       string
		expected []string
		wantErr  bool
	}{
		{
			name:     "simple DN",
			dn:       "uid=alice,ou=users,dc=example,dc=com",
			expected: []string{"dc=com", "dc=example", "ou=users", "uid=alice"},
			wantErr:  false,
		},
		{
			name:     "single component",
			dn:       "dc=com",
			expected: []string{"dc=com"},
			wantErr:  false,
		},
		{
			name:     "two components",
			dn:       "dc=example,dc=com",
			expected: []string{"dc=com", "dc=example"},
			wantErr:  false,
		},
		{
			name:     "with spaces",
			dn:       "uid=alice , ou=users , dc=example , dc=com",
			expected: []string{"dc=com", "dc=example", "ou=users", "uid=alice"},
			wantErr:  false,
		},
		{
			name:     "uppercase attribute types",
			dn:       "UID=alice,OU=users,DC=example,DC=com",
			expected: []string{"dc=com", "dc=example", "ou=users", "uid=alice"},
			wantErr:  false,
		},
		{
			name:    "empty DN",
			dn:      "",
			wantErr: true,
		},
		{
			name:    "whitespace only",
			dn:      "   ",
			wantErr: true,
		},
		{
			name:    "invalid RDN - no equals",
			dn:      "invalid",
			wantErr: true,
		},
		{
			name:    "invalid RDN - empty attribute type",
			dn:      "=value",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseDN(tt.dn)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if len(result) != len(tt.expected) {
				t.Errorf("expected %d components, got %d", len(tt.expected), len(result))
				return
			}
			for i, comp := range result {
				if comp != tt.expected[i] {
					t.Errorf("component %d: expected %q, got %q", i, tt.expected[i], comp)
				}
			}
		})
	}
}

func TestParseDNForward(t *testing.T) {
	dn := "uid=alice,ou=users,dc=example,dc=com"
	expected := []string{"uid=alice", "ou=users", "dc=example", "dc=com"}

	result, err := ParseDNForward(dn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != len(expected) {
		t.Fatalf("expected %d components, got %d", len(expected), len(result))
	}

	for i, comp := range result {
		if comp != expected[i] {
			t.Errorf("component %d: expected %q, got %q", i, expected[i], comp)
		}
	}
}

func TestJoinDN(t *testing.T) {
	components := []string{"uid=alice", "ou=users", "dc=example", "dc=com"}
	expected := "uid=alice,ou=users,dc=example,dc=com"

	result := JoinDN(components)
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestJoinDNReverse(t *testing.T) {
	components := []string{"dc=com", "dc=example", "ou=users", "uid=alice"}
	expected := "uid=alice,ou=users,dc=example,dc=com"

	result := JoinDNReverse(components)
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestGetParentDN(t *testing.T) {
	tests := []struct {
		name     string
		dn       string
		expected string
		wantErr  bool
	}{
		{
			name:     "normal DN",
			dn:       "uid=alice,ou=users,dc=example,dc=com",
			expected: "ou=users,dc=example,dc=com",
			wantErr:  false,
		},
		{
			name:     "two components",
			dn:       "dc=example,dc=com",
			expected: "dc=com",
			wantErr:  false,
		},
		{
			name:     "single component",
			dn:       "dc=com",
			expected: "",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := GetParentDN(tt.dn)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestGetRDN(t *testing.T) {
	dn := "uid=alice,ou=users,dc=example,dc=com"
	expected := "uid=alice"

	result, err := GetRDN(dn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestIsDescendantOf(t *testing.T) {
	tests := []struct {
		name     string
		childDN  string
		parentDN string
		expected bool
	}{
		{
			name:     "direct child",
			childDN:  "ou=users,dc=example,dc=com",
			parentDN: "dc=example,dc=com",
			expected: true,
		},
		{
			name:     "deep descendant",
			childDN:  "uid=alice,ou=users,dc=example,dc=com",
			parentDN: "dc=example,dc=com",
			expected: true,
		},
		{
			name:     "same DN",
			childDN:  "dc=example,dc=com",
			parentDN: "dc=example,dc=com",
			expected: false,
		},
		{
			name:     "not descendant",
			childDN:  "ou=groups,dc=example,dc=com",
			parentDN: "ou=users,dc=example,dc=com",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := IsDescendantOf(tt.childDN, tt.parentDN)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestIsDirectChildOf(t *testing.T) {
	tests := []struct {
		name     string
		childDN  string
		parentDN string
		expected bool
	}{
		{
			name:     "direct child",
			childDN:  "ou=users,dc=example,dc=com",
			parentDN: "dc=example,dc=com",
			expected: true,
		},
		{
			name:     "not direct child",
			childDN:  "uid=alice,ou=users,dc=example,dc=com",
			parentDN: "dc=example,dc=com",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := IsDirectChildOf(tt.childDN, tt.parentDN)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestDNDepth(t *testing.T) {
	tests := []struct {
		name     string
		dn       string
		expected int
	}{
		{
			name:     "four components",
			dn:       "uid=alice,ou=users,dc=example,dc=com",
			expected: 4,
		},
		{
			name:     "single component",
			dn:       "dc=com",
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := DNDepth(tt.dn)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestNormalizeDN(t *testing.T) {
	dn := "UID=alice , OU=users , DC=example , DC=com"
	expected := "uid=alice,ou=users,dc=example,dc=com"

	result, err := NormalizeDN(dn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestCompareDN(t *testing.T) {
	tests := []struct {
		name     string
		dn1      string
		dn2      string
		expected bool
	}{
		{
			name:     "equal DNs",
			dn1:      "uid=alice,ou=users,dc=example,dc=com",
			dn2:      "uid=alice,ou=users,dc=example,dc=com",
			expected: true,
		},
		{
			name:     "equal with different case",
			dn1:      "UID=alice,OU=users,DC=example,DC=com",
			dn2:      "uid=alice,ou=users,dc=example,dc=com",
			expected: true,
		},
		{
			name:     "different DNs",
			dn1:      "uid=alice,ou=users,dc=example,dc=com",
			dn2:      "uid=bob,ou=users,dc=example,dc=com",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := CompareDN(tt.dn1, tt.dn2)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestEscapedCommaInDN(t *testing.T) {
	// DN with escaped comma in value
	dn := "cn=Smith\\, John,ou=users,dc=example,dc=com"
	components, err := ParseDN(dn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(components) != 4 {
		t.Errorf("expected 4 components, got %d", len(components))
	}

	// The escaped comma should be preserved
	if components[3] != "cn=Smith\\, John" {
		t.Errorf("expected 'cn=Smith\\, John', got %q", components[3])
	}
}

// =============================================================================
// RadixTree Tests
// =============================================================================

func TestNewRadixTree(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewRadixTree(pm)
	if err != nil {
		t.Fatalf("failed to create radix tree: %v", err)
	}

	if tree == nil {
		t.Fatal("expected non-nil tree")
	}

	if tree.Root() == nil {
		t.Error("expected non-nil root")
	}

	if tree.RootPageID() == 0 {
		t.Error("expected non-zero root page ID")
	}

	if tree.EntryCount() != 0 {
		t.Errorf("expected 0 entries, got %d", tree.EntryCount())
	}
}

func TestNewRadixTreeNilPageManager(t *testing.T) {
	_, err := NewRadixTree(nil)
	if err != ErrInvalidPageManager {
		t.Errorf("expected ErrInvalidPageManager, got %v", err)
	}
}

func TestRadixTreeInsert(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewRadixTree(pm)
	if err != nil {
		t.Fatalf("failed to create radix tree: %v", err)
	}

	// Insert a DN
	dn := "uid=alice,ou=users,dc=example,dc=com"
	err = tree.Insert(dn, storage.PageID(42), 7)
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	// Verify entry count
	if tree.EntryCount() != 1 {
		t.Errorf("expected 1 entry, got %d", tree.EntryCount())
	}

	// Lookup the entry
	pageID, slotID, found := tree.Lookup(dn)
	if !found {
		t.Error("expected to find entry")
	}
	if pageID != 42 {
		t.Errorf("expected pageID 42, got %d", pageID)
	}
	if slotID != 7 {
		t.Errorf("expected slotID 7, got %d", slotID)
	}
}

func TestRadixTreeInsertMultiple(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewRadixTree(pm)
	if err != nil {
		t.Fatalf("failed to create radix tree: %v", err)
	}

	// Insert multiple DNs
	dns := []string{
		"dc=com",
		"dc=example,dc=com",
		"ou=users,dc=example,dc=com",
		"uid=alice,ou=users,dc=example,dc=com",
		"uid=bob,ou=users,dc=example,dc=com",
		"ou=groups,dc=example,dc=com",
		"cn=admins,ou=groups,dc=example,dc=com",
	}

	for i, dn := range dns {
		err = tree.Insert(dn, storage.PageID(uint64(i+1)), uint16(i))
		if err != nil {
			t.Fatalf("failed to insert %s: %v", dn, err)
		}
	}

	// Verify entry count
	if tree.EntryCount() != uint32(len(dns)) {
		t.Errorf("expected %d entries, got %d", len(dns), tree.EntryCount())
	}

	// Verify all entries can be looked up
	for i, dn := range dns {
		pageID, slotID, found := tree.Lookup(dn)
		if !found {
			t.Errorf("expected to find %s", dn)
			continue
		}
		if pageID != storage.PageID(uint64(i+1)) {
			t.Errorf("expected pageID %d for %s, got %d", i+1, dn, pageID)
		}
		if slotID != uint16(i) {
			t.Errorf("expected slotID %d for %s, got %d", i, dn, slotID)
		}
	}
}

func TestRadixTreeInsertDuplicate(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewRadixTree(pm)
	if err != nil {
		t.Fatalf("failed to create radix tree: %v", err)
	}

	dn := "uid=alice,ou=users,dc=example,dc=com"

	// First insert should succeed
	err = tree.Insert(dn, storage.PageID(42), 7)
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	// Second insert should fail
	err = tree.Insert(dn, storage.PageID(100), 10)
	if err != ErrEntryExists {
		t.Errorf("expected ErrEntryExists, got %v", err)
	}
}

func TestRadixTreeInsertInvalidDN(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewRadixTree(pm)
	if err != nil {
		t.Fatalf("failed to create radix tree: %v", err)
	}

	// Empty DN
	err = tree.Insert("", storage.PageID(42), 7)
	if err == nil {
		t.Error("expected error for empty DN")
	}

	// Invalid DN
	err = tree.Insert("invalid", storage.PageID(42), 7)
	if err == nil {
		t.Error("expected error for invalid DN")
	}
}

func TestRadixTreeDelete(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewRadixTree(pm)
	if err != nil {
		t.Fatalf("failed to create radix tree: %v", err)
	}

	dn := "uid=alice,ou=users,dc=example,dc=com"

	// Insert
	err = tree.Insert(dn, storage.PageID(42), 7)
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	// Delete
	err = tree.Delete(dn)
	if err != nil {
		t.Fatalf("failed to delete: %v", err)
	}

	// Verify entry count
	if tree.EntryCount() != 0 {
		t.Errorf("expected 0 entries, got %d", tree.EntryCount())
	}

	// Verify lookup fails
	_, _, found := tree.Lookup(dn)
	if found {
		t.Error("expected entry to be deleted")
	}
}

func TestRadixTreeDeleteNonExistent(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewRadixTree(pm)
	if err != nil {
		t.Fatalf("failed to create radix tree: %v", err)
	}

	err = tree.Delete("uid=nonexistent,dc=example,dc=com")
	if err != ErrEntryNotFound {
		t.Errorf("expected ErrEntryNotFound, got %v", err)
	}
}

func TestRadixTreeDeleteWithChildren(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewRadixTree(pm)
	if err != nil {
		t.Fatalf("failed to create radix tree: %v", err)
	}

	// Insert parent and child
	parentDN := "ou=users,dc=example,dc=com"
	childDN := "uid=alice,ou=users,dc=example,dc=com"

	err = tree.Insert(parentDN, storage.PageID(1), 1)
	if err != nil {
		t.Fatalf("failed to insert parent: %v", err)
	}

	err = tree.Insert(childDN, storage.PageID(2), 2)
	if err != nil {
		t.Fatalf("failed to insert child: %v", err)
	}

	// Delete parent (should succeed, child remains)
	err = tree.Delete(parentDN)
	if err != nil {
		t.Fatalf("failed to delete parent: %v", err)
	}

	// Parent should be gone
	_, _, found := tree.Lookup(parentDN)
	if found {
		t.Error("expected parent to be deleted")
	}

	// Child should still exist
	_, _, found = tree.Lookup(childDN)
	if !found {
		t.Error("expected child to still exist")
	}

	// Entry count should be 1
	if tree.EntryCount() != 1 {
		t.Errorf("expected 1 entry, got %d", tree.EntryCount())
	}
}

func TestRadixTreeDeleteCleansUpEmptyNodes(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewRadixTree(pm)
	if err != nil {
		t.Fatalf("failed to create radix tree: %v", err)
	}

	// Insert a deep entry
	dn := "uid=alice,ou=users,dc=example,dc=com"
	err = tree.Insert(dn, storage.PageID(42), 7)
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	// Delete it
	err = tree.Delete(dn)
	if err != nil {
		t.Fatalf("failed to delete: %v", err)
	}

	// The tree should be empty (intermediate nodes cleaned up)
	stats := tree.Stats()
	if stats.EntryNodes != 0 {
		t.Errorf("expected 0 entry nodes, got %d", stats.EntryNodes)
	}
}

func TestRadixTreeLookup(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewRadixTree(pm)
	if err != nil {
		t.Fatalf("failed to create radix tree: %v", err)
	}

	dn := "uid=alice,ou=users,dc=example,dc=com"
	err = tree.Insert(dn, storage.PageID(42), 7)
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	// Lookup existing
	pageID, slotID, found := tree.Lookup(dn)
	if !found {
		t.Error("expected to find entry")
	}
	if pageID != 42 || slotID != 7 {
		t.Errorf("expected (42, 7), got (%d, %d)", pageID, slotID)
	}

	// Lookup non-existing
	_, _, found = tree.Lookup("uid=bob,ou=users,dc=example,dc=com")
	if found {
		t.Error("expected not to find entry")
	}

	// Lookup intermediate node (no entry)
	_, _, found = tree.Lookup("ou=users,dc=example,dc=com")
	if found {
		t.Error("expected not to find intermediate node")
	}
}

func TestRadixTreeExists(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewRadixTree(pm)
	if err != nil {
		t.Fatalf("failed to create radix tree: %v", err)
	}

	dn := "uid=alice,ou=users,dc=example,dc=com"
	err = tree.Insert(dn, storage.PageID(42), 7)
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	if !tree.Exists(dn) {
		t.Error("expected entry to exist")
	}

	if tree.Exists("uid=bob,ou=users,dc=example,dc=com") {
		t.Error("expected entry not to exist")
	}
}

func TestRadixTreeUpdate(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewRadixTree(pm)
	if err != nil {
		t.Fatalf("failed to create radix tree: %v", err)
	}

	dn := "uid=alice,ou=users,dc=example,dc=com"
	err = tree.Insert(dn, storage.PageID(42), 7)
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	// Update
	err = tree.Update(dn, storage.PageID(100), 10)
	if err != nil {
		t.Fatalf("failed to update: %v", err)
	}

	// Verify
	pageID, slotID, found := tree.Lookup(dn)
	if !found {
		t.Error("expected to find entry")
	}
	if pageID != 100 || slotID != 10 {
		t.Errorf("expected (100, 10), got (%d, %d)", pageID, slotID)
	}
}

func TestRadixTreeUpdateNonExistent(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewRadixTree(pm)
	if err != nil {
		t.Fatalf("failed to create radix tree: %v", err)
	}

	err = tree.Update("uid=nonexistent,dc=example,dc=com", storage.PageID(100), 10)
	if err != ErrEntryNotFound {
		t.Errorf("expected ErrEntryNotFound, got %v", err)
	}
}

func TestRadixTreeGetSubtreeCount(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewRadixTree(pm)
	if err != nil {
		t.Fatalf("failed to create radix tree: %v", err)
	}

	// Insert entries
	dns := []string{
		"dc=example,dc=com",
		"ou=users,dc=example,dc=com",
		"uid=alice,ou=users,dc=example,dc=com",
		"uid=bob,ou=users,dc=example,dc=com",
		"ou=groups,dc=example,dc=com",
	}

	for i, dn := range dns {
		err = tree.Insert(dn, storage.PageID(uint64(i+1)), uint16(i))
		if err != nil {
			t.Fatalf("failed to insert %s: %v", dn, err)
		}
	}

	// Check subtree count for ou=users
	count, err := tree.GetSubtreeCount("ou=users,dc=example,dc=com")
	if err != nil {
		t.Fatalf("failed to get subtree count: %v", err)
	}
	// ou=users has itself + alice + bob = 3
	if count != 3 {
		t.Errorf("expected subtree count 3, got %d", count)
	}

	// Check subtree count for dc=example,dc=com
	count, err = tree.GetSubtreeCount("dc=example,dc=com")
	if err != nil {
		t.Fatalf("failed to get subtree count: %v", err)
	}
	// dc=example has all 5 entries
	if count != 5 {
		t.Errorf("expected subtree count 5, got %d", count)
	}
}

func TestRadixTreeHasChildren(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewRadixTree(pm)
	if err != nil {
		t.Fatalf("failed to create radix tree: %v", err)
	}

	// Insert parent and child
	parentDN := "ou=users,dc=example,dc=com"
	childDN := "uid=alice,ou=users,dc=example,dc=com"

	err = tree.Insert(parentDN, storage.PageID(1), 1)
	if err != nil {
		t.Fatalf("failed to insert parent: %v", err)
	}

	err = tree.Insert(childDN, storage.PageID(2), 2)
	if err != nil {
		t.Fatalf("failed to insert child: %v", err)
	}

	// Parent should have children
	hasChildren, err := tree.HasChildren(parentDN)
	if err != nil {
		t.Fatalf("failed to check children: %v", err)
	}
	if !hasChildren {
		t.Error("expected parent to have children")
	}

	// Child should not have children
	hasChildren, err = tree.HasChildren(childDN)
	if err != nil {
		t.Fatalf("failed to check children: %v", err)
	}
	if hasChildren {
		t.Error("expected child not to have children")
	}
}

func TestRadixTreeGetParent(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewRadixTree(pm)
	if err != nil {
		t.Fatalf("failed to create radix tree: %v", err)
	}

	dn := "uid=alice,ou=users,dc=example,dc=com"
	parentDN, err := tree.GetParent(dn)
	if err != nil {
		t.Fatalf("failed to get parent: %v", err)
	}

	expected := "ou=users,dc=example,dc=com"
	if parentDN != expected {
		t.Errorf("expected %q, got %q", expected, parentDN)
	}
}

func TestRadixTreeIterateSubtree(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewRadixTree(pm)
	if err != nil {
		t.Fatalf("failed to create radix tree: %v", err)
	}

	// Insert entries
	dns := []string{
		"dc=example,dc=com",
		"ou=users,dc=example,dc=com",
		"uid=alice,ou=users,dc=example,dc=com",
		"uid=bob,ou=users,dc=example,dc=com",
		"ou=groups,dc=example,dc=com",
	}

	for i, dn := range dns {
		err = tree.Insert(dn, storage.PageID(uint64(i+1)), uint16(i))
		if err != nil {
			t.Fatalf("failed to insert %s: %v", dn, err)
		}
	}

	// Iterate all entries
	var visited []string
	err = tree.IterateSubtree("", func(dn string, pageID storage.PageID, slotID uint16) bool {
		visited = append(visited, dn)
		return true
	})
	if err != nil {
		t.Fatalf("failed to iterate: %v", err)
	}

	if len(visited) != len(dns) {
		t.Errorf("expected %d entries, visited %d", len(dns), len(visited))
	}

	// Iterate subtree of ou=users
	visited = nil
	err = tree.IterateSubtree("ou=users,dc=example,dc=com", func(dn string, pageID storage.PageID, slotID uint16) bool {
		visited = append(visited, dn)
		return true
	})
	if err != nil {
		t.Fatalf("failed to iterate subtree: %v", err)
	}

	// Should include ou=users, alice, bob
	if len(visited) != 3 {
		t.Errorf("expected 3 entries in subtree, visited %d", len(visited))
	}
}

func TestRadixTreeIterateOneLevelChildren(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewRadixTree(pm)
	if err != nil {
		t.Fatalf("failed to create radix tree: %v", err)
	}

	// Insert entries
	dns := []string{
		"dc=example,dc=com",
		"ou=users,dc=example,dc=com",
		"uid=alice,ou=users,dc=example,dc=com",
		"uid=bob,ou=users,dc=example,dc=com",
		"ou=groups,dc=example,dc=com",
	}

	for i, dn := range dns {
		err = tree.Insert(dn, storage.PageID(uint64(i+1)), uint16(i))
		if err != nil {
			t.Fatalf("failed to insert %s: %v", dn, err)
		}
	}

	// Iterate one level children of dc=example,dc=com
	var visited []string
	err = tree.IterateOneLevelChildren("dc=example,dc=com", func(dn string, pageID storage.PageID, slotID uint16) bool {
		visited = append(visited, dn)
		return true
	})
	if err != nil {
		t.Fatalf("failed to iterate: %v", err)
	}

	// Should include ou=users and ou=groups
	if len(visited) != 2 {
		t.Errorf("expected 2 direct children, visited %d", len(visited))
	}
}

func TestRadixTreeClear(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewRadixTree(pm)
	if err != nil {
		t.Fatalf("failed to create radix tree: %v", err)
	}

	// Insert some entries
	err = tree.Insert("uid=alice,ou=users,dc=example,dc=com", storage.PageID(1), 1)
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	err = tree.Insert("uid=bob,ou=users,dc=example,dc=com", storage.PageID(2), 2)
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	// Clear
	err = tree.Clear()
	if err != nil {
		t.Fatalf("failed to clear: %v", err)
	}

	// Verify empty
	if tree.EntryCount() != 0 {
		t.Errorf("expected 0 entries, got %d", tree.EntryCount())
	}
}

func TestRadixTreeStats(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewRadixTree(pm)
	if err != nil {
		t.Fatalf("failed to create radix tree: %v", err)
	}

	// Insert entries
	dns := []string{
		"dc=example,dc=com",
		"ou=users,dc=example,dc=com",
		"uid=alice,ou=users,dc=example,dc=com",
	}

	for i, dn := range dns {
		err = tree.Insert(dn, storage.PageID(uint64(i+1)), uint16(i))
		if err != nil {
			t.Fatalf("failed to insert %s: %v", dn, err)
		}
	}

	stats := tree.Stats()

	if stats.TotalEntries != 3 {
		t.Errorf("expected 3 total entries, got %d", stats.TotalEntries)
	}

	if stats.EntryNodes != 3 {
		t.Errorf("expected 3 entry nodes, got %d", stats.EntryNodes)
	}

	// Max depth should be 4 (root -> dc=com -> dc=example -> ou=users -> uid=alice)
	if stats.MaxDepth < 3 {
		t.Errorf("expected max depth >= 3, got %d", stats.MaxDepth)
	}
}

func TestRadixTreePersistence(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "radix_persist_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	opts := storage.DefaultOptions()
	opts.CreateIfNew = true

	// Create and populate tree
	pm, err := storage.OpenPageManager(dbPath, opts)
	if err != nil {
		t.Fatalf("failed to create page manager: %v", err)
	}

	tree, err := NewRadixTree(pm)
	if err != nil {
		t.Fatalf("failed to create radix tree: %v", err)
	}

	rootPageID := tree.RootPageID()

	dn := "uid=alice,ou=users,dc=example,dc=com"
	err = tree.Insert(dn, storage.PageID(42), 7)
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	// Close
	pm.Close()

	// Reopen
	pm2, err := storage.OpenPageManager(dbPath, opts)
	if err != nil {
		t.Fatalf("failed to reopen page manager: %v", err)
	}
	defer pm2.Close()

	tree2, err := NewRadixTreeWithRoot(pm2, rootPageID)
	if err != nil {
		t.Fatalf("failed to load radix tree: %v", err)
	}

	// Verify entry persisted
	pageID, slotID, found := tree2.Lookup(dn)
	if !found {
		t.Error("expected to find entry after reload")
	}
	if pageID != 42 || slotID != 7 {
		t.Errorf("expected (42, 7), got (%d, %d)", pageID, slotID)
	}
}

func TestRadixTreeSubtreeCountAccuracy(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewRadixTree(pm)
	if err != nil {
		t.Fatalf("failed to create radix tree: %v", err)
	}

	// Insert entries in a specific order
	dns := []string{
		"dc=com",
		"dc=example,dc=com",
		"ou=users,dc=example,dc=com",
		"uid=alice,ou=users,dc=example,dc=com",
		"uid=bob,ou=users,dc=example,dc=com",
		"uid=charlie,ou=users,dc=example,dc=com",
		"ou=groups,dc=example,dc=com",
		"cn=admins,ou=groups,dc=example,dc=com",
	}

	for i, dn := range dns {
		err = tree.Insert(dn, storage.PageID(uint64(i+1)), uint16(i))
		if err != nil {
			t.Fatalf("failed to insert %s: %v", dn, err)
		}
	}

	// Verify subtree counts
	tests := []struct {
		dn       string
		expected uint32
	}{
		{"dc=com", 8},
		{"dc=example,dc=com", 7},
		{"ou=users,dc=example,dc=com", 4},
		{"ou=groups,dc=example,dc=com", 2},
		{"uid=alice,ou=users,dc=example,dc=com", 1},
	}

	for _, tt := range tests {
		count, err := tree.GetSubtreeCount(tt.dn)
		if err != nil {
			t.Errorf("failed to get subtree count for %s: %v", tt.dn, err)
			continue
		}
		if count != tt.expected {
			t.Errorf("subtree count for %s: expected %d, got %d", tt.dn, tt.expected, count)
		}
	}

	// Delete some entries and verify counts update
	err = tree.Delete("uid=alice,ou=users,dc=example,dc=com")
	if err != nil {
		t.Fatalf("failed to delete: %v", err)
	}

	count, _ := tree.GetSubtreeCount("ou=users,dc=example,dc=com")
	if count != 3 {
		t.Errorf("after delete, expected subtree count 3, got %d", count)
	}

	count, _ = tree.GetSubtreeCount("dc=com")
	if count != 7 {
		t.Errorf("after delete, expected root subtree count 7, got %d", count)
	}
}
