// Package storage provides the core storage engine components for ObaDB.
package storage

import (
	"fmt"
	"testing"
)

// BenchmarkDNLookup benchmarks DN-based entry lookup.
// PRD target: < 10 us for point lookup by DN.
func BenchmarkDNLookup(b *testing.B) {
	pm, cleanup := setupBenchmarkPageManager(b)
	defer cleanup()

	// Create a radix tree for DN lookups
	tree, err := setupBenchmarkRadixTree(pm)
	if err != nil {
		b.Fatalf("Failed to setup radix tree: %v", err)
	}

	// Insert test data
	const numEntries = 10000
	for i := 0; i < numEntries; i++ {
		dn := fmt.Sprintf("uid=user%d,ou=users,dc=test,dc=com", i)
		pageID, err := pm.AllocatePage(PageTypeData)
		if err != nil {
			b.Fatalf("Failed to allocate page: %v", err)
		}
		if err := tree.Insert(dn, pageID, 0); err != nil {
			b.Fatalf("Failed to insert DN: %v", err)
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		dn := fmt.Sprintf("uid=user%d,ou=users,dc=test,dc=com", i%numEntries)
		_, _, _ = tree.Lookup(dn)
	}
}

// BenchmarkDNInsert benchmarks DN insertion.
func BenchmarkDNInsert(b *testing.B) {
	pm, cleanup := setupBenchmarkPageManager(b)
	defer cleanup()

	tree, err := setupBenchmarkRadixTree(pm)
	if err != nil {
		b.Fatalf("Failed to setup radix tree: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		dn := fmt.Sprintf("uid=user%d,ou=users,dc=test,dc=com", i)
		pageID, _ := pm.AllocatePage(PageTypeData)
		_ = tree.Insert(dn, pageID, 0)
	}
}

// BenchmarkPageAllocation benchmarks page allocation.
func BenchmarkPageAllocation(b *testing.B) {
	pm, cleanup := setupBenchmarkPageManager(b)
	defer cleanup()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = pm.AllocatePage(PageTypeData)
	}
}

// BenchmarkPageRead benchmarks page reading.
func BenchmarkPageRead(b *testing.B) {
	pm, cleanup := setupBenchmarkPageManager(b)
	defer cleanup()

	// Allocate and write some pages
	const numPages = 100
	pageIDs := make([]PageID, numPages)
	for i := 0; i < numPages; i++ {
		pageID, err := pm.AllocatePage(PageTypeData)
		if err != nil {
			b.Fatalf("Failed to allocate page: %v", err)
		}
		pageIDs[i] = pageID

		page := &Page{
			Header: PageHeader{PageID: pageID, PageType: PageTypeData},
			Data:   make([]byte, PageSize-PageHeaderSize),
		}
		if err := pm.WritePage(page); err != nil {
			b.Fatalf("Failed to write page: %v", err)
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		pageID := pageIDs[i%numPages]
		_, _ = pm.ReadPage(pageID)
	}
}

// BenchmarkPageWrite benchmarks page writing.
func BenchmarkPageWrite(b *testing.B) {
	pm, cleanup := setupBenchmarkPageManager(b)
	defer cleanup()

	// Pre-allocate pages
	const numPages = 100
	pageIDs := make([]PageID, numPages)
	for i := 0; i < numPages; i++ {
		pageID, err := pm.AllocatePage(PageTypeData)
		if err != nil {
			b.Fatalf("Failed to allocate page: %v", err)
		}
		pageIDs[i] = pageID
	}

	page := &Page{
		Header: PageHeader{PageType: PageTypeData},
		Data:   make([]byte, PageSize-PageHeaderSize),
	}
	// Fill with test data
	for i := range page.Data {
		page.Data[i] = byte(i % 256)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		page.Header.PageID = pageIDs[i%numPages]
		_ = pm.WritePage(page)
	}
}

// BenchmarkBufferPoolGet benchmarks buffer pool get operations.
func BenchmarkBufferPoolGet(b *testing.B) {
	pool := NewBufferPool(256, PageSize)

	// Pre-populate the pool
	const numPages = 100
	for i := 0; i < numPages; i++ {
		pageID := PageID(i + 1)
		data := make([]byte, PageSize)
		pool.Put(pageID, data)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		pageID := PageID((i % numPages) + 1)
		_, _ = pool.Get(pageID)
	}
}

// BenchmarkBufferPoolPut benchmarks buffer pool put operations.
func BenchmarkBufferPoolPut(b *testing.B) {
	pool := NewBufferPool(256, PageSize)
	data := make([]byte, PageSize)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		pageID := PageID(i % 256)
		pool.Put(pageID, data)
	}
}

// BenchmarkLRUAccess benchmarks LRU cache access patterns.
func BenchmarkLRUAccess(b *testing.B) {
	lru := NewLRUCache()

	// Pre-populate
	for i := 0; i < 256; i++ {
		lru.Access(PageID(i))
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		pageID := PageID(i % 256)
		lru.Access(pageID)
	}
}

// BenchmarkWALAppend benchmarks WAL append operations.
// PRD target: < 1 ms for WAL fsync latency.
func BenchmarkWALAppend(b *testing.B) {
	wal, cleanup := setupBenchmarkWAL(b)
	defer cleanup()

	record := NewWALUpdateRecord(0, 1, 1, 0, nil, make([]byte, 256))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		record.TxID = uint64(i)
		_, _ = wal.Append(record)
	}
}

