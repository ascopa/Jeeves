CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TYPE ticket_status AS ENUM ('available', 'booked', 'sold');
CREATE TYPE ticket_type AS ENUM ('general', 'vip');

CREATE TABLE users (
    id    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name  TEXT NOT NULL,
    email TEXT NOT NULL UNIQUE
);

CREATE TABLE venues (
    id   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    size INT  NOT NULL CHECK (size > 0)
);

CREATE TABLE tickets (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    price_cents     BIGINT        NOT NULL CHECK (price_cents >= 0),
    seat_number     TEXT          NOT NULL,
    status          ticket_status NOT NULL DEFAULT 'available',
    type            ticket_type   NOT NULL DEFAULT 'general',
    booked_by       UUID REFERENCES users (id),
    booked_at       TIMESTAMPTZ,
    hold_expires_at TIMESTAMPTZ,
    sold_at         TIMESTAMPTZ,
    CHECK ((status = 'available') = (booked_by IS NULL))
);

CREATE INDEX tickets_holds_by_user_idx ON tickets (booked_by) WHERE status = 'booked';

CREATE TABLE events (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT        NOT NULL,
    date       TIMESTAMPTZ NOT NULL,
    venue_id   UUID        NOT NULL REFERENCES venues (id),
    ticket_ids UUID[]      NOT NULL DEFAULT '{}',
    queue_enabled      BOOLEAN NOT NULL DEFAULT false,
    admit_rate_per_sec INT     NOT NULL DEFAULT 50 CHECK (admit_rate_per_sec > 0),
    max_active_users   INT     NOT NULL DEFAULT 1000 CHECK (max_active_users > 0)
);

CREATE INDEX events_name_trgm_idx ON events USING gin (name gin_trgm_ops);
CREATE INDEX events_ticket_ids_idx ON events USING gin (ticket_ids);

CREATE TABLE payments (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ticket_id          UUID        NOT NULL REFERENCES tickets (id),
    user_id            UUID        NOT NULL REFERENCES users (id),
    amount_cents       BIGINT      NOT NULL CHECK (amount_cents >= 0),
    provider_charge_id TEXT,
    status             TEXT        NOT NULL CHECK (status IN ('pending', 'approved', 'declined', 'refunded', 'error')),
    idempotency_key    TEXT        NOT NULL UNIQUE,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

