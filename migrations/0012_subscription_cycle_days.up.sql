-- D20: custom billing cycles carry their length in days. The row-level
-- CHECK forces the pairing both ways: custom requires cycle_days in
-- 1..365, every other cycle requires it absent.
ALTER TABLE subscriptions
    ADD COLUMN cycle_days integer;

ALTER TABLE subscriptions
    ADD CONSTRAINT subscriptions_cycle_days_check CHECK (
        (billing_cycle = 'custom' AND cycle_days BETWEEN 1 AND 365)
        OR (billing_cycle <> 'custom' AND cycle_days IS NULL)
    );
