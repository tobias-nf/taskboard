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
    setup_instructions: "Get your API key from Taskboard Settings or ask an admin to create one. Format: hive_sk_<agent-id>_<secret>"
---

# Taskboard — Local-First Task Management with Cloud Sync

You manage tasks in a local workspace (`tasks/`) for instant response times. A background sync pushes tasks to the Taskboard API and pulls updates from other agents.

Local files are the fast path — write immediately, never block on API calls.
The Taskboard API is the shared source of truth — other agents, Slack, and the dashboard interact through it.

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

Read `tasks/sync.json` for the mapping between local filenames and Taskboard task IDs. If it doesn't exist, create it with `{"last_sync": null, "mappings": {}}`.

## Task Schema (tasks/open/<slug>.md)

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

<Description — markdown, as detailed as needed.>
```

The `task_id` field is null until the task is synced to Taskboard. Once synced, it contains the Taskboard ID (e.g. `T-2026-00042`).

## Signal Schema (tasks/signals/pending/<slug>.md)

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
<1-2 sentence description of the detected obligation.>
```

## Mode A: Passive Signal Detection

When the user says something that implies an obligation but is NOT explicitly asking to track it — silently extract a signal.

**Triggers:** "I need to...", "I promised Sarah...", "I should get back to...", "The report is due Friday", "They asked me to review..."

Strong passive signals (create even without "track this"):
- "The strategy team asked me to review the expansion reforecast this week."
- "Legal asked me to comment on the draft by Thursday."
- "Slack message from Priya: can you review the OAuth callback edge case this week?"

**Action:**
1. Check for duplicates: `memory_search` for key phrases within `tasks/`
2. If no duplicate, `memory_write` to `tasks/signals/pending/<slug>.md`
3. Only after the write succeeds, briefly note: "I've tracked a task about [topic]."

Do NOT interrupt the conversation flow. Signal extraction is a side-effect.

**Immediacy rules:**
- `realtime`: production incidents, security alerts, anything marked urgent
- `prompt`: urgent DMs from named people, time-sensitive requests
- `batch`: most obligations — meeting action items, reports, multi-day deadlines

## Mode B: Explicit Task Creation

When the user explicitly says: "track this", "create a task", "I need to do X by Friday".

**Action:**
1. Skip the signal stage — write directly to `tasks/open/<slug>.md`
2. Infer reasonable defaults. Ask only if truly ambiguous.
3. Confirm: "Tracked: [title], due [date], priority [level]."

**Priority rules:**
- `emergency`: due today or overdue, production impact
- `urgent`: due within 3 days
- `standard`: due within 2 weeks or no hard deadline
- `low`: no deadline, whenever

## Mode C: Task Updates and Resolution

When the user says they finished something: "done with X", "finished the review", "mark T-2026-00042 done".

**Action:**
1. `memory_tree("tasks/open/", depth=1)` to find the matching task
2. `memory_read` to confirm
3. Update status, write to `tasks/resolved/<slug>.md`
4. Clear the original: `memory_write(target="tasks/open/<slug>.md", content="", append=false)`
5. Confirm: "Resolved: [title]."

For status changes (blocked, in_progress, etc.), update the file in place.

## Mode D: Task Digest

When the user asks: "show my tasks", "what's overdue?", "what's on my plate?"

**Action:**
1. `memory_tree("tasks/open/", depth=1)` — list open tasks
2. `memory_read` each to extract frontmatter
3. Count pending signals: `memory_tree("tasks/signals/pending/", depth=1)`

Present grouped by urgency:

```
## Tasks — <today's date>

### Overdue / Emergency
- **<title>** (due <date>) — <status>

### Due This Week
- **<title>** (due <date>) — <status>, assigned to <agent>

### In Progress
- **<title>** — <status>

### Pending (not started)
- **<title>** — priority: <priority>

### Pending Signals (<count>)
<count> unprocessed signals. Say "review signals" to triage them.

---
Did I miss anything?
```

Omit empty sections. If zero tasks and zero signals: "No open tasks. You're clear."

## Mode E: Signal Triage

When reviewing signals (user says "review signals" or "triage"):

1. `memory_tree("tasks/signals/pending/", depth=1)`
2. For each signal, `memory_read` and decide:
   - Actionable → create task in `tasks/open/`, update signal `destination: task`
   - Not relevant → move to `signals/expired/`, set `destination: dismissed`
3. Confirm each decision briefly.

## Mode F: Cloud Sync

**When to sync:** After creating, updating, or resolving tasks locally. NOT during signal detection (too frequent). Sync is a background operation — never block conversation on it.

**Sync logic:**

### Push (local → cloud)
For each task in `tasks/open/` and `tasks/resolved/` where `task_id` is null or `synced_at` is older than local changes:

1. Read `tasks/sync.json` for existing mappings
2. If `task_id` is null → create via API:
   ```
   http(method="POST", url="https://taskboard.commitment-tracker-aiops-sandbox.site/api/v1/tasks", body={
     "title": "<title>",
     "description": "<description>",
     "priority": "<priority>",
     "deadline": "<due as ISO8601 or null>",
     "assigned_to": "<agent-id or null>",
     "parent_id": "<parent task_id or null>"
   })
   ```
3. Write the returned `task_id` back to the local file's frontmatter
4. Update `tasks/sync.json` with the mapping

If `task_id` exists but local is newer → update via API:
```
http(method="PATCH", url="https://taskboard.commitment-tracker-aiops-sandbox.site/api/v1/tasks/<task_id>", body={
  "status": "<status>",
  "priority": "<priority>",
  "description": "<description>"
})
```

### Pull (cloud → local)
Query the API for the user's tasks:
```
http(method="GET", url="https://taskboard.commitment-tracker-aiops-sandbox.site/api/v1/tasks/me")
```

For each cloud task:
- If it has a local file (via sync.json mapping) and cloud is newer → update local file
- If it has no local file → create one in `tasks/open/` with the `task_id` pre-filled
- If cloud status is completed/cancelled but local is still open → move to `tasks/resolved/`

### Conflict resolution
Cloud wins. If both local and cloud changed since last sync, take the cloud version and log the conflict in the task description.

### Sync state
After each sync, update `tasks/sync.json`:
```json
{
  "last_sync": "<ISO8601>",
  "mappings": {
    "review-sarah-deck.md": "T-2026-00042",
    "submit-q1-report.md": "T-2026-00043"
  }
}
```

## When to trigger sync

- After Mode B (explicit task creation) — push the new task
- After Mode C (resolution) — push the status change
- After Mode D (digest) — pull latest from cloud first, then show
- After Mode E (triage) — push any promoted tasks
- User says "sync tasks" — full push + pull
- Never during Mode A (passive signals) — too frequent, signals are local-only until promoted

## Filename Conventions

Slugify: lowercase, hyphens, no special chars, max 50 chars. Examples:
- "Review Sarah's deck" → `review-sarah-deck.md`
- "Submit Q1 tax filing" → `submit-q1-tax-filing.md`

## API Notes

- All API calls use `https://taskboard.commitment-tracker-aiops-sandbox.site/api/v1` as the base URL. Credentials are automatically injected — never construct Authorization headers manually.
- Task IDs look like `T-2026-00042`.
- Valid status transitions: pending → in_progress → completed/blocked/review. Blocked → in_progress. Review → completed/in_progress.
- Subtasks: set `parent_id` when creating. Subtask visibility is inherited from parent.
