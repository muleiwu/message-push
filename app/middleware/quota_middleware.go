package middleware

import (
	"context"
	"fmt"

	"cnb.cool/mliev/open/go-web/pkg/helper"
	httpInterfaces "cnb.cool/mliev/open/go-web/pkg/server/http_server/interfaces"
	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/controller"
	"cnb.cool/mliev/push/message-push/modules/quota"
)

// QuotaMiddleware 配额中间件：委托 quota 模块进行按应用每日配额校验。
func QuotaMiddleware() httpInterfaces.HandlerFunc {
	return func(c httpInterfaces.RouterContextInterface) {
		appDBID := c.Get("app_db_id")
		if appDBID == nil {
			c.Next()
			return
		}

		// 获取应用的每日配额配置
		var dailyQuota any = c.Get("daily_quota")
		if dailyQuota == nil {
			dailyQuota = 10000 // 默认配额
		}

		// 0 表示不限制
		if dailyQuota.(int) == 0 {
			c.Next()
			return
		}

		// 检查今日配额（校验并计数）
		allowed, err := quota.GetService().Check(context.Background(), appDBID.(uint), dailyQuota.(int))
		if err != nil {
			helper.GetLogger().Error(fmt.Sprintf("quota check error: %v", err))
			c.Next()
			return
		}

		if !allowed {
			controller.FailWithCode(c, constants.CodeQuotaExceeded)
			c.Abort()
			return
		}

		c.Next()
	}
}
