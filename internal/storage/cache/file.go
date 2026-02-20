package cache

import (
	"io"
	"os"
	"path/filepath"
)

// WriteFile writes cache data to a file atomically.
// Uses tmp + rename pattern to prevent corruption.
func WriteFile(path string, cacheType uint8, data []byte, entryCount, lastTxID uint64) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Create header
	header := NewHeader(cacheType, entryCount, lastTxID, data)
	headerBytes := header.Serialize()

	// Write to temp file
	tmpPath := path + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	// Write header
	if _, err := f.Write(headerBytes); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return err
	}

	// Write data
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return err
	}

	// Sync to disk
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return err
	}

	if err := f.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}

	// Atomic rename
	return os.Rename(tmpPath, path)
}

// ReadFile reads cache data from a file.
// Returns the data and header if valid, or an error if cache is missing/stale/corrupt.
func ReadFile(path string, expectedType uint8, expectedTxID uint64) ([]byte, *Header, error) {
	// Open file
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	// Read header
	headerBuf := make([]byte, HeaderSize)
	if _, err := io.ReadFull(f, headerBuf); err != nil {
		return nil, nil, err
	}

	// Parse header
	header := &Header{}
	if err := header.Deserialize(headerBuf); err != nil {
		return nil, nil, err
	}

	// Validate header CRC
	if err := header.ValidateHeaderCRC(headerBuf); err != nil {
		return nil, nil, err
	}

	// Validate header fields
	if err := header.Validate(expectedType, expectedTxID); err != nil {
		return nil, nil, err
	}

	// Read data
	data := make([]byte, header.DataLength)
	if _, err := io.ReadFull(f, data); err != nil {
		return nil, nil, err
	}

	// Validate data CRC
	if err := header.ValidateDataCRC(data); err != nil {
		return nil, nil, err
	}

	return data, header, nil
}

// Exists checks if a cache file exists.
func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Remove removes a cache file.
func Remove(path string) error {
	return os.Remove(path)
}
