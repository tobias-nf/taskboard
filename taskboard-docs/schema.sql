-- Taskboard — Current Database Schema
-- This reflects the actual production schema after all migrations.

CREATE TABLE agents (
    id              TEXT PRIMARY KEY,
    type            TEXT NOT NULL CHECK (type IN ('user', 'admin', 'service')),

    -- Auth
    api_key_hash    TEXT,
    api_key_prefix  TEXT,
    google_sub      TEXT UNIQUE,

    -- Metadata
    email           TEXT,
    slack_id        TEXT,
    preferred_tool  TEXT,

    -- Status
    active          BOOLEAN DEFAULT false,
    approved_by     TEXT REFERENCES agents(id),
    last_seen_at    TIMESTAMPTZ,

    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE SEQUENCE task_seq;

CREATE TABLE tasks (
    id              TEXT PRIMARY KEY DEFAULT 'T-' || to_char(now(), 'YYYY') || '-'
                    || lpad(nextval('task_seq')::text, 7, '0'),
    title           TEXT NOT NULL,
    description     TEXT,

    created_by      TEXT NOT NULL REFERENCES agents(id),
    assigned_to     TEXT REFERENCES agents(id),
    visibility      TEXT NOT NULL DEFAULT 'public' CHECK (visibility IN ('public', 'private')),

    status          TEXT NOT NULL DEFAULT 'pending' CHECK (status IN (
                        'draft', 'pending', 'in_progress', 'blocked',
                        'review', 'completed', 'failed', 'cancelled')),

    priority        TEXT DEFAULT 'standard' CHECK (priority IN ('low', 'standard', 'urgent', 'emergency')),
    deadline        TIMESTAMPTZ,
    parent_id       TEXT REFERENCES tasks(id),

    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE tags (
    id          BIGSERIAL PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    color       TEXT,
    created_by  TEXT NOT NULL REFERENCES agents(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE task_tags (
    task_id     TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    tag_id      BIGINT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    added_by    TEXT NOT NULL REFERENCES agents(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (task_id, tag_id)
);

CREATE TABLE task_owed_to (
    task_id     TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    agent_id    TEXT NOT NULL REFERENCES agents(id),
    added_by    TEXT NOT NULL REFERENCES agents(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (task_id, agent_id)
);

CREATE TABLE task_mentions (
    task_id     TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    agent_id    TEXT NOT NULL REFERENCES agents(id),
    added_by    TEXT NOT NULL REFERENCES agents(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (task_id, agent_id)
);

CREATE TABLE task_references (
    id              BIGSERIAL PRIMARY KEY,
    task_id         TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    type            TEXT NOT NULL CHECK (type IN ('origin', 'related', 'blocks', 'depends_on', 'output')),
    source          TEXT NOT NULL,
    external_id     TEXT,
    url             TEXT,
    title           TEXT NOT NULL,
    metadata        JSONB,
    created_by      TEXT NOT NULL REFERENCES agents(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE task_attachments (
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

CREATE TABLE task_activity (
    id              BIGSERIAL PRIMARY KEY,
    task_id         TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    type            TEXT NOT NULL,
    actor           TEXT NOT NULL REFERENCES agents(id),
    actor_type      TEXT DEFAULT 'agent',
    summary         TEXT,
    data            JSONB,
    old_value       TEXT,
    new_value       TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE admin_audit (
    id              BIGSERIAL PRIMARY KEY,
    action          TEXT NOT NULL,
    actor           TEXT NOT NULL,
    target_type     TEXT,
    target_id       TEXT,
    details         JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
