package user

import (
	"context"
	"testing"

	"github.com/k11v/merch/internal/app/apptest"
)

func Test(t *testing.T) {
	var (
		ctx = context.Background()
		db  = apptest.NewPostgresPool(t, ctx)
	)

	t.Run("creates and gets user with default balance", func(t *testing.T) {
		var (
			tx = apptest.BeginPostgresTx(t, ctx, db)
			ph = NewPasswordHasher(DefaultArgon2IDParams())
			c  = NewCreator(tx, ph)
			g  = NewGetter(tx)
		)

		u, err := c.CreateUser(ctx, "alice", "alice123")
		if err != nil {
			t.Fatalf("got %v error", err)
		}

		u, err = g.GetUser(ctx, u.ID)
		if err != nil {
			t.Fatalf("got %v error", err)
		}

		if got, want := u.Balance, DefaultBalance; got != want {
			t.Fatalf("got %d balance, want %d", got, want)
		}
	})

	t.Run("gets user by username", func(t *testing.T) {
		var (
			tx = apptest.BeginPostgresTx(t, ctx, db)
			ph = NewPasswordHasher(DefaultArgon2IDParams())
			c  = NewCreator(tx, ph)
			g  = NewGetter(tx)
		)

		cu, err := c.CreateUser(ctx, "alice", "alice123")
		if err != nil {
			t.Fatalf("got %v error", err)
		}

		gu, err := g.GetUserByUsername(ctx, "alice")
		if err != nil {
			t.Fatalf("got %v error", err)
		}

		if got, want := cu.ID, gu.ID; got != want {
			t.Fatalf("got %d user ID, want %d", got, want)
		}
	})
}
