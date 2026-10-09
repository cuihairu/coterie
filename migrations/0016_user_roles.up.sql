-- Platform admin role (design D26): the instance operator manages
-- abuse reports; nobody else gets elevated.

ALTER TABLE users ADD COLUMN role text NOT NULL DEFAULT 'user'
    CHECK (role IN ('user', 'admin'));
