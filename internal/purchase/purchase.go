package purchase

import (
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Purchase struct {
	ID        uuid.UUID
	CreatedAt time.Time
	UserID    uuid.UUID
	ItemID    uuid.UUID
	Amount    int
}

type Row struct {
	ID        uuid.UUID `db:"id"`
	CreatedAt time.Time `db:"created_at"`
	UserID    uuid.UUID `db:"user_id"`
	ItemID    uuid.UUID `db:"item_id"`
	Amount    int       `db:"amount"`
}

func RowToPurchase(collectable pgx.CollectableRow) (*Purchase, error) {
	collected, err := pgx.RowToStructByName[Row](collectable)
	if err != nil {
		return nil, err
	}

	return &Purchase{
		ID:        collected.ID,
		CreatedAt: collected.CreatedAt,
		UserID:    collected.UserID,
		ItemID:    collected.ItemID,
		Amount:    collected.Amount,
	}, nil
}

type ItemCount struct {
	UserID uuid.UUID
	ItemID uuid.UUID
	Count  int

	ItemName string
}

type ItemCountRow struct {
	UserID uuid.UUID `db:"user_id"`
	ItemID uuid.UUID `db:"item_id"`
	Count  int       `db:"count"`

	ItemName string `db:"item_name"`
}

func RowToItemCount(collectable pgx.CollectableRow) (*ItemCount, error) {
	collected, err := pgx.RowToStructByName[ItemCountRow](collectable)
	if err != nil {
		return nil, err
	}

	return &ItemCount{
		UserID:   collected.UserID,
		ItemID:   collected.ItemID,
		Count:    collected.Count,
		ItemName: collected.ItemName,
	}, nil
}
