package autoload

import (
	"cnb.cool/mliev/examples/go-web/app/controller"
	envInterface "cnb.cool/mliev/examples/go-web/internal/interfaces"
	"cnb.cool/mliev/examples/go-web/internal/pkg/http_server/impl"
	"github.com/gin-gonic/gin"
)

type Router struct {
}

func (receiver Router) InitConfig(helper envInterface.GetHelperInterface) map[string]any {
	return map[string]any{
		"http.router": func(router *gin.Engine, deps *impl.HttpDeps) {

			health := router.Group("/health")
			{
				// 健康检查接口
				health.GET("", deps.WrapHandler(controller.HealthController{}.GetHealth))
				health.GET("/simple", deps.WrapHandler(controller.HealthController{}.GetHealthSimple))
			}

			// 首页
			// router.GET("/", deps.WrapHandler(controller.IndexController{}.GetIndex))

			// API路由组
			//v1 := router.Group("/api/v1")
			//{
			//	// 这里添加v1版本的API路由
			//	_ = v1 // 暂时避免未使用变量的警告
			//}

		},
	}
}
