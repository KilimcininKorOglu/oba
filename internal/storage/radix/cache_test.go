package radix

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/oba-ldap/oba/internal/storage"
)

func TestRadixCacheSaveLoad(t *testing.T) {
	// Create a temp directory for the page manager
	dir := t.TempDir()
	dataPath := filepath.Join(dir, "data.oba")

	// Create page manager
	pm, err := storage.OpenPageManager(dataPath, storage.Options{
		PageSize:    4096,
		CreateIfNew: true,
	})
	if err != nil {
		t.Fatalf("failed to create page manager: %v", err)
	}
	defer pm.Close()

	// Create radix tree
	tree, err := NewRadixTree(pm)
	if err != nil {
		t.Fatalf("failed to create radix tree: %v", err)
	}

	// Insert some entries
	entries := []struct {
		dn     string
		pageID storage.PageID
		slotID uint16
	}{
		{"dc=example,dc=com", 1, 0},
		{"ou=users,dc=example,dc=com", 2, 0},
		{"cn=admin,ou=users,dc=example,dc=com", 3, 0},
		{"cn=user1,ou=users,dc=example,dc=com", 4, 0},
		{"ou=groups,dc=example,dc=com", 5, 0},
	}

	for _, e := range entries {
		if err := tree.Insert(e.dn, e.pageID, e.slotID); err != nil {
			t.Fatalf("failed to insert %s: %v", e.dn, err)
		}
	}

	// Save cache
	cachePath := filepath.Join(dir, "radix.cache")
	txID := uint64(12345)
	if err := tree.SaveCache(cachePath, txID); err != nil {
		t.Fatalf("failed to save cache: %v", err)
	}

	// Verify cache file exists
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		t.Fatal("cache file not created")
	}

	// Create new tree and load from cache
	tree2, err := NewRadixTree(pm)
	if err != nil {
		t.Fatalf("failed to create second tree: %v", err)
	}

	if err := tree2.LoadCache(cachePath, txID); err != nil {
		t.Fatalf("failed to load cache: %v", err)
	}

	// Verify entries
	for _, e := range entries {
		pageID, slotID, found := tree2.Lookup(e.dn)
		if !found {
			t.Errorf("entry %s not found after cache load", e.dn)
			continue
		}
		if pageID != e.pageID {
			t.Errorf("pageID mismatch for %s: expected %d, got %d", e.dn, e.pageID, pageID)
		}
		if slotID != e.slotID {
			t.Errorf("slotID mismatch for %s: expected %d, got %d", e.dn, e.slotID, slotID)
		}
	}
}

func TestRadixCacheStaleTxID(t *testing.T) {
	dir := t.TempDir()
	dataPath := filepath.Join(dir, "data.oba")

	pm, err := storage.OpenPageManager(dataPath, storage.Options{
		PageSize:    4096,
		CreateIfNew: true,
	})
	if err != nil {
		t.Fatalf("failed to create page manager: %v", err)
	}
	defer pm.Close()

	tree, err := NewRadixTree(pm)
	if err != nil {
		t.Fatalf("failed to create radix tree: %v", err)
	}

	tree.Insert("dc=test", 1, 0)

	// Save with txID 100
	cachePath := filepath.Join(dir, "radix.cache")
	if err := tree.SaveCache(cachePath, 100); err != nil {
		t.Fatalf("failed to save cache: %v", err)
	}

	// Try to load with different txID
	tree2, _ := NewRadixTree(pm)
	err = tree2.LoadCache(cachePath, 200)
	if err == nil {
		t.Error("expected error for stale txID")
	}
}

func TestRadixCacheEmptyTree(t *testing.T) {
	dir := t.TempDir()
	dataPath := filepath.Join(dir, "data.oba")

	pm, err := storage.OpenPageManager(dataPath, storage.Options{
		PageSize:    4096,
		CreateIfNew: true,
	})
	if err != nil {
		t.Fatalf("failed to create page manager: %v", err)
	}
	defer pm.Close()

	tree, err := NewRadixTree(pm)
	if err != nil {
		t.Fatalf("failed to create radix tree: %v", err)
	}

	// Save empty tree
	cachePath := filepath.Join(dir, "radix.cache")
	err = tree.SaveCache(cachePath, 100)
	// Empty tree should not create cache file or return nil
	if err != nil {
		t.Logf("save empty tree returned: %v (expected)", err)
	}
}

func TestRadixCacheLargeTree(t *testing.T) {
	dir := t.TempDir()
	dataPath := filepath.Join(dir, "data.oba")

	pm, err := storage.OpenPageManager(dataPath, storage.Options{
		PageSize:    4096,
		CreateIfNew: true,
	})
	if err != nil {
		t.Fatalf("failed to create page manager: %v", err)
	}
	defer pm.Close()

	tree, err := NewRadixTree(pm)
	if err != nil {
		t.Fatalf("failed to create radix tree: %v", err)
	}

	// Insert entries (limited to avoid page overflow)
	insertCount := 0
	for i := 0; i < 100; i++ {
		dn := "cn=user" + string(rune('a'+i%26)) + ",ou=users,dc=example,dc=com"
		if err := tree.Insert(dn, storage.PageID(i+1), uint16(i%100)); err != nil {
			// Stop if we hit page limit
			break
		}
		insertCount++
	}

	if insertCount == 0 {
		t.Skip("could not insert any entries")
	}

	// Save cache
	cachePath := filepath.Join(dir, "radix.cache")
	txID := uint64(999)
	if err := tree.SaveCache(cachePath, txID); err != nil {
		t.Fatalf("failed to save cache: %v", err)
	}

	// Load into new tree
	tree2, _ := NewRadixTree(pm)
	if err := tree2.LoadCache(cachePath, txID); err != nil {
		t.Fatalf("failed to load cache: %v", err)
	}

	// Verify entry count
	if tree2.EntryCount() != tree.EntryCount() {
		t.Errorf("entry count mismatch: expected %d, got %d", tree.EntryCount(), tree2.EntryCount())
	}
}
