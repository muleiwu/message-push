package assembly

import (
	"sync"

	"cnb.cool/mliev/push/message-push/internal/interfaces"
	"cnb.cool/mliev/push/message-push/internal/pkg/database/config"
	"cnb.cool/mliev/push/message-push/internal/pkg/database/impl"
)

type Database struct {
	Helper interfaces.HelperInterface
}

var (
	databaseOnce sync.Once
)

func (receiver *Database) Assembly() error {
	databaseConfig := config.NewConfig(receiver.Helper.GetConfig())

	database, err := impl.NewDatabase(receiver.Helper, databaseConfig.Driver, databaseConfig.Host, databaseConfig.Port, databaseConfig.DBName, databaseConfig.Username, databaseConfig.Password)

	receiver.Helper.SetDatabase(database)

	return err
}
