package registry

// ConfigField 配置字段定义
type ConfigField struct {
	Key            string `json:"key"`             // 参数key
	Label          string `json:"label"`           // 显示名称
	Description    string `json:"description"`     // 说明
	Type           string `json:"type"`            // 类型: text, password, number, url, textarea
	Required       bool   `json:"required"`        // 是否必填
	Example        string `json:"example"`         // 示例值
	Placeholder    string `json:"placeholder"`     // 占位符
	ValidationRule string `json:"validation_rule"` // 验证规则（如：min:6, max:100, url等）
	HelpLink       string `json:"help_link"`       // 帮助文档链接
	DefaultValue   string `json:"default_value"`   // 默认值
}

// ConfigFieldType 配置字段类型常量
const (
	FieldTypeText     = "text"     // 普通文本
	FieldTypePassword = "password" // 密码
	FieldTypeNumber   = "number"   // 数字
	FieldTypeURL      = "url"      // URL地址
	FieldTypeTextarea = "textarea" // 多行文本
)
