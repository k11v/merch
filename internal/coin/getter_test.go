package coin

import (
	"context"
	"testing"

	"github.com/k11v/merch/internal/app/apptest"
	"github.com/k11v/merch/internal/user/usertest"
)

func TestGetter(t *testing.T) {
	t.Run("gets balance", func(t *testing.T) {
		var (
			ctx   = context.Background()
			db    = apptest.NewPostgresPool(t, ctx)
			alice = usertest.CreateUser(t, ctx, db, "alice")
			g     = NewGetter(db)
		)

		_, err := g.GetBalance(ctx, alice.ID)
		if err != nil {
			t.Fatalf("got %v error", err)
		}
	})
}
