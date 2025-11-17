package service

import (
	"cnb.cool/mliev/examples/go-web/internal/interfaces"
	"cnb.cool/mliev/examples/go-web/internal/pkg/http_server/impl"
)

type HttpServer struct {
	Helper     interfaces.HelperInterface
	httpServer *impl.HttpServer
}

func (receiver *HttpServer) Run() error {
	receiver.httpServer = impl.NewHttpServer(receiver.Helper)
	receiver.httpServer.RunHttp()
	return nil
}

func (receiver *HttpServer) Stop() error {
	if receiver.httpServer == nil {
		return nil
	}
	return receiver.httpServer.Stop()
}
