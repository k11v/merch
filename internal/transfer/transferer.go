package transfer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k11v/merch/internal/app"
	"github.com/k11v/merch/internal/coin"
	"github.com/k11v/merch/internal/user"
)

type Transferer struct {
	db *pgxpool.Pool
}

func NewTransferer(db *pgxpool.Pool) *Transferer {
	return &Transferer{db: db}
}

func (t *Transferer) TransferByUsername(ctx context.Context, dstUsername string, srcUserID uuid.UUID, amount int) error {
	dstUser, err := user.NewGetter(t.db).GetUserByUsername(ctx, dstUsername)
	if err != nil {
		if errors.Is(err, user.ErrNotExist) {
			return fmt.Errorf("transfer.Transferer: %w", ErrDstUserNotFound)
		}
		return fmt.Errorf("transfer.Transferer: %w", err)
	}
	dstUserID := dstUser.ID

	if srcUserID == dstUserID {
		return fmt.Errorf("transfer.Transferer: %w", ErrSrcUserAndDstUserEqual)
	}

	tx, err := t.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("transfer.Transferer: %w", err)
	}
	defer func() {
		rollbackErr := tx.Rollback(ctx)
		if rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			slog.Error("didn't rollback", "err", rollbackErr)
		}
	}()

	usersMap, err := getUsersByIDsForUpdate(ctx, tx, srcUserID, dstUserID)
	if err != nil {
		return fmt.Errorf("transfer.Transferer: %w", err)
	}
	srcUser := usersMap[srcUserID]
	dstUser = usersMap[dstUserID]

	srcUserBalance := srcUser.Balance
	srcUserBalance -= amount
	if srcUserBalance < 0 {
		return fmt.Errorf("transfer.Transferer: %w", coin.ErrNotEnough)
	}

	dstUserBalance := dstUser.Balance
	dstUserBalance += amount

	_, err = createTransfer(ctx, tx, &dstUserID, &srcUserID, amount)
	if err != nil {
		return fmt.Errorf("transfer.Transferer: %w", err)
	}

	_, err = updateUserBalance(ctx, tx, srcUserID, srcUserBalance)
	if err != nil {
		return fmt.Errorf("transfer.Transferer: %w", err)
	}

	_, err = updateUserBalance(ctx, tx, dstUserID, dstUserBalance)
	if err != nil {
		return fmt.Errorf("transfer.Transferer: %w", err)
	}

	err = tx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("transfer.Transferer: %w", err)
	}

	return nil
}

func getUsersByIDsForUpdate(ctx context.Context, db app.PgxExecutor, ids ...uuid.UUID) (map[uuid.UUID]*user.User, error) {
	query := `
		SELECT id, username, password_hash, balance
		FROM users
		WHERE id = ANY($1)
		FOR UPDATE
	`
	args := []any{ids}

	rows, _ := db.Query(ctx, query, args...)
	users, err := pgx.CollectRows(rows, user.RowToUser)
	if err != nil {
		return nil, err
	}

	usersMap := make(map[uuid.UUID]*user.User)
	for _, u := range users {
		usersMap[u.ID] = u
	}

	for _, id := range ids {
		_, ok := usersMap[id]
		if !ok {
			return nil, user.ErrNotExist
		}
	}

	return usersMap, nil
}

func createTransfer(ctx context.Context, db app.PgxExecutor, dstUserID, srcUserID *uuid.UUID, amount int) (*Transfer, error) {
	query := `
		INSERT INTO transfers (dst_user_id, src_user_id, amount)
		VALUES ($1, $2, $3)
		RETURNING id, created_at, dst_user_id, src_user_id, amount
	`
	args := []any{dstUserID, srcUserID, amount}

	rows, _ := db.Query(ctx, query, args...)
	t, err := pgx.CollectExactlyOneRow(rows, RowToTransfer)
	if err != nil {
		return nil, err
	}

	return t, nil
}

func updateUserBalance(ctx context.Context, db app.PgxExecutor, id uuid.UUID, balance int) (*user.User, error) {
	query := `
		UPDATE users
		SET balance = $2
		WHERE id = $1
		RETURNING id, username, password_hash, balance
	`
	args := []any{id, balance}

	rows, _ := db.Query(ctx, query, args...)
	u, err := pgx.CollectExactlyOneRow(rows, user.RowToUser)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, user.ErrNotExist
		}
		return nil, err
	}

	return u, nil
}
