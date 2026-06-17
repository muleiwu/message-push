package autoload

import "cnb.cool/mliev/open/go-web/pkg/helper"

type Database struct {
}

func (receiver Database) InitConfig() map[string]any {
	env := helper.GetEnv()
	return map[string]any{
		"database.driver":   env.GetString("database.driver", "mysql"),
		"database.host":     env.GetString("database.host", "127.0.0.1"),
		"database.port":     env.GetInt("database.port", 3306),
		"database.dbname":   env.GetString("database.dbname", "test"),
		"database.username": env.GetString("database.username", "test"),
		"database.password": env.GetString("database.password", "123456"),
	}
}
