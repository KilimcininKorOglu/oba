package crypto

import (
	"bytes"
	"testing"
)

func TestCryptoWriter(t *testing.T) {
	key, _ := GenerateKey()
	encKey, _ := NewEncryptionKey(key)

	var buf bytes.Buffer
	writer := NewCryptoWriter(&buf, encKey)

	data := []byte("test record data")
	n, err := writer.WriteRecord(data)
	if err != nil {
		t.Fatalf("WriteRecord() error = %v", err)
	}

	// Should write length prefix (4) + nonce (12) + data + tag (16)
	expectedSize := 4 + NonceSize + len(data) + TagSize
	if n != expectedSize {
		t.Errorf("WriteRecord() wrote %d bytes, want %d", n, expectedSize)
	}
}

func TestCryptoReader(t *testing.T) {
	key, _ := GenerateKey()
	encKey, _ := NewEncryptionKey(key)

	// Write encrypted record
	var buf bytes.Buffer
	writer := NewCryptoWriter(&buf, encKey)

	data := []byte("test record data")
	writer.WriteRecord(data)

	// Read it back
	reader := NewCryptoReader(bytes.NewReader(buf.Bytes()), encKey)

	decrypted, err := reader.ReadRecord()
	if err != nil {
		t.Fatalf("ReadRecord() error = %v", err)
	}

	if !bytes.Equal(data, decrypted) {
		t.Errorf("ReadRecord() = %q, want %q", decrypted, data)
	}
}

func TestCryptoWriterMultipleRecords(t *testing.T) {
	key, _ := GenerateKey()
	encKey, _ := NewEncryptionKey(key)

	var buf bytes.Buffer
	writer := NewCryptoWriter(&buf, encKey)

	records := [][]byte{
		[]byte("first record"),
		[]byte("second record with more data"),
		[]byte("third"),
	}

	for _, data := range records {
		if _, err := writer.WriteRecord(data); err != nil {
			t.Fatalf("WriteRecord() error = %v", err)
		}
	}

	// Read all records back
	reader := NewCryptoReader(bytes.NewReader(buf.Bytes()), encKey)

	for i, expected := range records {
		decrypted, err := reader.ReadRecord()
		if err != nil {
			t.Fatalf("ReadRecord() %d error = %v", i, err)
		}

		if !bytes.Equal(expected, decrypted) {
			t.Errorf("ReadRecord() %d = %q, want %q", i, decrypted, expected)
		}
	}
}

func TestCryptoWriterWrite(t *testing.T) {
	key, _ := GenerateKey()
	encKey, _ := NewEncryptionKey(key)

	var buf bytes.Buffer
	writer := NewCryptoWriter(&buf, encKey)

	data := []byte("raw encrypted data")
	n, err := writer.Write(data)
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	expectedSize := NonceSize + len(data) + TagSize
	if n != expectedSize {
		t.Errorf("Write() wrote %d bytes, want %d", n, expectedSize)
	}

	// Read it back
	reader := NewCryptoReader(bytes.NewReader(buf.Bytes()), encKey)

	decrypted, err := reader.Read(n)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}

	if !bytes.Equal(data, decrypted) {
		t.Errorf("Read() = %q, want %q", decrypted, data)
	}
}

func TestEncryptedSize(t *testing.T) {
	tests := []struct {
		plaintext int
		expected  int
	}{
		{0, NonceSize + TagSize},
		{100, NonceSize + 100 + TagSize},
		{1024, NonceSize + 1024 + TagSize},
	}

	for _, tt := range tests {
		got := EncryptedSize(tt.plaintext)
		if got != tt.expected {
			t.Errorf("EncryptedSize(%d) = %d, want %d", tt.plaintext, got, tt.expected)
		}
	}
}

func TestPlaintextSize(t *testing.T) {
	tests := []struct {
		encrypted int
		expected  int
	}{
		{NonceSize + TagSize, 0},
		{NonceSize + 100 + TagSize, 100},
		{NonceSize + 1024 + TagSize, 1024},
	}

	for _, tt := range tests {
		got := PlaintextSize(tt.encrypted)
		if got != tt.expected {
			t.Errorf("PlaintextSize(%d) = %d, want %d", tt.encrypted, got, tt.expected)
		}
	}
}
