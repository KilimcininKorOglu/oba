// Package crypto provides AES-256-GCM encryption for ObaDB storage.
//
// This package implements transparent encryption at rest with:
//   - AES-256-GCM authenticated encryption
//   - Per-record unique nonce (12 bytes)
//   - Key rotation support
//   - Minimal performance overhead (~5-10%)
//
// Encrypted Record Format:
//
//	+--------+------------+----------------+----------+
//	| Nonce  | DataLength | Encrypted Data | Auth Tag |
//	| 12 B   | 4 B        | Variable       | 16 B     |
//	+--------+------------+----------------+----------+
//
// Usage:
//
//	// Generate a new key
//	key, err := crypto.GenerateKey()
//
//	// Create encryption key
//	encKey, err := crypto.NewEncryptionKey(key)
//
//	// Encrypt data
//	ciphertext, err := encKey.Encrypt(plaintext)
//
//	// Decrypt data
//	plaintext, err := encKey.Decrypt(ciphertext)
package crypto
