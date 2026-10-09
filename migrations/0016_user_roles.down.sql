-- Revert the platform admin role (design D26).

ALTER TABLE users DROP COLUMN role;
