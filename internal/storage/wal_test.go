// Package storage provides the core storage engine components for ObaDB.
package storage

import (
	"os"
	"path/filepath"
	"testing"
)

// TestWALRecordTypes tests WALType string representation.
func TestWALRecordTypes(t *testing.T) {
	tests := []struct {
		walType  WALType
		expected string
	}{
		{WALBegin, "Begin"},
		{WALCommit, "Commit"},
		{WALAbort, "Abort"},
		{WALUpdate, "Update"},
		{WALCheckpoint, "Checkpoint"},
		{WALType(255), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.walType.String(); got != tt.expected {
				t.Errorf("WALType.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestNewWALRecord tests creating a new WAL record.
func TestNewWALRecord(t *testing.T) {
	record := NewWALRecord(1, 100, WALBegin)

	if record.LSN != 1 {
		t.Errorf("LSN = %v, want 1", record.LSN)
	}
	if record.TxID != 100 {
		t.Errorf("TxID = %v, want 100", record.TxID)
	}
	if record.Type != WALBegin {
		t.Errorf("Type = %v, want WALBegin", record.Type)
	}
	if record.PageID != 0 {
		t.Errorf("PageID = %v, want 0", record.PageID)
	}
	if record.OldData != nil {
		t.Errorf("OldData = %v, want nil", record.OldData)
	}
	if record.NewData != nil {
		t.Errorf("NewData = %v, want nil", record.NewData)
	}
}

// TestNewWALUpdateRecord tests creating a new WAL update record.
func TestNewWALUpdateRecord(t *testing.T) {
	oldData := []byte("old value")
	newData := []byte("new value")
	record := NewWALUpdateRecord(5, 200, PageID(42), 100, oldData, newData)

	if record.LSN != 5 {
		t.Errorf("LSN = %v, want 5", record.LSN)
	}
	if record.TxID != 200 {
		t.Errorf("TxID = %v, want 200", record.TxID)
	}
	if record.Type != WALUpdate {
		t.Errorf("Type = %v, want WALUpdate", record.Type)
	}
	if record.PageID != 42 {
		t.Errorf("PageID = %v, want 42", record.PageID)
	}
	if record.Offset != 100 {
		t.Errorf("Offset = %v, want 100", record.Offset)
	}
	if string(record.OldData) != "old value" {
		t.Errorf("OldData = %v, want 'old value'", string(record.OldData))
	}
	if string(record.NewData) != "new value" {
		t.Errorf("NewData = %v, want 'new value'", string(record.NewData))
	}
}

// TestWALRecordSize tests the Size method.
func TestWALRecordSize(t *testing.T) {
	tests := []struct {
		name     string
		record   *WALRecord
		expected int
	}{
		{
			name:     "empty record",
			record:   NewWALRecord(1, 1, WALBegin),
			expected: WALRecordHeaderSize,
		},
		{
			name:     "record with old data",
			record:   &WALRecord{OldData: make([]byte, 100)},
			expected: WALRecordHeaderSize + 100,
		},
		{
			name:     "record with new data",
			record:   &WALRecord{NewData: make([]byte, 200)},
			expected: WALRecordHeaderSize + 200,
		},
		{
			name:     "record with both data",
			record:   &WALRecord{OldData: make([]byte, 50), NewData: make([]byte, 75)},
			expected: WALRecordHeaderSize + 50 + 75,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.record.Size(); got != tt.expected {
				t.Errorf("Size() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestWALRecordSerializeDeserialize tests serialization and deserialization.
func TestWALRecordSerializeDeserialize(t *testing.T) {
	tests := []struct {
		name   string
		record *WALRecord
	}{
		{
			name:   "begin record",
			record: NewWALRecord(1, 100, WALBegin),
		},
		{
			name:   "commit record",
			record: NewWALRecord(2, 100, WALCommit),
		},
		{
			name:   "abort record",
			record: NewWALRecord(3, 100, WALAbort),
		},
		{
			name:   "checkpoint record",
			record: NewWALRecord(4, 0, WALCheckpoint),
		},
		{
			name:   "update record with data",
			record: NewWALUpdateRecord(5, 200, PageID(42), 100, []byte("old"), []byte("new")),
		},
		{
			name:   "update record with large data",
			record: NewWALUpdateRecord(6, 300, PageID(99), 500, make([]byte, 1000), make([]byte, 2000)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Serialize
			buf, err := tt.record.Serialize()
			if err != nil {
				t.Fatalf("Serialize() error = %v", err)
			}

			// Deserialize
			deserialized := &WALRecord{}
			if err := deserialized.Deserialize(buf); err != nil {
				t.Fatalf("Deserialize() error = %v", err)
			}

			// Compare fields
			if deserialized.LSN != tt.record.LSN {
				t.Errorf("LSN = %v, want %v", deserialized.LSN, tt.record.LSN)
			}
			if deserialized.TxID != tt.record.TxID {
				t.Errorf("TxID = %v, want %v", deserialized.TxID, tt.record.TxID)
			}
			if deserialized.Type != tt.record.Type {
				t.Errorf("Type = %v, want %v", deserialized.Type, tt.record.Type)
			}
			if deserialized.PageID != tt.record.PageID {
				t.Errorf("PageID = %v, want %v", deserialized.PageID, tt.record.PageID)
			}
			if deserialized.Offset != tt.record.Offset {
				t.Errorf("Offset = %v, want %v", deserialized.Offset, tt.record.Offset)
			}
			if string(deserialized.OldData) != string(tt.record.OldData) {
				t.Errorf("OldData mismatch")
			}
			if string(deserialized.NewData) != string(tt.record.NewData) {
				t.Errorf("NewData mismatch")
			}
		})
	}
}

// TestWALRecordChecksum tests checksum validation.
func TestWALRecordChecksum(t *testing.T) {
	record := NewWALUpdateRecord(1, 100, PageID(42), 50, []byte("old"), []byte("new"))

	// Serialize
	buf, err := record.Serialize()
	if err != nil {
		t.Fatalf("Serialize() error = %v", err)
	}

	// Deserialize and validate
	deserialized := &WALRecord{}
	if err := deserialized.DeserializeAndValidate(buf); err != nil {
		t.Fatalf("DeserializeAndValidate() error = %v", err)
	}

	// Corrupt the data
	buf[WALRecordHeaderSize] ^= 0xFF

	// Should fail validation
	corrupted := &WALRecord{}
	if err := corrupted.DeserializeAndValidate(buf); err != ErrWALRecordChecksum {
		t.Errorf("Expected ErrWALRecordChecksum, got %v", err)
	}
}

// TestWALRecordClone tests the Clone method.
func TestWALRecordClone(t *testing.T) {
	original := NewWALUpdateRecord(1, 100, PageID(42), 50, []byte("old"), []byte("new"))
	original.Checksum = 12345

	clone := original.Clone()

	// Verify clone has same values
	if clone.LSN != original.LSN {
		t.Errorf("LSN = %v, want %v", clone.LSN, original.LSN)
	}
	if clone.TxID != original.TxID {
		t.Errorf("TxID = %v, want %v", clone.TxID, original.TxID)
	}
	if clone.Type != original.Type {
		t.Errorf("Type = %v, want %v", clone.Type, original.Type)
	}
	if clone.Checksum != original.Checksum {
		t.Errorf("Checksum = %v, want %v", clone.Checksum, original.Checksum)
	}

	// Verify data is copied (not shared)
	clone.OldData[0] = 'X'
	if original.OldData[0] == 'X' {
		t.Error("Clone shares OldData with original")
	}

	clone.NewData[0] = 'Y'
	if original.NewData[0] == 'Y' {
		t.Error("Clone shares NewData with original")
	}
}

// TestWALRecordHelpers tests helper methods.
func TestWALRecordHelpers(t *testing.T) {
	beginRecord := NewWALRecord(1, 100, WALBegin)
	commitRecord := NewWALRecord(2, 100, WALCommit)
	abortRecord := NewWALRecord(3, 100, WALAbort)
	updateRecord := NewWALRecord(4, 100, WALUpdate)
	checkpointRecord := NewWALRecord(5, 0, WALCheckpoint)

	// Test IsTransactionControl
	if !beginRecord.IsTransactionControl() {
		t.Error("Begin should be transaction control")
	}
	if !commitRecord.IsTransactionControl() {
		t.Error("Commit should be transaction control")
	}
	if !abortRecord.IsTransactionControl() {
		t.Error("Abort should be transaction control")
	}
	if updateRecord.IsTransactionControl() {
		t.Error("Update should not be transaction control")
	}
	if checkpointRecord.IsTransactionControl() {
		t.Error("Checkpoint should not be transaction control")
	}

	// Test IsDataModification
	if beginRecord.IsDataModification() {
		t.Error("Begin should not be data modification")
	}
	if commitRecord.IsDataModification() {
		t.Error("Commit should not be data modification")
	}
	if !updateRecord.IsDataModification() {
		t.Error("Update should be data modification")
	}
}

// TestOpenWAL tests opening a new WAL file.
func TestOpenWAL(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "test.wal")

	wal, err := OpenWAL(walPath)
	if err != nil {
		t.Fatalf("OpenWAL() error = %v", err)
	}
	defer wal.Close()

	// Verify file was created
	if _, err := os.Stat(walPath); os.IsNotExist(err) {
		t.Error("WAL file was not created")
	}

	// Verify initial LSN
	if wal.CurrentLSN() != 1 {
		t.Errorf("CurrentLSN() = %v, want 1", wal.CurrentLSN())
	}
}

// TestWALAppend tests appending records to the WAL.
func TestWALAppend(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "test.wal")

	wal, err := OpenWAL(walPath)
	if err != nil {
		t.Fatalf("OpenWAL() error = %v", err)
	}
	defer wal.Close()

	// Append records
	record1 := NewWALRecord(0, 100, WALBegin)
	lsn1, err := wal.Append(record1)
	if err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	if lsn1 != 1 {
		t.Errorf("First LSN = %v, want 1", lsn1)
	}

	record2 := NewWALUpdateRecord(0, 100, PageID(42), 50, []byte("old"), []byte("new"))
	lsn2, err := wal.Append(record2)
	if err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	if lsn2 != 2 {
		t.Errorf("Second LSN = %v, want 2", lsn2)
	}

	record3 := NewWALRecord(0, 100, WALCommit)
	lsn3, err := wal.Append(record3)
	if err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	if lsn3 != 3 {
		t.Errorf("Third LSN = %v, want 3", lsn3)
	}

	// Verify LSN is monotonically increasing
	if wal.CurrentLSN() != 4 {
		t.Errorf("CurrentLSN() = %v, want 4", wal.CurrentLSN())
	}
}

// TestWALSync tests syncing the WAL to disk.
func TestWALSync(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "test.wal")

	wal, err := OpenWAL(walPath)
	if err != nil {
		t.Fatalf("OpenWAL() error = %v", err)
	}
	defer wal.Close()

	// Append a record
	record := NewWALRecord(0, 100, WALBegin)
	_, err = wal.Append(record)
	if err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	// Sync should not error
	if err := wal.Sync(); err != nil {
		t.Errorf("Sync() error = %v", err)
	}
}

// TestWALIterator tests iterating over WAL records.
func TestWALIterator(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "test.wal")

	wal, err := OpenWAL(walPath)
	if err != nil {
		t.Fatalf("OpenWAL() error = %v", err)
	}

	// Append multiple records
	records := []*WALRecord{
		NewWALRecord(0, 100, WALBegin),
		NewWALUpdateRecord(0, 100, PageID(1), 10, []byte("a"), []byte("b")),
		NewWALUpdateRecord(0, 100, PageID(2), 20, []byte("c"), []byte("d")),
		NewWALRecord(0, 100, WALCommit),
	}

	for _, r := range records {
		if _, err := wal.Append(r); err != nil {
			t.Fatalf("Append() error = %v", err)
		}
	}

	// Sync to ensure records are written
	if err := wal.Sync(); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	// Iterate from beginning
	iter := wal.Iterator(1)
	count := 0
	expectedLSNs := []uint64{1, 2, 3, 4}

	for iter.Next() {
		record, err := iter.Record()
		if err != nil {
			t.Fatalf("Record() error = %v", err)
		}

		if count >= len(expectedLSNs) {
			t.Fatalf("Too many records: got more than %d", len(expectedLSNs))
		}

		if record.LSN != expectedLSNs[count] {
			t.Errorf("Record %d LSN = %v, want %v", count, record.LSN, expectedLSNs[count])
		}

		count++
	}

	if count != len(expectedLSNs) {
		t.Errorf("Iterated %d records, want %d", count, len(expectedLSNs))
	}

	wal.Close()
}

// TestWALIteratorFromMiddle tests iterating from a specific LSN.
func TestWALIteratorFromMiddle(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "test.wal")

	wal, err := OpenWAL(walPath)
	if err != nil {
		t.Fatalf("OpenWAL() error = %v", err)
	}

	// Append 5 records
	for i := 0; i < 5; i++ {
		record := NewWALRecord(0, uint64(100+i), WALBegin)
		if _, err := wal.Append(record); err != nil {
			t.Fatalf("Append() error = %v", err)
		}
	}

	if err := wal.Sync(); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	// Iterate from LSN 3
	iter := wal.Iterator(3)
	count := 0
	expectedLSNs := []uint64{3, 4, 5}

	for iter.Next() {
		record, err := iter.Record()
		if err != nil {
			t.Fatalf("Record() error = %v", err)
		}

		if count >= len(expectedLSNs) {
			break
		}

		if record.LSN != expectedLSNs[count] {
			t.Errorf("Record %d LSN = %v, want %v", count, record.LSN, expectedLSNs[count])
		}

		count++
	}

	if count != len(expectedLSNs) {
		t.Errorf("Iterated %d records, want %d", count, len(expectedLSNs))
	}

	wal.Close()
}

// TestWALRecovery tests WAL recovery after reopening.
func TestWALRecovery(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "test.wal")

	// Create WAL and append records
	wal, err := OpenWAL(walPath)
	if err != nil {
		t.Fatalf("OpenWAL() error = %v", err)
	}

	records := []*WALRecord{
		NewWALRecord(0, 100, WALBegin),
		NewWALUpdateRecord(0, 100, PageID(1), 10, []byte("old1"), []byte("new1")),
		NewWALRecord(0, 100, WALCommit),
	}

	for _, r := range records {
		if _, err := wal.Append(r); err != nil {
			t.Fatalf("Append() error = %v", err)
		}
	}

	// Sync and close
	if err := wal.Sync(); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if err := wal.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	// Reopen WAL
	wal2, err := OpenWAL(walPath)
	if err != nil {
		t.Fatalf("OpenWAL() (reopen) error = %v", err)
	}
	defer wal2.Close()

	// Verify LSN continues from where it left off
	if wal2.CurrentLSN() != 4 {
		t.Errorf("CurrentLSN() after recovery = %v, want 4", wal2.CurrentLSN())
	}

	// Verify records can be read
	iter := wal2.Iterator(1)
	count := 0

	for iter.Next() {
		record, err := iter.Record()
		if err != nil {
			t.Fatalf("Record() error = %v", err)
		}

		if record.TxID != 100 {
			t.Errorf("Record %d TxID = %v, want 100", count, record.TxID)
		}

		count++
	}

	if count != 3 {
		t.Errorf("Recovered %d records, want 3", count)
	}
}

// TestWALTruncate tests truncating the WAL.
func TestWALTruncate(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "test.wal")

	wal, err := OpenWAL(walPath)
	if err != nil {
		t.Fatalf("OpenWAL() error = %v", err)
	}
	defer wal.Close()

	// Append 5 records
	for i := 0; i < 5; i++ {
		record := NewWALRecord(0, uint64(100+i), WALBegin)
		if _, err := wal.Append(record); err != nil {
			t.Fatalf("Append() error = %v", err)
		}
	}

	if err := wal.Sync(); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	// Truncate records with LSN <= 3
	if err := wal.Truncate(3); err != nil {
		t.Fatalf("Truncate() error = %v", err)
	}

	// Verify only records 4 and 5 remain
	iter := wal.Iterator(1)
	count := 0
	expectedLSNs := []uint64{4, 5}

	for iter.Next() {
		record, err := iter.Record()
		if err != nil {
			t.Fatalf("Record() error = %v", err)
		}

		if count >= len(expectedLSNs) {
			break
		}

		if record.LSN != expectedLSNs[count] {
			t.Errorf("Record %d LSN = %v, want %v", count, record.LSN, expectedLSNs[count])
		}

		count++
	}

	if count != len(expectedLSNs) {
		t.Errorf("After truncate: %d records, want %d", count, len(expectedLSNs))
	}
}

// TestWALTruncateAll tests truncating all records.
func TestWALTruncateAll(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "test.wal")

	wal, err := OpenWAL(walPath)
	if err != nil {
		t.Fatalf("OpenWAL() error = %v", err)
	}
	defer wal.Close()

	// Append 3 records
	for i := 0; i < 3; i++ {
		record := NewWALRecord(0, uint64(100+i), WALBegin)
		if _, err := wal.Append(record); err != nil {
			t.Fatalf("Append() error = %v", err)
		}
	}

	if err := wal.Sync(); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	// Truncate all records
	if err := wal.Truncate(3); err != nil {
		t.Fatalf("Truncate() error = %v", err)
	}

	// Verify no records remain
	iter := wal.Iterator(1)
	if iter.Next() {
		t.Error("Expected no records after truncating all")
	}
}

// TestWALClosedOperations tests operations on a closed WAL.
func TestWALClosedOperations(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "test.wal")

	wal, err := OpenWAL(walPath)
	if err != nil {
		t.Fatalf("OpenWAL() error = %v", err)
	}

	// Close the WAL
	if err := wal.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	// Append should fail
	record := NewWALRecord(0, 100, WALBegin)
	if _, err := wal.Append(record); err != ErrWALClosed {
		t.Errorf("Append() on closed WAL: got %v, want ErrWALClosed", err)
	}

	// Sync should fail
	if err := wal.Sync(); err != ErrWALClosed {
		t.Errorf("Sync() on closed WAL: got %v, want ErrWALClosed", err)
	}

	// Truncate should fail
	if err := wal.Truncate(1); err != ErrWALClosed {
		t.Errorf("Truncate() on closed WAL: got %v, want ErrWALClosed", err)
	}

	// Double close should not error
	if err := wal.Close(); err != nil {
		t.Errorf("Double Close() error = %v", err)
	}
}

// TestWALLSNMonotonicallyIncreasing tests that LSN is always increasing.
func TestWALLSNMonotonicallyIncreasing(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "test.wal")

	wal, err := OpenWAL(walPath)
	if err != nil {
		t.Fatalf("OpenWAL() error = %v", err)
	}
	defer wal.Close()

	var prevLSN uint64 = 0

	// Append 100 records and verify LSN is always increasing
	for i := 0; i < 100; i++ {
		record := NewWALRecord(0, uint64(i), WALBegin)
		lsn, err := wal.Append(record)
		if err != nil {
			t.Fatalf("Append() error = %v", err)
		}

		if lsn <= prevLSN {
			t.Errorf("LSN not monotonically increasing: prev=%d, current=%d", prevLSN, lsn)
		}

		prevLSN = lsn
	}
}

