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
	handler := NewHandler()

	strictMiddlewares := []merch.StrictMiddlewareFunc{
		StrictAuthenticator(),
	}
	middlewares := []merch.MiddlewareFunc{
		Authenticator(), // Should be redundant.
	}

	responseErrorHandler := func(w http.ResponseWriter, r *http.Request, err error) {
		slog.Error("server error", "err", err)

		errors := new(string)
		*errors = "internal server error"
		response := merch.ErrorResponse{Errors: errors}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(response)
	}
	requestErrorHandler := func(w http.ResponseWriter, r *http.Request, err error) {
		errors := new(string)
		*errors = err.Error()
		response := merch.ErrorResponse{Errors: errors}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		encodeErr := json.NewEncoder(w).Encode(response)
		if encodeErr != nil {
			responseErrorHandler(w, r, encodeErr)
		}
	}

	mux := http.NewServeMux()
	ssi := merch.StrictServerInterface(handler)
	si := merch.NewStrictHandlerWithOptions(ssi, strictMiddlewares, merch.StrictHTTPServerOptions{
		RequestErrorHandlerFunc:  requestErrorHandler,
		ResponseErrorHandlerFunc: responseErrorHandler,
	})
	h := merch.HandlerWithOptions(si, merch.StdHTTPServerOptions{
		BaseRouter:       mux,
		Middlewares:      middlewares,
		ErrorHandlerFunc: requestErrorHandler,
	})

	addr := net.JoinHostPort(host, strconv.Itoa(port))
	logLogger := slog.NewLogLogger(slog.Default().Handler(), slog.LevelError)

	return &http.Server{
		Addr:     addr,
		Handler:  h,
		ErrorLog: logLogger,
	}
}

var _ merch.StrictServerInterface = (*Handler)(nil)

type Handler struct{}

func NewHandler() *Handler {
	return &Handler{}
}

// GetAPIBuyItem implements merch.StrictServerInterface.
func (s *Handler) GetAPIBuyItem(ctx context.Context, request merch.GetAPIBuyItemRequestObject) (merch.GetAPIBuyItemResponseObject, error) {
	panic("unimplemented")
}

// GetAPIInfo implements merch.StrictServerInterface.
func (s *Handler) GetAPIInfo(ctx context.Context, request merch.GetAPIInfoRequestObject) (merch.GetAPIInfoResponseObject, error) {
	panic("unimplemented")
}

// PostAPIAuth implements merch.StrictServerInterface.
func (s *Handler) PostAPIAuth(ctx context.Context, request merch.PostAPIAuthRequestObject) (merch.PostAPIAuthResponseObject, error) {
	panic("unimplemented")
}

// PostAPISendCoin implements merch.StrictServerInterface.
func (s *Handler) PostAPISendCoin(ctx context.Context, request merch.PostAPISendCoinRequestObject) (merch.PostAPISendCoinResponseObject, error) {
	panic("unimplemented")
}

func StrictAuthenticator() merch.StrictMiddlewareFunc {
	return func(f merch.StrictHandlerFunc, operationID string) merch.StrictHandlerFunc {
		return func(ctx context.Context, w http.ResponseWriter, r *http.Request, request interface{}) (response interface{}, err error) {
			switch operationID {
			case "PostAPIAuth":
			default:
				// Do authentication.
			}
			return f(ctx, w, r, request)
		}
	}
}

func Authenticator() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.Method == "POST" && r.URL.Path == "/api/auth":
			default:
				// Do authentication.
			}
			next.ServeHTTP(w, r)
		})
	}
}
