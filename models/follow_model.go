package models

import "time"

// Follow 对应 tb_follow 表
type Follow struct {
	ID           int64     `db:"id"             json:"id,string"`
	UserID       int64     `db:"user_id"        json:"userId,string"`
	FollowUserID int64     `db:"follow_user_id" json:"followUserId,string"`
	CreateTime   time.Time `db:"create_time"    json:"createTime"`
}
