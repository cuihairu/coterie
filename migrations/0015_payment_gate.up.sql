-- Payment gate (design D25): the coterie-level opt-in plus the
-- admission-charge ledger for pre-membership receipts.

ALTER TABLE coteries ADD COLUMN payment_gate boolean NOT NULL DEFAULT false;

-- The owner's consent with admission held for payment.
ALTER TABLE join_requests DROP CONSTRAINT join_requests_status_check;
ALTER TABLE join_requests
    ADD CONSTRAINT join_requests_status_check
    CHECK (status IN ('pending', 'awaiting_payment', 'accepted', 'declined', 'cancelled'));

CREATE TABLE admission_charges (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    coterie_id      uuid NOT NULL REFERENCES coteries,
    user_id         uuid NOT NULL REFERENCES users,
    join_request_id uuid NOT NULL REFERENCES join_requests,
    amount          numeric(12,2) NOT NULL CHECK (amount >= 0),
    currency        char(3) NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    method          text NOT NULL CHECK (method IN ('manual', 'sandbox', 'stripe')),
    status          text NOT NULL CHECK (status IN ('pending', 'succeeded', 'failed')),
    external_ref    text,
    created_at      timestamptz NOT NULL DEFAULT now(),
    confirmed_at    timestamptz
);

-- Webhook idempotency: one ledger row per channel transaction.
CREATE UNIQUE INDEX admission_charges_external_ref_key
    ON admission_charges (external_ref) WHERE external_ref IS NOT NULL;
-- At most one live (or settled) charge per join request; failed ones
-- may be retried with a fresh row.
CREATE UNIQUE INDEX admission_charges_one_live_key
    ON admission_charges (join_request_id) WHERE status IN ('pending', 'succeeded');
CREATE INDEX admission_charges_coterie_idx ON admission_charges (coterie_id);

-- The gate confirmed the join, so the requester joins: a dedicated
-- notification kind keeps the inbox honest.
ALTER TABLE notifications DROP CONSTRAINT notifications_type_check;
ALTER TABLE notifications
    ADD CONSTRAINT notifications_type_check
    CHECK (type IN ('invitation', 'payment_due', 'subscription_renewal',
                    'seat_assigned', 'subscription_expired',
                    'coterie_closed', 'system',
                    'join_requested', 'join_decided',
                    'payment_received', 'admission_due',
                    'dispute_opened', 'dispute_decided'));
