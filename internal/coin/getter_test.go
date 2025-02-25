package coin

import (
	"context"
	"testing"

	"github.com/k11v/merch/internal/app/apptest"
	"github.com/k11v/merch/internal/auth"
)

func TestGetter(t *testing.T) {
	t.Run("gets balance", func(t *testing.T) {
		var (
			ctx = context.Background()
			db  = apptest.NewPostgresPool(t, ctx)
			ph  = auth.NewPasswordHasher(auth.DefaultArgon2IDParams())
			pa  = auth.NewPasswordAuthenticator(db, ph)
			g   = NewGetter(db)
		)

		aliceData, err := pa.AuthenticatePassword(ctx, "alice", "alice123")
		if err != nil {
			t.Fatalf("got %v error", err)
		}

		_, err = g.GetBalance(ctx, aliceData.UserID)
		if err != nil {
			t.Fatalf("got %v error", err)
		}
	})
}
