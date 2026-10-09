-- Revert the payment gate (design D25).

DROP TABLE admission_charges;

ALTER TABLE coteries DROP COLUMN payment_gate;

-- Requests held for payment fall back to plain pending so the status
-- CHECK can narrow again; a charge in flight is unrecoverable here
-- (the table above is being dropped), the operator refunds out-of-band.
UPDATE join_requests SET status = 'pending' WHERE status = 'awaiting_payment';

ALTER TABLE join_requests DROP CONSTRAINT join_requests_status_check;
ALTER TABLE join_requests
    ADD CONSTRAINT join_requests_status_check
    CHECK (status IN ('pending', 'accepted', 'declined', 'cancelled'));

ALTER TABLE notifications DROP CONSTRAINT notifications_type_check;
ALTER TABLE notifications
    ADD CONSTRAINT notifications_type_check
    CHECK (type IN ('invitation', 'payment_due', 'subscription_renewal',
                    'seat_assigned', 'subscription_expired',
                    'coterie_closed', 'system',
                    'join_requested', 'join_decided',
                    'payment_received',
                    'dispute_opened', 'dispute_decided'));