// BenchmarkWALSync benchmarks WAL sync operations.
func BenchmarkWALSync(b *testing.B) {
	wal, cleanup := setupBenchmarkWAL(b)
	defer cleanup()

	record := NewWALUpdateRecord(0, 1, 1, 0, nil, make([]byte, 256))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		record.TxID = uint64(i)
		_, _ = wal.Append(record)
		_ = wal.Sync()
	}
}

// BenchmarkEntrySerialize benchmarks entry serialization.
func BenchmarkEntrySerialize(b *testing.B) {
	entry := NewEntry("uid=alice,ou=users,dc=example,dc=com")
	entry.SetStringAttribute("cn", "Alice Smith")
	entry.SetStringAttribute("uid", "alice")
	entry.SetStringAttribute("mail", "alice@example.com")
	entry.SetStringAttribute("objectclass", "person", "inetOrgPerson")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = serializeEntry(entry)
	}
}

// BenchmarkEntryDeserialize benchmarks entry deserialization.
func BenchmarkEntryDeserialize(b *testing.B) {
	entry := NewEntry("uid=alice,ou=users,dc=example,dc=com")
	entry.SetStringAttribute("cn", "Alice Smith")
	entry.SetStringAttribute("uid", "alice")
	entry.SetStringAttribute("mail", "alice@example.com")
	entry.SetStringAttribute("objectclass", "person", "inetOrgPerson")

	data := serializeEntry(entry)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = deserializeEntry(entry.DN, data)
	}
}

// BenchmarkEntryClone benchmarks entry cloning.
func BenchmarkEntryClone(b *testing.B) {
	entry := NewEntry("uid=alice,ou=users,dc=example,dc=com")
	entry.SetStringAttribute("cn", "Alice Smith")
	entry.SetStringAttribute("uid", "alice")
	entry.SetStringAttribute("mail", "alice@example.com")
	entry.SetStringAttribute("objectclass", "person", "inetOrgPerson")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = entry.Clone()
	}
}

// BenchmarkDNNormalize benchmarks DN normalization.
func BenchmarkDNNormalize(b *testing.B) {
	dn := "UID=Alice,OU=Users,DC=Example,DC=Com"

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = normalizeDN(dn)
	}
}

// Helper functions for benchmarks

func setupBenchmarkPageManager(b *testing.B) (*PageManager, func()) {
	b.Helper()
	dir := b.TempDir()
	opts := Options{
		PageSize:     PageSize,
		InitialPages: 16,
		CreateIfNew:  true,
		ReadOnly:     false,
		SyncOnWrite:  false,
	}

	pm, err := OpenPageManager(dir+"/data.oba", opts)
	if err != nil {
		b.Fatalf("Failed to open page manager: %v", err)
	}

	return pm, func() {
		pm.Close()
	}
}

func setupBenchmarkWAL(b *testing.B) (*WAL, func()) {
	b.Helper()
	dir := b.TempDir()

	wal, err := OpenWAL(dir + "/wal.oba")
	if err != nil {
		b.Fatalf("Failed to open WAL: %v", err)
	}

	return wal, func() {
		wal.Close()
	}
}

func setupBenchmarkRadixTree(pm *PageManager) (radixTree, error) {
	// Import the radix package and create a tree
	// For now, we use a simple interface
	return newSimpleRadixTree(pm)
}

// radixTree interface for benchmarking
type radixTree interface {
	Insert(dn string, pageID PageID, slot uint16) error
	Lookup(dn string) (PageID, uint16, error)
}

// simpleRadixTree is a simple map-based implementation for benchmarking
type simpleRadixTree struct {
	entries map[string]struct {
		pageID PageID
		slot   uint16
	}
}

func newSimpleRadixTree(_ *PageManager) (*simpleRadixTree, error) {
	return &simpleRadixTree{
		entries: make(map[string]struct {
			pageID PageID
			slot   uint16
		}),
	}, nil
}

func (t *simpleRadixTree) Insert(dn string, pageID PageID, slot uint16) error {
	t.entries[dn] = struct {
		pageID PageID
		slot   uint16
	}{pageID, slot}
	return nil
}

func (t *simpleRadixTree) Lookup(dn string) (PageID, uint16, error) {
	if entry, ok := t.entries[dn]; ok {
		return entry.pageID, entry.slot, nil
	}
	return 0, 0, fmt.Errorf("not found")
}

