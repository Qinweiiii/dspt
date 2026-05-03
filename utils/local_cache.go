package utils

import (
	"sync"
	"time"
)

// SeckillWindowCache 缓存秒杀券的时间窗口和售罄状态

// 设计思路：
//   - 用两个 sync.Map 分别存“时间窗口”和“已售罄标记”
//   - 时间窗口数据几乎不变（券创建后 begin/end 就固定了），永久缓存即可
//   - 售罄标记只写不删（一旦售罄就是售罄），也永久缓存
//   - 这两类数据在本地缓存里命中，就完全不需要打 Redis
//
// 挡住的请求类型：
//   1. 活动未开始 / 已结束的请求 -> 时间窗口检查直接拦
//   2. 已售罄后的持续请求 -> 售罄标记直接拦
//   这两类请求在真实抢票场景里占绝大多数
//
// 注意事项：
//   - 这个缓存是“本地缓存”，每台服务器独立维护，更新不需要通知其他服务器
//   - 由于数据量小且不变，内存占用极小，不考虑过期机制

type SeckillWindowCache struct {
	// windowMap 存 voucherID -> seckillWindow
	windowMap sync.Map

	// soldOutMap 存 voucherID -> struct{} （售罄标记，只写不删）
	soldOutMap sync.Map
}

type seckillWindow struct {
	BeginTime time.Time
	EndTime   time.Time
}

var DefaultSeckillCache = &SeckillWindowCache{}

// SetWindow 缓存某个券的时间窗口（AddSeckillVoucher 时调用）
func (c *SeckillWindowCache) SetWindow(voucherID int64, begin, end time.Time) {
	c.windowMap.Store(voucherID, seckillWindow{BeginTime: begin, EndTime: end})
}

// IsInWindow 判断当前时间是否在秒杀窗口内
// 返回 (inWindow bool, found bool)
// found=false 表示本地没有缓存，调用方需要回源查 DB
func (c *SeckillWindowCache) IsInWindow(voucherID int64) (inWindow bool, found bool) {
	val, ok := c.windowMap.Load(voucherID)
	if !ok {
		return false, false
	}
	w := val.(seckillWindow)
	now := time.Now()
	return now.After(w.BeginTime) && now.Before(w.EndTime), true
}

// MarkSoldOut 标记某个券已售罄（Lua 返回库存不足时调用）
func (c *SeckillWindowCache) MarkSoldOut(voucherID int64) {
	c.soldOutMap.Store(voucherID, struct{}{})
}

// IsSoldOut 判断是否已售罄（本地缓存命中则直接返回，不打 Redis）
func (c *SeckillWindowCache) IsSoldOut(voucherID int64) bool {
	_, ok := c.soldOutMap.Load(voucherID)
	return ok
}
