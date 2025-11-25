package helper

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
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
// Signature = HMAC-SHA256(secret, method + path + sorted(params) + body + timestamp + nonce)
func (h *SignatureHelper) GenerateSignature(secret, method, path string, params map[string]interface{}, timestamp int64, nonce string) string {
	sortedParams := h.sortParams(params)
	signContent := fmt.Sprintf("%s%s%s%d%s", method, path, sortedParams, timestamp, nonce)
	return h.hmacSha256(signContent, secret)
}

// VerifySignature 验证签名
// signature = HMAC(secret, method + path + sorted(params) + body + timestamp + nonce)
func (h *SignatureHelper) VerifySignature(secret, method, path string, params map[string]interface{}, timestamp int64, nonce, signature string) bool {
	// 检查时间戳（防重放攻击，允许 ±5 分钟）
	now := time.Now().Unix()
	if abs(now-timestamp) > 300 {
		return false
	}

	// 生成期望的签名
	expectedSignature := h.GenerateSignature(secret, method, path, params, timestamp, nonce)

	// 比对签名
	return signature == expectedSignature
}

// sortParams 对参数按 key 排序并序列化
func (h *SignatureHelper) sortParams(params map[string]interface{}) string {
	if len(params) == 0 {
		return ""
	}

	// 获取所有 key 并排序
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// 按排序后的 key 顺序序列化
	sortedMap := make(map[string]interface{})
	for _, k := range keys {
		sortedMap[k] = params[k]
	}

	result, _ := json.Marshal(sortedMap)
	return string(result)
}

// hmacSha256 计算 HMAC-SHA256
func (h *SignatureHelper) hmacSha256(data, key string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}

func abs(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}
