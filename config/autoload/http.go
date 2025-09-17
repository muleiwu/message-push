package autoload

import envInterface "cnb.cool/mliev/examples/go-web/internal/interfaces"

type Http struct {
}

func (receiver Http) InitConfig(helper envInterface.HelperInterface) map[string]any {
	return map[string]any{
		"http.load_static": false,
		"http.static_dir":  []string{},
	}
}
