package helper

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"
)

// SignatureHelper 签名助手
type SignatureHelper struct {
}

// NewSignatureHelper 创建签名助手
func NewSignatureHelper() *SignatureHelper {
	return &SignatureHelper{}
}

// GenerateSignature 生成签名
// Signature = Base64(HMAC-SHA256(SignContent, AppSecret))
// SignContent = AppId + "|" + Timestamp + "|" + MD5(RequestBody)
func (h *SignatureHelper) GenerateSignature(appID, appSecret string, timestamp int64, body []byte) string {
	bodyMd5 := h.md5Hash(body)
	signContent := fmt.Sprintf("%s|%d|%s", appID, timestamp, bodyMd5)
	return h.hmacSha256(signContent, appSecret)
}

// VerifySignature 验证签名
func (h *SignatureHelper) VerifySignature(appID, appSecret, signature string, timestamp int64, body []byte) bool {
	// 检查时间戳（防重放攻击，允许 ±5 分钟）
	now := time.Now().Unix()
	if abs(now-timestamp) > 300 {
		return false
	}

	// 生成期望的签名
	expectedSignature := h.GenerateSignature(appID, appSecret, timestamp, body)

	// 比对签名
	return signature == expectedSignature
}

// md5Hash 计算 MD5
func (h *SignatureHelper) md5Hash(data []byte) string {
	hash := md5.Sum(data)
	return fmt.Sprintf("%x", hash)
}

// hmacSha256 计算 HMAC-SHA256
func (h *SignatureHelper) hmacSha256(data, key string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(data))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func abs(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}
