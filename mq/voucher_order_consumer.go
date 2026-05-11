package mq

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/qinweiiii/dspt/models"
	"github.com/qinweiiii/dspt/utils"
	"github.com/redis/go-redis/v9"
)

// ========================================
// OrderCreator 接口 — 解耦 mq 包与 services 包
//
// Go 惯用做法：依赖接口而非具体类型。
// mq 包不 import services 包，双方只通过这个小接口交互。
// 测试时可以轻松 mock。
// ========================================

type OrderCreator interface {
	CreateVoucherOrder(ctx context.Context, order *models.VoucherOrder) error
}

// DeadLetterOrder 死信记录
type DeadLetterOrder struct {
	MsgID      string
	OrderID    int64
	UserID     int64
	VoucherID  int64
	RetryCount int
	LastError  string
}

// DeadLetterWriter 死信写入接口，解耦 mq 包与 DB 操作
type DeadLetterWriter interface {
	SaveDeadLetter(ctx context.Context, dl *DeadLetterOrder) error
}

// ========================================
// VoucherOrderConsumer
// ========================================

const (
	streamName   = "stream.orders"
	groupName    = "g1"
	consumerName = "c1"
	blockTimeout = 2 * time.Second
	retryDelay   = 20 * time.Millisecond

	// maxRetry 超过这个次数就放弃重试，写入死信表
	// 设 3 次：网络抖动通常 1~2 次就恢复，3 次还失败说明是真的有问题
	maxRetry = 3
)

type VoucherOrderConsumer struct {
	rdb        *redis.Client
	creator    OrderCreator
	locker     utils.Locker
	deadWriter DeadLetterWriter // ★ 新增：死信写入器
}

func NewVoucherOrderConsumer(rdb *redis.Client, creator OrderCreator, locker utils.Locker, deadWriter DeadLetterWriter) *VoucherOrderConsumer {
	return &VoucherOrderConsumer{rdb: rdb, creator: creator, locker: locker, deadWriter: deadWriter}
}

// Start 启动消费者，ctx 取消时优雅退出
//
// 对比 Java 的 @PostConstruct + while(true)，Go 版本：
//  1. 通过 ctx 支持优雅退出（配合 main 的 signal 处理）
//  2. 消费错误时自动进入 pending list 补偿，逻辑更清晰
//
// 调用：go consumer.Start(ctx)
func (c *VoucherOrderConsumer) Start(ctx context.Context) {
	log.Println("✅ VoucherOrderConsumer 启动，监听", streamName)
	for {
		select {
		case <-ctx.Done():
			log.Println("VoucherOrderConsumer 收到停止信号，退出")
			return
		default:
			if err := c.consume(ctx); err != nil {
				log.Printf("[Consumer] 消费异常: %v，进入 pending list 补偿", err)
				c.drainPendingList(ctx)
			}
		}
	}
}

// consume 读取一批最新消息并处理
func (c *VoucherOrderConsumer) consume(ctx context.Context) error {
	msgs, err := c.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    groupName,
		Consumer: consumerName,
		Streams:  []string{streamName, ">"},
		Count:    1,
		Block:    blockTimeout,
	}).Result()

	if err == redis.Nil {
		return nil // 正常超时，无新消息
	}
	if err != nil {
		// ctx 已取消时不打印多余日志
		if ctx.Err() != nil {
			return nil
		}
		return err
	}

	for _, stream := range msgs {
		for _, msg := range stream.Messages {
			if err := c.handleWithRetry(ctx, msg); err != nil {
				return err // 触发 pending list 补偿
			}
		}
	}
	return nil
}

// drainPendingList 消费所有未 ACK 的 pending 消息
func (c *VoucherOrderConsumer) drainPendingList(ctx context.Context) {
	for {
		msgs, err := c.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    groupName,
			Consumer: consumerName,
			Streams:  []string{streamName, "0"}, // "0" = 读 pending list
			Count:    1,
		}).Result()

		if err != nil || len(msgs) == 0 {
			return
		}

		empty := true
		for _, stream := range msgs {
			for _, msg := range stream.Messages {
				empty = false
				if err := c.handle(ctx, msg); err != nil {
					log.Printf("[PendingList] 重试失败 id=%s: %v", msg.ID, err)
					time.Sleep(retryDelay)
					return // 下次循环重试
				}
			}
		}
		if empty {
			return
		}
	}
}

