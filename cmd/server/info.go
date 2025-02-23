package main

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/k11v/merch/api/merch"
	"github.com/k11v/merch/internal/coin"
	"github.com/k11v/merch/internal/item"
	"github.com/k11v/merch/internal/transfer"
)

// GetAPIInfo implements merch.StrictServerInterface.
func (h *Handler) GetAPIInfo(ctx context.Context, request merch.GetAPIInfoRequestObject) (merch.GetAPIInfoResponseObject, error) {
	userID, ok := ctx.Value(ContextValueUserID).(uuid.UUID)
	if !ok {
		panic(fmt.Errorf("can't get %s context value", ContextValueUserID))
	}

	itemGetter := item.NewGetter(h.db)
	userItemCounts, err := itemGetter.GetUserItemCountsByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	transferGetter := transfer.NewGetter(h.db)
	transfers, err := transferGetter.GetTransfersByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	coinGetter := coin.NewGetter(h.db)
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
