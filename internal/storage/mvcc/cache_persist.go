package mvcc

import (
	"bytes"
	"encoding/binary"

	"github.com/oba-ldap/oba/internal/storage"
	"github.com/oba-ldap/oba/internal/storage/cache"
)

// SaveCache persists the version store cache to disk.
// This allows fast startup by avoiding disk I/O for cached entries.
func (vs *VersionStore) SaveCache(path string, txID uint64) error {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	if len(vs.versions) == 0 {
		return nil
	}

	var buf bytes.Buffer

	// Write entry count
	if err := binary.Write(&buf, binary.LittleEndian, uint32(len(vs.versions))); err != nil {
		return err
	}

	// Write each entry
	for dn, version := range vs.versions {
		// Skip uncommitted or deleted versions
		if !version.IsCommitted() || version.IsDeleted() {
			continue
		}

		data := version.GetData()
		if data == nil {
			continue
		}

		pageID, slotID := version.GetLocation()

		// Write DN length + DN
		if err := binary.Write(&buf, binary.LittleEndian, uint16(len(dn))); err != nil {
			return err
		}
		if _, err := buf.WriteString(dn); err != nil {
			return err
		}

		// Write data length + data
		if err := binary.Write(&buf, binary.LittleEndian, uint32(len(data))); err != nil {
			return err
		}
		if _, err := buf.Write(data); err != nil {
			return err
		}

		// Write PageID + SlotID
		if err := binary.Write(&buf, binary.LittleEndian, uint64(pageID)); err != nil {
			return err
		}
		if err := binary.Write(&buf, binary.LittleEndian, slotID); err != nil {
			return err
		}
	}

	return cache.WriteFile(path, cache.TypeEntry, buf.Bytes(), uint64(len(vs.versions)), txID)
}

// LoadCache loads the version store cache from disk.
// Returns nil if cache was loaded successfully, or an error if cache is missing/stale/corrupt.
func (vs *VersionStore) LoadCache(path string, expectedTxID uint64) error {
	data, _, err := cache.ReadFile(path, cache.TypeEntry, expectedTxID)
	if err != nil {
		return err
	}

	buf := bytes.NewReader(data)

	// Read entry count
	var count uint32
	if err := binary.Read(buf, binary.LittleEndian, &count); err != nil {
		return err
	}

	// Read each entry
	for i := uint32(0); i < count; i++ {
		// Read DN length
		var dnLen uint16
		if err := binary.Read(buf, binary.LittleEndian, &dnLen); err != nil {
			if err.Error() == "EOF" {
				break // End of data
			}
			return err
		}

		// Read DN
		dnBytes := make([]byte, dnLen)
		if _, err := buf.Read(dnBytes); err != nil {
			return err
		}
		dn := string(dnBytes)

		// Read data length
		var dataLen uint32
		if err := binary.Read(buf, binary.LittleEndian, &dataLen); err != nil {
			return err
		}

		// Read data
		entryData := make([]byte, dataLen)
		if _, err := buf.Read(entryData); err != nil {
			return err
		}

		// Read PageID + SlotID
		var pageID uint64
		var slotID uint16
		if err := binary.Read(buf, binary.LittleEndian, &pageID); err != nil {
			return err
		}
		if err := binary.Read(buf, binary.LittleEndian, &slotID); err != nil {
			return err
		}

		// Load into version store
		vs.LoadCommittedVersion(dn, entryData, storage.PageID(pageID), slotID)
	}

	return nil
}

// CacheEntryCount returns the number of entries that would be cached.
func (vs *VersionStore) CacheEntryCount() int {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	count := 0
	for _, version := range vs.versions {
		if version.IsCommitted() && !version.IsDeleted() && version.GetData() != nil {
			count++
		}
	}
	return count
}