// TestWALLargeRecords tests handling of large WAL records.
func TestWALLargeRecords(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "test.wal")

	wal, err := OpenWAL(walPath)
	if err != nil {
		t.Fatalf("OpenWAL() error = %v", err)
	}
	defer wal.Close()

	// Create a large record (but within limits)
	largeData := make([]byte, 10000)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	record := NewWALUpdateRecord(0, 100, PageID(1), 0, largeData, largeData)
	lsn, err := wal.Append(record)
	if err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	if err := wal.Sync(); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	// Read back and verify
	iter := wal.Iterator(lsn)
	if !iter.Next() {
		t.Fatal("Expected to find record")
	}

	readRecord, err := iter.Record()
	if err != nil {
		t.Fatalf("Record() error = %v", err)
	}

	if len(readRecord.OldData) != len(largeData) {
		t.Errorf("OldData length = %d, want %d", len(readRecord.OldData), len(largeData))
	}

	if len(readRecord.NewData) != len(largeData) {
		t.Errorf("NewData length = %d, want %d", len(readRecord.NewData), len(largeData))
	}

	// Verify data integrity
	for i := range largeData {
		if readRecord.OldData[i] != largeData[i] {
			t.Errorf("OldData mismatch at index %d", i)
			break
		}
		if readRecord.NewData[i] != largeData[i] {
			t.Errorf("NewData mismatch at index %d", i)
			break
		}
	}
}

