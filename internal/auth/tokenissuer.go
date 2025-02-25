package auth

import (
	"crypto/ed25519"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type TokenIssuer struct {
	jwtSignatureKey ed25519.PrivateKey
}

func NewTokenIssuer(jwtSignatureKey ed25519.PrivateKey) *TokenIssuer {
	return &TokenIssuer{jwtSignatureKey: jwtSignatureKey}
}

func (ti *TokenIssuer) IssueToken(userID uuid.UUID) (string, error) {
	id := uuid.New()
	issuedAt := time.Now()
	expiresAt := issuedAt.Add(time.Hour)
	claims := jwt.RegisteredClaims{
		Subject:   userID.String(),
		ExpiresAt: jwt.NewNumericDate(expiresAt),
		IssuedAt:  jwt.NewNumericDate(issuedAt),
		ID:        id.String(),
	}
	unsigned := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	signed, err := unsigned.SignedString(ti.jwtSignatureKey)
	if err != nil {
		return "", fmt.Errorf("TokenIssuer: %w", err)
	}
	return signed, err
}
