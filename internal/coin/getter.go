package coin

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/k11v/merch/internal/app"
)

var ErrUserNotExist = errors.New("user does not exist")

type User struct {
	ID           uuid.UUID
	Username     string
	PasswordHash string
	Balance      int
}

type Getter struct {
	db app.PgxExecutor
}

func NewGetter(db app.PgxExecutor) *Getter {
	return &Getter{db: db}
}

func (g *Getter) GetBalance(ctx context.Context, userID uuid.UUID) (int, error) {
	user, err := getUser(ctx, g.db, userID)
	if err != nil {
		return 0, err
	}
	return user.Balance, nil
}

func getUser(ctx context.Context, db app.PgxExecutor, id uuid.UUID) (*User, error) {
	query := `
		SELECT id, username, password_hash, balance
		FROM users
		WHERE id = $1
	`
	args := []any{id}

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
