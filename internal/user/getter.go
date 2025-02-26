package user

import (
	"context"
	"errors"
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

func (g *Getter) GetUser(ctx context.Context, id uuid.UUID) (*User, error) {
	u, err := getUser(ctx, g.db, id)
	if err != nil {
		return nil, fmt.Errorf("user.Getter: %w", err)
	}
	return u, nil
}

func (g *Getter) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	u, err := getUserByUsername(ctx, g.db, username)
	if err != nil {
		return nil, fmt.Errorf("user.Getter: %w", err)
	}
	return u, nil
}

func getUser(ctx context.Context, db app.PgxExecutor, id uuid.UUID) (*User, error) {
	query := `
		SELECT id, username, password_hash, balance
		FROM users
		WHERE id = $1
	`
	args := []any{id}

	rows, _ := db.Query(ctx, query, args...)
	u, err := pgx.CollectExactlyOneRow(rows, RowToUser)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotExist
		}
		return nil, err
	}

	return u, nil
}

func getUserByUsername(ctx context.Context, db app.PgxExecutor, username string) (*User, error) {
	query := `
		SELECT id, username, password_hash, balance
		FROM users
		WHERE username = $1
	`
	args := []any{username}

	rows, _ := db.Query(ctx, query, args...)
	u, err := pgx.CollectExactlyOneRow(rows, RowToUser)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotExist
		}
		return nil, err
	}

	return u, nil
}
