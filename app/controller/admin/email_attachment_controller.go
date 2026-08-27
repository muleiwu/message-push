package admin

import (
	httpInterfaces "cnb.cool/mliev/open/go-web/pkg/server/http_server/interfaces"
	"cnb.cool/mliev/push/message-push/app/controller"
	appHelper "cnb.cool/mliev/push/message-push/app/helper"
)

// EmailAttachmentController 提供管理端附件能力配置。
type EmailAttachmentController struct{}

func (EmailAttachmentController) GetLimits(ctx httpInterfaces.RouterContextInterface) {
	limits := appHelper.GetEmailAttachmentLimits()
	controller.SuccessResponse(ctx, map[string]any{
		"max_count":            limits.MaxCount,
		"max_file_size_bytes":  limits.MaxFileBytes,
		"max_total_size_bytes": limits.MaxTotalBytes,
	})
}
