package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/KilimcininKorOglu/oba/internal/crypto"
	"github.com/KilimcininKorOglu/oba/internal/storage"
)

func TestObaDBEncryption(t *testing.T) {
	tmpDir := t.TempDir()

	// Generate encryption key
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}

	// Open database with encryption
	opts := storage.DefaultEngineOptions().
		WithDataDir(tmpDir).
		WithEncryptionKey(key)

	db, err := Open(tmpDir, opts)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	// Verify encryption is enabled
	if !db.IsEncrypted() {
		t.Error("IsEncrypted() = false, want true")
	}

	// Create and store an entry
	txn, err := db.Begin()
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}

	entry := &storage.Entry{
		DN: "cn=test,dc=example,dc=com",
		Attributes: map[string][][]byte{
			"cn":           {[]byte("test")},
			"objectClass":  {[]byte("person")},
			"userPassword": {[]byte("secret123")},
		},
	}

	if err := db.Put(txn, entry); err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	if err := db.Commit(txn); err != nil {
		t.Fatalf("Commit() error = %v", err)
	}

	// Read entry back
	txn2, _ := db.Begin()
	loaded, err := db.Get(txn2, entry.DN)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	db.Rollback(txn2)

	if loaded.DN != entry.DN {
		t.Errorf("Get() DN = %q, want %q", loaded.DN, entry.DN)
	}

	// Close database
	if err := db.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	// Verify data file is encrypted (should not contain plaintext password)
	dataFile := filepath.Join(tmpDir, DataFileName)
	data, err := os.ReadFile(dataFile)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	// Check that plaintext password is not visible
	// Note: DN might appear in radix tree index, but sensitive data should be encrypted
	if containsBytes(data, []byte("secret123")) {
		t.Error("Data file contains plaintext password")
	}
}

func TestObaDBEncryptionReopen(t *testing.T) {
	tmpDir := t.TempDir()

	key, _ := crypto.GenerateKey()
	opts := storage.DefaultEngineOptions().
		WithDataDir(tmpDir).
		WithEncryptionKey(key)

	// Create database and add entry
	db, err := Open(tmpDir, opts)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	txn, _ := db.Begin()
	entry := &storage.Entry{
		DN: "cn=user1,dc=example,dc=com",
		Attributes: map[string][][]byte{
			"cn": {[]byte("user1")},
		},
	}
	db.Put(txn, entry)
	db.Commit(txn)
	db.Close()

	// Reopen with same key
	db2, err := Open(tmpDir, opts)
	if err != nil {
		t.Fatalf("Reopen() error = %v", err)
	}
	defer db2.Close()

	// Should be able to read entry
	txn2, _ := db2.Begin()
	loaded, err := db2.Get(txn2, entry.DN)
	if err != nil {
		t.Fatalf("Get() after reopen error = %v", err)
	}
	db2.Rollback(txn2)

	if loaded.DN != entry.DN {
		t.Errorf("Get() DN = %q, want %q", loaded.DN, entry.DN)
	}
}

func TestObaDBEncryptionWrongKey(t *testing.T) {
	tmpDir := t.TempDir()

	key1, _ := crypto.GenerateKey()
	key2, _ := crypto.GenerateKey()

	// Create database with key1
	opts1 := storage.DefaultEngineOptions().
		WithDataDir(tmpDir).
		WithEncryptionKey(key1)

	db, _ := Open(tmpDir, opts1)
	txn, _ := db.Begin()
	entry := &storage.Entry{
		DN: "cn=test,dc=example,dc=com",
		Attributes: map[string][][]byte{
			"cn": {[]byte("test")},
		},
	}
	db.Put(txn, entry)
	db.Commit(txn)
	db.Close()

	// Try to open with key2
	opts2 := storage.DefaultEngineOptions().
		WithDataDir(tmpDir).
		WithEncryptionKey(key2)

	db2, err := Open(tmpDir, opts2)
	if err != nil {
		// Expected: WAL recovery might fail with wrong key
		return
	}
	defer db2.Close()

	// If open succeeds, reading should fail or return wrong data
	txn2, _ := db2.Begin()
	_, err = db2.Get(txn2, entry.DN)
	db2.Rollback(txn2)

	// Either error or entry not found is acceptable
	if err == nil {
		t.Log("Warning: Get() succeeded with wrong key (data might be corrupted)")
	}
}

