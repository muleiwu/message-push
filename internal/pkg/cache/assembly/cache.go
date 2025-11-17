package assembly

import (
	"errors"
	"fmt"
	"time"

	"cnb.cool/mliev/push/message-push/internal/interfaces"
	gocache "github.com/muleiwu/go-cache"
	"github.com/muleiwu/gsr"
)

type Cache struct {
	Helper interfaces.HelperInterface
}

func (receiver *Cache) Assembly() error {

	driver := receiver.Helper.GetConfig().GetString("cache.driver", "redis")
	receiver.Helper.GetLogger().Debug("加载缓存驱动" + driver)

	if driver == "redis" && receiver.Helper.GetRedis() == nil {
		panic(errors.New("缓存服务驱动配置为：redis，但Redis服务不可用，拒绝启动"))
	}

	cacheDriver, err := receiver.GetDriver(driver)
	if err != nil {
		fmt.Printf("[cache] 加载缓存驱动失败: %s", err.Error())
	}
	receiver.Helper.SetCache(cacheDriver)

	return nil
}

func (receiver *Cache) GetDriver(driver string) (gsr.Cacher, error) {

	if driver == "redis" {
		return gocache.NewRedis(receiver.Helper.GetRedis()), nil
	} else if driver == "memory" || driver == "local" {
		// 设置超时时间和清理时间
		return gocache.NewMemory(5*time.Minute, 10*time.Minute), nil
	} else {
		return gocache.NewNone(), nil
	}
}
