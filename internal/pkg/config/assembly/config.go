package assembly

import (
	"sync"

	"cnb.cool/mliev/examples/go-web/internal/interfaces"
	configImpl "cnb.cool/mliev/examples/go-web/internal/pkg/config/impl"
)

type Config struct {
	Helper         interfaces.HelperInterface
	DefaultConfigs []interfaces.InitConfig
}

var (
	configOnce sync.Once
)

func (receiver *Config) Assembly() {
	configOnce.Do(func() {
		configHelper := configImpl.NewConfig()
		for _, defaultConfig := range receiver.DefaultConfigs {
			initConfigs := defaultConfig.InitConfig(receiver.Helper)
			for key, val := range initConfigs {
				configHelper.Set(key, val)
			}
		}

		receiver.Helper.SetConfig(configHelper)
	})
}
