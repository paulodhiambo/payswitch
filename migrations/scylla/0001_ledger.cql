CREATE KEYSPACE IF NOT EXISTS switch
WITH replication = {'class': 'SimpleStrategy', 'replication_factor': 1};

CREATE TABLE IF NOT EXISTS switch.ledger (
    participant_id  text,
    date_bucket     text,
    payment_id      uuid,
    event_type      text,
    payload         text,
    created_at      timestamp,
    PRIMARY KEY ((participant_id, date_bucket), payment_id, created_at)
) WITH CLUSTERING ORDER BY (payment_id ASC, created_at ASC);

CREATE TABLE IF NOT EXISTS switch.payment_by_status (
    status      text,
    payment_id  uuid,
    updated_at  timestamp,
    PRIMARY KEY (status, updated_at, payment_id)
) WITH CLUSTERING ORDER BY (updated_at DESC, payment_id ASC);
