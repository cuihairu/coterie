-- Dispute (Phase 3, FR-17): a member challenges a contribution; the
-- subscription owner decides. Resolution is advisory — the owner
-- adjusts the contribution itself through the existing settlement
-- surface, keeping the dispute ledger decoupled from money movement.

CREATE TABLE disputes (
    id                uuid PRIMARY KEY,
    contribution_id   uuid NOT NULL REFERENCES contributions(id),
    subscription_id   uuid NOT NULL REFERENCES subscriptions(id),
    raised_by         uuid NOT NULL REFERENCES users(id),
    reason            text NOT NULL,
    evidence          text,
    status            text NOT NULL,
    resolution_note   text,
    decided_by        uuid REFERENCES users(id),
    created_at        timestamptz NOT NULL DEFAULT now(),
    decided_at        timestamptz,
    CONSTRAINT disputes_status_check CHECK (status IN ('open', 'resolved', 'rejected')),
    CONSTRAINT disputes_reason_len CHECK (char_length(reason) <= 1000),
    CONSTRAINT disputes_evidence_len CHECK (char_length(evidence) <= 2000),
    CONSTRAINT disputes_resolution_len CHECK (char_length(resolution_note) <= 1000),
    CONSTRAINT disputes_decided_shape CHECK (
        (status = 'open' AND decided_at IS NULL AND decided_by IS NULL)
        OR (status IN ('resolved', 'rejected') AND decided_at IS NOT NULL AND decided_by IS NOT NULL)
    )
);

CREATE UNIQUE INDEX disputes_open_per_contribution
    ON disputes (contribution_id) WHERE status = 'open';
CREATE INDEX disputes_subscription_idx ON disputes (subscription_id, status);
CREATE INDEX disputes_raised_by_idx ON disputes (raised_by);

-- Dispute notification kinds.
ALTER TABLE notifications DROP CONSTRAINT notifications_type_check;
ALTER TABLE notifications
    ADD CONSTRAINT notifications_type_check
    CHECK (type IN ('invitation', 'payment_due', 'subscription_renewal',
                    'seat_assigned', 'subscription_expired',
                    'coterie_closed', 'system',
                    'join_requested', 'join_decided',
                    'payment_received',
                    'dispute_opened', 'dispute_decided'));
