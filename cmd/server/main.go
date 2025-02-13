package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/argon2"

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

	const envJWTVerificationKeyFile = "APP_JWT_VERIFICATION_KEY_FILE"
	jwtVerificationKeyFile := os.Getenv(envJWTVerificationKeyFile)
	if jwtVerificationKeyFile == "" {
		err := fmt.Errorf("%s env is empty", envJWTVerificationKeyFile)
		_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	const envJWTSignatureKeyFile = "APP_JWT_SIGNATURE_KEY_FILE"
	jwtSignatureKeyFile := os.Getenv(envJWTSignatureKeyFile)
	if jwtSignatureKeyFile == "" {
		err := fmt.Errorf("%s env is empty", envJWTSignatureKeyFile)
		_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	err := run(host, port, postgresURL, jwtVerificationKeyFile, jwtSignatureKeyFile)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	os.Exit(0)
}

func run(host string, port int, postgresURL, jwtVerificationKeyFile, jwtSignatureKeyFile string) error {
	ctx := context.Background()

	postgresPool, err := app.NewPostgresPool(ctx, postgresURL)
	if err != nil {
		return err
	}
	defer postgresPool.Close()

	jwtVerificationKey, err := readFileWithED25519PublicKey(jwtVerificationKeyFile)
	if err != nil {
		return err
	}

	jwtSignatureKey, err := readFileWithED25519PrivateKey(jwtSignatureKeyFile)
	if err != nil {
		return err
	}

	httpServer := newHTTPServer(postgresPool, host, port, jwtVerificationKey, jwtSignatureKey)

	slog.Info("starting HTTP server", "addr", httpServer.Addr)
	err = httpServer.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

func newHTTPServer(db *pgxpool.Pool, host string, port int, jwtVerificationKey ed25519.PublicKey, jwtSignatureKey ed25519.PrivateKey) *http.Server {
	handler := NewHandler(db, jwtSignatureKey)

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
		Authenticator(jwtVerificationKey),
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
	db              *pgxpool.Pool
	jwtSignatureKey ed25519.PrivateKey
}

func NewHandler(db *pgxpool.Pool, jwtSignatureKey ed25519.PrivateKey) *Handler {
	return &Handler{db: db, jwtSignatureKey: jwtSignatureKey}
}

// GetAPIBuyItem implements merch.StrictServerInterface.
func (h *Handler) GetAPIBuyItem(ctx context.Context, request merch.GetAPIBuyItemRequestObject) (merch.GetAPIBuyItemResponseObject, error) {
	userID, ok := ctx.Value(ContextValueUserID).(uuid.UUID)
	if !ok {
		panic(fmt.Errorf("can't get %s context value", ContextValueUserID))
	}

	itemName := request.Item
	if itemName == "" {
		errors := "empty item"
		return merch.GetAPIBuyItem400JSONResponse{Errors: &errors}, nil
	}

	item, err := getItemByName(ctx, h.db, itemName)
	if err != nil {
		if errors.Is(err, ErrItemNotExist) {
			errors := "item does not exist"
			return merch.GetAPIBuyItem400JSONResponse{Errors: &errors}, nil
		}
		return nil, err
	}

	tx, err := h.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = tx.Rollback(ctx)
		if err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			slog.Error("didn't rollback", "err", err)
		}
	}()

	user, err := getUserForUpdate(ctx, tx, userID)
	if err != nil {
		return nil, err
	}

	balance := user.Balance
	balance -= item.Price
	if balance < 0 {
		errors := "not enough coins"
		return merch.GetAPIBuyItem400JSONResponse{Errors: &errors}, nil
	}

	_, err = updateUserBalance(ctx, tx, userID, balance)
	if err != nil {
		return nil, err
	}

	_, err = createTransaction(ctx, tx, &userID, nil, item.Price)
	if err != nil {
		return nil, err
	}

	_, err = addUserItemAmount(ctx, tx, userID, item.ID, 1)
	if err != nil {
		return nil, err
	}

	err = tx.Commit(ctx)
	if err != nil {
		return nil, err
	}

	return merch.GetAPIBuyItem200Response{}, nil
}

