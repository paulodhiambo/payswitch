-- Sync existing payments to transaction_view
INSERT INTO transaction_view (
    payment_id, end_to_end_id, uetr, instruction_id,
    source_bic, source_bank_name, destination_bic, destination_bank_name,
    amount, currency, status, abort_reason, charge_bearer,
    settlement_date, debtor_name, creditor_name, purpose_code,
    remittance_info, created_at, updated_at
)
SELECT 
    p.id, p.end_to_end_id, 
    CASE WHEN p.uetr IS NULL OR p.uetr = '' THEN NULL ELSE p.uetr::uuid END, 
    p.instr_id,
    p.source_bic, COALESCE(sb.name, p.source_bic), p.destination_bic, COALESCE(db.name, p.destination_bic),
    p.amount, p.currency, p.status, NULL, p.charge_bearer, p.sttlm_dt, p.debtor_name, p.creditor_name, 
    p.purpose_code, p.remittance_info, p.created_at, p.updated_at
FROM payment p
LEFT JOIN bank sb ON sb.bic = p.source_bic
LEFT JOIN bank db ON db.bic = p.destination_bic
ON CONFLICT (payment_id) DO UPDATE SET
    status = EXCLUDED.status,
    uetr = EXCLUDED.uetr,
    instruction_id = EXCLUDED.instruction_id,
    updated_at = EXCLUDED.updated_at,
    settlement_date = EXCLUDED.settlement_date;

-- Sync existing events from outbox (for timeline)
INSERT INTO transaction_event_view (payment_id, event_type, detail, occurred_at)
SELECT 
    (payload->>'payment_id')::uuid AS payment_id,
    topic AS event_type,
    payload AS detail,
    CASE 
        WHEN payload->>'timestamp' IS NOT NULL AND payload->>'timestamp' <> '' AND payload->>'timestamp' <> '0001-01-01T00:00:00Z' AND payload->>'timestamp' <> '0001-01-01T00:00:00.000Z' 
        THEN (payload->>'timestamp')::timestamptz
        ELSE created_at
    END AS occurred_at
FROM outbox
WHERE topic LIKE 'payment.%'
ON CONFLICT DO NOTHING;

-- Fix any timeline records with zero time
UPDATE transaction_event_view
SET occurred_at = o.created_at
FROM outbox o
WHERE occurred_at = '0001-01-01 00:00:00+00'::timestamptz
  AND o.topic = transaction_event_view.event_type
  AND (o.payload->>'payment_id')::uuid = transaction_event_view.payment_id;

-- Create sync function for payment -> transaction_view
CREATE OR REPLACE FUNCTION sync_payment_to_transaction_view()
RETURNS TRIGGER AS $$
DECLARE
    src_name TEXT;
    dst_name TEXT;
BEGIN
    SELECT COALESCE(name, NEW.source_bic) INTO src_name FROM bank WHERE bic = NEW.source_bic;
    SELECT COALESCE(name, NEW.destination_bic) INTO dst_name FROM bank WHERE bic = NEW.destination_bic;

    INSERT INTO transaction_view (
        payment_id, end_to_end_id, uetr, instruction_id,
        source_bic, source_bank_name, destination_bic, destination_bank_name,
        amount, currency, status, abort_reason, charge_bearer,
        settlement_date, debtor_name, creditor_name, purpose_code,
        remittance_info, created_at, updated_at
    ) VALUES (
        NEW.id, NEW.end_to_end_id, 
        CASE WHEN NEW.uetr IS NULL OR NEW.uetr = '' THEN NULL ELSE NEW.uetr::uuid END, 
        NEW.instr_id,
        NEW.source_bic, COALESCE(src_name, NEW.source_bic), NEW.destination_bic, COALESCE(dst_name, NEW.destination_bic),
        NEW.amount, NEW.currency, NEW.status, NULL, NEW.charge_bearer, NEW.sttlm_dt, NEW.debtor_name, NEW.creditor_name, 
        NEW.purpose_code, NEW.remittance_info, NEW.created_at, NEW.updated_at
    )
    ON CONFLICT (payment_id) DO UPDATE SET
        status = EXCLUDED.status,
        uetr = EXCLUDED.uetr,
        instruction_id = EXCLUDED.instruction_id,
        updated_at = EXCLUDED.updated_at,
        settlement_date = EXCLUDED.settlement_date;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE TRIGGER trigger_sync_payment
AFTER INSERT OR UPDATE ON payment
FOR EACH ROW
EXECUTE FUNCTION sync_payment_to_transaction_view();

-- Create sync function for outbox -> transaction_event_view
CREATE OR REPLACE FUNCTION sync_outbox_to_transaction_event_view()
RETURNS TRIGGER AS $$
DECLARE
    pid_str TEXT;
    ts_str TEXT;
    occurred_dt TIMESTAMPTZ;
BEGIN
    IF NEW.topic LIKE 'payment.%' THEN
        pid_str := NEW.payload->>'payment_id';
        IF pid_str IS NOT NULL AND pid_str <> '' THEN
            ts_str := NEW.payload->>'timestamp';
            IF ts_str IS NOT NULL AND ts_str <> '' AND ts_str <> '0001-01-01T00:00:00Z' AND ts_str <> '0001-01-01T00:00:00.000Z' THEN
                occurred_dt := ts_str::timestamptz;
            ELSE
                occurred_dt := NEW.created_at;
            END IF;

            INSERT INTO transaction_event_view (payment_id, event_type, detail, occurred_at)
            VALUES (
                pid_str::uuid,
                NEW.topic,
                NEW.payload,
                occurred_dt
            );
        END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE TRIGGER trigger_sync_outbox
AFTER INSERT ON outbox
FOR EACH ROW
EXECUTE FUNCTION sync_outbox_to_transaction_event_view();
