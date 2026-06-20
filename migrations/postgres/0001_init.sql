CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE payment (
    id              UUID PRIMARY KEY,
    end_to_end_id   TEXT NOT NULL,
    source_bic      TEXT NOT NULL,
    destination_bic TEXT NOT NULL,
    source_account  TEXT NOT NULL,
    dest_account    TEXT NOT NULL,
    amount          BIGINT NOT NULL,
    currency        CHAR(3) NOT NULL,
    status          TEXT NOT NULL,
    quote_id        TEXT,
    reserved_at     TIMESTAMPTZ,
    expires_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (source_bic, end_to_end_id)
);

CREATE INDEX idx_payment_expires_at ON payment (expires_at) WHERE status = 'RESERVED';

CREATE TABLE outbox (
    id           BIGSERIAL PRIMARY KEY,
    topic        TEXT NOT NULL,
    msg_key      TEXT NOT NULL,
    payload      JSONB NOT NULL,
    status       TEXT NOT NULL DEFAULT 'PENDING',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ
);

CREATE INDEX idx_outbox_pending ON outbox (id) WHERE status = 'PENDING';
