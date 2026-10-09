DROP INDEX IF EXISTS payments_external_ref_key;

ALTER TABLE payments DROP CONSTRAINT IF EXISTS payments_method_check;
ALTER TABLE payments
    ADD CONSTRAINT payments_method_check
    CHECK (method IN ('manual', 'sandbox'));

-- No pending rows can survive the status roll-back; drop them first.
DELETE FROM payments WHERE status <> 'succeeded';

ALTER TABLE payments DROP CONSTRAINT IF EXISTS payments_status_check;
ALTER TABLE payments
    ADD CONSTRAINT payments_status_check
    CHECK (status IN ('succeeded'));

UPDATE payments SET paid_at = created_at WHERE paid_at IS NULL;
ALTER TABLE payments ALTER COLUMN paid_at SET NOT NULL;
