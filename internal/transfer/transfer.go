package transfer

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrDstUserNotFound        = errors.New("dst user not found")
	ErrSrcUserAndDstUserEqual = errors.New("src user and dst user are equal")
)

type Transfer struct {
	ID        uuid.UUID
	CreatedAt time.Time
	DstUserID uuid.UUID
	SrcUserID uuid.UUID
	Amount    int

	DstUsername string
	SrcUsername string
}

type Row struct {
	ID        uuid.UUID `db:"id"`
	CreatedAt time.Time `db:"created_at"`
	DstUserID uuid.UUID `db:"dst_user_id"`
	SrcUserID uuid.UUID `db:"src_user_id"`
	Amount    int       `db:"amount"`
}

func RowToTransfer(collectable pgx.CollectableRow) (*Transfer, error) {
	collected, err := pgx.RowToStructByName[Row](collectable)
	if err != nil {
		return nil, err
	}

	return &Transfer{
		ID:        collected.ID,
		CreatedAt: collected.CreatedAt,
		DstUserID: collected.DstUserID,
		SrcUserID: collected.SrcUserID,
		Amount:    collected.Amount,
	}, nil
}

type RowWithUsernames struct {
	Row
	DstUsername string `db:"dst_username"`
	SrcUsername string `db:"src_username"`
}

func RowToTransferWithUsernames(collectable pgx.CollectableRow) (*Transfer, error) {
	collected, err := pgx.RowToStructByName[RowWithUsernames](collectable)
	if err != nil {
		return nil, err
	}

	return &Transfer{
		ID:          collected.ID,
		CreatedAt:   collected.CreatedAt,
		DstUserID:   collected.DstUserID,
		SrcUserID:   collected.SrcUserID,
		Amount:      collected.Amount,
		DstUsername: collected.DstUsername,
		SrcUsername: collected.SrcUsername,
	}, nil
}
