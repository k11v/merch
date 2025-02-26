package user

import (
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrExist    = errors.New("already exists")
	ErrNotExist = errors.New("does not exist")
)

type User struct {
	ID           uuid.UUID
	Username     string
	PasswordHash string
	Balance      int
}

type Row struct {
	ID           uuid.UUID `db:"id"`
	Username     string    `db:"username"`
	PasswordHash string    `db:"password_hash"`
	Balance      int       `db:"balance"`
}

func RowToUser(collectable pgx.CollectableRow) (*User, error) {
	collected, err := pgx.RowToStructByName[Row](collectable)
	if err != nil {
		return nil, err
	}
	return &User{
		ID:           collected.ID,
		Username:     collected.Username,
		PasswordHash: collected.PasswordHash,
		Balance:      collected.Balance,
	}, nil
}
