package user

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/k11v/merch/internal/app"
)

const InitialBalance = 1000

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
	u, err := createUser(ctx, c.db, username, passwordHash, InitialBalance)
	if err != nil {
		return nil, fmt.Errorf("user.Creator: %w", err)
	}
	return u, nil
}

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

func createUser(ctx context.Context, db app.PgxExecutor, username string, passwordHash string, balance int) (*User, error) {
	query := `
		INSERT INTO users (username, password_hash, balance)
		VALUES ($1, $2, $3)
		RETURNING id, username, password_hash, balance
	`
	args := []any{username, passwordHash, balance}

	rows, _ := db.Query(ctx, query, args...)
	u, err := pgx.CollectExactlyOneRow(rows, rowToUser)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && isConstraintPgError(pgErr, "users_username_idx") {
			return nil, ErrExist
		}
		return nil, err
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
	u, err := pgx.CollectExactlyOneRow(rows, rowToUser)
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
	u, err := pgx.CollectExactlyOneRow(rows, rowToUser)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotExist
		}
		return nil, err
	}

	return u, nil
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

func isConstraintPgError(e *pgconn.PgError, constraint string) bool {
	return pgerrcode.IsIntegrityConstraintViolation(e.Code) && e.ConstraintName == constraint
}
