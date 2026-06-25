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

// RateLimitMiddleware 限流中间件
func RateLimitMiddleware(qps int) httpInterfaces.HandlerFunc {
	redisClient := helper.GetRedis()

	return func(c httpInterfaces.RouterContextInterface) {
		var appID any = c.Get("app_id")
		if appID == nil {
			// 如果没有app_id，使用IP限流
			appID = c.ClientIP()
		}

		key := fmt.Sprintf("rate_limit:%v", appID)
		ctx := context.Background()

		// 使用令牌桶算法
		allowed, err := checkRateLimit(ctx, redisClient, key, qps)
		if err != nil {
			helper.GetLogger().Error(fmt.Sprintf("rate limit check error: %v", err))
			c.Next()
			return
		}

		if !allowed {
			controller.FailWithCode(c, constants.CodeRateLimitExceeded)
			c.Abort()
			return
		}

		c.Next()
	}
}

// checkRateLimit 检查限流
func checkRateLimit(ctx context.Context, client *redis.Client, key string, qps int) (bool, error) {
	now := time.Now().Unix()
	window := 1 // 1秒窗口

	// 使用 Lua 脚本实现原子操作
	script := redis.NewScript(`
		local key = KEYS[1]
		local limit = tonumber(ARGV[1])
		local window = tonumber(ARGV[2])
		local now = tonumber(ARGV[3])
		
		local current = redis.call('GET', key)
		if current == false then
			redis.call('SET', key, 1, 'EX', window)
			return 1
		end
		
		if tonumber(current) < limit then
			redis.call('INCR', key)
			return 1
		end
		
		return 0
	`)

	result, err := script.Run(ctx, client, []string{key}, qps, window, now).Result()
	if err != nil {
		return false, err
	}

	return result.(int64) == 1, nil
}

// RateLimitByAppIDMiddleware 按AppID限流
func RateLimitByAppIDMiddleware() httpInterfaces.HandlerFunc {
	return func(c httpInterfaces.RouterContextInterface) {
		if c.Get("app_id") == nil {
			c.Next()
			return
		}

		// 从上下文获取该应用的QPS限制
		var rateLimit any = c.Get("rate_limit")
		if rateLimit == nil {
			rateLimit = 100 // 默认100 QPS
		}

		// 0 表示不限制
		if rateLimit.(int) == 0 {
			c.Next()
			return
		}

		// 执行限流检查
		RateLimitMiddleware(rateLimit.(int))(c)
	}
}
