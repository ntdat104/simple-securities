-- INSERT INTO notifications (uuid, user_id, type, title, body, read, viewed, created_by, updated_by)
-- VALUES
-- ('11111111-1111-1111-1111-111111111111', 1, 'info', 'Welcome!', 'This is your first notification.', 0, 0, 1, 1),
-- ('22222222-2222-2222-2222-222222222222', 1, 'alert', 'System Alert', 'Your system is up to date.', 0, 0, 1, 1),
-- ('33333333-3333-3333-3333-333333333333', 2, 'message', 'New Message', 'You have a new message.', 0, 0, 2, 2);

WITH RECURSIVE cnt(x) AS (
  SELECT 1
  UNION ALL
  SELECT x+1 FROM cnt WHERE x < 10000
)
INSERT INTO notifications (uuid, user_id, type, title, body, read, viewed, created_by, updated_by)
SELECT 
  lower(hex(randomblob(16))),                     -- random UUID-like value
  (abs(random()) % 100) + 1 AS user_id,           -- random user_id between 1–100
  CASE (abs(random()) % 3)                        -- type: cycle through info, alert, message
    WHEN 0 THEN 'info'
    WHEN 1 THEN 'alert'
    ELSE 'message'
  END,
  'Title ' || x,
  'Body content for notification #' || x,
  0,
  0,
  (abs(random()) % 10) + 1,                       -- created_by
  (abs(random()) % 10) + 1                        -- updated_by
FROM cnt;
