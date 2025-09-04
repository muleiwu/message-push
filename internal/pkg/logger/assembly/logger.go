package assembly

import (
	"sync"

	"cnb.cool/mliev/examples/go-web/internal/interfaces"
	"cnb.cool/mliev/examples/go-web/internal/pkg/logger/impl"
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
