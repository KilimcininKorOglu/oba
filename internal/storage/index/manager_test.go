package index

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/oba-ldap/oba/internal/storage"
	"github.com/oba-ldap/oba/internal/storage/btree"
)

// Helper function to create a temporary page manager for testing.
func createTestPageManager(t *testing.T) (*storage.PageManager, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "index_test_*")
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
// IndexType Tests
// =============================================================================

func TestIndexTypeString(t *testing.T) {
	tests := []struct {
		indexType IndexType
		expected  string
	}{
		{IndexEquality, "equality"},
		{IndexPresence, "presence"},
		{IndexSubstring, "substring"},
		{IndexType(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.indexType.String(); got != tt.expected {
				t.Errorf("IndexType.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// =============================================================================
// Entry Tests
// =============================================================================

func TestNewEntry(t *testing.T) {
	entry := NewEntry("uid=alice,ou=users,dc=example,dc=com")

	if entry.DN != "uid=alice,ou=users,dc=example,dc=com" {
		t.Errorf("DN = %v, want uid=alice,ou=users,dc=example,dc=com", entry.DN)
	}

	if entry.Attributes == nil {
		t.Error("Attributes should not be nil")
	}
}

func TestEntryAttributes(t *testing.T) {
	entry := NewEntry("uid=alice,dc=example,dc=com")

	// Test SetAttribute and GetAttribute
	entry.SetAttribute("uid", [][]byte{[]byte("alice")})
	values := entry.GetAttribute("uid")

	if len(values) != 1 {
		t.Fatalf("expected 1 value, got %d", len(values))
	}

	if string(values[0]) != "alice" {
		t.Errorf("expected 'alice', got '%s'", string(values[0]))
	}

	// Test HasAttribute
	if !entry.HasAttribute("uid") {
		t.Error("expected HasAttribute('uid') to be true")
	}

	if entry.HasAttribute("nonexistent") {
		t.Error("expected HasAttribute('nonexistent') to be false")
	}

	// Test AddAttributeValue
	entry.AddAttributeValue("mail", []byte("alice@example.com"))
	entry.AddAttributeValue("mail", []byte("alice2@example.com"))

	mailValues := entry.GetAttribute("mail")
	if len(mailValues) != 2 {
		t.Errorf("expected 2 mail values, got %d", len(mailValues))
	}
}

func TestEntryRef(t *testing.T) {
	entry := NewEntry("uid=alice,dc=example,dc=com")
	entry.PageID = 42
	entry.SlotID = 5

	ref := entry.EntryRef()

	if ref.PageID != 42 {
		t.Errorf("PageID = %v, want 42", ref.PageID)
	}

	if ref.SlotID != 5 {
		t.Errorf("SlotID = %v, want 5", ref.SlotID)
	}
}

// =============================================================================
// IndexManager Creation Tests
// =============================================================================

func TestNewIndexManager(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	im, err := NewIndexManager(pm)
	if err != nil {
		t.Fatalf("failed to create index manager: %v", err)
	}
	defer im.Close()

	// Verify default indexes were created
	defaultAttrs := DefaultIndexedAttributes()
	for _, attr := range defaultAttrs {
		if _, exists := im.GetIndex(attr); !exists {
			t.Errorf("default index for '%s' not found", attr)
		}
	}
}

func TestNewIndexManagerNilPageManager(t *testing.T) {
	_, err := NewIndexManager(nil)
	if err != ErrInvalidPageManager {
		t.Errorf("expected ErrInvalidPageManager, got %v", err)
	}
}

// =============================================================================
// Index Creation Tests
// =============================================================================

func TestCreateIndex(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	im, err := NewIndexManager(pm)
	if err != nil {
		t.Fatalf("failed to create index manager: %v", err)
	}
	defer im.Close()

	// Create a new index
	err = im.CreateIndex("customattr", IndexEquality)
	if err != nil {
		t.Fatalf("failed to create index: %v", err)
	}

	// Verify the index exists
	idx, exists := im.GetIndex("customattr")
	if !exists {
		t.Error("index should exist")
	}

	if idx.Attribute != "customattr" {
		t.Errorf("Attribute = %v, want customattr", idx.Attribute)
	}

	if idx.Type != IndexEquality {
		t.Errorf("Type = %v, want IndexEquality", idx.Type)
	}
}

func TestCreateIndexDuplicate(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	im, err := NewIndexManager(pm)
	if err != nil {
		t.Fatalf("failed to create index manager: %v", err)
	}
	defer im.Close()

	// Create an index
	err = im.CreateIndex("testattr", IndexEquality)
	if err != nil {
		t.Fatalf("failed to create index: %v", err)
	}

	// Try to create the same index again
	err = im.CreateIndex("testattr", IndexEquality)
	if err != ErrIndexExists {
		t.Errorf("expected ErrIndexExists, got %v", err)
	}
}

func TestCreateIndexInvalidAttribute(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	im, err := NewIndexManager(pm)
	if err != nil {
		t.Fatalf("failed to create index manager: %v", err)
	}
	defer im.Close()

	// Empty attribute
	err = im.CreateIndex("", IndexEquality)
	if err != ErrInvalidAttribute {
		t.Errorf("expected ErrInvalidAttribute for empty attr, got %v", err)
	}

	// Whitespace only
	err = im.CreateIndex("   ", IndexEquality)
	if err != ErrInvalidAttribute {
		t.Errorf("expected ErrInvalidAttribute for whitespace attr, got %v", err)
	}
}

func TestCreateIndexNormalization(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	im, err := NewIndexManager(pm)
	if err != nil {
		t.Fatalf("failed to create index manager: %v", err)
	}
	defer im.Close()

	// Create with uppercase
	err = im.CreateIndex("TESTATTR", IndexEquality)
	if err != nil {
		t.Fatalf("failed to create index: %v", err)
	}

	// Should be normalized to lowercase
	idx, exists := im.GetIndex("testattr")
	if !exists {
		t.Error("index should exist with lowercase name")
	}

	if idx.Attribute != "testattr" {
		t.Errorf("Attribute = %v, want testattr", idx.Attribute)
	}
}

// =============================================================================
// Index Deletion Tests
// =============================================================================

func TestDropIndex(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	im, err := NewIndexManager(pm)
	if err != nil {
		t.Fatalf("failed to create index manager: %v", err)
	}
	defer im.Close()

	// Create an index
	err = im.CreateIndex("dropme", IndexEquality)
	if err != nil {
		t.Fatalf("failed to create index: %v", err)
	}

	// Verify it exists
	if _, exists := im.GetIndex("dropme"); !exists {
		t.Error("index should exist before drop")
	}

	// Drop the index
	err = im.DropIndex("dropme")
	if err != nil {
		t.Fatalf("failed to drop index: %v", err)
	}

	// Verify it's gone
	if _, exists := im.GetIndex("dropme"); exists {
		t.Error("index should not exist after drop")
	}
}

func TestDropIndexNotFound(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	im, err := NewIndexManager(pm)
	if err != nil {
		t.Fatalf("failed to create index manager: %v", err)
	}
	defer im.Close()

	err = im.DropIndex("nonexistent")
	if err != ErrIndexNotFound {
		t.Errorf("expected ErrIndexNotFound, got %v", err)
	}
}

// =============================================================================
// Index Persistence Tests
// =============================================================================

func TestIndexPersistence(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "index_persist_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")

	// Create index manager and add a custom index
	func() {
		opts := storage.DefaultOptions()
		opts.CreateIfNew = true

		pm, err := storage.OpenPageManager(dbPath, opts)
		if err != nil {
			t.Fatalf("failed to create page manager: %v", err)
		}

		im, err := NewIndexManager(pm)
		if err != nil {
			pm.Close()
			t.Fatalf("failed to create index manager: %v", err)
		}

		// Create a custom index
		err = im.CreateIndex("customattr", IndexPresence)
		if err != nil {
			im.Close()
			pm.Close()
			t.Fatalf("failed to create index: %v", err)
		}

		// Add some data to the index
		entry := NewEntry("uid=alice,dc=example,dc=com")
		entry.SetAttribute("customattr", [][]byte{[]byte("value1")})
		entry.PageID = 100
		entry.SlotID = 5

		err = im.UpdateIndexes(nil, entry)
		if err != nil {
			im.Close()
			pm.Close()
			t.Fatalf("failed to update indexes: %v", err)
		}

		im.Close()
		pm.Close()
	}()

	// Reopen and verify persistence
	func() {
		opts := storage.DefaultOptions()
		opts.CreateIfNew = false

		pm, err := storage.OpenPageManager(dbPath, opts)
		if err != nil {
			t.Fatalf("failed to reopen page manager: %v", err)
		}
		defer pm.Close()

		im, err := NewIndexManager(pm)
		if err != nil {
			t.Fatalf("failed to reopen index manager: %v", err)
		}
		defer im.Close()

		// Verify custom index exists
		idx, exists := im.GetIndex("customattr")
		if !exists {
			t.Error("custom index should persist across restarts")
		}

		if idx.Type != IndexPresence {
			t.Errorf("index type should be IndexPresence, got %v", idx.Type)
		}

		// Verify default indexes still exist
		for _, attr := range DefaultIndexedAttributes() {
			if _, exists := im.GetIndex(attr); !exists {
				t.Errorf("default index '%s' should persist", attr)
			}
		}
	}()
}

// =============================================================================
// Index Maintenance Tests
// =============================================================================

func TestUpdateIndexesInsert(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	im, err := NewIndexManager(pm)
	if err != nil {
		t.Fatalf("failed to create index manager: %v", err)
	}
	defer im.Close()

	// Create an entry
	entry := NewEntry("uid=alice,ou=users,dc=example,dc=com")
	entry.SetAttribute("uid", [][]byte{[]byte("alice")})
	entry.SetAttribute("cn", [][]byte{[]byte("Alice Smith")})
	entry.SetAttribute("mail", [][]byte{[]byte("alice@example.com")})
	entry.PageID = 42
	entry.SlotID = 5

	// Insert the entry
	err = im.UpdateIndexes(nil, entry)
	if err != nil {
		t.Fatalf("failed to update indexes: %v", err)
	}

	// Verify the entry can be found via uid index
	refs, err := im.Search("uid", []byte("alice"))
	if err != nil {
		t.Fatalf("failed to search: %v", err)
	}

	if len(refs) != 1 {
		t.Fatalf("expected 1 result, got %d", len(refs))
	}

	if refs[0].PageID != 42 || refs[0].SlotID != 5 {
		t.Errorf("expected ref {42, 5}, got {%d, %d}", refs[0].PageID, refs[0].SlotID)
	}
}

func TestUpdateIndexesDelete(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	im, err := NewIndexManager(pm)
	if err != nil {
		t.Fatalf("failed to create index manager: %v", err)
	}
	defer im.Close()

	// Create and insert an entry
	entry := NewEntry("uid=bob,dc=example,dc=com")
	entry.SetAttribute("uid", [][]byte{[]byte("bob")})
	entry.PageID = 50
	entry.SlotID = 3

	err = im.UpdateIndexes(nil, entry)
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	// Verify it exists
	refs, err := im.Search("uid", []byte("bob"))
	if err != nil {
		t.Fatalf("failed to search: %v", err)
	}
	if len(refs) != 1 {
		t.Fatalf("expected 1 result before delete, got %d", len(refs))
	}

	// Delete the entry
	err = im.UpdateIndexes(entry, nil)
	if err != nil {
		t.Fatalf("failed to delete: %v", err)
	}

	// Verify it's gone
	refs, err = im.Search("uid", []byte("bob"))
	if err != nil {
		t.Fatalf("failed to search after delete: %v", err)
	}
	if len(refs) != 0 {
		t.Errorf("expected 0 results after delete, got %d", len(refs))
	}
}

func TestUpdateIndexesModify(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	im, err := NewIndexManager(pm)
	if err != nil {
		t.Fatalf("failed to create index manager: %v", err)
	}
	defer im.Close()

	// Create old entry
	oldEntry := NewEntry("uid=charlie,dc=example,dc=com")
	oldEntry.SetAttribute("uid", [][]byte{[]byte("charlie")})
	oldEntry.SetAttribute("mail", [][]byte{[]byte("charlie@old.com")})
	oldEntry.PageID = 60
	oldEntry.SlotID = 7

	// Insert old entry
	err = im.UpdateIndexes(nil, oldEntry)
	if err != nil {
		t.Fatalf("failed to insert old entry: %v", err)
	}

	// Create new entry with modified mail
	newEntry := NewEntry("uid=charlie,dc=example,dc=com")
	newEntry.SetAttribute("uid", [][]byte{[]byte("charlie")})
	newEntry.SetAttribute("mail", [][]byte{[]byte("charlie@new.com")})
	newEntry.PageID = 60
	newEntry.SlotID = 7

	// Update the entry
	err = im.UpdateIndexes(oldEntry, newEntry)
	if err != nil {
		t.Fatalf("failed to update entry: %v", err)
	}

	// Verify old mail is gone
	refs, err := im.Search("mail", []byte("charlie@old.com"))
	if err != nil {
		t.Fatalf("failed to search old mail: %v", err)
	}
	if len(refs) != 0 {
		t.Errorf("expected 0 results for old mail, got %d", len(refs))
	}

	// Verify new mail exists
	refs, err = im.Search("mail", []byte("charlie@new.com"))
	if err != nil {
		t.Fatalf("failed to search new mail: %v", err)
	}
	if len(refs) != 1 {
		t.Errorf("expected 1 result for new mail, got %d", len(refs))
	}
}

// =============================================================================
// Search Tests
// =============================================================================

func TestSearchNotFound(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	im, err := NewIndexManager(pm)
	if err != nil {
		t.Fatalf("failed to create index manager: %v", err)
	}
	defer im.Close()

	refs, err := im.Search("uid", []byte("nonexistent"))
	if err != nil {
		t.Fatalf("search should not error: %v", err)
	}

	if len(refs) != 0 {
		t.Errorf("expected 0 results, got %d", len(refs))
	}
}

func TestSearchIndexNotFound(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	im, err := NewIndexManager(pm)
	if err != nil {
		t.Fatalf("failed to create index manager: %v", err)
	}
	defer im.Close()

	_, err = im.Search("nonexistentattr", []byte("value"))
	if err != ErrIndexNotFound {
		t.Errorf("expected ErrIndexNotFound, got %v", err)
	}
}

func TestSearchRange(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	im, err := NewIndexManager(pm)
	if err != nil {
		t.Fatalf("failed to create index manager: %v", err)
	}
	defer im.Close()

	// Insert multiple entries
	for i := 0; i < 10; i++ {
		entry := NewEntry("uid=user" + string(rune('0'+i)) + ",dc=example,dc=com")
		entry.SetAttribute("uid", [][]byte{[]byte("user" + string(rune('0'+i)))})
		entry.PageID = storage.PageID(i + 1)
		entry.SlotID = 0

		err = im.UpdateIndexes(nil, entry)
		if err != nil {
			t.Fatalf("failed to insert entry %d: %v", i, err)
		}
	}

	// Search range
	refs, err := im.SearchRange("uid", []byte("user3"), []byte("user7"))
	if err != nil {
		t.Fatalf("failed to search range: %v", err)
	}

	if len(refs) != 5 { // user3, user4, user5, user6, user7
		t.Errorf("expected 5 results, got %d", len(refs))
	}
}

// =============================================================================
// ListIndexes and IndexCount Tests
// =============================================================================

func TestListIndexes(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	im, err := NewIndexManager(pm)
	if err != nil {
		t.Fatalf("failed to create index manager: %v", err)
	}
	defer im.Close()

	indexes := im.ListIndexes()

	// Should have at least the default indexes
	if len(indexes) < len(DefaultIndexedAttributes()) {
		t.Errorf("expected at least %d indexes, got %d", len(DefaultIndexedAttributes()), len(indexes))
	}
}

func TestIndexCount(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	im, err := NewIndexManager(pm)
	if err != nil {
		t.Fatalf("failed to create index manager: %v", err)
	}
	defer im.Close()

	initialCount := im.IndexCount()

	// Add a new index
	err = im.CreateIndex("newattr", IndexEquality)
	if err != nil {
		t.Fatalf("failed to create index: %v", err)
	}

	if im.IndexCount() != initialCount+1 {
		t.Errorf("expected count %d, got %d", initialCount+1, im.IndexCount())
	}

	// Drop the index
	err = im.DropIndex("newattr")
	if err != nil {
		t.Fatalf("failed to drop index: %v", err)
	}

	if im.IndexCount() != initialCount {
		t.Errorf("expected count %d after drop, got %d", initialCount, im.IndexCount())
	}
}

// =============================================================================
// Close and Error Handling Tests
// =============================================================================

func TestCloseManager(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	im, err := NewIndexManager(pm)
	if err != nil {
		t.Fatalf("failed to create index manager: %v", err)
	}

	err = im.Close()
	if err != nil {
		t.Fatalf("failed to close: %v", err)
	}

	// Operations on closed manager should fail
	err = im.CreateIndex("test", IndexEquality)
	if err != ErrManagerClosed {
		t.Errorf("expected ErrManagerClosed, got %v", err)
	}

	err = im.DropIndex("uid")
	if err != ErrManagerClosed {
		t.Errorf("expected ErrManagerClosed for drop, got %v", err)
	}

	_, err = im.Search("uid", []byte("test"))
	if err != ErrManagerClosed {
		t.Errorf("expected ErrManagerClosed for search, got %v", err)
	}

	// Double close should return error
	err = im.Close()
	if err != ErrManagerClosed {
		t.Errorf("expected ErrManagerClosed for double close, got %v", err)
	}
}

// =============================================================================
// Substring Index Tests
// =============================================================================

func TestGenerateSubstrings(t *testing.T) {
	value := []byte("admin")
	substrings := generateSubstrings(value)

	// Should generate substrings of length >= 3
	// "adm", "admi", "admin", "dmi", "dmin", "min"
	expectedCount := 6

	if len(substrings) != expectedCount {
		t.Errorf("expected %d substrings, got %d", expectedCount, len(substrings))
	}

	// Verify some expected substrings
	found := make(map[string]bool)
	for _, s := range substrings {
		found[string(s)] = true
	}

	expected := []string{"adm", "admi", "admin", "dmi", "dmin", "min"}
	for _, e := range expected {
		if !found[e] {
			t.Errorf("expected substring '%s' not found", e)
		}
	}
}

func TestGenerateSubstringsEmpty(t *testing.T) {
	substrings := generateSubstrings([]byte{})
	if len(substrings) != 0 {
		t.Errorf("expected 0 substrings for empty value, got %d", len(substrings))
	}
}

func TestGenerateSubstringsShort(t *testing.T) {
	// Value shorter than minimum substring length
	substrings := generateSubstrings([]byte("ab"))
	if len(substrings) != 0 {
		t.Errorf("expected 0 substrings for short value, got %d", len(substrings))
	}
}

// =============================================================================
// Multiple Entries Tests
// =============================================================================

func TestMultipleEntriesSameAttribute(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	im, err := NewIndexManager(pm)
	if err != nil {
		t.Fatalf("failed to create index manager: %v", err)
	}
	defer im.Close()

	// Insert multiple entries with the same objectclass
	for i := 0; i < 5; i++ {
		entry := NewEntry("uid=user" + string(rune('0'+i)) + ",dc=example,dc=com")
		entry.SetAttribute("objectclass", [][]byte{[]byte("person")})
		entry.PageID = storage.PageID(i + 1)
		entry.SlotID = uint16(i)

		err = im.UpdateIndexes(nil, entry)
		if err != nil {
			t.Fatalf("failed to insert entry %d: %v", i, err)
		}
	}

	// Search for all entries with objectclass=person
	refs, err := im.Search("objectclass", []byte("person"))
	if err != nil {
		t.Fatalf("failed to search: %v", err)
	}

	if len(refs) != 5 {
		t.Errorf("expected 5 results, got %d", len(refs))
	}
}

// =============================================================================
// Concurrent Access Tests
// =============================================================================

func TestConcurrentIndexOperations(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	im, err := NewIndexManager(pm)
	if err != nil {
		t.Fatalf("failed to create index manager: %v", err)
	}
	defer im.Close()

	numGoroutines := 10
	entriesPerGoroutine := 20

	done := make(chan bool, numGoroutines)
	errors := make(chan error, numGoroutines*entriesPerGoroutine)

	for g := 0; g < numGoroutines; g++ {
		go func(goroutineID int) {
			for i := 0; i < entriesPerGoroutine; i++ {
				entry := NewEntry("uid=g" + string(rune('0'+goroutineID)) + "_user" + string(rune('0'+i)) + ",dc=example,dc=com")
				entry.SetAttribute("uid", [][]byte{[]byte("g" + string(rune('0'+goroutineID)) + "_user" + string(rune('0'+i)))})
				entry.PageID = storage.PageID(goroutineID*1000 + i)
				entry.SlotID = uint16(i)

				err := im.UpdateIndexes(nil, entry)
				if err != nil {
					errors <- err
				}
			}
			done <- true
		}(g)
	}

	// Wait for all goroutines
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	close(errors)
	for err := range errors {
		t.Errorf("concurrent operation error: %v", err)
	}
}

// =============================================================================
// Default Indexed Attributes Tests
// =============================================================================

func TestDefaultIndexedAttributes(t *testing.T) {
	attrs := DefaultIndexedAttributes()

	if len(attrs) == 0 {
		t.Error("expected at least one default indexed attribute")
	}

	// Verify expected attributes are in the list
	expected := []string{"objectclass", "uid", "cn"}
	for _, e := range expected {
		found := false
		for _, a := range attrs {
			if a == e {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected '%s' in default indexed attributes", e)
		}
	}
}

// =============================================================================
// Index Types Tests
// =============================================================================

func TestCreateIndexWithDifferentTypes(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	im, err := NewIndexManager(pm)
	if err != nil {
		t.Fatalf("failed to create index manager: %v", err)
	}
	defer im.Close()

	// Create equality index
	err = im.CreateIndex("eqattr", IndexEquality)
	if err != nil {
		t.Fatalf("failed to create equality index: %v", err)
	}

	// Create presence index
	err = im.CreateIndex("presattr", IndexPresence)
	if err != nil {
		t.Fatalf("failed to create presence index: %v", err)
	}

	// Create substring index
	err = im.CreateIndex("subattr", IndexSubstring)
	if err != nil {
		t.Fatalf("failed to create substring index: %v", err)
	}

	// Verify types
	idx, _ := im.GetIndex("eqattr")
	if idx.Type != IndexEquality {
		t.Errorf("expected IndexEquality, got %v", idx.Type)
	}

	idx, _ = im.GetIndex("presattr")
	if idx.Type != IndexPresence {
		t.Errorf("expected IndexPresence, got %v", idx.Type)
	}

	idx, _ = im.GetIndex("subattr")
	if idx.Type != IndexSubstring {
		t.Errorf("expected IndexSubstring, got %v", idx.Type)
	}
}

// =============================================================================
// Entry Reference Tests
// =============================================================================

func TestEntryRefFromBtree(t *testing.T) {
	ref := btree.EntryRef{
		PageID: 123,
		SlotID: 45,
	}

	if ref.PageID != 123 {
		t.Errorf("PageID = %v, want 123", ref.PageID)
	}

	if ref.SlotID != 45 {
		t.Errorf("SlotID = %v, want 45", ref.SlotID)
	}
}

// =============================================================================
// Sync Tests
// =============================================================================

func TestSync(t *testing.T) {
	pm, cleanup := createTestPageManager(t)
	defer cleanup()

	im, err := NewIndexManager(pm)
	if err != nil {
		t.Fatalf("failed to create index manager: %v", err)
	}
	defer im.Close()

	// Create an index
	err = im.CreateIndex("synctest", IndexEquality)
	if err != nil {
		t.Fatalf("failed to create index: %v", err)
	}

	// Sync should not error
	err = im.Sync()
	if err != nil {
		t.Errorf("sync failed: %v", err)
	}
}
