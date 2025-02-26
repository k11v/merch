package user

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/k11v/merch/internal/app"
)

const DefaultBalance = 1000

type Creator struct {
	db app.PgxExecutor
	ph *PasswordHasher
}

func NewCreator(db app.PgxExecutor, ph *PasswordHasher) *Creator {
	return &Creator{db: db, ph: ph}
}

func (c *Creator) CreateUser(ctx context.Context, username string, password string) (*User, error) {
	passwordHash, err := c.ph.Hash(password)
	if err != nil {
		return nil, fmt.Errorf("user.Creator: %w", err)
	}
	u, err := createUser(ctx, c.db, username, passwordHash, DefaultBalance)
	if err != nil {
		return nil, fmt.Errorf("user.Creator: %w", err)
	}
	return u, nil
}

func createUser(ctx context.Context, db app.PgxExecutor, username string, passwordHash string, balance int) (*User, error) {
	query := `
		INSERT INTO users (username, password_hash, balance)
		VALUES ($1, $2, $3)
		RETURNING id, username, password_hash, balance
	`
	args := []any{username, passwordHash, balance}

	rows, _ := db.Query(ctx, query, args...)
	u, err := pgx.CollectExactlyOneRow(rows, RowToUser)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && isConstraintPgError(pgErr, "users_username_idx") {
			return nil, ErrExist
		}
		return nil, err
	}

	return u, nil
}

func isConstraintPgError(e *pgconn.PgError, constraint string) bool {
	return pgerrcode.IsIntegrityConstraintViolation(e.Code) && e.ConstraintName == constraint
}
