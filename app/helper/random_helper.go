package helper

import (
	"crypto/rand"
	"encoding/hex"
)

// GenerateRandomKey 生成指定长度的随机十六进制字符串（length 为最终字符数）。
func GenerateRandomKey(length int) (string, error) {
	bytes := make([]byte, length/2)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
