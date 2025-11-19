package sender

import (
	"context"
	"encoding/json"
	"fmt"
	"net/smtp"

	"cnb.cool/mliev/push/message-push/app/constants"
)

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
