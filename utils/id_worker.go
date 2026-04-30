package utils

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// IDWorker 基于 Redis 的全局唯一 ID 生成器
//
// ID 结构（64 bit）：
//
//	┌─────────┬──────────────────────────────┬──────────────────────────────────┐
//	│ 符号位(1)│         时间戳(31 bit)        │          序列号(32 bit)           │
//	└─────────┴──────────────────────────────┴──────────────────────────────────┘
//
// Key 格式：icr:{prefix}:{yyyyMMdd}  — 按天分 key，避免单 key 无限增长
type IDWorker struct {
	rdb            *redis.Client
	beginTimestamp int64 // epoch 偏移量，减小时间戳占用的数值大小
}

const defaultBeginTimestamp int64 = 1640995200 // 2022-01-01 00:00:00 UTC

func NewIDWorker(rdb *redis.Client) *IDWorker {
	return &IDWorker{rdb: rdb, beginTimestamp: defaultBeginTimestamp}
}

// NextID 生成下一个全局唯一 ID，keyPrefix 区分不同业务（如 "order"）
func (w *IDWorker) NextID(ctx context.Context, keyPrefix string) (int64, error) {
	now := time.Now().UTC()
	timestamp := now.Unix() - w.beginTimestamp

	// 按天分 key：icr:order:20240101
	key := fmt.Sprintf("icr:%s:%s", keyPrefix, now.Format("20060102"))
	seq, err := w.rdb.Incr(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("IDWorker.NextID: %w", err)
	}

	// 时间戳左移 32 位，低 32 位放序列号
	return (timestamp << 32) | seq, nil
}
