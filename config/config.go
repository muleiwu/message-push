package config

import (
	"cnb.cool/mliev/examples/go-web/config/autoload"
	"cnb.cool/mliev/examples/go-web/internal/interfaces"
)

type Config struct {
}

func (receiver Config) Get() []interfaces.InitConfig {
	return []interfaces.InitConfig{
		autoload.Base{},
		autoload.StaticFs{},
		autoload.Database{},
		autoload.Redis{},
		autoload.Middleware{},
		autoload.Router{},
	}
}
