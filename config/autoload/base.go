package autoload

import "cnb.cool/mliev/open/go-web/pkg/helper"

type Base struct {
}

func (receiver Base) InitConfig() map[string]any {
	env := helper.GetEnv()
	return map[string]any{
		"app.base.app_name": env.GetString("app.base.app_name", "go-web-app"),
		// app.mode 供 go-web logger 选择 development/production 驱动
		"app.mode":      env.GetString("app.mode", "debug"),
		"app.installed": env.GetBool("app.installed", false),
	}
}
