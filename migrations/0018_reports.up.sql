-- Abuse reports (design D26): users flag publicly listed coteries and
-- the platform admin triages the inbox. Deciding a report never acts
-- on the coterie itself — the admin works through the existing
-- surfaces.

CREATE TABLE reports (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    coterie_id      uuid NOT NULL REFERENCES coteries,
    reporter_id     uuid NOT NULL REFERENCES users,
    reason          text NOT NULL CHECK (length(reason) BETWEEN 1 AND 1000),
    status          text NOT NULL DEFAULT 'open'
                    CHECK (status IN ('open', 'resolved', 'dismissed')),
    resolution_note text CHECK (length(resolution_note) <= 1000),
    decided_at      timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now()
);

-- One open report per user and coterie; decided ones may repeat.
CREATE UNIQUE INDEX reports_one_open_key
    ON reports (coterie_id, reporter_id) WHERE status = 'open';
-- The admin inbox: newest first within a status.
CREATE INDEX reports_inbox_idx ON reports (status, created_at DESC);

-- The admin's decision joins the audit trail.
ALTER TABLE audit_logs DROP CONSTRAINT audit_logs_action_check;
ALTER TABLE audit_logs
    ADD CONSTRAINT audit_logs_action_check CHECK (action IN (
        'coterie_created', 'coterie_updated', 'member_removed', 'member_left',
        'seat_assigned', 'seat_released', 'seat_updated', 'subscription_updated',
        'contribution_updated', 'payment_recorded', 'period_closed',
        'report_decided'
    ));
