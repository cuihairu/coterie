-- 0002_auth.up.sql — platform authentication (M2a; design §6.1, ADR D8).
-- Email + bcrypt password sign-in. Sessions carry opaque bearer tokens;
-- only the SHA-256 hash is stored, so a database leak never reveals a
-- usable token. password_hash stays nullable so future OAuth / Passkey
-- identities can attach to the same user row.

ALTER TABLE users ADD COLUMN password_hash text;

CREATE TABLE sessions (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid NOT NULL REFERENCES users,
    token_hash text NOT NULL,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX sessions_token_hash_key ON sessions (token_hash);
CREATE INDEX sessions_user_idx ON sessions (user_id);
