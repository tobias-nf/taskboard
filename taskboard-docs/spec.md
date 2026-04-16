# Taskboard — Agent & Task Management Platform

## Overview

Taskboard is the central control plane for the agent ecosystem at NEAR Intents. It manages:

- **Agents** — Registration, capabilities, authentication
- **Tasks** — Universal task board with descriptions, playbooks, notes, references, and attachments
- **Activity** — Full audit trail of everything that happens
- **References** — Links between tasks and external systems (Jira, Slack, Gmail, meetings, etc.)
- **Auth** — Google OAuth for dashboard login, API keys for MCP/programmatic access

Taskboard is domain-agnostic. It doesn't know about AML cases, bug fixes, or marketing campaigns. Domain knowledge lives in the agents and their playbooks. Taskboard routes, stores, and secures.

## Architecture

```
+-----------------------------------------------------------------+
|                        Taskboard API                                  |
|                    (REST + SSE)                                   |
|                                                                  |
|  +----------+  +----------+  +----------+                        |
|  | Agents   |  | Tasks    |  | Activity |                        |
|  | Registry |  | Board    |  | Log      |                        |
|  +----------+  +----------+  +----------+                        |
|                                                                  |
|  +------------------------------------------------------------+  |
|  | Postgres (source of truth)                                  |  |
|  +------------------------------------------------------------+  |
+-----------------------------------------------------------------+
                             |
              +--------------+--------------+--------------+
              v              v              v              v
      Compliance     Bob's Agent    Sam's Agent   Sync Service
        Bot          (picks up      (approves      (mirrors to
      (creates        tasks,         escalations)   Jira/Linear)
       tasks)         works them)
```

## Data Model

### Agents

Every participant in the system — users (humans and bots), service integrations (Slack app, Fireflies bridge — elevated task access, no platform management), and admins (platform administrators). Humans log in via Google OAuth (`@near.foundation`); MCP clients and service integrations use API keys.

```sql
CREATE TABLE agents (
    id              TEXT PRIMARY KEY,        -- alice, meeting-tracker, hive-admin
    name            TEXT NOT NULL,           -- "Alice Johnson", "Meeting Tracker"
    type            TEXT NOT NULL
                    CHECK (type IN ('user', 'service', 'admin')),
    description     TEXT,

    -- Auth (API key — nullable for OAuth-only agents)
    api_key_hash    TEXT,
    api_key_prefix  TEXT,

    -- Google OAuth (nullable for API-key-only agents)
    google_sub      TEXT UNIQUE,

    -- Metadata
    email           TEXT,
    slack_id        TEXT,
    title           TEXT,

    -- Preferences
    preferred_tool  TEXT,

    -- Status
    active          BOOLEAN DEFAULT true,
    approved_by     TEXT REFERENCES agents(id),
    last_seen_at    TIMESTAMPTZ,

    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### Visibility Model

Tasks are **public by default**. Anyone authenticated can see public tasks. Tasks can be marked **private** to restrict visibility to specific people.

**Private task visibility**: creator, assignee, owed_to agents (stakeholders), and mentioned agents.

**Assignment**: Any authenticated agent can assign tasks to any active agent. No workspace gate.

### Tasks

Universal task structure with public/private visibility.

```sql
CREATE SEQUENCE task_seq;

