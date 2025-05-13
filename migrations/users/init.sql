CREATE TABLE IF NOT EXISTS users (
    id BIGINT PRIMARY KEY,
    phone_number TEXT NOT NULL UNIQUE,
    email TEXT NOT NULL UNIQUE,
    balance bigint,
    created_at timestamp NOT NULL,
    updated_at timestamp NOT NULL,
    is_blocked bool NOT NULL DEFAULT false
);
