package notification_test

import (
	"bufio"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/hkdf"

	"github.com/cuihairu/coterie/internal/notification"
	"github.com/cuihairu/coterie/internal/push"
	"github.com/cuihairu/coterie/internal/testsupport"
)

// testNotification is a representative in-app row handed to channels.
func testNotification() *notification.Notification {
	return &notification.Notification{
		ID:         "n-1",
		UserID:     "u-1",
		Type:       "payment_received",
		Title:      "Payment received",
		Body:       "A payment of 6.00 USD was recorded.",
		EntityType: "contribution",
		EntityID:   "c-9",
		CreatedAt:  time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC),
	}
}

func TestWebhookChannelSignsAndPosts(t *testing.T) {
	type received struct {
		body      []byte
		event     string
		signature string
	}
	got := make(chan received, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got <- received{body: body, event: r.Header.Get("X-Coterie-Event"), signature: r.Header.Get("X-Coterie-Signature")}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	const secret = "whsec-test"
	ch := notification.NewWebhookChannel(srv.URL, secret)
	n := testNotification()
	if err := ch.Deliver(context.Background(), n); err != nil {
		t.Fatalf("deliver: %v", err)
	}

	select {
	case r := <-got:
		var payload map[string]any
		if err := json.Unmarshal(r.body, &payload); err != nil {
			t.Fatalf("payload not json: %v: %s", err, r.body)
		}
		if payload["type"] != "payment_received" || payload["title"] != "Payment received" {
			t.Fatalf("payload = %v", payload)
		}
		if payload["user_id"] != "u-1" || payload["entity_id"] != "c-9" {
			t.Fatalf("payload = %v", payload)
		}
		if r.event != "payment_received" {
			t.Fatalf("event header = %q", r.event)
		}
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(r.body)
		want := hex.EncodeToString(mac.Sum(nil))
		if !hmac.Equal([]byte(r.signature), []byte(want)) {
			t.Fatalf("signature = %q, want %q", r.signature, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("webhook receiver got nothing")
	}
}

func TestWebhookChannelRejectsNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	ch := notification.NewWebhookChannel(srv.URL, "")
	if err := ch.Deliver(context.Background(), testNotification()); err == nil {
		t.Fatal("deliver to 500 receiver must fail")
	}
}

// fakeSMTP is a minimal SMTP server capturing one message's DATA.
type fakeSMTP struct {
	listener net.Listener
	data     chan string
}

func startFakeSMTP(t *testing.T) *fakeSMTP {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	f := &fakeSMTP{listener: l, data: make(chan string, 1)}
	go f.serve()
	t.Cleanup(func() { l.Close() })
	return f
}

func (f *fakeSMTP) addr() string { return f.listener.Addr().String() }

func (f *fakeSMTP) serve() {
	conn, err := f.listener.Accept()
	if err != nil {
		return
	}
	defer conn.Close()
	r := bufio.NewReader(conn)
	w := bufio.NewWriter(conn)
	write := func(s string) { w.WriteString(s + "\r\n"); w.Flush() }

	write("220 test ESMTP")
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		cmd := strings.ToUpper(strings.TrimSpace(line))
		switch {
		case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
			write("250 test")
		case strings.HasPrefix(cmd, "MAIL"), strings.HasPrefix(cmd, "RCPT"):
			write("250 ok")
		case strings.HasPrefix(cmd, "DATA"):
			write("354 go")
			var body strings.Builder
			for {
				l2, err := r.ReadString('\n')
				if err != nil {
					return
				}
				body.WriteString(l2)
				if strings.TrimSpace(l2) == "." {
					break
				}
			}
			f.data <- body.String()
			write("250 queued")
		case strings.HasPrefix(cmd, "QUIT"):
			write("221 bye")
			return
		default:
			write("250 ok")
		}
	}
}

func TestEmailChannelDelivers(t *testing.T) {
	// The channel resolves the recipient address from the user row,
	// so seed a real user through the API.
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db)
	client := srv.Client()
	_, userID := testsupport.RegisterAndLogin(t, client, srv.URL, "chan-mail")

	smtpSrv := startFakeSMTP(t)
	host, port, _ := net.SplitHostPort(smtpSrv.addr())
	ch := notification.NewEmailChannel(db, host, port, "", "", "noreply@coterie.test")
	if !ch.Enabled() {
		t.Fatal("channel must report enabled with host set")
	}

	n := testNotification()
	n.UserID = userID
	n.Title = "Payment received"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := ch.Deliver(ctx, n); err != nil {
		t.Fatalf("deliver: %v", err)
	}

	select {
	case body := <-smtpSrv.data:
		for _, want := range []string{"From: noreply@coterie.test", "Subject: Payment received", "A payment of 6.00 USD"} {
			if !strings.Contains(body, want) {
				t.Fatalf("message missing %q: %q", want, body)
			}
		}
	case <-time.After(2 * time.Second):
		t.Fatal("SMTP server captured nothing")
	}
}

