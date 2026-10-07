-- 0001_init.up.sql — Phase 1 core schema.
-- Domain model and invariants: docs/design.md §1.
-- Conventions: uuid PKs (gen_random_uuid, PG13+), timestamptz, text + CHECK
-- instead of native enums (simpler migrations), JSONB for provider-specific
-- metadata. Invariants that span tables (e.g. seat occupier must be an active
-- member of the bound coterie, contribution currency = subscription currency)
-- are enforced in the application layer, as noted inline.

CREATE TABLE users (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    username   text NOT NULL,
    email      text NOT NULL,
    avatar_url text,
    locale     text NOT NULL DEFAULT 'en',
    timezone   text NOT NULL DEFAULT 'UTC',
    status     text NOT NULL DEFAULT 'active',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX users_username_key ON users (lower(username));
CREATE UNIQUE INDEX users_email_key ON users (lower(email));

CREATE TABLE providers (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    slug       text NOT NULL,
    name       text NOT NULL,
    category   text NOT NULL,
    metadata   jsonb NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX providers_slug_key ON providers (lower(slug));

CREATE TABLE products (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_id uuid NOT NULL REFERENCES providers,
    name        text NOT NULL,
    tier        text,
    metadata    jsonb NOT NULL DEFAULT '{}',
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (provider_id, name)
);

CREATE TABLE subscriptions (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id     uuid NOT NULL REFERENCES products,
    owner_user_id  uuid NOT NULL REFERENCES users,
    billing_cycle  text NOT NULL CHECK (billing_cycle IN ('monthly', 'yearly', 'custom')),
    price          numeric(12,2) NOT NULL CHECK (price >= 0),
    currency       char(3) NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    start_date     date NOT NULL,
    renewal_date   date,
    status         text NOT NULL DEFAULT 'active',
    max_seats      int NOT NULL CHECK (max_seats > 0),
    max_members    int,
    sharing_policy jsonb NOT NULL DEFAULT '{}',
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE coteries (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    subscription_id uuid NOT NULL REFERENCES subscriptions,
    name            text NOT NULL,
    status          text NOT NULL DEFAULT 'draft'
                    CHECK (status IN ('draft', 'open', 'active', 'paused', 'closed')),
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);

-- Invariant 1 (D1): strictly one coterie per subscription.
CREATE UNIQUE INDEX coteries_subscription_id_key ON coteries (subscription_id);

CREATE TABLE members (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    coterie_id uuid NOT NULL REFERENCES coteries,
    user_id    uuid NOT NULL REFERENCES users,
    role       text NOT NULL DEFAULT 'member'
               CHECK (role IN ('owner', 'admin', 'member')),
    status     text NOT NULL DEFAULT 'active',
    joined_at  timestamptz NOT NULL DEFAULT now(),
    left_at    timestamptz
);

-- Same user holds at most one active membership per coterie;
-- left rows are kept for settlement traceability (soft leave).
CREATE UNIQUE INDEX members_coterie_user_active_key
    ON members (coterie_id, user_id) WHERE left_at IS NULL;

CREATE TABLE seats (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    subscription_id uuid NOT NULL REFERENCES subscriptions,
    member_id       uuid REFERENCES members,
    label           text NOT NULL,
    status          text NOT NULL DEFAULT 'free'
                    CHECK (status IN ('free', 'occupied', 'disabled')),
    metadata        jsonb NOT NULL DEFAULT '{}',
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (subscription_id, label),
    -- A seat is occupied if and only if it has an assignee (design §1.6).
    CHECK ((status = 'occupied') = (member_id IS NOT NULL))
);

CREATE TABLE billing_periods (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    subscription_id uuid NOT NULL REFERENCES subscriptions,
    start_date      date NOT NULL,
    end_date        date NOT NULL,
    status          text NOT NULL DEFAULT 'open',
    UNIQUE (subscription_id, start_date),
    CHECK (end_date > start_date)
);

CREATE TABLE contributions (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    billing_period_id uuid NOT NULL REFERENCES billing_periods,
    member_id         uuid NOT NULL REFERENCES members,
    amount            numeric(12,2) NOT NULL CHECK (amount >= 0),
    currency          char(3) NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    status            text NOT NULL DEFAULT 'pending'
                      CHECK (status IN ('pending', 'paid', 'waived', 'cancelled')),
    paid_at           timestamptz,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    -- One contribution per member per billing period (invariant 6).
    UNIQUE (billing_period_id, member_id)
);
-- Invariant 5 (D6): contributions.currency must equal the currency of the
-- subscription that owns the billing period — enforced in the application
-- layer alongside period ownership checks.

CREATE TABLE invitations (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    coterie_id  uuid NOT NULL REFERENCES coteries,
    token       text NOT NULL,
    role        text NOT NULL DEFAULT 'member' CHECK (role IN ('admin', 'member')),
    expire_at   timestamptz NOT NULL,
    created_by  uuid NOT NULL REFERENCES users,
    accepted_at timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX invitations_token_key ON invitations (token);

CREATE TABLE audit_logs (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    actor_user_id uuid REFERENCES users,
    action        text NOT NULL,
    entity_type   text NOT NULL,
    entity_id     uuid,
    before_state  jsonb,
    after_state   jsonb,
    created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX audit_logs_entity_idx ON audit_logs (entity_type, entity_id);

CREATE INDEX seats_subscription_idx ON seats (subscription_id);
CREATE INDEX members_coterie_idx ON members (coterie_id);
CREATE INDEX contributions_period_idx ON contributions (billing_period_id);
CREATE INDEX billing_periods_subscription_idx ON billing_periods (subscription_id);
