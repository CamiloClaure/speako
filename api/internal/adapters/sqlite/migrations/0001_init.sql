-- 0001_init.sql - initial schema.
-- Money is INTEGER cents, times are INTEGER unix seconds (UTC), ids are TEXT (UUIDv7).

CREATE TABLE users (
    id       TEXT PRIMARY KEY,
    email    TEXT NOT NULL UNIQUE,
    name     TEXT NOT NULL,
    role     TEXT NOT NULL,
    timezone TEXT NOT NULL
);

CREATE TABLE sessions (
    token_hash TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id),
    expires_at INTEGER NOT NULL
);

CREATE TABLE login_tokens (
    token_hash TEXT PRIMARY KEY,
    email      TEXT NOT NULL,
    expires_at INTEGER NOT NULL,
    used_at    INTEGER
);

CREATE TABLE products (
    id            TEXT PRIMARY KEY,
    slug          TEXT NOT NULL UNIQUE,
    title         TEXT NOT NULL,
    kind          TEXT NOT NULL,
    price_cents   INTEGER NOT NULL,
    file_key      TEXT NOT NULL,
    credits       INTEGER NOT NULL,
    validity_days INTEGER NOT NULL,
    active        INTEGER NOT NULL DEFAULT 1
);

CREATE TABLE orders (
    id              TEXT PRIMARY KEY,
    user_id         TEXT NOT NULL REFERENCES users(id),
    product_id      TEXT NOT NULL REFERENCES products(id),
    status          TEXT NOT NULL,
    amount_cents    INTEGER NOT NULL,
    provider_ref    TEXT,
    idempotency_key TEXT UNIQUE,
    created_at      INTEGER NOT NULL,
    paid_at         INTEGER
);

CREATE TABLE payment_events (
    event_id     TEXT NOT NULL UNIQUE,
    order_id     TEXT NOT NULL REFERENCES orders(id),
    processed_at INTEGER NOT NULL
);

CREATE TABLE entitlements (
    user_id    TEXT NOT NULL REFERENCES users(id),
    product_id TEXT NOT NULL REFERENCES products(id),
    granted_at INTEGER NOT NULL,
    PRIMARY KEY (user_id, product_id)
);

CREATE TABLE credit_entries (
    user_id    TEXT NOT NULL REFERENCES users(id),
    delta      INTEGER NOT NULL,
    reason     TEXT NOT NULL,
    ref_id     TEXT NOT NULL,
    expires_at INTEGER NOT NULL,
    created_at INTEGER NOT NULL
);

CREATE TABLE availability_rules (
    weekday      INTEGER NOT NULL,
    start_minute INTEGER NOT NULL,
    end_minute   INTEGER NOT NULL,
    slot_min     INTEGER NOT NULL
);

CREATE TABLE availability_exceptions (
    date         INTEGER PRIMARY KEY,
    blocked      INTEGER NOT NULL DEFAULT 0,
    start_minute INTEGER NOT NULL DEFAULT 0,
    end_minute   INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE bookings (
    id         TEXT PRIMARY KEY,
    student_id TEXT NOT NULL REFERENCES users(id),
    teacher_id TEXT NOT NULL REFERENCES users(id),
    starts_at  INTEGER NOT NULL,
    ends_at    INTEGER NOT NULL,
    status     TEXT NOT NULL
);

CREATE TABLE leads (
    email              TEXT PRIMARY KEY,
    source             TEXT NOT NULL,
    consent_at         INTEGER NOT NULL,
    consent_ip         TEXT NOT NULL,
    confirmed_at       INTEGER,
    confirm_token_hash TEXT NOT NULL
);

CREATE UNIQUE INDEX bookings_slot ON bookings(teacher_id, starts_at) WHERE status = 'scheduled';
CREATE INDEX bookings_teacher_starts ON bookings(teacher_id, starts_at);
CREATE INDEX credit_entries_user_created ON credit_entries(user_id, created_at);
CREATE INDEX orders_user_created ON orders(user_id, created_at);
CREATE INDEX entitlements_user ON entitlements(user_id);
