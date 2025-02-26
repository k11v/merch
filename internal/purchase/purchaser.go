package purchase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k11v/merch/internal/app"
)

var (
	ErrItemNotExist  = errors.New("item does not exist")
	ErrCoinNotEnough = errors.New("not enough coin")
	ErrUserNotExist  = errors.New("user does not exist")
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

type Item struct {
	ID    uuid.UUID
	Name  string
	Price int
}

type UserItem struct {
	UserID   uuid.UUID
	ItemID   uuid.UUID
	ItemName string
	Amount   int
}

type Purchaser struct {
	db *pgxpool.Pool
}

func NewPurchaser(db *pgxpool.Pool) *Purchaser {
	return &Purchaser{db: db}
}

func (h *Purchaser) PurchaseByName(ctx context.Context, itemName string, userID uuid.UUID) error {
	item, err := getItemByName(ctx, h.db, itemName)
	if err != nil {
		return fmt.Errorf("Purchaser: %w", err)
	}

	tx, err := h.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("Purchaser: %w", err)
	}
	defer func() {
		err = tx.Rollback(ctx)
		if err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			slog.Error("didn't rollback", "err", err)
		}
	}()

	user, err := getUserForUpdate(ctx, tx, userID)
	if err != nil {
		return fmt.Errorf("Purchaser: %w", err)
	}

	balance := user.Balance
	balance -= item.Price
	if balance < 0 {
		return fmt.Errorf("Purchaser: %w", ErrCoinNotEnough)
	}

	_, err = updateUserBalance(ctx, tx, userID, balance)
	if err != nil {
		return fmt.Errorf("Purchaser: %w", err)
	}

	_, err = createTransaction(ctx, tx, &userID, nil, item.Price)
	if err != nil {
		return fmt.Errorf("Purchaser: %w", err)
	}

	_, err = addUserItemAmount(ctx, tx, userID, item.ID, 1)
	if err != nil {
		return fmt.Errorf("Purchaser: %w", err)
	}

	err = tx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("Purchaser: %w", err)
	}

	return nil
}

func getItemByName(ctx context.Context, db app.PgxExecutor, name string) (*Item, error) {
	query := `
		SELECT id, name, price
		FROM items
		WHERE name = $1
	`
	args := []any{name}

	rows, _ := db.Query(ctx, query, args...)
	item, err := pgx.CollectExactlyOneRow(rows, rowToItem)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrItemNotExist
		}
		return nil, err
	}

	return item, nil
}

func getUserForUpdate(ctx context.Context, db app.PgxExecutor, id uuid.UUID) (*User, error) {
	query := `
		SELECT id, username, password_hash, balance
		FROM users
		WHERE id = $1
		FOR UPDATE
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

func updateUserBalance(ctx context.Context, db app.PgxExecutor, id uuid.UUID, balance int) (*User, error) {
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

func addUserItemAmount(ctx context.Context, db app.PgxExecutor, userID, itemID uuid.UUID, amount int) (*UserItem, error) {
	query := `
		INSERT INTO users_items (user_id, item_id, amount)
		VALUES ($1, $2, $3)
		ON CONFLICT ON CONSTRAINT users_user_id_item_id_pkey
		DO UPDATE SET amount = users_items.amount + excluded.amount
		RETURNING user_id, item_id, amount
	`
	args := []any{userID, itemID, amount}

	rows, _ := db.Query(ctx, query, args...)
	userItem, err := pgx.CollectExactlyOneRow(rows, rowToUserItem)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotExist
		}
		return nil, err
	}

	return userItem, nil
}

func createTransaction(ctx context.Context, db app.PgxExecutor, fromUserID, toUserID *uuid.UUID, amount int) (*Transaction, error) {
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

func rowToItem(collectable pgx.CollectableRow) (*Item, error) {
	type row struct {
		ID    uuid.UUID `db:"id"`
		Name  string    `db:"name"`
		Price int       `db:"price"`
	}

	collected, err := pgx.RowToStructByName[row](collectable)
	if err != nil {
		return nil, err
	}

	return &Item{
		ID:    collected.ID,
		Name:  collected.Name,
		Price: collected.Price,
	}, nil
}

func rowToUserItem(collectable pgx.CollectableRow) (*UserItem, error) {
	type row struct {
		UserID uuid.UUID `db:"user_id"`
		ItemID uuid.UUID `db:"item_id"`
		Amount int       `db:"amount"`
	}

	collected, err := pgx.RowToStructByName[row](collectable)
	if err != nil {
		return nil, err
	}

	return &UserItem{
		UserID: collected.UserID,
		ItemID: collected.ItemID,
		Amount: collected.Amount,
	}, nil
}
