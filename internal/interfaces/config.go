package interfaces

type InitConfig interface {
	InitConfig(helper GetHelperInterface) map[string]any
}
