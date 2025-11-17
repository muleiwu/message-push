package config

import (
	"cnb.cool/mliev/push/message-push/config/autoload"
	"cnb.cool/mliev/push/message-push/internal/interfaces"
)

type Config struct {
}

func (receiver Config) Get() []interfaces.InitConfig {
	return []interfaces.InitConfig{
		autoload.Base{},
		autoload.Cache{},
		autoload.Http{},
		autoload.StaticFs{},
		autoload.Database{},
		autoload.Redis{},
		autoload.Middleware{},
		autoload.Router{},
	}
}
