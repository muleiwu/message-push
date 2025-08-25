package assembly

import (
	"sync"

	"cnb.cool/mliev/examples/go-web/internal/interfaces"
	"cnb.cool/mliev/examples/go-web/internal/pkg/database/config"
	"cnb.cool/mliev/examples/go-web/internal/pkg/database/impl"
)

type Database struct {
	Helper interfaces.HelperInterface
}

var (
	databaseOnce sync.Once
)

func (receiver *Database) Assembly() {
	databaseOnce.Do(func() {

		databaseConfig := config.NewConfig(receiver.Helper.GetConfig())

		receiver.Helper.SetDatabase(impl.NewDatabase(receiver.Helper, databaseConfig.Driver, databaseConfig.Host, databaseConfig.Port, databaseConfig.DBName, databaseConfig.Username, databaseConfig.Password))
	})
}
