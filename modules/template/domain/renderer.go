// Package domain 是 template 限界上下文的领域层：
// 定义模板渲染端口（Renderer）与模板管理服务端口（Service）。
package domain

import "cnb.cool/mliev/push/message-push/app/model"

// Renderer 模板渲染端口：渲染模板内容、按映射转换参数。
// 被 messaging / delivery 等模块跨模块调用，经 assembly 注册到 go-web 容器。
type Renderer interface {
	// Render 渲染模板（支持 {{.variable}} 与 {variable} 两种占位符）
	Render(templateCode string, params map[string]string) (string, error)
	// RenderSimple 渲染简单模板（{variable} 占位符）
	RenderSimple(templateContent string, params map[string]string) (string, error)
	// MapParams 按参数映射配置转换为供应商变量名到值的映射
	MapParams(params map[string]string, mapping []model.ParamMappingItem) map[string]string
	// RenderJSON 将参数渲染为 JSON 字符串
	RenderJSON(params map[string]string) (string, error)
}
