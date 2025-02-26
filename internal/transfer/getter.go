package transfer

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k11v/merch/internal/app"
)

type Transfer struct {
	ID        uuid.UUID
	CreatedAt time.Time
	DstUserID uuid.UUID
	SrcUserID uuid.UUID
	Amount    int

	DstUsername string
	SrcUsername string
}

type Getter struct {
	db *pgxpool.Pool
}

func NewGetter(db *pgxpool.Pool) *Getter {
	return &Getter{db: db}
}

func (g *Getter) GetTransfersByUserID(ctx context.Context, userID uuid.UUID) ([]Transfer, error) {
	transfers, err := getTransfersByUserID(ctx, g.db, userID)
	if err != nil {
		return nil, fmt.Errorf("transfer.Getter: %w", err)
	}
	return transfers, nil
}

func getTransfersByUserID(ctx context.Context, db app.PgxExecutor, userID uuid.UUID) ([]Transfer, error) {
	query := `
		SELECT t.id, t.created_at, t.dst_user_id, t.src_user_id, t.amount,
			   dst_u.username as dst_username,
			   src_u.username as src_username
		FROM transfers t
		LEFT JOIN users dst_u ON t.dst_user_id = dst_u.id
		LEFT JOIN users src_u ON t.src_user_id = src_u.id
		WHERE t.dst_user_id = $1 OR t.src_user_id = $1
	`
	args := []any{userID}

	rows, _ := db.Query(ctx, query, args...)
	transfers, err := pgx.CollectRows(rows, rowToTransferWithUsernames)
	if err != nil {
		return nil, err
	}

	return transfers, nil
}

func rowToTransferWithUsernames(collectable pgx.CollectableRow) (Transfer, error) {
	type row struct {
		ID        uuid.UUID `db:"id"`
		CreatedAt time.Time `db:"created_at"`
		DstUserID uuid.UUID `db:"dst_user_id"`
		SrcUserID uuid.UUID `db:"src_user_id"`
		Amount    int       `db:"amount"`

		DstUsername string `db:"dst_username"`
		SrcUsername string `db:"src_username"`
	}

	collected, err := pgx.RowToStructByName[row](collectable)
	if err != nil {
		return Transfer{}, err
	}

	return Transfer{
		ID:          collected.ID,
		CreatedAt:   collected.CreatedAt,
		DstUserID:   collected.DstUserID,
		SrcUserID:   collected.SrcUserID,
		Amount:      collected.Amount,
		DstUsername: collected.DstUsername,
		SrcUsername: collected.SrcUsername,
	}, nil
}