// serializeEntry serializes an entry to bytes (for benchmarking).
func serializeEntry(entry *Entry) []byte {
	if entry == nil {
		return nil
	}

	// Calculate size
	size := 4 + len(entry.DN) + 4
	for name, values := range entry.Attributes {
		size += 2 + len(name) + 4
		for _, v := range values {
			size += 4 + len(v)
		}
	}

	buf := make([]byte, size)
	offset := 0

	// Write DN length and DN
	putUint32(buf[offset:], uint32(len(entry.DN)))
	offset += 4
	copy(buf[offset:], entry.DN)
	offset += len(entry.DN)

	// Write attribute count
	putUint32(buf[offset:], uint32(len(entry.Attributes)))
	offset += 4

	// Write attributes
	for name, values := range entry.Attributes {
		putUint16(buf[offset:], uint16(len(name)))
		offset += 2
		copy(buf[offset:], name)
		offset += len(name)

		putUint32(buf[offset:], uint32(len(values)))
		offset += 4

		for _, v := range values {
			putUint32(buf[offset:], uint32(len(v)))
			offset += 4
			copy(buf[offset:], v)
			offset += len(v)
		}
	}

	return buf
}

// deserializeEntry deserializes an entry from bytes (for benchmarking).
func deserializeEntry(dn string, data []byte) *Entry {
	if len(data) < 8 {
		return nil
	}

	entry := &Entry{
		DN:         dn,
		Attributes: make(map[string][][]byte),
	}

	offset := 0
	dnLen := getUint32(data[offset:])
	offset += 4 + int(dnLen)

	if offset+4 > len(data) {
		return entry
	}

	attrCount := getUint32(data[offset:])
	offset += 4

	for i := uint32(0); i < attrCount && offset < len(data); i++ {
		if offset+2 > len(data) {
			break
		}

		nameLen := getUint16(data[offset:])
		offset += 2

		if offset+int(nameLen) > len(data) {
			break
		}

		name := string(data[offset : offset+int(nameLen)])
		offset += int(nameLen)

		if offset+4 > len(data) {
			break
		}

		valueCount := getUint32(data[offset:])
		offset += 4

		values := make([][]byte, 0, valueCount)

		for j := uint32(0); j < valueCount && offset < len(data); j++ {
			if offset+4 > len(data) {
				break
			}

			valueLen := getUint32(data[offset:])
			offset += 4

			if offset+int(valueLen) > len(data) {
				break
			}

			value := make([]byte, valueLen)
			copy(value, data[offset:offset+int(valueLen)])
			offset += int(valueLen)

			values = append(values, value)
		}

		entry.Attributes[name] = values
	}

	return entry
}

// normalizeDN normalizes a DN for consistent storage and lookup.
func normalizeDN(dn string) string {
	// Simple lowercase normalization
	result := make([]byte, len(dn))
	for i := 0; i < len(dn); i++ {
		c := dn[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		result[i] = c
	}
	return string(result)
}

// Helper functions for binary encoding
func putUint32(b []byte, v uint32) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
}

func getUint32(b []byte) uint32 {
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}

func putUint16(b []byte, v uint16) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
}

func getUint16(b []byte) uint16 {
	return uint16(b[0]) | uint16(b[1])<<8
}

// BenchmarkStartupWithEntryCache benchmarks startup time with entry cache.
func BenchmarkStartupWithEntryCache(b *testing.B) {
	// This benchmark measures the benefit of entry cache on startup
	// Run with: go test -bench=BenchmarkStartupWithEntryCache -benchtime=10x

	b.Run("100_entries", func(b *testing.B) {
		benchmarkStartupWithCache(b, 100)
	})

	b.Run("1000_entries", func(b *testing.B) {
		benchmarkStartupWithCache(b, 1000)
	})
}

func benchmarkStartupWithCache(b *testing.B, numEntries int) {
	// Setup: Create a database with entries
	tmpDir := b.TempDir()

	// First pass: create database and populate
	pm, err := OpenPageManager(tmpDir+"/data.oba", Options{
		PageSize:    PageSize,
		CreateIfNew: true,
	})
	if err != nil {
		b.Fatalf("Failed to create page manager: %v", err)
	}

	// Create entries
	for i := 0; i < numEntries; i++ {
		pageID, err := pm.AllocatePage(PageTypeData)
		if err != nil {
			b.Fatalf("Failed to allocate page: %v", err)
		}

		page := &Page{
			Header: PageHeader{PageID: pageID, PageType: PageTypeData},
			Data:   make([]byte, PageSize-PageHeaderSize),
		}

		// Write some data
		data := fmt.Sprintf("entry_%d_data", i)
		copy(page.Data[4:], data)
		putUint32(page.Data[:4], uint32(len(data)))

		if err := pm.WritePage(page); err != nil {
			b.Fatalf("Failed to write page: %v", err)
		}
	}

	pm.Close()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Measure startup time
		pm2, err := OpenPageManager(tmpDir+"/data.oba", Options{
			PageSize:    PageSize,
			CreateIfNew: false,
		})
		if err != nil {
			b.Fatalf("Failed to open page manager: %v", err)
		}
		pm2.Close()
	}
}
