# Taskboard — Agent Management, Authentication & Authorization

## Overview

Every entity in Taskboard is an agent. There are three types: **user** (humans and bots), **service** (system integrations like the Slack app and Fireflies bridge), and **admin** (platform administrators). Humans authenticate via Google OAuth (restricted to `@near.foundation`); API/MCP clients and service integrations use API keys. Both methods produce a Bearer token accepted by the API.

## Principles

1. **Three agent types: user, service, and admin.** Users are humans or automated agents. Service agents are system integrations with elevated task access but no platform management privileges. Admins manage the platform.
2. **Google OAuth for humans.** Dashboard login via Google OAuth with `@near.foundation` domain restriction. Accounts are auto-created on first login.
3. **API keys for programmatic access.** MCP clients (Claude Code, Claude Desktop) and service integrations use API keys as Bearer tokens.
4. **Dual auth middleware.** Every request carries `Authorization: Bearer <token>`. The middleware accepts both JWT session tokens (from OAuth) and API keys (`hive_sk_*`).
5. **Tasks are public by default.** Private tasks are visible only to creator, assignee, owed_to stakeholders, and mentioned agents.
6. **Service and admin agents see all tasks.** They bypass visibility restrictions for task access. Service agents cannot manage agents, read audit logs, or delete tags — only admin can.

## Data Model

### Agents

```sql
CREATE TABLE agents (
    id              TEXT PRIMARY KEY,       -- alice, bob, meeting-tracker
    name            TEXT NOT NULL,          -- "Alice Johnson", "Meeting Tracker"
    type            TEXT NOT NULL
                    CHECK (type IN ('user', 'service', 'admin')),
    description     TEXT,

    -- Auth (API key — nullable for OAuth-only agents)
    api_key_hash    TEXT,                   -- bcrypt hash of secret portion
    api_key_prefix  TEXT,                   -- hive_sk_<agent-id>_ for lookup

    -- Google OAuth (nullable for API-key-only agents)
    google_sub      TEXT UNIQUE,            -- Google subject ID

    -- Metadata
    email           TEXT,
    slack_id        TEXT,
    title           TEXT,                   -- "Head of Legal", "Platform Lead"

    -- Preferences
    preferred_tool  TEXT,

    -- Status
    active          BOOLEAN DEFAULT true,
    approved_by     TEXT REFERENCES agents(id),

    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

Example agents:

| ID | Name | Type | Auth Method |
|----|------|------|-------------|
| `hive-admin` | Taskboard Admin | admin | API key (env-configured) |
| `alice` | Alice Johnson | user | Google OAuth + API key |
| `bob` | Bob Smith | user | Google OAuth + API key |
| `tobias.holenstein` | Tobias Holenstein | user | Google OAuth + API key |
| `meeting-tracker` | Meeting Tracker | user | API key only |

## Authentication

### Two Authentication Methods

#### 1. Google OAuth (Dashboard login)

For humans accessing the web dashboard:

```
1. User clicks "Sign in with Google" on the dashboard
2. Redirect to /auth/google → Google consent screen
3. Google redirects to /auth/google/callback with auth code
4. Server validates: email domain must be @near.foundation
5. Server finds or creates agent (ID derived from email prefix)
6. Server issues JWT session token (7-day expiry, HMAC-SHA256)
7. Redirect to frontend /auth/callback?token=<jwt>
8. Frontend stores JWT, uses as Bearer token for API calls
```

On first login, the dashboard shows an onboarding flow:
- Enter Slack ID (validated format: `U[A-Z0-9]+`)
- Receive a generated API key for MCP/API use
- Brief app overview

#### 2. API Key (MCP / programmatic access)

For MCP clients and service integrations:

```
Authorization: Bearer hive_sk_alice_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6
```

API key format: `hive_sk_<agent-id>_<random-32-hex-chars>`

Server flow:
1. Parse agent ID from key prefix
2. Lookup agent by ID
3. Verify secret against `api_key_hash` (bcrypt)
4. Check `agent.active = true`
5. Proceed to authorization

#### Dev Login (local development only)

When `DEV_MODE=true`, the endpoint `GET /auth/dev-login?email=<email>` creates a session without Google OAuth. Only accepts `@near.foundation` emails.

### Dual Auth Middleware

The middleware inspects the Bearer token:
- Starts with `hive_sk_` → API key authentication
- Otherwise → JWT session token validation

Both paths resolve to an authenticated agent in the request context.

### Key Rotation

```
POST /api/v1/agents/me/rotate-key
Authorization: Bearer <jwt-or-api-key>
→ { "api_key": "hive_sk_alice_<new-key>" }   ← old key immediately invalid
```

### Env-Configured Agents

Admin and service agents can be pre-configured via environment variables:

```
TASKBOARD_ADMIN_API_KEY=hive_sk_hive-admin_<secret>
TASKBOARD_ENSURE_AGENT_1=hive_sk_meeting-tracker_<secret>|Meeting Tracker|user|Extracts action items
```

These are upserted on every server startup. The admin key must use agent ID `hive-admin`.

## Authorization

### Permission Resolution

```
Action                    Who can do it
------                    -------------
Create task               Any active agent
Assign task               Any active agent (to any other active agent)
Read public task          Any authenticated agent
Read private task         Creator, assignee, owed_to, mentioned, or admin
Update task               Assignee (status, result)
                          Creator (reassign, cancel, deadline, priority)
                          Admin (anything)
Comment on task           Creator, assignee, owed_to, mentioned, or admin
Admin operations          Agent type = admin only
```

## Auth Routes

```
GET    /auth/google              Start Google OAuth flow
GET    /auth/google/callback     Google OAuth callback (exchanges code, sets session)
GET    /auth/dev-login           Dev-only login (requires DEV_MODE=true)
GET    /auth/logout              Redirect to frontend (session cleared client-side)
```

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `GOOGLE_CLIENT_ID` | For production | (empty) | Google OAuth client ID |
| `GOOGLE_CLIENT_SECRET` | For production | (empty) | Google OAuth client secret |
| `GOOGLE_REDIRECT_URI` | No | `http://localhost:4000/auth/google/callback` | OAuth callback URL |
| `SESSION_SECRET` | Yes (production) | `dev-secret-change-me` | HMAC key for JWT signing |
| `FRONTEND_URL` | No | `http://localhost:3000/app` | Redirect target after auth |
| `ALLOWED_EMAIL_DOMAIN` | No | `near.foundation` | Email domain restriction |
| `DEV_MODE` | No | `false` | Enable `/auth/dev-login` |
| `TASKBOARD_ADMIN_API_KEY` | Yes | — | Admin API key |
