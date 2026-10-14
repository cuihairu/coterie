package database_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/cuihairu/coterie/internal/database"
)

func TestParseDateRoundTrip(t *testing.T) {
	d, err := database.ParseDate("2026-01-02")
	if err != nil {
		t.Fatal(err)
	}
	if got := d.String(); got != "2026-01-02" {
		t.Fatalf("String() = %q, want 2026-01-02", got)
	}
	if _, err := database.ParseDate("2026-13-02"); err == nil {
		t.Fatal("expected an error for a non-existent month")
	}
	if _, err := database.ParseDate("2026-01-02T00:00:00Z"); err == nil {
		t.Fatal("expected an error for a timestamp instead of a date")
	}
}

func TestDateValue(t *testing.T) {
	d, err := database.ParseDate("2026-01-02")
	if err != nil {
		t.Fatal(err)
	}
	v, err := d.Value()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := v.(time.Time); !ok {
		t.Fatalf("Value() = %T, want time.Time", v)
	}

	var zero database.Date
	v, err = zero.Value()
	if err != nil {
		t.Fatal(err)
	}
	if v != nil {
		t.Fatalf("zero Date Value() = %v, want nil (SQL NULL)", v)
	}
}

func TestDateScan(t *testing.T) {
	cases := []struct {
		name string
		src  any
		want string
	}{
		{"null clears", nil, "0001-01-01"},
		{"time value", time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), "2026-01-02"},
		{"text value", "2026-01-02", "2026-01-02"},
		{"text bytes", []byte("2026-01-02"), "2026-01-02"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var d database.Date
			if err := d.Scan(tc.src); err != nil {
				t.Fatal(err)
			}
			if got := d.String(); got != tc.want {
				t.Fatalf("date = %q, want %q", got, tc.want)
			}
		})
	}

	t.Run("bad text", func(t *testing.T) {
		var d database.Date
		if err := d.Scan("yesterday"); err == nil {
			t.Fatal("expected an error for unparsable text")
		}
	})
	t.Run("unsupported type", func(t *testing.T) {
		var d database.Date
		if err := d.Scan(42); err == nil {
			t.Fatal("expected an error for an unsupported source type")
		}
	})
}

func TestDateJSON(t *testing.T) {
	t.Run("zero renders as null", func(t *testing.T) {
		var zero database.Date
		b, err := json.Marshal(zero)
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != "null" {
			t.Fatalf("marshal = %s, want null", b)
		}
	})

	t.Run("date renders as a bare string", func(t *testing.T) {
		d, err := database.ParseDate("2026-01-02")
		if err != nil {
			t.Fatal(err)
		}
		b, err := json.Marshal(d)
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != `"2026-01-02"` {
			t.Fatalf("marshal = %s, want \"2026-01-02\"", b)
		}
	})

	t.Run("null clears on decode", func(t *testing.T) {
		var d database.Date
		if err := json.Unmarshal([]byte("null"), &d); err != nil {
			t.Fatal(err)
		}
		if !d.IsZero() {
			t.Fatal("null should decode to the zero date")
		}
	})

	t.Run("string decodes", func(t *testing.T) {
		var d database.Date
		if err := json.Unmarshal([]byte(`"2026-01-02"`), &d); err != nil {
			t.Fatal(err)
		}
		if got := d.String(); got != "2026-01-02" {
			t.Fatalf("date = %q, want 2026-01-02", got)
		}
	})

	t.Run("non-string is rejected", func(t *testing.T) {
		var d database.Date
		if err := json.Unmarshal([]byte(`20260102`), &d); err == nil {
			t.Fatal("expected an error for a bare number")
		}
	})

	t.Run("bad format is rejected", func(t *testing.T) {
		var d database.Date
		if err := json.Unmarshal([]byte(`"02/01/2026"`), &d); err == nil {
			t.Fatal("expected an error for a non-ISO date")
		}
	})

	t.Run("struct field round trip", func(t *testing.T) {
		type period struct {
			Start database.Date `json:"start"`
		}
		var p period
		if err := json.Unmarshal([]byte(`{"start":"2026-01-02"}`), &p); err != nil {
			t.Fatal(err)
		}
		b, err := json.Marshal(p)
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != `{"start":"2026-01-02"}` {
			t.Fatalf("round trip = %s, want the bare date string", b)
		}
	})
}
