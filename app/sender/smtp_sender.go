package sender

import (
	"context"
	"encoding/json"
	"fmt"
	"net/smtp"
	"strings"
	"time"

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
		// 能力声明
		SupportsSend:      true,
		SupportsBatchSend: true,
		SupportsCallback:  true,
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
			TaskID:       req.Task.TaskID,
		}, nil
	}

	return &SendResponse{
		Success:    true,
		ProviderID: fmt.Sprintf("smtp_%s", req.Task.TaskID),
		TaskID:     req.Task.TaskID,
	}, nil
}

// ==================== BatchSender 接口实现 ====================

// SupportsBatchSend 是否支持批量发送
func (s *SMTPSender) SupportsBatchSend() bool {
	return true
}

// BatchSend 批量发送邮件（通过多收件人或循环发送）
func (s *SMTPSender) BatchSend(ctx context.Context, req *BatchSendRequest) (*BatchSendResponse, error) {
	if len(req.Tasks) == 0 {
		return &BatchSendResponse{Results: []*SendResponse{}}, nil
	}

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

	auth := smtp.PlainAuth("", config.Username, config.Password, config.Host)
	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)

	results := make([]*SendResponse, len(req.Tasks))

	// 逐个发送邮件（SMTP 批量发送时每个收件人内容可能不同）
	for i, task := range req.Tasks {
		subject := task.Title
		if subject == "" {
			subject = "通知"
		}

		message := fmt.Sprintf("From: %s\r\n", config.From)
		message += fmt.Sprintf("To: %s\r\n", task.Receiver)
		message += fmt.Sprintf("Subject: %s\r\n", subject)
		message += "Content-Type: text/plain; charset=UTF-8\r\n"
		message += "\r\n"
		message += task.Content

		err := smtp.SendMail(
			addr,
			auth,
			config.From,
			[]string{task.Receiver},
			[]byte(message),
		)

		if err != nil {
			results[i] = &SendResponse{
				Success:      false,
				ErrorMessage: err.Error(),
				TaskID:       task.TaskID,
			}
		} else {
			results[i] = &SendResponse{
				Success:    true,
				ProviderID: fmt.Sprintf("smtp_%s", task.TaskID),
				TaskID:     task.TaskID,
			}
		}
	}

	return &BatchSendResponse{Results: results}, nil
}

// ==================== CallbackHandler 接口实现 ====================

// GetProviderCode 获取服务商代码
func (s *SMTPSender) GetProviderCode() string {
	return constants.ProviderSMTP
}

// SupportsCallback 是否支持回调
func (s *SMTPSender) SupportsCallback() bool {
	// SMTP 通常通过退信（bounce）来通知发送失败
	// 这里提供基本的退信解析支持
	return true
}

// HandleCallback 处理 SMTP 退信回调
// 退信格式通常是邮件内容，包含原始邮件信息和退信原因
// 这里提供一个简化的实现，实际使用中可能需要更复杂的解析
func (s *SMTPSender) HandleCallback(ctx context.Context, req *CallbackRequest) ([]*CallbackResult, error) {
	// 尝试解析 JSON 格式的退信通知（如果使用邮件服务提供商的 webhook）
	var bounceReport struct {
		MessageID    string `json:"message_id"`
		Email        string `json:"email"`
		Status       string `json:"status"` // bounced, delivered, complained
		BounceType   string `json:"bounce_type"`
		ErrorCode    string `json:"error_code"`
		ErrorMessage string `json:"error_message"`
		Timestamp    string `json:"timestamp"`
	}

	if err := json.Unmarshal(req.RawBody, &bounceReport); err != nil {
		// 如果不是 JSON 格式，尝试从原始内容中提取信息
		bodyStr := string(req.RawBody)
		if strings.Contains(bodyStr, "Delivery Status Notification") ||
			strings.Contains(bodyStr, "Undelivered Mail") {
			return []*CallbackResult{{
				Status:       "failed",
				ErrorMessage: "Email delivery failed (bounce detected)",
				ReportTime:   time.Now(),
			}}, nil
		}
		return nil, fmt.Errorf("invalid callback data: %w", err)
	}

	status := "delivered"
	if bounceReport.Status == "bounced" || bounceReport.Status == "failed" {
		status = "failed"
	} else if bounceReport.Status == "complained" {
		status = "rejected"
	}

	reportTime, _ := time.Parse(time.RFC3339, bounceReport.Timestamp)
	if reportTime.IsZero() {
		reportTime = time.Now()
	}

	return []*CallbackResult{{
		ProviderID:   bounceReport.MessageID,
		Status:       status,
		ErrorCode:    bounceReport.ErrorCode,
		ErrorMessage: bounceReport.ErrorMessage,
		ReportTime:   reportTime,
	}}, nil
}
