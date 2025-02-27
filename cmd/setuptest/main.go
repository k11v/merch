package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/k11v/merch/internal/app"
	"github.com/k11v/merch/internal/user"
	"github.com/k11v/merch/internal/user/usertest"
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
		users, err = createUsers(ctx, db, *usersFlag)
		if err != nil {
			return err
		}
		slog.Info("created users", "count", *usersFlag)
	}

	if *writeUsersFlag != "" {
		err = writeUsersFile(*writeUsersFlag, users)
		if err != nil {
			return err
		}
		slog.Info("written users file", "name", *writeUsersFlag)
	}

	return nil
}

type User struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func createUsers(ctx context.Context, db app.PgxExecutor, count int) ([]*User, error) {
	password := usertest.DefaultPassword

	users := make([]*User, count)
	for i := range len(users) {
		users[i] = &User{
			Username: fmt.Sprintf("u%d", i),
			Password: password,
		}
	}

	passwordHasher := user.NewPasswordHasher(user.DefaultArgon2IDParams())
	passwordHash, err := passwordHasher.Hash(password)
	if err != nil {
		return nil, err
	}

	createUserParamsUsers := make([]*user.DataCreatorCreateUserParams, count)
	for i, u := range users {
		createUserParamsUsers[i] = &user.DataCreatorCreateUserParams{
			Username:     u.Username,
			PasswordHash: passwordHash,
			Balance:      user.DefaultBalance,
		}
	}

	dataCreator := user.NewDataCreator(db)
	err = dataCreator.CreateUsers(ctx, createUserParamsUsers)
	if err != nil {
		return nil, err
	}

	return users, nil
}

func writeUsersFile(name string, users []*User) error {
	f, err := os.Create(name)
	if err != nil {
		return err
	}
	defer func() {
		closeErr := f.Close()
		if closeErr != nil {
			slog.Error("didn't close file", "err", err)
		}
	}()

	err = json.NewEncoder(f).Encode(users)
	if err != nil {
		return err
	}

	return nil
}
