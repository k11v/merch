package usertest

import (
	"context"
	"testing"

	"github.com/k11v/merch/internal/app"
	"github.com/k11v/merch/internal/user"
)

const DefaultPassword = "password"

// CreateUser creates a user with [DefaultPassword].
func CreateUser(t testing.TB, ctx context.Context, db app.PgxExecutor, username string) *user.User {
	t.Helper()

	ph := user.NewPasswordHasher(user.DefaultArgon2IDParams())
	uc := user.NewCreator(db, ph)

	u, err := uc.CreateUser(ctx, username, DefaultPassword)
	if err != nil {
		t.Fatalf("can't create user: %v", err)
	}

	return u
}
