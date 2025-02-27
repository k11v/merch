package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/k11v/merch/internal/app"
	"github.com/k11v/merch/internal/user"
	"github.com/k11v/merch/internal/user/usertest"
)

var usersFlag = flag.Int("users", 0, "number of users to create")

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

	if *usersFlag > 0 {
		err = createUsers(ctx, db, *usersFlag)
		if err != nil {
			return err
		}
		slog.Info("created users", "count", *usersFlag)
	} else {
		slog.Info("skipped user creation")
	}

	return nil
}

func createUsers(ctx context.Context, db app.PgxExecutor, count int) error {
	passwordHasher := user.NewPasswordHasher(user.DefaultArgon2IDParams())
	passwordHash, err := passwordHasher.Hash(usertest.DefaultPassword)
	if err != nil {
		return err
	}

	users := make([]*user.DataCreatorCreateUserParams, 0)
	for i := range count {
		users = append(users, &user.DataCreatorCreateUserParams{
			Username:     fmt.Sprintf("u%d", i),
			PasswordHash: passwordHash,
			Balance:      user.DefaultBalance,
		})
	}

	dataCreator := user.NewDataCreator(db)
	err = dataCreator.CreateUsers(ctx, users)
	if err != nil {
		return err
	}

	return nil
}
