# Taskboard Slack App

Slack interface for the Taskboard — draft task approvals, natural language `/task` commands, and event-driven notifications.

## Features

### Draft task approvals

When a draft task is created (e.g. by the [Fireflies Bridge](../fireflies-bridge/)), the Slack app:
1. Receives a `task.created` SSE event from the Taskboard API
2. Looks up stakeholders (`owed_to`) and resolves their Slack IDs
3. Sends a DM with approve / edit / reject buttons

Approved tasks move from `draft → pending` and become visible to the assignee. Rejected tasks are cancelled. Edited tasks can have title, assignee, priority, and description changed before approval.

### `/task` slash command

Natural language interaction powered by Claude:

```
/task what are my priorities today?
/task mark T-2026-00042 as done
/task I'm blocked on the vendor review, waiting for legal
/task create: follow up with ACME by Friday
/task reassign T-2026-00042 to sarah
```

The Claude agent has access to Taskboard tools scoped to the requesting user:
- `list_my_tasks` — tasks assigned to me
- `list_tasks_created` — tasks I created
- `list_tasks_owed` — tasks where I'm a stakeholder
- `get_task` — full task details
- `update_task` — change status, priority, deadline
- `add_comment` — add to activity log
- `create_task` — create a new task

### Security model

The Slack user ID is the identity gate. On every request:
1. Slack user ID is resolved to a Taskboard agent via the `slack_id` field
2. All Taskboard API calls use the `X-Act-As` header for impersonation
3. The audit trail records the real user as the actor, with `via: taskboard-slack`
4. Unregistered Slack users get an error — no anonymous access

## Setup

### 1. Create the Slack app

Go to [api.slack.com/apps](https://api.slack.com/apps) → **Create New App** → **From an app manifest** → paste `slack-app-manifest.yaml`.

After creation:
- Install to your workspace
- Copy the **Bot User OAuth Token** (`xoxb-...`)
- Copy the **Signing Secret** from Basic Information

### 2. Register the Slack app as a Taskboard agent

```bash
curl -X POST https://taskboard.nearintents.org/api/v1/agents \
  -H "Authorization: Bearer <admin-key>" \
  -H "Content-Type: application/json" \
  -d '{"id": "taskboard-slack", "name": "Jarvis", "type": "user"}'
```

### 3. Ensure all users have `slack_id` set

Every agent that will use `/task` needs their Slack user ID on their agent profile:

```bash
curl -X PATCH https://taskboard.nearintents.org/api/v1/agents/tobias \
  -H "Authorization: Bearer <admin-key>" \
  -H "Content-Type: application/json" \
  -d '{"slack_id": "U_TOBIAS"}'
```

### 4. Environment variables

Copy `.env.example` to `.env` and fill in:

| Variable | Description |
|---|---|
| `TASKBOARD_URL` | Taskboard API base URL |
| `TASKBOARD_API_KEY` | API key for the `taskboard-slack` agent |
| `SLACK_BOT_TOKEN` | Bot User OAuth Token from the Slack app |
| `SLACK_SIGNING_SECRET` | Signing Secret from the Slack app |
| `ANTHROPIC_API_KEY` | For Claude-powered `/task` commands |
| `ADMIN_SLACK_ID` | Fallback Slack user ID for unresolvable agents |

### 5. Run

**Local dev (Socket Mode — recommended, no public URL needed):**

1. Enable Socket Mode in your Slack app: api.slack.com → your app → **Socket Mode** → toggle on
2. Generate an App-Level Token with `connections:write` scope
3. Add `SLACK_APP_TOKEN=xapp-...` to your `.env`
4. Run:

```bash
pip install .
python socket_mode.py
```

**Production (HTTP mode):**

```bash
pip install .
uvicorn main:app --host 0.0.0.0 --port 8000
```

Or with Docker:
```bash
docker build -t taskboard-slack .
docker run -p 8000:8000 --env-file .env taskboard-slack
```

### Quick test without Fireflies

Create a draft task directly to test the approval flow:

```bash
curl -X POST https://taskboard.nearintents.org/api/v1/tasks \
  -H "Authorization: Bearer <bridge-or-admin-api-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Test: send compliance report",
    "status": "draft",
    "visibility": "private",
    "assigned_to": "tobias",
    "description": "**From meeting:** Test Meeting\n\nThis is a test draft task.",
    "owed_to": ["tobias"],
    "mentions": ["taskboard-slack"],
    "tags": ["meeting-action-item"]
  }'
```

This triggers: SSE event → Slack app sends you a DM → test approve/edit/reject buttons.

For `/task` commands, just type in Slack:
```
/task what are my tasks?
```

## Endpoints (HTTP mode only)

| Method | Path | Description |
|---|---|---|
| `GET` | `/health` | Health check |
| `POST` | `/slack/commands` | `/task` slash command handler |
| `POST` | `/slack/interactions` | Button clicks and modal submissions |
| `POST` | `/slack/events` | Slack Events API (URL verification + DMs) |

In Socket Mode, none of these endpoints are needed — Slack communicates via WebSocket.

## Architecture

```
taskboard-slack/
├── main.py                 # FastAPI app for production (HTTP endpoints)
├── socket_mode.py          # Socket Mode entrypoint for local dev
├── config.py               # Environment settings
├── agent/
│   └── agent.py            # Claude agent: natural language → Taskboard tool calls
├── commands/
│   └── task_command.py     # /task slash command handler (HTTP mode)
├── events/
│   └── listener.py         # SSE subscriber: reacts to draft task creation
├── interactions/
│   └── handler.py          # Button/modal callbacks (HTTP mode)
├── messages/
│   └── blocks.py           # Slack Block Kit message builders
├── taskboard/
│   └── client.py           # Taskboard REST client (with X-Act-As impersonation)
├── resolver/
│   └── identity.py         # Bidirectional Slack user ID ↔ agent ID mapping
├── slack-app-manifest.yaml # Import this at api.slack.com/apps
└── Dockerfile
```

## Slack App Manifest

The `slack-app-manifest.yaml` defines:

- **Bot scopes**: `commands`, `chat:write`, `users:read`, `users:read.email`, `im:read`, `im:write`, `reactions:write`
- **Slash command**: `/task` with natural language usage hint
- **Interactivity**: enabled, pointing to `/slack/interactions`
- **Event subscriptions**: `message.im` for future DM-based interaction

## Remaining work

- **`X-Act-As` impersonation** needs to be implemented in the Taskboard API middleware. The client already sends the header — the API needs to honor it.
- **Batch approval messages**: when multiple drafts arrive from the same meeting, group them into a single Slack message instead of individual DMs. The event listener has a buffer structure scaffolded for this.
- **Notification expansion**: notify assignees when tasks are assigned to them, stakeholders on completion, etc.
