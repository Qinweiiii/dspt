package models

// Shop 对应 tb_shop 表
type Shop struct {
	ID         int64   `db:"id"          json:"id,string"`
	Name       string  `db:"name"        json:"name"`
	TypeID     int64   `db:"type_id"     json:"typeId"`
	Images     string  `db:"images"      json:"images"`
	Area       string  `db:"area"        json:"area"`
	Address    string  `db:"address"     json:"address"`
	X          float64 `db:"x"           json:"x"`
	Y          float64 `db:"y"           json:"y"`
	AvgPrice   int64   `db:"avg_price"   json:"avgPrice"`
	Sold       int     `db:"sold"        json:"sold"`
	Comments   int     `db:"comments"    json:"comments"`
	Score      int     `db:"score"       json:"score"`
	OpenHours  string  `db:"open_hours"  json:"openHours"`
	CreateTime string  `db:"create_time" json:"createTime"`
	UpdateTime string  `db:"update_time" json:"updateTime"`

	Distance float64 `db:"-" json:"distance,omitempty"` // 用于 GEO 查询返回距离
}

// ShopType 对应 tb_shop_type 表
type ShopType struct {
	ID         int64  `db:"id"          json:"id,string"`
	Name       string `db:"name"        json:"name"`
	Icon       string `db:"icon"        json:"icon"`
	Sort       int    `db:"sort"        json:"sort"`
	CreateTime string `db:"create_time" json:"-"`
	UpdateTime string `db:"update_time" json:"-"`
}

// RedisData 用于逻辑过期
type RedisData struct {
	ExpireTime string `json:"expireTime"` // 简单起见，使用格式化的字符串或直接时间戳
	Data       Shop   `json:"data"`
}
