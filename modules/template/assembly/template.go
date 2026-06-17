// Package assembly 将 template 模块的渲染器与模板管理服务注册到 go-web DI 容器。
package assembly

import (
	"reflect"

	"cnb.cool/mliev/push/message-push/modules/template/domain"
	"cnb.cool/mliev/push/message-push/modules/template/infrastructure"
	"gorm.io/gorm"
)

// Renderer 注册 domain.Renderer（模板渲染器，无外部依赖）。
type Renderer struct{}

func (receiver *Renderer) Type() reflect.Type {
	return reflect.TypeFor[domain.Renderer]()
}

func (receiver *Renderer) DependsOn() []reflect.Type {
	return nil
}

func (receiver *Renderer) Assembly() (any, error) {
	return infrastructure.NewTemplateHelper(), nil
}

// Service 注册 domain.Service（模板管理服务，经 DAO 访问数据库）。
type Service struct{}

func (receiver *Service) Type() reflect.Type {
	return reflect.TypeFor[domain.Service]()
}

func (receiver *Service) DependsOn() []reflect.Type {
	return []reflect.Type{
		reflect.TypeFor[*gorm.DB](),
	}
}

func (receiver *Service) Assembly() (any, error) {
	return infrastructure.NewTemplateService(), nil
}
