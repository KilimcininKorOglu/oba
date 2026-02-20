package crypto

import (
	"os"
	"path/filepath"
	"testing"
)

func TestKeyRotator(t *testing.T) {
	oldKey, _ := GenerateKey()
	newKey, _ := GenerateKey()

	oldEncKey, _ := NewEncryptionKey(oldKey)
	newEncKey, _ := NewEncryptionKey(newKey)

	rotator := NewKeyRotator(oldEncKey, newEncKey)

	// Test ReEncryptData
	plaintext := []byte("secret data")
	ciphertext, _ := oldEncKey.Encrypt(plaintext)

	reEncrypted, err := rotator.ReEncryptData(ciphertext)
	if err != nil {
		t.Fatalf("ReEncryptData() error = %v", err)
	}

	// Should decrypt with new key
	decrypted, err := newEncKey.Decrypt(reEncrypted)
	if err != nil {
		t.Fatalf("Decrypt() with new key error = %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("ReEncryptData() decrypted = %q, want %q", decrypted, plaintext)
	}

	// Should NOT decrypt with old key
	_, err = oldEncKey.Decrypt(reEncrypted)
	if err == nil {
		t.Error("ReEncryptData() should not decrypt with old key")
	}
}

func TestKeyRotatorRotateFile(t *testing.T) {
	tmpDir := t.TempDir()

	oldKey, _ := GenerateKey()
	newKey, _ := GenerateKey()

	oldEncKey, _ := NewEncryptionKey(oldKey)
	newEncKey, _ := NewEncryptionKey(newKey)

	// Create encrypted file with old key
	plaintext := []byte("file content to rotate")
	ciphertext, _ := oldEncKey.Encrypt(plaintext)

	filePath := filepath.Join(tmpDir, "test.enc")
	if err := os.WriteFile(filePath, ciphertext, 0600); err != nil {
		t.Fatal(err)
	}

	// Rotate
	rotator := NewKeyRotator(oldEncKey, newEncKey)
	if err := rotator.RotateFile(filePath); err != nil {
		t.Fatalf("RotateFile() error = %v", err)
	}

	// Read rotated file
	rotatedData, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}

	// Should decrypt with new key
	decrypted, err := newEncKey.Decrypt(rotatedData)
	if err != nil {
		t.Fatalf("Decrypt() rotated file error = %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("RotateFile() decrypted = %q, want %q", decrypted, plaintext)
	}

	// Backup should be removed
	backupPath := filePath + ".bak"
	if _, err := os.Stat(backupPath); !os.IsNotExist(err) {
		t.Error("RotateFile() backup file should be removed")
	}
}

func TestKeyRotatorRotateDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	oldKey, _ := GenerateKey()
	newKey, _ := GenerateKey()

	oldEncKey, _ := NewEncryptionKey(oldKey)
	newEncKey, _ := NewEncryptionKey(newKey)

	// Create multiple encrypted files
	files := []string{"file1.enc", "file2.enc", "file3.enc"}
	contents := []string{"content1", "content2", "content3"}

	for i, name := range files {
		ciphertext, _ := oldEncKey.Encrypt([]byte(contents[i]))
		path := filepath.Join(tmpDir, name)
		os.WriteFile(path, ciphertext, 0600)
	}

	// Also create a non-matching file
	os.WriteFile(filepath.Join(tmpDir, "other.txt"), []byte("plain"), 0600)

	// Rotate all .enc files
	rotator := NewKeyRotator(oldEncKey, newEncKey)
	if err := rotator.RotateDirectory(tmpDir, "*.enc"); err != nil {
		t.Fatalf("RotateDirectory() error = %v", err)
	}

	// Verify all .enc files are rotated
	for i, name := range files {
		path := filepath.Join(tmpDir, name)
		data, _ := os.ReadFile(path)

		decrypted, err := newEncKey.Decrypt(data)
		if err != nil {
			t.Errorf("RotateDirectory() file %s decrypt error = %v", name, err)
			continue
		}

		if string(decrypted) != contents[i] {
			t.Errorf("RotateDirectory() file %s = %q, want %q", name, decrypted, contents[i])
		}
	}

	// Non-matching file should be unchanged
	otherData, _ := os.ReadFile(filepath.Join(tmpDir, "other.txt"))
	if string(otherData) != "plain" {
		t.Error("RotateDirectory() modified non-matching file")
	}
}

func TestGenerateNewKey(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "new.key")

	if err := GenerateNewKey(path); err != nil {
		t.Fatalf("GenerateNewKey() error = %v", err)
	}

	// Should be loadable
	encKey, err := LoadKeyFromFile(path)
	if err != nil {
		t.Fatalf("LoadKeyFromFile() error = %v", err)
	}

	// Should be usable
	plaintext := []byte("test")
	ciphertext, err := encKey.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	decrypted, err := encKey.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("GenerateNewKey() key not working properly")
	}
}
