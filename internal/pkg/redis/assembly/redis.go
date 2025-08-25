package assembly

import (
	"sync"

	"cnb.cool/mliev/examples/go-web/internal/interfaces"
	"cnb.cool/mliev/examples/go-web/internal/pkg/redis/config"
	"cnb.cool/mliev/examples/go-web/internal/pkg/redis/impl"
)

type Redis struct {
	Helper interfaces.HelperInterface
}

var (
	redisOnce sync.Once
)

func (receiver *Redis) Assembly() {
	redisConfig := config.NewRedis(receiver.Helper.GetConfig())

	redisOnce.Do(func() {
		receiver.Helper.SetRedis(impl.NewRedis(receiver.Helper, redisConfig.Host, redisConfig.Port, redisConfig.DB, redisConfig.Password))
	})
}
