package apptest

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/k11v/merch/internal/app"
)

func NewPostgresPool(t testing.TB, ctx context.Context) *pgxpool.Pool {
	t.Helper()

	url := newPostgresURL(t, ctx)

	pool, err := app.NewPostgresPool(ctx, url)
	if err != nil {
		t.Fatalf("can't create Postgres pool: %v", err)
	}
	t.Cleanup(pool.Close)

	return pool
}

func newPostgresURL(t testing.TB, ctx context.Context) string {
	t.Helper()

	var (
		db       = "postgres"
		password = "postgres"
		user     = "postgres"
	)

	req := testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: "postgres:13",
			Env: map[string]string{
				"POSTGRES_DB":       db,
				"POSTGRES_PASSWORD": password,
				"POSTGRES_USER":     user,
			},
			ExposedPorts: []string{"5432/tcp"},
			WaitingFor: wait.ForAll(
				wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
				wait.ForListeningPort("5432/tcp"),
			).WithDeadline(60 * time.Second),
		},
		Started: true,
	}
	postgresContainer, err := testcontainers.GenericContainer(ctx, req)
	t.Cleanup(func() {
		terminateErr := testcontainers.TerminateContainer(postgresContainer)
		if terminateErr != nil {
			t.Errorf("didn't terminate Postgres container")
		}
	})
	if err != nil {
		t.Fatalf("can't create Postgres container: %v", err)
	}

	host, err := postgresContainer.Host(ctx)
	if err != nil {
		t.Fatalf("can't get Postgres container host: %v", err)
	}
	port, err := postgresContainer.MappedPort(ctx, "5432/tcp")
	if err != nil {
		t.Fatalf("can't get Postgres container port: %v", err)
	}
	url := fmt.Sprintf(
		"postgres://%s:%s@%s/%s?sslmode=disable",
		user,
		password,
		net.JoinHostPort(host, port.Port()),
		db,
	)

	err = app.SetupPostgres(url)
	if err != nil {
		t.Fatalf("can't setup Postgres: %v", err)
	}

	return url
}

// BeginPostgresTx begins a new Postgres transaction and returns it.
// It shouldn't be committed or rolled back manually.
// It is automatically rolled back on test cleanup.
func BeginPostgresTx(t testing.TB, ctx context.Context, db *pgxpool.Pool) pgx.Tx {
	t.Helper()

	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatalf("can't begin Postgres transaction: %v", err)
	}
	t.Cleanup(func() {
		rollbackErr := tx.Rollback(ctx)
		if rollbackErr != nil {
			t.Errorf("didn't rollback Postgres transaction: %v", err)
		}
	})

	return tx
}
