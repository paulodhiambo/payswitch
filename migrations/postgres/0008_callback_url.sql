ALTER TABLE bank ADD COLUMN callback_url TEXT;

UPDATE bank SET callback_url = 'http://bank-service:8081/payments/callback' WHERE bic IN ('BANKUS33', 'BANKDEFF', 'BANKGB2L');
