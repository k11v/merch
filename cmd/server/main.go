package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k11v/merch/api/merch"
	"github.com/k11v/merch/internal/app"
)

type ContextValue string

const ContextValueUserID ContextValue = "UserID"

func main() {
	const envHost = "APP_HOST"
	host := os.Getenv(envHost)
	if host == "" {
		host = "127.0.0.1"
	}

	const envPort = "APP_PORT"
	port := 0
	portEnv := os.Getenv(envPort)
	if portEnv != "" {
		var err error
		port, err = strconv.Atoi(portEnv)
		if err != nil {
			err = fmt.Errorf("%s env: %w", envPort, err)
			_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	}
	if port == 0 {
		port = 8080
	}

	const envPostgresURL = "APP_POSTGRES_URL"
	postgresURL := os.Getenv(envPostgresURL)
	if postgresURL == "" {
		err := fmt.Errorf("%s env is empty", envPostgresURL)
		_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	err := run(host, port, postgresURL)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	os.Exit(0)
}

func run(host string, port int, postgresURL string) error {
	ctx := context.Background()

	postgresPool, err := app.NewPostgresPool(ctx, postgresURL)
	if err != nil {
		return err
	}
	defer postgresPool.Close()

	httpServer := newHTTPServer(postgresPool, host, port)

	slog.Info("starting HTTP server", "addr", httpServer.Addr)
	err = httpServer.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

func newHTTPServer(db *pgxpool.Pool, host string, port int) *http.Server {
	handler := NewHandler(db)

	mux := http.NewServeMux()
	ssi := merch.StrictServerInterface(handler)
	si := merch.NewStrictHandlerWithOptions(ssi, nil, merch.StrictHTTPServerOptions{
		RequestErrorHandlerFunc:  serveRequestError,
		ResponseErrorHandlerFunc: serveResponseError,
	})
	h := merch.HandlerWithOptions(si, merch.StdHTTPServerOptions{
		BaseRouter:       mux,
		ErrorHandlerFunc: serveRequestError,
	})

	middlewares := []func(next http.Handler) http.Handler{
		Authenticator(),
	}
	for _, m := range middlewares {
		h = m(h)
	}

	addr := net.JoinHostPort(host, strconv.Itoa(port))
	logLogger := slog.NewLogLogger(slog.Default().Handler(), slog.LevelError)

	return &http.Server{
		Addr:     addr,
		Handler:  h,
		ErrorLog: logLogger,
	}
}

var _ merch.StrictServerInterface = (*Handler)(nil)

type Handler struct {
	db *pgxpool.Pool
}

func NewHandler(db *pgxpool.Pool) *Handler {
	return &Handler{db: db}
}

// GetAPIBuyItem implements merch.StrictServerInterface.
func (h *Handler) GetAPIBuyItem(ctx context.Context, request merch.GetAPIBuyItemRequestObject) (merch.GetAPIBuyItemResponseObject, error) {
	return merch.GetAPIBuyItem200Response{}, nil
}

// GetAPIInfo implements merch.StrictServerInterface.
func (h *Handler) GetAPIInfo(ctx context.Context, request merch.GetAPIInfoRequestObject) (merch.GetAPIInfoResponseObject, error) {
	return merch.GetAPIInfo200JSONResponse{}, nil
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
	passwordHash := "fakeHash(" + request.Body.Password + ")"

	user, err := createUser(ctx, h.db, username, passwordHash)
	if errors.Is(err, ErrUserExist) {
		user, err = getUserByUsername(ctx, h.db, username)
	}
	if err != nil {
		return nil, err
	}

	if user.PasswordHash != passwordHash {
		errors := "invalid username or password"
		return merch.PostAPIAuth401JSONResponse{Errors: &errors}, nil
	}

	token := "fakeToken(" + user.ID.String() + ")"
	return merch.PostAPIAuth200JSONResponse{Token: &token}, nil
}

// PostAPISendCoin implements merch.StrictServerInterface.
func (h *Handler) PostAPISendCoin(ctx context.Context, request merch.PostAPISendCoinRequestObject) (merch.PostAPISendCoinResponseObject, error) {
	toUsername := request.Body.ToUser
	if toUsername == "" {
		errors := "empty toUser body value"
		return merch.PostAPISendCoin400JSONResponse{Errors: &errors}, nil
	}

	amount := request.Body.Amount
	if amount <= 0 {
		errors := "non-positive amount body value"
		return merch.PostAPISendCoin400JSONResponse{Errors: &errors}, nil
	}

	currentUserID, ok := ctx.Value(ContextValueUserID).(uuid.UUID)
	if !ok {
		panic(fmt.Errorf("can't get %s context value", ContextValueUserID))
	}

	fromUserID := currentUserID

	toUser, err := getUserByUsername(ctx, h.db, toUsername)
	if err != nil {
		if errors.Is(err, ErrUserNotExist) {
			errors := "toUser doesn't exist"
			return merch.PostAPISendCoin400JSONResponse{Errors: &errors}, nil
		}
		return nil, err
	}
	toUserID := toUser.ID

	if fromUserID == toUserID {
		errors := "identical fromUser and toUser"
		return merch.PostAPISendCoin400JSONResponse{Errors: &errors}, nil
	}

	tx, err := h.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = tx.Rollback(ctx)
		if err != nil {
			slog.Error("didn't rollback", "err", err)
		}
	}()

	fromUser, err := getUserForUpdate(ctx, tx, fromUserID)
	if err != nil {
		return nil, err
	}

	toUser, err = getUserForUpdate(ctx, tx, toUserID)
	if err != nil {
		return nil, err
	}

	fromUserBalance := fromUser.Balance
	fromUserBalance -= amount
	if fromUserBalance < 0 {
		fmt.Println(fromUser)
		errors := "not enough coins"
		return merch.PostAPISendCoin400JSONResponse{Errors: &errors}, nil
	}

	toUserBalance := toUser.Balance
	toUserBalance += amount

	_, err = updateUserBalance(ctx, tx, fromUserID, fromUserBalance)
	if err != nil {
		return nil, err
	}

	_, err = updateUserBalance(ctx, tx, toUserID, toUserBalance)
	if err != nil {
		return nil, err
	}

	_, err = createTransaction(ctx, tx, fromUserID, toUserID, amount)
	if err != nil {
		return nil, err
	}

	err = tx.Commit(ctx)
	if err != nil {
		return nil, err
	}

	return merch.PostAPISendCoin200Response{}, nil
}

type pgxExecutor interface {
	Begin(ctx context.Context) (pgx.Tx, error)
	Exec(ctx context.Context, sql string, arguments ...any) (commandTag pgconn.CommandTag, err error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults
}

var (
	ErrUserExist    = errors.New("user already exists")
	ErrUserNotExist = errors.New("user does not exist")
)

type User struct {
	ID           uuid.UUID
	Username     string
	PasswordHash string
	Balance      int
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

type Transaction struct {
	ID         uuid.UUID
	FromUserID uuid.UUID
	ToUserID   uuid.UUID
	Amount     int
}

func rowToTransaction(collectable pgx.CollectableRow) (*Transaction, error) {
	type row struct {
		ID         uuid.UUID `db:"id"`
		FromUserID uuid.UUID `db:"from_user_id"`
		ToUserID   uuid.UUID `db:"to_user_id"`
		Amount     int       `db:"amount"`
	}

	collected, err := pgx.RowToStructByName[row](collectable)
	if err != nil {
		return nil, err
	}

	return &Transaction{
		ID:         collected.ID,
		FromUserID: collected.FromUserID,
		ToUserID:   collected.ToUserID,
		Amount:     collected.Amount,
	}, nil
}

func getUserForUpdate(ctx context.Context, db pgxExecutor, id uuid.UUID) (*User, error) {
	query := `
		SELECT id, username, password_hash, balance
		FROM users
		WHERE id = $1
		FOR UPDATE
	`
	args := []any{id}

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

func updateUserBalance(ctx context.Context, db pgxExecutor, id uuid.UUID, balance int) (*User, error) {
	query := `
		UPDATE users
		SET balance = $2
		WHERE id = $1
		RETURNING id, username, password_hash, balance
	`
	args := []any{id, balance}

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

func createTransaction(ctx context.Context, db pgxExecutor, fromUserID, toUserID uuid.UUID, amount int) (*Transaction, error) {
	query := `
		INSERT INTO transactions (from_user_id, to_user_id, amount)
		VALUES ($1, $2, $3)
		RETURNING id, from_user_id, to_user_id, amount
	`
	args := []any{fromUserID, toUserID, amount}

	rows, _ := db.Query(ctx, query, args...)
	transaction, err := pgx.CollectExactlyOneRow(rows, rowToTransaction)
	if err != nil {
		return nil, err
	}

	return transaction, nil
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

func serveRequestError(w http.ResponseWriter, r *http.Request, err error) {
	errors := new(string)
	*errors = err.Error()
	response := merch.ErrorResponse{Errors: errors}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	encodeErr := json.NewEncoder(w).Encode(response)
	if encodeErr != nil {
		serveResponseError(w, r, encodeErr)
	}
}

func serveResponseError(w http.ResponseWriter, r *http.Request, err error) {
	slog.Error("server error", "err", err)

	errors := new(string)
	*errors = "internal server error"
	response := merch.ErrorResponse{Errors: errors}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	_ = json.NewEncoder(w).Encode(response)
}

func Authenticator() func(next http.Handler) http.Handler {
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
					w.WriteHeader(http.StatusBadRequest)
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
					w.WriteHeader(http.StatusBadRequest)
					err := json.NewEncoder(w).Encode(response)
					if err != nil {
						serveResponseError(w, r, err)
						return
					}
					return
				}

				var userID uuid.UUID
				var err error
				if strings.HasPrefix(params, "fakeToken(") && strings.HasSuffix(params, ")") {
					userID, err = uuid.Parse(params[10 : len(params)-1])
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
				} else {
					errors := fmt.Sprintf("invalid %s header token", headerAuthorization)
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

				ctx := r.Context()
				ctx = context.WithValue(ctx, ContextValueUserID, userID)
				r = r.WithContext(ctx)
			}
			next.ServeHTTP(w, r)
		})
	}
}