// GetAPIInfo implements merch.StrictServerInterface.
func (h *Handler) GetAPIInfo(ctx context.Context, request merch.GetAPIInfoRequestObject) (merch.GetAPIInfoResponseObject, error) {
	userID, ok := ctx.Value(ContextValueUserID).(uuid.UUID)
	if !ok {
		panic(fmt.Errorf("can't get %s context value", ContextValueUserID))
	}

	user, err := getUser(ctx, h.db, userID)
	if err != nil {
		return nil, err
	}
	transactions, err := getTransactionsByUserID(ctx, h.db, userID)
	if err != nil {
		return nil, err
	}
	userItems, err := getUserItems(ctx, h.db, userID)
	if err != nil {
		return nil, err
	}

	coins := user.Balance

	type receivedHistoryItem = struct {
		Amount   *int    `json:"amount,omitempty"`
		FromUser *string `json:"fromUser,omitempty"`
	}
	type sentHistoryItem = struct {
		Amount *int    `json:"amount,omitempty"`
		ToUser *string `json:"toUser,omitempty"`
	}
	type history = struct {
		Received *[]receivedHistoryItem `json:"received,omitempty"`
		Sent     *[]sentHistoryItem     `json:"sent,omitempty"`
	}
	received := make([]receivedHistoryItem, 0)
	sent := make([]sentHistoryItem, 0)
	for _, transaction := range transactions {
		fromUserID := transaction.FromUserID
		toUserID := transaction.ToUserID
		if fromUserID == nil || toUserID == nil {
			continue
		}
		if *fromUserID == userID {
			sent = append(sent, sentHistoryItem{
				Amount: &transaction.Amount,
				ToUser: transaction.ToUsername,
			})
		}
		if *toUserID == userID {
			received = append(received, receivedHistoryItem{
				Amount:   &transaction.Amount,
				FromUser: transaction.FromUsername,
			})
		}
	}

	type inventoryItem = struct {
		Quantity *int    `json:"quantity,omitempty"`
		Type     *string `json:"type,omitempty"`
	}
	inventory := make([]inventoryItem, len(userItems))
	for i, userItem := range userItems {
		inventory[i] = inventoryItem{
			Quantity: &userItem.Amount,
			Type:     &userItem.ItemName,
		}
	}

	return merch.GetAPIInfo200JSONResponse{
		CoinHistory: &history{
			Received: &received,
			Sent:     &sent,
		},
		Coins:     &coins,
		Inventory: &inventory,
	}, nil
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

	// HACK: Race condition.
	user, err := getUserByUsername(ctx, h.db, username)
	if errors.Is(err, ErrUserNotExist) {
		tx, err := h.db.Begin(ctx)
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
	} else if err != nil {
		return nil, err
	}

	if user.PasswordHash != passwordHash {
		errors := "invalid username or password"
		return merch.PostAPIAuth401JSONResponse{Errors: &errors}, nil
	}

	token := Token{
		UserID:    user.ID,
		ExpiresAt: time.Now().Add(time.Hour),
		IssuedAt:  time.Now(),
		ID:        uuid.New(),
	}
	tokenString, err := formatAndSignToken(&token, h.jwtSignatureKey)
	if err != nil {
		return nil, err
	}

	return merch.PostAPIAuth200JSONResponse{Token: &tokenString}, nil
}

