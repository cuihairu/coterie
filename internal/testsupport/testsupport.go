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
	"github.com/cuihairu/coterie/internal/notification"
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
func NewServer(t *testing.T, db *gorm.DB, opts ...app.Option) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(app.New(db, TestLogger(), opts...))
	t.Cleanup(srv.Close)
	return srv
}

// NewServerWithChannels starts an httptest server with outbound
// notification channels registered (FR-12 dispatch path).
func NewServerWithChannels(t *testing.T, db *gorm.DB, channels ...notification.Channel) *httptest.Server {
	t.Helper()
	return NewServer(t, db, app.WithNotificationChannels(channels...))
}

// TestLogger returns a discard logger for handler assembly in tests.
func TestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// DoJSON sends a request with an optional JSON body to a public route
// and returns the status code plus the decoded response body (nil for
// an empty body, e.g. 204 No Content).
func DoJSON(t *testing.T, client *http.Client, method, url, body string) (int, map[string]any) {
	return do(t, client, method, url, body, "")
}

// DoAuthJSON is DoJSON with a bearer token attached, for protected
// routes.
func DoAuthJSON(t *testing.T, client *http.Client, method, url, body, token string) (int, map[string]any) {
	return do(t, client, method, url, body, token)
}

// RegisterAndLogin registers a fresh user under tag (username and
// email derived from it) and returns a bearer token plus the user id.
func RegisterAndLogin(t *testing.T, client *http.Client, baseURL, tag string) (string, string) {
	t.Helper()
	code, body := DoJSON(t, client, http.MethodPost, baseURL+"/api/v1/auth/register",
		fmt.Sprintf(`{"username":%q,"email":%q,"password":"password-123"}`, tag, tag+"@example.com"))
	if code != http.StatusCreated {
		t.Fatalf("register %s: status = %d: %v", tag, code, body)
	}
	token, _ := body["token"].(string)
	u, _ := body["user"].(map[string]any)
	id, _ := u["id"].(string)
	if token == "" || id == "" {
		t.Fatalf("register %s: missing token or user id: %v", tag, body)
	}
	return token, id
}

// OwnerMemberID returns the owner membership id of the coterie — the
// member id usage records and seat assignments need, as opposed to the
// owner's platform user id.
func OwnerMemberID(t *testing.T, client *http.Client, baseURL, coterieID, token string) string {
	t.Helper()
	code, body := DoAuthJSON(t, client, http.MethodGet,
		baseURL+"/api/v1/coteries/"+coterieID+"/members", "", token)
	if code != http.StatusOK {
		t.Fatalf("list members: status = %d: %v", code, body)
	}
	items, _ := body["items"].([]any)
	for _, raw := range items {
		m, _ := raw.(map[string]any)
		if m["role"] == "owner" {
			id, _ := m["id"].(string)
			if id != "" {
				return id
			}
		}
	}
	t.Fatalf("no owner member found: %v", body)
	return ""
}

// Member is a joined coterie member as seen by the API tests.
type Member struct {
	Token    string // the member user's bearer token
	UserID   string // platform user id
	MemberID string // membership id inside the coterie
}

// SeedChain creates provider → product → subscription through the API,
// owned by a freshly registered user, and returns the owner's bearer
// token, owner user id, and subscription id.
func SeedChain(t *testing.T, client *http.Client, baseURL, tag, price string, maxSeats int) (token, ownerID, subscriptionID string) {
	t.Helper()
	token, ownerID = RegisterAndLogin(t, client, baseURL, tag)

	code, body := DoAuthJSON(t, client, http.MethodPost, baseURL+"/api/v1/providers",
		fmt.Sprintf(`{"slug":"ts-%s","name":"Seed Provider %s","category":"video"}`, tag, tag), token)
	if code != http.StatusCreated {
		t.Fatalf("seed provider: status = %d: %v", code, body)
	}
	providerID, _ := body["id"].(string)

	code, body = DoAuthJSON(t, client, http.MethodPost, baseURL+"/api/v1/products",
		fmt.Sprintf(`{"provider_id":%q,"name":"Seed Product %s"}`, providerID, tag), token)
	if code != http.StatusCreated {
		t.Fatalf("seed product: status = %d: %v", code, body)
	}
	productID, _ := body["id"].(string)

	code, body = DoAuthJSON(t, client, http.MethodPost, baseURL+"/api/v1/subscriptions",
		fmt.Sprintf(`{"product_id":%q,"owner_user_id":%q,"billing_cycle":"monthly","price":%q,"currency":"USD","start_date":"2026-10-01","max_seats":%d}`, productID, ownerID, price, maxSeats), token)
	if code != http.StatusCreated {
		t.Fatalf("seed subscription: status = %d: %v", code, body)
	}
	subscriptionID, _ = body["id"].(string)
	return token, ownerID, subscriptionID
}

// SeedCircle builds a full sharing circle: SeedChain plus a coterie
// with capacity seats, published open, and n extra members joined via
// invitations. joined[i] corresponds to tag+"-m<i>".
func SeedCircle(t *testing.T, client *http.Client, baseURL, tag, price string, maxSeats, capacity, members int) (ownerToken, ownerID, subscriptionID, coterieID string, joined []Member) {
	t.Helper()
	ownerToken, ownerID, subscriptionID = SeedChain(t, client, baseURL, tag, price, maxSeats)

	code, body := DoAuthJSON(t, client, http.MethodPost, baseURL+"/api/v1/coteries",
		fmt.Sprintf(`{"subscription_id":%q,"name":"Circle %s","capacity":%d}`, subscriptionID, tag, capacity), ownerToken)
	if code != http.StatusCreated {
		t.Fatalf("seed coterie: status = %d: %v", code, body)
	}
	coterieID, _ = body["id"].(string)

	code, _ = DoAuthJSON(t, client, http.MethodPatch, baseURL+"/api/v1/coteries/"+coterieID,
		`{"status":"open"}`, ownerToken)
	if code != http.StatusOK {
		t.Fatalf("publish coterie: status = %d", code)
	}

	joined = make([]Member, members)
	for i := range joined {
		mtag := fmt.Sprintf("%s-m%d", tag, i)
		token, userID := RegisterAndLogin(t, client, baseURL, mtag)
		code, inv := DoAuthJSON(t, client, http.MethodPost,
			baseURL+"/api/v1/coteries/"+coterieID+"/invitations", `{"role":"member"}`, ownerToken)
		if code != http.StatusCreated {
			t.Fatalf("seed invitation: status = %d: %v", code, inv)
		}
		inviteToken, _ := inv["token"].(string)
		code, mem := DoAuthJSON(t, client, http.MethodPost, baseURL+"/api/v1/invitations/accept",
			fmt.Sprintf(`{"token":%q}`, inviteToken), token)
		if code != http.StatusCreated {
			t.Fatalf("seed member %s: status = %d: %v", mtag, code, mem)
		}
		memberID, _ := mem["id"].(string)
		joined[i] = Member{Token: token, UserID: userID, MemberID: memberID}
	}
	return ownerToken, ownerID, subscriptionID, coterieID, joined
}

func do(t *testing.T, client *http.Client, method, url, body, token string) (int, map[string]any) {
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
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
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
