package repositories

import (
	"context"
	"database/sql"

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
