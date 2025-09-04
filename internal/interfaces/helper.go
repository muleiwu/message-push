package interfaces

import (
	"github.com/muleiwu/gsr/config_interface"
	"github.com/muleiwu/gsr/env_interface"
	"github.com/muleiwu/gsr/logger_interface"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type GetHelperInterface interface {
	GetEnv() env_interface.EnvInterface
	GetConfig() config_interface.ConfigInterface
	GetLogger() logger_interface.LoggerInterface
	GetRedis() *redis.Client
	GetDatabase() *gorm.DB
}

type SetHelperInterface interface {
	SetEnv(env env_interface.EnvInterface)
	SetConfig(config config_interface.ConfigInterface)
	SetLogger(logger logger_interface.LoggerInterface)
	SetRedis(redis *redis.Client)
	SetDatabase(database *gorm.DB)
}

type HelperInterface interface {
	GetHelperInterface
	SetHelperInterface
}
