-- Marketplace (Phase 2, design D10): opt-in directory listing plus the
-- join request inbox. Coteries stay private unless the owner opts in.
ALTER TABLE coteries
    ADD COLUMN listing text NOT NULL DEFAULT 'private'
    CHECK (listing IN ('private', 'public'));

CREATE TABLE join_requests (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    coterie_id uuid NOT NULL REFERENCES coteries,
    user_id    uuid NOT NULL REFERENCES users,
    message    text NOT NULL DEFAULT '',
    status     text NOT NULL DEFAULT 'pending'
               CHECK (status IN ('pending', 'accepted', 'declined', 'cancelled')),
    created_at timestamptz NOT NULL DEFAULT now(),
    decided_at timestamptz
);

-- At most one live request per user per coterie; decided rows keep
-- history.
CREATE UNIQUE INDEX join_requests_pending_key
    ON join_requests (coterie_id, user_id) WHERE status = 'pending';
CREATE INDEX join_requests_coterie_idx ON join_requests (coterie_id, status);

-- New notification kinds for the request flow.
ALTER TABLE notifications DROP CONSTRAINT notifications_type_check;
ALTER TABLE notifications
    ADD CONSTRAINT notifications_type_check
    CHECK (type IN ('invitation', 'payment_due', 'subscription_renewal',
                    'seat_assigned', 'subscription_expired',
                    'coterie_closed', 'system',
                    'join_requested', 'join_decided'));
