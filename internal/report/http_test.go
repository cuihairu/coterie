package report_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/cuihairu/coterie/internal/testsupport"
	"github.com/cuihairu/coterie/internal/user"
)

// seedCoterie creates provider → product → subscription → coterie for
// the owner and returns the coterie id. listed toggles the marketplace
// listing: only publicly listed circles are reportable.
func seedCoterie(t *testing.T, client *http.Client, base, tag, tok, ownerID string, listed bool) string {
	t.Helper()
	code, body := testsupport.DoAuthJSON(t, client, http.MethodPost, base+"/api/v1/providers",
		fmt.Sprintf(`{"slug":"rpt-%s","name":"Report Provider %s","category":"video"}`, tag, tag), tok)
	if code != http.StatusCreated {
		t.Fatalf("seed provider: status = %d: %v", code, body)
	}
	providerID, _ := body["id"].(string)
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost, base+"/api/v1/products",
		fmt.Sprintf(`{"provider_id":%q,"name":"Report Product %s"}`, providerID, tag), tok)
	if code != http.StatusCreated {
		t.Fatalf("seed product: status = %d: %v", code, body)
	}
	productID, _ := body["id"].(string)
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost, base+"/api/v1/subscriptions",
		fmt.Sprintf(`{"product_id":%q,"owner_user_id":%q,"billing_cycle":"monthly","price":"9.99","currency":"USD","start_date":"2026-10-01","max_seats":2}`, productID, ownerID), tok)
	if code != http.StatusCreated {
		t.Fatalf("seed subscription: status = %d: %v", code, body)
	}
	subID, _ := body["id"].(string)
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost, base+"/api/v1/coteries",
		fmt.Sprintf(`{"subscription_id":%q,"name":"Report Circle %s","capacity":2}`, subID, tag), tok)
	if code != http.StatusCreated {
		t.Fatalf("seed coterie: status = %d: %v", code, body)
	}
	id, _ := body["id"].(string)
	patch := `{"status":"open"}`
	if listed {
		patch = `{"status":"open","listing":"public"}`
	}
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPatch, base+"/api/v1/coteries/"+id, patch, tok)
	if code != http.StatusOK {
		t.Fatalf("publish coterie: status = %d: %v", code, body)
	}
	return id
}

func TestReportFlow(t *testing.T) {
	db := testsupport.NewDB(t)
	srv := testsupport.NewServer(t, db)
	client := srv.Client()
	base := srv.URL

	otok, oid := testsupport.RegisterAndLogin(t, client, base, "rpt-owner")
	public := seedCoterie(t, client, base, "pub", otok, oid, true)
	private := seedCoterie(t, client, base, "prv", otok, oid, false)

	rtok, _ := testsupport.RegisterAndLogin(t, client, base, "rpt-reporter")

	// Reasons are validated before anything else.
	code, body := testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/coteries/"+public+"/report", `{"reason":"   "}`, rtok)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("blank reason status = %d, want 422: %v", code, body)
	}
	code, _ = testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/coteries/"+public+"/report",
		fmt.Sprintf(`{"reason":%q}`, strings.Repeat("x", 1001)), rtok)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("long reason status = %d, want 422", code)
	}

	// Private and unknown circles 404 alike.
	code, _ = testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/coteries/"+private+"/report", `{"reason":"smells"}`, rtok)
	if code != http.StatusNotFound {
		t.Fatalf("private coterie report status = %d, want 404", code)
	}
	code, _ = testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/coteries/00000000-0000-0000-0000-000000000000/report",
		`{"reason":"ghost"}`, rtok)
	if code != http.StatusNotFound {
		t.Fatalf("unknown coterie report status = %d, want 404", code)
	}

	// A clean filing carries the triage identities.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/coteries/"+public+"/report", `{"reason":"spam listings"}`, rtok)
	if code != http.StatusCreated {
		t.Fatalf("report status = %d, want 201: %v", code, body)
	}
	if body["status"] != "open" || body["coterie_name"] == "" ||
		body["reporter_username"] == "" || body["decided_at"] != nil {
		t.Fatalf("report view wrong: %v", body)
	}
	rid, _ := body["id"].(string)

	// One open report per user and coterie.
	code, _ = testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/coteries/"+public+"/report", `{"reason":"again"}`, rtok)
	if code != http.StatusConflict {
		t.Fatalf("duplicate open report status = %d, want 409", code)
	}

	// The inbox is admin-only. Promote an admin through the same
	// bootstrap the binary uses.
	atok, aid := testsupport.RegisterAndLogin(t, client, base, "rpt-admin")
	if err := db.Model(&user.User{}).Where("id = ?", aid).
		Update("role", user.RoleAdmin).Error; err != nil {
		t.Fatalf("promote admin: %v", err)
	}
	code, _ = testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/admin/reports?status=open", "", rtok)
	if code != http.StatusForbidden {
		t.Fatalf("non-admin inbox status = %d, want 403", code)
	}

	// Garbage filters are rejected rather than silently empty.
	code, _ = testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/admin/reports?status=wat", "", atok)
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("bad status filter = %d, want 422", code)
	}

	code, body = testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/admin/reports?status=open", "", atok)
	if code != http.StatusOK {
		t.Fatalf("admin inbox status = %d: %v", code, body)
	}
	meta, _ := body["meta"].(map[string]any)
	items, _ := body["items"].([]any)
	if meta["total"] != float64(1) || len(items) != 1 ||
		items[0].(map[string]any)["id"] != rid {
		t.Fatalf("inbox contents wrong: %v", body)
	}

	// Decisions: note lands, decided_at set, replay 409, unknown 404.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/admin/reports/"+rid+"/resolve", `{"note":"handled off-platform"}`, atok)
	if code != http.StatusOK {
		t.Fatalf("resolve status = %d: %v", code, body)
	}
	if body["status"] != "resolved" || body["decided_at"] == nil ||
		body["resolution_note"] != "handled off-platform" {
		t.Fatalf("resolved view wrong: %v", body)
	}
	code, _ = testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/admin/reports/"+rid+"/dismiss", `{}`, atok)
	if code != http.StatusConflict {
		t.Fatalf("re-decide status = %d, want 409", code)
	}
	code, _ = testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/admin/reports/00000000-0000-0000-0000-000000000000/resolve",
		`{}`, atok)
	if code != http.StatusNotFound {
		t.Fatalf("unknown report decide status = %d, want 404", code)
	}

	// The open inbox drains; the resolved one holds the decision.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodGet,
		base+"/api/v1/admin/reports?status=open", "", atok)
	if code != http.StatusOK || body["meta"].(map[string]any)["total"] != float64(0) {
		t.Fatalf("open inbox after resolve wrong: %v", body)
	}

	// A decided report frees the reporter to file again; dismiss
	// without a note keeps the note empty.
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/coteries/"+public+"/report", `{"reason":"still spammy"}`, rtok)
	if code != http.StatusCreated {
		t.Fatalf("re-report status = %d, want 201: %v", code, body)
	}
	rid2, _ := body["id"].(string)
	code, body = testsupport.DoAuthJSON(t, client, http.MethodPost,
		base+"/api/v1/admin/reports/"+rid2+"/dismiss", `{}`, atok)
	if code != http.StatusOK || body["status"] != "dismissed" ||
		body["resolution_note"] != nil {
		t.Fatalf("dismiss view wrong: %v", body)
	}
}
