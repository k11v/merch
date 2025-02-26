package item

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k11v/merch/internal/app"
)

type UserItemCount struct {
	UserID uuid.UUID
	ItemID uuid.UUID
	Count  int

	ItemName string
}

type UserItem struct {
	UserID   uuid.UUID
	ItemID   uuid.UUID
	ItemName string
	Amount   int
}

type Getter struct {
	db *pgxpool.Pool
}

func NewGetter(db *pgxpool.Pool) *Getter {
	return &Getter{db: db}
}

func (g *Getter) GetUserItemCountsByUserID(ctx context.Context, userID uuid.UUID) ([]*UserItemCount, error) {
	userItems, err := getUserItems(ctx, g.db, userID)
	if err != nil {
		return nil, err
	}
	userItemCounts := make([]*UserItemCount, 0)
	for _, ui := range userItems {
		userItemCounts = append(userItemCounts, &UserItemCount{
			UserID:   ui.UserID,
			ItemID:   ui.ItemID,
			Count:    ui.Amount,
			ItemName: ui.ItemName,
		})
	}
	return userItemCounts, nil
}

func getUserItems(ctx context.Context, db app.PgxExecutor, userID uuid.UUID) ([]*UserItem, error) {
	query := `
		SELECT ui.user_id, ui.item_id, i.name AS item_name, ui.amount
		FROM users_items ui
		JOIN items i ON ui.item_id = i.id
		WHERE ui.user_id = $1
	`
	args := []any{userID}

	rows, _ := db.Query(ctx, query, args...)
	userItems, err := pgx.CollectRows(rows, rowToUserItemWithName)
	if err != nil {
		return nil, err
	}

	return userItems, nil
}

func rowToUserItemWithName(collectable pgx.CollectableRow) (*UserItem, error) {
	type row struct {
		UserID uuid.UUID `db:"user_id"`
		ItemID uuid.UUID `db:"item_id"`
		Amount int       `db:"amount"`

		ItemName string `db:"item_name"`
	}

	collected, err := pgx.RowToStructByName[row](collectable)
	if err != nil {
		return nil, err
	}

	return &UserItem{
		UserID:   collected.UserID,
		ItemID:   collected.ItemID,
		Amount:   collected.Amount,
		ItemName: collected.ItemName,
	}, nil
}
