-- Drop triggers
DROP TRIGGER IF EXISTS update_notifications_updated_at ON notifications;
DROP TRIGGER IF EXISTS update_templates_updated_at ON templates;

-- Drop function
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop indexes
DROP INDEX IF EXISTS idx_notifications_status;
DROP INDEX IF EXISTS idx_notifications_channel;
DROP INDEX IF EXISTS idx_notifications_batch_id;
DROP INDEX IF EXISTS idx_notifications_scheduled_at;
DROP INDEX IF EXISTS idx_notifications_created_at;
DROP INDEX IF EXISTS idx_notifications_idempotency_key;
DROP INDEX IF EXISTS idx_templates_name;
DROP INDEX IF EXISTS idx_templates_channel;

-- Drop tables
DROP TABLE IF EXISTS templates;
DROP TABLE IF EXISTS notifications;
