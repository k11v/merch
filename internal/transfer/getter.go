package transfer

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k11v/merch/internal/app"
)

type Getter struct {
	db *pgxpool.Pool
}

func NewGetter(db *pgxpool.Pool) *Getter {
	return &Getter{db: db}
}

func (g *Getter) GetTransfersByUserID(ctx context.Context, userID uuid.UUID) ([]*Transfer, error) {
	transfers, err := getTransfersByUserID(ctx, g.db, userID)
	if err != nil {
		return nil, fmt.Errorf("transfer.Getter: %w", err)
	}
	return transfers, nil
}

func getTransfersByUserID(ctx context.Context, db app.PgxExecutor, userID uuid.UUID) ([]*Transfer, error) {
	query := `
		SELECT t.id, t.created_at, t.dst_user_id, t.src_user_id, t.amount,
			   dst_u.username as dst_username,
			   src_u.username as src_username
		FROM transfers t
		LEFT JOIN users dst_u ON t.dst_user_id = dst_u.id
		LEFT JOIN users src_u ON t.src_user_id = src_u.id
		WHERE t.dst_user_id = $1 OR t.src_user_id = $1
		ORDER BY t.created_at, t.id
	`
	args := []any{userID}

	rows, _ := db.Query(ctx, query, args...)
	transfers, err := pgx.CollectRows(rows, RowToTransferWithUsernames)
	if err != nil {
		return nil, err
	}

	return transfers, nil
}
