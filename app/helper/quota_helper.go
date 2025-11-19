package helper

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// GetQuotaUsage 获取配额使用情况
func GetQuotaUsage(ctx context.Context, client *redis.Client, appID uint) (used int64, limit int64, err error) {
	today := time.Now().Format("20060102")
	key := fmt.Sprintf("quota:%d:%s", appID, today)

	usedStr, err := client.Get(ctx, key).Result()
	if err != nil && err != redis.Nil {
		return 0, 0, err
	}

	used = 0
	if usedStr != "" {
		fmt.Sscanf(usedStr, "%d", &used)
	}

	limit = 10000 // 默认配额

	return used, limit, nil
}
