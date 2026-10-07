package database

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// JSONB maps a PostgreSQL jsonb column to arbitrary JSON.
// The zero value marshals and stores as an empty object.
type JSONB json.RawMessage

// Value implements driver.Valuer.
func (j JSONB) Value() (driver.Value, error) {
	if len(j) == 0 {
		return "{}", nil
	}
	if !json.Valid(j) {
		return nil, fmt.Errorf("invalid JSON: %s", j)
	}
	return string(j), nil
}

// Scan implements sql.Scanner.
func (j *JSONB) Scan(src any) error {
	switch v := src.(type) {
	case nil:
		*j = JSONB("{}")
	case []byte:
		*j = JSONB(append(json.RawMessage{}, v...))
	case string:
		*j = JSONB(v)
	default:
		return fmt.Errorf("unsupported type %T for JSONB", src)
	}
	return nil
}

// MarshalJSON renders the stored bytes as-is.
func (j JSONB) MarshalJSON() ([]byte, error) {
	if len(j) == 0 {
		return []byte("{}"), nil
	}
	return j, nil
}

// UnmarshalJSON stores raw bytes after validating them.
func (j *JSONB) UnmarshalJSON(data []byte) error {
	if !json.Valid(data) {
		return fmt.Errorf("invalid JSON: %s", data)
	}
	*j = JSONB(append(json.RawMessage{}, data...))
	return nil
}
