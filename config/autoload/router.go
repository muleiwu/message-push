package autoload

import (
	"cnb.cool/mliev/push/message-push/app/controller"
	"cnb.cool/mliev/push/message-push/app/controller/admin"
	"cnb.cool/mliev/push/message-push/app/middleware"
	envInterface "cnb.cool/mliev/push/message-push/internal/interfaces"
	"cnb.cool/mliev/push/message-push/internal/pkg/http_server/impl"
	"github.com/gin-gonic/gin"
)

type Router struct {
}

func (receiver Router) InitConfig(helper envInterface.HelperInterface) map[string]any {
	return map[string]any{
		"http.router": func(router *gin.Engine, deps *impl.HttpDeps) {

			// 首页
			router.GET("/", deps.WrapHandler(controller.IndexController{}.GetIndex))

			health := router.Group("/health")
			{
				// 健康检查接口
				health.GET("", deps.WrapHandler(controller.HealthController{}.GetHealth))
				health.GET("/simple", deps.WrapHandler(controller.HealthController{}.GetHealthSimple))
			}

			// Install API - 系统安装（不需要认证）
			install := router.Group("/api/install")
			{
				install.GET("/check", deps.WrapHandler(controller.InstallController{}.CheckInstall))
				install.POST("/test-connection", deps.WrapHandler(controller.InstallController{}.TestConnection))
				install.POST("/test-redis", deps.WrapHandler(controller.InstallController{}.TestRedisConnection))
				install.POST("/submit", deps.WrapHandler(controller.InstallController{}.SubmitInstall))
			}

			// API v1 - 需要认证、限流、配额检查
			v1 := router.Group("/api/v1")
			v1.Use(middleware.AuthMiddleware())
			v1.Use(middleware.RateLimitMiddleware(100)) // 默认100 QPS
			v1.Use(middleware.QuotaMiddleware())
			{
				// 消息发送接口
				v1.POST("/messages", deps.WrapHandler(controller.MessageController{}.Send))
				v1.POST("/messages/batch", deps.WrapHandler(controller.MessageController{}.BatchSend))

				// 任务查询接口
				v1.GET("/messages/:task_id", deps.WrapHandler(controller.MessageController{}.QueryTask))
			}

			// Admin API - 管理后台认证接口（不需要认证）
			adminAuth := router.Group("/api/admin/auth")
			{
				adminAuth.POST("/login", deps.WrapHandler(admin.AuthController{}.Login))
				adminAuth.POST("/logout", deps.WrapHandler(admin.AuthController{}.Logout))
			}

			// Admin API - 管理后台业务接口（需要 JWT 认证）
			adminGroup := router.Group("/api/admin")
			adminGroup.Use(middleware.AdminJWTMiddleware())
			{
				// 用户信息和权限
				adminGroup.GET("/user/info", deps.WrapHandler(admin.AuthController{}.GetUserInfo))
				adminGroup.GET("/auth/codes", deps.WrapHandler(admin.AuthController{}.GetAccessCodes))

				// 应用管理
				apps := adminGroup.Group("/applications")
				{
					apps.GET("", deps.WrapHandler(admin.ApplicationController{}.GetApplicationList))
					apps.POST("", deps.WrapHandler(admin.ApplicationController{}.CreateApplication))
					apps.GET("/:id", deps.WrapHandler(admin.ApplicationController{}.GetApplication))
					apps.PUT("/:id", deps.WrapHandler(admin.ApplicationController{}.UpdateApplication))
					apps.DELETE("/:id", deps.WrapHandler(admin.ApplicationController{}.DeleteApplication))
					apps.POST("/regenerate-secret", deps.WrapHandler(admin.ApplicationController{}.RegenerateSecret))
					apps.GET("/:id/quota-usage", deps.WrapHandler(admin.ApplicationController{}.GetQuotaUsage))
				}

				// 服务商管理
				providers := adminGroup.Group("/providers")
				{
					providers.GET("/active", deps.WrapHandler(admin.ProviderController{}.GetActiveProviders)) // 先注册 /active
					providers.GET("", deps.WrapHandler(admin.ProviderController{}.GetProviderList))
					providers.POST("", deps.WrapHandler(admin.ProviderController{}.CreateProvider))
					providers.GET("/:id", deps.WrapHandler(admin.ProviderController{}.GetProvider))
					providers.PUT("/:id", deps.WrapHandler(admin.ProviderController{}.UpdateProvider))
					providers.DELETE("/:id", deps.WrapHandler(admin.ProviderController{}.DeleteProvider))
					providers.POST("/:id/test", deps.WrapHandler(admin.ProviderController{}.TestProvider))
				}

				// 通道管理
				channels := adminGroup.Group("/channels")
				{
					channels.GET("/active", deps.WrapHandler(admin.ChannelController{}.GetActiveChannels)) // 先注册 /active
					channels.GET("", deps.WrapHandler(admin.ChannelController{}.GetChannelList))
					channels.POST("", deps.WrapHandler(admin.ChannelController{}.CreateChannel))
					channels.GET("/:id", deps.WrapHandler(admin.ChannelController{}.GetChannel))
					channels.PUT("/:id", deps.WrapHandler(admin.ChannelController{}.UpdateChannel))
					channels.DELETE("/:id", deps.WrapHandler(admin.ChannelController{}.DeleteChannel))
					channels.POST("/bind-provider", deps.WrapHandler(admin.ChannelController{}.BindProviderToChannel))
					channels.GET("/:id/providers", deps.WrapHandler(admin.ChannelController{}.GetChannelProviders))
					channels.PUT("/provider-relation/:relationId", deps.WrapHandler(admin.ChannelController{}.UpdateChannelProviderRelation))
					channels.DELETE("/provider-relation/:relationId", deps.WrapHandler(admin.ChannelController{}.UnbindChannelProvider))
				}

				// 统计查询
				stats := adminGroup.Group("/statistics")
				{
					stats.GET("", deps.WrapHandler(admin.StatisticsController{}.GetStatistics))
					stats.GET("/dashboard", deps.WrapHandler(admin.StatisticsController{}.GetDashboard))
					stats.GET("/top-applications", deps.WrapHandler(admin.StatisticsController{}.GetTopApplications))
					stats.GET("/recent-activities", deps.WrapHandler(admin.StatisticsController{}.GetRecentActivities))
				}

				// 日志管理
				logs := adminGroup.Group("/logs")
				{
					logs.GET("", deps.WrapHandler(admin.LogController{}.GetLogList))
					logs.GET("/:id", deps.WrapHandler(admin.LogController{}.GetLog))
				}
			}

		},
	}
}
