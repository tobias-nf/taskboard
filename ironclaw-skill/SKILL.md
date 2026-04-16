---
name: taskboard
version: "1.0.0"
description: Track tasks locally with background sync to Taskboard API. Replaces commitment-triage, commitment-digest, and commitment-setup.
activation:
  keywords:
    - need to
    - have to
    - must do
    - promised
    - committed to
    - deadline
    - by friday
    - by tomorrow
    - follow up
    - get back to
    - remind me
    - track this
    - mark done
    - my tasks
    - what's overdue
    - task
    - taskboard
    - action item
    - blocked
    - subtask
  patterns:
    - "(?i)I (need|have|should|must|ought) to"
    - "(?i)(remind me|don't let me forget|make sure I)"
    - "(?i)(by|before|until) (monday|tuesday|wednesday|thursday|friday|saturday|sunday|tomorrow|tonight|end of)"
    - "(?i)(promised|committed|agreed) (to|that)"
    - "(?i)(slack|email|dm|text) message from .+: .+"
  exclude_keywords:
    - setup commitments
    - install commitments
  tags:
    - task-management
    - commitments
    - personal-assistant
  max_context_tokens: 2000
credentials:
  - name: taskboard_api_key
    provider: taskboard
    location:
      type: bearer
    hosts:
      - "taskboard.commitment-tracker-aiops-sandbox.site"
      - "dev.taskboard.commitment-tracker-aiops-sandbox.site"
    setup_instructions: "Get your API key from Taskboard Settings or ask an admin. Format: hive_sk_<agent-id>_<secret>"
---

# Taskboard — Local-First Task Management with Cloud Sync

Tasks are stored locally as markdown files in `tasks/` for instant response times. Background sync pushes to and pulls from the Taskboard REST API so tasks are visible in the dashboard, Slack, and to other agents.

Local files are the fast path — write immediately, never block on API calls.
The Taskboard API is the shared source of truth for team visibility.

---

## Workspace Layout

```
tasks/
  open/           Active tasks (one file each)
  resolved/       Completed/cancelled tasks (archived)
  signals/        Raw extracted signals awaiting triage
    pending/      Not yet promoted to tasks
    expired/      Dismissed or expired signals
  sync.json       Sync state (last sync timestamp, task_id mappings)
```

## Task File Schema (tasks/open/<slug>.md)

```
---
type: task
status: pending | in_progress | blocked | review | completed | failed | cancelled
priority: low | standard | urgent | emergency
due: <YYYY-MM-DD> | null
created_at: <YYYY-MM-DD>
assigned_to: <agent-id> | null
parent_slug: <parent-filename> | null
task_id: <T-2026-XXXXX> | null
synced_at: <ISO8601> | null
tags: [<tag names>]
---
# <Title>

<Description — markdown.>
```

`task_id` is null until synced to cloud. Once synced, it holds the Taskboard ID.

## Signal File Schema (tasks/signals/pending/<slug>.md)

```
---
type: signal
source_channel: <channel name>
source_message: "<brief quote>"
detected_at: <YYYY-MM-DD>
immediacy: realtime | prompt | batch
expires_at: <YYYY-MM-DD> | null
confidence: high | medium | low
obligation_type: reply | deliver | attend | review | decide | follow-up
mentions: [<names>]
destination: null | task | dismissed
promoted_to: null | <task filename>
---
<1-2 sentence description.>
```

---

## Mode A: Passive Signal Detection

When the user says something that implies an obligation but is NOT explicitly asking to track it — silently extract a signal.

**Triggers:** "I need to...", "I promised Sarah...", "I should get back to...", "The report is due Friday", "They asked me to review..."

**Action:**
1. Check duplicates: `memory_search` for key phrases within `tasks/`
2. If no duplicate, `memory_write` to `tasks/signals/pending/<slug>.md`
3. After the write: "I've tracked a task about [topic]."

Do NOT interrupt conversation. Signal extraction is a side-effect. Do NOT sync to cloud during this mode.

**Immediacy:** realtime (production incidents) | prompt (named-person asks, urgent DMs) | batch (most items)

## Mode B: Explicit Task Creation

User says: "track this", "create a task", "I need to do X by Friday".

