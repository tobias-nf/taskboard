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
    - setup taskboard
  patterns:
    - "(?i)I (need|have|should|must|ought) to"
    - "(?i)(remind me|don't let me forget|make sure I)"
    - "(?i)(by|before|until) (monday|tuesday|wednesday|thursday|friday|saturday|sunday|tomorrow|tonight|end of)"
    - "(?i)(promised|committed|agreed) (to|that)"
    - "(?i)(slack|email|dm|text) message from .+: .+"
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

## Setup

When the user says "setup taskboard", "enable taskboard", or when this skill activates for the first time:

### Step 1: Check existing setup

Call `memory_read(path="tasks/README.md")`. If it exists, tell the user: "Taskboard is already set up. Want me to reinstall from scratch?" Stop unless they confirm.

### Step 2: Gather configuration

Ask the user:
1. **Taskboard URL** — which instance to sync with?
   - Production: `https://taskboard.commitment-tracker-aiops-sandbox.site` (default)
   - Dev: `https://dev.taskboard.commitment-tracker-aiops-sandbox.site`
   - Self-hosted: any URL
2. **Digest channel** — which channel for daily digest? (default: current channel)

The API key is handled by the credential system — do not ask for it directly.

### Step 3: Write workspace structure

Write `tasks/README.md` with the configuration and schema:

```
memory_write(target="tasks/README.md", append=false, content="...")
```

Content for README.md:

```markdown
# Taskboard

Local-first task tracking with cloud sync.

## Config

base_url: <user's chosen URL>
digest_channel: <channel name>

## Directory Layout

- `open/` — Active tasks (one file each)
- `resolved/` — Completed/cancelled tasks
- `signals/pending/` — Extracted signals awaiting triage
- `signals/expired/` — Dismissed signals
- `sync.json` — Sync state and task_id mappings
```

Create directory placeholders:
- `memory_write(target="tasks/open/README.md", content="Active tasks.", append=false)`
- `memory_write(target="tasks/resolved/README.md", content="Resolved tasks.", append=false)`
- `memory_write(target="tasks/signals/pending/README.md", content="Signals awaiting triage.", append=false)`
- `memory_write(target="tasks/signals/expired/README.md", content="Expired signals.", append=false)`

Write initial sync state:
- `memory_write(target="tasks/sync.json", content='{"last_sync": null, "mappings": {}}', append=false)`

### Step 4: Create missions

Check `mission_list` first — skip if already exists.

**Triage mission** (twice daily):
```
mission_create(
  name: "taskboard-triage",
  goal: "Review pending signals and task status. (1) memory_tree('tasks/signals/pending/', depth=1) — for signals past expires_at or older than 48h, move to signals/expired/. (2) memory_tree('tasks/open/', depth=1) and memory_read each — flag overdue tasks, stale items. (3) If any items need attention, send a message to the user.",
  cadence: "0 9,18 * * *"
)
```

**Digest mission** (weekday mornings):
```
mission_create(
  name: "taskboard-digest",
  goal: "Compose morning task digest. (1) Sync from cloud: http(method='GET', url='<base_url>/api/v1/tasks/me') and update local files. (2) memory_tree('tasks/open/', depth=1) and memory_read each. (3) Count pending signals. (4) Compose digest grouped by: Overdue first, Due This Week, In Progress, Pending. End with pending signal count. (5) Send via message tool.",
  cadence: "0 8 * * 1-5"
)
```

### Step 5: Initial sync

Pull existing tasks from the cloud:
```
http(method="GET", url="<base_url>/api/v1/tasks/me")
```

For each task returned, create a local file in `tasks/open/<slug>.md` with `task_id` pre-filled. Update `tasks/sync.json`.

### Step 6: Confirm

Tell the user:

> Taskboard is ready. Here's what I set up:
> - Workspace structure under `tasks/`
> - **Triage mission** runs twice daily (9am, 6pm) — flags overdue tasks, expires stale signals
> - **Digest mission** runs weekday mornings at 8am — syncs from cloud and summarizes your tasks
> - Pulled **N existing tasks** from the cloud
>
> I'll track obligations from our conversations automatically. Say **"my tasks"** to see your current status, or **"sync tasks"** to force a cloud sync.

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
  README.md       Config and schema reference
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

`task_id` is null until synced to cloud. Once synced, holds the Taskboard ID.

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

**Triggers:** "I need to...", "I promised Sarah...", "I should get back to...", "The report is due Friday"

**Action:**
1. Check duplicates: `memory_search` for key phrases within `tasks/`
2. If no duplicate, `memory_write` to `tasks/signals/pending/<slug>.md`
3. After the write: "I've tracked a task about [topic]."

Do NOT interrupt conversation. Signal extraction is a side-effect. Do NOT sync to cloud during this mode.

**Immediacy:** realtime (production incidents) | prompt (named-person asks) | batch (most items)

## Mode B: Explicit Task Creation

User says: "track this", "create a task", "I need to do X by Friday".

