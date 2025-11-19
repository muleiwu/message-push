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
		&model.Provider{},
		&model.Channel{},
		&model.ProviderChannel{},
		&model.PushChannel{},
		&model.ChannelProviderRelation{},

		// 推送任务
		&model.PushTask{},
		&model.PushBatchTask{},
		&model.PushLog{},

		// 健康检查和配额统计
		&model.ChannelHealthHistory{},
		&model.AppQuotaStat{},
		&model.ProviderQuotaStat{},
	}
}

func (receiver Migration) InitConfig(helper envInterface.HelperInterface) map[string]any {
	return map[string]any{
		"database.migration": receiver.Get(),
	}
}
