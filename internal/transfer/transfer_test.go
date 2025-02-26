package transfer

import (
	"context"
	"reflect"
	"testing"

	"github.com/k11v/merch/internal/app/apptest"
	"github.com/k11v/merch/internal/coin"
	"github.com/k11v/merch/internal/user/usertest"
)

func Test(t *testing.T) {
	t.Run("transfers by username and gets transfers by user ID", func(t *testing.T) {
		var (
			ctx   = context.Background()
			db    = apptest.NewPostgresPool(t, ctx)
			alice = usertest.CreateUser(t, ctx, db, "alice")
			bob   = usertest.CreateUser(t, ctx, db, "bob")
			cg    = coin.NewGetter(db)
			tg    = NewGetter(db)
			tt    = NewTransferer(db)
		)

		initialAliceBalance, err := cg.GetBalance(ctx, alice.ID)
		if err != nil {
			t.Fatalf("got %v error", err)
		}
		initialBobBalance, err := cg.GetBalance(ctx, bob.ID)
		if err != nil {
			t.Fatalf("got %v error", err)
		}

		err = tt.TransferByUsername(ctx, "bob", alice.ID, 40)
		if err != nil {
			t.Fatalf("got %v error", err)
		}
		err = tt.TransferByUsername(ctx, "bob", alice.ID, 41)
		if err != nil {
			t.Fatalf("got %v error", err)
		}
		err = tt.TransferByUsername(ctx, "alice", bob.ID, 43)
		if err != nil {
			t.Fatalf("got %v error", err)
		}

		aliceBalance, err := cg.GetBalance(ctx, alice.ID)
		if err != nil {
			t.Fatalf("got %v error", err)
		}
		bobBalance, err := cg.GetBalance(ctx, bob.ID)
		if err != nil {
			t.Fatalf("got %v error", err)
		}

		aliceTransfers, err := tg.GetTransfersByUserID(ctx, alice.ID)
		if err != nil {
			t.Fatalf("got %v error", err)
		}
		bobTransfers, err := tg.GetTransfersByUserID(ctx, bob.ID)
		if err != nil {
			t.Fatalf("got %v error", err)
		}

		wantTransfers := []*Transfer{
			{
				DstUserID:   bob.ID,
				SrcUserID:   alice.ID,
				Amount:      40,
				DstUsername: "bob",
				SrcUsername: "alice",
			},
			{
				DstUserID:   bob.ID,
				SrcUserID:   alice.ID,
				Amount:      41,
				DstUsername: "bob",
				SrcUsername: "alice",
			},
			{
				DstUserID:   alice.ID,
				SrcUserID:   bob.ID,
				Amount:      42,
				DstUsername: "alice",
				SrcUsername: "bob",
			},
		}

		if got, want := aliceBalance, initialAliceBalance-40-41+43; got != want {
			t.Errorf("got %d alice balance, want %d", got, want)
		}
		if got, want := bobBalance, initialBobBalance+40+41-43; got != want {
			t.Errorf("got %d bob balance, want %d", got, want)
		}
		if gotTransfers := aliceTransfers; !reflect.DeepEqual(gotTransfers, wantTransfers) {
			t.Logf("got %v alice transfers", gotTransfers)
			t.Errorf("want %v", wantTransfers)
		}
		if gotTransfers := bobTransfers; !reflect.DeepEqual(gotTransfers, wantTransfers) {
			t.Logf("got %v bob transfers", gotTransfers)
			t.Errorf("want %v", wantTransfers)
		}
	})
}
