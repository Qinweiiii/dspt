package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/qinweiiii/dspt/models"
)

type BlogRepository struct {
	db *sql.DB
}

func NewBlogRepository(db *sql.DB) *BlogRepository {
	return &BlogRepository{db: db}
}

const blogColumns = `id, shop_id, user_id, title, images, content, liked, comments, create_time, update_time`

func scanBlog(row interface {
	Scan(...any) error
}) (*models.Blog, error) {
	b := &models.Blog{}
	var liked, comments sql.NullInt64
	err := row.Scan(
		&b.ID, &b.ShopID, &b.UserID,
		&b.Title, &b.Images, &b.Content,
		&liked, &comments,
		&b.CreateTime, &b.UpdateTime,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	b.Liked = int(liked.Int64)
	b.Comments = int(comments.Int64)
	return b, err
}

// FindByID 根据 id 查 Blog
func (r *BlogRepository) FindByID(ctx context.Context, id int64) (*models.Blog, error) {
	query := fmt.Sprintf(`SELECT %s FROM tb_blog WHERE id = ? LIMIT 1`, blogColumns)
	return scanBlog(r.db.QueryRowContext(ctx, query, id))
}

// FindHotPage 热门博客分页（按 liked 降序）
func (r *BlogRepository) FindHotPage(ctx context.Context, offset, size int) ([]*models.Blog, error) {
	query := fmt.Sprintf(`SELECT %s FROM tb_blog ORDER BY liked DESC LIMIT ? OFFSET ?`, blogColumns)
	rows, err := r.db.QueryContext(ctx, query, size, offset)
	if err != nil {
		return nil, fmt.Errorf("FindHotPage: %w", err)
	}
	defer rows.Close()
	return scanBlogs(rows)
}

// FindByUserID 查某用户发布的博客（分页）
func (r *BlogRepository) FindByUserID(ctx context.Context, userID int64, offset, size int) ([]*models.Blog, error) {
	query := fmt.Sprintf(`SELECT %s FROM tb_blog WHERE user_id = ? ORDER BY create_time DESC LIMIT ? OFFSET ?`, blogColumns)
	rows, err := r.db.QueryContext(ctx, query, userID, size, offset)
	if err != nil {
		return nil, fmt.Errorf("FindByUserID: %w", err)
	}
	defer rows.Close()
	return scanBlogs(rows)
}

// FindByIDsOrdered 根据有序 id 列表查 Blog，并保持传入的顺序
func (r *BlogRepository) FindByIDsOrdered(ctx context.Context, ids []int64) ([]*models.Blog, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1]

	// FIELD() 保证返回结果与传入 id 顺序一致
	query := fmt.Sprintf(
		`SELECT %s FROM tb_blog WHERE id IN (%s) ORDER BY FIELD(id, %s)`,
		blogColumns, placeholders, placeholders,
	)

	// ids 需要出现两次：一次 IN，一次 FIELD
	args := make([]any, 0, len(ids)*2)
	for _, id := range ids {
		args = append(args, id)
	}
	for _, id := range ids {
		args = append(args, id)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("FindByIDsOrdered: %w", err)
	}
	defer rows.Close()
	return scanBlogs(rows)
}

// Save 保存 Blog，返回新记录 id
func (r *BlogRepository) Save(ctx context.Context, b *models.Blog) (int64, error) {
	query := `INSERT INTO tb_blog (shop_id, user_id, title, images, content) VALUES (?, ?, ?, ?, ?)`
	res, err := r.db.ExecContext(ctx, query, b.ShopID, b.UserID, b.Title, b.Images, b.Content)
	if err != nil {
		return 0, fmt.Errorf("Save blog: %w", err)
	}
	return res.LastInsertId()
}

// IncrLiked liked 字段 +1
func (r *BlogRepository) IncrLiked(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE tb_blog SET liked = liked + 1 WHERE id = ?`, id)
	return err
}

// DecrLiked liked 字段 -1
func (r *BlogRepository) DecrLiked(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE tb_blog SET liked = liked - 1 WHERE id = ?`, id)
	return err
}

// ----------------------------------------
// 内部工具
// ----------------------------------------

func scanBlogs(rows *sql.Rows) ([]*models.Blog, error) {
	var blogs []*models.Blog
	for rows.Next() {
		b, err := scanBlog(rows)
		if err != nil {
			return nil, err
		}
		blogs = append(blogs, b)
	}
	return blogs, rows.Err()
}
