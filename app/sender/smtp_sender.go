package sender

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
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
			{
				Key:          "encryption",
				Label:        "加密方式",
				Description:  "邮件传输加密方式",
				Type:         registry.FieldTypeSelect,
				Required:     false,
				DefaultValue: "starttls",
				Options: []registry.FieldOption{
					{Value: "none", Label: "无加密"},
					{Value: "starttls", Label: "STARTTLS (端口587)"},
					{Value: "ssl", Label: "SSL/TLS (端口465)"},
				},
			},
		},
		// 能力声明
		SupportsSend:      true,
		SupportsBatchSend: true,
		SupportsCallback:  true,
		// 扩展信息
		Website:    "",
		Icon:       "",
		DocsUrl:    "",
		ConsoleUrl: "",
		PricingUrl: "",
		SortOrder:  100,
		Tags:       []string{"通用", "邮件"},
		Regions:    []string{"全球"},
		Deprecated: false,
	})
}

// SMTPSender SMTP邮件发送器
type SMTPSender struct {
}

// smtpConfig SMTP配置
type smtpConfig struct {
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	From       string `json:"from"`
	Encryption string `json:"encryption"`
}

// NewSMTPSender 创建SMTP发送器
func NewSMTPSender() *SMTPSender {
	return &SMTPSender{}
}

// getEmailContentType 根据供应商模板的内容类型获取邮件 MIME 类型
func getEmailContentType(req *SendRequest) string {
	// 默认为纯文本
	contentType := "text/plain; charset=UTF-8"

	// 从 ChannelTemplateBinding -> ProviderTemplate 获取内容类型
	if req.ChannelTemplateBinding != nil &&
		req.ChannelTemplateBinding.ProviderTemplate != nil {
		switch req.ChannelTemplateBinding.ProviderTemplate.ContentType {
		case "html":
			contentType = "text/html; charset=UTF-8"
		case "markdown":
			// Markdown 暂时作为纯文本处理，后续可考虑转换为 HTML
			contentType = "text/plain; charset=UTF-8"
		default:
			contentType = "text/plain; charset=UTF-8"
		}
	}

	return contentType
}

// GetProviderCode 获取服务商代码
func (s *SMTPSender) GetProviderCode() string {
	return constants.ProviderSMTP
}

// sendMail 根据加密方式发送邮件
func (s *SMTPSender) sendMail(config *smtpConfig, to []string, msg []byte) error {
	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)
	auth := smtp.PlainAuth("", config.Username, config.Password, config.Host)

	switch config.Encryption {
	case "ssl":
		// 隐式 SSL/TLS (端口465): 直接建立加密连接
		return s.sendMailWithSSL(config, addr, auth, to, msg)
	case "none":
		// 无加密: 直接建立普通连接
		return s.sendMailWithoutEncryption(config, addr, auth, to, msg)
	default:
		// STARTTLS (端口587): 使用标准库 smtp.SendMail（已内置 STARTTLS 支持）
		return smtp.SendMail(addr, auth, config.From, to, msg)
	}
}

// sendMailWithSSL 使用隐式SSL发送邮件（端口465）
func (s *SMTPSender) sendMailWithSSL(config *smtpConfig, addr string, auth smtp.Auth, to []string, msg []byte) error {
	tlsConfig := &tls.Config{
		ServerName: config.Host,
	}

	// 建立TLS连接
	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("failed to connect with SSL: %w", err)
	}
	defer conn.Close()

	// 创建SMTP客户端
	client, err := smtp.NewClient(conn, config.Host)
	if err != nil {
		return fmt.Errorf("failed to create SMTP client: %w", err)
	}
	defer client.Close()

	// 认证
	if err = client.Auth(auth); err != nil {
		return fmt.Errorf("SMTP auth failed: %w", err)
	}

	// 设置发件人
	if err = client.Mail(config.From); err != nil {
		return fmt.Errorf("failed to set sender: %w", err)
	}

	// 设置收件人
	for _, recipient := range to {
		if err = client.Rcpt(recipient); err != nil {
			return fmt.Errorf("failed to set recipient %s: %w", recipient, err)
		}
	}

	// 写入邮件内容
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to get data writer: %w", err)
	}

	_, err = w.Write(msg)
	if err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	err = w.Close()
	if err != nil {
		return fmt.Errorf("failed to close data writer: %w", err)
	}

	return client.Quit()
}

