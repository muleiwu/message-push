package config

import (
	"cnb.cool/mliev/open/go-web/pkg/interfaces"
	"cnb.cool/mliev/push/message-push/config/autoload"
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
		autoload.Migration{},
		autoload.Middleware{},
		autoload.Jwt{},
		autoload.Oidc{},
		autoload.Router{},
	}
}
