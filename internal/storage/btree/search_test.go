package btree

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/oba-ldap/oba/internal/storage"
)

// =============================================================================
// Iterator Tests
// =============================================================================

func TestIteratorBasic(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewBPlusTree(pm, 0)
	if err != nil {
		t.Fatalf("failed to create B+ tree: %v", err)
	}

	// Insert some keys
	keys := []string{"apple", "banana", "cherry", "date", "elderberry"}
	for i, k := range keys {
		err = tree.Insert([]byte(k), EntryRef{PageID: storage.PageID(i + 1), SlotID: uint16(i)})
		if err != nil {
			t.Fatalf("failed to insert %s: %v", k, err)
		}
	}

	// Test All iterator
	iter := tree.All()
	defer iter.Close()

	var collected []string
	for {
		key, _, ok := iter.Next()
		if !ok {
			break
		}
		collected = append(collected, string(key))
	}

	if len(collected) != len(keys) {
		t.Errorf("expected %d keys, got %d", len(keys), len(collected))
	}

	// Verify keys are in sorted order
	for i := 1; i < len(collected); i++ {
		if collected[i] < collected[i-1] {
			t.Errorf("keys not in sorted order: %s < %s", collected[i], collected[i-1])
		}
	}
}

func TestIteratorEmpty(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewBPlusTree(pm, 0)
	if err != nil {
		t.Fatalf("failed to create B+ tree: %v", err)
	}

	// Test iterator on empty tree
	iter := tree.All()
	defer iter.Close()

	_, _, ok := iter.Next()
	if ok {
		t.Error("expected no results from empty tree")
	}

	if iter.Valid() {
		t.Error("expected iterator to be invalid on empty tree")
	}
}

func TestIteratorClose(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewBPlusTree(pm, 0)
	if err != nil {
		t.Fatalf("failed to create B+ tree: %v", err)
	}

	// Insert some keys
	for i := 0; i < 10; i++ {
		err = tree.Insert([]byte(fmt.Sprintf("key%02d", i)), EntryRef{PageID: storage.PageID(i + 1), SlotID: 0})
		if err != nil {
			t.Fatalf("failed to insert: %v", err)
		}
	}

	iter := tree.All()

	// Read a few entries
	iter.Next()
	iter.Next()

	// Close the iterator
	iter.Close()

	// Verify no more results after close
	_, _, ok := iter.Next()
	if ok {
		t.Error("expected no results after close")
	}

	if iter.Valid() {
		t.Error("expected iterator to be invalid after close")
	}
}

func TestIteratorPeek(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewBPlusTree(pm, 0)
	if err != nil {
		t.Fatalf("failed to create B+ tree: %v", err)
	}

	// Insert some keys
	for i := 0; i < 5; i++ {
		err = tree.Insert([]byte(fmt.Sprintf("key%02d", i)), EntryRef{PageID: storage.PageID(i + 1), SlotID: 0})
		if err != nil {
			t.Fatalf("failed to insert: %v", err)
		}
	}

	iter := tree.All()
	defer iter.Close()

	// Peek should return the same value multiple times
	key1, _, ok1 := iter.Peek()
	key2, _, ok2 := iter.Peek()

	if !ok1 || !ok2 {
		t.Error("peek should return valid results")
	}

	if !bytes.Equal(key1, key2) {
		t.Errorf("peek should return same key: %s vs %s", key1, key2)
	}

	// Next should return the same value as peek
	key3, _, ok3 := iter.Next()
	if !ok3 {
		t.Error("next should return valid result")
	}

	if !bytes.Equal(key1, key3) {
		t.Errorf("next should return same key as peek: %s vs %s", key1, key3)
	}

	// Next peek should be different
	key4, _, ok4 := iter.Peek()
	if !ok4 {
		t.Error("peek should return valid result")
	}

	if bytes.Equal(key3, key4) {
		t.Error("peek after next should return different key")
	}
}

