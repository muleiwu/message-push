package autoload

import "cnb.cool/mliev/open/go-web/pkg/helper"

// EmailAttachments 邮件附件容量配置。
type EmailAttachments struct{}

func (EmailAttachments) InitConfig() map[string]any {
	env := helper.GetEnv()
	return map[string]any{
		"email.attachments.max_count":         env.GetInt("email.attachments.max_count", 5),
		"email.attachments.max_file_size_mb":  env.GetInt("email.attachments.max_file_size_mb", 5),
		"email.attachments.max_total_size_mb": env.GetInt("email.attachments.max_total_size_mb", 10),
	}
}
