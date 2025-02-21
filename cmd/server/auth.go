package main

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k11v/merch/api/merch"
)

var (
	ErrPasswordNotMatch = errors.New("password does not match hash")
	ErrUserExist        = errors.New("user already exists")
	ErrUserNotExist     = errors.New("user does not exist")
)

type ContextValue string

const ContextValueUserID ContextValue = "UserID"

type User struct {
	ID           uuid.UUID
	Username     string
	PasswordHash string
	Balance      int
}

// PostAPIAuth implements merch.StrictServerInterface.
func (h *Handler) PostAPIAuth(ctx context.Context, request merch.PostAPIAuthRequestObject) (merch.PostAPIAuthResponseObject, error) {
	if request.Body.Username == "" {
		errors := "empty username"
		return merch.PostAPIAuth400JSONResponse{Errors: &errors}, nil
	}
	username := request.Body.Username

	if request.Body.Password == "" {
		errors := "empty password"
		return merch.PostAPIAuth400JSONResponse{Errors: &errors}, nil
	}
	password := request.Body.Password

	authenticator := NewAuthenticator(h.db, h.jwtSignatureKey)
	authData, err := authenticator.AuthenticateWithUsernameAndPassword(ctx, username, password)
	if err != nil {
		if errors.Is(err, ErrInvalidPassword) {
			errors := "invalid username or password"
			return merch.PostAPIAuth401JSONResponse{Errors: &errors}, nil
		}
		return nil, err
	}

	tokenIssuer := NewTokenIssuer(h.jwtSignatureKey)
	token, err := tokenIssuer.IssueToken(authData.UserID)
	if err != nil {
		return nil, err
	}

	return merch.PostAPIAuth200JSONResponse{Token: &token}, nil
}

var ErrInvalidPassword = errors.New("invalid password")

type AuthData struct {
	UserID uuid.UUID
}

type Authenticator struct {
	db              *pgxpool.Pool
	jwtSignatureKey ed25519.PrivateKey
}

func NewAuthenticator(db *pgxpool.Pool, jwtSignatureKey ed25519.PrivateKey) *Authenticator {
	return &Authenticator{db: db, jwtSignatureKey: jwtSignatureKey}
}

func (a *Authenticator) AuthenticateWithUsernameAndPassword(ctx context.Context, username, password string) (*AuthData, error) {
	// HACK: Race condition.
	user, err := getUserByUsername(ctx, a.db, username)
	if err == nil {
		err = VerifyPasswordArgon2ID(password, user.PasswordHash)
		if err != nil {
			return nil, err
		}
	} else if errors.Is(err, ErrUserNotExist) {
		tx, err := a.db.Begin(ctx)
		if err != nil {
			return nil, err
		}
		defer func() {
			err = tx.Rollback(ctx)
			if err != nil && !errors.Is(err, pgx.ErrTxClosed) {
				slog.Error("didn't rollback", "err", err)
			}
		}()

		initialBalance := 1000
		passwordHash, err := HashPasswordArgon2ID(password, DefaultArgon2IDParams())
		if err != nil {
			return nil, err
		}

		user, err = createUser(ctx, tx, username, passwordHash)
		if err != nil {
			return nil, err
		}
		user, err = updateUserBalance(ctx, tx, user.ID, initialBalance)
		if err != nil {
			return nil, err
		}
		_, err = createTransaction(ctx, tx, nil, &user.ID, initialBalance)
		if err != nil {
			return nil, err
		}

		err = tx.Commit(ctx)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, err
	}
	return &AuthData{UserID: user.ID}, nil
}

func (a *Authenticator) AuthenticateWithToken(token string) (*AuthData, error) {
	return &AuthData{}, nil
}

type PasswordHasher struct{}

func (ph *PasswordHasher) Hash(password string) (string, error) {
	return "", nil
}

func (ph *PasswordHasher) Verify(password, passwordHash string) error {
	return nil
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

				token, err := parseAndVerifyToken(params, jwtVerificationKey)
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
				userID := token.userID

				ctx := r.Context()
				ctx = context.WithValue(ctx, ContextValueUserID, userID)
				r = r.WithContext(ctx)
			}
			next.ServeHTTP(w, r)
		})
	}
}

func createUser(ctx context.Context, db pgxExecutor, username string, passwordHash string) (*User, error) {
	query := `
		INSERT INTO users (username, password_hash)
		VALUES ($1, $2)
		RETURNING id, username, password_hash, balance
	`
	args := []any{username, passwordHash}

	rows, _ := db.Query(ctx, query, args...)
	user, err := pgx.CollectExactlyOneRow(rows, rowToUser)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgerrcode.IsIntegrityConstraintViolation(pgErr.Code) && pgErr.ConstraintName == "users_username_idx" {
			return nil, ErrUserExist
		}
		return nil, err
	}

	return user, nil
}

func getUserByUsername(ctx context.Context, db pgxExecutor, username string) (*User, error) {
	query := `
		SELECT id, username, password_hash, balance
		FROM users
		WHERE username = $1
	`
	args := []any{username}

	rows, _ := db.Query(ctx, query, args...)
	user, err := pgx.CollectExactlyOneRow(rows, rowToUser)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotExist
		}
		return nil, err
	}

	return user, nil
}

func rowToUser(collectable pgx.CollectableRow) (*User, error) {
	type row struct {
		ID           uuid.UUID `db:"id"`
		Username     string    `db:"username"`
		PasswordHash string    `db:"password_hash"`
		Balance      int       `db:"balance"`
	}

	collected, err := pgx.RowToStructByName[row](collectable)
	if err != nil {
		return nil, err
	}

	return &User{
		ID:           collected.ID,
		Username:     collected.Username,
		PasswordHash: collected.PasswordHash,
		Balance:      collected.Balance,
	}, nil
}
