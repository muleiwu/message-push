package helper

import (
	"github.com/muleiwu/gsr/config_interface"
	"github.com/muleiwu/gsr/env_interface"
	"github.com/muleiwu/gsr/logger_interface"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type Helper struct {
	env      env_interface.EnvInterface
	config   config_interface.ConfigInterface
	logger   logger_interface.LoggerInterface
	redis    *redis.Client
	database *gorm.DB
}

func (receiver *Helper) GetEnv() env_interface.EnvInterface {
	return receiver.env
}

func (receiver *Helper) GetConfig() config_interface.ConfigInterface {
	return receiver.config
}

func (receiver *Helper) GetLogger() logger_interface.LoggerInterface {
	return receiver.logger
}

func (receiver *Helper) GetRedis() *redis.Client {
	return receiver.redis
}

func (receiver *Helper) GetDatabase() *gorm.DB {
	return receiver.database
}

func (receiver *Helper) SetEnv(env env_interface.EnvInterface) {
	receiver.env = env
}

func (receiver *Helper) SetConfig(config config_interface.ConfigInterface) {
	receiver.config = config
}

func (receiver *Helper) SetLogger(logger logger_interface.LoggerInterface) {
	receiver.logger = logger
}

func (receiver *Helper) SetRedis(redis *redis.Client) {
	receiver.redis = redis
}

func (receiver *Helper) SetDatabase(database *gorm.DB) {
	receiver.database = database
}
