-- Seed bank API configs for local development.
-- Points each bank's API at the mock bank-service container.

UPDATE bank SET
    api_base_url = 'http://bank-service:8081',
    api_enabled  = true
WHERE bic IN ('BANKUS33', 'BANKDEFF', 'BANKGB2L');
