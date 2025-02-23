package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/k11v/merch/api/merch"
	"github.com/k11v/merch/internal/purchase"
)

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

	purchaser := purchase.NewPurchaser(h.db)
	err := purchaser.PurchaseByName(ctx, itemName, userID)
	if err != nil {
		if errors.Is(err, purchase.ErrItemNotExist) {
			errors := "item does not exist"
			return merch.GetAPIBuyItem400JSONResponse{Errors: &errors}, nil
		}
		if errors.Is(err, purchase.ErrCoinNotEnough) {
			errors := "not enough coin"
			return merch.GetAPIBuyItem400JSONResponse{Errors: &errors}, nil
		}
		return nil, err
	}

	return merch.GetAPIBuyItem200Response{}, nil
}
