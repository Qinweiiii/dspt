package handlers

import (
	"net/http"
	"strconv"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/qinweiiii/dspt/models"
	"golang.org/x/time/rate"
)

// voucherLimiterStore 为每个 voucherID 维护一个独立的令牌桶
// 不同场次的抢票互不干扰：Taylor Swift 的限流不会影响草莓音乐节的正常用户
type voucherLimiterStore struct {
	mu       sync.Mutex
	limiters map[int64]*rate.Limiter
	rps      rate.Limit // 每秒允许的最大请求数
	burst    int        // 桶容量（允许的瞬时并发峰值）
}

func newVoucherLimiterStore(rps float64, burst int) *voucherLimiterStore {
	return &voucherLimiterStore{
		limiters: make(map[int64]*rate.Limiter),
		rps:      rate.Limit(rps),
		burst:    burst,
	}
}

// 全局默认限流器：每个券每秒 500 个请求进入 Redis Lua，瞬时最多 1000
var defaultVoucherLimiter = newVoucherLimiterStore(500, 1000)

func (s *voucherLimiterStore) get(voucherID int64) *rate.Limiter {
	s.mu.Lock()
	defer s.mu.Unlock()
	if l, ok := s.limiters[voucherID]; ok {
		return l
	}
	l := rate.NewLimiter(s.rps, s.burst)
	s.limiters[voucherID] = l
	return l
}

// SeckillRateLimitMiddleware 秒杀接口令牌桶限流
//
// 接入方式（在 VoucherOrderHandler.RegisterRoutes 里）：
//
//	g := r.Group("/voucher-order")
//	g.Use(auth)
//	g.Use(SeckillRateLimitMiddleware())  // ← 加这一行
//	g.POST("/seckill/:voucherId", h.SeckillVoucher)
func SeckillRateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		voucherID, err := strconv.ParseInt(c.Param("voucherId"), 10, 64)
		if err != nil {
			// 解析失败交给 handler 处理，限流层不拦
			c.Next()
			return
		}

		if !defaultVoucherLimiter.get(voucherID).Allow() {
			c.AbortWithStatusJSON(http.StatusOK, models.Fail("系统繁忙，请稍后重试"))
			return
		}
		c.Next()
	}
}
