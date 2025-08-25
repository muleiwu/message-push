package autoload

import (
	"cnb.cool/mliev/examples/go-web/app/controller"
	envInterface "cnb.cool/mliev/examples/go-web/internal/interfaces"
	"cnb.cool/mliev/examples/go-web/internal/pkg/http_server/impl"
	"github.com/gin-gonic/gin"
)

type Router struct {
}

func (receiver Router) InitConfig(env envInterface.GetHelperInterface) map[string]any {
	return map[string]any{
		"http.router": func(router *gin.Engine, deps *impl.HttpDeps) {

			// 健康检查接口
			router.GET("/health", deps.WrapHandler(controller.HealthController{}.GetHealth))
			router.GET("/health/simple", deps.WrapHandler(controller.HealthController{}.GetHealthSimple))

			// 首页
			router.GET("/", deps.WrapHandler(controller.IndexController{}.GetIndex))

			// API路由组
			v1 := router.Group("/api/v1")
			{
				// 这里添加v1版本的API路由
				_ = v1 // 暂时避免未使用变量的警告
			}

		},
	}
}
