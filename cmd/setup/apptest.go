package main

import (
	"context"
	"os"
	"path/filepath"

	"github.com/k11v/merch/internal/app"
)

type SetupApptestParams struct {
	PostgresURL         string
	JWTSignatureKeyFile string
	UserFile            string
	UserGenerateCount   int
	AuthFile            string
}

func SetupApptest(params *SetupApptestParams) error {
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

	users, err := GenerateUsers(ctx, db, params.UserGenerateCount)
	if err != nil {
		return err
	}

	err = os.MkdirAll(filepath.Dir(params.UserFile), 0o700)
	if err != nil {
		return err
	}
	err = WriteFileJSON(params.UserFile, users)
	if err != nil {
		return err
	}

	authTokens, err := GenerateAuthTokens(ctx, db, jwtSignatureKey, users)
	if err != nil {
		return err
	}

	err = os.MkdirAll(filepath.Dir(params.AuthFile), 0o700)
	if err != nil {
		return err
	}
	err = WriteFileJSON(params.AuthFile, authTokens)
	if err != nil {
		return err
	}

	return nil
}
