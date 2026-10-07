// Package database provides the GORM handle and schema migrations.
//
// The schema source of truth is the hand-written SQL in /migrations;
// GORM AutoMigrate must stay disabled (design decision D7) so the
// database-level invariants documented in docs/design.md §1.4 keep
// being enforced.
package database

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// connectWait bounds how long Migrate retries connection failures. A
// freshly started PostgreSQL (docker compose, testcontainers) publishes
// its port before it can serve, so early attempts fail with connection
// resets until the server is really up.
const connectWait = 30 * time.Second

// Open connects to PostgreSQL via GORM. PreferSimpleProtocol keeps value
// transport text-based, so numeric columns (price, amount) scan cleanly
// into Go strings; the session timezone is pinned to UTC so timestamps
// render deterministically (pgx otherwise uses the client's local zone).
func Open(url string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  withRuntimeParam(url, "timezone", "UTC"),
		PreferSimpleProtocol: true,
	}), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Warn),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql.DB: %w", err)
	}
	sqlDB.SetMaxOpenConns(20)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(time.Hour)

	return db, nil
}

// withRuntimeParam appends a PostgreSQL runtime parameter to a DSN,
// handling both URL ("postgres://…") and keyword/value forms.
func withRuntimeParam(dsn, key, value string) string {
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		sep := "?"
		if strings.Contains(dsn, "?") {
			sep = "&"
		}
		return dsn + sep + key + "=" + value
	}
	return dsn + " " + key + "=" + value
}

// Close releases the underlying connection pool.
func Close(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// Migrate applies pending migrations from dir via golang-migrate.
// It is idempotent: applied versions are recorded in schema_migrations.
// Connection failures (server still starting) are retried for up to
// connectWait; anything else fails immediately.
func Migrate(databaseURL string, dir fs.FS) error {
	deadline := time.Now().Add(connectWait)
	for {
		err := migrateUp(databaseURL, dir)
		if err == nil {
			return nil
		}
		var connErr *pgconn.ConnectError
		if !errors.As(err, &connErr) || time.Now().After(deadline) {
			return err
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// migrateUp runs one migration pass; the migrator is not reusable, so
// each attempt opens a fresh driver and source.
func migrateUp(databaseURL string, dir fs.FS) error {
	src, err := iofs.New(dir, ".")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}

	sqlDB, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("open database for migrations: %w", err)
	}
	defer sqlDB.Close()

	driver, err := pgx.WithInstance(sqlDB, &pgx.Config{})
	if err != nil {
		return fmt.Errorf("init migration driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", src, "coterie", driver)
	if err != nil {
		return fmt.Errorf("init migrator: %w", err)
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}
