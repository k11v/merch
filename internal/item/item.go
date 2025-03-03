package item

import (
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrNotExist = errors.New("does not exist")

type Item struct {
	ID    uuid.UUID
	Name  string
	Price int
}

func RowToItem(collectable pgx.CollectableRow) (*Item, error) {
	type row struct {
		ID    uuid.UUID `db:"id"`
		Name  string    `db:"name"`
		Price int       `db:"price"`
	}

	collected, err := pgx.RowToStructByName[row](collectable)
	if err != nil {
		return nil, err
	}

	return &Item{
		ID:    collected.ID,
		Name:  collected.Name,
		Price: collected.Price,
	}, nil
}
