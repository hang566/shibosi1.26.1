package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisCache Redis缓存管理器
type RedisCache struct {
	client *redis.Client
}

// NewRedisCache 创建Redis缓存管理器
func NewRedisCache(addr, password string, db, poolSize int) (*RedisCache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		PoolSize:     poolSize,
		MinIdleConns: poolSize / 4,
		MaxRetries:   1,
		DialTimeout:  3 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolTimeout:  4 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("Redis连接失败: %w", err)
	}

	return &RedisCache{client: client}, nil
}

// Close 关闭Redis连接
func (rc *RedisCache) Close() error {
	return rc.client.Close()
}

// Client 获取Redis客户端
func (rc *RedisCache) Client() *redis.Client {
	return rc.client
}

// Set 设置缓存
func (rc *RedisCache) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("序列化缓存数据失败: %w", err)
	}
	return rc.client.Set(ctx, key, data, expiration).Err()
}

// Get 获取缓存
func (rc *RedisCache) Get(ctx context.Context, key string, dest interface{}) error {
	data, err := rc.client.Get(ctx, key).Bytes()
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

// Delete 删除缓存
func (rc *RedisCache) Delete(ctx context.Context, keys ...string) error {
	return rc.client.Del(ctx, keys...).Err()
}

// DeletePattern 按模式删除缓存
func (rc *RedisCache) DeletePattern(ctx context.Context, pattern string) error {
	iter := rc.client.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		if err := rc.client.Del(ctx, iter.Val()).Err(); err != nil {
			return err
		}
	}
	return iter.Err()
}

// Exists 检查键是否存在
func (rc *RedisCache) Exists(ctx context.Context, keys ...string) (int64, error) {
	return rc.client.Exists(ctx, keys...).Result()
}

// SetNX 设置缓存（仅当键不存在时）
func (rc *RedisCache) SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) (bool, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return false, fmt.Errorf("序列化缓存数据失败: %w", err)
	}
	return rc.client.SetNX(ctx, key, data, expiration).Result()
}

// Incr 自增
func (rc *RedisCache) Incr(ctx context.Context, key string) (int64, error) {
	return rc.client.Incr(ctx, key).Result()
}

// Expire 设置过期时间
func (rc *RedisCache) Expire(ctx context.Context, key string, expiration time.Duration) error {
	return rc.client.Expire(ctx, key, expiration).Err()
}

// TTL 获取剩余过期时间
func (rc *RedisCache) TTL(ctx context.Context, key string) (time.Duration, error) {
	return rc.client.TTL(ctx, key).Result()
}

// 缓存键前缀常量
const (
	KeyUserToken    = "token:user:"      // 用户Token缓存
	KeyUserInfo     = "user:info:"       // 用户信息缓存
	KeyRolePerms    = "role:perms:"      // 角色权限缓存
	KeyMenuTree     = "menu:tree:"       // 菜单树缓存
	KeySystemConfig = "config:"          // 系统配置缓存
	KeyBlacklist    = "token:blacklist:" // Token黑名单
	KeyDashboard    = "dashboard:stats"  // 仪表盘统计缓存
	KeyRateLimit    = "ratelimit:"       // 限流计数
)
