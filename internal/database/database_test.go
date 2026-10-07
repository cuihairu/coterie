package database_test

import (
	"testing"

	"github.com/cuihairu/coterie/internal/testsupport"
)

// TestSessionTimezoneIsUTC pins the session timezone: pgx would
// otherwise use the client's local zone, making timestamptz rendering
// depend on where the server runs.
func TestSessionTimezoneIsUTC(t *testing.T) {
	db := testsupport.NewDB(t)

	var tz string
	if err := db.Raw("SHOW timezone").Scan(&tz).Error; err != nil {
		t.Fatal(err)
	}
	if tz != "UTC" {
		t.Fatalf("session timezone = %q, want UTC", tz)
	}
}
