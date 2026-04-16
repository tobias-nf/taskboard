-- Taskboard — Agent & Task Management Platform
-- PostgreSQL schema (idempotent — safe to run on every startup)

-- ============================================================
-- Agents
-- ============================================================

CREATE TABLE IF NOT EXISTS agents (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    type            TEXT NOT NULL CHECK (type IN ('service', 'personal', 'admin')),
    description     TEXT,

    -- Auth
    api_key_hash    TEXT NOT NULL,
    api_key_prefix  TEXT NOT NULL,

    -- Personal agent metadata (null for service/admin)
    email           TEXT,
    slack_id        TEXT,
    title           TEXT,

    -- Preferences
    preferred_tool  TEXT,
    tool_config     JSONB,
    domains         TEXT[] DEFAULT '{}',

    -- Status
    active          BOOLEAN DEFAULT false,
    approved_by     TEXT REFERENCES agents(id),

    last_seen_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_agents_type ON agents(type);
CREATE INDEX IF NOT EXISTS idx_agents_prefix ON agents(api_key_prefix);

-- ============================================================
-- Tasks
-- ============================================================

CREATE SEQUENCE IF NOT EXISTS task_seq;

CREATE TABLE IF NOT EXISTS tasks (
    id              TEXT PRIMARY KEY DEFAULT 'T-' || to_char(now(), 'YYYY') || '-'
                    || lpad(nextval('task_seq')::text, 7, '0'),

    title           TEXT NOT NULL,
    description     TEXT,

    created_by      TEXT NOT NULL REFERENCES agents(id),
    assigned_to     TEXT REFERENCES agents(id),
    visibility      TEXT NOT NULL DEFAULT 'public'
                    CHECK (visibility IN ('public', 'private')),

    status          TEXT NOT NULL DEFAULT 'pending'
                    CHECK (status IN (
                        'pending', 'accepted', 'in_progress', 'blocked',
                        'review', 'completed', 'failed', 'cancelled'
                    )),
    blocked_by      TEXT,

    priority        TEXT DEFAULT 'standard'
                    CHECK (priority IN ('low', 'standard', 'urgent', 'emergency')),
    deadline        TIMESTAMPTZ,

    result          JSONB,
    failure_reason  TEXT,

    parent_id       TEXT REFERENCES tasks(id),

    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    accepted_at     TIMESTAMPTZ,
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_tasks_assigned ON tasks(assigned_to, status);
CREATE INDEX IF NOT EXISTS idx_tasks_created ON tasks(created_by, status);
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_parent ON tasks(parent_id);
CREATE INDEX IF NOT EXISTS idx_tasks_visibility ON tasks(visibility);
CREATE INDEX IF NOT EXISTS idx_tasks_deadline ON tasks(deadline)
    WHERE status NOT IN ('completed', 'failed', 'cancelled');

-- ============================================================
-- Tags (organizational labels, no access control)
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
-- Owed-to (task stakeholders)
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
-- Mentions (explicit access grants for private tasks)
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
-- References
-- ============================================================

CREATE TABLE IF NOT EXISTS task_references (
    id              BIGSERIAL PRIMARY KEY,
    task_id         TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,

    type            TEXT NOT NULL
                    CHECK (type IN ('origin', 'related', 'blocks', 'depends_on', 'output')),

    source          TEXT NOT NULL,
    external_id     TEXT,
    url             TEXT,
    title           TEXT NOT NULL,
    metadata        JSONB,

    created_by      TEXT NOT NULL REFERENCES agents(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_refs_task ON task_references(task_id);
CREATE INDEX IF NOT EXISTS idx_refs_source ON task_references(source, external_id);
CREATE INDEX IF NOT EXISTS idx_refs_type ON task_references(task_id, type);

-- ============================================================
-- Attachments
-- ============================================================

CREATE TABLE IF NOT EXISTS task_attachments (
    id              BIGSERIAL PRIMARY KEY,
    task_id         TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,

    filename        TEXT NOT NULL,
    mime_type       TEXT,
    size_bytes      BIGINT,
    sha256          TEXT,

    storage_key     TEXT NOT NULL,
    storage_url     TEXT,

    label           TEXT,
    uploaded_by     TEXT NOT NULL REFERENCES agents(id),

    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_attachments_task ON task_attachments(task_id);

-- ============================================================
-- Activity (append-only)
-- ============================================================

CREATE TABLE IF NOT EXISTS task_activity (
    id              BIGSERIAL PRIMARY KEY,
    task_id         TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,

    type            TEXT NOT NULL
                    CHECK (type IN (
                        'created', 'status_changed', 'assigned', 'reassigned',
                        'commented', 'reference_added', 'reference_removed',
                        'playbook_step_completed', 'playbook_step_failed',
                        'escalated', 'blocked', 'unblocked',
                        'priority_changed', 'deadline_changed',
                        'synced', 'field_changed'
                    )),

    actor           TEXT NOT NULL,
    actor_type      TEXT DEFAULT 'agent'
                    CHECK (actor_type IN ('agent', 'human', 'system', 'webhook')),

    summary         TEXT,
    data            JSONB,

    old_value       TEXT,
    new_value       TEXT,

    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_activity_task ON task_activity(task_id, created_at);
CREATE INDEX IF NOT EXISTS idx_activity_actor ON task_activity(actor);
CREATE INDEX IF NOT EXISTS idx_activity_type ON task_activity(task_id, type);

-- ============================================================
-- Admin Audit (append-only)
-- ============================================================

CREATE TABLE IF NOT EXISTS admin_audit (
    id              BIGSERIAL PRIMARY KEY,
    action          TEXT NOT NULL,
    actor           TEXT NOT NULL,
    target_type     TEXT,
    target_id       TEXT,
    details         JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_audit_actor ON admin_audit(actor);
CREATE INDEX IF NOT EXISTS idx_audit_target ON admin_audit(target_type, target_id);
