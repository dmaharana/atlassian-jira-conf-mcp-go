package vault

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestEncryptDecrypt(t *testing.T) {
	key := bytes.Repeat([]byte("k"), 32)
	enc, err := NewEncryptor(key)
	if err != nil {
		t.Fatalf("unexpected NewEncryptor error: %v", err)
	}

	original := []byte("secret-atlassian-oauth-refresh-token-xyz-1234567890")
	ciphertext, err := enc.Encrypt(original)
	if err != nil {
		t.Fatalf("encrypt error: %v", err)
	}

	if bytes.Equal(ciphertext, original) {
		t.Fatalf("ciphertext must not match original plaintext")
	}

	decrypted, err := enc.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("decrypt error: %v", err)
	}

	if !bytes.Equal(decrypted, original) {
		t.Fatalf("decrypted %s does not match original %s", string(decrypted), string(original))
	}
}

func TestDecryptionTampering(t *testing.T) {
	key := bytes.Repeat([]byte("k"), 32)
	enc, _ := NewEncryptor(key)

	ciphertext, _ := enc.Encrypt([]byte("sensitive payload"))
	// Tamper with ciphertext
	ciphertext[len(ciphertext)-1] ^= 0xFF

	_, err := enc.Decrypt(ciphertext)
	if err == nil {
		t.Fatalf("expected error on tampered ciphertext, got nil")
	}
}

func TestLoadOrGenerateKey(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "master.key")

	// First run should generate file
	key1, err := LoadOrGenerateKey(keyPath)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	if len(key1) != 32 {
		t.Fatalf("expected 32 byte key, got %d", len(key1))
	}

	// Second run should read identical file
	key2, err := LoadOrGenerateKey(keyPath)
	if err != nil {
		t.Fatalf("failed to read existing key: %v", err)
	}
	if !bytes.Equal(key1, key2) {
		t.Fatalf("keys do not match between runs")
	}

	// Check file permissions
	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("failed to stat keyfile: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Fatalf("expected 0600 permissions, got %04o", perm)
	}
}
