BEGIN TRANSACTION;

CREATE TABLE IF NOT EXISTS user_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    uuid TEXT NOT NULL UNIQUE,
    email TEXT NOT NULL UNIQUE,
    hashed_password TEXT NOT NULL,
    refresh_token TEXT NULL,
    last_login_at DATETIME NULL,
    status TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by INTEGER NOT NULL,
    updated_by INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_user_history_uuid
    ON user_history(uuid);

CREATE INDEX IF NOT EXISTS idx_user_history_created_at
    ON user_history(created_at);

CREATE INDEX IF NOT EXISTS idx_user_history_last_login_at
    ON user_history(last_login_at);

CREATE INDEX IF NOT EXISTS idx_user_history_status
    ON user_history(status);

COMMIT;
