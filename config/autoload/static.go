package autoload

import (
	"embed"

	envInterface "cnb.cool/mliev/examples/go-web/internal/interfaces"
)

type Static struct {
}

func (receiver Static) InitConfig(helper envInterface.GetHelperInterface) map[string]any {
	return map[string]any{
		"file.static": func() map[string]embed.FS {
			return map[string]embed.FS{
				// 这里会在启动的时候在 main.go 注入静态资源进来，请在main.go 添加静态资源
				//"tempe": embed.FS{},
			}
		},
	}
}
