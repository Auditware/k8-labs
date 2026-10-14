// Package db wraps the Postgres connection pool. Postgres is self-hosted on
// the System VM, reusing the storage class set up for Vault's Raft storage
// (see docs/architecture/tech-stack.md). No ORM - pgx directly, raw SQL.
//
// Connection config is deliberately split into config (host/port/name/
// sslmode - a k8s ConfigMap in prod) and secrets (user/password - Vault in
// prod, git-secret locally). See docs/architecture/environments.md for the full
// local-vs-prod mapping; this package just reads whatever's in the
// environment, same code path either way.
package db

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

func connStringFromEnv() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		os.Getenv("DATABASE_USER"),
		os.Getenv("DATABASE_PASSWORD"),
		getenvDefault("DATABASE_HOST", "localhost"),
		getenvDefault("DATABASE_PORT", "5432"),
		getenvDefault("DATABASE_NAME", "platform"),
		getenvDefault("DATABASE_SSLMODE", "disable"),
	)
}

func getenvDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func Connect() (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(context.Background(), connStringFromEnv())
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}
