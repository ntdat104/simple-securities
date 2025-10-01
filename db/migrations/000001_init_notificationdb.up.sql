START TRANSACTION;

-- Create schema if not exists
CREATE SCHEMA IF NOT EXISTS "notification_service";

-- Create notifications table
CREATE TABLE "notification_service".notifications (
    id BIGSERIAL PRIMARY KEY,
    uuid VARCHAR(36) NOT NULL UNIQUE,
    user_id BIGINT NOT NULL,
    type VARCHAR(50) NOT NULL,
    title VARCHAR(255) NOT NULL,
    body TEXT NOT NULL,
    read BOOLEAN NOT NULL DEFAULT FALSE,
    read_at TIMESTAMP WITH TIME ZONE NULL,
    viewed BOOLEAN NOT NULL DEFAULT FALSE,
    viewed_at TIMESTAMP WITH TIME ZONE NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    created_by BIGINT NOT NULL,
    updated_by BIGINT NOT NULL
);

-- Index to quickly fetch all notifications for a user
CREATE INDEX idx_notifications_user_id 
    ON "notification_service".notifications(user_id);

-- Optional index if you often query unread notifications
CREATE INDEX idx_notifications_user_unread 
    ON "notification_service".notifications(user_id, read) 
    WHERE read = false;

-- Optional index if you often query unviewed notifications
CREATE INDEX idx_notifications_user_unviewed 
    ON "notification_service".notifications(user_id, viewed) 
    WHERE viewed = false;

COMMIT;
