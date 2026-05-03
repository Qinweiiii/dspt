package repositories

import (
	"context"
	"database/sql"

	"github.com/qinweiiii/dspt/mq"
)

type DeadLetterRepository struct {
	db *sql.DB
}

func NewDeadLetterRepository(db *sql.DB) *DeadLetterRepository {
	return &DeadLetterRepository{db: db}
}

func (r *DeadLetterRepository) SaveDeadLetter(ctx context.Context, dl *mq.DeadLetterOrder) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO tb_dead_letter_order 
		    (msg_id, order_id, user_id, voucher_id, retry_count, last_error)
		VALUES (?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
		    retry_count = VALUES(retry_count),
		    last_error  = VALUES(last_error),
		    update_time = NOW()`,
		dl.MsgID, dl.OrderID, dl.UserID, dl.VoucherID, dl.RetryCount, dl.LastError,
	)
	return err
}
