// Package assembly 将 delivery 模块的消息生产者注册到 go-web DI 容器。
package assembly

import (
	"reflect"

	"cnb.cool/mliev/open/go-web/pkg/helper"
	"cnb.cool/mliev/push/message-push/modules/delivery/domain"
	"cnb.cool/mliev/push/message-push/modules/delivery/infrastructure/queue"
	"github.com/redis/go-redis/v9"
)

// Producer 实现 go-web 的 AssemblyInterface，向容器提供 domain.Producer。
type Producer struct{}

// Type 返回此 Assembly 提供的服务类型（domain.Producer 接口）。
func (receiver *Producer) Type() reflect.Type {
	return reflect.TypeFor[domain.Producer]()
}

// DependsOn 生产者构造时需要 Redis 客户端。
func (receiver *Producer) DependsOn() []reflect.Type {
	return []reflect.Type{
		reflect.TypeFor[*redis.Client](),
	}
}

// Assembly 构建消息生产者（实现 domain.Producer），由框架注册到容器。
func (receiver *Producer) Assembly() (any, error) {
	return queue.NewProducer(helper.GetRedis()), nil
}
