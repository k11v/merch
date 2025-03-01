package main

import (
	"context"
	"crypto/ed25519"
	"fmt"

	"github.com/k11v/merch/internal/app"
	"github.com/k11v/merch/internal/auth"
)

func GenerateAuthTokens(ctx context.Context, db app.PgxExecutor, jwtSignatureKey ed25519.PrivateKey, users map[string]*User) (map[string]string, error) {
	tokenIssuer := auth.NewTokenIssuer(jwtSignatureKey)

	authTokens := make(map[string]string, len(users))
	for _, u := range users {
		token, err := tokenIssuer.IssueToken(u.ID)
		if err != nil {
			return nil, fmt.Errorf("GenerateAuthTokens: %w", err)
		}
		authTokens[u.Username] = token
	}

	return authTokens, nil
}
