// Package assembly 将 identity 模块的应用管理与管理员用户服务注册到 go-web DI 容器。
package assembly

import (
	"reflect"

	"cnb.cool/mliev/push/message-push/modules/identity/domain"
	"cnb.cool/mliev/push/message-push/modules/identity/infrastructure"
	"github.com/muleiwu/gsr"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// ApplicationService 注册 domain.ApplicationService（应用管理，经 DAO 访问 DB，
// 并经 helper 读取 Redis 配额）。
type ApplicationService struct{}

func (receiver *ApplicationService) Type() reflect.Type {
	return reflect.TypeFor[domain.ApplicationService]()
}

func (receiver *ApplicationService) DependsOn() []reflect.Type {
	return []reflect.Type{
		reflect.TypeFor[gsr.Logger](),
		reflect.TypeFor[*gorm.DB](),
		reflect.TypeFor[*redis.Client](),
	}
}

func (receiver *ApplicationService) Assembly() (any, error) {
	return infrastructure.NewAdminApplicationService(), nil
}

// UserService 注册 domain.UserService（管理员用户管理，经 DAO 访问 DB）。
type UserService struct{}

func (receiver *UserService) Type() reflect.Type {
	return reflect.TypeFor[domain.UserService]()
}

func (receiver *UserService) DependsOn() []reflect.Type {
	return []reflect.Type{
		reflect.TypeFor[gsr.Logger](),
		reflect.TypeFor[*gorm.DB](),
	}
}

func (receiver *UserService) Assembly() (any, error) {
	return infrastructure.NewAdminUserService(), nil
}
