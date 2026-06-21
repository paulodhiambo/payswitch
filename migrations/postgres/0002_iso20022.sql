ALTER TABLE payment
    ADD COLUMN uetr            TEXT,
    ADD COLUMN instr_id        TEXT,
    ADD COLUMN charge_bearer   CHAR(4),
    ADD COLUMN sttlm_dt        DATE,
    ADD COLUMN debtor_name     TEXT,
    ADD COLUMN creditor_name   TEXT,
    ADD COLUMN purpose_code    CHAR(4),
    ADD COLUMN remittance_info TEXT;
