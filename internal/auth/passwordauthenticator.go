package auth

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/k11v/merch/internal/app"
)

const InitialBalance = 1000

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

// Data represents authenticated data.
type Data struct {
	UserID uuid.UUID
}

type PasswordAuthenticator struct {
	db             app.PgxExecutor
	passwordHasher *PasswordHasher
}

func NewPasswordAuthenticator(db app.PgxExecutor, passwordHasher *PasswordHasher) *PasswordAuthenticator {
	return &PasswordAuthenticator{db: db, passwordHasher: passwordHasher}
}

func (pa *PasswordAuthenticator) AuthenticatePassword(ctx context.Context, username, password string) (*Data, error) {
	user, err := getUserByUsername(ctx, pa.db, username)
	switch {
	case err == nil:
		err = pa.passwordHasher.Verify(password, user.PasswordHash)
		if err != nil {
			return nil, err
		}
	case errors.Is(err, ErrUserNotExist):
		var passwordHash string
		passwordHash, err = pa.passwordHasher.Hash(password)
		if err != nil {
			return nil, err
		}
		user, err = createUser(ctx, pa.db, username, passwordHash, InitialBalance)
		if errors.Is(err, ErrUserExist) {
			// When two callers try to authenticate under the same username for
			// the first time, both of them can fail getUserByUsername with
			// ErrUserNotExist but only one of them can succeed createUser.
			user, err = getUserByUsername(ctx, pa.db, username)
			if err != nil {
				return nil, err
			}
		} else if err != nil {
			return nil, err
		}
	default:
		return nil, err
	}
	return &Data{UserID: user.ID}, nil
}

func createUser(ctx context.Context, db app.PgxExecutor, username string, passwordHash string, balance int) (*User, error) {
	query := `
		INSERT INTO users (username, password_hash, balance)
		VALUES ($1, $2, $3)
		RETURNING id, username, password_hash, balance
	`
	args := []any{username, passwordHash, balance}

	rows, _ := db.Query(ctx, query, args...)
	user, err := pgx.CollectExactlyOneRow(rows, rowToUser)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && isConstraintPgError(pgErr, "users_username_idx") {
			return nil, ErrUserExist
		}
		return nil, err
	}

	return user, nil
}

func getUserByUsername(ctx context.Context, db app.PgxExecutor, username string) (*User, error) {
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

func isConstraintPgError(e *pgconn.PgError, constraint string) bool {
	return pgerrcode.IsIntegrityConstraintViolation(e.Code) && e.ConstraintName == constraint
}
