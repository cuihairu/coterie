package database_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/cuihairu/coterie/internal/database"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestSQLStatePredicates(t *testing.T) {
	t.Run("unique violation", func(t *testing.T) {
		err := &pgconn.PgError{Code: "23505", ConstraintName: "users_email_key"}
		if !database.IsUniqueViolation(err) {
			t.Fatal("23505 should read as a unique violation")
		}
		if database.IsUniqueViolation(&pgconn.PgError{Code: "23503"}) {
			t.Fatal("23503 is not a unique violation")
		}
	})

	t.Run("foreign key violation", func(t *testing.T) {
		err := &pgconn.PgError{Code: "23503", ConstraintName: "fk_child"}
		if !database.IsFKViolation(err) {
			t.Fatal("23503 should read as a foreign key violation")
		}
		if database.IsFKViolation(&pgconn.PgError{Code: "23505"}) {
			t.Fatal("23505 is not a foreign key violation")
		}
	})

	t.Run("wrapped errors are unwrapped", func(t *testing.T) {
		inner := &pgconn.PgError{Code: "23505"}
		if !database.IsUniqueViolation(fmt.Errorf("create: %w", inner)) {
			t.Fatal("a wrapped unique violation should still be recognized")
		}
	})

	t.Run("other errors are not pg errors", func(t *testing.T) {
		plain := errors.New("connection reset")
		if database.IsUniqueViolation(plain) || database.IsFKViolation(plain) || database.IsCheckViolation(plain) {
			t.Fatal("a plain error must not read as any SQLSTATE violation")
		}
		if database.IsUniqueViolation(nil) || database.IsFKViolation(nil) || database.IsCheckViolation(nil) {
			t.Fatal("nil must not read as a violation")
		}
	})
}

func TestIsCheckViolation(t *testing.T) {
	err := &pgconn.PgError{Code: "23514", ConstraintName: "subscriptions_seats_check"}

	t.Run("empty list matches any check violation", func(t *testing.T) {
		if !database.IsCheckViolation(err) {
			t.Fatal("no constraint list should match any 23514")
		}
	})

	t.Run("named constraint matches", func(t *testing.T) {
		if !database.IsCheckViolation(err, "other_check", "subscriptions_seats_check") {
			t.Fatal("a listed constraint should match")
		}
	})

	t.Run("other constraint does not match", func(t *testing.T) {
		if database.IsCheckViolation(err, "coteries_capacity_check") {
			t.Fatal("a constraint the row did not trip must not match")
		}
	})

	t.Run("other sqlstate does not match", func(t *testing.T) {
		if database.IsCheckViolation(&pgconn.PgError{Code: "23505", ConstraintName: "subscriptions_seats_check"}) {
			t.Fatal("only 23514 is a check violation")
		}
	})
}
