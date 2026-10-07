ALTER TABLE notifications DROP CONSTRAINT notifications_type_check;
ALTER TABLE notifications
    ADD CONSTRAINT notifications_type_check
    CHECK (type IN ('invitation', 'payment_due', 'subscription_renewal',
                    'seat_assigned', 'subscription_expired',
                    'coterie_closed', 'system',
                    'join_requested', 'join_decided',
                    'payment_received'));

DROP TABLE disputes;