**Action:**
1. Write directly to `tasks/open/<slug>.md` — skip signal stage
2. Infer defaults, ask only if truly ambiguous
3. Confirm: "Tracked: [title], due [date], priority [level]."
4. Then sync: push to cloud (see Mode F)

**Priority:** emergency (today/overdue) | urgent (within 3 days) | standard (within 2 weeks) | low (no deadline)

## Mode C: Task Updates and Resolution

User says: "done with X", "finished the review", "mark T-2026-00042 done".

**Action:**
1. Find matching task in `tasks/open/`
2. Update status, move to `tasks/resolved/<slug>.md` if completed/cancelled
3. Confirm: "Resolved: [title]."
4. Then sync: push status change to cloud

## Mode D: Task Digest

User asks: "show my tasks", "what's overdue?", "what's on my plate?"

**Action:**
1. First pull from cloud (Mode F pull) to get latest shared state
2. Then read local files and present grouped:

```
## Tasks — <today's date>

### Overdue / Emergency
- **<title>** (due <date>) — <status>

### Due This Week
- **<title>** (due <date>) — <status>

### In Progress
- **<title>** — <status>

### Pending
- **<title>** — priority: <priority>

### Pending Signals (<count>)
Say "review signals" to triage.
```

Omit empty sections. Zero tasks: "No open tasks. You're clear."

## Mode E: Signal Triage

User says "review signals" or "triage".

1. List `tasks/signals/pending/`
2. For each: actionable → create task in `tasks/open/`, not relevant → move to `signals/expired/`
3. Sync promoted tasks to cloud

## Mode F: Cloud Sync

Never block conversation. Run after Modes B, C, D, E. Never during Mode A.

### Push (local → cloud)

For tasks where `task_id` is null (new) or local is newer than `synced_at`:

**Create task:**
```
http(method="POST", url="https://taskboard.commitment-tracker-aiops-sandbox.site/api/v1/tasks", body={
  "title": "...",
  "description": "...",
  "priority": "standard",
  "deadline": "2026-04-25T00:00:00Z",
  "assigned_to": "agent-id",
  "parent_id": "T-2026-XXXXX",
  "visibility": "public",
  "tags": ["tag-name"]
})
→ {"id": "T-2026-00042", "title": "...", "status": "pending", ...}
```

Write returned `task_id` back to local file frontmatter. Update `tasks/sync.json`.

**Update task:**
```
http(method="PATCH", url="https://taskboard.commitment-tracker-aiops-sandbox.site/api/v1/tasks/{task_id}", body={
  "status": "in_progress",
  "priority": "urgent",
  "description": "updated...",
  "assigned_to": "other-agent",
  "parent_id": "T-2026-XXXXX"
})
→ {"id": "T-2026-00042", "status": "in_progress", ...}
```

**Add comment:**
```
http(method="POST", url="https://taskboard.commitment-tracker-aiops-sandbox.site/api/v1/tasks/{task_id}/activity", body={
  "body": "Progress update: completed phase 1."
})
```

### Pull (cloud → local)

**Get my tasks:**
```
http(method="GET", url="https://taskboard.commitment-tracker-aiops-sandbox.site/api/v1/tasks/me")
→ {"tasks": [...], "total": 15}
```

**Get tasks I created:**
```
http(method="GET", url="https://taskboard.commitment-tracker-aiops-sandbox.site/api/v1/tasks/me/created")
```

**Get tasks owed to me:**
```
http(method="GET", url="https://taskboard.commitment-tracker-aiops-sandbox.site/api/v1/tasks/me/owed")
```

For each cloud task:
- Has local file (via sync.json) and cloud is newer → update local
- No local file → create in `tasks/open/` with `task_id` pre-filled
- Cloud completed/cancelled but local open → move to `tasks/resolved/`

### Conflict: cloud wins.

### sync.json
```json
{
  "last_sync": "2026-04-17T10:00:00Z",
  "mappings": {
    "review-sarah-deck.md": "T-2026-00042",
    "submit-q1-report.md": "T-2026-00043"
  }
}
```

---

## Taskboard REST API Reference

Base URL: `https://taskboard.commitment-tracker-aiops-sandbox.site/api/v1`
Auth: credentials are injected automatically — never construct Authorization headers.

### Task Object

