package crypto

import (
	"fmt"
	"os"
	"path/filepath"
)

// KeyRotator handles encryption key rotation for ObaDB.
type KeyRotator struct {
	oldKey *EncryptionKey
	newKey *EncryptionKey
}

// NewKeyRotator creates a new key rotator.
func NewKeyRotator(oldKey, newKey *EncryptionKey) *KeyRotator {
	return &KeyRotator{
		oldKey: oldKey,
		newKey: newKey,
	}
}

// RotateFile re-encrypts a single file with the new key.
// Creates a backup before rotation.
func (kr *KeyRotator) RotateFile(path string) error {
	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	// Decrypt with old key
	plaintext, err := kr.oldKey.Decrypt(data)
	if err != nil {
		return fmt.Errorf("decrypt: %w", err)
	}

	// Create backup
	backupPath := path + ".bak"
	if err := os.WriteFile(backupPath, data, 0600); err != nil {
		return fmt.Errorf("create backup: %w", err)
	}

	// Encrypt with new key
	ciphertext, err := kr.newKey.Encrypt(plaintext)
	if err != nil {
		return fmt.Errorf("encrypt: %w", err)
	}

	// Write new file
	if err := os.WriteFile(path, ciphertext, 0600); err != nil {
		// Restore from backup
		os.Rename(backupPath, path)
		return fmt.Errorf("write file: %w", err)
	}

	// Remove backup
	os.Remove(backupPath)

	return nil
}

// RotateDirectory re-encrypts all files in a directory.
func (kr *KeyRotator) RotateDirectory(dir string, pattern string) error {
	matches, err := filepath.Glob(filepath.Join(dir, pattern))
	if err != nil {
		return err
	}

	for _, path := range matches {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if info.IsDir() {
			continue
		}

		if err := kr.RotateFile(path); err != nil {
			return fmt.Errorf("rotate %s: %w", path, err)
		}
	}

	return nil
}

// ReEncryptData re-encrypts data from old key to new key.
func (kr *KeyRotator) ReEncryptData(ciphertext []byte) ([]byte, error) {
	plaintext, err := kr.oldKey.Decrypt(ciphertext)
	if err != nil {
		return nil, err
	}
	return kr.newKey.Encrypt(plaintext)
}

// GenerateNewKey generates a new encryption key and saves it to a file.
func GenerateNewKey(path string) error {
	key, err := GenerateKey()
	if err != nil {
		return err
	}
	return SaveKeyToFile(key, path)
}
