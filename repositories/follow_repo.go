package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/qinweiiii/dspt/models"
)

type FollowRepository struct {
	db *sql.DB
}

func NewFollowRepository(db *sql.DB) *FollowRepository {
	return &FollowRepository{db: db}
}

// Exists 判断关注关系是否已存在
func (r *FollowRepository) Exists(ctx context.Context, userID, followUserID int64) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(1) FROM tb_follow WHERE user_id = ? AND follow_user_id = ?`,
		userID, followUserID,
	).Scan(&count)
	return count > 0, err
}

// Insert 新增关注记录
func (r *FollowRepository) Insert(ctx context.Context, userID, followUserID int64) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO tb_follow (user_id, follow_user_id) VALUES (?, ?)`,
		userID, followUserID,
	)
	if err != nil {
		return fmt.Errorf("Follow insert: %w", err)
	}
	return nil
}

// Delete 取消关注
func (r *FollowRepository) Delete(ctx context.Context, userID, followUserID int64) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM tb_follow WHERE user_id = ? AND follow_user_id = ?`,
		userID, followUserID,
	)
	if err != nil {
		return fmt.Errorf("Follow delete: %w", err)
	}
	return nil
}

// FindFollowUserIDs 查询 userID 关注的所有 follow_user_id
func (r *FollowRepository) FindFollowUserIDs(ctx context.Context, userID int64) ([]int64, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT follow_user_id FROM tb_follow WHERE user_id = ?`, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("FindFollowUserIDs: %w", err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// FindFanUserIDs 查询关注了 followUserID 的所有粉丝 user_id
// 供 BlogService.SaveBlog 推 Feed 流使用
func (r *FollowRepository) FindFanUserIDs(ctx context.Context, followUserID int64) ([]int64, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT user_id FROM tb_follow WHERE follow_user_id = ?`, followUserID,
	)
	if err != nil {
		return nil, fmt.Errorf("FindFanUserIDs: %w", err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	fmt.Printf("[FindFanUserIDs] followUserID=%d, with the fans=%v\n", followUserID, ids)
	return ids, rows.Err()
}

// FindCommonFollowUserIDs 查询两个用户共同关注的 follow_user_id
// 直接用 INNER JOIN 等价实现
func (r *FollowRepository) FindCommonFollowUserIDs(ctx context.Context, userID1, userID2 int64) ([]int64, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT f1.follow_user_id
		FROM tb_follow f1
		INNER JOIN tb_follow f2 ON f1.follow_user_id = f2.follow_user_id
		WHERE f1.user_id = ? AND f2.user_id = ?`,
		userID1, userID2,
	)
	if err != nil {
		return nil, fmt.Errorf("FindCommonFollowUserIDs: %w", err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// FindUsersByIDs 仅用于内部 — 批量查 Follow 记录（预留扩展）
func (r *FollowRepository) FindUsersByIDs(ctx context.Context, ids []int64) ([]models.Follow, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1]

	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}

	rows, err := r.db.QueryContext(ctx,
		fmt.Sprintf(`SELECT id, user_id, follow_user_id, create_time FROM tb_follow WHERE id IN (%s)`, placeholders),
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("FindUsersByIDs: %w", err)
	}
	defer rows.Close()

	var follows []models.Follow
	for rows.Next() {
		f := models.Follow{}
		if err := rows.Scan(&f.ID, &f.UserID, &f.FollowUserID, &f.CreateTime); err != nil {
			return nil, err
		}
		follows = append(follows, f)
	}
	return follows, rows.Err()
}
