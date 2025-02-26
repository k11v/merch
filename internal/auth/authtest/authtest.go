package authtest

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/k11v/merch/internal/app"
	"github.com/k11v/merch/internal/auth"
)

func CreateOrGetUserID(t testing.TB, ctx context.Context, db app.PgxExecutor, username string) (userID uuid.UUID) {
	t.Helper()

	ph := auth.NewPasswordHasher(auth.DefaultArgon2IDParams())
	pa := auth.NewPasswordAuthenticator(db, ph)
	password := username

	d, err := pa.AuthenticatePassword(ctx, username, password)
	if err != nil {
		t.Fatalf("can't authenticate password: %v", err)
	}

	return d.UserID
}