// sendMailWithoutEncryption 不使用加密发送邮件
func (s *SMTPSender) sendMailWithoutEncryption(config *smtpConfig, addr string, auth smtp.Auth, to []string, msg []byte) error {
	// 建立普通TCP连接
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close()

	// 创建SMTP客户端
	client, err := smtp.NewClient(conn, config.Host)
	if err != nil {
		return fmt.Errorf("failed to create SMTP client: %w", err)
	}
	defer client.Close()

	// 认证（如果服务器支持）
	if auth != nil {
		if err = client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP auth failed: %w", err)
		}
	}

	// 设置发件人
	if err = client.Mail(config.From); err != nil {
		return fmt.Errorf("failed to set sender: %w", err)
	}

	// 设置收件人
	for _, recipient := range to {
		if err = client.Rcpt(recipient); err != nil {
			return fmt.Errorf("failed to set recipient %s: %w", recipient, err)
		}
	}

	// 写入邮件内容
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to get data writer: %w", err)
	}

	_, err = w.Write(msg)
	if err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	err = w.Close()
	if err != nil {
		return fmt.Errorf("failed to close data writer: %w", err)
	}

	return client.Quit()
}

// Send 发送邮件
func (s *SMTPSender) Send(ctx context.Context, req *SendRequest) (*SendResponse, error) {
	// 解析服务商配置
	var config smtpConfig
	if err := json.Unmarshal([]byte(req.ProviderAccount.Config), &config); err != nil {
		return nil, fmt.Errorf("invalid provider config: %w", err)
	}

	// 构建邮件内容
	subject := req.Task.Signature
	if subject == "" {
		subject = "通知"
	}

	// 获取邮件内容类型
	contentType := getEmailContentType(req)

	message := fmt.Sprintf("From: %s\r\n", config.From)
	message += fmt.Sprintf("To: %s\r\n", req.Task.Receiver)
	message += fmt.Sprintf("Subject: %s\r\n", subject)
	message += fmt.Sprintf("Content-Type: %s\r\n", contentType)
	message += "\r\n"
	message += req.Task.Content

	// 构建请求数据用于调试（不包含密码）
	requestData, _ := json.Marshal(map[string]interface{}{
		"host":        config.Host,
		"port":        config.Port,
		"from":        config.From,
		"to":          req.Task.Receiver,
		"subject":     subject,
		"encryption":  config.Encryption,
		"contentType": contentType,
	})

	// 发送邮件
	err := s.sendMail(&config, []string{req.Task.Receiver}, []byte(message))

	if err != nil {
		return &SendResponse{
			Success:      false,
			ErrorMessage: err.Error(),
			TaskID:       req.Task.TaskID,
			RequestData:  string(requestData),
			ResponseData: "{}",
		}, nil
	}

	// 构建响应数据
	responseData, _ := json.Marshal(map[string]interface{}{
		"status":  "sent",
		"message": "Email sent successfully",
	})

	return &SendResponse{
		Success:      true,
		ProviderID:   fmt.Sprintf("smtp_%s", req.Task.TaskID),
		TaskID:       req.Task.TaskID,
		Status:       constants.TaskStatusSuccess, // 邮件发送成功即完成
		RequestData:  string(requestData),
		ResponseData: string(responseData),
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
	var config smtpConfig
	if err := json.Unmarshal([]byte(req.ProviderAccount.Config), &config); err != nil {
		return nil, fmt.Errorf("invalid provider config: %w", err)
	}

	results := make([]*SendResponse, len(req.Tasks))

	// 获取邮件内容类型
	contentType := "text/plain; charset=UTF-8"
	if req.ChannelTemplateBinding != nil &&
		req.ChannelTemplateBinding.ProviderTemplate != nil {
		switch req.ChannelTemplateBinding.ProviderTemplate.ContentType {
		case "html":
			contentType = "text/html; charset=UTF-8"
		case "markdown":
			contentType = "text/plain; charset=UTF-8"
		}
	}

	// 逐个发送邮件（SMTP 批量发送时每个收件人内容可能不同）
	for i, task := range req.Tasks {
		subject := task.Signature
		if subject == "" {
			subject = "通知"
		}

		message := fmt.Sprintf("From: %s\r\n", config.From)
		message += fmt.Sprintf("To: %s\r\n", task.Receiver)
		message += fmt.Sprintf("Subject: %s\r\n", subject)
		message += fmt.Sprintf("Content-Type: %s\r\n", contentType)
		message += "\r\n"
		message += task.Content

		// 构建请求数据用于调试（不包含密码）
		requestData, _ := json.Marshal(map[string]interface{}{
			"host":        config.Host,
			"port":        config.Port,
			"from":        config.From,
			"to":          task.Receiver,
			"subject":     subject,
			"encryption":  config.Encryption,
			"contentType": contentType,
		})

		err := s.sendMail(&config, []string{task.Receiver}, []byte(message))

		if err != nil {
			results[i] = &SendResponse{
				Success:      false,
				ErrorMessage: err.Error(),
				TaskID:       task.TaskID,
				RequestData:  string(requestData),
				ResponseData: "{}",
			}
		} else {
			// 构建响应数据
			responseData, _ := json.Marshal(map[string]interface{}{
				"status":  "sent",
				"message": "Email sent successfully",
			})
			results[i] = &SendResponse{
				Success:      true,
				ProviderID:   fmt.Sprintf("smtp_%s", task.TaskID),
				TaskID:       task.TaskID,
				Status:       constants.TaskStatusSuccess, // 邮件发送成功即完成
				RequestData:  string(requestData),
				ResponseData: string(responseData),
			}
		}
	}

	return &BatchSendResponse{Results: results}, nil
}

// ==================== CallbackHandler 接口实现 ====================

// SupportsCallback 是否支持回调
func (s *SMTPSender) SupportsCallback() bool {
	// SMTP 通常通过退信（bounce）来通知发送失败
	// 这里提供基本的退信解析支持
	return true
}

// HandleCallback 处理 SMTP 退信回调
// 退信格式通常是邮件内容，包含原始邮件信息和退信原因
// 这里提供一个简化的实现，实际使用中可能需要更复杂的解析
func (s *SMTPSender) HandleCallback(ctx context.Context, req *CallbackRequest) (CallbackResponse, []*CallbackResult, error) {
	// 默认响应
	resp := CallbackResponse{
		StatusCode: 200,
		Body:       `{"status":"ok"}`,
	}

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
			return resp, []*CallbackResult{{
				Status:       constants.CallbackStatusFailed,
				ErrorMessage: "Email delivery failed (bounce detected)",
				ReportTime:   time.Now(),
			}}, nil
		}
		// 即使解析失败也返回成功响应，避免服务商重复推送
		return resp, nil, fmt.Errorf("invalid callback data: %w", err)
	}

	status := constants.CallbackStatusDelivered
	if bounceReport.Status == "bounced" || bounceReport.Status == "failed" {
		status = constants.CallbackStatusFailed
	} else if bounceReport.Status == "complained" {
		status = constants.CallbackStatusRejected
	}

	reportTime, _ := time.Parse(time.RFC3339, bounceReport.Timestamp)
	if reportTime.IsZero() {
		reportTime = time.Now()
	}

	return resp, []*CallbackResult{{
		ProviderID:   bounceReport.MessageID,
		Status:       status,
		ErrorCode:    bounceReport.ErrorCode,
		ErrorMessage: bounceReport.ErrorMessage,
		ReportTime:   reportTime,
	}}, nil
}
