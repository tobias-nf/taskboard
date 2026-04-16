-- Taskboard — Add 'draft' status for approval workflows
-- Tasks in 'draft' are not visible to their assignee until approved.

-- 1. Widen the status CHECK constraint to include 'draft'
ALTER TABLE tasks DROP CONSTRAINT IF EXISTS tasks_status_check;
ALTER TABLE tasks ADD CONSTRAINT tasks_status_check
    CHECK (status IN (
        'draft', 'pending', 'accepted', 'in_progress', 'blocked',
        'review', 'completed', 'failed', 'cancelled'
    ));
