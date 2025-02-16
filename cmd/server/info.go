package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/k11v/merch/api/merch"
)

// GetAPIInfo implements merch.StrictServerInterface.
func (h *Handler) GetAPIInfo(ctx context.Context, request merch.GetAPIInfoRequestObject) (merch.GetAPIInfoResponseObject, error) {
	userID, ok := ctx.Value(ContextValueUserID).(uuid.UUID)
	if !ok {
		panic(fmt.Errorf("can't get %s context value", ContextValueUserID))
	}

	user, err := getUser(ctx, h.db, userID)
	if err != nil {
		return nil, err
	}
	transactions, err := getTransactionsByUserID(ctx, h.db, userID)
	if err != nil {
		return nil, err
	}
	userItems, err := getUserItems(ctx, h.db, userID)
	if err != nil {
		return nil, err
	}

	coins := user.Balance

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
	for _, transaction := range transactions {
		fromUserID := transaction.FromUserID
		toUserID := transaction.ToUserID
		if fromUserID == nil || toUserID == nil {
			continue
		}
		if *fromUserID == userID {
			sent = append(sent, sentHistoryItem{
				Amount: &transaction.Amount,
				ToUser: transaction.ToUsername,
			})
		}
		if *toUserID == userID {
			received = append(received, receivedHistoryItem{
				Amount:   &transaction.Amount,
				FromUser: transaction.FromUsername,
			})
		}
	}

	type inventoryItem = struct {
		Quantity *int    `json:"quantity,omitempty"`
		Type     *string `json:"type,omitempty"`
	}
	inventory := make([]inventoryItem, len(userItems))
	for i, userItem := range userItems {
		inventory[i] = inventoryItem{
			Quantity: &userItem.Amount,
			Type:     &userItem.ItemName,
		}
	}

	return merch.GetAPIInfo200JSONResponse{
		CoinHistory: &history{
			Received: &received,
			Sent:     &sent,
		},
		Coins:     &coins,
		Inventory: &inventory,
	}, nil
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
