-- 0001_init.down.sql — reverse of 0001_init.up.sql.

DROP INDEX IF EXISTS billing_periods_subscription_idx;
DROP INDEX IF EXISTS contributions_period_idx;
DROP INDEX IF EXISTS members_coterie_idx;
DROP INDEX IF EXISTS seats_subscription_idx;

DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS invitations;
DROP TABLE IF EXISTS contributions;
DROP TABLE IF EXISTS billing_periods;
DROP TABLE IF EXISTS seats;
DROP TABLE IF EXISTS members;
DROP TABLE IF EXISTS coteries;
DROP TABLE IF EXISTS subscriptions;
DROP TABLE IF EXISTS products;
DROP TABLE IF EXISTS providers;
DROP TABLE IF EXISTS users;
