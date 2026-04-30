package models

import "time"

// Voucher 对应 tb_voucher 表
type Voucher struct {
	ID          int64     `db:"id"           json:"id,string"`
	ShopID      int64     `db:"shop_id"      json:"shopId,string"`
	Title       string    `db:"title"        json:"title"`
	SubTitle    string    `db:"sub_title"    json:"subTitle"`
	Rules       string    `db:"rules"        json:"rules"`
	PayValue    int64     `db:"pay_value"    json:"payValue"`
	ActualValue int64     `db:"actual_value" json:"actualValue"`
	Type        int       `db:"type"         json:"type"`
	Status      int       `db:"status"       json:"status"`
	CreateTime  time.Time `db:"create_time"  json:"-"`
	UpdateTime  time.Time `db:"update_time"  json:"-"`

	// @TableField(exist = false) — 仅用于 addSeckillVoucher 入参，不映射数据库列
	Stock     int       `db:"-" json:"stock"`
	BeginTime time.Time `db:"-" json:"beginTime"`
	EndTime   time.Time `db:"-" json:"endTime"`
}

// SeckillVoucher 对应 tb_seckill_voucher 表
type SeckillVoucher struct {
	VoucherID  int64     `db:"voucher_id"  json:"voucherId,string"`
	Stock      int       `db:"stock"       json:"stock"`
	BeginTime  time.Time `db:"begin_time"  json:"beginTime"`
	EndTime    time.Time `db:"end_time"    json:"endTime"`
	CreateTime time.Time `db:"create_time" json:"-"`
	UpdateTime time.Time `db:"update_time" json:"-"`
}

// VoucherOrder 对应 tb_voucher_order 表
// 对应 Java VoucherOrder.java
type VoucherOrder struct {
	ID         int64     `db:"id"          json:"id,string"`
	UserID     int64     `db:"user_id"     json:"userId,string"`
	VoucherID  int64     `db:"voucher_id"  json:"voucherId,string"`
	PayType    int       `db:"pay_type"    json:"payType"`
	Status     int       `db:"status"      json:"status"`
	CreateTime time.Time `db:"create_time" json:"createTime"`
	PayTime    time.Time `db:"pay_time"    json:"payTime"`
	UseTime    time.Time `db:"use_time"    json:"useTime"`
	RefundTime time.Time `db:"refund_time" json:"refundTime"`
	UpdateTime time.Time `db:"update_time" json:"updateTime"`
}
