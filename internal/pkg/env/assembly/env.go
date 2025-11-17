package assembly

import (
	"cnb.cool/mliev/push/message-push/internal/interfaces"
	"cnb.cool/mliev/push/message-push/internal/pkg/env/impl"
)

type Env struct {
	Helper interfaces.HelperInterface
}

func (receiver *Env) Assembly() error {
	receiver.Helper.SetEnv(impl.NewEnv())

	return nil
}
