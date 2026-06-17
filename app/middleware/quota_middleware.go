package middleware

import (
	"context"
	"fmt"
	"time"

	"cnb.cool/mliev/open/go-web/pkg/helper"
	httpInterfaces "cnb.cool/mliev/open/go-web/pkg/server/http_server/interfaces"
	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/controller"
	"github.com/redis/go-redis/v9"
)

// QuotaMiddleware 配额中间件
func QuotaMiddleware() httpInterfaces.HandlerFunc {
	redisClient := helper.GetRedis()

	return func(c httpInterfaces.RouterContextInterface) {
		appDBID := c.Get("app_db_id")
		if appDBID == nil {
			c.Next()
			return
		}

		// 获取应用的每日配额配置
		var dailyQuota any = c.Get("daily_quota")
		if dailyQuota == nil {
			dailyQuota = 10000 // 默认配额
		}

		// 0 表示不限制
		if dailyQuota.(int) == 0 {
			c.Next()
			return
		}

		// 检查今日配额
		allowed, err := checkQuota(context.Background(), redisClient, appDBID.(uint), dailyQuota.(int))
		if err != nil {
			helper.GetLogger().Error(fmt.Sprintf("quota check error: %v", err))
			c.Next()
			return
		}

		if !allowed {
			controller.FailWithCode(c, constants.CodeQuotaExceeded)
			c.Abort()
			return
		}

		c.Next()
	}
}

// checkQuota 检查配额
func checkQuota(ctx context.Context, client *redis.Client, appID uint, dailyLimit int) (bool, error) {
	today := time.Now().Format("20060102")
	key := fmt.Sprintf("quota:%d:%s", appID, today)

	// 使用 Lua 脚本实现原子操作
	script := redis.NewScript(`
		local key = KEYS[1]
		local limit = tonumber(ARGV[1])
		local ttl = tonumber(ARGV[2])
		
		local current = redis.call('GET', key)
		if current == false then
			redis.call('SET', key, 1, 'EX', ttl)
			return 1
		end
		
		current = tonumber(current)
		if current < limit then
			redis.call('INCR', key)
			return 1
		end
		
		return 0
	`)

	// TTL设为48小时（考虑跨天情况）
	ttl := 48 * 3600

	result, err := script.Run(ctx, client, []string{key}, dailyLimit, ttl).Result()
	if err != nil {
		return false, err
	}

	return result.(int64) == 1, nil
}

// IncrementQuota 增加配额计数（在实际发送后调用）
func IncrementQuota(ctx context.Context, client *redis.Client, appID uint, count int) error {
	today := time.Now().Format("20060102")
	key := fmt.Sprintf("quota:%d:%s", appID, today)

	_, err := client.IncrBy(ctx, key, int64(count)).Result()
	if err != nil {
		return err
	}

	// 设置过期时间
	client.Expire(ctx, key, 48*time.Hour)

	return nil
}

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