// ★ handleWithRetry：在原来 handle 基础上加重试计数和死信降级
//
// 核心改动逻辑：
//  1. 先查这条消息已经被 pending 了多少次（XPENDING 返回的 delivery-count）
//  2. 超过 maxRetry → 写死信表 + ACK（不再阻塞队列）
//  3. 未超过 → 正常处理，失败则留在 pending list 等下次补偿
func (c *VoucherOrderConsumer) handleWithRetry(ctx context.Context, msg redis.XMessage) error {
	// 查这条消息的投递次数
	deliveryCount, err := c.getDeliveryCount(ctx, msg.ID)
	if err != nil {
		log.Printf("[Consumer] 查询投递次数失败 id=%s: %v，按正常处理", msg.ID, err)
		// 查询失败不影响正常流程，降级为正常处理
		return c.handle(ctx, msg)
	}

	// 超过最大重试次数 → 写死信
	if deliveryCount > maxRetry {
		log.Printf("[Consumer] 消息 id=%s 已投递 %d 次，超过上限，写入死信表", msg.ID, deliveryCount)
		c.moveToDeadLetter(ctx, msg, deliveryCount, "超过最大重试次数")
		// ACK 掉，不再阻塞后续消息
		c.ack(ctx, msg.ID)
		return nil
	}

	return c.handle(ctx, msg)
}

// getDeliveryCount 通过 XPENDING 查询某条消息的投递次数
func (c *VoucherOrderConsumer) getDeliveryCount(ctx context.Context, msgID string) (int64, error) {
	// XPENDING stream group - + 10 consumer
	// 只取这一条消息的信息
	pending, err := c.rdb.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: streamName,
		Group:  groupName,
		Start:  msgID,
		End:    msgID,
		Count:  1,
	}).Result()
	if err != nil {
		return 0, err
	}
	if len(pending) == 0 {
		return 0, nil
	}
	return pending[0].RetryCount, nil
}

// moveToDeadLetter 把失败消息写入死信表
func (c *VoucherOrderConsumer) moveToDeadLetter(ctx context.Context, msg redis.XMessage, retryCount int64, reason string) {
	order, err := parseOrder(msg.Values)
	if err != nil {
		log.Printf("[DeadLetter] 解析消息失败 id=%s，无法写死信: %v", msg.ID, err)
		return
	}

	dl := &DeadLetterOrder{
		MsgID:      msg.ID,
		OrderID:    order.ID,
		UserID:     order.UserID,
		VoucherID:  order.VoucherID,
		RetryCount: int(retryCount),
		LastError:  reason,
	}

	if err := c.deadWriter.SaveDeadLetter(ctx, dl); err != nil {
		// 死信写入失败只打日志，不影响主流程（消息还是会被 ACK 掉）
		log.Printf("[DeadLetter] 写入死信表失败 id=%s: %v", msg.ID, err)
		return
	}
	log.Printf("[DeadLetter] 消息 id=%s 已写入死信表，人工处理", msg.ID)
}

// handle 处理单条消息（逻辑不变）
func (c *VoucherOrderConsumer) handle(ctx context.Context, msg redis.XMessage) error {
	order, err := parseOrder(msg.Values)
	if err != nil {
		log.Printf("[Consumer] 解析消息失败 id=%s: %v，直接 ACK 丢弃", msg.ID, err)
		c.ack(ctx, msg.ID) // 无法解析的消息直接丢，不阻塞队列
		return nil
	}

	// 分布式锁（兜底，Lua 已做过一人一单预检）
	unlock, ok := c.locker.TryLock(ctx, fmt.Sprintf("order:%d", order.UserID), 10*time.Second)
	if !ok {
		return fmt.Errorf("获取锁失败 userId=%d", order.UserID)
	}
	defer unlock()

	if err := c.creator.CreateVoucherOrder(ctx, order); err != nil {
		return err
	}

	c.ack(ctx, msg.ID)
	return nil
}

func (c *VoucherOrderConsumer) ack(ctx context.Context, msgID string) {
	if err := c.rdb.XAck(ctx, streamName, groupName, msgID).Err(); err != nil {
		log.Printf("[Consumer] XACK 失败 id=%s: %v", msgID, err)
	}
}

// parseOrder 从 Stream 消息 values 解析 VoucherOrder
func parseOrder(values map[string]interface{}) (*models.VoucherOrder, error) {
	get := func(key string) (int64, error) {
		v, ok := values[key]
		if !ok {
			return 0, fmt.Errorf("missing field %q", key)
		}
		s, ok := v.(string)
		if !ok {
			return 0, fmt.Errorf("field %q is not string", key)
		}
		return strconv.ParseInt(s, 10, 64)
	}

	userID, err := get("userId")
	if err != nil {
		return nil, err
	}
	voucherID, err := get("voucherId")
	if err != nil {
		return nil, err
	}
	orderID, err := get("id")
	if err != nil {
		return nil, err
	}

	return &models.VoucherOrder{
		ID:        orderID,
		UserID:    userID,
		VoucherID: voucherID,
	}, nil
}
