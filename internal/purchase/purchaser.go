package purchase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/k11v/merch/internal/app"
	"github.com/k11v/merch/internal/coin"
	"github.com/k11v/merch/internal/item"
	"github.com/k11v/merch/internal/user"
)

type Purchaser struct {
	db app.PgxExecutor
}

func NewPurchaser(db app.PgxExecutor) *Purchaser {
	return &Purchaser{db: db}
}

func (h *Purchaser) PurchaseByName(ctx context.Context, itemName string, userID uuid.UUID) (*Purchase, error) {
	i, err := item.NewGetter(h.db).GetItemByName(ctx, itemName)
	if err != nil {
		return nil, fmt.Errorf("purhcase.Purchaser: %w", err)
	}

	tx, err := h.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("purhcase.Purchaser: %w", err)
	}
	defer func() {
		err = tx.Rollback(ctx)
		if err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			slog.Error("didn't rollback", "err", err)
		}
	}()

	u, err := getUserForUpdate(ctx, tx, userID)
	if err != nil {
		return nil, fmt.Errorf("purhcase.Purchaser: %w", err)
	}

	balance := u.Balance
	balance -= i.Price
	if balance < 0 {
		return nil, fmt.Errorf("purhcase.Purchaser: %w", coin.ErrNotEnough)
	}

	p, err := createPurchase(ctx, tx, userID, i.ID, i.Price)
	if err != nil {
		return nil, fmt.Errorf("purhcase.Purchaser: %w", err)
	}

	_, err = updateUserBalance(ctx, tx, userID, balance)
	if err != nil {
		return nil, fmt.Errorf("purhcase.Purchaser: %w", err)
	}

	err = tx.Commit(ctx)
	if err != nil {
		return nil, fmt.Errorf("purhcase.Purchaser: %w", err)
	}

	return p, nil
}

func getUserForUpdate(ctx context.Context, db app.PgxExecutor, id uuid.UUID) (*user.User, error) {
	query := `
		SELECT id, username, password_hash, balance
		FROM users
		WHERE id = $1
		FOR UPDATE
	`
	args := []any{id}

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

func createPurchase(ctx context.Context, db app.PgxExecutor, userID uuid.UUID, itemID uuid.UUID, amount int) (*Purchase, error) {
	query := `
		INSERT INTO purchases (user_id, item_id, amount)
		VALUES ($1, $2, $3)
		RETURNING id, created_at, user_id, item_id, amount
	`
	args := []any{userID, itemID, amount}

	rows, _ := db.Query(ctx, query, args...)
	p, err := pgx.CollectExactlyOneRow(rows, RowToPurchase)
	if err != nil {
		return nil, err
	}

	return p, nil
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
