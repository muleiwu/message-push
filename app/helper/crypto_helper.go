package helper

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

// CryptoHelper 加密助手
type CryptoHelper struct {
	key []byte
}

// NewCryptoHelper 创建加密助手
// encryptionKey 必须是 16、24 或 32 字节（AES-128、AES-192、AES-256）
func NewCryptoHelper(encryptionKey string) (*CryptoHelper, error) {
	key := []byte(encryptionKey)

	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		return nil, fmt.Errorf("invalid key length: must be 16, 24, or 32 bytes")
	}

	return &CryptoHelper{
		key: key,
	}, nil
}

// Encrypt 加密数据
func (h *CryptoHelper) Encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(h.key)
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

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt 解密数据
func (h *CryptoHelper) Decrypt(ciphertext string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(h.key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce := data[:nonceSize]
	ciphertextBytes := data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}
