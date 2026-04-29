package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// CacheClient 封装缓存通用操作，解决缓存穿透、缓存击穿等问题
// 对应 Java 中的 CacheClient.java
type CacheClient struct {
	rdb *redis.Client
}

func NewCacheClient(rdb *redis.Client) *CacheClient {
	return &CacheClient{rdb: rdb}
}

// 缓存空值标记
const emptyCacheValue = ""

// ----------------------------------------
// QueryWithPassThrough 缓存穿透处理
// 对应 Java: queryWithPassThrough
// T: 实体类型
// dbQuery: 数据库查询回调函数
// ----------------------------------------
func QueryWithPassThrough[T any](
	ctx context.Context,
	cc *CacheClient,
	keyPrefix string,
	id interface{},
	ttl time.Duration,
	dbQuery func() (*T, error),
) (*T, error) {
	key := fmt.Sprintf("%s%v", keyPrefix, id)

	// 1. 从 Redis 查询缓存
	jsonStr, err := cc.rdb.Get(ctx, key).Result()
	if err == nil {
		// 命中，且为空值（防止缓存穿透的标记）
		if jsonStr == emptyCacheValue {
			return nil, nil // 返回 nil 表示确实不存在
		}
		// 命中真实数据，反序列化返回
		var data T
		if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
			return nil, fmt.Errorf("QueryWithPassThrough json.Unmarshal: %w", err)
		}
		return &data, nil
	} else if !errors.Is(err, redis.Nil) {
		// 其他 Redis 异常
		return nil, fmt.Errorf("QueryWithPassThrough redis.Get: %w", err)
	}

	// 2. 未命中，查询数据库
	data, err := dbQuery()
	if err != nil {
		return nil, fmt.Errorf("QueryWithPassThrough dbQuery: %w", err)
	}

	// 3. 数据库不存在，写入空值（防缓存穿透），较短过期时间（如 2 分钟）
	if data == nil {
		cc.rdb.Set(ctx, key, emptyCacheValue, 2*time.Minute)
		return nil, nil
	}

	// 4. 数据库存在，写入真实数据
	bytes, _ := json.Marshal(data)
	cc.rdb.Set(ctx, key, string(bytes), ttl)

	return data, nil
}

// ========================================
// 逻辑过期（缓存击穿处理）相关
// ========================================

// RedisData 包装真实数据与逻辑过期时间
// 对应 Java: RedisData.java
type RedisData[T any] struct {
	ExpireTime time.Time `json:"expireTime"`
	Data       T         `json:"data"`
}

// QueryWithLogicalExpire 逻辑过期解决缓存击穿
// 对应 Java: queryWithLogicalExpire
func QueryWithLogicalExpire[T any](
	ctx context.Context,
	cc *CacheClient,
	keyPrefix string,
	id interface{},
	lockKeyPrefix string,
	ttl time.Duration,
	dbQuery func() (*T, error),
) (*T, error) {
	key := fmt.Sprintf("%s%v", keyPrefix, id)

	// 1. 从 Redis 查询缓存
	jsonStr, err := cc.rdb.Get(ctx, key).Result()
	// 如果未命中缓存，直接返回 nil（逻辑过期策略假设热点 key 已经在缓存中，未命中说明非热点或活动未开始）
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("QueryWithLogicalExpire redis.Get: %w", err)
	}

	// 2. 反序列化，检查是否过期
	var redisData RedisData[T]
	if err := json.Unmarshal([]byte(jsonStr), &redisData); err != nil {
		return nil, fmt.Errorf("QueryWithLogicalExpire json.Unmarshal: %w", err)
	}

	// 如果未过期，直接返回真实数据
	if redisData.ExpireTime.After(time.Now()) {
		return &redisData.Data, nil
	}

	// 3. 已过期，尝试获取互斥锁
	lockKey := fmt.Sprintf("%s%v", lockKeyPrefix, id)
	acquired := tryLock(ctx, cc.rdb, lockKey)
	if acquired {
		// 4. 获取到锁，开启独立 goroutine 异步重建缓存
		go func() {
			defer unlock(context.Background(), cc.rdb, lockKey) // 注意用新的 context，因为原请求的 ctx 可能结束了

			// 查询数据库
			data, dbErr := dbQuery()
			if dbErr != nil || data == nil {
				return
			}
			
			// 写入新数据到缓存
			newRedisData := RedisData[T]{
				ExpireTime: time.Now().Add(ttl),
				Data:       *data,
			}
			bytes, _ := json.Marshal(newRedisData)
			// 注意这里的 key 不设置 Redis 层面的 TTL，或者设置很久
			cc.rdb.Set(context.Background(), key, string(bytes), 0)
		}()
	}

	// 5. 无论是否获取到锁，都返回旧数据
	return &redisData.Data, nil
}

// ----------------------------------------
// 互斥锁工具
// ----------------------------------------

func tryLock(ctx context.Context, rdb *redis.Client, key string) bool {
	// setnx 10秒过期
	ok, _ := rdb.SetNX(ctx, key, "1", 10*time.Second).Result()
	return ok
}

func unlock(ctx context.Context, rdb *redis.Client, key string) {
	rdb.Del(ctx, key)
}
