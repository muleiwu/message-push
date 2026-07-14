package autoload

import (
	gowebInterfaces "cnb.cool/mliev/open/go-web/pkg/server/http_server/interfaces"
	"cnb.cool/mliev/push/message-push/app/controller"
	"cnb.cool/mliev/push/message-push/app/controller/admin"
	"cnb.cool/mliev/push/message-push/app/middleware"
)

type Router struct {
}

func (receiver Router) InitConfig() map[string]any {
	return map[string]any{
		"http.router": func(router gowebInterfaces.RouterInterface) {

			// 首页
			router.GET("/", controller.IndexController{}.GetIndex)

			health := router.Group("/health")
			{
				// 健康检查接口
				health.GET("", controller.HealthController{}.GetHealth)
				health.GET("/simple", controller.HealthController{}.GetHealthSimple)
			}

			// Install API - 系统安装（不需要认证）
			install := router.Group("/api/install")
			{
				// 检查安装状态 - 始终可访问
				install.GET("/check", controller.InstallController{}.CheckInstall)
			}

			// Install API - 安装操作（已安装后禁止访问）
			installOps := router.Group("/api/install")
			installOps.Use(middleware.BlockIfInstalledMiddleware())
			{
				installOps.POST("/test-connection", controller.InstallController{}.TestConnection)
				installOps.POST("/test-redis", controller.InstallController{}.TestRedisConnection)
				installOps.POST("/submit", controller.InstallController{}.SubmitInstall)
			}

			// Callback API - 服务商回调（不需要认证，供服务商调用）
			callback := router.Group("/api/callback")
			{
				// 动态路由，支持所有账号的回调
				callback.POST("/:id", controller.CallbackController{}.Handle)
				callback.GET("/:id", controller.CallbackController{}.Handle)
			}

			// API v1 - 需要认证、限流、配额检查
			v1 := router.Group("/api/v1")
			v1.Use(middleware.AuthMiddleware())
			v1.Use(middleware.RateLimitMiddleware(100)) // 默认100 QPS
			v1.Use(middleware.QuotaMiddleware())
			{
				// 消息发送接口
				v1.POST("/messages", controller.MessageController{}.Send)
				v1.POST("/messages/batch", controller.MessageController{}.BatchSend)

				// 任务查询接口
				v1.GET("/messages/:task_id", controller.MessageController{}.QueryTask)
			}

			// Admin API - 管理后台认证接口（不需要认证）
			adminAuth := router.Group("/api/admin/auth")
			{
				adminAuth.POST("/login", admin.AuthController{}.Login)
				adminAuth.POST("/logout", admin.AuthController{}.Logout)

				// OIDC SSO 登录
				adminAuth.GET("/oidc/status", admin.OIDCController{}.GetStatus)
				adminAuth.GET("/oidc/authorize", admin.OIDCController{}.Authorize)
				adminAuth.GET("/oidc/callback", admin.OIDCController{}.Callback)
				adminAuth.POST("/oidc/exchange", admin.OIDCController{}.Exchange)
			}

			// Admin API - 管理后台业务接口（需要 JWT 认证）
			adminGroup := router.Group("/api/admin")
			adminGroup.Use(middleware.AdminJWTMiddleware())
			{
				// 用户信息和权限
				adminGroup.GET("/user/info", admin.AuthController{}.GetUserInfo)
				adminGroup.GET("/auth/codes", admin.AuthController{}.GetAccessCodes)

				// 管理员用户管理
				users := adminGroup.Group("/users")
				{
					users.GET("", admin.AdminUserController{}.GetUserList)
					users.POST("", admin.AdminUserController{}.CreateUser)
					users.GET("/:id", admin.AdminUserController{}.GetUser)
					users.PUT("/:id", admin.AdminUserController{}.UpdateUser)
					users.DELETE("/:id", admin.AdminUserController{}.DeleteUser)
					users.POST("/:id/reset-password", admin.AdminUserController{}.ResetPassword)
				}

				// 应用管理
				apps := adminGroup.Group("/applications")
				{
					apps.GET("", admin.ApplicationController{}.GetApplicationList)
					apps.POST("", admin.ApplicationController{}.CreateApplication)
					apps.GET("/:id", admin.ApplicationController{}.GetApplication)
					apps.PUT("/:id", admin.ApplicationController{}.UpdateApplication)
					apps.DELETE("/:id", admin.ApplicationController{}.DeleteApplication)
					apps.POST("/regenerate-secret", admin.ApplicationController{}.RegenerateSecret)
					apps.GET("/:id/quota-usage", admin.ApplicationController{}.GetQuotaUsage)
				}

				// 服务商账号配置管理（新版）
				providerAccounts := adminGroup.Group("/provider-accounts")
				{
					providerAccounts.GET("/available", admin.ProviderAccountController{}.GetAvailableProviders)                     // 获取可用服务商列表
					providerAccounts.GET("/config-fields/:providerCode", admin.ProviderAccountController{}.GetProviderConfigFields) // 获取配置字段定义
					providerAccounts.GET("/active", admin.ProviderAccountController{}.GetActiveProviderAccounts)                    // 获取活跃账号
					providerAccounts.GET("", admin.ProviderAccountController{}.GetProviderAccountList)
					providerAccounts.POST("", admin.ProviderAccountController{}.CreateProviderAccount)
					providerAccounts.GET("/:id", admin.ProviderAccountController{}.GetProviderAccount)
					providerAccounts.PUT("/:id", admin.ProviderAccountController{}.UpdateProviderAccount)
					providerAccounts.DELETE("/:id", admin.ProviderAccountController{}.DeleteProviderAccount)
					providerAccounts.POST("/:id/test", admin.ProviderAccountController{}.TestProviderAccount)

					// 签名管理（嵌套在账号下）
					providerAccounts.GET("/:id/signatures", admin.ProviderSignatureController{}.GetSignatureList)
					providerAccounts.POST("/:id/signatures", admin.ProviderSignatureController{}.CreateSignature)
				}

				// 服务商签名管理（独立路由，用于更新/删除操作）
				signatures := adminGroup.Group("/provider-signatures")
				{
					signatures.GET("/:id", admin.ProviderSignatureController{}.GetSignature)
					signatures.PUT("/:id", admin.ProviderSignatureController{}.UpdateSignature)
					signatures.DELETE("/:id", admin.ProviderSignatureController{}.DeleteSignature)
				}

				// 通道管理
				channels := adminGroup.Group("/channels")
				{
					channels.GET("/active", admin.ChannelController{}.GetActiveChannels) // 先注册 /active
					channels.GET("", admin.ChannelController{}.GetChannelList)
					channels.POST("", admin.ChannelController{}.CreateChannel)
					channels.GET("/:id", admin.ChannelController{}.GetChannel)
					channels.PUT("/:id", admin.ChannelController{}.UpdateChannel)
					channels.DELETE("/:id", admin.ChannelController{}.DeleteChannel)
					channels.POST("/:id/test", admin.ChannelController{}.TestChannel)                               // 测试通道发送
					channels.GET("/:id/available-bindings", admin.ChannelController{}.GetAvailableTemplateBindings) // 先注册具体路径
					channels.GET("/:id/bindings", admin.ChannelController{}.GetChannelBindings)
					channels.POST("/:id/bindings", admin.ChannelController{}.CreateChannelBinding)
					channels.GET("/:id/bindings/:bindingId", admin.ChannelController{}.GetChannelBinding)
					channels.PUT("/:id/bindings/:bindingId", admin.ChannelController{}.UpdateChannelBinding)
					channels.DELETE("/:id/bindings/:bindingId", admin.ChannelController{}.DeleteChannelBinding)
					// 签名映射路由
					channels.GET("/:id/available-signatures", admin.ChannelController{}.GetAvailableProviderSignatures)
					channels.GET("/:id/signature-mappings", admin.ChannelController{}.GetChannelSignatureMappings)
					channels.POST("/:id/signature-mappings", admin.ChannelController{}.CreateChannelSignatureMapping)
					channels.GET("/:id/signature-mappings/:mappingId", admin.ChannelController{}.GetChannelSignatureMapping)
					channels.PUT("/:id/signature-mappings/:mappingId", admin.ChannelController{}.UpdateChannelSignatureMapping)
					channels.DELETE("/:id/signature-mappings/:mappingId", admin.ChannelController{}.DeleteChannelSignatureMapping)
				}

				// 统计查询
				stats := adminGroup.Group("/statistics")
				{
					stats.GET("", admin.StatisticsController{}.GetStatistics)
					stats.GET("/dashboard", admin.StatisticsController{}.GetDashboard)
					stats.GET("/top-applications", admin.StatisticsController{}.GetTopApplications)
					stats.GET("/recent-activities", admin.StatisticsController{}.GetRecentActivities)
				}

				// 日志管理
				logs := adminGroup.Group("/logs")
				{
					logs.GET("", admin.LogController{}.GetLogList)
					logs.GET("/:id", admin.LogController{}.GetLog)
					logs.GET("/task/:task_id", admin.LogController{}.GetLogsByTaskID)
					logs.GET("/task/:task_id/callback", admin.LogController{}.GetCallbackLogsByTaskID)
					logs.GET("/task/:task_id/webhook", admin.LogController{}.GetWebhookLogsByTaskID)
				}

				// 服务商回调记录（统一下行回执与上行短信，按 type 区分）
				callbacks := adminGroup.Group("/callbacks")
				{
					callbacks.GET("", admin.CallbackController{}.GetCallbackList)
				}

				// 任务管理
				pushTasks := adminGroup.Group("/push-tasks")
				{
					pushTasks.GET("", admin.TaskController{}.GetPushTaskList)
					pushTasks.GET("/:id", admin.TaskController{}.GetPushTask)
				}

				// 批量任务管理
				batchTasks := adminGroup.Group("/batch-tasks")
				{
					batchTasks.GET("", admin.TaskController{}.GetPushBatchTaskList)
					batchTasks.GET("/:id", admin.TaskController{}.GetPushBatchTask)
					batchTasks.GET("/:id/tasks", admin.TaskController{}.GetBatchTaskDetails)
				}

				// 模板管理
				templates := adminGroup.Group("/templates")
				{
					templates.GET("", admin.TemplateController{}.ListMessageTemplates)
					templates.POST("", admin.TemplateController{}.CreateMessageTemplate)
					templates.GET("/:id", admin.TemplateController{}.GetMessageTemplate)
					templates.PUT("/:id", admin.TemplateController{}.UpdateMessageTemplate)
					templates.DELETE("/:id", admin.TemplateController{}.DeleteMessageTemplate)
				}

				// 供应商模板管理
				providerTemplates := adminGroup.Group("/provider-templates")
				{
					providerTemplates.GET("", admin.TemplateController{}.ListProviderTemplates)
					providerTemplates.POST("", admin.TemplateController{}.CreateProviderTemplate)
					providerTemplates.GET("/:id", admin.TemplateController{}.GetProviderTemplate)
					providerTemplates.PUT("/:id", admin.TemplateController{}.UpdateProviderTemplate)
					providerTemplates.DELETE("/:id", admin.TemplateController{}.DeleteProviderTemplate)
				}

				// 失败规则管理
				failureRules := adminGroup.Group("/failure-rules")
				{
					failureRules.GET("/options", admin.FailureRuleController{}.GetFailureRuleOptions)
					failureRules.POST("/refresh-cache", admin.FailureRuleController{}.RefreshRuleCache)
					failureRules.GET("", admin.FailureRuleController{}.GetFailureRuleList)
					failureRules.POST("", admin.FailureRuleController{}.CreateFailureRule)
					failureRules.GET("/:id", admin.FailureRuleController{}.GetFailureRule)
					failureRules.PUT("/:id", admin.FailureRuleController{}.UpdateFailureRule)
					failureRules.DELETE("/:id", admin.FailureRuleController{}.DeleteFailureRule)
				}
			}

		},
	}
}
