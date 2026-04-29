package models

import "time"

// ========================================
// 数据库实体
// ========================================

// User 对应 tb_user 表
type User struct {
	ID         int64     `db:"id"`
	Phone      string    `db:"phone"`
	Password   string    `db:"password"`
	NickName   string    `db:"nick_name"`
	Icon       string    `db:"icon"`
	CreateTime time.Time `db:"create_time"`
	UpdateTime time.Time `db:"update_time"`
}

// UserInfo 对应 tb_user_info 表
type UserInfo struct {
	UserID     int64     `db:"user_id"    json:"userId"`
	City       string    `db:"city"       json:"city"`
	Introduce  string    `db:"introduce"  json:"introduce"`
	Fans       int       `db:"fans"       json:"fans"`
	Followee   int       `db:"followee"   json:"followee"`
	Gender     bool      `db:"gender"     json:"gender"`
	Birthday   string    `db:"birthday"   json:"birthday"` // DATE 类型用 string 存
	Credits    int       `db:"credits"    json:"credits"`
	Level      int       `db:"level"      json:"level"`
	CreateTime time.Time `db:"create_time" json:"-"` // 不暴露给前端
	UpdateTime time.Time `db:"update_time" json:"-"`
}

// ========================================
// DTO
// ========================================

// UserDTO
// 存入 Redis Hash 和 ThreadLocal（gin.Context）的精简用户信息，避免暴露敏感字段
type UserDTO struct {
	ID       int64  `json:"id,string"` // ,string 防止 JS 大数精度丢失
	NickName string `json:"nickName"`
	Icon     string `json:"icon"`
}

// LoginFormDTO
// 登录请求体：手机号 + 验证码（或密码）
type LoginFormDTO struct {
	Phone    string `json:"phone"    binding:"required"` // 这个字段必须传
	Code     string `json:"code"`
	Password string `json:"password"`
}

// ========================================
// 统一响应体
// ========================================

// Result
// omitempty: 空字段不序列化
type Result struct {
	Success bool   `json:"success"`
	ErrMsg  string `json:"errorMsg,omitempty"`
	Data    any    `json:"data,omitempty"`
	Total   int64  `json:"total,omitempty"`
}

// OK 对应 Result.ok()
func OK() Result { return Result{Success: true} }

// OKData 对应 Result.ok(data)
func OKData(data any) Result { return Result{Success: true, Data: data} }

// OKPage 对应 Result.ok(list, total)
func OKPage(data any, total int64) Result { return Result{Success: true, Data: data, Total: total} }

// Fail 对应 Result.fail(errorMsg)
func Fail(msg string) Result { return Result{Success: false, ErrMsg: msg} }
