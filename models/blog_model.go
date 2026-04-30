package models

import "time"

// Blog 对应 tb_blog 表
// @TableField(exist = false) 的字段用 db:"-" 标记，不参与数据库扫描
type Blog struct {
	ID       int64  `db:"id"      json:"id,string"`     // 只输出，不从请求体接收，保留
	ShopID   int64  `db:"shop_id" json:"shopId"`        // 从请求体接收，去掉 ,string ✅
	UserID   int64  `db:"user_id" json:"userId,string"` // 由服务端填充，不从请求体接收，保留
	Title    string `db:"title"   json:"title"`
	Images   string `db:"images"  json:"images"`
	Content  string `db:"content" json:"content"`
	Liked    int    `db:"liked"   json:"liked"`
	Comments int    `db:"comments" json:"comments"`

	CreateTime time.Time `db:"create_time" json:"createTime"`
	UpdateTime time.Time `db:"update_time" json:"updateTime"`

	Icon   string `db:"-" json:"icon"`
	Name   string `db:"-" json:"name"`
	IsLike bool   `db:"-" json:"isLike"`
}

// ScrollResult Feed 流滚动分页返回体
// 对应 Java: ScrollResult.java
type ScrollResult struct {
	List    any   `json:"list"`
	MinTime int64 `json:"minTime"` // 本次返回中最小的时间戳，下次请求用作 max
	Offset  int   `json:"offset"`  // 与 minTime 相同时间戳的元素个数，用于跳过重复
}
