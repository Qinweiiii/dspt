package utils

import (
	"context"
	"fmt"
	"log"

	"github.com/redis/go-redis/v9"
)

// SeckillStockLoader 启动时把 tb_seckill_voucher 的 stock 同步到 Redis
//
// Redis Key：seckill:stock:{voucherId}  （与 seckill.lua 和 AddSeckillVoucher 保持一致）
// 幂等：key 已存在则跳过，不覆盖运行时被 Lua 扣减过的值。
type SeckillStockLoader struct {
	rdb *redis.Client
}

func NewSeckillStockLoader(rdb *redis.Client) *SeckillStockLoader {
	return &SeckillStockLoader{rdb: rdb}
}

// StockProvider 供 Loader 查询所有秒杀库存（接口隔离，避免循环依赖）
type StockProvider interface {
	FindSeckillStocks(ctx context.Context) (map[int64]int, error)
}

// Load 把 DB 中 stock > 0 的秒杀券库存写入 Redis
// 幂等：key 已存在时跳过，保证重启不会用 DB 旧值覆盖 Redis 实时值。
func (l *SeckillStockLoader) Load(ctx context.Context, provider StockProvider) error {
	stocks, err := provider.FindSeckillStocks(ctx)
	if err != nil {
		return fmt.Errorf("SeckillStockLoader.Load query: %w", err)
	}
	if len(stocks) == 0 {
		log.Println("[SeckillStockLoader] 没有需要同步的秒杀库存")
		return nil
	}

	for voucherID, stock := range stocks {
		key := fmt.Sprintf("seckill:stock:%d", voucherID)

		// 幂等：key 已存在则跳过（运行时 Lua 已扣减过，不能用 DB 值覆盖）
		exists, err := l.rdb.Exists(ctx, key).Result()
		if err != nil {
			return fmt.Errorf("SeckillStockLoader check key %s: %w", key, err)
		}
		if exists > 0 {
			log.Printf("[SeckillStockLoader] key=%s 已存在，跳过", key)
			continue
		}

		if err := l.rdb.Set(ctx, key, stock, 0).Err(); err != nil {
			return fmt.Errorf("SeckillStockLoader set key %s: %w", key, err)
		}
		log.Printf("[SeckillStockLoader] key=%s 写入 stock=%d", key, stock)
	}
	return nil
}
