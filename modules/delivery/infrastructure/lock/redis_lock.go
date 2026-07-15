package lock

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

// unlockScript 释放锁脚本：仅当锁仍由自己持有时删除，避免误删其他实例的锁。
var unlockScript = redis.NewScript(`
if redis.call('GET', KEYS[1]) == ARGV[1] then
    return redis.call('DEL', KEYS[1])
end
return 0
`)

// RedisLock 基于 Redis SET NX + TTL 的轻量分布式锁（非可重入、不自动续期）。
// 用于调度器 tick 级互斥：拿不到锁直接跳过本轮，由持锁实例执行。
// 约定：TTL 必须大于单轮工作的最长耗时，由调用方按业务设定。
type RedisLock struct {
	redis *redis.Client
	key   string
	value string
	ttl   time.Duration
}

// NewRedisLock 创建分布式锁。name 用于区分不同调度器的锁键。
func NewRedisLock(redisClient *redis.Client, name string, ttl time.Duration) *RedisLock {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	// 持有者标识：hostname-pid-随机串，保证只有持有者能释放锁
	buf := make([]byte, 4)
	_, _ = rand.Read(buf)

	return &RedisLock{
		redis: redisClient,
		key:   "push:lock:scheduler:" + name,
		value: fmt.Sprintf("%s-%d-%s", hostname, os.Getpid(), hex.EncodeToString(buf)),
		ttl:   ttl,
	}
}

// TryLock 尝试获取锁，成功返回 true；非阻塞。
// Redis 出错时返回 error，调用方应跳过本轮（Redis 不可用时队列本身也无法工作）。
func (l *RedisLock) TryLock(ctx context.Context) (bool, error) {
	return l.redis.SetNX(ctx, l.key, l.value, l.ttl).Result()
}

// Unlock 释放锁，仅当锁仍由自己持有时删除。
func (l *RedisLock) Unlock(ctx context.Context) error {
	return unlockScript.Run(ctx, l.redis, []string{l.key}, l.value).Err()
}
