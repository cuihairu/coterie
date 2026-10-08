DROP INDEX IF EXISTS audit_logs_coterie_idx;
DROP INDEX IF EXISTS audit_logs_subscription_idx;

ALTER TABLE audit_logs DROP CONSTRAINT IF EXISTS audit_logs_action_check;

ALTER TABLE audit_logs
    DROP COLUMN IF EXISTS coterie_id,
    DROP COLUMN IF EXISTS subscription_id;
