package purchase

import (
	"context"
	"reflect"
	"testing"

	"github.com/k11v/merch/internal/app/apptest"
	"github.com/k11v/merch/internal/coin"
	"github.com/k11v/merch/internal/user/usertest"
)

func TestPurchase(t *testing.T) {
	t.Run("purchases and gets item counts", func(t *testing.T) {
		var (
			ctx  = context.Background()
			db   = apptest.NewPostgresPool(t, ctx)
			user = usertest.CreateUser(t, ctx, db, "alice")
			cg   = coin.NewGetter(db)
			pg   = NewGetter(db)
			pp   = NewPurchaser(db)
		)

		initialBalance, err := cg.GetBalance(ctx, user.ID)
		if err != nil {
			t.Fatalf("got %v error", err)
		}

		cupPurchase, err := pp.PurchaseByName(ctx, "cup", user.ID)
		if err != nil {
			t.Fatalf("got %v error", err)
		}
		walletPurchase1, err := pp.PurchaseByName(ctx, "wallet", user.ID)
		if err != nil {
			t.Fatalf("got %v error", err)
		}
		walletPurchase2, err := pp.PurchaseByName(ctx, "wallet", user.ID)
		if err != nil {
			t.Fatalf("got %v error", err)
		}

		balance, err := cg.GetBalance(ctx, user.ID)
		if err != nil {
			t.Fatalf("got %v error", err)
		}
		itemCounts, err := pg.GetItemCountsByUserID(ctx, user.ID)
		if err != nil {
			t.Fatalf("got %v error", err)
		}

		wantBalance := initialBalance - cupPurchase.Amount - walletPurchase1.Amount - walletPurchase2.Amount
		wantItemCounts := []*ItemCount{
			{
				UserID:   user.ID,
				ItemID:   cupPurchase.ItemID,
				Count:    1,
				ItemName: "cup",
			},
			{
				UserID:   user.ID,
				ItemID:   walletPurchase1.ItemID,
				Count:    2,
				ItemName: "wallet",
			},
		}

		if got, want := balance, wantBalance; got != want {
			t.Errorf("got %d balance, want %d", got, want)
		}
		if got, want := itemCounts, wantItemCounts; !reflect.DeepEqual(got, want) {
			t.Logf("got %v item counts", got)
			t.Errorf("want %v", want)
		}
	})
}