func TestIteratorCollect(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewBPlusTree(pm, 0)
	if err != nil {
		t.Fatalf("failed to create B+ tree: %v", err)
	}

	numKeys := 50
	for i := 0; i < numKeys; i++ {
		err = tree.Insert([]byte(fmt.Sprintf("key%03d", i)), EntryRef{PageID: storage.PageID(i + 1), SlotID: 0})
		if err != nil {
			t.Fatalf("failed to insert: %v", err)
		}
	}

	iter := tree.All()
	keys, refs := iter.Collect()

	if len(keys) != numKeys {
		t.Errorf("expected %d keys, got %d", numKeys, len(keys))
	}

	if len(refs) != numKeys {
		t.Errorf("expected %d refs, got %d", numKeys, len(refs))
	}

	// Iterator should be exhausted
	_, _, ok := iter.Next()
	if ok {
		t.Error("iterator should be exhausted after collect")
	}
}

func TestIteratorCollectRefs(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewBPlusTree(pm, 0)
	if err != nil {
		t.Fatalf("failed to create B+ tree: %v", err)
	}

	numKeys := 30
	for i := 0; i < numKeys; i++ {
		err = tree.Insert([]byte(fmt.Sprintf("key%03d", i)), EntryRef{PageID: storage.PageID(i + 1), SlotID: uint16(i)})
		if err != nil {
			t.Fatalf("failed to insert: %v", err)
		}
	}

	iter := tree.All()
	refs := iter.CollectRefs()

	if len(refs) != numKeys {
		t.Errorf("expected %d refs, got %d", numKeys, len(refs))
	}
}

func TestIteratorCount(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewBPlusTree(pm, 0)
	if err != nil {
		t.Fatalf("failed to create B+ tree: %v", err)
	}

	numKeys := 25
	for i := 0; i < numKeys; i++ {
		err = tree.Insert([]byte(fmt.Sprintf("key%03d", i)), EntryRef{PageID: storage.PageID(i + 1), SlotID: 0})
		if err != nil {
			t.Fatalf("failed to insert: %v", err)
		}
	}

	iter := tree.All()
	count := iter.Count()

	if count != numKeys {
		t.Errorf("expected count %d, got %d", numKeys, count)
	}
}

func TestIteratorSkip(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewBPlusTree(pm, 0)
	if err != nil {
		t.Fatalf("failed to create B+ tree: %v", err)
	}

	numKeys := 20
	for i := 0; i < numKeys; i++ {
		err = tree.Insert([]byte(fmt.Sprintf("key%03d", i)), EntryRef{PageID: storage.PageID(i + 1), SlotID: 0})
		if err != nil {
			t.Fatalf("failed to insert: %v", err)
		}
	}

	iter := tree.All()
	defer iter.Close()

	// Skip 5 entries
	skipped := iter.Skip(5)
	if skipped != 5 {
		t.Errorf("expected to skip 5, skipped %d", skipped)
	}

	// Next should return key005
	key, _, ok := iter.Next()
	if !ok {
		t.Error("expected valid result after skip")
	}

	if string(key) != "key005" {
		t.Errorf("expected key005, got %s", key)
	}
}

func TestIteratorTake(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewBPlusTree(pm, 0)
	if err != nil {
		t.Fatalf("failed to create B+ tree: %v", err)
	}

	numKeys := 20
	for i := 0; i < numKeys; i++ {
		err = tree.Insert([]byte(fmt.Sprintf("key%03d", i)), EntryRef{PageID: storage.PageID(i + 1), SlotID: 0})
		if err != nil {
			t.Fatalf("failed to insert: %v", err)
		}
	}

	iter := tree.All()
	defer iter.Close()

	// Take 5 entries
	keys, refs := iter.Take(5)

	if len(keys) != 5 {
		t.Errorf("expected 5 keys, got %d", len(keys))
	}

	if len(refs) != 5 {
		t.Errorf("expected 5 refs, got %d", len(refs))
	}

	// Verify first key
	if string(keys[0]) != "key000" {
		t.Errorf("expected key000, got %s", keys[0])
	}

	// Verify last key
	if string(keys[4]) != "key004" {
		t.Errorf("expected key004, got %s", keys[4])
	}
}

// =============================================================================
// Range Search Tests
// =============================================================================

