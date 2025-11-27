package autoload

import envInterface "cnb.cool/mliev/push/message-push/internal/interfaces"

type Cache struct {
}

func (receiver Cache) InitConfig(helper envInterface.HelperInterface) map[string]any {
	return map[string]any{
		"cache.driver": helper.GetEnv().GetString("cache.driver", "redis"), // memory,redis,none
	}
}
