package middleware

import (
	"net/http"
	"strings"

	"cnb.cool/mliev/open/go-web/pkg/helper"
	httpInterfaces "cnb.cool/mliev/open/go-web/pkg/server/http_server/interfaces"
)

// BlockIfInstalledMiddleware 阻止已安装系统访问安装路由
// 如果系统已安装，返回 403 禁止访问
func BlockIfInstalledMiddleware() httpInterfaces.HandlerFunc {
	return func(c httpInterfaces.RouterContextInterface) {
		installed := helper.GetConfig().GetBool("app.installed", false)
		if installed {
			c.JSON(http.StatusForbidden, map[string]any{
				"code":    403,
				"message": "系统已安装，此接口不可用",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

// InstallCheckMiddleware 安装检查中间件
// 如果系统未安装，除豁免路径外的所有请求都将重定向到安装页面
func InstallCheckMiddleware() httpInterfaces.HandlerFunc {
	return func(c httpInterfaces.RouterContextInterface) {
		// 检查系统是否已安装
		installed := helper.GetConfig().GetBool("app.installed", false)

		// 如果已安装，直接放行
		if installed {
			c.Next()
			return
		}

		// 系统未安装时，获取当前请求路径
		path := c.Request().URL.Path

		// 豁免路径列表（这些路径在未安装时也可以访问）
		exemptPaths := []string{
			"/api/install/",  // 安装API接口
			"/health",        // 健康检查
			"/health/simple", // 健康检查简单版本
			"/install",       // 安装页面静态资源
		}

		// 检查是否为豁免路径
		for _, exemptPath := range exemptPaths {
			// 前缀匹配
			if strings.HasPrefix(path, exemptPath) {
				c.Next()
				return
			}
		}

		// 如果已经在安装页面路径，避免重定向循环
		if strings.HasPrefix(path, "/install") {
			c.Next()
			return
		}

		// 非豁免路径，重定向到安装页面
		c.Redirect(http.StatusFound, "/install/")
		c.Abort()
	}
}
