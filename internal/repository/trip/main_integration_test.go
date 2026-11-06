//go:build integration

package trip_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestMain(m *testing.M) {
	if err := checkDatabase(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "PostgreSQL preflight failed:", err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func integrationConfig() (*pgxpool.Config, error) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = os.Getenv("DATABASE_URL")
	}
	if url == "" {
		return nil, errors.New("set TEST_DATABASE_URL or run make test-integration with the tripgoctl .env")
	}
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, errors.New("invalid integration database configuration; check TEST_DATABASE_URL or DATABASE_URL")
	}
	cfg.MaxConns = 4
	cfg.ConnConfig.ConnectTimeout = 5 * time.Second
	return cfg, nil
}

func checkDatabase(ctx context.Context) error {
	cfg, err := integrationConfig()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	conn, err := pgx.ConnectConfig(ctx, cfg.ConnConfig)
	if err != nil {
		return databaseUnavailable()
	}
	pingErr := conn.Ping(ctx)
	cleanup, cleanupCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cleanupCancel()
	closeErr := conn.Close(cleanup)
	if pingErr != nil {
		return databaseUnavailable()
	}
	if closeErr != nil {
		return errors.New("could not close the PostgreSQL preflight connection")
	}
	return nil
}

func databaseUnavailable() error {
	return errors.New("database is unavailable; run 'tripgoctl environment start' and 'tripgoctl connect', then check TEST_DATABASE_URL or DATABASE_URL")
}
