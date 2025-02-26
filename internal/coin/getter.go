package coin

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/k11v/merch/internal/app"
	"github.com/k11v/merch/internal/user"
)

var ErrNotEnough = errors.New("not enough")

type Getter struct {
	db app.PgxExecutor
}

func NewGetter(db app.PgxExecutor) *Getter {
	return &Getter{db: db}
}

func (g *Getter) GetBalance(ctx context.Context, userID uuid.UUID) (int, error) {
	u, err := user.NewGetter(g.db).GetUser(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("coin.Getter: %w", err)
	}
	return u.Balance, nil
}
