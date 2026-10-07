-- Payment records (design §4.2): a payment marks one contribution's
-- receivable as received, driven by the configured payment adapter.
-- Append-only like usage_records — corrections are new rows, never
-- edits. The method/status CHECKs pin today's single adapter; new
-- channels extend the list by migration.

CREATE TABLE payments (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    contribution_id uuid NOT NULL REFERENCES contributions,
    subscription_id uuid NOT NULL REFERENCES subscriptions,
    payer_user_id   uuid NOT NULL REFERENCES users,
    amount          numeric(12,2) NOT NULL CHECK (amount >= 0),
    currency        char(3) NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    method          text NOT NULL CHECK (method IN ('manual')),
    status          text NOT NULL CHECK (status IN ('succeeded')),
    external_ref    text,
    paid_at         timestamptz NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX payments_contribution_idx ON payments (contribution_id);
CREATE INDEX payments_subscription_idx ON payments (subscription_id);

-- Payment-received notification kind.
ALTER TABLE notifications DROP CONSTRAINT notifications_type_check;
ALTER TABLE notifications
    ADD CONSTRAINT notifications_type_check
    CHECK (type IN ('invitation', 'payment_due', 'subscription_renewal',
                    'seat_assigned', 'subscription_expired',
                    'coterie_closed', 'system',
                    'join_requested', 'join_decided',
                    'payment_received'));
