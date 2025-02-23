package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k11v/merch/api/merch"
)

// GetAPIInfo implements merch.StrictServerInterface.
func (h *Handler) GetAPIInfo(ctx context.Context, request merch.GetAPIInfoRequestObject) (merch.GetAPIInfoResponseObject, error) {
	userID, ok := ctx.Value(ContextValueUserID).(uuid.UUID)
	if !ok {
		panic(fmt.Errorf("can't get %s context value", ContextValueUserID))
	}

	itemGetter := NewPurchaseGetter(h.db)
	userItemCounts, err := itemGetter.GetUserItemCountsByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	transferGetter := NewTransferGetter(h.db)
	transfers, err := transferGetter.GetTransfersByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	coinGetter := NewCoinGetter(h.db)
	balance, err := coinGetter.GetBalance(ctx, userID)
	if err != nil {
		return nil, err
	}

	type receivedHistoryItem = struct {
		Amount   *int    `json:"amount,omitempty"`
		FromUser *string `json:"fromUser,omitempty"`
	}
	type sentHistoryItem = struct {
		Amount *int    `json:"amount,omitempty"`
		ToUser *string `json:"toUser,omitempty"`
	}
	type history = struct {
		Received *[]receivedHistoryItem `json:"received,omitempty"`
		Sent     *[]sentHistoryItem     `json:"sent,omitempty"`
	}
	received := make([]receivedHistoryItem, 0)
	sent := make([]sentHistoryItem, 0)
	for _, t := range transfers {
		if t.SrcUserID == userID {
			sent = append(sent, sentHistoryItem{
				Amount: &t.Amount,
				ToUser: &t.DstUsername,
			})
		}
		if t.DstUserID == userID {
			received = append(received, receivedHistoryItem{
				Amount:   &t.Amount,
				FromUser: &t.SrcUsername,
			})
		}
	}

	type inventoryItem = struct {
		Quantity *int    `json:"quantity,omitempty"`
		Type     *string `json:"type,omitempty"`
	}
	inventory := make([]inventoryItem, len(userItemCounts))
	for i, userItem := range userItemCounts {
		inventory[i] = inventoryItem{
			Quantity: &userItem.Count,
			Type:     &userItem.ItemName,
		}
	}

	return merch.GetAPIInfo200JSONResponse{
		Coins: &balance,
		CoinHistory: &history{
			Received: &received,
			Sent:     &sent,
		},
		Inventory: &inventory,
	}, nil
}

type CoinGetter struct {
	db *pgxpool.Pool
}

func NewCoinGetter(db *pgxpool.Pool) *CoinGetter {
	return &CoinGetter{db: db}
}

func (g *CoinGetter) GetBalance(ctx context.Context, userID uuid.UUID) (int, error) {
	user, err := getUser(ctx, g.db, userID)
	if err != nil {
		return 0, err
	}
	return user.Balance, nil
}

type Transfer struct {
	DstUserID uuid.UUID
	SrcUserID uuid.UUID
	Amount    int

	DstUsername string
	SrcUsername string
}

type TransferGetter struct {
	db *pgxpool.Pool
}

func NewTransferGetter(db *pgxpool.Pool) *TransferGetter {
	return &TransferGetter{db: db}
}

func (g *TransferGetter) GetTransfersByUserID(ctx context.Context, userID uuid.UUID) ([]*Transfer, error) {
	transactions, err := getTransactionsByUserID(ctx, g.db, userID)
	if err != nil {
		return nil, err
	}
	transfers := make([]*Transfer, 0)
	for _, t := range transactions {
		if t.ToUserID == nil || t.FromUserID == nil {
			continue
		}
		transfers = append(transfers, &Transfer{
			DstUserID:   *t.ToUserID,
			SrcUserID:   *t.FromUserID,
			Amount:      t.Amount,
			DstUsername: *t.ToUsername,
			SrcUsername: *t.FromUsername,
		})
	}
	return transfers, nil
}

type UserItemCount struct {
	UserID uuid.UUID
	ItemID uuid.UUID
	Count  int

	ItemName string
}

type ItemGetter struct {
	db *pgxpool.Pool
}

func NewPurchaseGetter(db *pgxpool.Pool) *ItemGetter {
	return &ItemGetter{db: db}
}

func (g *ItemGetter) GetUserItemCountsByUserID(ctx context.Context, userID uuid.UUID) ([]*UserItemCount, error) {
	userItems, err := getUserItems(ctx, g.db, userID)
	if err != nil {
		return nil, err
	}
	userItemCounts := make([]*UserItemCount, 0)
	for _, ui := range userItems {
		userItemCounts = append(userItemCounts, &UserItemCount{
			UserID:   ui.UserID,
			ItemID:   ui.ItemID,
			Count:    ui.Amount,
			ItemName: ui.ItemName,
		})
	}
	return userItemCounts, nil
}

func getUser(ctx context.Context, db pgxExecutor, id uuid.UUID) (*User, error) {
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

func getTransactionsByUserID(ctx context.Context, db pgxExecutor, userID uuid.UUID) ([]*Transaction, error) {
	query := `
		SELECT t.id, t.from_user_id, from_u.username as from_username, t.to_user_id, to_u.username as to_username, t.amount
		FROM transactions t
		LEFT JOIN users from_u ON t.from_user_id = from_u.id
		LEFT JOIN users to_u ON t.to_user_id = to_u.id
		WHERE t.from_user_id = $1 OR t.to_user_id = $1
	`
	args := []any{userID}

	rows, _ := db.Query(ctx, query, args...)
	transactions, err := pgx.CollectRows(rows, rowToTransactionWithUsernames)
	if err != nil {
		return nil, err
	}

	return transactions, nil
}

func getUserItems(ctx context.Context, db pgxExecutor, userID uuid.UUID) ([]*UserItem, error) {
	query := `
		SELECT ui.user_id, ui.item_id, i.name AS item_name, ui.amount
		FROM users_items ui
		JOIN items i ON ui.item_id = i.id
		WHERE ui.user_id = $1
	`
	args := []any{userID}

	rows, _ := db.Query(ctx, query, args...)
	userItems, err := pgx.CollectRows(rows, rowToUserItemWithName)
	if err != nil {
		return nil, err
	}

	return userItems, nil
}
