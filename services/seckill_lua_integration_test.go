//go:build integration

package services

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/redis/go-redis/v9"
)

func newIntegrationRedis(t *testing.T) *redis.Client {
	t.Helper()

	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "127.0.0.1:6378"
	}
	db := 15
	if rawDB := os.Getenv("REDIS_DB"); rawDB != "" {
		parsed, err := strconv.Atoi(rawDB)
		if err != nil {
			t.Fatalf("invalid REDIS_DB %q: %v", rawDB, err)
		}
		db = parsed
	}
	rdb := redis.NewClient(&redis.Options{Addr: addr, DB: db})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Skipf("Redis integration test skipped; cannot connect to %s db=%d: %v", addr, db, err)
	}
	t.Cleanup(func() {
		_ = rdb.Close()
	})
	return rdb
}

func TestSeckillLuaIntegration(t *testing.T) {
	ctx := context.Background()
	rdb := newIntegrationRedis(t)
	voucherID := int64(990001)
	userID := int64(880001)
	orderID := int64(770001)

	stockKey := fmt.Sprintf("seckill:stock:%d", voucherID)
	orderKey := fmt.Sprintf("seckill:order:%d", voucherID)
	if err := rdb.Del(ctx, stockKey, orderKey, "stream.orders").Err(); err != nil {
		t.Fatalf("cleanup redis keys: %v", err)
	}
	t.Cleanup(func() {
		_ = rdb.Del(ctx, stockKey, orderKey, "stream.orders").Err()
	})
	if err := rdb.Set(ctx, stockKey, 1, 0).Err(); err != nil {
		t.Fatalf("set stock: %v", err)
	}

	res, err := redis.NewScript(seckillLua).Run(ctx, rdb, []string{},
		strconv.FormatInt(voucherID, 10),
		strconv.FormatInt(userID, 10),
		strconv.FormatInt(orderID, 10),
	).Int64()
	if err != nil {
		t.Fatalf("run lua success case: %v", err)
	}
	if res != 0 {
		t.Fatalf("lua result = %d, want 0", res)
	}
	if got, err := rdb.Get(ctx, stockKey).Int(); err != nil || got != 0 {
		t.Fatalf("stock = %d, err = %v; want stock 0", got, err)
	}

	res, err = redis.NewScript(seckillLua).Run(ctx, rdb, []string{},
		strconv.FormatInt(voucherID, 10),
		strconv.FormatInt(userID, 10),
		strconv.FormatInt(orderID+1, 10),
	).Int64()
	if err != nil {
		t.Fatalf("run lua sold-out duplicate case: %v", err)
	}
	if res != 1 {
		t.Fatalf("lua result = %d, want 1 because stock is already 0", res)
	}

	if err := rdb.Set(ctx, stockKey, 1, 0).Err(); err != nil {
		t.Fatalf("reset stock: %v", err)
	}
	res, err = redis.NewScript(seckillLua).Run(ctx, rdb, []string{},
		strconv.FormatInt(voucherID, 10),
		strconv.FormatInt(userID, 10),
		strconv.FormatInt(orderID+2, 10),
	).Int64()
	if err != nil {
		t.Fatalf("run lua duplicate case: %v", err)
	}
	if res != 2 {
		t.Fatalf("lua result = %d, want 2 for duplicate order", res)
	}
}
