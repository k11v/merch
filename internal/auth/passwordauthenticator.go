package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/k11v/merch/internal/app"
	"github.com/k11v/merch/internal/user"
)

var ErrInvalidUsernameOrPassword = errors.New("invalid username or password")

type User struct {
	ID           uuid.UUID
	Username     string
	PasswordHash string
	Balance      int
}

// Data represents authenticated data.
type Data struct {
	UserID uuid.UUID
}

type PasswordAuthenticator struct {
	db app.PgxExecutor
	ph *user.PasswordHasher
}

func NewPasswordAuthenticator(db app.PgxExecutor, passwordHasher *user.PasswordHasher) *PasswordAuthenticator {
	return &PasswordAuthenticator{db: db, ph: passwordHasher}
}

func (pa *PasswordAuthenticator) AuthenticatePassword(ctx context.Context, username, password string) (*Data, error) {
	u, err := user.NewGetter(pa.db).GetUserByUsername(ctx, username)
	switch {
	case err == nil:
		err = pa.ph.Verify(password, u.PasswordHash)
		if err != nil {
			if errors.Is(err, user.ErrPasswordNotMatch) {
				return nil, fmt.Errorf("auth.PasswordAuthenticator: %w", ErrInvalidUsernameOrPassword)
			}
			return nil, fmt.Errorf("auth.PasswordAuthenticator: %w", err)
		}
	case errors.Is(err, user.ErrNotExist):
		u, err = user.NewCreator(pa.db, pa.ph).CreateUser(ctx, username, password)
		if errors.Is(err, user.ErrExist) {
			// When two callers try to authenticate under the same username for
			// the first time, both of them can fail GetUserByUsername with
			// ErrNotExist but only one of them can succeed CreateUser.
			u, err = user.NewGetter(pa.db).GetUserByUsername(ctx, username)
			if err != nil {
				return nil, fmt.Errorf("auth.PasswordAuthenticator: %w", err)
			}
		} else if err != nil {
			return nil, fmt.Errorf("auth.PasswordAuthenticator: %w", err)
		}
	default:
		return nil, fmt.Errorf("auth.PasswordAuthenticator: %w", err)
	}
	return &Data{UserID: u.ID}, nil
}
