package app

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPostgresPool(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pgxConfig, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("app.NewPostgresPool: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, pgxConfig)
	if err != nil {
		return nil, fmt.Errorf("app.NewPostgresPool: %w", err)
	}
	return pool, nil
}
