package config

import (
	"cnb.cool/mliev/push/message-push/config/autoload"
	"cnb.cool/mliev/push/message-push/internal/interfaces"
	"cnb.cool/mliev/push/message-push/internal/pkg/http_server/service"
	"cnb.cool/mliev/push/message-push/internal/service/migration"
	"cnb.cool/mliev/push/message-push/internal/service/scheduler_service"
	"cnb.cool/mliev/push/message-push/internal/service/worker_service"
)

type Server struct {
	Helper interfaces.HelperInterface
}

func (receiver Server) Get() []interfaces.ServerInterface {
	return []interfaces.ServerInterface{
		&migration.Migration{
			Helper:    receiver.Helper,
			Migration: autoload.Migration{}.Get(),
		},
		&worker_service.WorkerService{
			Helper: receiver.Helper,
		},
		&scheduler_service.SchedulerService{
			Helper: receiver.Helper,
		},
		&service.HttpServer{
			Helper: receiver.Helper,
		},
	}
}
