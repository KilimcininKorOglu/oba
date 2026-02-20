package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"strings"
)

// Constants for AES-256-GCM encryption.
const (
	NonceSize = 12 // GCM standard nonce size
	TagSize   = 16 // GCM authentication tag size
	KeySize   = 32 // AES-256 key size
)

// Errors returned by crypto operations.
var (
	ErrInvalidKey       = errors.New("invalid encryption key: must be 32 bytes")
	ErrDecryptFailed    = errors.New("decryption failed: authentication error")
	ErrInvalidNonce     = errors.New("invalid nonce: must be 12 bytes")
	ErrInvalidCiphertext = errors.New("invalid ciphertext: too short")
	ErrKeyFileNotFound  = errors.New("encryption key file not found")
	ErrInvalidKeyFormat = errors.New("invalid key format: must be 32 bytes or 64 hex chars")
)

// EncryptionKey holds the AES-256 key and cipher instance.
type EncryptionKey struct {
	key    []byte
	cipher cipher.AEAD
}

// NewEncryptionKey creates a new encryption key from raw bytes.
func NewEncryptionKey(key []byte) (*EncryptionKey, error) {
	if len(key) != KeySize {
		return nil, ErrInvalidKey
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	keyCopy := make([]byte, KeySize)
	copy(keyCopy, key)

	return &EncryptionKey{
		key:    keyCopy,
		cipher: gcm,
	}, nil
}

// GenerateKey generates a new random 256-bit encryption key.
func GenerateKey() ([]byte, error) {
	key := make([]byte, KeySize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, err
	}
	return key, nil
}

// LoadKeyFromFile loads an encryption key from a file.
// The file can contain either raw 32 bytes or 64 hex characters.
func LoadKeyFromFile(path string) (*EncryptionKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrKeyFileNotFound
		}
		return nil, err
	}

	var key []byte

	// Check if it's raw 32 bytes first
	if len(data) == KeySize {
		key = data
	} else {
		// Trim whitespace for hex-encoded keys
		trimmed := []byte(strings.TrimSpace(string(data)))

		switch len(trimmed) {
		case KeySize:
			// Raw 32 bytes (after trimming whitespace)
			key = trimmed
		case KeySize * 2:
			// Hex encoded (64 chars)
			key = make([]byte, KeySize)
			if _, err := hex.Decode(key, trimmed); err != nil {
				return nil, ErrInvalidKeyFormat
			}
		default:
			return nil, ErrInvalidKeyFormat
		}
	}

	return NewEncryptionKey(key)
}

// SaveKeyToFile saves an encryption key to a file in hex format.
func SaveKeyToFile(key []byte, path string) error {
	if len(key) != KeySize {
		return ErrInvalidKey
	}

	hexKey := hex.EncodeToString(key)
	return os.WriteFile(path, []byte(hexKey), 0600)
}

// Encrypt encrypts plaintext using AES-256-GCM with a random nonce.
// Returns: nonce (12 bytes) + ciphertext + auth tag (16 bytes)
func (k *EncryptionKey) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, NonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	// Seal appends ciphertext + auth tag to nonce
	ciphertext := k.cipher.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt decrypts ciphertext that was encrypted with Encrypt.
// Input format: nonce (12 bytes) + ciphertext + auth tag (16 bytes)
func (k *EncryptionKey) Decrypt(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < NonceSize+TagSize {
		return nil, ErrInvalidCiphertext
	}

	nonce := ciphertext[:NonceSize]
	encrypted := ciphertext[NonceSize:]

	plaintext, err := k.cipher.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return nil, ErrDecryptFailed
	}

	return plaintext, nil
}

// EncryptWithNonce encrypts with a specific nonce (for deterministic encryption).
// Warning: Using the same nonce twice with the same key breaks security.
func (k *EncryptionKey) EncryptWithNonce(plaintext, nonce []byte) ([]byte, error) {
	if len(nonce) != NonceSize {
		return nil, ErrInvalidNonce
	}

	result := make([]byte, NonceSize)
	copy(result, nonce)

	ciphertext := k.cipher.Seal(result, nonce, plaintext, nil)
	return ciphertext, nil
}

// KeyBytes returns a copy of the raw key bytes.
func (k *EncryptionKey) KeyBytes() []byte {
	keyCopy := make([]byte, KeySize)
	copy(keyCopy, k.key)
	return keyCopy
}

// Clear zeros out the key material for security.
func (k *EncryptionKey) Clear() {
	for i := range k.key {
		k.key[i] = 0
	}
}
