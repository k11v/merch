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
	usersFlag      = flag.Int("users", 0, "number of users to create")
	writeUsersFlag = flag.String("writeusers", "", "write created users to JSON file")
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

	err := run(postgresURL)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	os.Exit(0)
}

func run(postgresURL string) error {
	ctx := context.Background()

	db, err := app.NewPostgresPool(ctx, postgresURL)
	if err != nil {
		return err
	}
	defer db.Close()

	var users []*User
	if *usersFlag > 0 {
		users, err = CreateUsers(ctx, db, *usersFlag)
		if err != nil {
			return err
		}
		slog.Info("created users", "count", *usersFlag)
	}

	if *writeUsersFlag != "" {
		err = WriteUsersFile(*writeUsersFlag, users)
		if err != nil {
			return err
		}
		slog.Info("written users file", "name", *writeUsersFlag)
	}

	return nil
}
