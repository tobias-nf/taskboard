-- Remove blocked_by, result, and failure_reason from tasks.
-- Blocking context and results should be documented via comments/activity.
ALTER TABLE tasks DROP COLUMN IF EXISTS blocked_by;
ALTER TABLE tasks DROP COLUMN IF EXISTS result;
ALTER TABLE tasks DROP COLUMN IF EXISTS failure_reason;
