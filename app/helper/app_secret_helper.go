package helper

import (
	"fmt"
	"os"
)

var (
	cryptoHelper *CryptoHelper
)

// InitCryptoHelper 初始化加密助手
func InitCryptoHelper() error {
	// 从环境变量获取加密密钥，如果没有则使用默认值（开发环境）
	encryptionKey := os.Getenv("APP_ENCRYPTION_KEY")
	if encryptionKey == "" {
		// 默认密钥（仅用于开发环境，生产环境必须设置环境变量）
		encryptionKey = "default-32-byte-encryption-key!!"
	}

	// 确保密钥长度正确
	if len(encryptionKey) != 32 {
		return fmt.Errorf("encryption key must be 32 bytes, got %d", len(encryptionKey))
	}

	var err error
	cryptoHelper, err = NewCryptoHelper(encryptionKey)
	if err != nil {
		return err
	}

	return nil
}

// EncryptAppSecret 加密AppSecret
func EncryptAppSecret(appSecret string) (string, error) {
	if cryptoHelper == nil {
		if err := InitCryptoHelper(); err != nil {
			return "", err
		}
	}
	return cryptoHelper.Encrypt(appSecret)
}

// DecryptAppSecret 解密AppSecret
func DecryptAppSecret(encryptedSecret string) (string, error) {
	if cryptoHelper == nil {
		if err := InitCryptoHelper(); err != nil {
			return "", err
		}
	}
	return cryptoHelper.Decrypt(encryptedSecret)
}
