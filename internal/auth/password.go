package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/argon2"
)

var (
	ErrInvalidUsernameOrPassword = errors.New("invalid username or password")
	ErrPasswordNotMatch          = errors.New("password does not match hash")
	ErrUserExist                 = errors.New("user already exists")
	ErrUserNotExist              = errors.New("user does not exist")
)

type User struct {
	ID           uuid.UUID
	Username     string
	PasswordHash string
	Balance      int
}

type Transaction struct {
	ID         uuid.UUID
	FromUserID *uuid.UUID
	ToUserID   *uuid.UUID
	Amount     int

	FromUsername *string
	ToUsername   *string
}

// Data represents authenticated data.
type Data struct {
	UserID uuid.UUID
}

type PasswordAuthenticator struct {
	db             *pgxpool.Pool
	passwordHasher *PasswordHasher
}

func NewPasswordAuthenticator(db *pgxpool.Pool, passwordHasher *PasswordHasher) *PasswordAuthenticator {
	return &PasswordAuthenticator{db: db, passwordHasher: passwordHasher}
}

func (pa *PasswordAuthenticator) AuthenticatePassword(ctx context.Context, username, password string) (*Data, error) {
	// HACK: Race condition.
	user, err := getUserByUsername(ctx, pa.db, username)
	if err == nil {
		err = pa.passwordHasher.Verify(password, user.PasswordHash)
		if err != nil {
			return nil, err
		}
	} else if errors.Is(err, ErrUserNotExist) {
		tx, err := pa.db.Begin(ctx)
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
		passwordHash, err := pa.passwordHasher.Hash(password)
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
	return &Data{UserID: user.ID}, nil
}

type pgxExecutor interface {
	Begin(ctx context.Context) (pgx.Tx, error)
	Exec(ctx context.Context, sql string, arguments ...any) (commandTag pgconn.CommandTag, err error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults
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

type Argon2IDParams struct {
	Memory      uint32
	Time        uint32
	Parallelism uint8
	SaltLen     uint32
	HashLen     uint32
}

func DefaultArgon2IDParams() *Argon2IDParams {
	return &Argon2IDParams{
		Memory:      1,
		Time:        64 * 1024,
		Parallelism: 1,
		SaltLen:     16,
		HashLen:     32,
	}
}

type PasswordHasher struct {
	argon2IDParams *Argon2IDParams
}

func NewPasswordHasher(argon2IDParams *Argon2IDParams) *PasswordHasher {
	return &PasswordHasher{argon2IDParams: argon2IDParams}
}

// Hash derives an Argon2ID hash from the password
// and returns it in the PHC string format.
// See https://github.com/P-H-C/phc-string-format/blob/master/phc-sf-spec.md.
func (ph *PasswordHasher) Hash(password string) (string, error) {
	salt := make([]byte, ph.argon2IDParams.SaltLen)
	_, err := rand.Read(salt)
	if err != nil {
		return "", fmt.Errorf("HashPasswordArgon2ID: %w", err)
	}
	hashRaw := argon2.IDKey(
		[]byte(password),
		salt,
		ph.argon2IDParams.Time,
		ph.argon2IDParams.Memory,
		ph.argon2IDParams.Parallelism,
		ph.argon2IDParams.HashLen,
	)
	hashEnc := formatPasswordHashArgon2ID(hashRaw, salt, ph.argon2IDParams)
	return hashEnc, nil
}

// Verify checks that the password matches the Argon2ID hash
// provided in the PHC string format.
func (ph *PasswordHasher) Verify(password, passwordHash string) error {
	wantHash, salt, params, err := parsePasswordHashArgon2ID(passwordHash)
	if err != nil {
		return fmt.Errorf("VerifyPasswordArgon2ID: %w", err)
	}
	gotHash := argon2.IDKey(
		[]byte(password),
		salt,
		params.Time,
		params.Memory,
		params.Parallelism,
		params.HashLen,
	)
	if subtle.ConstantTimeCompare(gotHash, wantHash) == 1 {
		return nil
	}
	return fmt.Errorf("VerifyPasswordArgon2ID: %w", ErrInvalidUsernameOrPassword)
}

func parsePasswordHashArgon2ID(passwordHash string) (hash []byte, salt []byte, params *Argon2IDParams, err error) {
	fieldsString, found := strings.CutPrefix(passwordHash, "$")
	if !found {
		return nil, nil, nil, errors.New("invalid prefix")
	}
	fields := strings.Split(fieldsString, "$")
	if len(fields) != 5 {
		return nil, nil, nil, errors.New("invalid fields count")
	}

	idField := fields[0]
	if idField != "argon2id" {
		return nil, nil, nil, errors.New("unsupported id")
	}

	versionField := fields[1]
	if versionField != "v="+strconv.Itoa(argon2.Version) {
		return nil, nil, nil, errors.New("unsupported version")
	}

	paramsFieldReader := strings.NewReader(fields[2])
	var (
		m uint32
		t uint32
		p uint8
	)
	_, err = fmt.Fscanf(paramsFieldReader, "m=%d,t=%d,p=%d", &m, &t, &p)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("params: %w", err)
	}
	if paramsFieldReader.Len() != 0 {
		return nil, nil, nil, errors.New("params: extra string")
	}

	saltField := fields[3]
	salt, err = base64.RawStdEncoding.DecodeString(saltField)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("salt: %w", err)
	}
	saltLen, err := intToUint32(len(salt))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("salt: %w", err)
	}

	hashField := fields[4]
	hash, err = base64.RawStdEncoding.DecodeString(hashField)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("hash: %w", err)
	}
	hashLen, err := intToUint32(len(hash))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("hash: %w", err)
	}

	params = &Argon2IDParams{
		Memory:      m,
		Time:        t,
		Parallelism: p,
		SaltLen:     saltLen,
		HashLen:     hashLen,
	}

	return hash, salt, params, nil
}

func formatPasswordHashArgon2ID(hash []byte, salt []byte, params *Argon2IDParams) string {
	saltB64 := base64.RawStdEncoding.EncodeToString(salt)
	hashB64 := base64.RawStdEncoding.EncodeToString(hash)

	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		params.Memory,
		params.Time,
		params.Parallelism,
		saltB64,
		hashB64,
	)
}

func intToUint32(i int) (uint32, error) {
	if i < 0 || i > math.MaxUint32 {
		return 0, errors.New("int out of uint32 bounds")
	}
	return uint32(i), nil
}
