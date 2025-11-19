package autoload

import envInterface "cnb.cool/mliev/push/message-push/internal/interfaces"

type Base struct {
}

func (receiver Base) InitConfig(helper envInterface.HelperInterface) map[string]any {
	return map[string]any{
		"app.base.app_name": helper.GetEnv().GetString("app.base.app_name", "go-web-app"),
		"app.installed":     helper.GetEnv().GetBool("app.installed", false),
	}
}
