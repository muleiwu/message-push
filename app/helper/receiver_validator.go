package helper

import (
	"fmt"
	"regexp"
	"strings"

	"cnb.cool/mliev/push/message-push/app/constants"

	"github.com/nyaruka/phonenumbers"
)

// ReceiverValidator 接收者校验器接口
type ReceiverValidator interface {
	// Validate 校验单个接收者
	Validate(receiver string) error
	// ValidateBatch 批量校验接收者
	ValidateBatch(receivers []string) error
}

// SMSReceiverValidator 短信接收者校验器
type SMSReceiverValidator struct{}

// EmailReceiverValidator 邮件接收者校验器
type EmailReceiverValidator struct{}

// DefaultReceiverValidator 默认接收者校验器（不做特殊校验）
type DefaultReceiverValidator struct{}

// 批量发送手机号最大数量
const MaxBatchSMSReceivers = 200

// 预编译正则表达式
var (
	// 中国大陆手机号正则（纯11位）：1开头，第二位为3-9
	chinaPhoneRegex = regexp.MustCompile(`^1[3-9]\d{9}$`)
	// E.164 格式正则：+[国家码][手机号]
	e164Regex = regexp.MustCompile(`^\+\d{1,3}\d{6,14}$`)
	// 00前缀格式：00[国家码][手机号]
	doubleZeroPrefixRegex = regexp.MustCompile(`^00\d{1,3}\d{6,14}$`)
	// 无+号的国家码格式：[国家码][手机号]（如 8618501234444）
	noPlusPrefixRegex = regexp.MustCompile(`^\d{1,3}\d{11,14}$`)
	// 邮箱正则
	emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
)

// PhoneInfo 手机号解析结果
type PhoneInfo struct {
	Region         string // 地区码（如 "CN"）
	CountryCode    string // 国家区号（如 "86"）
	NationalNumber string // 国内号码（裸号码，如 "13800138000"）
	E164           string // E.164 国际电话号码（如 "+8613800138000"）
	Valid          bool   // 是否为有效手机号
}

// ParsePhoneNumber 解析手机号，返回地区、裸号码与 E.164 格式。
// 以中国大陆（CN）为默认地区，兼容纯 11 位、+86、0086、86 前缀及带国家码的国际号码。
// 解析失败或非有效号码时返回 Valid=false（其余字段为空）。
func ParsePhoneNumber(receiver string) PhoneInfo {
	receiver = strings.TrimSpace(receiver)
	if receiver == "" {
		return PhoneInfo{}
	}

	num, err := phonenumbers.Parse(receiver, "CN")
	if err != nil || !phonenumbers.IsValidNumber(num) {
		return PhoneInfo{}
	}

	return PhoneInfo{
		Region:         phonenumbers.GetRegionCodeForNumber(num),
		CountryCode:    fmt.Sprintf("%d", num.GetCountryCode()),
		NationalNumber: fmt.Sprintf("%d", num.GetNationalNumber()),
		E164:           phonenumbers.Format(num, phonenumbers.E164),
		Valid:          true,
	}
}

// GetReceiverValidator 根据消息类型获取对应的校验器
func GetReceiverValidator(messageType string) ReceiverValidator {
	switch messageType {
	case constants.MessageTypeSMS:
		return &SMSReceiverValidator{}
	case constants.MessageTypeEmail:
		return &EmailReceiverValidator{}
	default:
		return &DefaultReceiverValidator{}
	}
}

// Validate 校验手机号格式
// 支持 E.164 标准格式：
// - +8618501234444 (E.164 标准格式)
// - 18501234444 (纯11位手机号，默认+86)
// - 008618501234444 (00前缀格式)
// - 8618501234444 (不带+的国家码格式)
func (v *SMSReceiverValidator) Validate(receiver string) error {
	receiver = strings.TrimSpace(receiver)
	if receiver == "" {
		return fmt.Errorf("phone number cannot be empty")
	}
	if !isValidPhoneNumber(receiver) {
		return fmt.Errorf("invalid phone number format: %s", receiver)
	}
	return nil
}

