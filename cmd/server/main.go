package main

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k11v/merch/api/merch"
	"github.com/k11v/merch/internal/app"
)

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

	jwtVerificationKey, err := app.ReadFileED25519PublicKey(jwtVerificationKeyFile)
	if err != nil {
		return err
	}

	jwtSignatureKey, err := app.ReadFileED25519PrivateKey(jwtSignatureKeyFile)
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
		Authentication(jwtVerificationKey),
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

var _ merch.StrictServerInterface = (*Handler)(nil)

type Handler struct {
	db              *pgxpool.Pool
	jwtSignatureKey ed25519.PrivateKey
}

func NewHandler(db *pgxpool.Pool, jwtSignatureKey ed25519.PrivateKey) *Handler {
	return &Handler{db: db, jwtSignatureKey: jwtSignatureKey}
}
