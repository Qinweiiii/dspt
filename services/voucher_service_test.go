package services

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/qinweiiii/dspt/models"
	"github.com/qinweiiii/dspt/repositories"
	"github.com/redis/go-redis/v9"
)

type fakeVoucherRepo struct{}

func (fakeVoucherRepo) FindByShopID(ctx context.Context, shopID int64) ([]models.Voucher, error) {
	return nil, nil
}

func (fakeVoucherRepo) SaveVoucher(ctx context.Context, v *models.Voucher) (int64, error) {
	return 0, nil
}

func (fakeVoucherRepo) SaveSeckillVoucher(ctx context.Context, sv *models.SeckillVoucher) error {
	return nil
}

func (fakeVoucherRepo) WithTx(ctx context.Context, fn repositories.CreateOrderFn) error {
	return nil
}

func (fakeVoucherRepo) CountOrder(ctx context.Context, tx *sql.Tx, userID, voucherID int64) (int64, error) {
	return 0, nil
}

func (fakeVoucherRepo) DecrStock(ctx context.Context, tx *sql.Tx, voucherID int64) (int64, error) {
	return 0, nil
}

func (fakeVoucherRepo) SaveOrder(ctx context.Context, tx *sql.Tx, order *models.VoucherOrder) error {
	return nil
}

type fixedIDWorker struct {
	id  int64
	err error
}

func (w fixedIDWorker) NextID(ctx context.Context, keyPrefix string) (int64, error) {
	return w.id, w.err
}

type fakeEligibility struct {
	allowed bool
	exists  bool
}

func (e fakeEligibility) IsAllowed(ctx context.Context, voucherID, userID int64) (bool, bool) {
	return e.allowed, e.exists
}

func newTestVoucherService(t *testing.T, eligibility seckillEligibility) (*VoucherService, *miniredis.Miniredis, *redis.Client) {
	t.Helper()

	srv := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: srv.Addr()})
	t.Cleanup(func() {
		_ = rdb.Close()
	})

	return NewVoucherServiceWithDeps(
		fakeVoucherRepo{},
		rdb,
		fixedIDWorker{id: 9001},
		nil,
		eligibility,
	), srv, rdb
}

func TestSeckillVoucherSuccessWritesStreamAndDeductsStock(t *testing.T) {
	ctx := context.Background()
	svc, redisSrv, rdb := newTestVoucherService(t, nil)
	redisSrv.Set("seckill:stock:101", "2")

	result, err := svc.SeckillVoucher(ctx, 101, 501)
	if err != nil {
		t.Fatalf("SeckillVoucher returned error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success result, got %#v", result)
	}
	if got, err := redisSrv.Get("seckill:stock:101"); err != nil || got != "1" {
		t.Fatalf("stock = %s, want 1", got)
	}
	if ok, err := redisSrv.SIsMember("seckill:order:101", "501"); err != nil || !ok {
		t.Fatalf("expected user to be recorded in seckill order set")
	}
	if got, err := rdb.XLen(ctx, "stream.orders").Result(); err != nil || got != 1 {
		t.Fatalf("stream length = %d, want 1", got)
	}
}

func TestSeckillVoucherRejectsDuplicateOrder(t *testing.T) {
	ctx := context.Background()
	svc, redisSrv, _ := newTestVoucherService(t, nil)
	redisSrv.Set("seckill:stock:102", "2")
	redisSrv.SAdd("seckill:order:102", "502")

	result, err := svc.SeckillVoucher(ctx, 102, 502)
	if err != nil {
		t.Fatalf("SeckillVoucher returned error: %v", err)
	}
	if result.Success || result.ErrMsg != ErrDuplicateOrder.Error() {
		t.Fatalf("expected duplicate order failure, got %#v", result)
	}
	if got, err := redisSrv.Get("seckill:stock:102"); err != nil || got != "2" {
		t.Fatalf("stock = %s, want unchanged 2", got)
	}
}

func TestSeckillVoucherRejectsSoldOut(t *testing.T) {
	ctx := context.Background()
	svc, redisSrv, _ := newTestVoucherService(t, nil)
	redisSrv.Set("seckill:stock:103", "0")

	result, err := svc.SeckillVoucher(ctx, 103, 503)
	if err != nil {
		t.Fatalf("SeckillVoucher returned error: %v", err)
	}
	if result.Success || result.ErrMsg != ErrStockInsufficient.Error() {
		t.Fatalf("expected stock failure, got %#v", result)
	}
}

func TestSeckillVoucherRejectsIneligibleUserBeforeLua(t *testing.T) {
	ctx := context.Background()
	svc, redisSrv, rdb := newTestVoucherService(t, fakeEligibility{allowed: false, exists: true})
	redisSrv.Set("seckill:stock:104", "2")

	result, err := svc.SeckillVoucher(ctx, 104, 504)
	if err != nil {
		t.Fatalf("SeckillVoucher returned error: %v", err)
	}
	if result.Success || result.ErrMsg != "无抢购资格" {
		t.Fatalf("expected eligibility failure, got %#v", result)
	}
	if got, err := redisSrv.Get("seckill:stock:104"); err != nil || got != "2" {
		t.Fatalf("stock = %s, want unchanged 2", got)
	}
	if got, err := rdb.XLen(ctx, "stream.orders").Result(); err != nil && err != redis.Nil || got != 0 {
		t.Fatalf("stream length = %d, want 0", got)
	}
}

func TestSeckillVoucherIDWorkerFailure(t *testing.T) {
	ctx := context.Background()
	srv := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: srv.Addr()})
	t.Cleanup(func() {
		_ = rdb.Close()
	})
	srv.Set("seckill:stock:105", "2")

	wantErr := errors.New("id worker down")
	svc := NewVoucherServiceWithDeps(fakeVoucherRepo{}, rdb, fixedIDWorker{err: wantErr}, nil, nil)
	result, err := svc.SeckillVoucher(ctx, 105, 505)
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
	if result.Success || result.ErrMsg != "系统异常" {
		t.Fatalf("expected system failure result, got %#v", result)
	}
}
