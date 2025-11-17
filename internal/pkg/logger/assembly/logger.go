package assembly

import (
	"sync"

	"cnb.cool/mliev/push/message-push/internal/interfaces"
	"cnb.cool/mliev/push/message-push/internal/pkg/logger/impl"
)

type Logger struct {
	Helper interfaces.HelperInterface
}

var (
	loggerOnce sync.Once
)

func (receiver *Logger) Assembly() error {

	receiver.Helper.SetLogger(impl.NewLogger())
	return nil
}