func TestRangeIterator(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewBPlusTree(pm, 0)
	if err != nil {
		t.Fatalf("failed to create B+ tree: %v", err)
	}

	// Insert keys
	for i := 0; i < 100; i++ {
		key := []byte(fmt.Sprintf("key%03d", i))
		err = tree.Insert(key, EntryRef{PageID: storage.PageID(i + 1), SlotID: 0})
		if err != nil {
			t.Fatalf("failed to insert: %v", err)
		}
	}

	// Test range [key020, key030]
	iter := tree.Range([]byte("key020"), []byte("key030"))
	defer iter.Close()

	count := 0
	for {
		key, _, ok := iter.Next()
		if !ok {
			break
		}
		count++

		// Verify key is in range
		if string(key) < "key020" || string(key) > "key030" {
			t.Errorf("key %s is out of range [key020, key030]", key)
		}
	}

	if count != 11 { // 020 to 030 inclusive
		t.Errorf("expected 11 results, got %d", count)
	}
}

func TestRangeIteratorOpenStart(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewBPlusTree(pm, 0)
	if err != nil {
		t.Fatalf("failed to create B+ tree: %v", err)
	}

	// Insert keys
	for i := 0; i < 50; i++ {
		key := []byte(fmt.Sprintf("key%03d", i))
		err = tree.Insert(key, EntryRef{PageID: storage.PageID(i + 1), SlotID: 0})
		if err != nil {
			t.Fatalf("failed to insert: %v", err)
		}
	}

	// Test range [nil, key010]
	iter := tree.Range(nil, []byte("key010"))
	defer iter.Close()

	count := iter.Count()

	if count != 11 { // 000 to 010 inclusive
		t.Errorf("expected 11 results, got %d", count)
	}
}

func TestRangeIteratorOpenEnd(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewBPlusTree(pm, 0)
	if err != nil {
		t.Fatalf("failed to create B+ tree: %v", err)
	}

	// Insert keys
	for i := 0; i < 50; i++ {
		key := []byte(fmt.Sprintf("key%03d", i))
		err = tree.Insert(key, EntryRef{PageID: storage.PageID(i + 1), SlotID: 0})
		if err != nil {
			t.Fatalf("failed to insert: %v", err)
		}
	}

	// Test range [key040, nil]
	iter := tree.Range([]byte("key040"), nil)
	defer iter.Close()

	count := iter.Count()

	if count != 10 { // 040 to 049 inclusive
		t.Errorf("expected 10 results, got %d", count)
	}
}

// =============================================================================
// Prefix Search Tests
// =============================================================================

func TestPrefixIterator(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewBPlusTree(pm, 0)
	if err != nil {
		t.Fatalf("failed to create B+ tree: %v", err)
	}

	// Insert keys with different prefixes
	prefixes := []string{"admin", "user", "guest"}
	for _, prefix := range prefixes {
		for i := 0; i < 10; i++ {
			key := []byte(fmt.Sprintf("%s%02d", prefix, i))
			err = tree.Insert(key, EntryRef{PageID: storage.PageID(i + 1), SlotID: 0})
			if err != nil {
				t.Fatalf("failed to insert: %v", err)
			}
		}
	}

	// Test prefix "user"
	iter := tree.Prefix([]byte("user"))
	defer iter.Close()

	count := 0
	for {
		key, _, ok := iter.Next()
		if !ok {
			break
		}
		count++

		if !bytes.HasPrefix(key, []byte("user")) {
			t.Errorf("key %s does not have prefix 'user'", key)
		}
	}

	if count != 10 {
		t.Errorf("expected 10 results for prefix 'user', got %d", count)
	}
}

func TestPrefixIteratorNoMatch(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewBPlusTree(pm, 0)
	if err != nil {
		t.Fatalf("failed to create B+ tree: %v", err)
	}

	// Insert keys
	for i := 0; i < 10; i++ {
		key := []byte(fmt.Sprintf("key%02d", i))
		err = tree.Insert(key, EntryRef{PageID: storage.PageID(i + 1), SlotID: 0})
		if err != nil {
			t.Fatalf("failed to insert: %v", err)
		}
	}

	// Test prefix that doesn't match
	iter := tree.Prefix([]byte("nomatch"))
	defer iter.Close()

	count := iter.Count()

	if count != 0 {
		t.Errorf("expected 0 results for non-matching prefix, got %d", count)
	}
}

