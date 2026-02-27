package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
)

// Encryptor отвечает за шифрование и дешифрование чувствительных данных
type Encryptor struct {
	key []byte
}

// NewEncryptor создает новый Encryptor с заданным ключом
// Ключ должен быть 32 байта для AES-256
func NewEncryptor(key string) *Encryptor {
	// Если ключ слишком короткий, хешируем его до 32 байт
	hash := sha256.Sum256([]byte(key))
	return &Encryptor{
		key: hash[:],
	}
}

// Encrypt шифрует строку с использованием AES-GCM
func (e *Encryptor) Encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt расшифровывает строку
func (e *Encryptor) Decrypt(encrypted string) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt: %w", err)
	}

	return string(plaintext), nil
}

// MustEncrypt шифрует и паникует при ошибке (для использования в тестах)
func (e *Encryptor) MustEncrypt(plaintext string) string {
	encrypted, err := e.Encrypt(plaintext)
	if err != nil {
		panic(err)
	}
	return encrypted
}
