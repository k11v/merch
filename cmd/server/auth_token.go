package main

import (
	"crypto/ed25519"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type TokenAuthenticator struct {
	jwtVerificationKey ed25519.PublicKey
}

func NewTokenAuthenticator(jwtVerificationKey ed25519.PublicKey) *TokenAuthenticator {
	return &TokenAuthenticator{jwtVerificationKey: jwtVerificationKey}
}

func (ta *TokenAuthenticator) AuthenticateToken(token string) (*AuthData, error) {
	jwtToken, err := jwt.ParseWithClaims(
		token,
		&jwt.RegisteredClaims{},
		func(t *jwt.Token) (any, error) {
			return ta.jwtVerificationKey, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodEdDSA.Alg()}),
	)
	if err != nil {
		return nil, err
	}
	claims, ok := jwtToken.Claims.(*jwt.RegisteredClaims)
	if !ok {
		return nil, errors.New("invalid claims type")
	}

	if claims.Subject == "" {
		return nil, errors.New("empty sub token claim")
	}
	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return nil, fmt.Errorf("sub token claim: %w", err)
	}

	if claims.ExpiresAt == nil {
		return nil, errors.New("empty exp token claim")
	}
	_ = claims.ExpiresAt.Time

	if claims.IssuedAt == nil {
		return nil, errors.New("empty iat token claim")
	}
	_ = claims.IssuedAt.Time

	if claims.ID == "" {
		return nil, errors.New("empty jti token claim")
	}
	_, err = uuid.Parse(claims.ID)
	if err != nil {
		return nil, fmt.Errorf("jti token claim: %w", err)
	}

	return &AuthData{UserID: userID}, nil
}

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
