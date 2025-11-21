package sender

import (
	"context"
	"encoding/json"
	"fmt"
	"net/smtp"

	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/registry"
)

func init() {
	// 注册SMTP邮件服务商
	registry.Register(&registry.ProviderMeta{
		Code:        constants.ProviderSMTP,
		Name:        "SMTP邮件",
		Type:        constants.MessageTypeEmail,
		Description: "通用SMTP邮件发送服务，支持各类邮件服务器",
		ConfigFields: []registry.ConfigField{
			{
				Key:         "host",
				Label:       "SMTP服务器",
				Description: "SMTP服务器地址",
				Type:        registry.FieldTypeText,
				Required:    true,
				Example:     "smtp.qq.com",
				Placeholder: "请输入SMTP服务器地址",
			},
			{
				Key:          "port",
				Label:        "端口",
				Description:  "SMTP服务器端口",
				Type:         registry.FieldTypeNumber,
				Required:     true,
				Example:      "587",
				Placeholder:  "请输入端口号",
				DefaultValue: "587",
			},
			{
				Key:         "username",
				Label:       "用户名",
				Description: "SMTP登录用户名",
				Type:        registry.FieldTypeText,
				Required:    true,
				Example:     "your-email@example.com",
				Placeholder: "请输入用户名",
			},
			{
				Key:         "password",
				Label:       "密码",
				Description: "SMTP登录密码或授权码",
				Type:        registry.FieldTypePassword,
				Required:    true,
				Example:     "your-password",
				Placeholder: "请输入密码或授权码",
			},
			{
				Key:         "from",
				Label:       "发件人地址",
				Description: "邮件发件人地址",
				Type:        registry.FieldTypeText,
				Required:    true,
				Example:     "noreply@example.com",
				Placeholder: "请输入发件人地址",
			},
		},
	})
}

// SMTPSender SMTP邮件发送器
type SMTPSender struct {
}

// NewSMTPSender 创建SMTP发送器
func NewSMTPSender() *SMTPSender {
	return &SMTPSender{}
}

// GetType 获取发送器类型
func (s *SMTPSender) GetType() string {
	return constants.MessageTypeEmail
}

// Send 发送邮件
func (s *SMTPSender) Send(ctx context.Context, req *SendRequest) (*SendResponse, error) {
	// 解析服务商配置
	var config struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		Username string `json:"username"`
		Password string `json:"password"`
		From     string `json:"from"`
	}

	if err := json.Unmarshal([]byte(req.Provider.Config), &config); err != nil {
		return nil, fmt.Errorf("invalid provider config: %w", err)
	}

	// 构建邮件内容
	subject := req.Task.Title
	if subject == "" {
		subject = "通知"
	}

	message := fmt.Sprintf("From: %s\r\n", config.From)
	message += fmt.Sprintf("To: %s\r\n", req.Task.Receiver)
	message += fmt.Sprintf("Subject: %s\r\n", subject)
	message += "Content-Type: text/plain; charset=UTF-8\r\n"
	message += "\r\n"
	message += req.Task.Content

	// 发送邮件
	auth := smtp.PlainAuth("", config.Username, config.Password, config.Host)
	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)

	err := smtp.SendMail(
		addr,
		auth,
		config.From,
		[]string{req.Task.Receiver},
		[]byte(message),
	)

	if err != nil {
		return &SendResponse{
			Success:      false,
			ErrorMessage: err.Error(),
		}, nil
	}

	return &SendResponse{
		Success:    true,
		ProviderID: fmt.Sprintf("smtp_%s", req.Task.TaskID),
	}, nil
}
