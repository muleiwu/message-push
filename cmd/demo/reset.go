package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/migration"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type resetOptions struct {
	DBPath    string
	RedisAddr string
	RedisDB   int
}

type resetSummary struct {
	DBPath           string
	Applications     int64
	Channels         int64
	Tasks            int64
	UpstreamMessages int64
}

var ownedRedisPatterns = []string{
	"push:*",
	"quota:*",
	"rate_limit:*",
	"channel_selector:*",
	"channel_weight:*",
	"channel_last_provider:*",
	"wechat:token:*",
	"dingtalk:token:*",
	"oidc:state:*",
	"oidc:ticket:*",
}

func resetDemo(ctx context.Context, opts resetOptions) (*resetSummary, error) {
	target, err := validateDBPath(opts.DBPath)
	if err != nil {
		return nil, err
	}
	if opts.RedisAddr != "" && opts.RedisDB != defaultRedisDB {
		return nil, fmt.Errorf("演示 Redis 清理仅允许 DB %d", defaultRedisDB)
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return nil, fmt.Errorf("创建演示目录失败: %w", err)
	}
	tempFile, err := os.CreateTemp(filepath.Dir(target), ".message-push-demo-*.sqlite")
	if err != nil {
		return nil, fmt.Errorf("创建临时数据库失败: %w", err)
	}
	tempPath := tempFile.Name()
	if err := tempFile.Close(); err != nil {
		return nil, fmt.Errorf("关闭临时数据库占位文件失败: %w", err)
	}
	defer func() {
		_ = os.Remove(tempPath)
		_ = os.Remove(tempPath + "-wal")
		_ = os.Remove(tempPath + "-shm")
	}()

	db, err := gorm.Open(sqlite.Open(tempPath), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, fmt.Errorf("打开临时 SQLite 失败: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("获取 SQLite 连接失败: %w", err)
	}
	closed := false
	defer func() {
		if !closed {
			_ = sqlDB.Close()
		}
	}()

	if err := migration.RunGooseMigrations(sqlDB, "sqlite", quietMigrationLogger{}); err != nil {
		return nil, err
	}
	// 当前管理后台支持按 batch_id 查看批次明细，但生产模型尚未把该
	// 关联列纳入迁移。该列只存在于本地演示库，不改变生产迁移结构。
	if err := db.Exec("ALTER TABLE push_tasks ADD COLUMN batch_id VARCHAR(36)").Error; err != nil {
		return nil, fmt.Errorf("创建演示批次关联列失败: %w", err)
	}
	if err := seedDemo(db, demoAnchor(time.Now())); err != nil {
		return nil, fmt.Errorf("填充演示数据失败: %w", err)
	}
	summary, err := validateDemo(db)
	if err != nil {
		return nil, fmt.Errorf("校验演示数据失败: %w", err)
	}

	if opts.RedisAddr != "" {
		if err := cleanOwnedRedisKeys(ctx, opts.RedisAddr, opts.RedisDB); err != nil {
			return nil, err
		}
	}
	if err := sqlDB.Close(); err != nil {
		return nil, fmt.Errorf("关闭 SQLite 失败: %w", err)
	}
	closed = true

	if err := os.Rename(tempPath, target); err != nil {
		return nil, fmt.Errorf("原子替换演示数据库失败: %w", err)
	}
	summary.DBPath = target
	return summary, nil
}

func validateDBPath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("数据库路径不能为空")
	}
	abs, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", fmt.Errorf("解析数据库路径失败: %w", err)
	}
	ext := strings.ToLower(filepath.Ext(abs))
	if ext != ".sqlite" && ext != ".sqlite3" && ext != ".db" {
		return "", errors.New("数据库文件扩展名必须是 .sqlite、.sqlite3 或 .db")
	}
	if info, statErr := os.Stat(abs); statErr == nil && info.IsDir() {
		return "", errors.New("数据库路径不能是目录")
	} else if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return "", fmt.Errorf("检查数据库路径失败: %w", statErr)
	}
	return abs, nil
}

func demoAnchor(now time.Time) time.Time {
	local := now.In(time.Local)
	return time.Date(local.Year(), local.Month(), local.Day(), 12, 0, 0, 0, time.Local)
}

func cleanOwnedRedisKeys(ctx context.Context, addr string, db int) error {
	client := redis.NewClient(&redis.Options{Addr: addr, DB: db})
	defer client.Close()
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		return fmt.Errorf("连接 Redis %s/%d 失败: %w", addr, db, err)
	}
	for _, pattern := range ownedRedisPatterns {
		var cursor uint64
		for {
			keys, next, err := client.Scan(ctx, cursor, pattern, 200).Result()
			if err != nil {
				return fmt.Errorf("扫描 Redis 键 %q 失败: %w", pattern, err)
			}
			if len(keys) > 0 {
				if err := client.Del(ctx, keys...).Err(); err != nil {
					return fmt.Errorf("清理 Redis 键 %q 失败: %w", pattern, err)
				}
			}
			cursor = next
			if cursor == 0 {
				break
			}
		}
	}
	return nil
}

func validateDemo(db *gorm.DB) (*resetSummary, error) {
	summary := &resetSummary{}
	checks := []struct {
		model any
		value *int64
		min   int64
		name  string
	}{
		{&model.Application{}, &summary.Applications, 2, "应用"},
		{&model.Channel{}, &summary.Channels, 4, "通道"},
		{&model.PushTask{}, &summary.Tasks, 60, "发送任务"},
	}
	for _, check := range checks {
		if err := db.Model(check.model).Count(check.value).Error; err != nil {
			return nil, err
		}
		if *check.value < check.min {
			return nil, fmt.Errorf("%s数量不足: %d < %d", check.name, *check.value, check.min)
		}
	}
	if err := db.Model(&model.CallbackLog{}).Where("type = ?", "upstream").Count(&summary.UpstreamMessages).Error; err != nil {
		return nil, err
	}
	if summary.UpstreamMessages < 2 {
		return nil, errors.New("上行短信假数据不足")
	}
	return summary, nil
}

type quietMigrationLogger struct{}

func (quietMigrationLogger) Info(string)  {}
func (quietMigrationLogger) Warn(string)  {}
func (quietMigrationLogger) Error(string) {}
