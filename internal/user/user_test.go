package user

import (
	"context"
	"testing"

	"github.com/k11v/merch/internal/app/apptest"
)

func Test(t *testing.T) {
	t.Run("creates and gets user with initial balance", func(t *testing.T) {
		var (
			ctx = context.Background()
			tx  = apptest.NewPostgresPool(t, ctx)
			ph  = NewPasswordHasher(DefaultArgon2IDParams())
			c   = NewCreator(tx, ph)
			g   = NewGetter(tx)
		)

		_, err := c.CreateUser(ctx, "alice", "alice123")
		if err != nil {
			t.Fatalf("got %v error", err)
		}

		u, err := g.GetUserByUsername(ctx, "alice")
		if err != nil {
			t.Fatalf("got %v error", err)
		}

		if got, want := u.Balance, InitialBalance; got != want {
			t.Fatalf("got %d balance, want %d", got, want)
		}
	})
}