// TestWALRecordDataTooLarge tests that oversized data is rejected.
func TestWALRecordDataTooLarge(t *testing.T) {
	// Create a record with data exceeding MaxWALDataSize
	oversizedData := make([]byte, MaxWALDataSize+1)
	record := NewWALUpdateRecord(1, 100, PageID(1), 0, oversizedData, nil)

	_, err := record.Serialize()
	if err != ErrWALDataTooLarge {
		t.Errorf("Expected ErrWALDataTooLarge, got %v", err)
	}
}

// TestWALRecordSerializeToSmallBuffer tests serialization to a small buffer.
func TestWALRecordSerializeToSmallBuffer(t *testing.T) {
	record := NewWALRecord(1, 100, WALBegin)
	smallBuf := make([]byte, 10) // Too small

	err := record.SerializeTo(smallBuf)
	if err != ErrWALRecordTooSmall {
		t.Errorf("Expected ErrWALRecordTooSmall, got %v", err)
	}
}

// TestWALRecordDeserializeSmallBuffer tests deserialization from a small buffer.
func TestWALRecordDeserializeSmallBuffer(t *testing.T) {
	record := &WALRecord{}
	smallBuf := make([]byte, 10) // Too small

	err := record.Deserialize(smallBuf)
	if err != ErrWALRecordTooSmall {
		t.Errorf("Expected ErrWALRecordTooSmall, got %v", err)
	}
}
