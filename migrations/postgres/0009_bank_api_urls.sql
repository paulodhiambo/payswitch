ALTER TABLE bank
    ADD COLUMN lookup_api_url TEXT,
    ADD COLUMN payment_api_url TEXT,
    ADD COLUMN status_check_api_url TEXT;

UPDATE bank
SET lookup_api_url = 'http://bank-service:8081',
    payment_api_url = 'http://bank-service:8081',
    status_check_api_url = 'http://bank-service:8081'
WHERE bic IN ('BANKUS33', 'BANKDEFF', 'BANKGB2L');
