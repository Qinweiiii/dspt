package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/qinweiiii/dspt/models"
)

type VoucherRepository struct {
	db *sql.DB
}

func NewVoucherRepository(db *sql.DB) *VoucherRepository {
	return &VoucherRepository{db: db}
}

// ----------------------------------------
// tb_voucher
// ----------------------------------------

// FindByShopID 查询某店铺的所有优惠券（含秒杀信息，LEFT JOIN）
func (r *VoucherRepository) FindByShopID(ctx context.Context, shopID int64) ([]models.Voucher, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT v.id, v.shop_id, v.title, v.sub_title, v.rules,
		       v.pay_value, v.actual_value, v.type, v.status,
		       sv.stock, sv.begin_time, sv.end_time
		FROM tb_voucher v
		LEFT JOIN tb_seckill_voucher sv ON v.id = sv.voucher_id
		WHERE v.shop_id = ? AND v.status = 1`, shopID)
	if err != nil {
		return nil, fmt.Errorf("FindByShopID: %w", err)
	}
	defer rows.Close()

	var vouchers []models.Voucher
	for rows.Next() {
		var v models.Voucher
		var stock sql.NullInt64
		var beginTime, endTime sql.NullTime
		if err := rows.Scan(
			&v.ID, &v.ShopID, &v.Title, &v.SubTitle, &v.Rules,
			&v.PayValue, &v.ActualValue, &v.Type, &v.Status,
			&stock, &beginTime, &endTime,
		); err != nil {
			return nil, fmt.Errorf("FindByShopID scan: %w", err)
		}
		if stock.Valid {
			v.Stock = int(stock.Int64)
		}
		if beginTime.Valid {
			v.BeginTime = beginTime.Time
		}
		if endTime.Valid {
			v.EndTime = endTime.Time
		}
		vouchers = append(vouchers, v)
	}
	return vouchers, rows.Err()
}

// SaveVoucher 新增普通优惠券，返回自增 id
func (r *VoucherRepository) SaveVoucher(ctx context.Context, v *models.Voucher) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO tb_voucher (shop_id, title, sub_title, rules, pay_value, actual_value, type, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		v.ShopID, v.Title, v.SubTitle, v.Rules,
		v.PayValue, v.ActualValue, v.Type, v.Status,
	)
	if err != nil {
		return 0, fmt.Errorf("SaveVoucher: %w", err)
	}
	return res.LastInsertId()
}

// ----------------------------------------
// tb_seckill_voucher
// ----------------------------------------

// SaveSeckillVoucher 新增秒杀库存记录
func (r *VoucherRepository) SaveSeckillVoucher(ctx context.Context, sv *models.SeckillVoucher) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO tb_seckill_voucher (voucher_id, stock, begin_time, end_time) VALUES (?, ?, ?, ?)`,
		sv.VoucherID, sv.Stock, sv.BeginTime, sv.EndTime,
	)
	if err != nil {
		return fmt.Errorf("SaveSeckillVoucher: %w", err)
	}
	return nil
}

// ----------------------------------------
// tb_voucher_order — 事务操作
//
// Go 惯用做法：把"需要在同一事务内执行的逻辑"封装为一个函数，
// 由 repository 负责事务的开启/提交/回滚，调用方只关心业务逻辑。
// 对比 Java 的 @Transactional，这里是显式但干净的。
// ----------------------------------------

// CreateOrderFn 在事务内执行的业务函数签名
// 入参 tx 仅在此函数内使用，不会泄漏到外部
type CreateOrderFn func(ctx context.Context, tx *sql.Tx) error

// WithTx 通用事务包装器：开启 → 执行 fn → 提交/回滚
func (r *VoucherRepository) WithTx(ctx context.Context, fn CreateOrderFn) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	if err := fn(ctx, tx); err != nil {
		tx.Rollback() // 忽略 rollback 错误，原始错误更重要
		return err
	}
	return tx.Commit()
}

// CountOrder 查询某用户对某券的已有订单数（一人一单校验）
func (r *VoucherRepository) CountOrder(ctx context.Context, tx *sql.Tx, userID, voucherID int64) (int64, error) {
	var count int64
	err := tx.QueryRowContext(ctx,
		`SELECT COUNT(1) FROM tb_voucher_order WHERE user_id = ? AND voucher_id = ?`,
		userID, voucherID,
	).Scan(&count)
	return count, err
}

// DecrStock 扣减库存（乐观锁：WHERE stock > 0，返回受影响行数）
func (r *VoucherRepository) DecrStock(ctx context.Context, tx *sql.Tx, voucherID int64) (int64, error) {
	res, err := tx.ExecContext(ctx,
		`UPDATE tb_seckill_voucher SET stock = stock - 1 WHERE voucher_id = ? AND stock > 0`,
		voucherID,
	)
	if err != nil {
		return 0, fmt.Errorf("DecrStock: %w", err)
	}
	return res.RowsAffected()
}

// SaveOrder 写入订单
func (r *VoucherRepository) SaveOrder(ctx context.Context, tx *sql.Tx, order *models.VoucherOrder) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO tb_voucher_order (id, user_id, voucher_id, pay_type, status, create_time)
		 VALUES (?, ?, ?, 1, 1, ?)`,
		order.ID, order.UserID, order.VoucherID, time.Now(),
	)
	if err != nil {
		return fmt.Errorf("SaveOrder: %w", err)
	}
	return nil
}
