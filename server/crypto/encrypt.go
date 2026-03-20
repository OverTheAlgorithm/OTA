package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
)

// Encrypt encrypts plaintext using AES-256-GCM with the provided 32-byte key.
// The returned ciphertext is hex-encoded and includes the nonce prepended.
// Returns plaintext unchanged if key is empty (backward-compatible mode).
func Encrypt(key, plaintext string) (string, error) {
	if key == "" {
		return plaintext, nil
	}

	keyBytes, err := hex.DecodeString(key)
	if err != nil {
		return "", fmt.Errorf("invalid encryption key: %w", err)
	}
	if len(keyBytes) != 32 {
		return "", errors.New("encryption key must be 32 bytes (64 hex chars)")
	}

	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return hex.EncodeToString(ciphertext), nil
}

// Decrypt decrypts a hex-encoded AES-256-GCM ciphertext produced by Encrypt.
// Returns ciphertext unchanged if key is empty (backward-compatible mode).
func Decrypt(key, ciphertext string) (string, error) {
	if key == "" {
		return ciphertext, nil
	}

	keyBytes, err := hex.DecodeString(key)
	if err != nil {
		return "", fmt.Errorf("invalid encryption key: %w", err)
	}
	if len(keyBytes) != 32 {
		return "", errors.New("encryption key must be 32 bytes (64 hex chars)")
	}

	data, err := hex.DecodeString(ciphertext)
	if err != nil {
		// Not hex-encoded — assume plaintext (migration path for existing records)
		return ciphertext, nil
	}

	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create gcm: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		// Decryption failed — might be a legacy plaintext value, return as-is
		return ciphertext, nil
	}

	return string(plaintext), nil
}
