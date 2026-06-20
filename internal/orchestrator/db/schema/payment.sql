CREATE TABLE payment (
    id              TEXT PRIMARY KEY,
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
