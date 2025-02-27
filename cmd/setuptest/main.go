package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/k11v/merch/internal/app"
)

var (
	genUsersFlag   = flag.Int("genusers", 0, "number of users to generate")
	writeUsersFlag = flag.String("writeusers", "", "write users to JSON file")
	genAuthsFlag   = flag.Int("genauths", 0, "number of users to authenticate")
	writeAuthsFlag = flag.String("writeauths", "", "write auth tokens to JSON file")
)

func main() {
	flag.Parse()

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

	err := run(postgresURL, jwtSignatureKeyFile)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	os.Exit(0)
}

func run(postgresURL string, jwtSignatureKeyFile string) error {
	ctx := context.Background()

	db, err := app.NewPostgresPool(ctx, postgresURL)
	if err != nil {
		return err
	}
	defer db.Close()

	jwtSignatureKey, err := app.ReadFileED25519PrivateKey(jwtSignatureKeyFile)
	if err != nil {
		return err
	}

	var users []*User
	userCount := *genUsersFlag
	if userCount > 0 {
		users, err = GenerateUsers(ctx, db, userCount)
		if err != nil {
			return err
		}
		slog.Info("generated users", "count", userCount)
	}

	userFile := *writeUsersFlag
	if userFile != "" {
		err = WriteFileJSON(userFile, users)
		if err != nil {
			return err
		}
		slog.Info("written user file", "name", userFile)
	}

	var authTokens []*AuthToken
	authTokenCount := *genAuthsFlag
	if authTokenCount > 0 {
		authTokens, err = GenerateAuthTokens(ctx, db, jwtSignatureKey, users, authTokenCount)
		if err != nil {
			return err
		}
		slog.Info("generated auth tokens", "count", authTokenCount)
	}

	authTokenFile := *writeAuthsFlag
	if authTokenFile != "" {
		err = WriteFileJSON(authTokenFile, authTokens)
		if err != nil {
			return err
		}
		slog.Info("written auth token file", "name", authTokenFile)
	}

	return nil
}