CREATE TABLE tasks (
    id              TEXT PRIMARY KEY DEFAULT 'T-' || to_char(now(), 'YYYY') || '-'
                    || lpad(nextval('task_seq')::text, 7, '0'),

    -- What
    title           TEXT NOT NULL,
    description     TEXT,
    playbook        TEXT,                    -- markdown playbook: step-by-step instructions
    notes           TEXT,                    -- research, analysis, working notes (markdown)

    -- Who
    created_by      TEXT NOT NULL REFERENCES agents(id),
    assigned_to     TEXT REFERENCES agents(id),
    visibility      TEXT NOT NULL DEFAULT 'public'
                    CHECK (visibility IN ('public', 'private')),

    -- State
    status          TEXT NOT NULL DEFAULT 'pending'
                    CHECK (status IN (
                        'draft',
                        'pending',
                        'accepted',
                        'in_progress',
                        'blocked',
                        'review',
                        'completed',
                        'failed',
                        'cancelled'
                    )),
    blocked_by      TEXT,                    -- task ID or free text

    -- Priority and timing
    priority        TEXT DEFAULT 'standard'
                    CHECK (priority IN ('low', 'standard', 'urgent', 'emergency')),
    deadline        TIMESTAMPTZ,

    -- Result
    result          JSONB,
    failure_reason  TEXT,

    -- Hierarchy
    parent_id       TEXT REFERENCES tasks(id),

    -- Timestamps
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    accepted_at     TIMESTAMPTZ,
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### Tags

Organizational labels for tasks. No access control implications — purely for filtering and grouping.

```sql
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
```

### Owed-to (Stakeholders)

Agents the task result is owed to. Grants visibility on private tasks.

```sql
CREATE TABLE task_owed_to (
    task_id     TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    agent_id    TEXT NOT NULL REFERENCES agents(id),
    added_by    TEXT NOT NULL REFERENCES agents(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (task_id, agent_id)
);
```

### Mentions

Explicit access grants. Mentioning an agent on a private task grants them visibility.

```sql
CREATE TABLE task_mentions (
    task_id     TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    agent_id    TEXT NOT NULL REFERENCES agents(id),
    added_by    TEXT NOT NULL REFERENCES agents(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (task_id, agent_id)
);
```

### References

Links between tasks and external systems.

```sql
CREATE TABLE task_references (
    id              BIGSERIAL PRIMARY KEY,
    task_id         TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,

    -- Relationship
    type            TEXT NOT NULL
                    CHECK (type IN (
                        'origin',            -- where the task came from
                        'related',           -- relevant context
                        'blocks',            -- this reference blocks the task
                        'depends_on',        -- task depends on this
                        'output'             -- something produced by the task
                    )),

    -- The external thing
    source          TEXT NOT NULL,           -- jira, slack, gmail, linear, github,
                                             -- meeting, document, task, other
    external_id     TEXT,                    -- JIRA-1234, slack ts, email ID, task ID
    url             TEXT,                    -- clickable link
    title           TEXT NOT NULL,           -- human-readable label
    metadata        JSONB,                   -- source-specific data

    created_by      TEXT NOT NULL REFERENCES agents(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_refs_task ON task_references(task_id);
CREATE INDEX idx_refs_source ON task_references(source, external_id);
CREATE INDEX idx_refs_type ON task_references(task_id, type);
```

### Activity

Append-only timeline of everything that happens.

```sql
CREATE TABLE task_activity (
    id              BIGSERIAL PRIMARY KEY,
    task_id         TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,

    -- What happened
    type            TEXT NOT NULL
                    CHECK (type IN (
                        'created',
                        'status_changed',
                        'assigned',
                        'reassigned',
                        'commented',
                        'reference_added',
                        'reference_removed',
                        'playbook_step_completed',
                        'playbook_step_failed',
                        'escalated',
                        'blocked',
                        'unblocked',
                        'priority_changed',
                        'deadline_changed',
                        'synced',
                        'field_changed'
                    )),

    -- Who did it
    actor           TEXT NOT NULL,           -- agent ID, person name, or "jira-webhook"
    actor_type      TEXT DEFAULT 'agent'
                    CHECK (actor_type IN ('agent', 'human', 'system', 'webhook')),

    -- Details
    summary         TEXT,                    -- human-readable one-liner
    data            JSONB,                   -- structured details

    -- For state changes
    old_value       TEXT,
    new_value       TEXT,

    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Append-only: no UPDATE or DELETE policy
CREATE INDEX idx_activity_task ON task_activity(task_id, created_at);
CREATE INDEX idx_activity_actor ON task_activity(actor);
CREATE INDEX idx_activity_type ON task_activity(task_id, type);
```

### Attachments

Files attached to tasks — PDFs, screenshots, drafts, evidence, etc. Files are stored in object storage (S3/MinIO), metadata in Postgres.

```sql
CREATE TABLE task_attachments (
    id              BIGSERIAL PRIMARY KEY,
    task_id         TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    filename        TEXT NOT NULL,
    mime_type       TEXT,
    size_bytes      BIGINT,
    sha256          TEXT,
    storage_key     TEXT NOT NULL,            -- path in object store
    storage_url     TEXT,                     -- pre-signed or public URL
    label           TEXT,                     -- "official_request_pdf", "draft_v2", etc.
    uploaded_by     TEXT NOT NULL REFERENCES agents(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

Upload flow: agent POSTs multipart file to the API. The API stores the file in object storage, records metadata in Postgres, and returns the attachment ID + download URL.

Playbooks are written per-task in the `playbook` TEXT field on tasks. Notes and references carry the evolving research and links.

## API

### Authentication

```
All endpoints (except registration) require:
  Authorization: Bearer hive_sk_<agent-id>_<secret>
```

### Agents

```
POST   /api/v1/agents/register           Register an agent (unauthenticated)
POST   /api/v1/agents/me/rotate-key      Rotate API key
GET    /api/v1/agents/me                 Get current agent
PATCH  /api/v1/agents/me                 Update my profile
GET    /api/v1/agents/me/assignable      Active agents available for assignment
GET    /api/v1/agents                    List agents (admin: paginated with search)
GET    /api/v1/agents/:id               Get agent details
```

### Tasks

```
POST   /api/v1/tasks                     Create task (use parent_id for subtasks)
GET    /api/v1/tasks/visible             All tasks visible to the caller
GET    /api/v1/tasks/:id                 Get task details
PATCH  /api/v1/tasks/:id                 Update task (status, priority, parent_id, reassign, etc.)

GET    /api/v1/tasks/me                  Tasks assigned to me
GET    /api/v1/tasks/me/created          Tasks I created
GET    /api/v1/tasks/me/owed             Tasks owed to me (stakeholder)
```

### Activity

```
POST   /api/v1/tasks/:id/activity        Add activity (comment, status change, etc.)
GET    /api/v1/tasks/:id/activity         Get full activity timeline
```

### References

```
POST   /api/v1/tasks/:id/references      Add reference
GET    /api/v1/tasks/:id/references       List references
DELETE /api/v1/tasks/:id/references/:ref  Remove reference
```

### Attachments

```
POST   /api/v1/tasks/:id/attachments     Upload file (multipart/form-data)
GET    /api/v1/tasks/:id/attachments     List attachments
GET    /api/v1/attachments/:id/download  Download file (returns pre-signed URL or streams)
DELETE /api/v1/attachments/:id           Delete attachment
```

Upload example:
```
POST /api/v1/tasks/T-2026-00042/attachments
Content-Type: multipart/form-data
  file: (binary)
  label: "official_request_pdf"
-> { "id": 1, "filename": "Vendor_Review_2026.pdf", "size_bytes": 245000,
    "url": "https://storage.../T-2026-00042/Vendor_Review_2026.pdf" }
```

### Events (SSE)

```
GET    /api/v1/events                    SSE stream — real-time task events
                                         (task.created, task.updated, task.commented,
                                          task.cancelled, task.completed, task.failed)
```

Agents connect and receive events for tasks where they are creator, assignee, or owed-to/mentioned. Heartbeat every 30s. On reconnect, poll GET /tasks/me to catch up.

## Security Model

- **Agent-level auth**: API key per agent (format: `hive_sk_<agent-id>_<secret>`). Secret portion bcrypt-hashed.
- **Google OAuth for humans**: Dashboard login restricted to `@near.foundation`. JWT sessions.
- **Public-by-default visibility**: All authenticated agents see public tasks. Private tasks visible only to creator, assignee, owed_to stakeholders, mentioned agents, and service/admin agents.
- **Activity is append-only**: No UPDATE or DELETE on task_activity. Full audit trail.
- **Service/admin bypass**: Service and admin agents can see all tasks and bypass visibility restrictions.
- **Subtask visibility**: Inherited from parent task, cannot be changed independently.
