-- Reverse 0018: abuse reports.

ALTER TABLE audit_logs DROP CONSTRAINT audit_logs_action_check;
ALTER TABLE audit_logs
    ADD CONSTRAINT audit_logs_action_check CHECK (action IN (
        'coterie_created', 'coterie_updated', 'member_removed', 'member_left',
        'seat_assigned', 'seat_released', 'seat_updated', 'subscription_updated',
        'contribution_updated', 'payment_recorded', 'period_closed'
    ));

DROP TABLE reports;
