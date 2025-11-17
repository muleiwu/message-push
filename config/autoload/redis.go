package autoload

import (
	envInterface "cnb.cool/mliev/push/message-push/internal/interfaces"
	"github.com/muleiwu/gsr"
)

type Redis struct {
	env gsr.Enver
}

func (receiver Redis) InitConfig(helper envInterface.HelperInterface) map[string]any {
	return map[string]any{
		"redis.host":     helper.GetEnv().GetString("redis.host", "localhost"),
		"redis.port":     helper.GetEnv().GetInt("redis.port", 6379),
		"redis.password": helper.GetEnv().GetString("redis.password", ""),
		"redis.db":       helper.GetEnv().GetInt("redis.db", 0),
	}
}
