CREATE TABLE IF NOT EXISTS transaction (
    ID uuid PRIMARY KEY,
    from_id bigint,
    to_id bigint,
    amount int,
    created_at timestamp NOT NULL
)