func TestPrefixIteratorEmptyPrefix(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewBPlusTree(pm, 0)
	if err != nil {
		t.Fatalf("failed to create B+ tree: %v", err)
	}

	// Insert keys
	numKeys := 20
	for i := 0; i < numKeys; i++ {
		key := []byte(fmt.Sprintf("key%02d", i))
		err = tree.Insert(key, EntryRef{PageID: storage.PageID(i + 1), SlotID: 0})
		if err != nil {
			t.Fatalf("failed to insert: %v", err)
		}
	}

	// Empty prefix should match all
	iter := tree.Prefix([]byte{})
	defer iter.Close()

	count := iter.Count()

	if count != numKeys {
		t.Errorf("expected %d results for empty prefix, got %d", numKeys, count)
	}
}

func TestSearchPrefix(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewBPlusTree(pm, 0)
	if err != nil {
		t.Fatalf("failed to create B+ tree: %v", err)
	}

	// Insert keys
	keys := []string{"admin01", "admin02", "admin03", "user01", "user02", "guest01"}
	for i, k := range keys {
		err = tree.Insert([]byte(k), EntryRef{PageID: storage.PageID(i + 1), SlotID: 0})
		if err != nil {
			t.Fatalf("failed to insert: %v", err)
		}
	}

	// Search for prefix "admin"
	refs, err := tree.SearchPrefix([]byte("admin"))
	if err != nil {
		t.Fatalf("failed to search prefix: %v", err)
	}

	if len(refs) != 3 {
		t.Errorf("expected 3 results for prefix 'admin', got %d", len(refs))
	}
}

// =============================================================================
// Greater Than / Less Than Tests
// =============================================================================

func TestGreaterThan(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewBPlusTree(pm, 0)
	if err != nil {
		t.Fatalf("failed to create B+ tree: %v", err)
	}

	// Insert keys
	for i := 0; i < 20; i++ {
		key := []byte(fmt.Sprintf("key%02d", i))
		err = tree.Insert(key, EntryRef{PageID: storage.PageID(i + 1), SlotID: 0})
		if err != nil {
			t.Fatalf("failed to insert: %v", err)
		}
	}

	// Test greater than key10
	iter := tree.GreaterThan([]byte("key10"))
	defer iter.Close()

	count := 0
	for {
		key, _, ok := iter.Next()
		if !ok {
			break
		}
		count++

		if string(key) <= "key10" {
			t.Errorf("key %s should be > key10", key)
		}
	}

	if count != 9 { // key11 to key19
		t.Errorf("expected 9 results, got %d", count)
	}
}

func TestGreaterThanOrEqual(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewBPlusTree(pm, 0)
	if err != nil {
		t.Fatalf("failed to create B+ tree: %v", err)
	}

	// Insert keys
	for i := 0; i < 20; i++ {
		key := []byte(fmt.Sprintf("key%02d", i))
		err = tree.Insert(key, EntryRef{PageID: storage.PageID(i + 1), SlotID: 0})
		if err != nil {
			t.Fatalf("failed to insert: %v", err)
		}
	}

	// Test greater than or equal to key10
	iter := tree.GreaterThanOrEqual([]byte("key10"))
	defer iter.Close()

	count := 0
	for {
		key, _, ok := iter.Next()
		if !ok {
			break
		}
		count++

		if string(key) < "key10" {
			t.Errorf("key %s should be >= key10", key)
		}
	}

	if count != 10 { // key10 to key19
		t.Errorf("expected 10 results, got %d", count)
	}
}

func TestLessThan(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewBPlusTree(pm, 0)
	if err != nil {
		t.Fatalf("failed to create B+ tree: %v", err)
	}

	// Insert keys
	for i := 0; i < 20; i++ {
		key := []byte(fmt.Sprintf("key%02d", i))
		err = tree.Insert(key, EntryRef{PageID: storage.PageID(i + 1), SlotID: 0})
		if err != nil {
			t.Fatalf("failed to insert: %v", err)
		}
	}

	// Test less than key10
	iter := tree.LessThan([]byte("key10"))
	defer iter.Close()

	count := 0
	for {
		key, _, ok := iter.Next()
		if !ok {
			break
		}
		count++

		if string(key) >= "key10" {
			t.Errorf("key %s should be < key10", key)
		}
	}

	if count != 10 { // key00 to key09
		t.Errorf("expected 10 results, got %d", count)
	}
}

