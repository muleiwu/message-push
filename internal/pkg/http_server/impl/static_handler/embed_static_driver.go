package static_handler

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// EmbedStaticDriver embed.FS 驱动实现
type EmbedStaticDriver struct {
	embedFS embed.FS
}

// NewEmbedStaticDriver 创建 embed 驱动实例
func NewEmbedStaticDriver(embedFS embed.FS) *EmbedStaticDriver {
	return &EmbedStaticDriver{
		embedFS: embedFS,
	}
}

// FileExists 检查文件是否存在于 embed.FS 中
func (d *EmbedStaticDriver) FileExists(path string) bool {
	// 移除开头的斜杠
	path = strings.TrimPrefix(path, "/")
	_, err := fs.Stat(d.embedFS, path)
	return err == nil
}

// GetFS 获取指定目录的文件系统
func (d *EmbedStaticDriver) GetFS(dir string) (http.FileSystem, error) {
	subPath := fmt.Sprintf("static/%s", dir)
	subFS, err := fs.Sub(d.embedFS, subPath)
	if err != nil {
		return nil, err
	}
	return http.FS(subFS), nil
}

// ServeFile 提供文件到 gin.Context
func (d *EmbedStaticDriver) ServeFile(c *gin.Context, dir string, relativePath string) error {
	subPath := fmt.Sprintf("static/%s", dir)
	subFS, err := fs.Sub(d.embedFS, subPath)
	if err != nil {
		return err
	}

	// 移除开头的斜杠
	relativePath = strings.TrimPrefix(relativePath, "/")

	// 先检查文件本身是否存在
	if d.fileExistsInFS(subFS, relativePath) {
		c.FileFromFS(relativePath, http.FS(subFS))
		return nil
	}

	// 检查 路径/index.html 是否存在
	indexPath := strings.TrimSuffix(relativePath, "/") + "/index.html"
	if d.fileExistsInFS(subFS, indexPath) {
		c.FileFromFS(indexPath, http.FS(subFS))
		return nil
	}

	return fmt.Errorf("file not found")
}

// fileExistsInFS 检查文件是否存在于文件系统中
func (d *EmbedStaticDriver) fileExistsInFS(filesystem fs.FS, path string) bool {
	path = strings.TrimPrefix(path, "/")
	_, err := fs.Stat(filesystem, path)
	return err == nil
}

// GetDriverName 获取驱动名称
func (d *EmbedStaticDriver) GetDriverName() string {
	return "embed"
}
