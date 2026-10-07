-- Automation (Phase 3): subscriptions may opt into scheduler-driven
-- billing rollover. The first period always stays manual (price and
-- start date are owner decisions); when enabled, the scheduler opens
-- the next period and generates equal shares once the current one ends.
ALTER TABLE subscriptions ADD COLUMN auto_billing boolean NOT NULL DEFAULT false;
