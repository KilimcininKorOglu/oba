package crypto

import (
	"encoding/binary"
	"io"
)

// RecordOverhead is the total overhead added to each encrypted record.
// Nonce (12) + Length (4) + Auth Tag (16) = 32 bytes
const RecordOverhead = NonceSize + 4 + TagSize

// CryptoWriter wraps an io.Writer with encryption.
type CryptoWriter struct {
	w   io.Writer
	key *EncryptionKey
}

// NewCryptoWriter creates a new encrypting writer.
func NewCryptoWriter(w io.Writer, key *EncryptionKey) *CryptoWriter {
	return &CryptoWriter{w: w, key: key}
}

// WriteRecord encrypts and writes a record with length prefix.
// Format: [length:4][nonce:12][ciphertext][tag:16]
func (cw *CryptoWriter) WriteRecord(data []byte) (int, error) {
	encrypted, err := cw.key.Encrypt(data)
	if err != nil {
		return 0, err
	}

	// Write length prefix (4 bytes)
	length := uint32(len(encrypted))
	if err := binary.Write(cw.w, binary.LittleEndian, length); err != nil {
		return 0, err
	}

	// Write encrypted data
	n, err := cw.w.Write(encrypted)
	if err != nil {
		return 4, err
	}

	return 4 + n, nil
}

// Write encrypts and writes data without length prefix.
func (cw *CryptoWriter) Write(data []byte) (int, error) {
	encrypted, err := cw.key.Encrypt(data)
	if err != nil {
		return 0, err
	}
	return cw.w.Write(encrypted)
}

// CryptoReader wraps an io.Reader with decryption.
type CryptoReader struct {
	r   io.Reader
	key *EncryptionKey
}

// NewCryptoReader creates a new decrypting reader.
func NewCryptoReader(r io.Reader, key *EncryptionKey) *CryptoReader {
	return &CryptoReader{r: r, key: key}
}

// ReadRecord reads and decrypts a length-prefixed record.
func (cr *CryptoReader) ReadRecord() ([]byte, error) {
	// Read length prefix
	var length uint32
	if err := binary.Read(cr.r, binary.LittleEndian, &length); err != nil {
		return nil, err
	}

	// Sanity check
	if length > 100*1024*1024 { // 100MB max
		return nil, ErrInvalidCiphertext
	}

	// Read encrypted data
	encrypted := make([]byte, length)
	if _, err := io.ReadFull(cr.r, encrypted); err != nil {
		return nil, err
	}

	return cr.key.Decrypt(encrypted)
}

// Read reads and decrypts data of known size.
func (cr *CryptoReader) Read(size int) ([]byte, error) {
	encrypted := make([]byte, size)
	if _, err := io.ReadFull(cr.r, encrypted); err != nil {
		return nil, err
	}
	return cr.key.Decrypt(encrypted)
}

// EncryptedSize returns the size of encrypted data for a given plaintext size.
func EncryptedSize(plaintextSize int) int {
	return NonceSize + plaintextSize + TagSize
}

// PlaintextSize returns the size of plaintext for a given encrypted data size.
func PlaintextSize(encryptedSize int) int {
	return encryptedSize - NonceSize - TagSize
}
