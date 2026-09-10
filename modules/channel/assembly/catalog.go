package assembly

import (
	"reflect"

	"cnb.cool/mliev/open/go-web/pkg/helper"
	"cnb.cool/mliev/push/message-push/modules/channel/domain"
	"cnb.cool/mliev/push/message-push/modules/channel/infrastructure"
	"gorm.io/gorm"
)

// Catalog registers the read-only channel catalog independently of the selector.
type Catalog struct{}

func (*Catalog) Type() reflect.Type { return reflect.TypeFor[domain.CatalogService]() }

func (*Catalog) DependsOn() []reflect.Type {
	return []reflect.Type{reflect.TypeFor[*gorm.DB]()}
}

func (*Catalog) Assembly() (any, error) {
	return infrastructure.NewCatalogService(helper.GetDatabase()), nil
}
