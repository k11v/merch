package usertest

import (
	"context"
	"testing"

	"github.com/k11v/merch/internal/app"
	"github.com/k11v/merch/internal/user"
)

func CreateUser(t testing.TB, ctx context.Context, db app.PgxExecutor, username string) *user.User {
	t.Helper()

	ph := user.NewPasswordHasher(user.DefaultArgon2IDParams())
	uc := user.NewCreator(db, ph)
	password := username

	u, err := uc.CreateUser(ctx, username, password)
	if err != nil {
		t.Fatalf("can't create user: %v", err)
	}

	return u
}
