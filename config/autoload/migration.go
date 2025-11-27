package autoload

import (
	"cnb.cool/mliev/push/message-push/app/model"
	envInterface "cnb.cool/mliev/push/message-push/internal/interfaces"
)

type Migration struct {
}

func (receiver Migration) Get() []any {
	return []any{
		// 应用和通道管理
		&model.Application{},
		&model.ProviderAccount{},   // 服务商账号配置表
		&model.ProviderSignature{}, // 服务商签名配置表
		&model.Channel{},

		// 推送任务
		&model.PushTask{},
		&model.PushBatchTask{},
		&model.PushLog{},

		// 模板管理
		&model.MessageTemplate{},
		&model.ProviderTemplate{},
		&model.ChannelTemplateBinding{},  // 通道模板绑定配置表
		&model.ChannelSignatureMapping{}, // 通道签名映射表

		// 健康检查和配额统计
		&model.ChannelHealthHistory{},
		&model.AppQuotaStat{},
		&model.ProviderQuotaStat{},

		// 管理员模块
		&model.AdminUser{},

		// Webhook 配置
		&model.WebhookConfig{},

		// 回调和通知日志
		&model.CallbackLog{},
		&model.WebhookLog{},
	}
}

func (receiver Migration) InitConfig(helper envInterface.HelperInterface) map[string]any {
	return map[string]any{
		"database.migration": receiver.Get(),
	}
}
