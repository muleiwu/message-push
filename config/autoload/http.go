package autoload

import "cnb.cool/mliev/open/go-web/pkg/helper"

type Http struct {
}

func (receiver Http) InitConfig() map[string]any {
	env := helper.GetEnv()
	return map[string]any{
		"http.addr":        env.GetString("http.addr", ":8080"),
		"http.mode":        env.GetString("http.mode", "debug"), // debug release
		"http.load_static": env.GetBool("http.load_static", true),
		"http.static_mode": env.GetString("http.static_mode", "embed"), // disk embed
		"http.static_dir":  []string{"install", "admin", "image"},
	}
}
