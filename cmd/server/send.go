package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/k11v/merch/api/merch"
)

type Transaction struct {
	ID         uuid.UUID
	FromUserID *uuid.UUID
	ToUserID   *uuid.UUID
	Amount     int

	FromUsername *string
	ToUsername   *string
}

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

	toUser, err := getUserByUsername(ctx, h.db, toUsername)
	if err != nil {
		if errors.Is(err, ErrUserNotExist) {
			errors := "toUser doesn't exist"
			return merch.PostAPISendCoin400JSONResponse{Errors: &errors}, nil
		}
		return nil, err
	}
	toUserID := toUser.ID

	if fromUserID == toUserID {
		errors := "identical fromUser and toUser"
		return merch.PostAPISendCoin400JSONResponse{Errors: &errors}, nil
	}

	tx, err := h.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = tx.Rollback(ctx)
		if err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			slog.Error("didn't rollback", "err", err)
		}
	}()

	usersMap, err := getUsersByIDsForUpdate(ctx, tx, fromUserID, toUserID)
	if err != nil {
		return nil, err
	}
	fromUser := usersMap[fromUserID]
	toUser = usersMap[toUserID]

	fromUserBalance := fromUser.Balance
	fromUserBalance -= amount
	if fromUserBalance < 0 {
		errors := "not enough coins"
		return merch.PostAPISendCoin400JSONResponse{Errors: &errors}, nil
	}

	toUserBalance := toUser.Balance
	toUserBalance += amount

	_, err = updateUserBalance(ctx, tx, fromUserID, fromUserBalance)
	if err != nil {
		return nil, err
	}

	_, err = updateUserBalance(ctx, tx, toUserID, toUserBalance)
	if err != nil {
		return nil, err
	}

	_, err = createTransaction(ctx, tx, &fromUserID, &toUserID, amount)
	if err != nil {
		return nil, err
	}

	err = tx.Commit(ctx)
	if err != nil {
		return nil, err
	}

	return merch.PostAPISendCoin200Response{}, nil
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

func rowToTransactionWithUsernames(collectable pgx.CollectableRow) (*Transaction, error) {
	type row struct {
		ID         uuid.UUID  `db:"id"`
		FromUserID *uuid.UUID `db:"from_user_id"`
		ToUserID   *uuid.UUID `db:"to_user_id"`
		Amount     int        `db:"amount"`

		FromUsername *string `db:"from_username"`
		ToUsername   *string `db:"to_username"`
	}

	collected, err := pgx.RowToStructByName[row](collectable)
	if err != nil {
		return nil, err
	}

	return &Transaction{
		ID:           collected.ID,
		FromUserID:   collected.FromUserID,
		ToUserID:     collected.ToUserID,
		Amount:       collected.Amount,
		FromUsername: collected.FromUsername,
		ToUsername:   collected.ToUsername,
	}, nil
}
