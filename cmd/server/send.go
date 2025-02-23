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

	transferer := NewTransferer(h.db)
	err := transferer.TransferByUsername(ctx, toUsername, fromUserID, amount)
	if err != nil {
		if errors.Is(err, ErrDstUserNotFound) {
			errors := "toUser doesn't exist"
			return merch.PostAPISendCoin400JSONResponse{Errors: &errors}, nil
		}
		if errors.Is(err, ErrSrcUserAndDstUserEqual) {
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

var (
	ErrDstUserNotFound        = errors.New("dst user not found")
	ErrSrcUserAndDstUserEqual = errors.New("src user and dst user are equal")
)

type Transferer struct {
	db *pgxpool.Pool
}

func NewTransferer(db *pgxpool.Pool) *Transferer {
	return &Transferer{db: db}
}

func (h *Transferer) TransferByUsername(ctx context.Context, dstUsername string, srcUserID uuid.UUID, amount int) error {
	dstUser, err := getUserByUsername(ctx, h.db, dstUsername)
	if err != nil {
		if errors.Is(err, ErrUserNotExist) {
			return fmt.Errorf("Transferer: %w", ErrDstUserNotFound)
		}
		return fmt.Errorf("Transferer: %w", err)
	}
	dstUserID := dstUser.ID

	if srcUserID == dstUserID {
		return fmt.Errorf("Transferer: %w", ErrSrcUserAndDstUserEqual)
	}

	tx, err := h.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("Transferer: %w", err)
	}
	defer func() {
		err = tx.Rollback(ctx)
		if err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			slog.Error("didn't rollback", "err", err)
		}
	}()

	usersMap, err := getUsersByIDsForUpdate(ctx, tx, srcUserID, dstUserID)
	if err != nil {
		return fmt.Errorf("Transferer: %w", err)
	}
	srcUser := usersMap[srcUserID]
	dstUser = usersMap[dstUserID]

	srcUserBalance := srcUser.Balance
	srcUserBalance -= amount
	if srcUserBalance < 0 {
		return fmt.Errorf("Transferer: %w", ErrCoinNotEnough)
	}

	dstUserBalance := dstUser.Balance
	dstUserBalance += amount

	_, err = updateUserBalance(ctx, tx, srcUserID, srcUserBalance)
	if err != nil {
		return fmt.Errorf("Transferer: %w", err)
	}

	_, err = updateUserBalance(ctx, tx, dstUserID, dstUserBalance)
	if err != nil {
		return fmt.Errorf("Transferer: %w", err)
	}

	_, err = createTransaction(ctx, tx, &srcUserID, &dstUserID, amount)
	if err != nil {
		return fmt.Errorf("Transferer: %w", err)
	}

	err = tx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("Transferer: %w", err)
	}

	return nil
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