func TestLessThanOrEqual(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewBPlusTree(pm, 0)
	if err != nil {
		t.Fatalf("failed to create B+ tree: %v", err)
	}

	// Insert keys
	for i := 0; i < 20; i++ {
		key := []byte(fmt.Sprintf("key%02d", i))
		err = tree.Insert(key, EntryRef{PageID: storage.PageID(i + 1), SlotID: 0})
		if err != nil {
			t.Fatalf("failed to insert: %v", err)
		}
	}

	// Test less than or equal to key10
	iter := tree.LessThanOrEqual([]byte("key10"))
	defer iter.Close()

	count := 0
	for {
		key, _, ok := iter.Next()
		if !ok {
			break
		}
		count++

		if string(key) > "key10" {
			t.Errorf("key %s should be <= key10", key)
		}
	}

	if count != 11 { // key00 to key10
		t.Errorf("expected 11 results, got %d", count)
	}
}

// =============================================================================
// Search Helper Function Tests
// =============================================================================

func TestSearchGreaterThan(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewBPlusTree(pm, 0)
	if err != nil {
		t.Fatalf("failed to create B+ tree: %v", err)
	}

	// Insert keys
	for i := 0; i < 10; i++ {
		key := []byte(fmt.Sprintf("key%02d", i))
		err = tree.Insert(key, EntryRef{PageID: storage.PageID(i + 1), SlotID: 0})
		if err != nil {
			t.Fatalf("failed to insert: %v", err)
		}
	}

	refs, err := tree.SearchGreaterThan([]byte("key05"))
	if err != nil {
		t.Fatalf("failed to search: %v", err)
	}

	if len(refs) != 4 { // key06 to key09
		t.Errorf("expected 4 results, got %d", len(refs))
	}
}

func TestSearchLessThan(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewBPlusTree(pm, 0)
	if err != nil {
		t.Fatalf("failed to create B+ tree: %v", err)
	}

	// Insert keys
	for i := 0; i < 10; i++ {
		key := []byte(fmt.Sprintf("key%02d", i))
		err = tree.Insert(key, EntryRef{PageID: storage.PageID(i + 1), SlotID: 0})
		if err != nil {
			t.Fatalf("failed to insert: %v", err)
		}
	}

	refs, err := tree.SearchLessThan([]byte("key05"))
	if err != nil {
		t.Fatalf("failed to search: %v", err)
	}

	if len(refs) != 5 { // key00 to key04
		t.Errorf("expected 5 results, got %d", len(refs))
	}
}

func TestSearchGreaterThanOrEqual(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewBPlusTree(pm, 0)
	if err != nil {
		t.Fatalf("failed to create B+ tree: %v", err)
	}

	// Insert keys
	for i := 0; i < 10; i++ {
		key := []byte(fmt.Sprintf("key%02d", i))
		err = tree.Insert(key, EntryRef{PageID: storage.PageID(i + 1), SlotID: 0})
		if err != nil {
			t.Fatalf("failed to insert: %v", err)
		}
	}

	refs, err := tree.SearchGreaterThanOrEqual([]byte("key05"))
	if err != nil {
		t.Fatalf("failed to search: %v", err)
	}

	if len(refs) != 5 { // key05 to key09
		t.Errorf("expected 5 results, got %d", len(refs))
	}
}

func TestSearchLessThanOrEqual(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewBPlusTree(pm, 0)
	if err != nil {
		t.Fatalf("failed to create B+ tree: %v", err)
	}

	// Insert keys
	for i := 0; i < 10; i++ {
		key := []byte(fmt.Sprintf("key%02d", i))
		err = tree.Insert(key, EntryRef{PageID: storage.PageID(i + 1), SlotID: 0})
		if err != nil {
			t.Fatalf("failed to insert: %v", err)
		}
	}

	refs, err := tree.SearchLessThanOrEqual([]byte("key05"))
	if err != nil {
		t.Fatalf("failed to search: %v", err)
	}

	if len(refs) != 6 { // key00 to key05
		t.Errorf("expected 6 results, got %d", len(refs))
	}
}

