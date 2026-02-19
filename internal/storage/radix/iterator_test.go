package radix

import (
	"sort"
	"testing"

	"github.com/oba-ldap/oba/internal/storage"
)

// =============================================================================
// Scope Tests
// =============================================================================

func TestScopeString(t *testing.T) {
	tests := []struct {
		scope    Scope
		expected string
	}{
		{ScopeBase, "base"},
		{ScopeOneLevel, "onelevel"},
		{ScopeSubtree, "subtree"},
		{Scope(99), "unknown"},
	}

	for _, tt := range tests {
		result := tt.scope.String()
		if result != tt.expected {
			t.Errorf("Scope(%d).String() = %q, want %q", tt.scope, result, tt.expected)
		}
	}
}

// =============================================================================
// Iterator Creation Tests
// =============================================================================

func TestIteratorCreation(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewRadixTree(pm)
	if err != nil {
		t.Fatalf("failed to create radix tree: %v", err)
	}

	// Insert a test entry
	dn := "uid=alice,ou=users,dc=example,dc=com"
	err = tree.Insert(dn, storage.PageID(1), 1)
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	// Test creating iterator with valid DN
	iter, err := tree.Iterator(dn, ScopeBase)
	if err != nil {
		t.Fatalf("failed to create iterator: %v", err)
	}
	if iter == nil {
		t.Fatal("expected non-nil iterator")
	}
	if iter.Scope() != ScopeBase {
		t.Errorf("expected scope %v, got %v", ScopeBase, iter.Scope())
	}
	if iter.BaseDN() != dn {
		t.Errorf("expected baseDN %q, got %q", dn, iter.BaseDN())
	}
	iter.Close()
}

func TestIteratorCreationInvalidDN(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewRadixTree(pm)
	if err != nil {
		t.Fatalf("failed to create radix tree: %v", err)
	}

	// Test creating iterator with invalid DN
	_, err = tree.Iterator("", ScopeBase)
	if err == nil {
		t.Error("expected error for empty DN")
	}

	_, err = tree.Iterator("invalid", ScopeBase)
	if err == nil {
		t.Error("expected error for invalid DN")
	}
}

// =============================================================================
// Base Scope Tests
// =============================================================================

func TestIteratorBaseScopeExistingEntry(t *testing.T) {
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

	iter, err := tree.Iterator(dn, ScopeBase)
	if err != nil {
		t.Fatalf("failed to create iterator: %v", err)
	}
	defer iter.Close()

	// First call should return the entry
	resultDN, pageID, slotID, ok := iter.Next()
	if !ok {
		t.Error("expected to find entry")
	}
	if resultDN != dn {
		t.Errorf("expected DN %q, got %q", dn, resultDN)
	}
	if pageID != 42 {
		t.Errorf("expected pageID 42, got %d", pageID)
	}
	if slotID != 7 {
		t.Errorf("expected slotID 7, got %d", slotID)
	}

	// Second call should return nothing
	_, _, _, ok = iter.Next()
	if ok {
		t.Error("expected no more entries")
	}

	// Iterator should be done
	if !iter.IsDone() {
		t.Error("expected iterator to be done")
	}
}

func TestIteratorBaseScopeNonExistentEntry(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewRadixTree(pm)
	if err != nil {
		t.Fatalf("failed to create radix tree: %v", err)
	}

	// Create iterator for non-existent entry
	iter, err := tree.Iterator("uid=nonexistent,dc=example,dc=com", ScopeBase)
	if err != nil {
		t.Fatalf("failed to create iterator: %v", err)
	}
	defer iter.Close()

	// Should return nothing
	_, _, _, ok := iter.Next()
	if ok {
		t.Error("expected no entries for non-existent DN")
	}
}

func TestIteratorBaseScopeIntermediateNode(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewRadixTree(pm)
	if err != nil {
		t.Fatalf("failed to create radix tree: %v", err)
	}

	// Insert child but not parent
	childDN := "uid=alice,ou=users,dc=example,dc=com"
	err = tree.Insert(childDN, storage.PageID(1), 1)
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	// Create iterator for intermediate node (no entry)
	parentDN := "ou=users,dc=example,dc=com"
	iter, err := tree.Iterator(parentDN, ScopeBase)
	if err != nil {
		t.Fatalf("failed to create iterator: %v", err)
	}
	defer iter.Close()

	// Should return nothing (intermediate node has no entry)
	_, _, _, ok := iter.Next()
	if ok {
		t.Error("expected no entries for intermediate node without entry")
	}
}

