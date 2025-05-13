CREATE TABLE IF NOT EXISTS idempotence (
    id uuid NOT NULL,
    type VARCHAR NOT NULL,
    created_at timestamp NOT NULL,
    PRIMARY KEY (id, type)
);