func TestSearchEmptyKeyErrors(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewBPlusTree(pm, 0)
	if err != nil {
		t.Fatalf("failed to create B+ tree: %v", err)
	}

	// Test empty key errors
	_, err = tree.SearchGreaterThan([]byte{})
	if err != ErrEmptyKey {
		t.Errorf("expected ErrEmptyKey, got %v", err)
	}

	_, err = tree.SearchLessThan([]byte{})
	if err != ErrEmptyKey {
		t.Errorf("expected ErrEmptyKey, got %v", err)
	}

	_, err = tree.SearchGreaterThanOrEqual([]byte{})
	if err != ErrEmptyKey {
		t.Errorf("expected ErrEmptyKey, got %v", err)
	}

	_, err = tree.SearchLessThanOrEqual([]byte{})
	if err != ErrEmptyKey {
		t.Errorf("expected ErrEmptyKey, got %v", err)
	}
}

// =============================================================================
// Contains, First, Last Tests
// =============================================================================

func TestContains(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewBPlusTree(pm, 0)
	if err != nil {
		t.Fatalf("failed to create B+ tree: %v", err)
	}

	// Insert keys
	for i := 0; i < 10; i++ {
		key := []byte(fmt.Sprintf("key%02d", i))
		err = tree.Insert(key, EntryRef{PageID: storage.PageID(i + 1), SlotID: 0})
		if err != nil {
			t.Fatalf("failed to insert: %v", err)
		}
	}

	// Test existing key
	exists, err := tree.Contains([]byte("key05"))
	if err != nil {
		t.Fatalf("failed to check contains: %v", err)
	}
	if !exists {
		t.Error("expected key05 to exist")
	}

	// Test non-existing key
	exists, err = tree.Contains([]byte("key99"))
	if err != nil {
		t.Fatalf("failed to check contains: %v", err)
	}
	if exists {
		t.Error("expected key99 to not exist")
	}
}

func TestContainsEmptyKey(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewBPlusTree(pm, 0)
	if err != nil {
		t.Fatalf("failed to create B+ tree: %v", err)
	}

	_, err = tree.Contains([]byte{})
	if err != ErrEmptyKey {
		t.Errorf("expected ErrEmptyKey, got %v", err)
	}
}

func TestFirst(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewBPlusTree(pm, 0)
	if err != nil {
		t.Fatalf("failed to create B+ tree: %v", err)
	}

	// Insert keys in random order
	keys := []string{"cherry", "apple", "banana", "date"}
	for i, k := range keys {
		err = tree.Insert([]byte(k), EntryRef{PageID: storage.PageID(i + 1), SlotID: 0})
		if err != nil {
			t.Fatalf("failed to insert: %v", err)
		}
	}

	key, ref, err := tree.First()
	if err != nil {
		t.Fatalf("failed to get first: %v", err)
	}

	if string(key) != "apple" {
		t.Errorf("expected first key 'apple', got '%s'", key)
	}

	if ref.PageID != 2 { // apple was inserted second
		t.Errorf("expected PageID 2, got %d", ref.PageID)
	}
}

func TestFirstEmptyTree(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewBPlusTree(pm, 0)
	if err != nil {
		t.Fatalf("failed to create B+ tree: %v", err)
	}

	_, _, err = tree.First()
	if err != ErrTreeEmpty {
		t.Errorf("expected ErrTreeEmpty, got %v", err)
	}
}

func TestLast(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewBPlusTree(pm, 0)
	if err != nil {
		t.Fatalf("failed to create B+ tree: %v", err)
	}

	// Insert keys in random order
	keys := []string{"cherry", "apple", "banana", "date"}
	for i, k := range keys {
		err = tree.Insert([]byte(k), EntryRef{PageID: storage.PageID(i + 1), SlotID: 0})
		if err != nil {
			t.Fatalf("failed to insert: %v", err)
		}
	}

	key, ref, err := tree.Last()
	if err != nil {
		t.Fatalf("failed to get last: %v", err)
	}

	if string(key) != "date" {
		t.Errorf("expected last key 'date', got '%s'", key)
	}

	if ref.PageID != 4 { // date was inserted fourth
		t.Errorf("expected PageID 4, got %d", ref.PageID)
	}
}

