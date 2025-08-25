package impl

import (
	"context"
	"fmt"
	"sync"
	"time"

	"cnb.cool/mliev/examples/go-web/internal/interfaces"
	"github.com/redis/go-redis/v9"
)

type Redis struct {
	client      *redis.Client
	initialized bool
	initOnce    sync.Once
	initError   error
}

func getOptions(host string, port int, db int, password string) *redis.Options {
	return &redis.Options{
		Addr:     fmt.Sprintf("%s:%d", host, port),
		Password: password,
		DB:       db,
	}
}

func NewRedis(Helper interfaces.HelperInterface, host string, port int, db int, password string) *Redis {

	r := &Redis{}
	r.initOnce.Do(func() {
		// 创建Redis客户端
		r.client = redis.NewClient(getOptions(host, port, db, password))

		// 测试连接
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := r.client.Ping(ctx).Err(); err != nil {
			r.initError = err
			return
		}

		r.initialized = true
	})

	return r
}

// 基础连接方法
func (r *Redis) GetClient() *redis.Client {
	return r.client
}

func (r *Redis) SetClient(client *redis.Client) {
	r.client = client
}

func (r *Redis) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

func (r *Redis) Close() error {
	return r.client.Close()
}

// 字符串操作
func (r *Redis) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return r.client.Set(ctx, key, value, expiration).Err()
}

func (r *Redis) Get(ctx context.Context, key string) (string, error) {
	return r.client.Get(ctx, key).Result()
}

func (r *Redis) GetSet(ctx context.Context, key string, value interface{}) (string, error) {
	return r.client.GetSet(ctx, key, value).Result()
}

func (r *Redis) SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) (bool, error) {
	return r.client.SetNX(ctx, key, value, expiration).Result()
}

func (r *Redis) SetEX(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return r.client.SetEx(ctx, key, value, expiration).Err()
}

func (r *Redis) Del(ctx context.Context, keys ...string) (int64, error) {
	return r.client.Del(ctx, keys...).Result()
}

func (r *Redis) Exists(ctx context.Context, keys ...string) (int64, error) {
	return r.client.Exists(ctx, keys...).Result()
}

func (r *Redis) Expire(ctx context.Context, key string, expiration time.Duration) (bool, error) {
	return r.client.Expire(ctx, key, expiration).Result()
}

func (r *Redis) TTL(ctx context.Context, key string) (time.Duration, error) {
	return r.client.TTL(ctx, key).Result()
}

// 列表操作
func (r *Redis) LPush(ctx context.Context, key string, values ...interface{}) (int64, error) {
	return r.client.LPush(ctx, key, values...).Result()
}

func (r *Redis) RPush(ctx context.Context, key string, values ...interface{}) (int64, error) {
	return r.client.RPush(ctx, key, values...).Result()
}

func (r *Redis) LPop(ctx context.Context, key string) (string, error) {
	return r.client.LPop(ctx, key).Result()
}

func (r *Redis) RPop(ctx context.Context, key string) (string, error) {
	return r.client.RPop(ctx, key).Result()
}

func (r *Redis) LLen(ctx context.Context, key string) (int64, error) {
	return r.client.LLen(ctx, key).Result()
}

func (r *Redis) LRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return r.client.LRange(ctx, key, start, stop).Result()
}

// 哈希操作
func (r *Redis) HSet(ctx context.Context, key string, values ...interface{}) (int64, error) {
	return r.client.HSet(ctx, key, values...).Result()
}

func (r *Redis) HGet(ctx context.Context, key, field string) (string, error) {
	return r.client.HGet(ctx, key, field).Result()
}

func (r *Redis) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return r.client.HGetAll(ctx, key).Result()
}

func (r *Redis) HDel(ctx context.Context, key string, fields ...string) (int64, error) {
	return r.client.HDel(ctx, key, fields...).Result()
}

func (r *Redis) HExists(ctx context.Context, key, field string) (bool, error) {
	return r.client.HExists(ctx, key, field).Result()
}

func (r *Redis) HLen(ctx context.Context, key string) (int64, error) {
	return r.client.HLen(ctx, key).Result()
}

// 集合操作
func (r *Redis) SAdd(ctx context.Context, key string, members ...interface{}) (int64, error) {
	return r.client.SAdd(ctx, key, members...).Result()
}

func (r *Redis) SRem(ctx context.Context, key string, members ...interface{}) (int64, error) {
	return r.client.SRem(ctx, key, members...).Result()
}

func (r *Redis) SMembers(ctx context.Context, key string) ([]string, error) {
	return r.client.SMembers(ctx, key).Result()
}

func (r *Redis) SIsMember(ctx context.Context, key string, member interface{}) (bool, error) {
	return r.client.SIsMember(ctx, key, member).Result()
}

func (r *Redis) SCard(ctx context.Context, key string) (int64, error) {
	return r.client.SCard(ctx, key).Result()
}

// 有序集合操作
func (r *Redis) ZAdd(ctx context.Context, key string, members ...redis.Z) (int64, error) {
	return r.client.ZAdd(ctx, key, members...).Result()
}

func (r *Redis) ZRem(ctx context.Context, key string, members ...interface{}) (int64, error) {
	return r.client.ZRem(ctx, key, members...).Result()
}

func (r *Redis) ZRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return r.client.ZRange(ctx, key, start, stop).Result()
}

func (r *Redis) ZRangeWithScores(ctx context.Context, key string, start, stop int64) ([]redis.Z, error) {
	return r.client.ZRangeWithScores(ctx, key, start, stop).Result()
}

func (r *Redis) ZScore(ctx context.Context, key, member string) (float64, error) {
	return r.client.ZScore(ctx, key, member).Result()
}

func (r *Redis) ZCard(ctx context.Context, key string) (int64, error) {
	return r.client.ZCard(ctx, key).Result()
}

// 发布订阅
func (r *Redis) Publish(ctx context.Context, channel string, message interface{}) (int64, error) {
	return r.client.Publish(ctx, channel, message).Result()
}

func (r *Redis) Subscribe(ctx context.Context, channels ...string) *redis.PubSub {
	return r.client.Subscribe(ctx, channels...)
}

// 事务
func (r *Redis) TxPipeline() redis.Pipeliner {
	return r.client.TxPipeline()
}

func (r *Redis) Pipeline() redis.Pipeliner {
	return r.client.Pipeline()
}

// 分布式锁
func (r *Redis) SetNXWithExpire(ctx context.Context, key string, value interface{}, expiration time.Duration) (bool, error) {
	return r.client.SetNX(ctx, key, value, expiration).Result()
}

// 计数器
func (r *Redis) Incr(ctx context.Context, key string) (int64, error) {
	return r.client.Incr(ctx, key).Result()
}

func (r *Redis) Decr(ctx context.Context, key string) (int64, error) {
	return r.client.Decr(ctx, key).Result()
}

func (r *Redis) IncrBy(ctx context.Context, key string, value int64) (int64, error) {
	return r.client.IncrBy(ctx, key, value).Result()
}

func (r *Redis) DecrBy(ctx context.Context, key string, value int64) (int64, error) {
	return r.client.DecrBy(ctx, key, value).Result()
}
