package static_handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// StaticFileDriver 静态文件驱动接口
type StaticFileDriver interface {
	// FileExists 检查文件是否存在
	FileExists(path string) bool

	// GetFS 获取文件系统（用于 gin.StaticFS）
	GetFS(dir string) (http.FileSystem, error)

	// ServeFile 直接提供文件到 gin.Context
	ServeFile(c *gin.Context, dir string, relativePath string) error

	// GetDriverName 获取驱动名称（用于日志）
	GetDriverName() string
}
