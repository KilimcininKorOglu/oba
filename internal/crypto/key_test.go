package crypto

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateKey(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}

	if len(key) != KeySize {
		t.Errorf("GenerateKey() key length = %d, want %d", len(key), KeySize)
	}

	// Keys should be unique
	key2, _ := GenerateKey()
	if bytes.Equal(key, key2) {
		t.Error("GenerateKey() generated duplicate keys")
	}
}

func TestNewEncryptionKey(t *testing.T) {
	key, _ := GenerateKey()

	encKey, err := NewEncryptionKey(key)
	if err != nil {
		t.Fatalf("NewEncryptionKey() error = %v", err)
	}

	if encKey == nil {
		t.Fatal("NewEncryptionKey() returned nil")
	}
}

func TestNewEncryptionKeyInvalidSize(t *testing.T) {
	tests := []struct {
		name    string
		keySize int
	}{
		{"too short", 16},
		{"too long", 64},
		{"empty", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := make([]byte, tt.keySize)
			_, err := NewEncryptionKey(key)
			if err != ErrInvalidKey {
				t.Errorf("NewEncryptionKey() error = %v, want %v", err, ErrInvalidKey)
			}
		})
	}
}

func TestEncryptDecrypt(t *testing.T) {
	key, _ := GenerateKey()
	encKey, _ := NewEncryptionKey(key)

	plaintext := []byte("Hello, World! This is a test message.")

	ciphertext, err := encKey.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	// Ciphertext should be different from plaintext
	if bytes.Equal(plaintext, ciphertext) {
		t.Error("Encrypt() ciphertext equals plaintext")
	}

	// Ciphertext should be larger (nonce + tag overhead)
	expectedSize := len(plaintext) + NonceSize + TagSize
	if len(ciphertext) != expectedSize {
		t.Errorf("Encrypt() ciphertext size = %d, want %d", len(ciphertext), expectedSize)
	}

	// Decrypt should return original
	decrypted, err := encKey.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Errorf("Decrypt() = %q, want %q", decrypted, plaintext)
	}
}

func TestEncryptDecryptEmpty(t *testing.T) {
	key, _ := GenerateKey()
	encKey, _ := NewEncryptionKey(key)

	plaintext := []byte{}

	ciphertext, err := encKey.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	decrypted, err := encKey.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}

	if len(decrypted) != 0 {
		t.Errorf("Decrypt() = %q, want empty", decrypted)
	}
}

func TestEncryptDecryptLargeData(t *testing.T) {
	key, _ := GenerateKey()
	encKey, _ := NewEncryptionKey(key)

	// 1MB of data
	plaintext := make([]byte, 1024*1024)
	for i := range plaintext {
		plaintext[i] = byte(i % 256)
	}

	ciphertext, err := encKey.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	decrypted, err := encKey.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Error("Decrypt() large data mismatch")
	}
}

func TestDecryptWrongKey(t *testing.T) {
	key1, _ := GenerateKey()
	key2, _ := GenerateKey()

	encKey1, _ := NewEncryptionKey(key1)
	encKey2, _ := NewEncryptionKey(key2)

	plaintext := []byte("secret message")
	ciphertext, _ := encKey1.Encrypt(plaintext)

	// Decrypt with wrong key should fail
	_, err := encKey2.Decrypt(ciphertext)
	if err != ErrDecryptFailed {
		t.Errorf("Decrypt() with wrong key error = %v, want %v", err, ErrDecryptFailed)
	}
}

func TestDecryptTamperedData(t *testing.T) {
	key, _ := GenerateKey()
	encKey, _ := NewEncryptionKey(key)

	plaintext := []byte("secret message")
	ciphertext, _ := encKey.Encrypt(plaintext)

	// Tamper with ciphertext
	ciphertext[len(ciphertext)-1] ^= 0xFF

	// Decrypt should fail due to authentication
	_, err := encKey.Decrypt(ciphertext)
	if err != ErrDecryptFailed {
		t.Errorf("Decrypt() tampered data error = %v, want %v", err, ErrDecryptFailed)
	}
}

func TestDecryptTooShort(t *testing.T) {
	key, _ := GenerateKey()
	encKey, _ := NewEncryptionKey(key)

	// Too short to contain nonce + tag
	shortData := make([]byte, NonceSize+TagSize-1)

	_, err := encKey.Decrypt(shortData)
	if err != ErrInvalidCiphertext {
		t.Errorf("Decrypt() short data error = %v, want %v", err, ErrInvalidCiphertext)
	}
}

func TestEncryptWithNonce(t *testing.T) {
	key, _ := GenerateKey()
	encKey, _ := NewEncryptionKey(key)

	plaintext := []byte("test message")
	nonce := make([]byte, NonceSize)
	for i := range nonce {
		nonce[i] = byte(i)
	}

	ciphertext, err := encKey.EncryptWithNonce(plaintext, nonce)
	if err != nil {
		t.Fatalf("EncryptWithNonce() error = %v", err)
	}

	// Verify nonce is at the beginning
	if !bytes.Equal(ciphertext[:NonceSize], nonce) {
		t.Error("EncryptWithNonce() nonce not at beginning")
	}

	// Should decrypt correctly
	decrypted, err := encKey.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Errorf("Decrypt() = %q, want %q", decrypted, plaintext)
	}
}

