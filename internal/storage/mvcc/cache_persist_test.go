package mvcc

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/oba-ldap/oba/internal/storage"
)

func TestVersionStoreSaveLoadCache(t *testing.T) {
	// Create a temporary directory for the cache file
	tmpDir, err := os.MkdirTemp("", "mvcc_cache_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cachePath := filepath.Join(tmpDir, "entry.cache")
	txID := uint64(12345)

	// Create a version store and add some entries
	vs := NewVersionStore(nil)

	// Add committed versions
	vs.LoadCommittedVersion("cn=user1,dc=example,dc=com", []byte("data1"), storage.PageID(1), 0)
	vs.LoadCommittedVersion("cn=user2,dc=example,dc=com", []byte("data2"), storage.PageID(2), 0)
	vs.LoadCommittedVersion("cn=user3,dc=example,dc=com", []byte("data3"), storage.PageID(3), 0)

	// Save cache
	if err := vs.SaveCache(cachePath, txID); err != nil {
		t.Fatalf("SaveCache() error = %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		t.Fatal("cache file was not created")
	}

	// Create a new version store and load cache
	vs2 := NewVersionStore(nil)

	if err := vs2.LoadCache(cachePath, txID); err != nil {
		t.Fatalf("LoadCache() error = %v", err)
	}

	// Verify entries were loaded
	if vs2.EntryCount() != 3 {
		t.Errorf("EntryCount() = %d, want 3", vs2.EntryCount())
	}

	// Verify each entry
	testCases := []struct {
		dn   string
		data string
	}{
		{"cn=user1,dc=example,dc=com", "data1"},
		{"cn=user2,dc=example,dc=com", "data2"},
		{"cn=user3,dc=example,dc=com", "data3"},
	}

	for _, tc := range testCases {
		version := vs2.GetLatestVersion(tc.dn)
		if version == nil {
			t.Errorf("entry %s not found", tc.dn)
			continue
		}
		if string(version.GetData()) != tc.data {
			t.Errorf("entry %s data = %q, want %q", tc.dn, string(version.GetData()), tc.data)
		}
	}
}

func TestVersionStoreCacheStaleTxID(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mvcc_cache_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cachePath := filepath.Join(tmpDir, "entry.cache")

	// Create and save cache with txID 100
	vs := NewVersionStore(nil)
	vs.LoadCommittedVersion("cn=test,dc=example,dc=com", []byte("data"), storage.PageID(1), 0)

	if err := vs.SaveCache(cachePath, 100); err != nil {
		t.Fatalf("SaveCache() error = %v", err)
	}

	// Try to load with different txID - should fail
	vs2 := NewVersionStore(nil)
	err = vs2.LoadCache(cachePath, 200)
	if err == nil {
		t.Error("LoadCache() should fail with stale txID")
	}
}

func TestVersionStoreCacheEmptyStore(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mvcc_cache_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cachePath := filepath.Join(tmpDir, "entry.cache")

	// Create empty version store
	vs := NewVersionStore(nil)

	// Save should succeed (no-op for empty store)
	if err := vs.SaveCache(cachePath, 100); err != nil {
		t.Fatalf("SaveCache() error = %v", err)
	}

	// File should not be created for empty store
	if _, err := os.Stat(cachePath); !os.IsNotExist(err) {
		t.Error("cache file should not be created for empty store")
	}
}

func TestVersionStoreCacheMissingFile(t *testing.T) {
	vs := NewVersionStore(nil)

	err := vs.LoadCache("/nonexistent/path/entry.cache", 100)
	if err == nil {
		t.Error("LoadCache() should fail for missing file")
	}
}

func TestVersionStoreCacheEntryCount(t *testing.T) {
	vs := NewVersionStore(nil)

	// Add committed versions
	vs.LoadCommittedVersion("cn=user1,dc=example,dc=com", []byte("data1"), storage.PageID(1), 0)
	vs.LoadCommittedVersion("cn=user2,dc=example,dc=com", []byte("data2"), storage.PageID(2), 0)

	count := vs.CacheEntryCount()
	if count != 2 {
		t.Errorf("CacheEntryCount() = %d, want 2", count)
	}
}

func TestVersionStoreCacheLargeData(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mvcc_cache_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cachePath := filepath.Join(tmpDir, "entry.cache")
	txID := uint64(999)

	// Create version store with large data
	vs := NewVersionStore(nil)

	// Add entries with varying data sizes
	for i := 0; i < 100; i++ {
		dn := "cn=user" + string(rune('0'+i%10)) + string(rune('0'+i/10)) + ",dc=example,dc=com"
		data := make([]byte, 1000+i*10) // 1KB to 2KB
		for j := range data {
			data[j] = byte(i + j)
		}
		vs.LoadCommittedVersion(dn, data, storage.PageID(uint64(i+1)), 0)
	}

	// Save cache
	if err := vs.SaveCache(cachePath, txID); err != nil {
		t.Fatalf("SaveCache() error = %v", err)
	}

	// Load cache
	vs2 := NewVersionStore(nil)
	if err := vs2.LoadCache(cachePath, txID); err != nil {
		t.Fatalf("LoadCache() error = %v", err)
	}

	// Verify count
	if vs2.EntryCount() != 100 {
		t.Errorf("EntryCount() = %d, want 100", vs2.EntryCount())
	}
}
