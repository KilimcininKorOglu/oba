package raft

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/KilimcininKorOglu/oba/internal/storage"
)

func TestSerializeDeserializeEntry(t *testing.T) {
	entry := storage.NewEntry("cn=test,dc=example,dc=com")
	entry.SetStringAttribute("cn", "test")
	entry.SetStringAttribute("objectclass", "person", "top")
	entry.SetAttribute("mail", [][]byte{[]byte("test@example.com")})

	// Serialize
	data := serializeEntry(entry)

	// Deserialize
	restored, err := deserializeEntry(data)
	if err != nil {
		t.Fatalf("deserializeEntry failed: %v", err)
	}

	// Verify DN
	if restored.DN != entry.DN {
		t.Errorf("DN mismatch: got %s, want %s", restored.DN, entry.DN)
	}

	// Verify attributes
	if len(restored.Attributes) != len(entry.Attributes) {
		t.Errorf("Attribute count mismatch: got %d, want %d", len(restored.Attributes), len(entry.Attributes))
	}

	// Check cn
	cn := restored.GetAttribute("cn")
	if len(cn) != 1 || string(cn[0]) != "test" {
		t.Errorf("cn attribute mismatch")
	}

	// Check objectclass
	oc := restored.GetAttribute("objectclass")
	if len(oc) != 2 {
		t.Errorf("objectclass should have 2 values")
	}

	// Check mail
	mail := restored.GetAttribute("mail")
	if len(mail) != 1 || string(mail[0]) != "test@example.com" {
		t.Errorf("mail attribute mismatch")
	}
}

func TestSerializeDeserializeEmptyEntry(t *testing.T) {
	entry := storage.NewEntry("dc=example,dc=com")

	data := serializeEntry(entry)
	restored, err := deserializeEntry(data)
	if err != nil {
		t.Fatalf("deserializeEntry failed: %v", err)
	}

	if restored.DN != entry.DN {
		t.Errorf("DN mismatch")
	}
	if len(restored.Attributes) != 0 {
		t.Errorf("Should have no attributes")
	}
}

func TestDeserializeEntryCorrupted(t *testing.T) {
	// Empty data
	_, err := deserializeEntry(nil)
	if err != ErrLogCorrupted {
		t.Errorf("Expected ErrLogCorrupted for nil data")
	}

	_, err = deserializeEntry([]byte{})
	if err != ErrLogCorrupted {
		t.Errorf("Expected ErrLogCorrupted for empty data")
	}

	// Truncated data
	_, err = deserializeEntry([]byte{0x05, 0x00}) // DN length but no DN
	if err != ErrLogCorrupted {
		t.Errorf("Expected ErrLogCorrupted for truncated data")
	}
}

func TestCreatePutCommand(t *testing.T) {
	entry := storage.NewEntry("cn=user,dc=example,dc=com")
	entry.SetStringAttribute("cn", "user")

	cmd := CreatePutCommand(entry)

	if cmd.Type != CmdPut {
		t.Errorf("Type should be CmdPut")
	}
	if cmd.DN != entry.DN {
		t.Errorf("DN mismatch")
	}
	if len(cmd.EntryData) == 0 {
		t.Errorf("EntryData should not be empty")
	}

	// Verify entry can be deserialized
	restored, err := deserializeEntry(cmd.EntryData)
	if err != nil {
		t.Fatalf("deserializeEntry failed: %v", err)
	}
	if restored.DN != entry.DN {
		t.Errorf("Restored DN mismatch")
	}
}

func TestCreateDeleteCommand(t *testing.T) {
	cmd := CreateDeleteCommand("cn=todelete,dc=example,dc=com")

	if cmd.Type != CmdDelete {
		t.Errorf("Type should be CmdDelete")
	}
	if cmd.DN != "cn=todelete,dc=example,dc=com" {
		t.Errorf("DN mismatch")
	}
}

func TestCreateModifyDNCommand(t *testing.T) {
	newEntry := storage.NewEntry("cn=newname,dc=example,dc=com")
	newEntry.SetStringAttribute("cn", "newname")

	cmd := CreateModifyDNCommand("cn=oldname,dc=example,dc=com", newEntry)

	if cmd.Type != CmdModifyDN {
		t.Errorf("Type should be CmdModifyDN")
	}
	if cmd.OldDN != "cn=oldname,dc=example,dc=com" {
		t.Errorf("OldDN mismatch")
	}
	if cmd.DN != newEntry.DN {
		t.Errorf("DN mismatch")
	}
}

// MockStorageEngine implements storage.StorageEngine for testing.
type MockStorageEngine struct {
	entries map[string]*storage.Entry
}

func NewMockStorageEngine() *MockStorageEngine {
	return &MockStorageEngine{
		entries: make(map[string]*storage.Entry),
	}
}

func (m *MockStorageEngine) Begin() (interface{}, error) {
	return "tx", nil
}

func (m *MockStorageEngine) Commit(tx interface{}) error {
	return nil
}

func (m *MockStorageEngine) Rollback(tx interface{}) error {
	return nil
}

func (m *MockStorageEngine) Get(tx interface{}, dn string) (*storage.Entry, error) {
	if entry, ok := m.entries[dn]; ok {
		return entry.Clone(), nil
	}
	return nil, nil
}

func (m *MockStorageEngine) Put(tx interface{}, entry *storage.Entry) error {
	m.entries[entry.DN] = entry.Clone()
	return nil
}

