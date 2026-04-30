package utils

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// Locker 分布式锁接口 — 面向接口编程，方便测试 mock
type Locker interface {
	// TryLock 尝试获取锁，返回 unlock 函数和是否成功
	// 调用方拿到 unlock 后 defer unlock() 即可，不需要关心释放细节
	TryLock(ctx context.Context, name string, ttl time.Duration) (unlock func(), ok bool)
}

// RedisLocker 基于 Redis SETNX + Lua 原子释放的分布式锁
type RedisLocker struct {
	rdb *redis.Client
}

func NewRedisLocker(rdb *redis.Client) *RedisLocker {
	return &RedisLocker{rdb: rdb}
}

// unlockScript 原子释放：只删自己持有的锁，防止误删他人锁
// 对应 Java: unlock.lua
var unlockScript = redis.NewScript(`
if redis.call('get', KEYS[1]) == ARGV[1] then
    return redis.call('del', KEYS[1])
end
return 0
`)

// TryLock 尝试获取锁
//
//	lockVal, ok := locker.TryLock(ctx, "order:123", 10*time.Second)
//	if !ok { return errors.New("获取锁失败") }
//	defer unlock()
func (l *RedisLocker) TryLock(ctx context.Context, name string, ttl time.Duration) (unlock func(), ok bool) {
	key := "lock:" + name
	// UUID 作为锁持有者标识，防止进程崩溃重启后误删新持有者的锁
	val := uuid.New().String()

	success, err := l.rdb.SetNX(ctx, key, val, ttl).Result()
	if err != nil || !success {
		return func() {}, false
	}

	unlock = func() {
		// 即使 unlock 失败（网络抖动），锁也会在 TTL 后自动释放
		if err := unlockScript.Run(ctx, l.rdb, []string{key}, val).Err(); err != nil {
			// 记录日志但不 panic —— TTL 兜底
			fmt.Printf("[RedisLocker] unlock failed key=%s: %v\n", key, err)
		}
	}
	return unlock, true
}