**Action:**
1. Write directly to `tasks/open/<slug>.md`
2. Infer defaults, ask only if truly ambiguous
3. Confirm: "Tracked: [title], due [date], priority [level]."
4. Sync: push to cloud (Mode F)

**Priority:** emergency (today/overdue) | urgent (within 3 days) | standard (within 2 weeks) | low (no deadline)

## Mode C: Task Updates and Resolution

User says: "done with X", "mark T-2026-00042 done", "I'm blocked on the review".

**Action:**
1. Find task in `tasks/open/`
2. Update status, move to `tasks/resolved/` if completed/cancelled
3. Confirm: "Resolved: [title]."
4. Sync: push status change to cloud

## Mode D: Task Digest

User asks: "show my tasks", "what's overdue?", "what's on my plate?"

**Action:**
1. Pull from cloud first (Mode F pull) for latest shared state
2. Present grouped:

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

Read `tasks/README.md` for the `base_url`. Never block conversation. Run after Modes B, C, D, E. Never during Mode A.

### Push (local → cloud)

For tasks where `task_id` is null or local is newer than `synced_at`:

**Create task:**
```
http(method="POST", url="{base_url}/api/v1/tasks", body={
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

Write returned `task_id` back to local file. Update `tasks/sync.json`.

**Update task:**
```
http(method="PATCH", url="{base_url}/api/v1/tasks/{task_id}", body={
  "status": "in_progress",
  "priority": "urgent",
  "description": "...",
  "assigned_to": "agent-id",
  "parent_id": "T-2026-XXXXX"
})
```

**Add comment:**
```
http(method="POST", url="{base_url}/api/v1/tasks/{task_id}/activity", body={
  "body": "Progress update: completed phase 1."
})
```

### Pull (cloud → local)

```
http(method="GET", url="{base_url}/api/v1/tasks/me")
→ {"tasks": [...], "total": 15}
```

For each cloud task:
- Has local file and cloud newer → update local
- No local file → create in `tasks/open/` with `task_id` pre-filled
- Cloud completed but local open → move to `tasks/resolved/`

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

Base URL: read from `tasks/README.md` config (set during setup).
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

**Types:** user, service, admin

### Endpoints

#### Tasks

| Method | Path | Description |
|--------|------|-------------|
| POST | /tasks | Create task |
| GET | /tasks/me | My assigned tasks |
| GET | /tasks/me/created | Tasks I created |
| GET | /tasks/me/owed | Tasks owed to me |
| GET | /tasks/visible | All visible tasks |
| GET | /tasks/{id} | Get task detail |
| PATCH | /tasks/{id} | Update task |
| DELETE | /tasks/{id} | Cancel task |

**Create fields:** title (required), description, assigned_to, priority, deadline, parent_id, visibility, tags[]
**Update fields:** title, description, status, priority, deadline, assigned_to, parent_id, visibility
**Query params:** status, priority, tag, assigned_to, sort, limit, offset

#### Activity

| Method | Path | Description |
|--------|------|-------------|
| POST | /tasks/{id}/activity | Add comment: `{"body": "..."}` |
| GET | /tasks/{id}/activity | Get timeline |

#### Tags

| Method | Path | Description |
|--------|------|-------------|
| GET | /tags | List all tags |
| POST | /tags | Create: `{"name": "...", "color": "#hex"}` |
| GET | /tasks/{id}/tags | Tags on task |
| POST | /tasks/{id}/tags | Add: `{"tag_name": "..."}` |
| DELETE | /tasks/{id}/tags/{tagId} | Remove |

#### Stakeholders (owed_to)

| Method | Path | Description |
|--------|------|-------------|
| GET | /tasks/{id}/owed-to | List |
| POST | /tasks/{id}/owed-to | Add: `{"agent_id": "..."}` |
| DELETE | /tasks/{id}/owed-to/{agentId} | Remove |

#### Mentions

| Method | Path | Description |
|--------|------|-------------|
| GET | /tasks/{id}/mentions | List |
| POST | /tasks/{id}/mentions | Add: `{"agent_id": "..."}` |
| DELETE | /tasks/{id}/mentions/{agentId} | Remove |

#### References

| Method | Path | Description |
|--------|------|-------------|
| GET | /tasks/{id}/references | List |
| POST | /tasks/{id}/references | Add: `{"type": "related", "source": "slack", "title": "...", "url": "..."}` |
| DELETE | /tasks/{id}/references/{refId} | Remove |

Reference types: origin, related, blocks, depends_on, output

#### Agents

| Method | Path | Description |
|--------|------|-------------|
| GET | /agents/me | Current agent |
| PATCH | /agents/me | Update: email, slack_id, preferred_tool |
| GET | /agents/me/assignable | Active agents for assignment |
| GET | /agents | List all |
| GET | /agents/{id} | Get detail |

---

## Filename Conventions

Slugify: lowercase, hyphens, no special chars, max 50 chars.
- "Review Sarah's deck" → `review-sarah-deck.md`
- "Submit Q1 tax filing" → `submit-q1-tax-filing.md`
