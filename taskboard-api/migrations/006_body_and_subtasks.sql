-- Taskboard — Merge playbook/notes into description + subtask visibility inheritance
-- Idempotent migration: safe to run on every startup

-- ============================================================
-- 1. Merge playbook and notes into description, then drop old columns
-- ============================================================

-- Only run if playbook column still exists (first run)
DO $$ BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'tasks' AND column_name = 'playbook'
    ) THEN
        -- Merge playbook and notes into description
        UPDATE tasks SET description = CONCAT_WS(E'\n\n',
            CASE WHEN description IS NOT NULL AND description != '' THEN description END,
            CASE WHEN playbook IS NOT NULL AND playbook != ''
                 THEN '## Playbook' || E'\n' || playbook END,
            CASE WHEN notes IS NOT NULL AND notes != ''
                 THEN '## Notes' || E'\n' || notes END
        ) WHERE playbook IS NOT NULL OR notes IS NOT NULL;

        ALTER TABLE tasks DROP COLUMN playbook;
    END IF;
END $$;

DO $$ BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'tasks' AND column_name = 'notes'
    ) THEN
        ALTER TABLE tasks DROP COLUMN notes;
    END IF;
END $$;

-- ============================================================
-- 2. Sync subtask visibility with parent on existing data
-- ============================================================

UPDATE tasks c
SET visibility = p.visibility
FROM tasks p
WHERE c.parent_id = p.id
  AND c.visibility != p.visibility;
