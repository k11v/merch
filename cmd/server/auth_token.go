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
	return &AuthData{}, nil
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

type Token struct {
	UserID    uuid.UUID
	ExpiresAt time.Time
	IssuedAt  time.Time
	ID        uuid.UUID
}

func NewToken(userID uuid.UUID, expiresAt time.Time) *Token {
	return &Token{
		UserID:    userID,
		ExpiresAt: expiresAt,
		IssuedAt:  time.Now(),
		ID:        uuid.New(),
	}
}

func formatAndSignToken(token *Token, jwtSignatureKey ed25519.PrivateKey) (string, error) {
	jwtToken := jwt.NewWithClaims(jwt.SigningMethodEdDSA, jwt.RegisteredClaims{
		Subject:   token.UserID.String(),
		ExpiresAt: jwt.NewNumericDate(token.ExpiresAt), // TODO: Consider time zones.
		IssuedAt:  jwt.NewNumericDate(token.IssuedAt),  // TODO: Consider time zones.
		ID:        token.ID.String(),
	})
	return jwtToken.SignedString(jwtSignatureKey)
}

func parseAndVerifyToken(s string, jwtVerificationKey ed25519.PublicKey) (*Token, error) {
	jwtToken, err := jwt.ParseWithClaims(
		s,
		&jwt.RegisteredClaims{},
		func(t *jwt.Token) (any, error) {
			return jwtVerificationKey, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodEdDSA.Alg()}),
	)
	if err != nil {
		return nil, err
	}
	claims := jwtToken.Claims.(*jwt.RegisteredClaims)

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
	expiresAt := claims.ExpiresAt.Time

	if claims.IssuedAt == nil {
		return nil, errors.New("empty iat token claim")
	}
	issuedAt := claims.IssuedAt.Time

	if claims.ID == "" {
		return nil, errors.New("empty jti token claim")
	}
	id, err := uuid.Parse(claims.ID)
	if err != nil {
		return nil, fmt.Errorf("jti token claim: %w", err)
	}

	return &Token{
		UserID:    userID,
		ExpiresAt: expiresAt,
		IssuedAt:  issuedAt,
		ID:        id,
	}, nil
}
