// Package backend provides the LDAP backend interface tests.
package backend

import (
	"errors"
	"testing"

	"github.com/oba-ldap/oba/internal/config"
	"github.com/oba-ldap/oba/internal/filter"
	"github.com/oba-ldap/oba/internal/server"
	"github.com/oba-ldap/oba/internal/storage"
)

// mockStorageEngine is a mock implementation of storage.StorageEngine for testing.
type mockStorageEngine struct {
	entries map[string]*storage.Entry
	txID    uint64
}

func newMockStorageEngine() *mockStorageEngine {
	return &mockStorageEngine{
		entries: make(map[string]*storage.Entry),
	}
}

func (m *mockStorageEngine) Begin() (interface{}, error) {
	m.txID++
	return m.txID, nil
}

func (m *mockStorageEngine) Commit(tx interface{}) error {
	return nil
}

func (m *mockStorageEngine) Rollback(tx interface{}) error {
	return nil
}

func (m *mockStorageEngine) Get(tx interface{}, dn string) (*storage.Entry, error) {
	entry, ok := m.entries[dn]
	if !ok {
		return nil, errors.New("entry not found")
	}
	return entry.Clone(), nil
}

func (m *mockStorageEngine) Put(tx interface{}, entry *storage.Entry) error {
	m.entries[entry.DN] = entry.Clone()
	return nil
}

func (m *mockStorageEngine) Delete(tx interface{}, dn string) error {
	if _, ok := m.entries[dn]; !ok {
		return errors.New("entry not found")
	}
	delete(m.entries, dn)
	return nil
}

func (m *mockStorageEngine) HasChildren(tx interface{}, dn string) (bool, error) {
	// Check if any entry has this DN as a parent
	for entryDN := range m.entries {
		if entryDN != dn && len(entryDN) > len(dn) {
			// Check if entryDN ends with ","+dn (is a child)
			suffix := "," + dn
			if len(entryDN) > len(suffix) && entryDN[len(entryDN)-len(suffix):] == suffix {
				return true, nil
			}
		}
	}
	return false, nil
}

func (m *mockStorageEngine) SearchByDN(tx interface{}, baseDN string, scope storage.Scope) storage.Iterator {
	var results []*storage.Entry
	for dn, entry := range m.entries {
		if matchesDNScope(dn, baseDN, scope) {
			results = append(results, entry.Clone())
		}
	}
	return &mockIterator{entries: results, index: -1}
}

func (m *mockStorageEngine) SearchByFilter(tx interface{}, baseDN string, f interface{}) storage.Iterator {
	var results []*storage.Entry
	matcher, _ := f.(storage.FilterMatcher)
	for dn, entry := range m.entries {
		if matchesDNScope(dn, baseDN, storage.ScopeSubtree) {
			if matcher == nil || matcher.Match(entry) {
				results = append(results, entry.Clone())
			}
		}
	}
	return &mockIterator{entries: results, index: -1}
}

func (m *mockStorageEngine) CreateIndex(attribute string, indexType storage.IndexType) error {
	return nil
}

func (m *mockStorageEngine) DropIndex(attribute string) error {
	return nil
}

func (m *mockStorageEngine) Checkpoint() error {
	return nil
}

func (m *mockStorageEngine) Compact() error {
	return nil
}

func (m *mockStorageEngine) Stats() *storage.EngineStats {
	return &storage.EngineStats{
		EntryCount: uint64(len(m.entries)),
	}
}

func (m *mockStorageEngine) Close() error {
	return nil
}

// matchesDNScope checks if a DN matches the base DN with the given scope.
func matchesDNScope(dn, baseDN string, scope storage.Scope) bool {
	if baseDN == "" {
		return true
	}
	switch scope {
	case storage.ScopeBase:
		return dn == baseDN
	case storage.ScopeOneLevel:
		// Simplified: check if DN ends with baseDN and has one more component
		return len(dn) > len(baseDN) && dn[len(dn)-len(baseDN):] == baseDN
	case storage.ScopeSubtree:
		return dn == baseDN || (len(dn) > len(baseDN) && dn[len(dn)-len(baseDN)-1:] == ","+baseDN)
	}
	return false
}