// PostAPISendCoin implements merch.StrictServerInterface.
func (h *Handler) PostAPISendCoin(ctx context.Context, request merch.PostAPISendCoinRequestObject) (merch.PostAPISendCoinResponseObject, error) {
	requestUserID, ok := ctx.Value(ContextValueUserID).(uuid.UUID)
	if !ok {
		panic(fmt.Errorf("can't get %s context value", ContextValueUserID))
	}

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

	fromUserID := requestUserID

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
		if err != nil && !errors.Is(err, pgx.ErrTxClosed) {
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

	_, err = createTransaction(ctx, tx, &fromUserID, &toUserID, amount)
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
	ErrItemNotExist = errors.New("item does not exist")
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
	FromUserID *uuid.UUID
	ToUserID   *uuid.UUID
	Amount     int

	FromUsername *string
	ToUsername   *string
}

type Item struct {
	ID    uuid.UUID
	Name  string
	Price int
}

func rowToTransaction(collectable pgx.CollectableRow) (*Transaction, error) {
	type row struct {
		ID         uuid.UUID  `db:"id"`
		FromUserID *uuid.UUID `db:"from_user_id"`
		ToUserID   *uuid.UUID `db:"to_user_id"`
		Amount     int        `db:"amount"`
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

func rowToTransactionWithUsernames(collectable pgx.CollectableRow) (*Transaction, error) {
	type row struct {
		ID         uuid.UUID  `db:"id"`
		FromUserID *uuid.UUID `db:"from_user_id"`
		ToUserID   *uuid.UUID `db:"to_user_id"`
		Amount     int        `db:"amount"`

		FromUsername *string `db:"from_username"`
		ToUsername   *string `db:"to_username"`
	}

	collected, err := pgx.RowToStructByName[row](collectable)
	if err != nil {
		return nil, err
	}

	return &Transaction{
		ID:           collected.ID,
		FromUserID:   collected.FromUserID,
		ToUserID:     collected.ToUserID,
		Amount:       collected.Amount,
		FromUsername: collected.FromUsername,
		ToUsername:   collected.ToUsername,
	}, nil
}

func rowToItem(collectable pgx.CollectableRow) (*Item, error) {
	type row struct {
		ID    uuid.UUID `db:"id"`
		Name  string    `db:"name"`
		Price int       `db:"price"`
	}

	collected, err := pgx.RowToStructByName[row](collectable)
	if err != nil {
		return nil, err
	}

	return &Item{
		ID:    collected.ID,
		Name:  collected.Name,
		Price: collected.Price,
	}, nil
}

type UserItem struct {
	UserID   uuid.UUID
	ItemID   uuid.UUID
	ItemName string
	Amount   int
}

func rowToUserItem(collectable pgx.CollectableRow) (*UserItem, error) {
	type row struct {
		UserID uuid.UUID `db:"user_id"`
		ItemID uuid.UUID `db:"item_id"`
		Amount int       `db:"amount"`
	}

	collected, err := pgx.RowToStructByName[row](collectable)
	if err != nil {
		return nil, err
	}

	return &UserItem{
		UserID: collected.UserID,
		ItemID: collected.ItemID,
		Amount: collected.Amount,
	}, nil
}

func rowToUserItemWithName(collectable pgx.CollectableRow) (*UserItem, error) {
	type row struct {
		UserID uuid.UUID `db:"user_id"`
		ItemID uuid.UUID `db:"item_id"`
		Amount int       `db:"amount"`

		ItemName string `db:"item_name"`
	}

	collected, err := pgx.RowToStructByName[row](collectable)
	if err != nil {
		return nil, err
	}

	return &UserItem{
		UserID:   collected.UserID,
		ItemID:   collected.ItemID,
		Amount:   collected.Amount,
		ItemName: collected.ItemName,
	}, nil
}

func getItemByName(ctx context.Context, db pgxExecutor, name string) (*Item, error) {
	query := `
		SELECT id, name, price
		FROM items
		WHERE name = $1
	`
	args := []any{name}

	rows, _ := db.Query(ctx, query, args...)
	item, err := pgx.CollectExactlyOneRow(rows, rowToItem)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrItemNotExist
		}
		return nil, err
	}

	return item, nil
}

func addUserItemAmount(ctx context.Context, db pgxExecutor, userID, itemID uuid.UUID, amount int) (*UserItem, error) {
	query := `
		INSERT INTO users_items (user_id, item_id, amount)
		VALUES ($1, $2, $3)
		ON CONFLICT ON CONSTRAINT users_user_id_item_id_pkey
		DO UPDATE SET amount = users_items.amount + excluded.amount
		RETURNING user_id, item_id, amount
	`
	args := []any{userID, itemID, amount}

	rows, _ := db.Query(ctx, query, args...)
	userItem, err := pgx.CollectExactlyOneRow(rows, rowToUserItem)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotExist
		}
		return nil, err
	}

	return userItem, nil
}

