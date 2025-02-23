package main

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrPasswordNotMatch = errors.New("password does not match hash")
	ErrUserExist        = errors.New("user already exists")
	ErrUserNotExist     = errors.New("user does not exist")
)

type User struct {
	ID           uuid.UUID
	Username     string
	PasswordHash string
	Balance      int
}

func getUserByUsername(ctx context.Context, db pgxExecutor, username string) (*User, error) {
	query := `
		SELECT id, username, password_hash, balance
		FROM users
		WHERE username = $1
	`
	args := []any{username}

	rows, _ := db.Query(ctx, query, args...)
	user, err := pgx.CollectExactlyOneRow(rows, rowToUser)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotExist
		}
		return nil, err
	}

	return user, nil
}

func rowToUser(collectable pgx.CollectableRow) (*User, error) {
	type row struct {
		ID           uuid.UUID `db:"id"`
		Username     string    `db:"username"`
		PasswordHash string    `db:"password_hash"`
		Balance      int       `db:"balance"`
	}

	collected, err := pgx.RowToStructByName[row](collectable)
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