// mockIterator is a mock implementation of storage.Iterator.
type mockIterator struct {
	entries []*storage.Entry
	index   int
	err     error
}

func (it *mockIterator) Next() bool {
	it.index++
	return it.index < len(it.entries)
}

func (it *mockIterator) Entry() *storage.Entry {
	if it.index < 0 || it.index >= len(it.entries) {
		return nil
	}
	return it.entries[it.index]
}

func (it *mockIterator) Error() error {
	return it.err
}

func (it *mockIterator) Close() {}

// TestNewBackend tests creating a new backend.
func TestNewBackend(t *testing.T) {
	engine := newMockStorageEngine()
	cfg := &config.Config{
		Directory: config.DirectoryConfig{
			RootDN:       "cn=admin,dc=example,dc=com",
			RootPassword: "{CLEARTEXT}secret",
		},
	}

	backend := NewBackend(engine, cfg)

	if backend == nil {
		t.Fatal("expected backend to be created")
	}

	if backend.engine != engine {
		t.Error("expected engine to be set")
	}

	if backend.rootDN != "cn=admin,dc=example,dc=com" {
		t.Errorf("expected rootDN to be 'cn=admin,dc=example,dc=com', got '%s'", backend.rootDN)
	}
}

// TestNewBackendNilConfig tests creating a backend with nil config.
func TestNewBackendNilConfig(t *testing.T) {
	engine := newMockStorageEngine()
	backend := NewBackend(engine, nil)

	if backend == nil {
		t.Fatal("expected backend to be created")
	}

	if backend.rootDN != "" {
		t.Errorf("expected rootDN to be empty, got '%s'", backend.rootDN)
	}
}

// TestBindAnonymous tests anonymous bind.
func TestBindAnonymous(t *testing.T) {
	engine := newMockStorageEngine()
	backend := NewBackend(engine, nil)

	err := backend.Bind("", "")
	if err != nil {
		t.Errorf("expected anonymous bind to succeed, got error: %v", err)
	}
}

