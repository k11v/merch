package main

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"log/slog"

	"github.com/k11v/merch/internal/app"
	"github.com/k11v/merch/internal/auth"
	"github.com/k11v/merch/internal/user"
)

type AuthToken struct {
	Username string `json:"username"`
	Token    string `json:"token"`
}

func GenerateAuthTokens(ctx context.Context, db app.PgxExecutor, jwtSignatureKey ed25519.PrivateKey, users []*User, count int) ([]*AuthToken, error) {
	if len(users) < count {
		return nil, errors.New("GenerateAuthTokens: not enough users")
	}

	userGetter := user.NewGetter(db)
	tokenIssuer := auth.NewTokenIssuer(jwtSignatureKey)

	authTokens := make([]*AuthToken, count)
	for i, u := range users[:count] {
		gotUser, err := userGetter.GetUserByUsername(ctx, u.Username)
		if err != nil {
			return nil, fmt.Errorf("GenerateAuthTokens: %w", err)
		}
		token, err := tokenIssuer.IssueToken(gotUser.ID)
		if err != nil {
			return nil, fmt.Errorf("GenerateAuthTokens: %w", err)
		}
		authTokens[i] = &AuthToken{
			Username: gotUser.Username,
			Token:    token,
		}
		if c := i + 1; c%1000 == 0 && c < count {
			slog.Info("generated some auth tokens", "count", c)
		}
	}

	return authTokens, nil
}
