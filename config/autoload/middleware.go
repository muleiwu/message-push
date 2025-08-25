package autoload

import (
	"cnb.cool/mliev/examples/go-web/app/middleware"
	envInterface "cnb.cool/mliev/examples/go-web/internal/interfaces"
	"github.com/gin-gonic/gin"
)

type Middleware struct {
}

func (receiver Middleware) InitConfig(helper envInterface.GetHelperInterface) map[string]any {
	return map[string]any{
		"http.middleware": []gin.HandlerFunc{
			middleware.CorsMiddleware(helper),
		},
	}
}