// TestBindRootDN tests root DN bind.
func TestBindRootDN(t *testing.T) {
	engine := newMockStorageEngine()
	cfg := &config.Config{
		Directory: config.DirectoryConfig{
			RootDN:       "cn=admin,dc=example,dc=com",
			RootPassword: "{CLEARTEXT}secret",
		},
	}
	backend := NewBackend(engine, cfg)

	tests := []struct {
		name     string
		dn       string
		password string
		wantErr  bool
	}{
		{"correct password", "cn=admin,dc=example,dc=com", "secret", false},
		{"wrong password", "cn=admin,dc=example,dc=com", "wrong", true},
		{"case insensitive DN", "CN=Admin,DC=Example,DC=Com", "secret", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := backend.Bind(tt.dn, tt.password)
			if (err != nil) != tt.wantErr {
				t.Errorf("Bind() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestBindUserEntry tests binding with a user entry.
func TestBindUserEntry(t *testing.T) {
	engine := newMockStorageEngine()
	backend := NewBackend(engine, nil)

	// Create a user entry with a password
	hashedPassword, _ := server.HashPassword("userpassword", server.SchemeSHA256)
	userEntry := storage.NewEntry("uid=alice,ou=users,dc=example,dc=com")
	userEntry.SetStringAttribute("objectclass", "person", "inetOrgPerson")
	userEntry.SetStringAttribute("uid", "alice")
	userEntry.SetStringAttribute("cn", "Alice Smith")
	userEntry.SetStringAttribute("userpassword", hashedPassword)
	engine.entries["uid=alice,ou=users,dc=example,dc=com"] = userEntry

	tests := []struct {
		name     string
		dn       string
		password string
		wantErr  bool
	}{
		{"correct password", "uid=alice,ou=users,dc=example,dc=com", "userpassword", false},
		{"wrong password", "uid=alice,ou=users,dc=example,dc=com", "wrongpassword", true},
		{"non-existent user", "uid=bob,ou=users,dc=example,dc=com", "password", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := backend.Bind(tt.dn, tt.password)
			if (err != nil) != tt.wantErr {
				t.Errorf("Bind() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestBindNoPassword tests binding with an entry that has no password.
func TestBindNoPassword(t *testing.T) {
	engine := newMockStorageEngine()
	backend := NewBackend(engine, nil)

	// Create a user entry without a password
	userEntry := storage.NewEntry("uid=alice,ou=users,dc=example,dc=com")
	userEntry.SetStringAttribute("objectclass", "person")
	userEntry.SetStringAttribute("uid", "alice")
	engine.entries["uid=alice,ou=users,dc=example,dc=com"] = userEntry

	err := backend.Bind("uid=alice,ou=users,dc=example,dc=com", "anypassword")
	if err != ErrNoPassword {
		t.Errorf("expected ErrNoPassword, got %v", err)
	}
}

// TestAdd tests adding entries.
func TestAdd(t *testing.T) {
	engine := newMockStorageEngine()
	backend := NewBackend(engine, nil)

	entry := NewEntry("uid=alice,ou=users,dc=example,dc=com")
	entry.SetAttribute("objectclass", "person", "inetOrgPerson")
	entry.SetAttribute("uid", "alice")
	entry.SetAttribute("cn", "Alice Smith")

	err := backend.Add(entry)
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	// Verify entry was added
	if _, ok := engine.entries["uid=alice,ou=users,dc=example,dc=com"]; !ok {
		t.Error("expected entry to be added to storage")
	}
}

// TestAddDuplicate tests adding a duplicate entry.
func TestAddDuplicate(t *testing.T) {
	engine := newMockStorageEngine()
	backend := NewBackend(engine, nil)

	// Add first entry
	entry := NewEntry("uid=alice,ou=users,dc=example,dc=com")
	entry.SetAttribute("objectclass", "person")
	entry.SetAttribute("uid", "alice")

	err := backend.Add(entry)
	if err != nil {
		t.Fatalf("first Add() error = %v", err)
	}

	// Try to add duplicate
	err = backend.Add(entry)
	if err != ErrEntryExists {
		t.Errorf("expected ErrEntryExists, got %v", err)
	}
}

// TestAddInvalidEntry tests adding invalid entries.
func TestAddInvalidEntry(t *testing.T) {
	engine := newMockStorageEngine()
	backend := NewBackend(engine, nil)

	tests := []struct {
		name    string
		entry   *Entry
		wantErr error
	}{
		{"nil entry", nil, ErrInvalidEntry},
		{"empty DN", NewEntry(""), ErrInvalidEntry},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := backend.Add(tt.entry)
			if err != tt.wantErr {
				t.Errorf("Add() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestDelete tests deleting entries.
func TestDelete(t *testing.T) {
	engine := newMockStorageEngine()
	backend := NewBackend(engine, nil)

	// Add an entry first
	entry := storage.NewEntry("uid=alice,ou=users,dc=example,dc=com")
	entry.SetStringAttribute("objectclass", "person")
	engine.entries["uid=alice,ou=users,dc=example,dc=com"] = entry

	err := backend.Delete("uid=alice,ou=users,dc=example,dc=com")
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Verify entry was deleted
	if _, ok := engine.entries["uid=alice,ou=users,dc=example,dc=com"]; ok {
		t.Error("expected entry to be deleted from storage")
	}
}

// TestDeleteNonExistent tests deleting a non-existent entry.
func TestDeleteNonExistent(t *testing.T) {
	engine := newMockStorageEngine()
	backend := NewBackend(engine, nil)

	err := backend.Delete("uid=nonexistent,dc=example,dc=com")
	if err != ErrEntryNotFound {
		t.Errorf("expected ErrEntryNotFound, got %v", err)
	}
}

// TestDeleteInvalidDN tests deleting with invalid DN.
func TestDeleteInvalidDN(t *testing.T) {
	engine := newMockStorageEngine()
	backend := NewBackend(engine, nil)

	err := backend.Delete("")
	if err != ErrInvalidDN {
		t.Errorf("expected ErrInvalidDN, got %v", err)
	}
}

// TestHasChildren tests checking if an entry has children.
func TestHasChildren(t *testing.T) {
	engine := newMockStorageEngine()
	backend := NewBackend(engine, nil)

	// Add parent entry
	parent := storage.NewEntry("ou=users,dc=example,dc=com")
	parent.SetStringAttribute("objectclass", "organizationalUnit")
	engine.entries["ou=users,dc=example,dc=com"] = parent

	// Add child entry
	child := storage.NewEntry("uid=alice,ou=users,dc=example,dc=com")
	child.SetStringAttribute("objectclass", "person")
	engine.entries["uid=alice,ou=users,dc=example,dc=com"] = child

	// Parent should have children
	hasChildren, err := backend.HasChildren("ou=users,dc=example,dc=com")
	if err != nil {
		t.Fatalf("HasChildren() error = %v", err)
	}
	if !hasChildren {
		t.Error("expected parent to have children")
	}

	// Child should not have children
	hasChildren, err = backend.HasChildren("uid=alice,ou=users,dc=example,dc=com")
	if err != nil {
		t.Fatalf("HasChildren() error = %v", err)
	}
	if hasChildren {
		t.Error("expected child to not have children")
	}
}

// TestHasChildrenInvalidDN tests HasChildren with invalid DN.
func TestHasChildrenInvalidDN(t *testing.T) {
	engine := newMockStorageEngine()
	backend := NewBackend(engine, nil)

	_, err := backend.HasChildren("")
	if err != ErrInvalidDN {
		t.Errorf("expected ErrInvalidDN, got %v", err)
	}
}

// TestDeleteEntry tests the DeleteEntry method with children check.
func TestDeleteEntry(t *testing.T) {
	engine := newMockStorageEngine()
	backend := NewBackend(engine, nil)

	// Add a leaf entry
	entry := storage.NewEntry("uid=alice,ou=users,dc=example,dc=com")
	entry.SetStringAttribute("objectclass", "person")
	engine.entries["uid=alice,ou=users,dc=example,dc=com"] = entry

	err := backend.DeleteEntry("uid=alice,ou=users,dc=example,dc=com")
	if err != nil {
		t.Fatalf("DeleteEntry() error = %v", err)
	}

	// Verify entry was deleted
	if _, ok := engine.entries["uid=alice,ou=users,dc=example,dc=com"]; ok {
		t.Error("expected entry to be deleted from storage")
	}
}

// TestDeleteEntryWithChildren tests that DeleteEntry fails for non-leaf entries.
func TestDeleteEntryWithChildren(t *testing.T) {
	engine := newMockStorageEngine()
	backend := NewBackend(engine, nil)

	// Add parent entry
	parent := storage.NewEntry("ou=users,dc=example,dc=com")
	parent.SetStringAttribute("objectclass", "organizationalUnit")
	engine.entries["ou=users,dc=example,dc=com"] = parent

	// Add child entry
	child := storage.NewEntry("uid=alice,ou=users,dc=example,dc=com")
	child.SetStringAttribute("objectclass", "person")
	engine.entries["uid=alice,ou=users,dc=example,dc=com"] = child

	// Try to delete parent - should fail
	err := backend.DeleteEntry("ou=users,dc=example,dc=com")
	if err != ErrNotAllowedOnNonLeaf {
		t.Errorf("expected ErrNotAllowedOnNonLeaf, got %v", err)
	}

	// Verify parent was not deleted
	if _, ok := engine.entries["ou=users,dc=example,dc=com"]; !ok {
		t.Error("expected parent entry to still exist")
	}
}

// TestDeleteEntryNonExistent tests DeleteEntry with non-existent entry.
func TestDeleteEntryNonExistent(t *testing.T) {
	engine := newMockStorageEngine()
	backend := NewBackend(engine, nil)

	err := backend.DeleteEntry("uid=nonexistent,dc=example,dc=com")
	if err != ErrEntryNotFound {
		t.Errorf("expected ErrEntryNotFound, got %v", err)
	}
}

// TestDeleteEntryInvalidDN tests DeleteEntry with invalid DN.
func TestDeleteEntryInvalidDN(t *testing.T) {
	engine := newMockStorageEngine()
	backend := NewBackend(engine, nil)

	err := backend.DeleteEntry("")
	if err != ErrInvalidDN {
		t.Errorf("expected ErrInvalidDN, got %v", err)
	}
}

// TestModify tests modifying entries.
func TestModify(t *testing.T) {
	engine := newMockStorageEngine()
	backend := NewBackend(engine, nil)

	// Add an entry first
	entry := storage.NewEntry("uid=alice,ou=users,dc=example,dc=com")
	entry.SetStringAttribute("objectclass", "person")
	entry.SetStringAttribute("cn", "Alice")
	entry.SetStringAttribute("mail", "alice@example.com")
	engine.entries["uid=alice,ou=users,dc=example,dc=com"] = entry

	// Modify the entry
	changes := []Modification{
		{Type: ModReplace, Attribute: "cn", Values: []string{"Alice Smith"}},
		{Type: ModAdd, Attribute: "telephonenumber", Values: []string{"555-1234"}},
		{Type: ModDelete, Attribute: "mail", Values: nil},
	}

	err := backend.Modify("uid=alice,ou=users,dc=example,dc=com", changes)
	if err != nil {
		t.Fatalf("Modify() error = %v", err)
	}

	// Verify modifications
	modified := engine.entries["uid=alice,ou=users,dc=example,dc=com"]
	if modified == nil {
		t.Fatal("expected entry to exist")
	}

	// Check cn was replaced
	cn := modified.GetAttribute("cn")
	if len(cn) != 1 || string(cn[0]) != "Alice Smith" {
		t.Errorf("expected cn to be 'Alice Smith', got %v", cn)
	}

	// Check telephonenumber was added
	phone := modified.GetAttribute("telephonenumber")
	if len(phone) != 1 || string(phone[0]) != "555-1234" {
		t.Errorf("expected telephonenumber to be '555-1234', got %v", phone)
	}

	// Check mail was deleted
	mail := modified.GetAttribute("mail")
	if len(mail) != 0 {
		t.Errorf("expected mail to be deleted, got %v", mail)
	}
}

// TestModifyNonExistent tests modifying a non-existent entry.
func TestModifyNonExistent(t *testing.T) {
	engine := newMockStorageEngine()
	backend := NewBackend(engine, nil)

	changes := []Modification{
		{Type: ModReplace, Attribute: "cn", Values: []string{"Test"}},
	}

	err := backend.Modify("uid=nonexistent,dc=example,dc=com", changes)
	if err != ErrEntryNotFound {
		t.Errorf("expected ErrEntryNotFound, got %v", err)
	}
}

// TestModifyEmptyChanges tests modifying with empty changes.
func TestModifyEmptyChanges(t *testing.T) {
	engine := newMockStorageEngine()
	backend := NewBackend(engine, nil)

	err := backend.Modify("uid=alice,dc=example,dc=com", nil)
	if err != nil {
		t.Errorf("expected no error for empty changes, got %v", err)
	}

	err = backend.Modify("uid=alice,dc=example,dc=com", []Modification{})
	if err != nil {
		t.Errorf("expected no error for empty changes, got %v", err)
	}
}

// TestModifyInvalidDN tests modifying with invalid DN.
func TestModifyInvalidDN(t *testing.T) {
	engine := newMockStorageEngine()
	backend := NewBackend(engine, nil)

	changes := []Modification{
		{Type: ModReplace, Attribute: "cn", Values: []string{"Test"}},
	}

	err := backend.Modify("", changes)
	if err != ErrInvalidDN {
		t.Errorf("expected ErrInvalidDN, got %v", err)
	}
}

// TestSearch tests searching entries.
func TestSearch(t *testing.T) {
	engine := newMockStorageEngine()
	backend := NewBackend(engine, nil)

	// Add some entries
	entry1 := storage.NewEntry("uid=alice,ou=users,dc=example,dc=com")
	entry1.SetStringAttribute("objectclass", "person")
	entry1.SetStringAttribute("uid", "alice")
	entry1.SetStringAttribute("cn", "Alice Smith")
	engine.entries["uid=alice,ou=users,dc=example,dc=com"] = entry1

	entry2 := storage.NewEntry("uid=bob,ou=users,dc=example,dc=com")
	entry2.SetStringAttribute("objectclass", "person")
	entry2.SetStringAttribute("uid", "bob")
	entry2.SetStringAttribute("cn", "Bob Jones")
	engine.entries["uid=bob,ou=users,dc=example,dc=com"] = entry2

	// Search without filter
	results, err := backend.Search("dc=example,dc=com", int(storage.ScopeSubtree), nil)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

// TestSearchWithFilter tests searching with a filter.
func TestSearchWithFilter(t *testing.T) {
	engine := newMockStorageEngine()
	backend := NewBackend(engine, nil)

	// Add some entries
	entry1 := storage.NewEntry("uid=alice,ou=users,dc=example,dc=com")
	entry1.SetStringAttribute("objectclass", "person")
	entry1.SetStringAttribute("uid", "alice")
	entry1.SetStringAttribute("cn", "Alice Smith")
	engine.entries["uid=alice,ou=users,dc=example,dc=com"] = entry1

	entry2 := storage.NewEntry("uid=bob,ou=users,dc=example,dc=com")
	entry2.SetStringAttribute("objectclass", "person")
	entry2.SetStringAttribute("uid", "bob")
	entry2.SetStringAttribute("cn", "Bob Jones")
	engine.entries["uid=bob,ou=users,dc=example,dc=com"] = entry2

	// Search with equality filter
	f := filter.NewEqualityFilter("uid", []byte("alice"))
	results, err := backend.Search("dc=example,dc=com", int(storage.ScopeSubtree), f)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}

	if len(results) > 0 && results[0].GetFirstAttribute("uid") != "alice" {
		t.Errorf("expected uid to be 'alice', got '%s'", results[0].GetFirstAttribute("uid"))
	}
}

// TestSearchBaseScope tests searching with base scope.
func TestSearchBaseScope(t *testing.T) {
	engine := newMockStorageEngine()
	backend := NewBackend(engine, nil)

	// Add an entry
	entry := storage.NewEntry("uid=alice,ou=users,dc=example,dc=com")
	entry.SetStringAttribute("objectclass", "person")
	entry.SetStringAttribute("uid", "alice")
	engine.entries["uid=alice,ou=users,dc=example,dc=com"] = entry

	// Search with base scope
	results, err := backend.Search("uid=alice,ou=users,dc=example,dc=com", int(storage.ScopeBase), nil)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

// Entry tests

// TestNewEntry tests creating a new entry.
func TestNewEntry(t *testing.T) {
	entry := NewEntry("uid=alice,dc=example,dc=com")

	if entry == nil {
		t.Fatal("expected entry to be created")
	}

	if entry.DN != "uid=alice,dc=example,dc=com" {
		t.Errorf("expected DN to be 'uid=alice,dc=example,dc=com', got '%s'", entry.DN)
	}

	if entry.Attributes == nil {
		t.Error("expected Attributes to be initialized")
	}
}

// TestEntrySetAttribute tests setting attributes.
func TestEntrySetAttribute(t *testing.T) {
	entry := NewEntry("uid=alice,dc=example,dc=com")

	entry.SetAttribute("cn", "Alice Smith")
	entry.SetAttribute("mail", "alice@example.com", "alice.smith@example.com")

	cn := entry.GetAttribute("cn")
	if len(cn) != 1 || cn[0] != "Alice Smith" {
		t.Errorf("expected cn to be ['Alice Smith'], got %v", cn)
	}

	mail := entry.GetAttribute("mail")
	if len(mail) != 2 {
		t.Errorf("expected mail to have 2 values, got %d", len(mail))
	}
}

// TestEntryGetAttribute tests getting attributes.
func TestEntryGetAttribute(t *testing.T) {
	entry := NewEntry("uid=alice,dc=example,dc=com")
	entry.SetAttribute("cn", "Alice Smith")

	// Test case-insensitive lookup
	cn := entry.GetAttribute("CN")
	if len(cn) != 1 || cn[0] != "Alice Smith" {
		t.Errorf("expected cn to be ['Alice Smith'], got %v", cn)
	}

	// Test non-existent attribute
	nonExistent := entry.GetAttribute("nonexistent")
	if nonExistent != nil {
		t.Errorf("expected nil for non-existent attribute, got %v", nonExistent)
	}
}

// TestEntryGetFirstAttribute tests getting the first attribute value.
func TestEntryGetFirstAttribute(t *testing.T) {
	entry := NewEntry("uid=alice,dc=example,dc=com")
	entry.SetAttribute("cn", "Alice Smith", "Alice")

	first := entry.GetFirstAttribute("cn")
	if first != "Alice Smith" {
		t.Errorf("expected first value to be 'Alice Smith', got '%s'", first)
	}

	// Test non-existent attribute
	nonExistent := entry.GetFirstAttribute("nonexistent")
	if nonExistent != "" {
		t.Errorf("expected empty string for non-existent attribute, got '%s'", nonExistent)
	}
}

// TestEntryHasAttribute tests checking for attribute existence.
func TestEntryHasAttribute(t *testing.T) {
	entry := NewEntry("uid=alice,dc=example,dc=com")
	entry.SetAttribute("cn", "Alice Smith")

	if !entry.HasAttribute("cn") {
		t.Error("expected HasAttribute('cn') to return true")
	}

	if !entry.HasAttribute("CN") {
		t.Error("expected HasAttribute('CN') to return true (case-insensitive)")
	}

	if entry.HasAttribute("nonexistent") {
		t.Error("expected HasAttribute('nonexistent') to return false")
	}
}

// TestEntryAddAttributeValue tests adding attribute values.
func TestEntryAddAttributeValue(t *testing.T) {
	entry := NewEntry("uid=alice,dc=example,dc=com")

	entry.AddAttributeValue("mail", "alice@example.com")
	entry.AddAttributeValue("mail", "alice.smith@example.com")

	mail := entry.GetAttribute("mail")
	if len(mail) != 2 {
		t.Errorf("expected mail to have 2 values, got %d", len(mail))
	}
}

// TestEntryDeleteAttribute tests deleting attributes.
func TestEntryDeleteAttribute(t *testing.T) {
	entry := NewEntry("uid=alice,dc=example,dc=com")
	entry.SetAttribute("cn", "Alice Smith")
	entry.SetAttribute("mail", "alice@example.com")

	entry.DeleteAttribute("mail")

	if entry.HasAttribute("mail") {
		t.Error("expected mail attribute to be deleted")
	}

	if !entry.HasAttribute("cn") {
		t.Error("expected cn attribute to still exist")
	}
}

// TestEntryDeleteAttributeValue tests deleting specific attribute values.
func TestEntryDeleteAttributeValue(t *testing.T) {
	entry := NewEntry("uid=alice,dc=example,dc=com")
	entry.SetAttribute("mail", "alice@example.com", "alice.smith@example.com")

	entry.DeleteAttributeValue("mail", "alice@example.com")

	mail := entry.GetAttribute("mail")
	if len(mail) != 1 || mail[0] != "alice.smith@example.com" {
		t.Errorf("expected mail to be ['alice.smith@example.com'], got %v", mail)
	}

	// Delete last value - attribute should be removed
	entry.DeleteAttributeValue("mail", "alice.smith@example.com")
	if entry.HasAttribute("mail") {
		t.Error("expected mail attribute to be deleted when last value is removed")
	}
}

// TestEntryClone tests cloning an entry.
func TestEntryClone(t *testing.T) {
	entry := NewEntry("uid=alice,dc=example,dc=com")
	entry.SetAttribute("cn", "Alice Smith")
	entry.SetAttribute("mail", "alice@example.com")

	clone := entry.Clone()

	if clone == nil {
		t.Fatal("expected clone to be created")
	}

	if clone.DN != entry.DN {
		t.Errorf("expected clone DN to be '%s', got '%s'", entry.DN, clone.DN)
	}

	// Modify clone and verify original is unchanged
	clone.SetAttribute("cn", "Modified")
	if entry.GetFirstAttribute("cn") != "Alice Smith" {
		t.Error("expected original entry to be unchanged after modifying clone")
	}
}

// TestEntryCloneNil tests cloning a nil entry.
func TestEntryCloneNil(t *testing.T) {
	var entry *Entry
	clone := entry.Clone()

	if clone != nil {
		t.Error("expected clone of nil entry to be nil")
	}
}

// TestEntryAttributeNames tests getting attribute names.
func TestEntryAttributeNames(t *testing.T) {
	entry := NewEntry("uid=alice,dc=example,dc=com")
	entry.SetAttribute("cn", "Alice Smith")
	entry.SetAttribute("mail", "alice@example.com")
	entry.SetAttribute("uid", "alice")

	names := entry.AttributeNames()
	if len(names) != 3 {
		t.Errorf("expected 3 attribute names, got %d", len(names))
	}
}

// TestModificationType tests modification type string representation.
func TestModificationType(t *testing.T) {
	tests := []struct {
		modType  ModificationType
		expected string
	}{
		{ModAdd, "add"},
		{ModDelete, "delete"},
		{ModReplace, "replace"},
		{ModificationType(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if tt.modType.String() != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, tt.modType.String())
			}
		})
	}
}

// TestNewModification tests creating a new modification.
func TestNewModification(t *testing.T) {
	mod := NewModification(ModAdd, "mail", "alice@example.com", "alice.smith@example.com")

	if mod == nil {
		t.Fatal("expected modification to be created")
	}

	if mod.Type != ModAdd {
		t.Errorf("expected type to be ModAdd, got %v", mod.Type)
	}

	if mod.Attribute != "mail" {
		t.Errorf("expected attribute to be 'mail', got '%s'", mod.Attribute)
	}

	if len(mod.Values) != 2 {
		t.Errorf("expected 2 values, got %d", len(mod.Values))
	}
}

// TestMultiValuedAttributes tests that entries support multi-valued attributes.
func TestMultiValuedAttributes(t *testing.T) {
	entry := NewEntry("uid=alice,dc=example,dc=com")

	// Set multiple values
	entry.SetAttribute("objectclass", "top", "person", "inetOrgPerson")
	entry.SetAttribute("mail", "alice@example.com", "alice.smith@example.com", "a.smith@example.com")

	objectClass := entry.GetAttribute("objectclass")
	if len(objectClass) != 3 {
		t.Errorf("expected objectClass to have 3 values, got %d", len(objectClass))
	}

	mail := entry.GetAttribute("mail")
	if len(mail) != 3 {
		t.Errorf("expected mail to have 3 values, got %d", len(mail))
	}

	// Verify all values are present
	expectedOC := []string{"top", "person", "inetOrgPerson"}
	for i, expected := range expectedOC {
		if objectClass[i] != expected {
			t.Errorf("expected objectClass[%d] to be '%s', got '%s'", i, expected, objectClass[i])
		}
	}
}
