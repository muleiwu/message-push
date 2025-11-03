package helper

import (
	"sync"

	"cnb.cool/mliev/examples/go-web/internal/interfaces"
	"github.com/muleiwu/gsr"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type Helper struct {
	env      gsr.Enver
	cache    gsr.Cacher
	config   gsr.Provider
	logger   gsr.Logger
	redis    *redis.Client
	database *gorm.DB
}

var helperOnce sync.Once
var helperData interfaces.HelperInterface

func GetHelper() interfaces.HelperInterface {
	helperOnce.Do(func() {
		helperData = &Helper{}
	})
	return helperData
}

func (receiver *Helper) GetEnv() gsr.Enver {
	return receiver.env
}

func (receiver *Helper) GetCache() gsr.Cacher {
	return receiver.cache
}

func (receiver *Helper) GetConfig() gsr.Provider {
	return receiver.config
}

func (receiver *Helper) GetLogger() gsr.Logger {
	return receiver.logger
}

func (receiver *Helper) GetRedis() *redis.Client {
	return receiver.redis
}

func (receiver *Helper) GetDatabase() *gorm.DB {
	return receiver.database
}

func (receiver *Helper) SetEnv(env gsr.Enver) {
	receiver.env = env
}

func (receiver *Helper) SetCache(cache gsr.Cacher) {
	receiver.cache = cache
}

func (receiver *Helper) SetConfig(config gsr.Provider) {
	receiver.config = config
}

func (receiver *Helper) SetLogger(logger gsr.Logger) {
	receiver.logger = logger
}

func (receiver *Helper) SetRedis(redis *redis.Client) {
	receiver.redis = redis
}

func (receiver *Helper) SetDatabase(database *gorm.DB) {
	receiver.database = database
}
