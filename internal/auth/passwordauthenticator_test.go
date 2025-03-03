package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/k11v/merch/internal/app/apptest"
	"github.com/k11v/merch/internal/user"
)

func TestPasswordAuthenticator(t *testing.T) {
	var (
		ctx = context.Background()
		db  = apptest.NewPostgresPool(t, ctx)
	)

	t.Run("creates different users", func(t *testing.T) {
		var (
			tx = apptest.BeginPostgresTx(t, ctx, db)
			ph = user.NewPasswordHasher(user.DefaultArgon2IDParams())
			pa = NewPasswordAuthenticator(tx, ph)
		)

		aliceData, err := pa.AuthenticatePassword(ctx, "alice", "alice123")
		if err != nil {
			t.Fatalf("got %v error", err)
		}

		bobData, err := pa.AuthenticatePassword(ctx, "bob", "bob123")
		if err != nil {
			t.Fatalf("got %v error", err)
		}

		if got, notWant := bobData.UserID, aliceData.UserID; got == notWant {
			t.Log("got identical user IDs for different users")
			t.Fatalf("got %v user ID, not want %v", got, notWant)
		}
	})

	t.Run("creates and gets user", func(t *testing.T) {
		var (
			tx = apptest.BeginPostgresTx(t, ctx, db)
			ph = user.NewPasswordHasher(user.DefaultArgon2IDParams())
			pa = NewPasswordAuthenticator(tx, ph)
		)

		aliceData1, err := pa.AuthenticatePassword(ctx, "alice", "alice123")
		if err != nil {
			t.Fatalf("got %v error", err)
		}

		aliceData2, err := pa.AuthenticatePassword(ctx, "alice", "alice123")
		if err != nil {
			t.Fatalf("got %v error", err)
		}

		if got, want := aliceData1.UserID, aliceData2.UserID; got != want {
			t.Log("got different user IDs for identical users")
			t.Fatalf("got %v user ID, want %v", got, want)
		}
	})

	t.Run("doesn't get user with different password", func(t *testing.T) {
		var (
			tx = apptest.BeginPostgresTx(t, ctx, db)
			ph = user.NewPasswordHasher(user.DefaultArgon2IDParams())
			pa = NewPasswordAuthenticator(tx, ph)
		)

		_, err := pa.AuthenticatePassword(ctx, "alice", "alice123")
		if err != nil {
			t.Fatalf("got %v error", err)
		}

		_, err = pa.AuthenticatePassword(ctx, "alice", "bob123")
		if got, want := err, ErrInvalidUsernameOrPassword; !errors.Is(got, want) {
			t.Fatalf("got %v error, want %v", got, want)
		}
	})
}
