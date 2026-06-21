-- Portal read projection tables (separate logical concern from orchestrator)

CREATE TABLE IF NOT EXISTS bank (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    bic                 TEXT NOT NULL UNIQUE,
    name                TEXT NOT NULL,
    country             CHAR(2) NOT NULL,
    status              TEXT NOT NULL DEFAULT 'APPLICATION',
    settlement_account  TEXT NOT NULL,
    notes               TEXT,
    onboarded_at        TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS bank_certificate (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    bank_id     UUID NOT NULL REFERENCES bank(id),
    subject     TEXT NOT NULL,
    fingerprint TEXT NOT NULL UNIQUE,
    not_before  TIMESTAMPTZ NOT NULL,
    not_after   TIMESTAMPTZ NOT NULL,
    status      TEXT NOT NULL DEFAULT 'ACTIVE',
    issued_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at  TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS transaction_view (
    payment_id             UUID PRIMARY KEY,
    end_to_end_id          TEXT NOT NULL,
    uetr                   UUID,
    instruction_id         TEXT,
    source_bic             TEXT NOT NULL,
    source_bank_name       TEXT NOT NULL,
    destination_bic        TEXT NOT NULL,
    destination_bank_name  TEXT NOT NULL,
    amount                 BIGINT NOT NULL,
    currency               CHAR(3) NOT NULL,
    status                 TEXT NOT NULL,
    abort_reason           TEXT,
    charge_bearer          TEXT,
    settlement_date        DATE,
    debtor_name            TEXT,
    creditor_name          TEXT,
    purpose_code           TEXT,
    remittance_info        TEXT,
    created_at             TIMESTAMPTZ NOT NULL,
    updated_at             TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS transaction_event_view (
    id           BIGSERIAL PRIMARY KEY,
    payment_id   UUID NOT NULL REFERENCES transaction_view(payment_id),
    event_type   TEXT NOT NULL,
    detail       JSONB,
    occurred_at  TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS audit_log (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_id    TEXT NOT NULL,
    actor_role  TEXT NOT NULL,
    action      TEXT NOT NULL,
    target_type TEXT NOT NULL,
    target_id   UUID NOT NULL,
    diff        JSONB,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS export_job (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id     TEXT NOT NULL,
    format       TEXT NOT NULL,
    filters      JSONB,
    status       TEXT NOT NULL DEFAULT 'PENDING',
    download_url TEXT,
    row_count    INTEGER,
    error        TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_txview_source ON transaction_view (source_bic, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_txview_dest   ON transaction_view (destination_bic, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_txview_status ON transaction_view (status);
CREATE INDEX IF NOT EXISTS idx_txevent_payment ON transaction_event_view (payment_id, occurred_at);
CREATE INDEX IF NOT EXISTS idx_audit_actor   ON audit_log (actor_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_action  ON audit_log (action, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_bank_cert_bank ON bank_certificate (bank_id);
CREATE INDEX IF NOT EXISTS idx_export_owner  ON export_job (owner_id, created_at DESC);
