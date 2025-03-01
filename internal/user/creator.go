package user

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/k11v/merch/internal/app"
)

const DefaultBalance = 1000

type Creator struct {
	dc *DataCreator
	ph *PasswordHasher
}

func NewCreator(db app.PgxExecutor, ph *PasswordHasher) *Creator {
	dc := NewDataCreator(db)
	return &Creator{dc: dc, ph: ph}
}

func (c *Creator) CreateUser(ctx context.Context, username string, password string) (*User, error) {
	passwordHash, err := c.ph.Hash(password)
	if err != nil {
		return nil, fmt.Errorf("user.Creator: %w", err)
	}
	u, err := c.dc.CreateUser(ctx, &DataCreatorCreateUserParams{
		Username:     username,
		PasswordHash: passwordHash,
		Balance:      DefaultBalance,
	})
	if err != nil {
		return nil, fmt.Errorf("user.Creator: %w", err)
	}
	return u, nil
}

// DataCreator creates user data.
// Usually [Creator] should be used to create users.
type DataCreator struct {
	db app.PgxExecutor
}

func NewDataCreator(db app.PgxExecutor) *DataCreator {
	return &DataCreator{db: db}
}

type DataCreatorCreateUserParams struct {
	Username     string
	PasswordHash string
	Balance      int
}

func (dc *DataCreator) CreateUser(ctx context.Context, params *DataCreatorCreateUserParams) (*User, error) {
	query := `
		INSERT INTO users (username, password_hash, balance)
		VALUES ($1, $2, $3)
		RETURNING id, username, password_hash, balance
	`
	args := []any{params.Username, params.PasswordHash, params.Balance}

	rows, _ := dc.db.Query(ctx, query, args...)
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

func (dc *DataCreator) CreateUsers(ctx context.Context, paramsUsers []*DataCreatorCreateUserParams) ([]*User, error) {
	tx, err := dc.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		rollbackErr := tx.Rollback(ctx)
		if rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			slog.Error("didn't rollback transaction", "err", rollbackErr)
		}
	}()

	_, err = tx.Exec(ctx, `
		CREATE TEMPORARY TABLE temp_users (
			username text,
			password_hash text,
			balance int
		)
		ON COMMIT DROP
	`)
	if err != nil {
		return nil, err
	}

	_, err = tx.CopyFrom(
		ctx,
		pgx.Identifier{"temp_users"},
		[]string{"username", "password_hash", "balance"},
		pgx.CopyFromSlice(len(paramsUsers), func(i int) ([]any, error) {
			u := paramsUsers[i]
			return []any{u.Username, u.PasswordHash, u.Balance}, nil
		}),
	)
	if err != nil {
		return nil, err
	}

	rows, _ := tx.Query(ctx, `
		INSERT INTO users (username, password_hash, balance)
		SELECT username, password_hash, balance FROM temp_users
		RETURNING id, username, password_hash, balance
	`)
	users, err := pgx.CollectRows(rows, RowToUser)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && isConstraintPgError(pgErr, "users_username_idx") {
			return nil, ErrExist
		}
		return nil, err
	}

	err = tx.Commit(ctx)
	if err != nil {
		return nil, err
	}

	return users, nil
}

func isConstraintPgError(e *pgconn.PgError, constraint string) bool {
	return pgerrcode.IsIntegrityConstraintViolation(e.Code) && e.ConstraintName == constraint
}
