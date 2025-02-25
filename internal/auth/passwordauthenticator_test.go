package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k11v/merch/internal/app"
	"github.com/k11v/merch/internal/app/apptest"
)

func TestPasswordAuthenticator(t *testing.T) {
	var (
		ctx = context.Background()
		db  = newTestPgxPool(t, ctx)
	)

	t.Run("creates different users", func(t *testing.T) {
		var (
			tx = newTestPgxTx(t, ctx, db)
			ph = NewPasswordHasher(DefaultArgon2IDParams())
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
			tx = newTestPgxTx(t, ctx, db)
			ph = NewPasswordHasher(DefaultArgon2IDParams())
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
			tx = newTestPgxTx(t, ctx, db)
			ph = NewPasswordHasher(DefaultArgon2IDParams())
			pa = NewPasswordAuthenticator(tx, ph)
		)

		_, err := pa.AuthenticatePassword(ctx, "alice", "alice123")
		if err != nil {
			t.Fatalf("got %v error", err)
		}

		_, err = pa.AuthenticatePassword(ctx, "alice", "bob123")
		if got, want := err, ErrPasswordNotMatch; !errors.Is(got, want) {
			t.Fatalf("got %v error, want %v", got, want)
		}
	})

	t.Run("creates user with initial balance", func(t *testing.T) {
		var (
			tx = newTestPgxTx(t, ctx, db)
			ph = NewPasswordHasher(DefaultArgon2IDParams())
			pa = NewPasswordAuthenticator(tx, ph)
		)

		_, err := pa.AuthenticatePassword(ctx, "alice", "alice123")
		if err != nil {
			t.Fatalf("got %v error", err)
		}

		// TODO: Find a public type/func to use instead.
		user, err := getUserByUsername(ctx, tx, "alice")
		if err != nil {
			t.Fatalf("got %v error", err)
		}

		if got, want := user.Balance, InitialBalance; got != want {
			t.Fatalf("got %d balance, want %d", got, want)
		}
	})
}

func newTestPgxPool(t testing.TB, ctx context.Context) *pgxpool.Pool {
	t.Helper()

	connectionString, teardown, err := apptest.SetupPostgres(ctx)
	if err != nil {
		t.Fatalf("can't setup Postgres: %v", err)
	}
	t.Cleanup(func() {
		if teardownErr := teardown(); teardownErr != nil {
			t.Errorf("didn't teardown Postgres: %v", teardownErr)
		}
	})

	pool, err := app.NewPostgresPool(ctx, connectionString)
	if err != nil {
		t.Fatalf("can't create Postgres pool: %v", err)
	}
	t.Cleanup(pool.Close)

	return pool
}

// newTestPgxTx begins a new Postgres transaction and returns it.
// It shouldn't be committed or rolled back manually.
// It is automatically rolled back on test cleanup.
func newTestPgxTx(t testing.TB, ctx context.Context, db *pgxpool.Pool) pgx.Tx {
	t.Helper()

	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatalf("can't begin Postgres transaction: %v", err)
	}
	t.Cleanup(func() {
		rollbackErr := tx.Rollback(ctx)
		if rollbackErr != nil {
			t.Errorf("didn't rollback Postgres transaction: %v", err)
		}
	})

	return tx
}
