package interfaces

import (
	"github.com/muleiwu/gsr"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type GetHelperInterface interface {
	GetEnv() gsr.Enver
	GetConfig() gsr.Provider
	GetLogger() gsr.Logger
	GetRedis() *redis.Client
	GetDatabase() *gorm.DB
}

type SetHelperInterface interface {
	SetEnv(env gsr.Enver)
	SetConfig(config gsr.Provider)
	SetLogger(logger gsr.Logger)
	SetRedis(redis *redis.Client)
	SetDatabase(database *gorm.DB)
}

type HelperInterface interface {
	GetHelperInterface
	SetHelperInterface
}
