-- Taskboard — Replace workspaces with public/private visibility model
-- Adds: visibility, tags, task_tags, task_owed_to, task_mentions
-- Drops: workspaces, workspace_members, tasks.workspace_id
-- Idempotent migration: safe to run on every startup

-- ============================================================
-- 1. Add visibility column to tasks
-- ============================================================

DO $$ BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'tasks' AND column_name = 'visibility'
    ) THEN
        ALTER TABLE tasks ADD COLUMN visibility TEXT NOT NULL DEFAULT 'public'
            CHECK (visibility IN ('public', 'private'));
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_tasks_visibility ON tasks(visibility);

-- ============================================================
-- 2. New tables: tags
-- ============================================================

CREATE TABLE IF NOT EXISTS tags (
    id          BIGSERIAL PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    color       TEXT,
    created_by  TEXT NOT NULL REFERENCES agents(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS task_tags (
    task_id     TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    tag_id      BIGINT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    added_by    TEXT NOT NULL REFERENCES agents(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (task_id, tag_id)
);

CREATE INDEX IF NOT EXISTS idx_task_tags_tag ON task_tags(tag_id);

-- ============================================================
-- 3. New tables: owed_to (stakeholders)
-- ============================================================

CREATE TABLE IF NOT EXISTS task_owed_to (
    task_id     TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    agent_id    TEXT NOT NULL REFERENCES agents(id),
    added_by    TEXT NOT NULL REFERENCES agents(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (task_id, agent_id)
);

CREATE INDEX IF NOT EXISTS idx_task_owed_to_agent ON task_owed_to(agent_id);

-- ============================================================
-- 4. New tables: mentions (explicit access grants)
-- ============================================================

CREATE TABLE IF NOT EXISTS task_mentions (
    task_id     TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    agent_id    TEXT NOT NULL REFERENCES agents(id),
    added_by    TEXT NOT NULL REFERENCES agents(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (task_id, agent_id)
);

CREATE INDEX IF NOT EXISTS idx_task_mentions_agent ON task_mentions(agent_id);

-- ============================================================
-- 5. Drop workspace_id from tasks
-- ============================================================

DROP INDEX IF EXISTS idx_tasks_workspace;

DO $$ BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'tasks' AND column_name = 'workspace_id'
    ) THEN
        ALTER TABLE tasks DROP COLUMN workspace_id;
    END IF;
END $$;

-- ============================================================
-- 6. Drop workspace tables
-- ============================================================

DROP INDEX IF EXISTS idx_workspace_members_agent;
DROP TABLE IF EXISTS workspace_members CASCADE;
DROP TABLE IF EXISTS workspaces CASCADE;
