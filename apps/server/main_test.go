package main

import (
	"bytes"
	"io"
	"io/fs"

	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/cuihairu/coterie/internal/config"
	"github.com/cuihairu/coterie/internal/notification"
	"github.com/cuihairu/coterie/internal/payment"
	"github.com/cuihairu/coterie/internal/provider"
	"github.com/cuihairu/coterie/providers/claude"
)

// embeddedIndex reads the client entry the binary ships, so the SPA
// assertions compare against the real asset instead of a literal.
func embeddedIndex(t *testing.T) []byte {
	t.Helper()
	data, err := readIndex()
	if err != nil {
		t.Fatalf("read embedded index: %v", err)
	}
	return data
}

func readIndex() ([]byte, error) {
	f, err := webRoot.Open("index.html")
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return io.ReadAll(f)
}

func serveSPA(t *testing.T, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	spaHandler(rec, httptest.NewRequest("GET", path, nil))
	return rec
}

func TestSPAHandlerServesIndexAtRoot(t *testing.T) {
	rec := serveSPA(t, "/")
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !bytes.Equal(rec.Body.Bytes(), embeddedIndex(t)) {
		t.Fatal("root did not serve the embedded index")
	}
}

// withWebRoot swaps the embedded client tree for the duration of a
// test, so the real-asset branch can be exercised with a fixture that
// has more than the placeholder entry file.
func withWebRoot(t *testing.T, fsys fs.FS) {
	t.Helper()
	orig := webRoot
	webRoot = fsys
	t.Cleanup(func() { webRoot = orig })
}

func TestSPAHandlerPrefersRealAsset(t *testing.T) {
	withWebRoot(t, fstest.MapFS{
		"index.html":    &fstest.MapFile{Data: []byte("<html>shell</html>")},
		"assets/app.js": &fstest.MapFile{Data: []byte("console.log(1)")},
	})

	rec := serveSPA(t, "/assets/app.js")
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Body.String() != "console.log(1)" {
		t.Fatalf("asset body = %q, want the embedded file", rec.Body.String())
	}
}