// =============================================================================
// OneLevel Scope Tests
// =============================================================================

func TestIteratorOneLevelScopeWithChildren(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewRadixTree(pm)
	if err != nil {
		t.Fatalf("failed to create radix tree: %v", err)
	}

	// Insert parent and children
	parentDN := "ou=users,dc=example,dc=com"
	child1DN := "uid=alice,ou=users,dc=example,dc=com"
	child2DN := "uid=bob,ou=users,dc=example,dc=com"
	grandchildDN := "cn=test,uid=alice,ou=users,dc=example,dc=com"

	err = tree.Insert(parentDN, storage.PageID(1), 1)
	if err != nil {
		t.Fatalf("failed to insert parent: %v", err)
	}
	err = tree.Insert(child1DN, storage.PageID(2), 2)
	if err != nil {
		t.Fatalf("failed to insert child1: %v", err)
	}
	err = tree.Insert(child2DN, storage.PageID(3), 3)
	if err != nil {
		t.Fatalf("failed to insert child2: %v", err)
	}
	err = tree.Insert(grandchildDN, storage.PageID(4), 4)
	if err != nil {
		t.Fatalf("failed to insert grandchild: %v", err)
	}

	// Create iterator for onelevel scope
	iter, err := tree.Iterator(parentDN, ScopeOneLevel)
	if err != nil {
		t.Fatalf("failed to create iterator: %v", err)
	}
	defer iter.Close()

	// Collect all results
	var results []string
	for {
		dn, _, _, ok := iter.Next()
		if !ok {
			break
		}
		results = append(results, dn)
	}

	// Should only include immediate children (not parent, not grandchild)
	if len(results) != 2 {
		t.Errorf("expected 2 children, got %d: %v", len(results), results)
	}

	// Sort for consistent comparison
	sort.Strings(results)
	expected := []string{child1DN, child2DN}
	sort.Strings(expected)

	for i, dn := range results {
		if dn != expected[i] {
			t.Errorf("result[%d] = %q, want %q", i, dn, expected[i])
		}
	}
}

func TestIteratorOneLevelScopeNoChildren(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewRadixTree(pm)
	if err != nil {
		t.Fatalf("failed to create radix tree: %v", err)
	}

	// Insert leaf entry
	dn := "uid=alice,ou=users,dc=example,dc=com"
	err = tree.Insert(dn, storage.PageID(1), 1)
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	// Create iterator for onelevel scope on leaf
	iter, err := tree.Iterator(dn, ScopeOneLevel)
	if err != nil {
		t.Fatalf("failed to create iterator: %v", err)
	}
	defer iter.Close()

	// Should return nothing (no children)
	_, _, _, ok := iter.Next()
	if ok {
		t.Error("expected no children for leaf node")
	}
}

func TestIteratorOneLevelScopeChildrenWithoutEntries(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewRadixTree(pm)
	if err != nil {
		t.Fatalf("failed to create radix tree: %v", err)
	}

	// Insert only grandchild (creates intermediate nodes without entries)
	grandchildDN := "uid=alice,ou=users,dc=example,dc=com"
	err = tree.Insert(grandchildDN, storage.PageID(1), 1)
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	// Create iterator for onelevel scope on dc=example,dc=com
	iter, err := tree.Iterator("dc=example,dc=com", ScopeOneLevel)
	if err != nil {
		t.Fatalf("failed to create iterator: %v", err)
	}
	defer iter.Close()

	// Should return nothing (ou=users exists but has no entry)
	_, _, _, ok := iter.Next()
	if ok {
		t.Error("expected no children with entries")
	}
}

// =============================================================================
// Subtree Scope Tests
// =============================================================================

