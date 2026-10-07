-- Notifications: in-app messages for a user (FR-12, Phase 1 = Web
-- channel; Email arrives with the Phase 2 adapter).
CREATE TABLE notifications (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     uuid NOT NULL REFERENCES users,
    type        text NOT NULL
                CHECK (type IN ('invitation', 'payment_due', 'subscription_renewal',
                                'seat_assigned', 'subscription_expired',
                                'coterie_closed', 'system')),
    title       text NOT NULL,
    body        text NOT NULL DEFAULT '',
    entity_type text,
    entity_id   uuid,
    read_at     timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX notifications_user_idx ON notifications (user_id, created_at DESC);
