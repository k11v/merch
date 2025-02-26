package item

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/k11v/merch/internal/app"
)

type Getter struct {
	db app.PgxExecutor
}

func NewGetter(db app.PgxExecutor) *Getter {
	return &Getter{db: db}
}

func (g *Getter) GetItemByName(ctx context.Context, name string) (*Item, error) {
	i, err := getItemByName(ctx, g.db, name)
	if err != nil {
		return nil, fmt.Errorf("item.Getter: %w", err)
	}
	return i, nil
}

func getItemByName(ctx context.Context, db app.PgxExecutor, name string) (*Item, error) {
	query := `
		SELECT id, name, price
		FROM items
		WHERE name = $1
	`
	args := []any{name}

	rows, _ := db.Query(ctx, query, args...)
	item, err := pgx.CollectExactlyOneRow(rows, RowToItem)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotExist
		}
		return nil, err
	}

	return item, nil
}
