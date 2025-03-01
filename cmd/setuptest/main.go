package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"

	"github.com/k11v/merch/internal/app"
)

func main() {
	const envPostgresURL = "APP_POSTGRES_URL"
	postgresURL := os.Getenv(envPostgresURL)
	if postgresURL == "" {
		err := fmt.Errorf("%s env is empty", envPostgresURL)
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

	const envUserFile = "APPTEST_USER_FILE"
	userFile := os.Getenv(envUserFile)

	const envUserGenerate = "APPTEST_USER_GENERATE"
	userGenerateEnv := os.Getenv(envUserGenerate)
	userGenerate := false
	if userGenerateEnv != "" {
		var err error
		userGenerate, err = strconv.ParseBool(userGenerateEnv)
		if err != nil {
			err = fmt.Errorf("%s env: %w", envUserGenerate, err)
			_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	}

	const envUserGenerateCount = "APPTEST_USER_GENERATE_COUNT"
	userGenerateCountEnv := os.Getenv(envUserGenerateCount)
	userGenerateCount := 0
	if userGenerateCountEnv != "" {
		var err error
		userGenerateCount, err = strconv.Atoi(userGenerateCountEnv)
		if err != nil {
			err = fmt.Errorf("%s env: %w", envUserGenerateCount, err)
			_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	}

	const envAuthFile = "APPTEST_AUTH_FILE"
	authFile := os.Getenv(envAuthFile)

	const envAuthGenerate = "APPTEST_AUTH_GENERATE"
	authGenerateEnv := os.Getenv(envAuthGenerate)
	authGenerate := false
	if authGenerateEnv != "" {
		var err error
		authGenerate, err = strconv.ParseBool(authGenerateEnv)
		if err != nil {
			err = fmt.Errorf("%s env: %w", envAuthGenerate, err)
			_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	}

	err := run(&runParams{
		PostgresURL:         postgresURL,
		JWTSignatureKeyFile: jwtSignatureKeyFile,
		UserFile:            userFile,
		UserGenerate:        userGenerate,
		UserGenerateCount:   userGenerateCount,
		AuthFile:            authFile,
		AuthGenerate:        authGenerate,
	})
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	os.Exit(0)
}

type runParams struct {
	PostgresURL         string
	JWTSignatureKeyFile string
	UserFile            string
	UserGenerate        bool
	UserGenerateCount   int
	AuthFile            string
	AuthGenerate        bool
}

func run(params *runParams) error {
	ctx := context.Background()

	db, err := app.NewPostgresPool(ctx, params.PostgresURL)
	if err != nil {
		return err
	}
	defer db.Close()

	jwtSignatureKey, err := app.ReadFileED25519PrivateKey(params.JWTSignatureKeyFile)
	if err != nil {
		return err
	}

	var users map[string]*User
	if params.UserGenerate {
		users, err = GenerateUsers(ctx, db, params.UserGenerateCount)
		if err != nil {
			return err
		}
		slog.Info("generated users", "count", len(users))
	}

	if params.UserFile != "" {
		err = os.MkdirAll(filepath.Dir(params.UserFile), 0o700)
		if err != nil {
			return err
		}
		err = WriteFileJSON(params.UserFile, users)
		if err != nil {
			return err
		}
		slog.Info("written user file", "name", params.UserFile)
	}

	var authTokens map[string]string
	if params.AuthGenerate {
		authTokens, err = GenerateAuthTokens(ctx, db, jwtSignatureKey, users)
		if err != nil {
			return err
		}
		slog.Info("generated auth tokens", "count", len(authTokens))
	}

	if params.AuthFile != "" {
		err = os.MkdirAll(filepath.Dir(params.AuthFile), 0o700)
		if err != nil {
			return err
		}
		err = WriteFileJSON(params.AuthFile, authTokens)
		if err != nil {
			return err
		}
		slog.Info("written auth file", "name", params.AuthFile)
	}

	return nil
}
