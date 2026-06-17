package autoload

import "cnb.cool/mliev/open/go-web/pkg/helper"

type Cache struct {
}

func (receiver Cache) InitConfig() map[string]any {
	return map[string]any{
		"cache.driver": helper.GetEnv().GetString("cache.driver", "redis"), // memory,redis,none
	}
}
