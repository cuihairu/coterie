-- Usage records: append-only metered consumption ledger (Phase 2 usage
-- tracking). Rows are never updated or deleted — corrections are
-- negative records. Quota seats (D4) mirror seat-attributed sums into
-- metadata.used as a projection (design D9).
CREATE TABLE usage_records (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    subscription_id uuid NOT NULL REFERENCES subscriptions,
    member_id       uuid NOT NULL REFERENCES members,
    seat_id         uuid REFERENCES seats,
    amount          numeric(14,4) NOT NULL,
    unit            text NOT NULL CHECK (unit <> ''),
    metadata        jsonb NOT NULL DEFAULT '{}',
    recorded_at     timestamptz NOT NULL DEFAULT now(),
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX usage_records_subscription_idx ON usage_records (subscription_id, recorded_at);
CREATE INDEX usage_records_member_idx ON usage_records (member_id);
CREATE INDEX usage_records_seat_idx ON usage_records (seat_id);
