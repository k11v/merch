package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
)

var (
	appFlag     = flag.Bool("app", false, "set up app, including postgres and jwt")
	apptestFlag = flag.Bool("apptest", false, "set up apptest, including user and auth")
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

	const envTestUserFile = "APPTEST_USER_FILE"
	testUserFile := os.Getenv(envTestUserFile)
	if testUserFile == "" {
		err := fmt.Errorf("%s env is empty", envTestUserFile)
		_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	const envTestUserCount = "APPTEST_USER_COUNT"
	testUserCountEnv := os.Getenv(envTestUserCount)
	if testUserCountEnv == "" {
		err := fmt.Errorf("%s env is empty", envTestUserCount)
		_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	testUserCount, err := strconv.Atoi(testUserCountEnv)
	if err != nil {
		err = fmt.Errorf("%s env: %w", envTestUserCount, err)
		_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	const envTestAuthTokenFile = "APPTEST_AUTH_TOKEN_FILE" // #nosec G101
	testAuthTokenFile := os.Getenv(envTestAuthTokenFile)
	if testAuthTokenFile == "" {
		err = fmt.Errorf("%s env is empty", envTestAuthTokenFile)
		_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	doneSetups := make([]string, 0)
	if *appFlag {
		err = SetupApp(&SetupAppParams{
			PostgresURL:            postgresURL,
			JWTVerificationKeyFile: jwtVerificationKeyFile,
			JWTSignatureKeyFile:    jwtSignatureKeyFile,
		})
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		doneSetups = append(doneSetups, "app")
	}
	if *apptestFlag {
		err = SetupApptest(&SetupApptestParams{
			PostgresURL:         postgresURL,
			JWTSignatureKeyFile: jwtSignatureKeyFile,
			UserFile:            testUserFile,
			UserCount:           testUserCount,
			AuthTokenFile:       testAuthTokenFile,
		})
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		doneSetups = append(doneSetups, "apptest")
	}
	if len(doneSetups) == 0 {
		_, _ = fmt.Fprint(os.Stderr, "error: no setups were specified\n")
		os.Exit(1)
	}

	os.Exit(0)
}
