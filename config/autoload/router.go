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

				// 服务商账号配置管理（新版）
				providerAccounts := adminGroup.Group("/provider-accounts")
				{
					providerAccounts.GET("/available", deps.WrapHandler(admin.ProviderAccountController{}.GetAvailableProviders))                     // 获取可用服务商列表
					providerAccounts.GET("/config-fields/:providerCode", deps.WrapHandler(admin.ProviderAccountController{}.GetProviderConfigFields)) // 获取配置字段定义
					providerAccounts.GET("/active", deps.WrapHandler(admin.ProviderAccountController{}.GetActiveProviderAccounts))                    // 获取活跃账号
					providerAccounts.GET("", deps.WrapHandler(admin.ProviderAccountController{}.GetProviderAccountList))
					providerAccounts.POST("", deps.WrapHandler(admin.ProviderAccountController{}.CreateProviderAccount))
					providerAccounts.GET("/:id", deps.WrapHandler(admin.ProviderAccountController{}.GetProviderAccount))
					providerAccounts.PUT("/:id", deps.WrapHandler(admin.ProviderAccountController{}.UpdateProviderAccount))
					providerAccounts.DELETE("/:id", deps.WrapHandler(admin.ProviderAccountController{}.DeleteProviderAccount))
					providerAccounts.POST("/:id/test", deps.WrapHandler(admin.ProviderAccountController{}.TestProviderAccount))

					// 签名管理（嵌套在账号下）
					providerAccounts.GET("/:id/signatures", deps.WrapHandler(admin.ProviderSignatureController{}.GetSignatureList))
					providerAccounts.POST("/:id/signatures", deps.WrapHandler(admin.ProviderSignatureController{}.CreateSignature))
				}

				// 服务商签名管理（独立路由，用于更新/删除操作）
				signatures := adminGroup.Group("/provider-signatures")
				{
					signatures.GET("/:id", deps.WrapHandler(admin.ProviderSignatureController{}.GetSignature))
					signatures.PUT("/:id", deps.WrapHandler(admin.ProviderSignatureController{}.UpdateSignature))
					signatures.DELETE("/:id", deps.WrapHandler(admin.ProviderSignatureController{}.DeleteSignature))
				}

				// 服务商管理（旧版，保持向后兼容）
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
					channels.GET("/:id/available-bindings", deps.WrapHandler(admin.ChannelController{}.GetAvailableTemplateBindings)) // 先注册具体路径
					channels.GET("/:id/bindings", deps.WrapHandler(admin.ChannelController{}.GetChannelBindings))
					channels.POST("/:id/bindings", deps.WrapHandler(admin.ChannelController{}.CreateChannelBinding))
					channels.GET("/:id/bindings/:bindingId", deps.WrapHandler(admin.ChannelController{}.GetChannelBinding))
					channels.PUT("/:id/bindings/:bindingId", deps.WrapHandler(admin.ChannelController{}.UpdateChannelBinding))
					channels.DELETE("/:id/bindings/:bindingId", deps.WrapHandler(admin.ChannelController{}.DeleteChannelBinding))
					// 签名映射路由
					channels.GET("/:id/available-signatures", deps.WrapHandler(admin.ChannelController{}.GetAvailableProviderSignatures))
					channels.GET("/:id/signature-mappings", deps.WrapHandler(admin.ChannelController{}.GetChannelSignatureMappings))
					channels.POST("/:id/signature-mappings", deps.WrapHandler(admin.ChannelController{}.CreateChannelSignatureMapping))
					channels.GET("/:id/signature-mappings/:mappingId", deps.WrapHandler(admin.ChannelController{}.GetChannelSignatureMapping))
					channels.PUT("/:id/signature-mappings/:mappingId", deps.WrapHandler(admin.ChannelController{}.UpdateChannelSignatureMapping))
					channels.DELETE("/:id/signature-mappings/:mappingId", deps.WrapHandler(admin.ChannelController{}.DeleteChannelSignatureMapping))
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

				// 模板管理
				templates := adminGroup.Group("/templates")
				{
					templates.GET("", deps.WrapHandler(admin.TemplateController{}.ListMessageTemplates))
					templates.POST("", deps.WrapHandler(admin.TemplateController{}.CreateMessageTemplate))
					templates.GET("/:id", deps.WrapHandler(admin.TemplateController{}.GetMessageTemplate))
					templates.PUT("/:id", deps.WrapHandler(admin.TemplateController{}.UpdateMessageTemplate))
					templates.DELETE("/:id", deps.WrapHandler(admin.TemplateController{}.DeleteMessageTemplate))
				}

				// 供应商模板管理
				providerTemplates := adminGroup.Group("/provider-templates")
				{
					providerTemplates.GET("", deps.WrapHandler(admin.TemplateController{}.ListProviderTemplates))
					providerTemplates.POST("", deps.WrapHandler(admin.TemplateController{}.CreateProviderTemplate))
					providerTemplates.GET("/:id", deps.WrapHandler(admin.TemplateController{}.GetProviderTemplate))
					providerTemplates.PUT("/:id", deps.WrapHandler(admin.TemplateController{}.UpdateProviderTemplate))
					providerTemplates.DELETE("/:id", deps.WrapHandler(admin.TemplateController{}.DeleteProviderTemplate))
				}
			}

		},
	}
}
