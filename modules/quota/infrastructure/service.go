// Package infrastructure 提供 quota 领域端口的 Redis 实现。
package infrastructure

import (
	"context"
	"fmt"
	"time"

	"cnb.cool/mliev/open/go-web/pkg/helper"
	"cnb.cool/mliev/push/message-push/modules/quota/domain"
	"github.com/redis/go-redis/v9"
)

// 确保 QuotaService 实现 domain.Service 端口
var _ domain.Service = (*QuotaService)(nil)

// defaultDailyQuota 默认每日配额上限。
const defaultDailyQuota int64 = 10000

// QuotaService 基于 Redis 的配额服务。
type QuotaService struct {
	redis *redis.Client
}

// New 创建配额服务。
func New() *QuotaService {
	return &QuotaService{redis: helper.GetRedis()}
}

func quotaKey(appID uint) string {
	return fmt.Sprintf("quota:%d:%s", appID, time.Now().Format("20060102"))
}

// Check 使用 Lua 脚本原子校验并计数。
func (s *QuotaService) Check(ctx context.Context, appID uint, dailyLimit int) (bool, error) {
	key := quotaKey(appID)

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

	// TTL 设为 48 小时（考虑跨天情况）
	ttl := 48 * 3600

	result, err := script.Run(ctx, s.redis, []string{key}, dailyLimit, ttl).Result()
	if err != nil {
		return false, err
	}
	return result.(int64) == 1, nil
}

// Increment 增加配额计数（在实际发送后调用）。
func (s *QuotaService) Increment(ctx context.Context, appID uint, count int) error {
	key := quotaKey(appID)
	if _, err := s.redis.IncrBy(ctx, key, int64(count)).Result(); err != nil {
		return err
	}
	s.redis.Expire(ctx, key, 48*time.Hour)
	return nil
}

// GetUsage 获取今日已用量与配额上限。
func (s *QuotaService) GetUsage(ctx context.Context, appID uint) (used int64, limit int64, err error) {
	key := quotaKey(appID)

	usedStr, err := s.redis.Get(ctx, key).Result()
	if err != nil && err != redis.Nil {
		return 0, 0, err
	}

	used = 0
	if usedStr != "" {
		fmt.Sscanf(usedStr, "%d", &used)
	}

	return used, defaultDailyQuota, nil
}