```json
{
  "id": "T-2026-00042",
  "title": "Review partnership agreement",
  "description": "Markdown description...",
  "created_by": "tobias.holenstein",
  "assigned_to": "alice",
  "visibility": "public",
  "status": "in_progress",
  "priority": "standard",
  "deadline": "2026-04-25T00:00:00Z",
  "parent_id": null,
  "created_at": "2026-04-15T08:00:00Z",
  "started_at": "2026-04-15T09:00:00Z",
  "completed_at": null,
  "updated_at": "2026-04-16T10:00:00Z"
}
```

**Statuses:** draft, pending, in_progress, blocked, review, completed, failed, cancelled
**Priorities:** low, standard, urgent, emergency
**Visibility:** public (default), private
**Valid transitions:** pending → in_progress/completed/blocked/cancelled. in_progress → blocked/review/completed/failed/cancelled. blocked → in_progress/cancelled. review → completed/in_progress/cancelled.

### Agent Object

```json
{
  "id": "tobias.holenstein",
  "type": "user",
  "email": "tobias.holenstein@near.foundation",
  "slack_id": "U0AJ1HULDL3",
  "active": true
}
```

**Types:** user (humans/bots), service (system integrations), admin

### Endpoints

#### Tasks

| Method | Path | Description |
|--------|------|-------------|
| POST | /tasks | Create task |
| GET | /tasks/me | My assigned tasks |
| GET | /tasks/me/created | Tasks I created |
| GET | /tasks/me/owed | Tasks owed to me (stakeholder) |
| GET | /tasks/visible | All visible tasks |
| GET | /tasks/{id} | Get task detail |
| PATCH | /tasks/{id} | Update task fields |
| DELETE | /tasks/{id} | Cancel task |

**Create task fields:** title (required), description, assigned_to, priority, deadline, parent_id, visibility, tags (string array)
**Update task fields:** title, description, status, priority, deadline, assigned_to, parent_id, visibility

**Query params for list endpoints:** status (comma-separated), priority (comma-separated), tag, assigned_to, sort, limit, offset

#### Task Activity

| Method | Path | Description |
|--------|------|-------------|
| POST | /tasks/{id}/activity | Add comment: `{"body": "..."}` |
| GET | /tasks/{id}/activity | Get activity timeline |

#### Task Tags

| Method | Path | Description |
|--------|------|-------------|
| GET | /tasks/{id}/tags | List tags on task |
| POST | /tasks/{id}/tags | Add tag: `{"tag_name": "..."}` |
| DELETE | /tasks/{id}/tags/{tagId} | Remove tag |

#### Task Stakeholders (owed_to)

| Method | Path | Description |
|--------|------|-------------|
| GET | /tasks/{id}/owed-to | List stakeholders |
| POST | /tasks/{id}/owed-to | Add: `{"agent_id": "..."}` |
| DELETE | /tasks/{id}/owed-to/{agentId} | Remove |

#### Task Mentions

| Method | Path | Description |
|--------|------|-------------|
| GET | /tasks/{id}/mentions | List mentions |
| POST | /tasks/{id}/mentions | Add: `{"agent_id": "..."}` |
| DELETE | /tasks/{id}/mentions/{agentId} | Remove |

#### Task References

| Method | Path | Description |
|--------|------|-------------|
| GET | /tasks/{id}/references | List references |
| POST | /tasks/{id}/references | Add: `{"type": "related", "source": "slack", "title": "...", "url": "..."}` |
| DELETE | /tasks/{id}/references/{refId} | Remove |

Reference types: origin, related, blocks, depends_on, output

#### Agents

| Method | Path | Description |
|--------|------|-------------|
| GET | /agents/me | Current agent profile |
| PATCH | /agents/me | Update profile: email, slack_id, preferred_tool |
| GET | /agents/me/assignable | Active agents for assignment |
| GET | /agents | List all agents |
| GET | /agents/{id} | Get agent detail |

#### Tags

| Method | Path | Description |
|--------|------|-------------|
| GET | /tags | List all tags |
| POST | /tags | Create tag: `{"name": "...", "color": "#hex"}` |

---

## Filename Conventions

Slugify: lowercase, hyphens, no special chars, max 50 chars.
- "Review Sarah's deck" → `review-sarah-deck.md`
- "Submit Q1 tax filing" → `submit-q1-tax-filing.md`
