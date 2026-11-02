-- Forbid overlapping billing periods per subscription (code review C1).
-- Periods use half-open windows [start_date, end_date): the usage split
-- reads recorded_at >= start AND recorded_at < end, so two periods are
-- disjoint exactly when each starts on or after the other's end. A
-- boundary day belongs to the later period only, and back-to-back periods
-- (end == next start) are allowed — the rollover convention opens the
-- next period on end + 1 day. True overlaps made usage-limit windows and
-- usage splits attribute the same records to two periods (PeriodCovering
-- picks the earliest start, and each split reads its own window). The
-- service rejects overlaps with a 422 first (a same-start date is kept a
-- 409 by the (subscription_id, start_date) unique key); this constraint is
-- the DB-level guarantee against the race between two concurrent creates.
-- Existing overlapping rows — if any — must be resolved by hand before
-- this migration can apply.

CREATE EXTENSION IF NOT EXISTS btree_gist;

ALTER TABLE billing_periods
    ADD CONSTRAINT billing_periods_no_overlap
    EXCLUDE USING gist (
        subscription_id WITH =,
        daterange(start_date, end_date, '[)') WITH &&
    );
