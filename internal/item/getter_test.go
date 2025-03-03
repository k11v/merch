package item

import (
	"context"
	"errors"
	"testing"

	"github.com/k11v/merch/internal/app/apptest"
)

func TestGetter(t *testing.T) {
	var (
		ctx = context.Background()
		db  = apptest.NewPostgresPool(t, ctx)
	)

	t.Run("gets item by name", func(t *testing.T) {
		var (
			tx = apptest.BeginPostgresTx(t, ctx, db)
			g  = NewGetter(tx)
		)

		i, err := g.GetItemByName(ctx, "t-shirt")
		if err != nil {
			t.Fatalf("got %v error", err)
		}

		if got, want := i.Name, "t-shirt"; got != want {
			t.Fatalf("got %s name, want %s", got, want)
		}
	})

	t.Run("doesn't get item by nonexistent name", func(t *testing.T) {
		var (
			tx = apptest.BeginPostgresTx(t, ctx, db)
			g  = NewGetter(tx)
		)

		_, err := g.GetItemByName(ctx, "nonexistent")
		if got, want := err, ErrNotExist; !errors.Is(got, want) {
			t.Fatalf("got %v error, want %v", got, want)
		}
	})
}
