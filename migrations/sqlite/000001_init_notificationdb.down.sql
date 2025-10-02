BEGIN TRANSACTION;

-- Drop indexes first (to avoid orphaned indexes)
DROP INDEX IF EXISTS idx_notifications_user_unviewed;
DROP INDEX IF EXISTS idx_notifications_user_unread;
DROP INDEX IF EXISTS idx_notifications_user_id;

-- Then drop the table
DROP TABLE IF EXISTS notifications;

COMMIT;
