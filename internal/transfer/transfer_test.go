package transfer

import (
	"context"
	"reflect"
	"testing"

	"github.com/k11v/merch/internal/app/apptest"
	"github.com/k11v/merch/internal/auth/authtest"
	"github.com/k11v/merch/internal/coin"
)

func Test(t *testing.T) {
	t.Run("transfers by username and gets transfers by user ID", func(t *testing.T) {
		var (
			ctx         = context.Background()
			db          = apptest.NewPostgresPool(t, ctx)
			aliceUserID = authtest.CreateOrGetUserID(t, ctx, db, "alice")
			bobUserID   = authtest.CreateOrGetUserID(t, ctx, db, "bob")
			cg          = coin.NewGetter(db)
			tg          = NewGetter(db)
			tt          = NewTransferer(db)
		)

		initialAliceBalance, err := cg.GetBalance(ctx, aliceUserID)
		if err != nil {
			t.Fatalf("got %v error", err)
		}
		initialBobBalance, err := cg.GetBalance(ctx, bobUserID)
		if err != nil {
			t.Fatalf("got %v error", err)
		}

		err = tt.TransferByUsername(ctx, "bob", aliceUserID, 40)
		if err != nil {
			t.Fatalf("got %v error", err)
		}
		err = tt.TransferByUsername(ctx, "bob", aliceUserID, 41)
		if err != nil {
			t.Fatalf("got %v error", err)
		}
		err = tt.TransferByUsername(ctx, "alice", bobUserID, 43)
		if err != nil {
			t.Fatalf("got %v error", err)
		}

		aliceBalance, err := cg.GetBalance(ctx, aliceUserID)
		if err != nil {
			t.Fatalf("got %v error", err)
		}
		bobBalance, err := cg.GetBalance(ctx, bobUserID)
		if err != nil {
			t.Fatalf("got %v error", err)
		}

		aliceTransfers, err := tg.GetTransfersByUserID(ctx, aliceUserID)
		if err != nil {
			t.Fatalf("got %v error", err)
		}
		bobTransfers, err := tg.GetTransfersByUserID(ctx, bobUserID)
		if err != nil {
			t.Fatalf("got %v error", err)
		}

		wantTransfers := []*Transfer{
			{
				DstUserID:   bobUserID,
				SrcUserID:   aliceUserID,
				Amount:      40,
				DstUsername: "bob",
				SrcUsername: "alice",
			},
			{
				DstUserID:   bobUserID,
				SrcUserID:   aliceUserID,
				Amount:      41,
				DstUsername: "bob",
				SrcUsername: "alice",
			},
			{
				DstUserID:   aliceUserID,
				SrcUserID:   bobUserID,
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
