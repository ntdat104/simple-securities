START TRANSACTION;

-- Drop indexes first (to avoid orphaned indexes)
DROP INDEX IF EXISTS "notification_service".idx_notifications_user_unviewed;
DROP INDEX IF EXISTS "notification_service".idx_notifications_user_unread;
DROP INDEX IF EXISTS "notification_service".idx_notifications_user_id;

-- Then drop the table
DROP TABLE IF EXISTS "notification_service".notifications;

-- Optionally drop schema (uncomment if you want full cleanup)
-- DROP SCHEMA IF EXISTS "notification_service" CASCADE;

COMMIT;
