package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/k11v/merch/api/merch"
	"github.com/k11v/merch/internal/transfer"
)

// PostAPISendCoin implements merch.StrictServerInterface.
func (h *Handler) PostAPISendCoin(ctx context.Context, request merch.PostAPISendCoinRequestObject) (merch.PostAPISendCoinResponseObject, error) {
	requestUserID, ok := ctx.Value(ContextValueUserID).(uuid.UUID)
	if !ok {
		panic(fmt.Errorf("can't get %s context value", ContextValueUserID))
	}

	toUsername := request.Body.ToUser
	if toUsername == "" {
		errors := "empty toUser body value"
		return merch.PostAPISendCoin400JSONResponse{Errors: &errors}, nil
	}

	amount := request.Body.Amount
	if amount <= 0 {
		errors := "non-positive amount body value"
		return merch.PostAPISendCoin400JSONResponse{Errors: &errors}, nil
	}

	fromUserID := requestUserID

	transferer := transfer.NewTransferer(h.db)
	err := transferer.TransferByUsername(ctx, toUsername, fromUserID, amount)
	if err != nil {
		if errors.Is(err, transfer.ErrDstUserNotFound) {
			errors := "toUser doesn't exist"
			return merch.PostAPISendCoin400JSONResponse{Errors: &errors}, nil
		}
		if errors.Is(err, transfer.ErrSrcUserAndDstUserEqual) {
			errors := "fromUser and toUser are equal"
			return merch.PostAPISendCoin400JSONResponse{Errors: &errors}, nil
		}
		if errors.Is(err, ErrCoinNotEnough) {
			errors := "not enough coins"
			return merch.PostAPISendCoin400JSONResponse{Errors: &errors}, nil
		}
		return nil, err
	}

	return merch.PostAPISendCoin200Response{}, nil
}
