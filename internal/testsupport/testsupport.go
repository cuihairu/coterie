// Package testsupport provides a shared throwaway PostgreSQL for
// integration tests. The container lives for the whole test binary and
// is reaped by ryuk when the process exits; tests skip when no Docker
// daemon is reachable.
package testsupport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	mobyclient "github.com/moby/moby/client"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/gorm"

	"github.com/cuihairu/coterie/internal/app"
	"github.com/cuihairu/coterie/internal/database"
)

var (
	once     sync.Once
	sharedDB *gorm.DB
	setupErr error
)

// NewDB returns a *gorm.DB connected to a shared PostgreSQL 18 container
// with all migrations applied. The first call starts the container
// (slow); later calls reuse it. Skips the test when Docker is not
// available.
func NewDB(t *testing.T) *gorm.DB {
	t.Helper()
	once.Do(setup)
	if setupErr != nil {
		t.Skipf("integration database unavailable: %v", setupErr)
	}
	return sharedDB
}

// NewServer starts an httptest server around the fully assembled app
// handler backed by db.
func NewServer(t *testing.T, db *gorm.DB) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(app.New(db, TestLogger()))
	t.Cleanup(srv.Close)
	return srv
}

// TestLogger returns a discard logger for handler assembly in tests.
func TestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// DoJSON sends a request with an optional JSON body and returns the
// status code plus the decoded response body (nil for an empty body,
// e.g. 204 No Content).
func DoJSON(t *testing.T, client *http.Client, method, url, body string) (int, map[string]any) {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatalf("non-JSON response (%d): %s", resp.StatusCode, raw)
		}
	}
	return resp.StatusCode, out
}

func setup() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	if !dockerAvailable(ctx) {
		setupErr = errors.New("docker daemon not reachable")
		return
	}

	container, err := postgres.Run(ctx, "postgres:18-alpine",
		postgres.WithDatabase("coterie"),
		postgres.WithUsername("coterie"),
		postgres.WithPassword("coterie"),
	)
	if err != nil {
		setupErr = fmt.Errorf("start postgres container: %w", err)
		return
	}

	url, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		setupErr = fmt.Errorf("container connection string: %w", err)
		return
	}

	migrationsDir, err := findMigrationsDir()
	if err != nil {
		setupErr = err
		return
	}
	if err := database.Migrate(url, os.DirFS(migrationsDir)); err != nil {
		setupErr = fmt.Errorf("apply migrations: %w", err)
		return
	}

	sharedDB, err = database.Open(url)
	if err != nil {
		setupErr = fmt.Errorf("open gorm: %w", err)
	}
}

func dockerAvailable(ctx context.Context) bool {
	cli, err := mobyclient.NewClientWithOpts(mobyclient.FromEnv, mobyclient.WithAPIVersionNegotiation())
	if err != nil {
		return false
	}
	defer cli.Close()
	ping, err := cli.Ping(ctx, mobyclient.PingOptions{})
	return err == nil && ping.APIVersion != ""
}

// findMigrationsDir walks up from the working directory until it finds
// the repo's migrations folder, so tests work from any package.
func findMigrationsDir() (string, error) {
	marker := filepath.Join("migrations", "0001_init.up.sql")
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for range 8 {
		if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
			return filepath.Join(dir, "migrations"), nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", errors.New("migrations directory not found")
}
