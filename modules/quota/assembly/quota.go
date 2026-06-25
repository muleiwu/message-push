// Package assembly 将 quota 模块的配额服务注册到 go-web DI 容器。
package assembly

import (
	"reflect"

	"cnb.cool/mliev/push/message-push/modules/quota/domain"
	"cnb.cool/mliev/push/message-push/modules/quota/infrastructure"
	"github.com/redis/go-redis/v9"
)

// Quota 实现 go-web 的 AssemblyInterface，向容器提供 domain.Service。
type Quota struct{}

func (receiver *Quota) Type() reflect.Type {
	return reflect.TypeFor[domain.Service]()
}

// DependsOn 配额服务构造时通过 helper 访问 Redis。
func (receiver *Quota) DependsOn() []reflect.Type {
	return []reflect.Type{
		reflect.TypeFor[*redis.Client](),
	}
}

func (receiver *Quota) Assembly() (any, error) {
	return infrastructure.New(), nil
}
