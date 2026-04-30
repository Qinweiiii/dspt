package repositories

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/qinweiiii/dspt/models"
)

// UserRepository 负责 tb_user / tb_user_info 的所有数据库操作
// UserRepository 是一个 “用户仓库”，它里面自带一个数据库连接 db,方法绑定在 UserRepository 上
// 结构体里放 db，方法绑定结构体 → 方法就能直接用 db，不用传参。
type UserRepository struct {
	db *sql.DB
}

// NewUserRepository 构造函数（依赖注入）
func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

// ----------------------------------------
// tb_user 相关操作
// ----------------------------------------

// FindByPhone 根据手机号查用户
func (r *UserRepository) FindByPhone(ctx context.Context, phone string) (*models.User, error) {
	user := &models.User{}
	query := `SELECT id, phone, password, nick_name, icon, create_time, update_time
	          FROM tb_user WHERE phone = ? LIMIT 1`
	err := r.db.QueryRowContext(ctx, query, phone).Scan(
		&user.ID, &user.Phone, &user.Password,
		&user.NickName, &user.Icon,
		&user.CreateTime, &user.UpdateTime,
	)
	if err == sql.ErrNoRows {
		return nil, nil // 不存在返回 nil，与 Java getById 返回 null 一致
	}
	if err != nil {
		return nil, fmt.Errorf("FindByPhone: %w", err)
	}
	return user, nil
}

// FindByID 根据 id 查用户
func (r *UserRepository) FindByID(ctx context.Context, id int64) (*models.User, error) {
	user := &models.User{}
	query := `SELECT id, phone, password, nick_name, icon, create_time, update_time
	          FROM tb_user WHERE id = ? LIMIT 1`
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID, &user.Phone, &user.Password,
		&user.NickName, &user.Icon,
		&user.CreateTime, &user.UpdateTime,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("FindByID: %w", err)
	}
	return user, nil
}

// Create 创建新用户
// 返回数据库自增生成的 id
func (r *UserRepository) Create(ctx context.Context, user *models.User) (int64, error) {
	query := `INSERT INTO tb_user (phone, nick_name, icon) VALUES (?, ?, ?)`
	result, err := r.db.ExecContext(ctx, query, user.Phone, user.NickName, user.Icon)
	if err != nil {
		return 0, fmt.Errorf("Create user: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("Get LastInsertId: %w", err)
	}
	return id, nil
}

// ----------------------------------------
// tb_user_info 相关操作
// ----------------------------------------

// FindInfoByUserID 根据 userId 查用户详情
func (r *UserRepository) FindInfoByUserID(ctx context.Context, userID int64) (*models.UserInfo, error) {
	info := &models.UserInfo{}
	query := `SELECT user_id, city, introduce, fans, followee, gender, birthday, credits, level
	          FROM tb_user_info WHERE user_id = ? LIMIT 1`
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&info.UserID, &info.City, &info.Introduce,
		&info.Fans, &info.Followee, &info.Gender,
		&info.Birthday, &info.Credits, &info.Level,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("FindInfoByUserID: %w", err)
	}
	return info, nil
}

// IncrFans 粉丝数 +1 / -1
func (r *UserRepository) UpdateFans(ctx context.Context, userID int64, delta int) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO tb_user_info (user_id, fans) VALUES (?, ?)
         ON DUPLICATE KEY UPDATE fans = fans + ?`,
		userID, delta, delta,
	)
	return err
}

// UpdateFollowee 关注数 +1 / -1
func (r *UserRepository) UpdateFollowee(ctx context.Context, userID int64, delta int) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO tb_user_info (user_id, followee) VALUES (?, ?)
         ON DUPLICATE KEY UPDATE followee = followee + ?`,
		userID, delta, delta,
	)
	return err
}
