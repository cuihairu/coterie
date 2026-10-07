package notification_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/cuihairu/coterie/internal/testsupport"
)

// listNotifications fetches the actor's notification list.
func listNotifications(t *testing.T, client *http.Client, base, tok string, unreadOnly bool) map[string]any {
	t.Helper()
	url := base + "/api/v1/notifications"
	if unreadOnly {
		url += "?unread=true"
	}
	code, body := testsupport.DoAuthJSON(t, client, http.MethodGet, url, "", tok)
	if code != http.StatusOK {
		t.Fatalf("list notifications: status = %d: %v", code, body)
	}
	return body
}

// findByType returns the id of the actor's first notification of the
// given type.
func findByType(t *testing.T, resp map[string]any, typ string) string {
	t.Helper()
	items, _ := resp["items"].([]any)
	for _, raw := range items {
		item, _ := raw.(map[string]any)
		if item["type"] == typ {
			id, _ := item["id"].(string)
			return id
		}
	}
	return ""
}

func TestNotificationFlow(t *testing.T) {
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db)
	client := srv.Client()
	tok, _, subID, coterieID, joined := testsupport.SeedCircle(t, client, srv.URL, "notif", "12.00", 2, 1, 1)
	member := joined[0]

	// A fresh member has an empty inbox.
	body := listNotifications(t, client, srv.URL, member.Token, false)
	if total, _ := body["meta"].(map[string]any)["total"].(float64); total != 0 {
		t.Fatalf("fresh inbox total = %v, want 0", body["meta"])
	}

	// Generating contributions notifies every contributing member.
	code, period := testsupport.DoAuthJSON(t, client, http.MethodPost,
		srv.URL+"/api/v1/subscriptions/"+subID+"/billing-periods",
		`{"start_date":"2026-10-01","end_date":"2026-11-01"}`, tok)
	if code != http.StatusCreated {
		t.Fatalf("create period: status = %d: %v", code, period)
	}
	periodID, _ := period["id"].(string)
	code, _ = testsupport.DoAuthJSON(t, client, http.MethodPost,
		srv.URL+"/api/v1/billing-periods/"+periodID+"/contributions/generate", `{}`, tok)
	if code != http.StatusCreated {
		t.Fatalf("generate: status = %d", code)
	}
	body = listNotifications(t, client, srv.URL, member.Token, false)
	if n := findByType(t, body, "payment_due"); n == "" {
		t.Fatalf("member has no payment_due: %v", body)
	}
	ownerBox := listNotifications(t, client, srv.URL, tok, false)
	if n := findByType(t, ownerBox, "payment_due"); n == "" {
		t.Fatalf("owner has no payment_due: %v", ownerBox)
	}

	// Assigning a seat notifies the assignee.
	code, seats := testsupport.DoAuthJSON(t, client, http.MethodPost,
		srv.URL+"/api/v1/subscriptions/"+subID+"/seats", `{"count":1}`, tok)
	if code != http.StatusCreated {
		t.Fatalf("provision seats: status = %d: %v", code, seats)
	}
	seatItems, _ := seats["items"].([]any)
	seat, _ := seatItems[0].(map[string]any)
	seatID, _ := seat["id"].(string)
	code, assignBody := testsupport.DoAuthJSON(t, client, http.MethodPost,
		srv.URL+"/api/v1/seats/"+seatID+"/assign",
		fmt.Sprintf(`{"member_id":%q}`, member.MemberID), tok)
	if code != http.StatusOK {
		t.Fatalf("assign seat: status = %d: %v", code, assignBody)
	}
	body = listNotifications(t, client, srv.URL, member.Token, true)
	if n := findByType(t, body, "seat_assigned"); n == "" {
		t.Fatalf("member has no unread seat_assigned: %v", body)
	}
	if total, _ := body["meta"].(map[string]any)["total"].(float64); total != 2 {
		t.Fatalf("unread total = %v, want 2", body["meta"])
	}

	// Marking read clears it from the unread view; someone else's
	// notification is a 404.
	paymentDue := findByType(t, listNotifications(t, client, srv.URL, member.Token, true), "payment_due")
	code, read := testsupport.DoAuthJSON(t, client, http.MethodPost,
		srv.URL+"/api/v1/notifications/"+paymentDue+"/read", "", member.Token)
	if code != http.StatusOK || read["read_at"] == nil {
		t.Fatalf("mark read: status = %d body = %v", code, read)
	}
	body = listNotifications(t, client, srv.URL, member.Token, true)
	if total, _ := body["meta"].(map[string]any)["total"].(float64); total != 1 {
		t.Fatalf("unread after read = %v, want 1", body["meta"])
	}
	ownerNotification := findByType(t, ownerBox, "payment_due")
	code, _ = testsupport.DoAuthJSON(t, client, http.MethodPost,
		srv.URL+"/api/v1/notifications/"+ownerNotification+"/read", "", member.Token)
	if code != http.StatusNotFound {
		t.Fatalf("read someone else's notification: status = %d, want 404", code)
	}

	// Closing the coterie notifies the remaining members.
	code, _ = testsupport.DoAuthJSON(t, client, http.MethodPatch,
		srv.URL+"/api/v1/coteries/"+coterieID, `{"status":"closed"}`, tok)
	if code != http.StatusOK {
		t.Fatalf("close coterie: status = %d", code)
	}
	body = listNotifications(t, client, srv.URL, member.Token, false)
	if n := findByType(t, body, "coterie_closed"); n == "" {
		t.Fatalf("member has no coterie_closed: %v", body)
	}
	if total, _ := body["meta"].(map[string]any)["total"].(float64); total != 3 {
		t.Fatalf("inbox total = %v, want 3", body["meta"])
	}
}
