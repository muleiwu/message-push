package autoload

import "embed"

type StaticFs struct {
}

func (receiver StaticFs) InitConfig() map[string]any {
	return map[string]any{
		// 实际的静态资源（templates / web.static）由 main.go 通过
		// cmd.WithTemplateFs / cmd.WithWebStaticFs 注入，框架在启动时覆盖此键。
		"static.fs": map[string]embed.FS{},
	}
}
