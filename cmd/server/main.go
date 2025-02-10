package main

import (
	"fmt"
	"os"
	"strconv"
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
	return nil
}