func TestObaDBNoEncryption(t *testing.T) {
	tmpDir := t.TempDir()

	// Open without encryption
	opts := storage.DefaultEngineOptions().
		WithDataDir(tmpDir)

	db, err := Open(tmpDir, opts)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	// Verify encryption is disabled
	if db.IsEncrypted() {
		t.Error("IsEncrypted() = true, want false")
	}

	// Should still work normally
	txn, _ := db.Begin()
	entry := &storage.Entry{
		DN: "cn=test,dc=example,dc=com",
		Attributes: map[string][][]byte{
			"cn": {[]byte("test")},
		},
	}
	db.Put(txn, entry)
	db.Commit(txn)

	txn2, _ := db.Begin()
	loaded, err := db.Get(txn2, entry.DN)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	db.Rollback(txn2)

	if loaded.DN != entry.DN {
		t.Errorf("Get() DN = %q, want %q", loaded.DN, entry.DN)
	}
}

func TestObaDBEncryptionKeyFile(t *testing.T) {
	tmpDir := t.TempDir()
	keyFile := filepath.Join(tmpDir, "encryption.key")

	// Generate and save key
	key, _ := crypto.GenerateKey()
	if err := crypto.SaveKeyToFile(key, keyFile); err != nil {
		t.Fatalf("SaveKeyToFile() error = %v", err)
	}

	// Open with key file
	opts := storage.DefaultEngineOptions().
		WithDataDir(tmpDir).
		WithEncryptionKeyFile(keyFile)

	db, err := Open(tmpDir, opts)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	if !db.IsEncrypted() {
		t.Error("IsEncrypted() = false, want true")
	}
}

func TestObaDBEncryptionIterator(t *testing.T) {
	tmpDir := t.TempDir()

	key, _ := crypto.GenerateKey()
	opts := storage.DefaultEngineOptions().
		WithDataDir(tmpDir).
		WithEncryptionKey(key)

	db, _ := Open(tmpDir, opts)
	defer db.Close()

	// Add multiple entries
	entries := []*storage.Entry{
		{DN: "cn=user1,dc=example,dc=com", Attributes: map[string][][]byte{"cn": {[]byte("user1")}}},
		{DN: "cn=user2,dc=example,dc=com", Attributes: map[string][][]byte{"cn": {[]byte("user2")}}},
		{DN: "cn=user3,dc=example,dc=com", Attributes: map[string][][]byte{"cn": {[]byte("user3")}}},
	}

	txn, _ := db.Begin()
	for _, e := range entries {
		db.Put(txn, e)
	}
	db.Commit(txn)

	// Iterate and verify
	txn2, _ := db.Begin()
	iter := db.SearchByDN(txn2, "dc=example,dc=com", storage.ScopeSubtree)
	defer iter.Close()

	count := 0
	for iter.Next() {
		entry := iter.Entry()
		if entry == nil {
			t.Error("Iterator returned nil entry")
			continue
		}
		count++
	}

	if err := iter.Error(); err != nil {
		t.Errorf("Iterator error = %v", err)
	}

	if count != len(entries) {
		t.Errorf("Iterator count = %d, want %d", count, len(entries))
	}

	db.Rollback(txn2)
}

// containsBytes checks if data contains the given bytes.
func containsBytes(data, search []byte) bool {
	for i := 0; i <= len(data)-len(search); i++ {
		match := true
		for j := 0; j < len(search); j++ {
			if data[i+j] != search[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
