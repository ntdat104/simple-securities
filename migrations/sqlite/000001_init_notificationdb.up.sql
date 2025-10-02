BEGIN TRANSACTION;

-- Create notifications table
CREATE TABLE IF NOT EXISTS notifications (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    uuid TEXT NOT NULL UNIQUE,
    user_id INTEGER NOT NULL,
    type TEXT NOT NULL,
    title TEXT NOT NULL,
    body TEXT NOT NULL,
    read BOOLEAN NOT NULL DEFAULT 0,
    read_at DATETIME NULL,
    viewed BOOLEAN NOT NULL DEFAULT 0,
    viewed_at DATETIME NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by INTEGER NOT NULL,
    updated_by INTEGER NOT NULL
);

-- Index to quickly fetch all notifications for a user
CREATE INDEX IF NOT EXISTS idx_notifications_user_id 
    ON notifications(user_id);

-- Optional index if you often query unread notifications
CREATE INDEX IF NOT EXISTS idx_notifications_user_unread 
    ON notifications(user_id) WHERE read = 0;

-- Optional index if you often query unviewed notifications
CREATE INDEX IF NOT EXISTS idx_notifications_user_unviewed 
    ON notifications(user_id) WHERE viewed = 0;

COMMIT;
