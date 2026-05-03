package utils

import (
	"context"
	"fmt"
	"log"
	"strconv"

	"github.com/redis/go-redis/v9"
)

// SeckillBloomFilter 布隆过滤器

// 解决的问题：
// 	在真实抢票场景里，大量请求来自“根本抢不到”的用户——
// 	比如没有购票资格的账号（未实名、被风控标记）在用脚本刷接口
// 	布隆过滤器在 Redis Lua 之前一刀切掉这些请求。

// 实现方案：
// 	使用 Redis Stack 的 BF（Bloom Filter）模块，命令是 BF.ADD / BF.EXISTS。
// 	key 格式：seckill:bf:{voucherId}
// 	存储内容：有资格参与该券抢购的 userID

// 使用前提：
// 	Redis 需要安装 RedisBloom 模块（Redis Stack 默认包含）。
// 	如果用的是普通 Redis，见下方“降级方案”。

// 误判率说明：
// 	布隆过滤器存在误判（把合法用户判为非法），但不存在漏判（非法用户一定被拦）。
// 	实际抢票场景里，误判率设 0.01%（1/10000），几乎可以忽略不计。
// 	真正有资格的用户极少数被误拦，可以让他们刷新重试，体验影响可接受。

type SeckillBloomFilter struct {
	rdb *redis.Client
}

func NewSeckillBloomFilter(rdb *redis.Client) *SeckillBloomFilter {
	return &SeckillBloomFilter{rdb: rdb}
}

func bloomKey(voucherID int64) string {
	return fmt.Sprintf("seckill:bf:%d", voucherID)
}

// Init 初始化某个券的布隆过滤器
// capacity=预计有资格的用户数, errorRate=误判率
// 在 AddSeckillVoucher 时调用
func (bf *SeckillBloomFilter) Init(ctx context.Context, voucherID int64, capacity int64, errorRate float64) error {
	key := bloomKey(voucherID)
	// BF.RESERVE key error_rate capacity
	// 如果 key 已存在会报错，用 EXIST 先判断
	exists, err := bf.rdb.Exists(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("BF init check: %w", err)
	}
	if exists > 0 {
		return nil // 已存在，幂等跳过
	}

	// 用 Do 直接发 BF.RESERVE 命令
	if err := bf.rdb.Do(ctx, "BF.RESERVE", key, errorRate, capacity).Err(); err != nil {
		return fmt.Errorf("BF.RESERVE: %w", err)
	}
	log.Printf("[BloomFilter] voucherID=%d 初始化完成, capacity=%d, errorRate=%.4f", voucherID, capacity, errorRate)
	return nil
}

// AddUser 把有购票资格的 userID 加入布隆过滤器
// 比如：完成实名认证、购买了会员资格、系统白名单等
func (bf *SeckillBloomFilter) AddUser(ctx context.Context, voucherID, userID int64) error {
	key := bloomKey(voucherID)
	if err := bf.rdb.Do(ctx, "BF.ADD", key, strconv.FormatInt(userID, 10)).Err(); err != nil {
		return fmt.Errorf("BF.ADD: %w", err)
	}
	return nil
}

// IsAllowed 判断用户是否有资格参与抢购
// 返回 (allowed bool, exists bool)
// exists=false 说明这个券根本没有初始化布隆过滤器（全员可抢的券），直接放行
func (bf *SeckillBloomFilter) IsAllowed(ctx context.Context, voucherID, userID int64) (allowed bool, exists bool) {
	key := bloomKey(voucherID)

	// 先确认这个券有没有开启布隆过滤器（没开启 = 全员可抢，直接放行）
	keyExists, err := bf.rdb.Exists(ctx, key).Result()
	if err != nil || keyExists == 0 {
		return true, false // 没有过滤器，全员放行
	}

	result, err := bf.rdb.Do(ctx, "BF.EXISTS", key, strconv.FormatInt(userID, 10)).Int()
	if err != nil {
		// Redis 故障时降级放行，不影响正常用户
		log.Printf("[BloomFilter] BF.EXISTS 失败，降级放行: %v", err)
		return true, true
	}
	return result == 1, true
}

// ======================================================
// 降级方案：如果 Redis 没有 BF 模块，用 Redis SET 模拟
// 用 SADD seckill:whitelist:{voucherID} userID 做白名单
// 精确无误判，但内存占用比布隆过滤器大 5-10 倍
// ======================================================

type SeckillWhitelistFallback struct {
	rdb *redis.Client
}

func NewSeckillWhitelistFallback(rdb *redis.Client) *SeckillWhitelistFallback {
	return &SeckillWhitelistFallback{rdb: rdb}
}

func whitelistKey(voucherID int64) string {
	return fmt.Sprintf("seckill:whitelist:%d", voucherID)
}

func (w *SeckillWhitelistFallback) AddUser(ctx context.Context, voucherID, userID int64) error {
	return w.rdb.SAdd(ctx, whitelistKey(voucherID), strconv.FormatInt(userID, 10)).Err()
}

func (w *SeckillWhitelistFallback) IsAllowed(ctx context.Context, voucherID, userID int64) bool {
	exists, err := w.rdb.Exists(ctx, whitelistKey(voucherID)).Result()
	if err != nil || exists == 0 {
		return true // 没有白名单 = 全员可抢
	}
	ok, err := w.rdb.SIsMember(ctx, whitelistKey(voucherID), strconv.FormatInt(userID, 10)).Result()
	if err != nil {
		return true // 故障降级放行
	}
	return ok
}
