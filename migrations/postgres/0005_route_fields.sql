ALTER TABLE payment
    ADD COLUMN route_fee         BIGINT,
    ADD COLUMN route_estimated_ms INT;
