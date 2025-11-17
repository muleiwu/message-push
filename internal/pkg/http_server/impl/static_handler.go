package impl

import (
	"embed"
	"fmt"
	"net/http"
	"strings"

	"cnb.cool/mliev/examples/go-web/internal/interfaces"
	"cnb.cool/mliev/examples/go-web/internal/pkg/http_server/impl/static_handler"
	"github.com/gin-gonic/gin"
)

type StaticHandler struct {
	helper interfaces.HelperInterface
	engine *gin.Engine
	driver static_handler.StaticFileDriver
}

func NewStaticHandler(helper interfaces.HelperInterface, engine *gin.Engine) *StaticHandler {
	return &StaticHandler{
		helper: helper,
		engine: engine,
	}
}

// setupStaticFileServers 为嵌入的静态文件设置HTTP服务
func (receiver *StaticHandler) setupStaticFileServers() {

	if !receiver.helper.GetConfig().GetBool("http.load_static", false) {
		return
	}

	staticDirSlice := receiver.helper.GetConfig().GetStringSlice("http.static_dir", []string{})

	if len(staticDirSlice) == 0 {
		receiver.helper.GetLogger().Warn("没有配置需要加载的目录")
		return
	}

	// 初始化驱动（只判断一次）
	staticMode := receiver.helper.GetConfig().GetString("http.static_mode", "embed")
	receiver.helper.GetLogger().Debug(fmt.Sprintf("当前静态文件模式：%s", staticMode))

	if staticMode == "disk" {
		// disk 模式下使用磁盘驱动，支持热更新
		receiver.driver = static_handler.NewDiskStaticDriver("./static")
		receiver.helper.GetLogger().Info("Disk 模式：使用磁盘驱动加载静态文件，支持热更新")
	} else {
		// embed 模式下使用 embed 驱动
		staticFs := receiver.helper.GetConfig().Get("static.fs", map[string]embed.FS{}).(map[string]embed.FS)
		embeddedFs, ok := staticFs["web.static"]
		if !ok {
			receiver.helper.GetLogger().Debug("不存在需要对Web暴露的静态资源")
			return
		}
		receiver.driver = static_handler.NewEmbedStaticDriver(embeddedFs)
		receiver.helper.GetLogger().Info("Embed 模式：使用 embed 驱动加载静态文件")
	}

	for i, dir := range staticDirSlice {
		receiver.loadStatic(dir)
		receiver.helper.GetLogger().Debug(fmt.Sprintf("序号：%d 加载文件夹：%s", i, dir))
	}

	// 统一处理未匹配的路由
	receiver.engine.NoRoute(func(c *gin.Context) {
		// 只处理 GET 请求
		if c.Request.Method != "GET" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Not Found"})
			return
		}

		path := c.Request.URL.Path

		// 排除 API 路径和静态资源文件
		if receiver.excludeStaticPath(path) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Not Found"})
			return
		}

		// 检查请求路径是否匹配配置的静态目录
		for _, dir := range staticDirSlice {
			staticPrefix := fmt.Sprintf("/%s", dir)
			if strings.HasPrefix(path, staticPrefix) || path == staticPrefix {
				// 去掉前缀，获取相对路径
				relativePath := strings.TrimPrefix(path, staticPrefix)
				if relativePath == "" || relativePath == "/" {
					relativePath = "/index.html"
				}

				// 使用驱动提供文件
				err := receiver.driver.ServeFile(c, dir, relativePath)
				if err == nil {
					return
				}

				// 文件不存在，返回 404
				c.String(http.StatusNotFound, "Not Found")
				return
			}
		}

		// 不匹配任何静态目录，返回 404
		c.String(http.StatusNotFound, "Not Found")
	})

}

func (receiver *StaticHandler) loadStatic(dir string) {
	relativePath := fmt.Sprintf("/%s", dir)

	// 使用驱动获取文件系统
	fileSystem, err := receiver.driver.GetFS(dir)
	if err == nil {
		receiver.engine.StaticFS(relativePath, fileSystem)
		receiver.helper.GetLogger().Info(fmt.Sprintf("已启用 %s 静态文件服务 (%s驱动)", relativePath, receiver.driver.GetDriverName()))
	} else {
		receiver.helper.GetLogger().Error(fmt.Sprintf("设置%s静态文件失败: %v", relativePath, err))
	}
}

// excludeStaticPath 判断是否排除某些路径不重定向到单页应用
func (receiver *StaticHandler) excludeStaticPath(path string) bool {
	// 排除 API 路径（这些路径通常以 /admin/api, /api 等开头）
	apiPrefixes := []string{"/admin/api/", "/api/"}
	for _, prefix := range apiPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}

	// 排除已知的静态文件扩展名
	extensions := []string{".js", ".css", ".png", ".jpg", ".jpeg", ".gif", ".svg", ".ico", ".woff", ".woff2", ".ttf", ".eot"}
	for _, ext := range extensions {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}

	return false
}
