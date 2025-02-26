package coin

import (
	"context"
	"testing"

	"github.com/k11v/merch/internal/app/apptest"
	"github.com/k11v/merch/internal/auth/authtest"
)

func TestGetter(t *testing.T) {
	t.Run("gets balance", func(t *testing.T) {
		var (
			ctx    = context.Background()
			db     = apptest.NewPostgresPool(t, ctx)
			userID = authtest.CreateOrGetUserID(t, ctx, db, "alice")
			g      = NewGetter(db)
		)

		_, err := g.GetBalance(ctx, userID)
		if err != nil {
			t.Fatalf("got %v error", err)
		}
	})
}