func TestLastEmptyTree(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewBPlusTree(pm, 0)
	if err != nil {
		t.Fatalf("failed to create B+ tree: %v", err)
	}

	_, _, err = tree.Last()
	if err != ErrTreeEmpty {
		t.Errorf("expected ErrTreeEmpty, got %v", err)
	}
}

// =============================================================================
// Reverse Iterator Tests
// =============================================================================

func TestReverseIterator(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewBPlusTree(pm, 0)
	if err != nil {
		t.Fatalf("failed to create B+ tree: %v", err)
	}

	// Insert keys
	for i := 0; i < 20; i++ {
		key := []byte(fmt.Sprintf("key%02d", i))
		err = tree.Insert(key, EntryRef{PageID: storage.PageID(i + 1), SlotID: 0})
		if err != nil {
			t.Fatalf("failed to insert: %v", err)
		}
	}

	// Test reverse iteration
	iter := tree.RangeReverse(nil, nil)
	defer iter.Close()

	var keys []string
	for {
		key, _, ok := iter.Next()
		if !ok {
			break
		}
		keys = append(keys, string(key))
	}

	if len(keys) != 20 {
		t.Errorf("expected 20 keys, got %d", len(keys))
	}

	// Verify keys are in reverse order
	for i := 1; i < len(keys); i++ {
		if keys[i] > keys[i-1] {
			t.Errorf("keys not in reverse order: %s > %s", keys[i], keys[i-1])
		}
	}

	// First key should be key19
	if keys[0] != "key19" {
		t.Errorf("expected first key 'key19', got '%s'", keys[0])
	}

	// Last key should be key00
	if keys[len(keys)-1] != "key00" {
		t.Errorf("expected last key 'key00', got '%s'", keys[len(keys)-1])
	}
}

func TestReverseIteratorWithBounds(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewBPlusTree(pm, 0)
	if err != nil {
		t.Fatalf("failed to create B+ tree: %v", err)
	}

	// Insert keys
	for i := 0; i < 20; i++ {
		key := []byte(fmt.Sprintf("key%02d", i))
		err = tree.Insert(key, EntryRef{PageID: storage.PageID(i + 1), SlotID: 0})
		if err != nil {
			t.Fatalf("failed to insert: %v", err)
		}
	}

	// Test reverse iteration with bounds [key05, key15]
	iter := tree.RangeReverse([]byte("key05"), []byte("key15"))
	defer iter.Close()

	var keys []string
	for {
		key, _, ok := iter.Next()
		if !ok {
			break
		}
		keys = append(keys, string(key))
	}

	if len(keys) != 11 { // key05 to key15
		t.Errorf("expected 11 keys, got %d", len(keys))
	}

	// First key should be key15
	if keys[0] != "key15" {
		t.Errorf("expected first key 'key15', got '%s'", keys[0])
	}

	// Last key should be key05
	if keys[len(keys)-1] != "key05" {
		t.Errorf("expected last key 'key05', got '%s'", keys[len(keys)-1])
	}
}

// =============================================================================
// Large Dataset Tests
// =============================================================================

func TestIteratorLargeDataset(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewBPlusTree(pm, 0)
	if err != nil {
		t.Fatalf("failed to create B+ tree: %v", err)
	}

	// Insert many keys to cause multiple leaf splits
	numKeys := 1000
	for i := 0; i < numKeys; i++ {
		key := []byte(fmt.Sprintf("key%05d", i))
		err = tree.Insert(key, EntryRef{PageID: storage.PageID(i + 1), SlotID: 0})
		if err != nil {
			t.Fatalf("failed to insert key %d: %v", i, err)
		}
	}

	// Test All iterator
	iter := tree.All()
	count := iter.Count()

	if count != numKeys {
		t.Errorf("expected %d keys, got %d", numKeys, count)
	}

	// Test range iterator
	iter = tree.Range([]byte("key00100"), []byte("key00200"))
	count = iter.Count()

	if count != 101 { // 00100 to 00200 inclusive
		t.Errorf("expected 101 keys in range, got %d", count)
	}

	// Test prefix iterator
	iter = tree.Prefix([]byte("key001"))
	count = iter.Count()

	if count != 100 { // key00100 to key00199
		t.Errorf("expected 100 keys with prefix 'key001', got %d", count)
	}
}

