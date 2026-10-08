-- Audit log (Phase 4, FR-15 / D17): wire the M1 skeleton table
-- (0001_init: id/actor_user_id/action/entity/before_state/after_state)
-- into the recording backbone — subscription/coterie query pivots, an
-- action CHECK extended by migration like every other enum, and the
-- list indexes. Entries are appended inside the same business
-- transaction as the mutation they describe; there is no update or
-- delete surface.

ALTER TABLE audit_logs
    ADD COLUMN subscription_id uuid REFERENCES subscriptions(id) ON DELETE CASCADE,
    ADD COLUMN coterie_id uuid REFERENCES coteries(id) ON DELETE CASCADE;

ALTER TABLE audit_logs
    ADD CONSTRAINT audit_logs_action_check CHECK (action IN (
        'coterie_created', 'coterie_updated', 'member_removed', 'member_left',
        'seat_assigned', 'seat_released', 'seat_updated', 'subscription_updated',
        'contribution_updated', 'payment_recorded', 'period_closed'
    ));

CREATE INDEX audit_logs_subscription_idx ON audit_logs (subscription_id, created_at DESC);
CREATE INDEX audit_logs_coterie_idx ON audit_logs (coterie_id, created_at DESC);
