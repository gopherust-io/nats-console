package crypto

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

const encryptedPrefix = "enc:"

var (
	ErrMissingKey   = errors.New("encryption key is required")
	ErrInvalidKey   = errors.New("encryption key must be at least 16 characters")
	ErrDecrypt      = errors.New("decrypt credential")
	ErrNotEncrypted = errors.New("value is not encrypted")
)

type Encryptor struct {
	gcm cipher.AEAD
}

func New(key string) (*Encryptor, error) {
	if key == "" {
		return nil, ErrMissingKey
	}
	if len(key) < 16 {
		return nil, ErrInvalidKey
	}

	sum := sha256.Sum256([]byte(key))
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return nil, fmt.Errorf("aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}
	return &Encryptor{gcm: gcm}, nil
}

func IsEncrypted(value string) bool {
	return strings.HasPrefix(value, encryptedPrefix)
}

func (e *Encryptor) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	if IsEncrypted(plaintext) {
		return plaintext, nil
	}

	nonce := make([]byte, e.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("nonce: %w", err)
	}

	ciphertext := e.gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return encryptedPrefix + base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (e *Encryptor) Decrypt(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if !IsEncrypted(value) {
		return value, nil
	}

	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, encryptedPrefix))
	if err != nil {
		return "", fmt.Errorf("%w: base64: %w", ErrDecrypt, err)
	}

	nonceSize := e.gcm.NonceSize()
	if len(raw) < nonceSize {
		return "", fmt.Errorf("%w: ciphertext too short", ErrDecrypt)
	}

	nonce, ciphertext := raw[:nonceSize], raw[nonceSize:]
	plaintext, err := e.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrDecrypt, err)
	}
	return string(plaintext), nil
}

func (e *Encryptor) EncryptIfNeeded(value string) (string, error) {
	if value == "" || IsEncrypted(value) {
		return value, nil
	}
	return e.Encrypt(value)
}