func TestEncryptWithNonceInvalidSize(t *testing.T) {
	key, _ := GenerateKey()
	encKey, _ := NewEncryptionKey(key)

	plaintext := []byte("test")
	badNonce := make([]byte, NonceSize-1)

	_, err := encKey.EncryptWithNonce(plaintext, badNonce)
	if err != ErrInvalidNonce {
		t.Errorf("EncryptWithNonce() error = %v, want %v", err, ErrInvalidNonce)
	}
}

func TestKeyBytes(t *testing.T) {
	key, _ := GenerateKey()
	encKey, _ := NewEncryptionKey(key)

	keyBytes := encKey.KeyBytes()

	if !bytes.Equal(key, keyBytes) {
		t.Error("KeyBytes() returned different key")
	}

	// Modifying returned bytes should not affect original
	keyBytes[0] ^= 0xFF
	keyBytes2 := encKey.KeyBytes()
	if keyBytes2[0] == keyBytes[0] {
		t.Error("KeyBytes() returned reference to internal key")
	}
}

func TestClear(t *testing.T) {
	key, _ := GenerateKey()
	encKey, _ := NewEncryptionKey(key)

	encKey.Clear()

	keyBytes := encKey.KeyBytes()
	for _, b := range keyBytes {
		if b != 0 {
			t.Error("Clear() did not zero key")
			break
		}
	}
}

func TestLoadKeyFromFile(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("raw bytes", func(t *testing.T) {
		key, _ := GenerateKey()
		path := filepath.Join(tmpDir, "raw.key")

		if err := os.WriteFile(path, key, 0600); err != nil {
			t.Fatal(err)
		}

		encKey, err := LoadKeyFromFile(path)
		if err != nil {
			t.Fatalf("LoadKeyFromFile() error = %v", err)
		}

		if !bytes.Equal(key, encKey.KeyBytes()) {
			t.Error("LoadKeyFromFile() key mismatch")
		}
	})

	t.Run("hex encoded", func(t *testing.T) {
		key, _ := GenerateKey()
		path := filepath.Join(tmpDir, "hex.key")

		if err := SaveKeyToFile(key, path); err != nil {
			t.Fatal(err)
		}

		encKey, err := LoadKeyFromFile(path)
		if err != nil {
			t.Fatalf("LoadKeyFromFile() error = %v", err)
		}

		if !bytes.Equal(key, encKey.KeyBytes()) {
			t.Error("LoadKeyFromFile() key mismatch")
		}
	})

	t.Run("with whitespace", func(t *testing.T) {
		key, _ := GenerateKey()
		path := filepath.Join(tmpDir, "whitespace.key")

		if err := SaveKeyToFile(key, path); err != nil {
			t.Fatal(err)
		}

		// Add whitespace
		data, _ := os.ReadFile(path)
		os.WriteFile(path, append(data, '\n', ' ', '\t'), 0600)

		encKey, err := LoadKeyFromFile(path)
		if err != nil {
			t.Fatalf("LoadKeyFromFile() error = %v", err)
		}

		if !bytes.Equal(key, encKey.KeyBytes()) {
			t.Error("LoadKeyFromFile() key mismatch with whitespace")
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := LoadKeyFromFile("/nonexistent/path")
		if err != ErrKeyFileNotFound {
			t.Errorf("LoadKeyFromFile() error = %v, want %v", err, ErrKeyFileNotFound)
		}
	})

	t.Run("invalid format", func(t *testing.T) {
		path := filepath.Join(tmpDir, "invalid.key")
		os.WriteFile(path, []byte("not a valid key"), 0600)

		_, err := LoadKeyFromFile(path)
		if err != ErrInvalidKeyFormat {
			t.Errorf("LoadKeyFromFile() error = %v, want %v", err, ErrInvalidKeyFormat)
		}
	})
}

func TestSaveKeyToFile(t *testing.T) {
	tmpDir := t.TempDir()
	key, _ := GenerateKey()
	path := filepath.Join(tmpDir, "test.key")

	if err := SaveKeyToFile(key, path); err != nil {
		t.Fatalf("SaveKeyToFile() error = %v", err)
	}

	// Verify file permissions
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if info.Mode().Perm() != 0600 {
		t.Errorf("SaveKeyToFile() permissions = %o, want 0600", info.Mode().Perm())
	}

	// Verify content is hex encoded
	data, _ := os.ReadFile(path)
	if len(data) != KeySize*2 {
		t.Errorf("SaveKeyToFile() file size = %d, want %d", len(data), KeySize*2)
	}
}

func TestSaveKeyToFileInvalidKey(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.key")

	err := SaveKeyToFile([]byte("short"), path)
	if err != ErrInvalidKey {
		t.Errorf("SaveKeyToFile() error = %v, want %v", err, ErrInvalidKey)
	}
}