func (m *MockStorageEngine) Delete(tx interface{}, dn string) error {
	delete(m.entries, dn)
	return nil
}

func (m *MockStorageEngine) HasChildren(tx interface{}, dn string) (bool, error) {
	return false, nil
}

func (m *MockStorageEngine) SearchByDN(tx interface{}, baseDN string, scope storage.Scope) storage.Iterator {
	return &mockIterator{entries: m.getAllEntries()}
}

func (m *MockStorageEngine) SearchByFilter(tx interface{}, baseDN string, f interface{}) storage.Iterator {
	return &mockIterator{entries: m.getAllEntries()}
}

func (m *MockStorageEngine) getAllEntries() []*storage.Entry {
	entries := make([]*storage.Entry, 0, len(m.entries))
	for _, e := range m.entries {
		entries = append(entries, e.Clone())
	}
	return entries
}

func (m *MockStorageEngine) CreateIndex(attribute string, indexType storage.IndexType) error {
	return nil
}

func (m *MockStorageEngine) DropIndex(attribute string) error {
	return nil
}

func (m *MockStorageEngine) Checkpoint() error {
	return nil
}

func (m *MockStorageEngine) Compact() error {
	return nil
}

func (m *MockStorageEngine) Stats() *storage.EngineStats {
	return &storage.EngineStats{EntryCount: uint64(len(m.entries))}
}

func (m *MockStorageEngine) Close() error {
	return nil
}

type mockIterator struct {
	entries []*storage.Entry
	index   int
	err     error
}

func (i *mockIterator) Next() bool {
	if i.index < len(i.entries) {
		i.index++
		return true
	}
	return false
}

func (i *mockIterator) Entry() *storage.Entry {
	if i.index > 0 && i.index <= len(i.entries) {
		return i.entries[i.index-1]
	}
	return nil
}

func (i *mockIterator) Error() error {
	return i.err
}

func (i *mockIterator) Close() {}

func TestObaDBStateMachineApply(t *testing.T) {
	engine := NewMockStorageEngine()
	sm := NewObaDBStateMachine(engine)

	// Test Put
	entry := storage.NewEntry("cn=test,dc=example,dc=com")
	entry.SetStringAttribute("cn", "test")

	cmd := CreatePutCommand(entry)
	if err := sm.Apply(cmd); err != nil {
		t.Fatalf("Apply Put failed: %v", err)
	}

	// Verify entry was stored
	if _, ok := engine.entries["cn=test,dc=example,dc=com"]; !ok {
		t.Error("Entry should be stored")
	}

	// Test Delete
	deleteCmd := CreateDeleteCommand("cn=test,dc=example,dc=com")
	if err := sm.Apply(deleteCmd); err != nil {
		t.Fatalf("Apply Delete failed: %v", err)
	}

	// Verify entry was deleted
	if _, ok := engine.entries["cn=test,dc=example,dc=com"]; ok {
		t.Error("Entry should be deleted")
	}
}

func TestObaDBStateMachineSnapshot(t *testing.T) {
	engine := NewMockStorageEngine()
	sm := NewObaDBStateMachine(engine)

	// Add some entries
	entry1 := storage.NewEntry("dc=example,dc=com")
	entry1.SetStringAttribute("dc", "example")
	engine.entries[entry1.DN] = entry1

	entry2 := storage.NewEntry("cn=user,dc=example,dc=com")
	entry2.SetStringAttribute("cn", "user")
	engine.entries[entry2.DN] = entry2

	// Create snapshot
	data, err := sm.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}

	if len(data) == 0 {
		t.Error("Snapshot should not be empty")
	}

	// Verify snapshot contains entry count
	reader := bytes.NewReader(data)
	var count uint32
	if err := binary.Read(reader, binary.LittleEndian, &count); err != nil {
		t.Fatalf("Failed to read count: %v", err)
	}
	if count != 2 {
		t.Errorf("Snapshot should contain 2 entries, got %d", count)
	}
}

func TestObaDBStateMachineRestore(t *testing.T) {
	engine1 := NewMockStorageEngine()
	sm1 := NewObaDBStateMachine(engine1)

	// Add entries to first engine
	entry1 := storage.NewEntry("dc=example,dc=com")
	entry1.SetStringAttribute("dc", "example")
	engine1.entries[entry1.DN] = entry1

	entry2 := storage.NewEntry("cn=user,dc=example,dc=com")
	entry2.SetStringAttribute("cn", "user")
	engine1.entries[entry2.DN] = entry2

	// Create snapshot
	data, _ := sm1.Snapshot()

	// Restore to new engine
	engine2 := NewMockStorageEngine()
	sm2 := NewObaDBStateMachine(engine2)

	if err := sm2.Restore(data); err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	// Verify entries were restored
	if len(engine2.entries) != 2 {
		t.Errorf("Should have 2 entries, got %d", len(engine2.entries))
	}
}

func TestObaDBStateMachineRestoreEmpty(t *testing.T) {
	engine := NewMockStorageEngine()
	sm := NewObaDBStateMachine(engine)

	// Restore empty data should succeed
	if err := sm.Restore(nil); err != nil {
		t.Errorf("Restore nil should succeed: %v", err)
	}

	if err := sm.Restore([]byte{}); err != nil {
		t.Errorf("Restore empty should succeed: %v", err)
	}
}
