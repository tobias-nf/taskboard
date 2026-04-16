# Fireflies Bridge

Extracts action items from Fireflies meeting transcripts and creates draft tasks in the Taskboard API. No Slack knowledge — the [Taskboard Slack App](../taskboard-slack/) handles approvals and user interaction independently.

## How it works

```
Fireflies webhook → Fetch transcript → AI enrichment → AI dedup → Create draft tasks
```

1. **Webhook** receives `Transcription completed` from Fireflies
2. **Fetches** the full transcript via Fireflies GraphQL API (attendees, speakers, summary, sentences)
3. **Extracts** structured action items using Claude (title, description, assignee, priority, context)
4. **Deduplicates** against existing open tasks — if a high-confidence match is found, updates the existing task with a meeting comment instead of creating a duplicate
5. **Creates draft tasks** in Taskboard — private, invisible to the assignee until approved

Draft tasks include:
- Meeting context in the description
- Organizer as stakeholder (`owed_to`) so they get the approval prompt
- `taskboard-slack` in `mentions` so the Slack app receives SSE events
- A reference back to the Fireflies transcript
- `meeting-action-item` tag for filtering

## Deduplication

The matcher uses Claude to semantically compare new action items against existing open tasks for the same assignee. Three outcomes:

| Confidence | Action |
|---|---|
| >= 0.8 | Update existing task (add comment, adjust deadline/priority) |
| 0.5 - 0.8 | Create draft, but flag potential duplicate |
| < 0.5 | Create new draft task |

## Setup

### 1. Register the bridge as a Taskboard agent

```bash
curl -X POST https://taskboard.nearintents.org/api/v1/agents \
  -H "Authorization: Bearer <admin-key>" \
  -H "Content-Type: application/json" \
  -d '{"id": "fireflies-bridge", "name": "Fireflies Bridge", "type": "user"}'
```

### 2. Configure Fireflies webhook

In Fireflies: Settings → Developer Settings → Webhook URL:
```
https://fireflies-bridge.nearintents.org/webhooks/fireflies
```

Set a webhook secret (16-32 chars) and note it for the env config.

### 3. Environment variables

Copy `.env.example` to `.env` and fill in:

| Variable | Description |
|---|---|
| `FIREFLIES_API_KEY` | From Fireflies Settings → Developer Settings |
| `FIREFLIES_WEBHOOK_SECRET` | The secret you set in Fireflies webhook config |
| `ANTHROPIC_API_KEY` | For Claude-powered extraction and dedup |
| `TASKBOARD_URL` | Taskboard API base URL |
| `TASKBOARD_API_KEY` | API key for the `fireflies-bridge` agent |
| `SLACK_APP_AGENT_ID` | Agent ID of the Slack app (default: `taskboard-slack`) |

### 4. Run

```bash
pip install .
uvicorn main:app --host 0.0.0.0 --port 8000
```

Or with Docker:
```bash
docker build -t fireflies-bridge .
docker run -p 8000:8000 --env-file .env fireflies-bridge
```

## Endpoints

| Method | Path | Description |
|---|---|---|
| `GET` | `/health` | Health check |
| `POST` | `/webhooks/fireflies` | Fireflies webhook receiver |

## Architecture

```
fireflies-bridge/
├── main.py              # FastAPI app, webhook endpoint
├── config.py            # Environment settings
├── pipeline.py          # Orchestrator: fetch → enrich → dedup → create
├── fireflies/
│   ├── client.py        # GraphQL client for Fireflies API
│   └── models.py        # Pydantic models for Fireflies responses
├── enrichment/
│   └── extractor.py     # Claude: raw transcript → structured action items
├── dedup/
│   └── matcher.py       # Claude: semantic matching against existing tasks
├── resolver/
│   └── people.py        # Email → Taskboard agent ID (cached)
├── taskboard/
│   └── client.py        # REST client for Taskboard API
└── store/
    └── processed.py     # Idempotency: track processed meeting IDs
```
