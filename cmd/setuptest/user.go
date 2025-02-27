package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/k11v/merch/internal/app"
	"github.com/k11v/merch/internal/user"
	"github.com/k11v/merch/internal/user/usertest"
)

type User struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func CreateUsers(ctx context.Context, db app.PgxExecutor, count int) ([]*User, error) {
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
		return nil, fmt.Errorf("CreateUsers: %w", err)
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
		return nil, fmt.Errorf("CreateUsers: %w", err)
	}

	return users, nil
}

func WriteUsersFile(name string, users []*User) error {
	f, err := os.Create(name)
	if err != nil {
		return fmt.Errorf("WriteUsersFile: %w", err)
	}
	defer func() {
		closeErr := f.Close()
		if closeErr != nil {
			slog.Error("didn't close file", "err", err)
		}
	}()

	err = json.NewEncoder(f).Encode(users)
	if err != nil {
		return fmt.Errorf("WriteUsersFile: %w", err)
	}

	return nil
}
