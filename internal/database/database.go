// Package database owns the application's database connection policy.
// All database sessions and GORM-generated timestamps use UTC.
package database

import (
	"fmt"
	"net"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"

	"cnb.cool/mliev/open/go-web/pkg/container"
	"cnb.cool/mliev/open/go-web/pkg/interfaces"
	webdbconfig "cnb.cool/mliev/open/go-web/pkg/server/database/config"
	"github.com/glebarez/sqlite"
	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/muleiwu/gsr"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const mysqlSessionTimeZone = "'+00:00'"

type Config struct {
	Driver   string
	Host     string
	Port     int
	DBName   string
	Username string
	Password string
	Charset  string
}

func FromProvider(provider gsr.Provider) Config {
	return Config{
		Driver:   provider.GetString("database.driver", "mysql"),
		Host:     provider.GetString("database.host", "127.0.0.1"),
		Port:     provider.GetInt("database.port", 3306),
		DBName:   provider.GetString("database.dbname", "test"),
		Username: provider.GetString("database.username", "test"),
		Password: provider.GetString("database.password", "123456"),
		Charset:  provider.GetString("database.charset", "utf8mb4"),
	}
}

func MySQLConfig(cfg Config) (*mysqlDriver.Config, error) {
	host, err := webdbconfig.NormalizeTCPHost(cfg.Host)
	if err != nil {
		return nil, err
	}
	if err := webdbconfig.ValidateTCPPort(cfg.Port); err != nil {
		return nil, err
	}

	driverCfg := mysqlDriver.NewConfig()
	driverCfg.User = cfg.Username
	driverCfg.Passwd = cfg.Password
	driverCfg.Net = "tcp"
	driverCfg.Addr = net.JoinHostPort(host, strconv.Itoa(cfg.Port))
	driverCfg.DBName = cfg.DBName
	driverCfg.ParseTime = true
	driverCfg.Loc = time.UTC
	driverCfg.MultiStatements = true
	driverCfg.Params = map[string]string{"time_zone": mysqlSessionTimeZone}
	driverCfg.AllowAllFiles = false
	driverCfg.AllowCleartextPasswords = false
	driverCfg.AllowFallbackToPlaintext = false
	driverCfg.AllowOldPasswords = false
	charset := cfg.Charset
	if charset == "" {
		charset = "utf8mb4"
	}
	if err := driverCfg.Apply(mysqlDriver.Charset(charset, "")); err != nil {
		return nil, err
	}
	return driverCfg, nil
}

func PostgreSQLDSN(cfg Config) (string, error) {
	host, err := webdbconfig.NormalizeTCPHost(cfg.Host)
	if err != nil {
		return "", err
	}
	if err := webdbconfig.ValidateTCPPort(cfg.Port); err != nil {
		return "", err
	}
	return (&url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(cfg.Username, cfg.Password),
		Host:   net.JoinHostPort(host, strconv.Itoa(cfg.Port)),
		Path:   cfg.DBName,
		RawQuery: url.Values{
			"sslmode":  []string{"disable"},
			"TimeZone": []string{"UTC"},
		}.Encode(),
	}).String(), nil
}

func GORMConfig() *gorm.Config {
	return &gorm.Config{NowFunc: func() time.Time { return time.Now().UTC() }}
}

func Open(cfg Config) (*gorm.DB, error) {
	switch cfg.Driver {
	case "mysql", "mariadb", "":
		driverCfg, err := MySQLConfig(cfg)
		if err != nil {
			return nil, fmt.Errorf("build MySQL connection config: %w", err)
		}
		return gorm.Open(mysql.New(mysql.Config{DSNConfig: driverCfg}), GORMConfig())
	case "postgres", "postgresql":
		dsn, err := PostgreSQLDSN(cfg)
		if err != nil {
			return nil, fmt.Errorf("build PostgreSQL connection config: %w", err)
		}
		return gorm.Open(postgres.New(postgres.Config{DSN: dsn, PreferSimpleProtocol: true}), GORMConfig())
	case "sqlite", "sqlite3":
		return gorm.Open(sqlite.Open(sqliteUTCDataSource(cfg.Host)), GORMConfig())
	case "memory":
		return gorm.Open(sqlite.Open(sqliteUTCDataSource(":memory:")), GORMConfig())
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", cfg.Driver)
	}
}

func sqliteUTCDataSource(dataSource string) string {
	separator := "?"
	if strings.Contains(dataSource, "?") {
		separator = "&"
	}
	return dataSource + separator + "_time_format=sqlite&_timezone=UTC"
}

// Assembly provides the UTC-configured *gorm.DB to the go-web container.
type Assembly struct{}

var _ interfaces.AssemblyInterface = (*Assembly)(nil)

func (*Assembly) Type() reflect.Type { return reflect.TypeFor[*gorm.DB]() }

func (*Assembly) DependsOn() []reflect.Type {
	return []reflect.Type{reflect.TypeFor[gsr.Provider]()}
}

func (*Assembly) Assembly() (any, error) {
	provider := container.MustGet[gsr.Provider]()
	return Open(FromProvider(provider))
}
