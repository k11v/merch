package main

import "github.com/k11v/merch/internal/app"

type SetupAppParams struct {
	PostgresURL            string
	JWTVerificationKeyFile string
	JWTSignatureKeyFile    string
}

func SetupApp(params *SetupAppParams) error {
	err := app.SetupPostgres(params.PostgresURL)
	if err != nil {
		return err
	}

	err = app.SetupJWT(params.JWTVerificationKeyFile, params.JWTSignatureKeyFile)
	if err != nil {
		return err
	}

	return nil
}
