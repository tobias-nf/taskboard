-- Cleanup for removed context feature
-- Safe to run on every startup.

DROP INDEX IF EXISTS idx_tasks_schema;

DO $$ BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'tasks' AND column_name = 'context_schema'
    ) THEN
        ALTER TABLE tasks DROP COLUMN context_schema;
    END IF;
END $$;

DO $$ BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'tasks' AND column_name = 'context_version'
    ) THEN
        ALTER TABLE tasks DROP COLUMN context_version;
    END IF;
END $$;

DO $$ BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'tasks' AND column_name = 'context_data'
    ) THEN
        ALTER TABLE tasks DROP COLUMN context_data;
    END IF;
END $$;

DROP TABLE IF EXISTS contexts;