func TestIteratorSubtreeScopeAllDescendants(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewRadixTree(pm)
	if err != nil {
		t.Fatalf("failed to create radix tree: %v", err)
	}

	// Insert a hierarchy
	dns := []string{
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

	// Create iterator for subtree scope from root
	iter, err := tree.Iterator("dc=example,dc=com", ScopeSubtree)
	if err != nil {
		t.Fatalf("failed to create iterator: %v", err)
	}
	defer iter.Close()

	// Collect all results
	var results []string
	for {
		dn, _, _, ok := iter.Next()
		if !ok {
			break
		}
		results = append(results, dn)
	}

	// Should include all entries
	if len(results) != len(dns) {
		t.Errorf("expected %d entries, got %d: %v", len(dns), len(results), results)
	}

	// Verify all DNs are present
	resultSet := make(map[string]bool)
	for _, dn := range results {
		resultSet[dn] = true
	}
	for _, dn := range dns {
		if !resultSet[dn] {
			t.Errorf("missing DN: %s", dn)
		}
	}
}

func TestIteratorSubtreeScopePartialTree(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewRadixTree(pm)
	if err != nil {
		t.Fatalf("failed to create radix tree: %v", err)
	}

	// Insert a hierarchy
	dns := []string{
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

	// Create iterator for subtree scope from ou=users
	iter, err := tree.Iterator("ou=users,dc=example,dc=com", ScopeSubtree)
	if err != nil {
		t.Fatalf("failed to create iterator: %v", err)
	}
	defer iter.Close()

	// Collect all results
	var results []string
	for {
		dn, _, _, ok := iter.Next()
		if !ok {
			break
		}
		results = append(results, dn)
	}

	// Should include ou=users, alice, bob (not dc=example, ou=groups, cn=admins)
	expected := []string{
		"ou=users,dc=example,dc=com",
		"uid=alice,ou=users,dc=example,dc=com",
		"uid=bob,ou=users,dc=example,dc=com",
	}

	if len(results) != len(expected) {
		t.Errorf("expected %d entries, got %d: %v", len(expected), len(results), results)
	}

	resultSet := make(map[string]bool)
	for _, dn := range results {
		resultSet[dn] = true
	}
	for _, dn := range expected {
		if !resultSet[dn] {
			t.Errorf("missing DN: %s", dn)
		}
	}
}

func TestIteratorSubtreeScopeSingleEntry(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewRadixTree(pm)
	if err != nil {
		t.Fatalf("failed to create radix tree: %v", err)
	}

	// Insert single entry
	dn := "uid=alice,ou=users,dc=example,dc=com"
	err = tree.Insert(dn, storage.PageID(42), 7)
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	// Create iterator for subtree scope on leaf
	iter, err := tree.Iterator(dn, ScopeSubtree)
	if err != nil {
		t.Fatalf("failed to create iterator: %v", err)
	}
	defer iter.Close()

	// Should return only the entry itself
	resultDN, pageID, slotID, ok := iter.Next()
	if !ok {
		t.Error("expected to find entry")
	}
	if resultDN != dn {
		t.Errorf("expected DN %q, got %q", dn, resultDN)
	}
	if pageID != 42 || slotID != 7 {
		t.Errorf("expected (42, 7), got (%d, %d)", pageID, slotID)
	}

	// No more entries
	_, _, _, ok = iter.Next()
	if ok {
		t.Error("expected no more entries")
	}
}

// =============================================================================
// Empty Results Tests
// =============================================================================

func TestIteratorEmptyTree(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewRadixTree(pm)
	if err != nil {
		t.Fatalf("failed to create radix tree: %v", err)
	}

	// Try to iterate on non-existent base
	iter, err := tree.Iterator("dc=example,dc=com", ScopeSubtree)
	if err != nil {
		t.Fatalf("failed to create iterator: %v", err)
	}
	defer iter.Close()

	// Should return nothing
	_, _, _, ok := iter.Next()
	if ok {
		t.Error("expected no entries in empty tree")
	}
}

func TestIteratorNonExistentBase(t *testing.T) {
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

	// Try to iterate on non-existent base
	iter, err := tree.Iterator("ou=groups,dc=example,dc=com", ScopeSubtree)
	if err != nil {
		t.Fatalf("failed to create iterator: %v", err)
	}
	defer iter.Close()

	// Should return nothing
	_, _, _, ok := iter.Next()
	if ok {
		t.Error("expected no entries for non-existent base")
	}
}

// =============================================================================
// Iterator Methods Tests
// =============================================================================

func TestIteratorReset(t *testing.T) {
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

	iter, err := tree.Iterator(dn, ScopeBase)
	if err != nil {
		t.Fatalf("failed to create iterator: %v", err)
	}
	defer iter.Close()

	// First iteration
	_, _, _, ok := iter.Next()
	if !ok {
		t.Error("expected to find entry")
	}
	_, _, _, ok = iter.Next()
	if ok {
		t.Error("expected no more entries")
	}

	// Reset and iterate again
	iter.Reset()
	if iter.IsDone() {
		t.Error("expected iterator not to be done after reset")
	}

	resultDN, _, _, ok := iter.Next()
	if !ok {
		t.Error("expected to find entry after reset")
	}
	if resultDN != dn {
		t.Errorf("expected DN %q, got %q", dn, resultDN)
	}
}

func TestIteratorCollect(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewRadixTree(pm)
	if err != nil {
		t.Fatalf("failed to create radix tree: %v", err)
	}

	// Insert entries
	dns := []string{
		"ou=users,dc=example,dc=com",
		"uid=alice,ou=users,dc=example,dc=com",
		"uid=bob,ou=users,dc=example,dc=com",
	}

	for i, dn := range dns {
		err = tree.Insert(dn, storage.PageID(uint64(i+1)), uint16(i))
		if err != nil {
			t.Fatalf("failed to insert %s: %v", dn, err)
		}
	}

	iter, err := tree.Iterator("ou=users,dc=example,dc=com", ScopeSubtree)
	if err != nil {
		t.Fatalf("failed to create iterator: %v", err)
	}
	defer iter.Close()

	entries := iter.Collect()
	if len(entries) != len(dns) {
		t.Errorf("expected %d entries, got %d", len(dns), len(entries))
	}

	// Verify entries
	dnSet := make(map[string]bool)
	for _, entry := range entries {
		dnSet[entry.DN] = true
	}
	for _, dn := range dns {
		if !dnSet[dn] {
			t.Errorf("missing DN in collected entries: %s", dn)
		}
	}
}

// =============================================================================
// Edge Cases Tests
// =============================================================================

func TestIteratorDeepHierarchy(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewRadixTree(pm)
	if err != nil {
		t.Fatalf("failed to create radix tree: %v", err)
	}

	// Create a deep hierarchy
	dns := []string{
		"dc=com",
		"dc=example,dc=com",
		"ou=departments,dc=example,dc=com",
		"ou=engineering,ou=departments,dc=example,dc=com",
		"ou=backend,ou=engineering,ou=departments,dc=example,dc=com",
		"uid=alice,ou=backend,ou=engineering,ou=departments,dc=example,dc=com",
	}

	for i, dn := range dns {
		err = tree.Insert(dn, storage.PageID(uint64(i+1)), uint16(i))
		if err != nil {
			t.Fatalf("failed to insert %s: %v", dn, err)
		}
	}

	// Test subtree from middle
	iter, err := tree.Iterator("ou=engineering,ou=departments,dc=example,dc=com", ScopeSubtree)
	if err != nil {
		t.Fatalf("failed to create iterator: %v", err)
	}
	defer iter.Close()

	entries := iter.Collect()
	if len(entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(entries))
	}
}

func TestIteratorMultipleBranches(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewRadixTree(pm)
	if err != nil {
		t.Fatalf("failed to create radix tree: %v", err)
	}

	// Create multiple branches
	dns := []string{
		"dc=example,dc=com",
		"ou=users,dc=example,dc=com",
		"uid=alice,ou=users,dc=example,dc=com",
		"uid=bob,ou=users,dc=example,dc=com",
		"ou=groups,dc=example,dc=com",
		"cn=admins,ou=groups,dc=example,dc=com",
		"cn=developers,ou=groups,dc=example,dc=com",
		"ou=services,dc=example,dc=com",
		"cn=ldap,ou=services,dc=example,dc=com",
	}

	for i, dn := range dns {
		err = tree.Insert(dn, storage.PageID(uint64(i+1)), uint16(i))
		if err != nil {
			t.Fatalf("failed to insert %s: %v", dn, err)
		}
	}

	// Test subtree from root
	iter, err := tree.Iterator("dc=example,dc=com", ScopeSubtree)
	if err != nil {
		t.Fatalf("failed to create iterator: %v", err)
	}
	defer iter.Close()

	entries := iter.Collect()
	if len(entries) != len(dns) {
		t.Errorf("expected %d entries, got %d", len(dns), len(entries))
	}

	// Test onelevel from root
	iter2, err := tree.Iterator("dc=example,dc=com", ScopeOneLevel)
	if err != nil {
		t.Fatalf("failed to create iterator: %v", err)
	}
	defer iter2.Close()

	entries2 := iter2.Collect()
	if len(entries2) != 3 {
		t.Errorf("expected 3 immediate children, got %d", len(entries2))
	}
}

func TestIteratorPageIDAndSlotID(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewRadixTree(pm)
	if err != nil {
		t.Fatalf("failed to create radix tree: %v", err)
	}

	// Insert with specific page and slot IDs
	testCases := []struct {
		dn     string
		pageID storage.PageID
		slotID uint16
	}{
		{"uid=alice,ou=users,dc=example,dc=com", 100, 10},
		{"uid=bob,ou=users,dc=example,dc=com", 200, 20},
		{"uid=charlie,ou=users,dc=example,dc=com", 300, 30},
	}

	for _, tc := range testCases {
		err = tree.Insert(tc.dn, tc.pageID, tc.slotID)
		if err != nil {
			t.Fatalf("failed to insert %s: %v", tc.dn, err)
		}
	}

	// Verify each entry returns correct page and slot IDs
	for _, tc := range testCases {
		iter, err := tree.Iterator(tc.dn, ScopeBase)
		if err != nil {
			t.Fatalf("failed to create iterator for %s: %v", tc.dn, err)
		}

		dn, pageID, slotID, ok := iter.Next()
		iter.Close()

		if !ok {
			t.Errorf("expected to find entry %s", tc.dn)
			continue
		}
		if dn != tc.dn {
			t.Errorf("expected DN %q, got %q", tc.dn, dn)
		}
		if pageID != tc.pageID {
			t.Errorf("expected pageID %d for %s, got %d", tc.pageID, tc.dn, pageID)
		}
		if slotID != tc.slotID {
			t.Errorf("expected slotID %d for %s, got %d", tc.slotID, tc.dn, slotID)
		}
	}
}

// =============================================================================
// Concurrent Access Tests
// =============================================================================

func TestIteratorConcurrentRead(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewRadixTree(pm)
	if err != nil {
		t.Fatalf("failed to create radix tree: %v", err)
	}

	// Insert entries
	for i := 0; i < 10; i++ {
		dn := "uid=user" + string(rune('0'+i)) + ",ou=users,dc=example,dc=com"
		err = tree.Insert(dn, storage.PageID(uint64(i+1)), uint16(i))
		if err != nil {
			t.Fatalf("failed to insert: %v", err)
		}
	}

	// Create multiple iterators concurrently
	done := make(chan bool, 3)

	for i := 0; i < 3; i++ {
		go func() {
			iter, err := tree.Iterator("ou=users,dc=example,dc=com", ScopeSubtree)
			if err != nil {
				t.Errorf("failed to create iterator: %v", err)
				done <- false
				return
			}
			defer iter.Close()

			count := 0
			for {
				_, _, _, ok := iter.Next()
				if !ok {
					break
				}
				count++
			}

			if count != 10 {
				t.Errorf("expected 10 entries, got %d", count)
				done <- false
				return
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 3; i++ {
		<-done
	}
}

// =============================================================================
// IteratorEntry Tests
// =============================================================================

func TestIteratorEntry(t *testing.T) {
	entry := IteratorEntry{
		DN:     "uid=alice,ou=users,dc=example,dc=com",
		PageID: storage.PageID(42),
		SlotID: 7,
	}

	if entry.DN != "uid=alice,ou=users,dc=example,dc=com" {
		t.Errorf("unexpected DN: %s", entry.DN)
	}
	if entry.PageID != 42 {
		t.Errorf("unexpected PageID: %d", entry.PageID)
	}
	if entry.SlotID != 7 {
		t.Errorf("unexpected SlotID: %d", entry.SlotID)
	}
}
