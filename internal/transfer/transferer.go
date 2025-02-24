package transfer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type User struct {
	ID           uuid.UUID
	Username     string
	PasswordHash string
	Balance      int
}

type pgxExecutor interface {
	Begin(ctx context.Context) (pgx.Tx, error)
	Exec(ctx context.Context, sql string, arguments ...any) (commandTag pgconn.CommandTag, err error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults
}

var (
	ErrCoinNotEnough          = errors.New("not enough coin")
	ErrDstUserNotFound        = errors.New("dst user not found")
	ErrSrcUserAndDstUserEqual = errors.New("src user and dst user are equal")
	ErrUserExist              = errors.New("user already exists")
	ErrUserNotExist           = errors.New("user does not exist")
)

type Transaction struct {
	ID         uuid.UUID
	FromUserID *uuid.UUID
	ToUserID   *uuid.UUID
	Amount     int

	FromUsername *string
	ToUsername   *string
}

type Transferer struct {
	db *pgxpool.Pool
}

func NewTransferer(db *pgxpool.Pool) *Transferer {
	return &Transferer{db: db}
}

func (t *Transferer) TransferByUsername(ctx context.Context, dstUsername string, srcUserID uuid.UUID, amount int) error {
	dstUser, err := getUserByUsername(ctx, t.db, dstUsername)
	if err != nil {
		if errors.Is(err, ErrUserNotExist) {
			return fmt.Errorf("Transferer: %w", ErrDstUserNotFound)
		}
		return fmt.Errorf("Transferer: %w", err)
	}
	dstUserID := dstUser.ID

	if srcUserID == dstUserID {
		return fmt.Errorf("Transferer: %w", ErrSrcUserAndDstUserEqual)
	}

	tx, err := t.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("Transferer: %w", err)
	}
	defer func() {
		err = tx.Rollback(ctx)
		if err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			slog.Error("didn't rollback", "err", err)
		}
	}()

	usersMap, err := getUsersByIDsForUpdate(ctx, tx, srcUserID, dstUserID)
	if err != nil {
		return fmt.Errorf("Transferer: %w", err)
	}
	srcUser := usersMap[srcUserID]
	dstUser = usersMap[dstUserID]

	srcUserBalance := srcUser.Balance
	srcUserBalance -= amount
	if srcUserBalance < 0 {
		return fmt.Errorf("Transferer: %w", ErrCoinNotEnough)
	}

	dstUserBalance := dstUser.Balance
	dstUserBalance += amount

	_, err = updateUserBalance(ctx, tx, srcUserID, srcUserBalance)
	if err != nil {
		return fmt.Errorf("Transferer: %w", err)
	}

	_, err = updateUserBalance(ctx, tx, dstUserID, dstUserBalance)
	if err != nil {
		return fmt.Errorf("Transferer: %w", err)
	}

	_, err = createTransaction(ctx, tx, &srcUserID, &dstUserID, amount)
	if err != nil {
		return fmt.Errorf("Transferer: %w", err)
	}

	err = tx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("Transferer: %w", err)
	}

	return nil
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

func getUsersByIDsForUpdate(ctx context.Context, db pgxExecutor, ids ...uuid.UUID) (map[uuid.UUID]*User, error) {
	query := `
		SELECT id, username, password_hash, balance
		FROM users
		WHERE id = ANY($1)
		FOR UPDATE
	`
	args := []any{ids}

	rows, _ := db.Query(ctx, query, args...)
	users, err := pgx.CollectRows(rows, rowToUser)
	if err != nil {
		return nil, err
	}

	usersMap := make(map[uuid.UUID]*User)
	for _, u := range users {
		usersMap[u.ID] = u
	}

	for _, id := range ids {
		_, ok := usersMap[id]
		if !ok {
			return nil, ErrUserNotExist
		}
	}

	return usersMap, nil
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
