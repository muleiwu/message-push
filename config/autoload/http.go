package autoload

import envInterface "cnb.cool/mliev/push/message-push/internal/interfaces"

type Http struct {
}

func (receiver Http) InitConfig(helper envInterface.HelperInterface) map[string]any {
	return map[string]any{
		"http.load_static": helper.GetEnv().GetBool("http.load_static", true),
		"http.static_mode": helper.GetEnv().GetString("http.static_mode", "disk"), // disk embed
		"http.static_dir":  []string{"install", "admin", "image"},
	}
}
