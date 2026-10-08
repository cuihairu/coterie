ALTER TABLE subscriptions DROP CONSTRAINT IF EXISTS subscriptions_cycle_days_check;
ALTER TABLE subscriptions DROP COLUMN IF EXISTS cycle_days;
