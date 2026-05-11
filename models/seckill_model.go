package models

import "time"

// SeckillStockInfo 启动时批量加载秒杀库存用
type SeckillStockInfo struct {
	Stock     int
	BeginTime time.Time
	EndTime   time.Time
}
