-- D22: browser push endpoints a user registered for Web Push delivery.
-- p256dh/auth are the client key material the server needs verbatim to
-- encrypt payloads (RFC 8291); endpoint is globally unique because a
-- browser registers the same URL once.
CREATE TABLE push_subscriptions (
    id         uuid PRIMARY KEY,
    user_id    uuid NOT NULL REFERENCES users,
    endpoint   text NOT NULL,
    p256dh     text NOT NULL,
    auth       text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX push_subscriptions_endpoint_key ON push_subscriptions (endpoint);
CREATE INDEX push_subscriptions_user_idx ON push_subscriptions (user_id);
