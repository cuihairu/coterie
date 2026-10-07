package notification_test

import (
	"bufio"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/coterie/internal/notification"
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
