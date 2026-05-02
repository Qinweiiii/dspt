package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/qinweiiii/dspt/models"
)

type ShopRepository struct {
	db *sql.DB
}

func NewShopRepository(db *sql.DB) *ShopRepository {
	return &ShopRepository{db: db}
}

func (r *ShopRepository) FindByID(ctx context.Context, id int64) (*models.Shop, error) {
	shop := &models.Shop{}
	query := `SELECT id, name, type_id, images, area, address, x, y, avg_price, sold, comments, score, open_hours FROM tb_shop WHERE id = ?`
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&shop.ID, &shop.Name, &shop.TypeID, &shop.Images,
		&shop.Area, &shop.Address, &shop.X, &shop.Y,
		&shop.AvgPrice, &shop.Sold, &shop.Comments, &shop.Score, &shop.OpenHours,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return shop, err
}

func (r *ShopRepository) Update(ctx context.Context, shop *models.Shop) error {
	query := `UPDATE tb_shop SET name=?, type_id=?, images=?, area=?, address=?, x=?, y=?, avg_price=?, open_hours=? WHERE id=?`
	_, err := r.db.ExecContext(ctx, query, shop.Name, shop.TypeID, shop.Images, shop.Area, shop.Address, shop.X, shop.Y, shop.AvgPrice, shop.OpenHours, shop.ID)
	return err
}

func (r *ShopRepository) Save(ctx context.Context, shop *models.Shop) (int64, error) {
	query := `INSERT INTO tb_shop (name, type_id, images, area, address, x, y, avg_price, open_hours)
	         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	res, err := r.db.ExecContext(ctx, query,
		shop.Name, shop.TypeID, shop.Images, shop.Area, shop.Address,
		shop.X, shop.Y, shop.AvgPrice, shop.OpenHours,
	)
	if err != nil {
		return 0, fmt.Errorf("Save shop: %w", err)
	}
	return res.LastInsertId()
}

func (r *ShopRepository) FindAllShopTypes(ctx context.Context) ([]models.ShopType, error) {
	query := `SELECT id, name, icon, sort FROM tb_shop_type ORDER BY sort ASC`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var types []models.ShopType
	for rows.Next() {
		var t models.ShopType
		if err := rows.Scan(&t.ID, &t.Name, &t.Icon, &t.Sort); err != nil {
			return nil, err
		}
		types = append(types, t)
	}
	return types, nil
}

// FindByTypeID 按商铺类型分页查询（无 GEO，纯 DB 分页）
func (r *ShopRepository) FindByTypeID(ctx context.Context, typeID, offset, size int) ([]*models.Shop, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, type_id, images, area, address, x, y, avg_price, sold, comments, score, open_hours
		 FROM tb_shop WHERE type_id = ? LIMIT ? OFFSET ?`,
		typeID, size, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("FindByTypeID: %w", err)
	}
	defer rows.Close()
	return scanShops(rows)
}

// FindByName 按名称模糊查询分页
func (r *ShopRepository) FindByName(ctx context.Context, name string, offset, size int) ([]*models.Shop, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, type_id, images, area, address, x, y, avg_price, sold, comments, score, open_hours
		 FROM tb_shop WHERE name LIKE ? LIMIT ? OFFSET ?`,
		"%"+name+"%", size, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("FindByName: %w", err)
	}
	defer rows.Close()
	return scanShops(rows)
}

// FindByIDsOrdered 按给定 id 顺序批量查询（保持 GEO 返回的距离排序）
func (r *ShopRepository) FindByIDsOrdered(ctx context.Context, ids []int64) ([]*models.Shop, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1]

	query := fmt.Sprintf(
		`SELECT id, name, type_id, images, area, address, x, y, avg_price, sold, comments, score, open_hours
		 FROM tb_shop WHERE id IN (%s) ORDER BY FIELD(id, %s)`,
		placeholders, placeholders,
	)

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
	return scanShops(rows)
}

// FindAllWithGeo 查询所有有坐标的商铺（供 GEO 数据导入使用）
func (r *ShopRepository) FindAllWithGeo(ctx context.Context) ([]*models.Shop, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, type_id, images, area, address, x, y, avg_price, sold, comments, score, open_hours
		 FROM tb_shop WHERE x IS NOT NULL AND y IS NOT NULL`,
	)
	if err != nil {
		return nil, fmt.Errorf("FindAllWithGeo: %w", err)
	}
	defer rows.Close()
	return scanShops(rows)
}

// ----------------------------------------
// 内部工具
// ----------------------------------------

func scanShops(rows *sql.Rows) ([]*models.Shop, error) {
	var shops []*models.Shop
	for rows.Next() {
		s := &models.Shop{}
		if err := rows.Scan(
			&s.ID, &s.Name, &s.TypeID, &s.Images,
			&s.Area, &s.Address, &s.X, &s.Y,
			&s.AvgPrice, &s.Sold, &s.Comments, &s.Score, &s.OpenHours,
		); err != nil {
			return nil, fmt.Errorf("scanShops: %w", err)
		}
		shops = append(shops, s)
	}
	return shops, rows.Err()
}
