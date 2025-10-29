BEGIN TRANSACTION;

-- Create users table
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    uuid TEXT NOT NULL UNIQUE,
    email TEXT NOT NULL UNIQUE,
    hashed_password TEXT NOT NULL,
    refresh_token TEXT NULL,
    last_login_at DATETIME NULL,
    -- Status is expected to be a string or integer based on the Go enum. 
    -- Assuming a TEXT representation for flexibility, matching Go's string conversion of enums.
    status TEXT NOT NULL, 
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by INTEGER NOT NULL,
    updated_by INTEGER NOT NULL
);

-- Index to quickly search or join by UUID
CREATE INDEX IF NOT EXISTS idx_users_uuid 
    ON users(uuid);

-- Index to quickly search by email (already enforced by UNIQUE, but useful for lookups)
-- The UNIQUE constraint on email is sufficient, but a separate index might still be helpful for specific query patterns.
-- We'll rely on the UNIQUE constraint for primary email lookup optimization.

COMMIT;