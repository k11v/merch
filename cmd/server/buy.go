package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k11v/merch/api/merch"
)

var ErrItemNotExist = errors.New("item does not exist")

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

// GetAPIBuyItem implements merch.StrictServerInterface.
func (h *Handler) GetAPIBuyItem(ctx context.Context, request merch.GetAPIBuyItemRequestObject) (merch.GetAPIBuyItemResponseObject, error) {
	userID, ok := ctx.Value(ContextValueUserID).(uuid.UUID)
	if !ok {
		panic(fmt.Errorf("can't get %s context value", ContextValueUserID))
	}

	itemName := request.Item
	if itemName == "" {
		errors := "empty item"
		return merch.GetAPIBuyItem400JSONResponse{Errors: &errors}, nil
	}

	purchaser := NewPurchaser(h.db)
	err := purchaser.PurchaseByName(ctx, itemName, userID)
	if err != nil {
		if errors.Is(err, ErrItemNotExist) {
			errors := "item does not exist"
			return merch.GetAPIBuyItem400JSONResponse{Errors: &errors}, nil
		}
		if errors.Is(err, ErrCoinNotEnough) {
			errors := "not enough coin"
			return merch.GetAPIBuyItem400JSONResponse{Errors: &errors}, nil
		}
		return nil, err
	}

	return merch.GetAPIBuyItem200Response{}, nil
}

var ErrCoinNotEnough = errors.New("not enough coin")

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

func getItemByName(ctx context.Context, db pgxExecutor, name string) (*Item, error) {
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

func getUserForUpdate(ctx context.Context, db pgxExecutor, id uuid.UUID) (*User, error) {
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

func addUserItemAmount(ctx context.Context, db pgxExecutor, userID, itemID uuid.UUID, amount int) (*UserItem, error) {
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

func rowToUserItemWithName(collectable pgx.CollectableRow) (*UserItem, error) {
	type row struct {
		UserID uuid.UUID `db:"user_id"`
		ItemID uuid.UUID `db:"item_id"`
		Amount int       `db:"amount"`

		ItemName string `db:"item_name"`
	}

	collected, err := pgx.RowToStructByName[row](collectable)
	if err != nil {
		return nil, err
	}

	return &UserItem{
		UserID:   collected.UserID,
		ItemID:   collected.ItemID,
		Amount:   collected.Amount,
		ItemName: collected.ItemName,
	}, nil
}
