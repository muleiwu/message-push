package autoload

import (
	gowebInterfaces "cnb.cool/mliev/open/go-web/pkg/server/http_server/interfaces"
	"cnb.cool/mliev/push/message-push/app/middleware"
)

type Middleware struct {
}

func (receiver Middleware) InitConfig() map[string]any {
	return map[string]any{
		"http.middleware": []gowebInterfaces.HandlerFunc{
			middleware.CorsMiddleware(),         // 跨域中间件
			middleware.InstallCheckMiddleware(), // 安装检查中间件
		},
	}
}
