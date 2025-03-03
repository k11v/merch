package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/google/uuid"

	"github.com/k11v/merch/internal/app"
	"github.com/k11v/merch/internal/user"
	"github.com/k11v/merch/internal/user/usertest"
)

type User struct {
	ID       uuid.UUID `json:"id"`
	Username string    `json:"username"`
	Password string    `json:"password"`
	Balance  int       `json:"balance"`
}

func GenerateUsers(ctx context.Context, db app.PgxExecutor, count int) ([]*User, error) {
	password := usertest.DefaultPassword
	passwordHasher := user.NewPasswordHasher(user.DefaultArgon2IDParams())
	passwordHash, err := passwordHasher.Hash(password)
	if err != nil {
		return nil, fmt.Errorf("GenerateUsers: %w", err)
	}

	createUserParamsUsers := make([]*user.DataCreatorCreateUserParams, count)
	for i := range len(createUserParamsUsers) {
		createUserParamsUsers[i] = &user.DataCreatorCreateUserParams{
			Username:     fmt.Sprintf("u%d", i),
			PasswordHash: passwordHash,
			Balance:      user.DefaultBalance,
		}
	}

	dataCreator := user.NewDataCreator(db)
	userUsers, err := dataCreator.CreateUsers(ctx, createUserParamsUsers)
	if err != nil {
		return nil, fmt.Errorf("GenerateUsers: %w", err)
	}

	users := make([]*User, len(userUsers))
	for i, u := range userUsers {
		users[i] = &User{
			ID:       u.ID,
			Username: u.Username,
			Password: password,
			Balance:  u.Balance,
		}
	}

	return users, nil
}

func WriteFileJSON(name string, v any) error {
	f, err := os.Create(name)
	if err != nil {
		return fmt.Errorf("WriteFileJSON: %w", err)
	}
	defer func() {
		closeErr := f.Close()
		if closeErr != nil {
			slog.Error("didn't close file", "err", err)
		}
	}()

	err = json.NewEncoder(f).Encode(v)
	if err != nil {
		return fmt.Errorf("WriteFileJSON: %w", err)
	}

	return nil
}
