package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"

	_ "embed"

	"github.com/qinweiiii/dspt/models"
	"github.com/qinweiiii/dspt/repositories"
	"github.com/qinweiiii/dspt/utils"
	"github.com/redis/go-redis/v9"
)

//go:embed seckill.lua
var seckillLua string

// ========================================
// 哨兵错误 — Go 惯用的错误判断方式
// 调用方用 errors.Is() 区分业务错误和系统错误，
// 不需要像 Java 那样靠字符串或自定义异常类型树
// ========================================

var (
	ErrStockInsufficient = errors.New("库存不足")
	ErrDuplicateOrder    = errors.New("禁止重复下单")
)

// ========================================
// VoucherService
// ========================================

type VoucherService struct {
	repo     *repositories.VoucherRepository
	rdb      *redis.Client
	idWorker *utils.IDWorker
	locker   utils.Locker
	bf       *utils.SeckillBloomFilter
}

func NewVoucherService(
	repo *repositories.VoucherRepository,
	rdb *redis.Client,
	idWorker *utils.IDWorker,
	locker utils.Locker,
	bf *utils.SeckillBloomFilter,
) *VoucherService {
	return &VoucherService{
		repo:     repo,
		rdb:      rdb,
		idWorker: idWorker,
		locker:   locker,
		bf:       bf,
	}
}

// QueryVoucherOfShop 查询店铺优惠券列表
func (s *VoucherService) QueryVoucherOfShop(ctx context.Context, shopID int64) (models.Result, error) {
	vouchers, err := s.repo.FindByShopID(ctx, shopID)
	if err != nil {
		return models.Fail("系统异常"), err
	}
	return models.OKData(vouchers), nil
}

// AddVoucher 新增普通优惠券
func (s *VoucherService) AddVoucher(ctx context.Context, v *models.Voucher) (models.Result, error) {
	id, err := s.repo.SaveVoucher(ctx, v)
	if err != nil {
		return models.Fail("系统异常"), err
	}
	v.ID = id
	return models.OKData(id), nil
}

// AddSeckillVoucher 新增秒杀券（DB + Redis 库存）
func (s *VoucherService) AddSeckillVoucher(ctx context.Context, v *models.Voucher) (models.Result, error) {
	id, err := s.repo.SaveVoucher(ctx, v)
	if err != nil {
		return models.Fail("系统异常"), err
	}
	v.ID = id

	sv := &models.SeckillVoucher{
		VoucherID: id,
		Stock:     v.Stock,
		BeginTime: v.BeginTime,
		EndTime:   v.EndTime,
	}

	if err := s.repo.SaveSeckillVoucher(ctx, sv); err != nil {
		return models.Fail("系统异常"), err
	}

	// 写 Redis 库存，供 Lua 脚本原子检查
	if err := s.rdb.Set(ctx, fmt.Sprintf("seckill:stock:%d", id), v.Stock, 0).Err(); err != nil {
		return models.Fail("系统异常"), fmt.Errorf("set seckill stock: %w", err)
	}

	utils.DefaultSeckillCache.SetWindow(id, v.BeginTime, v.EndTime)

	return models.OKData(id), nil
}

// SeckillVoucher 秒杀下单（主入口，纯 Redis 原子判断，异步写 DB）
//
// 请求处理顺序（从快到慢）：
//  1. 本地缓存：售罄标记     → 直接返回，0 网络开销
//  2. 本地缓存：时间窗口检查  → 直接返回，0 网络开销
//  3. Redis Lua 原子操作     → 一次网络 RTT
//  4. Redis Stream 写消息    → 包含在 Lua 里，无额外开销
//  5. MySQL 异步写（消费者）  → 异步，不阻塞响应
func (s *VoucherService) SeckillVoucher(ctx context.Context, voucherID, loginUserID int64) (models.Result, error) {
	// 第一层拦截：本地售罄缓存（最快，不走任何网络）
	if utils.DefaultSeckillCache.IsSoldOut(voucherID) {
		return models.Fail(ErrStockInsufficient.Error()), nil
	}

	// 第二层拦截：本地时间窗口缓存
	// found=false 说明本地没缓存（启动时已有的老券），降级让 Lua 去处理
	if inWindow, found := utils.DefaultSeckillCache.IsInWindow(voucherID); found && !inWindow {
		return models.Fail("活动未开始或已结束"), nil
	}

	// 第三层拦截：Redis Lua 院子判断（库存 + 一人一单 + 写 Stream）
	orderID, err := s.idWorker.NextID(ctx, "order")
	if err != nil {
		return models.Fail("系统异常"), err
	}

	res, err := redis.NewScript(seckillLua).Run(ctx, s.rdb,
		[]string{},
		strconv.FormatInt(voucherID, 10),
		strconv.FormatInt(loginUserID, 10),
		strconv.FormatInt(orderID, 10),
	).Int64()
	if err != nil {
		return models.Fail("系统异常"), fmt.Errorf("seckill lua: %w", err)
	}

	switch res {
	case 1:
		// Lua 返回库存不足时，写入本地售罄缓存，后续请求直接在本地拦截
		utils.DefaultSeckillCache.MarkSoldOut(voucherID)
		return models.Fail(ErrStockInsufficient.Error()), nil
	case 2:
		return models.Fail(ErrDuplicateOrder.Error()), nil
	}

	return models.OKData(orderID), nil
}

// CreateVoucherOrder 写 DB（由消费者调用）
//
// 设计要点：
//   - 事务由 repo.WithTx 管理，service 层只写业务逻辑
//   - 分布式锁在消费者层持有，这里不重复加锁
//   - 一人一单在 DB 层再校验一次（Lua 是预检，DB 是保障）
func (s *VoucherService) CreateVoucherOrder(ctx context.Context, order *models.VoucherOrder) error {
	return s.repo.WithTx(ctx, func(ctx context.Context, tx *sql.Tx) error {
		// 一人一单 DB 兜底
		count, err := s.repo.CountOrder(ctx, tx, order.UserID, order.VoucherID)
		if err != nil {
			return fmt.Errorf("count order: %w", err)
		}
		if count > 0 {
			// 重复消费视为幂等，正常 ACK 即可
			return nil
		}

		// 扣减库存（乐观锁）
		affected, err := s.repo.DecrStock(ctx, tx, order.VoucherID)
		if err != nil {
			return err
		}
		if affected == 0 {
			return ErrStockInsufficient
		}

		return s.repo.SaveOrder(ctx, tx, order)
	})
}