func TestSPAHandlerCanonicalizesIndexRequest(t *testing.T) {
	// net/http answers "/index.html" with the canonical "./" redirect;
	// the client follows it to the shell.
	rec := serveSPA(t, "/index.html")
	if rec.Code != 301 {
		t.Fatalf("status = %d, want 301", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "./" {
		t.Fatalf("Location = %q, want %q", loc, "./")
	}
}

func TestSPAHandlerReportsUnbuiltUI(t *testing.T) {
	// The placeholder tree is what `go run` serves before the client is
	// built into the binary; a tree without index.html must say so
	// instead of serving an empty 200.
	withWebRoot(t, fstest.MapFS{"placeholder.txt": &fstest.MapFile{Data: []byte("x")}})

	rec := serveSPA(t, "/")
	if rec.Code != 503 {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "web ui not built") {
		t.Fatalf("body = %q, want the not-built notice", rec.Body.String())
	}
}

func TestSPAHandlerFallsBackForClientRoutes(t *testing.T) {
	want := embeddedIndex(t)
	for _, path := range []string{"/coteries", "/notifications", "/assets/missing.js"} {
		t.Run(path, func(t *testing.T) {
			rec := serveSPA(t, path)
			if rec.Code != 200 {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			if !bytes.Equal(rec.Body.Bytes(), want) {
				t.Fatalf("%s did not fall back to the SPA shell", path)
			}
		})
	}
}

func TestSPAHandlerStaysInsideEmbeddedTree(t *testing.T) {
	want := embeddedIndex(t)
	for _, path := range []string{
		"/../../etc/passwd",
		"/assets/../../../etc/passwd",
		"/..%2f..%2fetc/passwd",
		"/./secret",
	} {
		t.Run(path, func(t *testing.T) {
			rec := serveSPA(t, path)
			if rec.Code == 200 && !bytes.Equal(rec.Body.Bytes(), want) {
				t.Fatalf("%s escaped the embedded tree", path)
			}
			if strings.Contains(rec.Body.String(), "root:") {
				t.Fatalf("%s leaked a host file", path)
			}
		})
	}
}

func TestAPIOnlySplitsAPIFromClient(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"/healthz", true},
		{"/api/v1/auth/login", true},
		{"/api/v1/notifications", true},
		{"/healthz/", false},
		{"/api", false},
		{"/", false},
		{"/notifications", false},
		{"/assets/index.js", false},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			if got := apiOnly(tc.path); got != tc.want {
				t.Fatalf("apiOnly(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

// captureLog returns a logger writing into buf so wiring warnings can be
// asserted on.
func captureLog() (*slog.Logger, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	return slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelWarn})), buf
}

func channelNames(chans []notification.Channel) []string {
	out := make([]string, 0, len(chans))
	for _, c := range chans {
		out = append(out, c.Name())
	}
	return out
}

func adapterNames(adapters []payment.Adapter) []string {
	out := make([]string, 0, len(adapters))
	for _, a := range adapters {
		out = append(out, a.Name())
	}
	return out
}

func pluginSlugs(plugins []provider.Plugin) []string {
	out := make([]string, 0, len(plugins))
	for _, p := range plugins {
		out = append(out, p.Slug())
	}
	return out
}

func TestChannelsFollowConfig(t *testing.T) {
	t.Run("empty config enables nothing", func(t *testing.T) {
		if got := channels(nil, config.Config{}); len(got) != 0 {
			t.Fatalf("channels = %v, want none", channelNames(got))
		}
	})

	t.Run("email needs host and from", func(t *testing.T) {
		got := channels(nil, config.Config{SMTPHost: "smtp.example.com"})
		if len(got) != 0 {
			t.Fatalf("host without from enabled %v, want none", channelNames(got))
		}
		got = channels(nil, config.Config{SMTPHost: "smtp.example.com", SMTPFrom: "billing@example.com"})
		if len(got) != 1 || got[0].Name() != "email" {
			t.Fatalf("channels = %v, want [email]", channelNames(got))
		}
	})

	t.Run("webhook needs only the url", func(t *testing.T) {
		got := channels(nil, config.Config{WebhookURL: "https://hooks.example.com/x"})
		if len(got) != 1 || got[0].Name() != "webhook" {
			t.Fatalf("channels = %v, want [webhook]", channelNames(got))
		}
	})

	t.Run("partial vapid keys stay off", func(t *testing.T) {
		got := channels(nil, config.Config{
			PushVapidPublicKey:  "public",
			PushVapidPrivateKey: "",
			PushVapidSubject:    "mailto:ops@example.com",
		})
		if len(got) != 0 {
			t.Fatalf("channels = %v, want none", channelNames(got))
		}
	})
}

func TestPaymentAdaptersFollowConfig(t *testing.T) {
	t.Run("sandbox registers by name", func(t *testing.T) {
		log, _ := captureLog()
		got := paymentAdapters(config.Config{PaymentMethods: []string{"sandbox"}}, log)
		if len(got) != 1 || got[0].Name() != "sandbox" {
			t.Fatalf("adapters = %v, want [sandbox]", adapterNames(got))
		}
	})

	t.Run("stripe without a key is skipped", func(t *testing.T) {
		log, buf := captureLog()
		got := paymentAdapters(config.Config{PaymentMethods: []string{"stripe"}}, log)
		if len(got) != 0 {
			t.Fatalf("adapters = %v, want none", adapterNames(got))
		}
		if !strings.Contains(buf.String(), "STRIPE_SECRET_KEY") {
			t.Fatalf("warning missing, got %q", buf.String())
		}
	})

	t.Run("stripe carries its configuration", func(t *testing.T) {
		log, _ := captureLog()
		got := paymentAdapters(config.Config{
			PaymentMethods:      []string{"stripe"},
			StripeSecretKey:     "sk_test_1",
			StripeWebhookSecret: "whsec_1",
			StripeAPIBase:       "https://api.example.com",
		}, log)
		if len(got) != 1 {
			t.Fatalf("adapters = %v, want one", adapterNames(got))
		}
		st, ok := got[0].(payment.Stripe)
		if !ok {
			t.Fatalf("adapter type = %T, want payment.Stripe", got[0])
		}
		if st.SecretKey != "sk_test_1" || st.WebhookSecret != "whsec_1" || st.APIBase != "https://api.example.com" {
			t.Fatalf("adapter config not carried over: %+v", st)
		}
	})

	t.Run("unknown method is skipped with a warning", func(t *testing.T) {
		log, buf := captureLog()
		got := paymentAdapters(config.Config{PaymentMethods: []string{"bogus"}}, log)
		if len(got) != 0 {
			t.Fatalf("adapters = %v, want none", adapterNames(got))
		}
		if !strings.Contains(buf.String(), "bogus") {
			t.Fatalf("warning missing, got %q", buf.String())
		}
	})
}

func TestMarketplaceAdaptersGateOnStripe(t *testing.T) {
	t.Run("sync channels never gate admission", func(t *testing.T) {
		log, _ := captureLog()
		got := marketplaceAdapters(config.Config{PaymentMethods: []string{"sandbox"}}, log)
		if len(got) != 0 {
			t.Fatalf("adapters = %v, want none", adapterNames(got))
		}
	})

	t.Run("own webhook secret wins", func(t *testing.T) {
		log, _ := captureLog()
		got := marketplaceAdapters(config.Config{
			PaymentMethods:                 []string{"stripe"},
			StripeSecretKey:                "sk_test_1",
			StripeWebhookSecret:            "whsec_payments",
			StripeMarketplaceWebhookSecret: "whsec_gate",
		}, log)
		if len(got) != 1 {
			t.Fatalf("adapters = %v, want one", adapterNames(got))
		}
		st, ok := got[0].(payment.Stripe)
		if !ok {
			t.Fatalf("adapter type = %T, want payment.Stripe", got[0])
		}
		if st.WebhookSecret != "whsec_gate" {
			t.Fatalf("webhook secret = %q, want the gate's own", st.WebhookSecret)
		}
	})

	t.Run("falls back to the payments secret", func(t *testing.T) {
		log, _ := captureLog()
		got := marketplaceAdapters(config.Config{
			PaymentMethods:      []string{"stripe"},
			StripeSecretKey:     "sk_test_1",
			StripeWebhookSecret: "whsec_payments",
		}, log)
		if len(got) != 1 {
			t.Fatalf("adapters = %v, want one", adapterNames(got))
		}
		st, ok := got[0].(payment.Stripe)
		if !ok {
			t.Fatalf("adapter type = %T, want payment.Stripe", got[0])
		}
		if st.WebhookSecret != "whsec_payments" {
			t.Fatalf("webhook secret = %q, want the payments fallback", st.WebhookSecret)
		}
	})

	t.Run("stripe without a key is skipped", func(t *testing.T) {
		log, buf := captureLog()
		got := marketplaceAdapters(config.Config{PaymentMethods: []string{"stripe"}}, log)
		if len(got) != 0 {
			t.Fatalf("adapters = %v, want none", adapterNames(got))
		}
		if !strings.Contains(buf.String(), "STRIPE_SECRET_KEY") {
			t.Fatalf("warning missing, got %q", buf.String())
		}
	})
}

func TestProviderPluginsFollowConfig(t *testing.T) {
	t.Run("claude registers by slug", func(t *testing.T) {
		log, _ := captureLog()
		got := providerPlugins(config.Config{ProviderPlugins: []string{claude.Slug}}, log)
		if len(got) != 1 || got[0].Slug() != claude.Slug {
			t.Fatalf("plugins = %v, want one %q", pluginSlugs(got), claude.Slug)
		}
	})

	t.Run("unknown plugin is skipped with a warning", func(t *testing.T) {
		log, buf := captureLog()
		got := providerPlugins(config.Config{ProviderPlugins: []string{"bogus"}}, log)
		if len(got) != 0 {
			t.Fatalf("plugins = %v, want none", pluginSlugs(got))
		}
		if !strings.Contains(buf.String(), "bogus") {
			t.Fatalf("warning missing, got %q", buf.String())
		}
	})

	t.Run("empty config keeps every provider generic", func(t *testing.T) {
		log, _ := captureLog()
		if got := providerPlugins(config.Config{}, log); len(got) != 0 {
			t.Fatalf("plugins = %v, want none", pluginSlugs(got))
		}
	})
}
