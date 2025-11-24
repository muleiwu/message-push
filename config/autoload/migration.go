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
		&model.ProviderAccount{}, // 新的服务商账号配置表
		&model.Provider{},        // 保留旧表以支持数据迁移
		&model.Channel{},
		&model.ProviderChannel{},
		&model.PushChannel{},
		&model.ChannelProviderRelation{}, // 保留旧表以支持数据迁移

		// 推送任务
		&model.PushTask{},
		&model.PushBatchTask{},
		&model.PushLog{},

		// 模板管理
		&model.MessageTemplate{},
		&model.ProviderTemplate{},
		&model.ChannelTemplateBinding{}, // 通道模板绑定配置表（已集成模板绑定功能）

		// 健康检查和配额统计
		&model.ChannelHealthHistory{},
		&model.AppQuotaStat{},
		&model.ProviderQuotaStat{},

		// 管理员模块
		&model.AdminUser{},
	}
}

func (receiver Migration) InitConfig(helper envInterface.HelperInterface) map[string]any {
	return map[string]any{
		"database.migration": receiver.Get(),
	}
}
