ALTER TABLE bank
    ADD COLUMN api_base_url TEXT,
    ADD COLUMN api_key      TEXT,
    ADD COLUMN api_enabled  BOOLEAN NOT NULL DEFAULT false;
