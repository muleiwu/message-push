package interfaces

type InitConfig interface {
	InitConfig(helper HelperInterface) map[string]any
}
