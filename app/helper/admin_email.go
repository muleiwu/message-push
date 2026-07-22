package helper

import (
	"fmt"
	"net/mail"
	"strings"
)

// NormalizeAdminEmail 规范化并校验管理员邮箱。
//
// 空值用于兼容尚未补录邮箱的历史管理员；需要邮箱必填的调用方应在
// 规范化后单独检查空字符串。
func NormalizeAdminEmail(value string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return "", nil
	}
	if len(normalized) > 255 {
		return "", fmt.Errorf("邮箱长度不能超过 255 字节")
	}
	for _, char := range normalized {
		if char > 127 {
			return "", fmt.Errorf("邮箱仅支持 ASCII 字符")
		}
	}

	address, err := mail.ParseAddress(normalized)
	if err != nil || address.Address != normalized {
		return "", fmt.Errorf("邮箱格式无效")
	}

	local, domain, found := strings.Cut(normalized, "@")
	if !found || local == "" || domain == "" || !strings.Contains(domain, ".") {
		return "", fmt.Errorf("邮箱格式无效")
	}

	return normalized, nil
}
