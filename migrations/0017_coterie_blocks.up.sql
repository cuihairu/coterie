-- Coterie-level blocks (design D26): the owner's abuse-prevention
-- primitive. A blocked user is refused on every admission path; an
-- existing member is not removed automatically.

CREATE TABLE coterie_blocks (
    coterie_id  uuid        NOT NULL REFERENCES coteries,
    user_id     uuid        NOT NULL REFERENCES users,
    created_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (coterie_id, user_id)
);
