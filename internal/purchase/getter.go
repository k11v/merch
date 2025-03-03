package purchase

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/k11v/merch/internal/app"
)

type Getter struct {
	db app.PgxExecutor
}

func NewGetter(db app.PgxExecutor) *Getter {
	return &Getter{db: db}
}

func (g *Getter) GetItemCountsByUserID(ctx context.Context, userID uuid.UUID) ([]*ItemCount, error) {
	itemCounts, err := getItemCountsByUserID(ctx, g.db, userID)
	if err != nil {
		return nil, fmt.Errorf("purchase.Getter: %w", err)
	}
	return itemCounts, nil
}

func getItemCountsByUserID(ctx context.Context, db app.PgxExecutor, userID uuid.UUID) ([]*ItemCount, error) {
	query := `
		SELECT p.user_id, p.item_id, count(*) AS count, i.name AS item_name
		FROM purchases p
		JOIN items i ON p.item_id = i.id
		WHERE p.user_id = $1
		GROUP BY user_id, item_id, item_name
		ORDER BY item_name, item_id
	`
	args := []any{userID}

	rows, _ := db.Query(ctx, query, args...)
	itemCounts, err := pgx.CollectRows(rows, RowToItemCount)
	if err != nil {
		return nil, err
	}

	return itemCounts, nil
}
