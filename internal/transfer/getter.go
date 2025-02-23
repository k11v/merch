package transfer

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Transfer struct {
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

func (g *Getter) GetTransfersByUserID(ctx context.Context, userID uuid.UUID) ([]*Transfer, error) {
	transactions, err := getTransactionsByUserID(ctx, g.db, userID)
	if err != nil {
		return nil, err
	}
	transfers := make([]*Transfer, 0)
	for _, t := range transactions {
		if t.ToUserID == nil || t.FromUserID == nil {
			continue
		}
		transfers = append(transfers, &Transfer{
			DstUserID:   *t.ToUserID,
			SrcUserID:   *t.FromUserID,
			Amount:      t.Amount,
			DstUsername: *t.ToUsername,
			SrcUsername: *t.FromUsername,
		})
	}
	return transfers, nil
}

func getTransactionsByUserID(ctx context.Context, db pgxExecutor, userID uuid.UUID) ([]*Transaction, error) {
	query := `
		SELECT t.id, t.from_user_id, from_u.username as from_username, t.to_user_id, to_u.username as to_username, t.amount
		FROM transactions t
		LEFT JOIN users from_u ON t.from_user_id = from_u.id
		LEFT JOIN users to_u ON t.to_user_id = to_u.id
		WHERE t.from_user_id = $1 OR t.to_user_id = $1
	`
	args := []any{userID}

	rows, _ := db.Query(ctx, query, args...)
	transactions, err := pgx.CollectRows(rows, rowToTransactionWithUsernames)
	if err != nil {
		return nil, err
	}

	return transactions, nil
}

func rowToTransactionWithUsernames(collectable pgx.CollectableRow) (*Transaction, error) {
	type row struct {
		ID         uuid.UUID  `db:"id"`
		FromUserID *uuid.UUID `db:"from_user_id"`
		ToUserID   *uuid.UUID `db:"to_user_id"`
		Amount     int        `db:"amount"`

		FromUsername *string `db:"from_username"`
		ToUsername   *string `db:"to_username"`
	}

	collected, err := pgx.RowToStructByName[row](collectable)
	if err != nil {
		return nil, err
	}

	return &Transaction{
		ID:           collected.ID,
		FromUserID:   collected.FromUserID,
		ToUserID:     collected.ToUserID,
		Amount:       collected.Amount,
		FromUsername: collected.FromUsername,
		ToUsername:   collected.ToUsername,
	}, nil
}
