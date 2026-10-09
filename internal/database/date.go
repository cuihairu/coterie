package database

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

const dateLayout = "2006-01-02"

// Date maps a PostgreSQL date column to a calendar date. It renders as
// and parses from "YYYY-MM-DD" in JSON, so the API never leaks a bogus
// midnight timestamp for date-only fields.
type Date struct {
	time.Time
}

// ParseDate builds a Date from "YYYY-MM-DD".
func ParseDate(s string) (Date, error) {
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		return Date{}, fmt.Errorf("invalid date %q: %w", s, err)
	}
	return Date{Time: t}, nil
}

// String renders the date as "YYYY-MM-DD".
func (d Date) String() string { return d.Format(dateLayout) }

// Value implements driver.Valuer; a zero Date becomes SQL NULL.
func (d Date) Value() (driver.Value, error) {
	if d.IsZero() {
		return nil, nil
	}
	return d.Time, nil
}

// Scan implements sql.Scanner; source may be a time.Time (binary
// protocol) or raw text (simple protocol).
func (d *Date) Scan(src any) error {
	switch v := src.(type) {
	case nil:
		d.Time = time.Time{}
	case time.Time:
		d.Time = v
	case string:
		return d.parseText(v)
	case []byte:
		return d.parseText(string(v))
	default:
		return fmt.Errorf("unsupported type %T for Date", src)
	}
	return nil
}

func (d *Date) parseText(s string) error {
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		return fmt.Errorf("scan date %q: %w", s, err)
	}
	d.Time = t
	return nil
}

// MarshalJSON renders as "YYYY-MM-DD"; a zero Date renders as null.
func (d Date) MarshalJSON() ([]byte, error) {
	if d.IsZero() {
		return []byte("null"), nil
	}
	return []byte(`"` + d.String() + `"`), nil
}

// UnmarshalJSON accepts "YYYY-MM-DD" or null.
func (d *Date) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		d.Time = time.Time{}
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("date must be a \"YYYY-MM-DD\" string: %w", err)
	}
	parsed, err := ParseDate(s)
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}