// TestDispatchThroughApp drives a real domain action (seat assignment)
// on a server wired with a webhook channel and waits for the receiver
// to see the event — the full Notify → dispatchAsync → Channel path.
func TestDispatchThroughApp(t *testing.T) {
	received := make(chan map[string]any, 1)
	sink := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		if json.Unmarshal(body, &payload) == nil {
			received <- payload
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer sink.Close()

	db := testsupport.NewDB(t)
	srv := testsupport.NewServerWithChannels(t, db, notification.NewWebhookChannel(sink.URL, ""))
	client, base := srv.Client(), srv.URL

	tok, _, subID, coterieID, joined := testsupport.SeedCircle(t, client, base, "chan-disp", "10.00", 2, 1, 1)
	_ = coterieID

	code, body := testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/subscriptions/"+subID+"/seats", "", tok)
	if code != http.StatusOK {
		t.Fatalf("list seats: status = %d: %v", code, body)
	}
	items, _ := body["items"].([]any)
	seat, _ := items[0].(map[string]any)
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/seats/"+seat["id"].(string)+"/assign",
		fmt.Sprintf(`{"member_id":%q}`, joined[0].MemberID), tok)
	if code != http.StatusOK {
		t.Fatalf("assign seat: status = %d: %v", code, body)
	}

	select {
	case payload := <-received:
		if payload["type"] != "seat_assigned" {
			t.Fatalf("event type = %v, want seat_assigned", payload["type"])
		}
		if payload["user_id"] != joined[0].UserID {
			t.Fatalf("user_id = %v, want %s", payload["user_id"], joined[0].UserID)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("webhook channel delivered nothing")
	}
}

// TestPushChannelDeliversAndPrunes covers the D22 channel: payloads
// arrive encrypted (the fake push service decrypts with the client
// key), and 410 endpoints are pruned from the registry.
func TestPushChannelDeliversAndPrunes(t *testing.T) {
	db := testsupport.NewDB(t)

	// The channel resolves endpoints by the recipient's user id, so
	// seed a real user through the API (uuid column).
	srvAPI := testsupport.NewServer(t, db)
	userID := func() string {
		tok, id := testsupport.RegisterAndLogin(t, srvAPI.Client(), srvAPI.URL, "chan-push")
		_ = tok
		return id
	}()

	// Client-side keys the fake push service decrypts with.
	priv, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	authSecret := make([]byte, 16)
	if _, err := rand.Read(authSecret); err != nil {
		t.Fatal(err)
	}
	enc := base64.RawURLEncoding

	type pushReq struct {
		auth   string
		ttl    string
		encHdr string
		body   []byte
	}
	seen := make(chan pushReq, 4)
	gone := make(chan int, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		seen <- pushReq{auth: r.Header.Get("Authorization"), ttl: r.Header.Get("TTL"), encHdr: r.Header.Get("Content-Encoding"), body: body}
		if strings.HasSuffix(r.URL.Path, "/gone") {
			gone <- http.StatusGone
			w.WriteHeader(http.StatusGone)
			return
		}
		gone <- 0
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	pubB64, privB64 := vapidPair(t)
	ch := notification.NewPushChannel(push.NewService(db).Store(),
		pubB64, privB64, "mailto:ops@example.com")
	if !ch.Enabled() {
		t.Fatal("channel disabled with keys present")
	}

	// Register one healthy and one doomed endpoint for the user.
	store := push.NewService(db).Store()
	for _, ep := range []string{srv.URL + "/alive", srv.URL + "/gone"} {
		if err := store.UpsertByEndpoint(context.Background(), &push.Subscription{
			ID: uuid.NewString(), UserID: userID, Endpoint: ep,
			P256dh: enc.EncodeToString(priv.PublicKey().Bytes()), Auth: enc.EncodeToString(authSecret),
		}); err != nil {
			t.Fatal(err)
		}
	}

	note := testNotification()
	note.UserID = userID
	if err := ch.Deliver(context.Background(), note); err != nil {
		t.Fatalf("deliver: %v", err)
	}

	for i := 0; i < 2; i++ {
		req := <-seen
		if req.ttl == "" || req.encHdr != "aes128gcm" || !strings.HasPrefix(req.auth, "vapid t=") {
			t.Fatalf("push request headers: %+v", req)
		}
		pt := decryptForTest(t, req.body, priv, authSecret)
		var payload map[string]string
		if err := json.Unmarshal(pt, &payload); err != nil {
			t.Fatalf("payload not the notification json: %v (%q)", err, pt)
		}
		if payload["title"] != "Payment received" {
			t.Fatalf("payload title: %v", payload)
		}
	}
	if g1, g2 := <-gone, <-gone; g1 != http.StatusGone && g2 != http.StatusGone {
		t.Fatalf("expected a 410 exchange, got %d and %d", g1, g2)
	}
	// The doomed endpoint must be pruned; the healthy one kept.
	subs, err := store.ListByUser(context.Background(), userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 1 || !strings.HasSuffix(subs[0].Endpoint, "/alive") {
		t.Fatalf("prune left %d subs: %+v", len(subs), subs)
	}
}

// vapidPair mints a valid VAPID key pair in the config encoding.
func vapidPair(t *testing.T) (string, string) {
	t.Helper()
	priv, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	enc := base64.RawURLEncoding
	return enc.EncodeToString(priv.PublicKey().Bytes()), enc.EncodeToString(priv.Bytes())
}

// decryptForTest is the receiver side of RFC 8291 for channel tests.
func decryptForTest(t *testing.T, body []byte, priv *ecdh.PrivateKey, authSecret []byte) []byte {
	t.Helper()
	salt, rest := body[:16], body[16:]
	rs := new(big.Int).SetBytes(rest[:4]).Uint64()
	idlen := int(rest[4])
	keyid := rest[5 : 5+idlen]
	ct := rest[5+idlen:]
	if int(rs) != len(body) {
		t.Fatalf("rs = %d, body = %d", rs, len(body))
	}
	shared, err := priv.ECDH(mustECPub(t, keyid))
	if err != nil {
		t.Fatal(err)
	}
	info := append([]byte("WebPush: info\x00"), priv.PublicKey().Bytes()...)
	info = append(info, keyid...)
	prkKey := hkdfRead(t, shared, authSecret, info, 32)
	ikm := hkdfRead(t, prkKey, salt, []byte("Content-Encoding: aes128gcm\x00"), 16)
	nonce := hkdfRead(t, prkKey, salt, []byte("Content-Encoding: nonce\x00"), 12)
	block, _ := aes.NewCipher(ikm)
	gcm, _ := cipher.NewGCM(block)
	pt, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	return pt[:len(pt)-1]
}

func mustECPub(t *testing.T, raw []byte) *ecdh.PublicKey {
	t.Helper()
	p, err := ecdh.P256().NewPublicKey(raw)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func hkdfRead(t *testing.T, secret, salt, info []byte, n int) []byte {
	t.Helper()
	out := make([]byte, n)
	r := hkdf.New(sha256.New, secret, salt, info)
	if _, err := r.Read(out); err != nil {
		t.Fatal(err)
	}
	return out
}
