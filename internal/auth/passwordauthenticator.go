package auth

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
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

type Transaction struct {
	ID         uuid.UUID
	FromUserID *uuid.UUID
	ToUserID   *uuid.UUID
	Amount     int

	FromUsername *string
	ToUsername   *string
}

// Data represents authenticated data.
type Data struct {
	UserID uuid.UUID
}

type PasswordAuthenticator struct {
	db             *pgxpool.Pool
	passwordHasher *PasswordHasher
}

func NewPasswordAuthenticator(db *pgxpool.Pool, passwordHasher *PasswordHasher) *PasswordAuthenticator {
	return &PasswordAuthenticator{db: db, passwordHasher: passwordHasher}
}

func (pa *PasswordAuthenticator) AuthenticatePassword(ctx context.Context, username, password string) (*Data, error) {
	// HACK: Race condition.
	user, err := getUserByUsername(ctx, pa.db, username)
	if err == nil {
		err = pa.passwordHasher.Verify(password, user.PasswordHash)
		if err != nil {
			return nil, err
		}
	} else if errors.Is(err, ErrUserNotExist) {
		tx, err := pa.db.Begin(ctx)
		if err != nil {
			return nil, err
		}
		defer func() {
			err = tx.Rollback(ctx)
			if err != nil && !errors.Is(err, pgx.ErrTxClosed) {
				slog.Error("didn't rollback", "err", err)
			}
		}()

		initialBalance := 1000
		passwordHash, err := pa.passwordHasher.Hash(password)
		if err != nil {
			return nil, err
		}

		user, err = createUser(ctx, tx, username, passwordHash)
		if err != nil {
			return nil, err
		}
		user, err = updateUserBalance(ctx, tx, user.ID, initialBalance)
		if err != nil {
			return nil, err
		}
		_, err = createTransaction(ctx, tx, nil, &user.ID, initialBalance)
		if err != nil {
			return nil, err
		}

		err = tx.Commit(ctx)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, err
	}
	return &Data{UserID: user.ID}, nil
}

type pgxExecutor interface {
	Begin(ctx context.Context) (pgx.Tx, error)
	Exec(ctx context.Context, sql string, arguments ...any) (commandTag pgconn.CommandTag, err error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults
}

func createUser(ctx context.Context, db pgxExecutor, username string, passwordHash string) (*User, error) {
	query := `
		INSERT INTO users (username, password_hash)
		VALUES ($1, $2)
		RETURNING id, username, password_hash, balance
	`
	args := []any{username, passwordHash}

	rows, _ := db.Query(ctx, query, args...)
	user, err := pgx.CollectExactlyOneRow(rows, rowToUser)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgerrcode.IsIntegrityConstraintViolation(pgErr.Code) && pgErr.ConstraintName == "users_username_idx" {
			return nil, ErrUserExist
		}
		return nil, err
	}

	return user, nil
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

func updateUserBalance(ctx context.Context, db pgxExecutor, id uuid.UUID, balance int) (*User, error) {
	query := `
		UPDATE users
		SET balance = $2
		WHERE id = $1
		RETURNING id, username, password_hash, balance
	`
	args := []any{id, balance}

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

func createTransaction(ctx context.Context, db pgxExecutor, fromUserID, toUserID *uuid.UUID, amount int) (*Transaction, error) {
	query := `
		INSERT INTO transactions (from_user_id, to_user_id, amount)
		VALUES ($1, $2, $3)
		RETURNING id, from_user_id, to_user_id, amount
	`
	args := []any{fromUserID, toUserID, amount}

	rows, _ := db.Query(ctx, query, args...)
	transaction, err := pgx.CollectExactlyOneRow(rows, rowToTransaction)
	if err != nil {
		return nil, err
	}

	return transaction, nil
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

func rowToTransaction(collectable pgx.CollectableRow) (*Transaction, error) {
	type row struct {
		ID         uuid.UUID  `db:"id"`
		FromUserID *uuid.UUID `db:"from_user_id"`
		ToUserID   *uuid.UUID `db:"to_user_id"`
		Amount     int        `db:"amount"`
	}

	collected, err := pgx.RowToStructByName[row](collectable)
	if err != nil {
		return nil, err
	}

	return &Transaction{
		ID:         collected.ID,
		FromUserID: collected.FromUserID,
		ToUserID:   collected.ToUserID,
		Amount:     collected.Amount,
	}, nil
}