func getTransactionsByUserID(ctx context.Context, db pgxExecutor, userID uuid.UUID) ([]*Transaction, error) {
	query := `
		SELECT t.id, t.from_user_id, from_u.username as from_username, t.to_user_id, to_u.username as to_username, t.amount
		FROM transactions t
		LEFT JOIN users from_u ON t.from_user_id = from_u.id
		LEFT JOIN users to_u ON t.to_user_id = to_u.id
		WHERE t.from_user_id = $1 OR t.to_user_id = $1
	`
	args := []any{userID}

	rows, _ := db.Query(ctx, query, args...)
	transactions, err := pgx.CollectRows(rows, rowToTransactionWithUsernames)
	if err != nil {
		return nil, err
	}

	return transactions, nil
}

func getUserItems(ctx context.Context, db pgxExecutor, userID uuid.UUID) ([]*UserItem, error) {
	query := `
		SELECT ui.user_id, ui.item_id, i.name AS item_name, ui.amount
		FROM users_items ui
		JOIN items i ON ui.item_id = i.id
		WHERE ui.user_id = $1
	`
	args := []any{userID}

	rows, _ := db.Query(ctx, query, args...)
	userItems, err := pgx.CollectRows(rows, rowToUserItemWithName)
	if err != nil {
		return nil, err
	}

	return userItems, nil
}

func getUser(ctx context.Context, db pgxExecutor, id uuid.UUID) (*User, error) {
	query := `
		SELECT id, username, password_hash, balance
		FROM users
		WHERE id = $1
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

func createTransaction(ctx context.Context, db pgxExecutor, fromUserID, toUserID *uuid.UUID, amount int) (*Transaction, error) {
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

func Authenticator(jwtVerificationKey ed25519.PublicKey) func(next http.Handler) http.Handler {
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
				userID := token.UserID

				ctx := r.Context()
				ctx = context.WithValue(ctx, ContextValueUserID, userID)
				r = r.WithContext(ctx)
			}
			next.ServeHTTP(w, r)
		})
	}
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

func readFileWithED25519PublicKey(name string) (ed25519.PublicKey, error) {
	publicKeyPemBytes, err := os.ReadFile(name)
	if err != nil {
		return ed25519.PublicKey{}, err
	}
	publicKeyPemBlock, _ := pem.Decode(publicKeyPemBytes)
	if publicKeyPemBlock == nil {
		return ed25519.PublicKey{}, err
	}
	publicKeyX509Bytes := publicKeyPemBlock.Bytes
	publicKeyAny, err := x509.ParsePKIXPublicKey(publicKeyX509Bytes)
	if err != nil {
		return ed25519.PublicKey{}, err
	}
	publicKey, ok := publicKeyAny.(ed25519.PublicKey)
	if !ok {
		return ed25519.PublicKey{}, errors.New("not an ed25519 public key file")
	}
	return publicKey, nil
}

func readFileWithED25519PrivateKey(name string) (ed25519.PrivateKey, error) {
	privateKeyPemBytes, err := os.ReadFile(name)
	if err != nil {
		return ed25519.PrivateKey{}, err
	}
	privateKeyPemBlock, _ := pem.Decode(privateKeyPemBytes)
	if privateKeyPemBlock == nil {
		return ed25519.PrivateKey{}, err
	}
	privateKeyX509Bytes := privateKeyPemBlock.Bytes
	privateKeyAny, err := x509.ParsePKCS8PrivateKey(privateKeyX509Bytes)
	if err != nil {
		return ed25519.PrivateKey{}, err
	}
	privateKey, ok := privateKeyAny.(ed25519.PrivateKey)
	if !ok {
		return ed25519.PrivateKey{}, errors.New("not an ed25519 private key file")
	}
	return privateKey, nil
}

func hashPassword(password string) string {
	const (
		saltLen     = 16
		timeCost    = 1
		memoryCost  = 64 * 1024
		parallelism = 1
		hashLen     = 32
	)

	salt := make([]byte, saltLen)
	_, err := rand.Read(salt)
	if err != nil {
		panic(err)
	}

	hash := argon2.IDKey([]byte(password), salt, timeCost, memoryCost, parallelism, hashLen)

	saltBase64 := base64.RawStdEncoding.EncodeToString(salt)
	hashBase64 := base64.RawStdEncoding.EncodeToString(hash)
	encoded := fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, memoryCost, timeCost, parallelism, saltBase64, hashBase64)

	return encoded
}
