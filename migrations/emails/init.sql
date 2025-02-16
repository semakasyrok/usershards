CREATE TABLE IF NOT EXISTS emails (
    user_id BIGINT NOT NULL UNIQUE,
    email TEXT NOT NULL UNIQUE,
    created_at timestamp NOT NULL,
    updated_at timestamp NOT NULL
);