func TestIteratorAcrossMultipleLeaves(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewBPlusTree(pm, 0)
	if err != nil {
		t.Fatalf("failed to create B+ tree: %v", err)
	}

	// Insert enough keys to span multiple leaves
	numKeys := BPlusLeafCapacity * 3
	for i := 0; i < numKeys; i++ {
		key := []byte(fmt.Sprintf("key%05d", i))
		err = tree.Insert(key, EntryRef{PageID: storage.PageID(i + 1), SlotID: 0})
		if err != nil {
			t.Fatalf("failed to insert key %d: %v", i, err)
		}
	}

	// Verify tree has multiple leaves
	stats, err := tree.Stats()
	if err != nil {
		t.Fatalf("failed to get stats: %v", err)
	}

	if stats.LeafNodes < 3 {
		t.Errorf("expected at least 3 leaf nodes, got %d", stats.LeafNodes)
	}

	// Test iterator traverses all leaves
	iter := tree.All()
	count := iter.Count()

	if count != numKeys {
		t.Errorf("expected %d keys, got %d", numKeys, count)
	}
}

// =============================================================================
// Edge Cases
// =============================================================================

func TestIteratorSingleKey(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewBPlusTree(pm, 0)
	if err != nil {
		t.Fatalf("failed to create B+ tree: %v", err)
	}

	// Insert single key
	err = tree.Insert([]byte("only"), EntryRef{PageID: 1, SlotID: 0})
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	// Test All iterator
	iter := tree.All()
	key, ref, ok := iter.Next()

	if !ok {
		t.Error("expected one result")
	}

	if string(key) != "only" {
		t.Errorf("expected key 'only', got '%s'", key)
	}

	if ref.PageID != 1 {
		t.Errorf("expected PageID 1, got %d", ref.PageID)
	}

	// No more results
	_, _, ok = iter.Next()
	if ok {
		t.Error("expected no more results")
	}
}

func TestIteratorDuplicateKeys(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewBPlusTree(pm, 0)
	if err != nil {
		t.Fatalf("failed to create B+ tree: %v", err)
	}

	// Insert duplicate keys
	key := []byte("duplicate")
	for i := 0; i < 5; i++ {
		err = tree.Insert(key, EntryRef{PageID: storage.PageID(i + 1), SlotID: uint16(i)})
		if err != nil {
			t.Fatalf("failed to insert: %v", err)
		}
	}

	// Test All iterator
	iter := tree.All()
	count := iter.Count()

	if count != 5 {
		t.Errorf("expected 5 results for duplicate keys, got %d", count)
	}
}

func TestRangeNoResults(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewBPlusTree(pm, 0)
	if err != nil {
		t.Fatalf("failed to create B+ tree: %v", err)
	}

	// Insert keys
	for i := 0; i < 10; i++ {
		key := []byte(fmt.Sprintf("key%02d", i))
		err = tree.Insert(key, EntryRef{PageID: storage.PageID(i + 1), SlotID: 0})
		if err != nil {
			t.Fatalf("failed to insert: %v", err)
		}
	}

	// Test range that doesn't match any keys
	iter := tree.Range([]byte("zzz"), []byte("zzzz"))
	count := iter.Count()

	if count != 0 {
		t.Errorf("expected 0 results for non-matching range, got %d", count)
	}
}

func TestIteratorValid(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	tree, err := NewBPlusTree(pm, 0)
	if err != nil {
		t.Fatalf("failed to create B+ tree: %v", err)
	}

	// Insert keys
	for i := 0; i < 5; i++ {
		key := []byte(fmt.Sprintf("key%02d", i))
		err = tree.Insert(key, EntryRef{PageID: storage.PageID(i + 1), SlotID: 0})
		if err != nil {
			t.Fatalf("failed to insert: %v", err)
		}
	}

	iter := tree.All()
	defer iter.Close()

	// Valid should be true initially
	if !iter.Valid() {
		t.Error("expected iterator to be valid initially")
	}

	// Exhaust the iterator
	for iter.Valid() {
		iter.Next()
	}

	// Valid should be false after exhaustion
	if iter.Valid() {
		t.Error("expected iterator to be invalid after exhaustion")
	}
}
