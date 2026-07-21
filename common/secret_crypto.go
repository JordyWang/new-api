package common

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

const encryptedSecretVersion = "v1"

// EncryptSecret encrypts a secret with AES-GCM using a key derived from
// CryptoSecret. The context is authenticated as additional data so ciphertext
// cannot be moved between unrelated secret fields.
func EncryptSecret(plaintext string, context string) (string, error) {
	if strings.TrimSpace(CryptoSecret) == "" {
		return "", errors.New("crypto secret is empty")
	}

	key := sha256.Sum256([]byte(CryptoSecret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	sealed := gcm.Seal(nil, nonce, []byte(plaintext), []byte(context))
	payload := append(nonce, sealed...)
	return encryptedSecretVersion + "." + base64.RawURLEncoding.EncodeToString(payload), nil
}

// DecryptSecret decrypts a value written by EncryptSecret.
func DecryptSecret(encrypted string, context string) (string, error) {
	version, payloadText, ok := strings.Cut(strings.TrimSpace(encrypted), ".")
	if !ok || version != encryptedSecretVersion {
		return "", errors.New("unsupported encrypted secret format")
	}
	if strings.TrimSpace(CryptoSecret) == "" {
		return "", errors.New("crypto secret is empty")
	}

	payload, err := base64.RawURLEncoding.DecodeString(payloadText)
	if err != nil {
		return "", fmt.Errorf("decode encrypted secret: %w", err)
	}

	key := sha256.Sum256([]byte(CryptoSecret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(payload) < gcm.NonceSize() {
		return "", errors.New("encrypted secret payload is too short")
	}

	nonce := payload[:gcm.NonceSize()]
	ciphertext := payload[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, []byte(context))
	if err != nil {
		return "", errors.New("decrypt encrypted secret failed")
	}
	return string(plaintext), nil
}
