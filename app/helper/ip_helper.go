package helper

import (
	"fmt"
	"net"
	"strings"
)

// ValidateIPWhitelist 验证IP白名单格式
// 输入：换行分隔的IP/CIDR列表
// 返回：验证后的IP列表（去除空行和空格），错误信息
func ValidateIPWhitelist(whitelist string) (string, error) {
	if whitelist == "" {
		return "", nil
	}

	lines := strings.Split(whitelist, "\n")
	var validIPs []string

	for _, line := range lines {
		// 去除首尾空格
		ip := strings.TrimSpace(line)
		if ip == "" {
			continue
		}

		// 验证IP或CIDR格式
		if err := validateIPOrCIDR(ip); err != nil {
			return "", fmt.Errorf("无效的IP格式 '%s': %w", ip, err)
		}

		validIPs = append(validIPs, ip)
	}

	return strings.Join(validIPs, "\n"), nil
}

// validateIPOrCIDR 验证单个IP或CIDR格式
func validateIPOrCIDR(s string) error {
	// 检查是否是CIDR格式（包含/）
	if strings.Contains(s, "/") {
		_, _, err := net.ParseCIDR(s)
		if err != nil {
			return fmt.Errorf("无效的CIDR格式: %w", err)
		}
		return nil
	}

	// 验证单个IP地址
	ip := net.ParseIP(s)
	if ip == nil {
		return fmt.Errorf("无效的IP地址")
	}

	return nil
}

// IsIPInWhitelist 检查IP是否在白名单中
// whitelist: 换行分隔的IP/CIDR列表
// clientIP: 要检查的客户端IP
func IsIPInWhitelist(whitelist string, clientIP string) bool {
	if whitelist == "" {
		return true // 空白名单表示不限制
	}

	ip := net.ParseIP(clientIP)
	if ip == nil {
		return false
	}

	lines := strings.Split(whitelist, "\n")
	for _, line := range lines {
		entry := strings.TrimSpace(line)
		if entry == "" {
			continue
		}

		// CIDR格式检查
		if strings.Contains(entry, "/") {
			_, network, err := net.ParseCIDR(entry)
			if err != nil {
				continue
			}
			if network.Contains(ip) {
				return true
			}
		} else {
			// 精确IP匹配
			if entry == clientIP {
				return true
			}
		}
	}

	return false
}