// ValidateBatch 批量校验手机号
// - 最多支持200个手机号
// - 要求全为境内手机号或全为境外手机号
func (v *SMSReceiverValidator) ValidateBatch(receivers []string) error {
	if len(receivers) == 0 {
		return fmt.Errorf("receivers cannot be empty")
	}
	if len(receivers) > MaxBatchSMSReceivers {
		return fmt.Errorf("too many receivers: %d, max allowed: %d", len(receivers), MaxBatchSMSReceivers)
	}

	var invalidPhones []string
	var domesticCount, internationalCount int

	for _, receiver := range receivers {
		receiver = strings.TrimSpace(receiver)
		if err := v.Validate(receiver); err != nil {
			invalidPhones = append(invalidPhones, receiver)
			continue
		}

		// 统计境内/境外号码数量
		if isDomesticPhone(receiver) {
			domesticCount++
		} else {
			internationalCount++
		}
	}

	if len(invalidPhones) > 0 {
		return fmt.Errorf("invalid phone numbers: %v", invalidPhones)
	}

	// 检查是否混合了境内和境外号码
	if domesticCount > 0 && internationalCount > 0 {
		return fmt.Errorf("cannot mix domestic and international phone numbers in one batch request")
	}

	return nil
}

// isValidPhoneNumber 校验手机号格式是否有效
func isValidPhoneNumber(phone string) bool {
	// 纯11位中国大陆手机号
	if chinaPhoneRegex.MatchString(phone) {
		return true
	}
	// E.164 格式：+[国家码][手机号]
	if e164Regex.MatchString(phone) {
		return true
	}
	// 00前缀格式：00[国家码][手机号]
	if doubleZeroPrefixRegex.MatchString(phone) {
		return true
	}
	// 无+号的国家码格式（长度大于11位，如 8618501234444）
	if len(phone) > 11 && noPlusPrefixRegex.MatchString(phone) {
		return true
	}
	return false
}

// isDomesticPhone 判断是否为中国大陆手机号
func isDomesticPhone(phone string) bool {
	// 纯11位中国大陆手机号
	if chinaPhoneRegex.MatchString(phone) {
		return true
	}
	// +86 开头
	if strings.HasPrefix(phone, "+86") {
		return true
	}
	// 0086 开头
	if strings.HasPrefix(phone, "0086") {
		return true
	}
	// 86 开头且后面是11位手机号
	if strings.HasPrefix(phone, "86") && len(phone) == 13 {
		suffix := phone[2:]
		return chinaPhoneRegex.MatchString(suffix)
	}
	return false
}

// Validate 校验邮箱格式
func (v *EmailReceiverValidator) Validate(receiver string) error {
	receiver = strings.TrimSpace(receiver)
	if receiver == "" {
		return fmt.Errorf("email address cannot be empty")
	}
	if !emailRegex.MatchString(receiver) {
		return fmt.Errorf("invalid email format: %s", receiver)
	}
	return nil
}

// ValidateBatch 批量校验邮箱
func (v *EmailReceiverValidator) ValidateBatch(receivers []string) error {
	if len(receivers) == 0 {
		return fmt.Errorf("receivers cannot be empty")
	}
	var invalidEmails []string
	for _, receiver := range receivers {
		if err := v.Validate(receiver); err != nil {
			invalidEmails = append(invalidEmails, receiver)
		}
	}
	if len(invalidEmails) > 0 {
		return fmt.Errorf("invalid email addresses: %v", invalidEmails)
	}
	return nil
}

// Validate 默认校验（只检查非空）
func (v *DefaultReceiverValidator) Validate(receiver string) error {
	receiver = strings.TrimSpace(receiver)
	if receiver == "" {
		return fmt.Errorf("receiver cannot be empty")
	}
	return nil
}

// ValidateBatch 默认批量校验（只检查非空）
func (v *DefaultReceiverValidator) ValidateBatch(receivers []string) error {
	if len(receivers) == 0 {
		return fmt.Errorf("receivers cannot be empty")
	}
	for i, receiver := range receivers {
		if strings.TrimSpace(receiver) == "" {
			return fmt.Errorf("receiver at index %d cannot be empty", i)
		}
	}
	return nil
}
