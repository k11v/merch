package main

import (
	"context"
	"crypto/ed25519"
	"fmt"

	"github.com/k11v/merch/internal/app"
	"github.com/k11v/merch/internal/auth"
)

// GenerateAuthTokens generates and returns authentication tokens for the provided users.
// The tokens correspond to the users by their index in the slice.
func GenerateAuthTokens(ctx context.Context, db app.PgxExecutor, jwtSignatureKey ed25519.PrivateKey, users []*User) ([]string, error) {
	tokenIssuer := auth.NewTokenIssuer(jwtSignatureKey)

	authTokens := make([]string, len(users))
	for i, u := range users {
		token, err := tokenIssuer.IssueToken(u.ID)
		if err != nil {
			return nil, fmt.Errorf("GenerateAuthTokens: %w", err)
		}
		authTokens[i] = token
	}

	return authTokens, nil
}
