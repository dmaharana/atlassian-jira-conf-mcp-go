package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

var (
	ErrInvalidKeySize = errors.New("encryption key must be exactly 32 bytes (256-bit)")
	ErrDecryption     = errors.New("ciphertext decryption failed: data corrupted or key mismatch")
)

// Encryptor provides AES-256-GCM encryption and decryption.
type Encryptor struct {
	gcm cipher.AEAD
}

// NewEncryptor creates an Encryptor with a 32-byte key.
func NewEncryptor(key []byte) (*Encryptor, error) {
	if len(key) != 32 {
		return nil, ErrInvalidKeySize
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("init aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("init gcm: %w", err)
	}
	return &Encryptor{gcm: gcm}, nil
}

// Encrypt encrypts plaintext using AES-GCM with a random nonce prepended to the ciphertext.
func (e *Encryptor) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, e.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}
	ciphertext := e.gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt decrypts ciphertext where the first NonceSize bytes are the nonce.
func (e *Encryptor) Decrypt(ciphertext []byte) ([]byte, error) {
	nonceSize := e.gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, ErrDecryption
	}
	nonce, actualCiphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := e.gcm.Open(nil, nonce, actualCiphertext, nil)
	if err != nil {
		return nil, ErrDecryption
	}
	return plaintext, nil
}

// LoadOrGenerateKey resolves the 32-byte master key from ATLASSIAN_MCP_ENCRYPTION_KEY env var
// or reads/generates a master key file at keyFilePath with 0600 permissions.
func LoadOrGenerateKey(keyFilePath string) ([]byte, error) {
	if envHex := os.Getenv("ATLASSIAN_MCP_ENCRYPTION_KEY"); envHex != "" {
		key, err := hex.DecodeString(envHex)
		if err == nil && len(key) == 32 {
			return key, nil
		}
		if len(envHex) == 32 {
			return []byte(envHex), nil
		}
		return nil, fmt.Errorf("ATLASSIAN_MCP_ENCRYPTION_KEY must be a 64-char hex string or 32 raw bytes")
	}

	if keyFilePath == "" {
		keyFilePath = filepath.Join("data", "master.key")
	}

	if data, err := os.ReadFile(keyFilePath); err == nil {
		if len(data) == 32 {
			return data, nil
		}
		key, err := hex.DecodeString(string(data))
		if err == nil && len(key) == 32 {
			return key, nil
		}
	}

	// Generate new key
	if err := os.MkdirAll(filepath.Dir(keyFilePath), 0700); err != nil {
		return nil, fmt.Errorf("create dir for keyfile: %w", err)
	}
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("generate random master key: %w", err)
	}

	if err := os.WriteFile(keyFilePath, key, 0600); err != nil {
		return nil, fmt.Errorf("save master key file: %w", err)
	}
	return key, nil
}
