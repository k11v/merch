package auth

import (
	"crypto/ed25519"
	"errors"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type TokenAuthenticator struct {
	jwtVerificationKey ed25519.PublicKey
}

func NewTokenAuthenticator(jwtVerificationKey ed25519.PublicKey) *TokenAuthenticator {
	return &TokenAuthenticator{jwtVerificationKey: jwtVerificationKey}
}

func (ta *TokenAuthenticator) AuthenticateToken(token string) (*Data, error) {
	jwtToken, err := jwt.ParseWithClaims(
		token,
		&jwt.RegisteredClaims{},
		func(t *jwt.Token) (any, error) {
			return ta.jwtVerificationKey, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodEdDSA.Alg()}),
	)
	if err != nil {
		return nil, fmt.Errorf("auth.TokenAuthenticator: %w", err)
	}
	claims, ok := jwtToken.Claims.(*jwt.RegisteredClaims)
	if !ok {
		return nil, errors.New("auth.TokenAuthenticator: invalid claims type")
	}

	if claims.Subject == "" {
		return nil, errors.New("auth.TokenAuthenticator: empty sub token claim")
	}
	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return nil, fmt.Errorf("auth.TokenAuthenticator: sub token claim: %w", err)
	}

	if claims.ExpiresAt == nil {
		return nil, errors.New("auth.TokenAuthenticator: empty exp token claim")
	}
	_ = claims.ExpiresAt.Time

	if claims.IssuedAt == nil {
		return nil, errors.New("auth.TokenAuthenticator: empty iat token claim")
	}
	_ = claims.IssuedAt.Time

	if claims.ID == "" {
		return nil, errors.New("auth.TokenAuthenticator: empty jti token claim")
	}
	_, err = uuid.Parse(claims.ID)
	if err != nil {
		return nil, fmt.Errorf("auth.TokenAuthenticator: jti token claim: %w", err)
	}

	return &Data{UserID: userID}, nil
}
