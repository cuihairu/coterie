-- D24: real payment channels confirm asynchronously. A charge first
-- lands as pending (its channel intent id in external_ref, paid_at
-- null); the channel's webhook later flips it to succeeded or failed.
-- The method list gains stripe; the status list gains pending/failed.
ALTER TABLE payments ALTER COLUMN paid_at DROP NOT NULL;

ALTER TABLE payments DROP CONSTRAINT payments_status_check;
ALTER TABLE payments
    ADD CONSTRAINT payments_status_check
    CHECK (status IN ('pending', 'succeeded', 'failed'));

ALTER TABLE payments DROP CONSTRAINT payments_method_check;
ALTER TABLE payments
    ADD CONSTRAINT payments_method_check
    CHECK (method IN ('manual', 'sandbox', 'stripe'));

-- A channel's intent id identifies at most one payment (webhook lookup).
CREATE UNIQUE INDEX payments_external_ref_key
    ON payments (external_ref) WHERE external_ref IS NOT NULL;
