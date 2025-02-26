package main

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/k11v/merch/api/merch"
	"github.com/k11v/merch/internal/auth"
	"github.com/k11v/merch/internal/user"
)

type ContextValue string

const ContextValueUserID ContextValue = "UserID"

// PostAPIAuth implements merch.StrictServerInterface.
func (h *Handler) PostAPIAuth(ctx context.Context, request merch.PostAPIAuthRequestObject) (merch.PostAPIAuthResponseObject, error) {
	username := request.Body.Username
	if username == "" {
		errors := "empty username"
		return merch.PostAPIAuth400JSONResponse{Errors: &errors}, nil
	}

	password := request.Body.Password
	if password == "" {
		errors := "empty password"
		return merch.PostAPIAuth400JSONResponse{Errors: &errors}, nil
	}

	passwordHasher := user.NewPasswordHasher(user.DefaultArgon2IDParams())
	passwordAuthenticator := auth.NewPasswordAuthenticator(h.db, passwordHasher)
	authData, err := passwordAuthenticator.AuthenticatePassword(ctx, username, password)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidUsernameOrPassword) {
			errors := "invalid username or password"
			return merch.PostAPIAuth401JSONResponse{Errors: &errors}, nil
		}
		return nil, err
	}

	tokenIssuer := auth.NewTokenIssuer(h.jwtSignatureKey)
	token, err := tokenIssuer.IssueToken(authData.UserID)
	if err != nil {
		return nil, err
	}

	return merch.PostAPIAuth200JSONResponse{Token: &token}, nil
}

func Authentication(jwtVerificationKey ed25519.PublicKey) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.Method == "POST" && r.URL.Path == "/api/auth":
			default:
				const headerAuthorization = "Authorization"
				authorizationHeader := r.Header.Get(headerAuthorization)
				if authorizationHeader == "" {
					errors := fmt.Sprintf("empty %s header", headerAuthorization)
					response := merch.ErrorResponse{Errors: &errors}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusUnauthorized)
					err := json.NewEncoder(w).Encode(response)
					if err != nil {
						serveResponseError(w, r, err)
						return
					}
					return
				}

				scheme, params, found := strings.Cut(authorizationHeader, " ")
				if !strings.EqualFold(scheme, "Bearer") || !found {
					errors := fmt.Sprintf("invalid %s header scheme", headerAuthorization)
					response := merch.ErrorResponse{Errors: &errors}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusUnauthorized)
					err := json.NewEncoder(w).Encode(response)
					if err != nil {
						serveResponseError(w, r, err)
						return
					}
					return
				}

				tokenAuthenticator := auth.NewTokenAuthenticator(jwtVerificationKey)
				authData, err := tokenAuthenticator.AuthenticateToken(params)
				if err != nil {
					errors := fmt.Sprintf("%s header: %v", headerAuthorization, err)
					response := merch.ErrorResponse{Errors: &errors}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusUnauthorized)
					err = json.NewEncoder(w).Encode(response)
					if err != nil {
						serveResponseError(w, r, err)
						return
					}
					return
				}
				userID := authData.UserID

				ctx := r.Context()
				ctx = context.WithValue(ctx, ContextValueUserID, userID)
				r = r.WithContext(ctx)
			}
			next.ServeHTTP(w, r)
		})
	}
}
