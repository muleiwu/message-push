package reload

import "sync"

var (
	reloadChan chan struct{}
	once       sync.Once
)

// GetReloadChan 获取重载通道（单例模式）
func GetReloadChan() <-chan struct{} {
	once.Do(func() {
		reloadChan = make(chan struct{}, 1)
	})
	return reloadChan
}

// TriggerReload 触发重载信号
func TriggerReload() {
	once.Do(func() {
		reloadChan = make(chan struct{}, 1)
	})
	select {
	case reloadChan <- struct{}{}:
		// 成功发送重载信号
	default:
		// 通道已满，忽略本次请求
	}
}
