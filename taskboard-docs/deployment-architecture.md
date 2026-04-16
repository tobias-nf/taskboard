# Taskboard — Deployment Architecture

## Overview

Taskboard is split into two main containers: the **API server** (business logic, Postgres, S3, embedded MCP) and the **Dashboard** (Next.js).

```
╔══════════════════════════════════════════════════════════════════════════════╗
║               taskboard.commitment-tracker-aiops-sandbox.site               ║
║                                                                              ║
║  ┌───────────────────────────┐      ┌───────────────────────────┐           ║
║  │  API Server               │      │  Dashboard                │           ║
║  │  (Go)          │      │  (Next.js)                │           ║
║  │                           │      │                           │           ║
║  │  /api/v1/agents           │      │  /app/                    │           ║
║  │  /api/v1/tasks            │      │  /app/tasks               │           ║
║  │  /api/v1/events (SSE)     │      │  /app/agents              │           ║
║  │  /mcp/sse (MCP)           │      │  /app/admin               │           ║
║  │  /auth/* (OAuth)          │      │  /app/audit               │           ║
║  │  /docs (OpenAPI)          │      │  /app/settings            │           ║
║  │                           │      │  SSR for login + initial  │           ║
║  │                           │      │  load. Client-side for    │           ║
║  │  Connects to:             │      │  interactivity.           │           ║
║  │  • Postgres               │      │                           │           ║
║  │  • S3 (attachments)       │      │  Talks to API server      │           ║
║  │                           │      │  via /api/v1/* proxy      │           ║
║  └───────────────────────────┘      └───────────────────────────┘           ║
║                                                                              ║
║  ┌──────────────────────────────────────────────────────────────┐           ║
║  │  S3 + CloudFront (static assets)                             │           ║
║  │                                                              │           ║
║  │  /docs/*                    Public documentation             │           ║
║  └──────────────────────────────────────────────────────────────┘           ║
║                                                                              ║
╚══════════════════════════════════════════════════════════════════════════════╝
```

## URL Routing

```
taskboard.commitment-tracker-aiops-sandbox.site
│
│  CloudFront / ALB routes by path:
│
├── /api/v1/*       → API container (ECS Fargate)
├── /app/*          → Dashboard container (ECS Fargate)
└── /docs/*         → S3 origin
```

## Who Accesses What

```
┌─────────────────────────────────────────────────────────────────────────┐
│                                                                         │
│  HUMANS (via browser)                                                   │
│  ════════════════════                                                   │
│                                                                         │
│  Browser ──▶ taskboard.commitment-tracker-aiops-sandbox.site/app/       │
│             (Next.js, SSR login page)                                   │
│                   │                                                     │
│                   │ enters API key                                      │
│                   │ (stored in httpOnly cookie or session)               │
│                   ▼                                                     │
│             Dashboard pages call API:                                   │
│             /app/tasks ──▶ Next.js server ──▶ /api/v1/tasks/me          │
│             /app/tasks/:id ──▶ Next.js ──▶ /api/v1/tasks/:id           │
│                                                                         │
│  ┌──────────────────────────────────────────────────────────────┐       │
│  │  Dashboard (Next.js)                                         │       │
│  │                                                              │       │
│  │  ┌────────────┐  ┌────────────┐  ┌────────────────────────┐ │       │
│  │  │ Login      │  │ My Tasks   │  │ Task Detail            │ │       │
│  │  │            │  │            │  │                        │ │       │
│  │  │ Enter API  │  │ pending    │  │ description + playbook │ │       │
│  │  │ key        │  │ active     │  │ notes + references     │ │       │
│  │  │            │  │ completed  │  │ attachments + activity │ │       │
│  │  │ ──▶ /app/  │  │ overdue    │  │ status actions         │ │       │
│  │  │     tasks  │  │            │  │ comment box            │ │       │
│  │  └────────────┘  └────────────┘  └────────────────────────┘ │       │
│  │                                                              │       │
│  │  ┌────────────┐  ┌────────────┐  ┌────────────────────────┐ │       │
│  │  │ Admin:     │  │ Admin:     │  │ Admin:                 │ │       │
│  │  │ Agents     │  │ Org Tree   │  │ Grants                 │ │       │
│  │  │            │  │            │  │                        │ │       │
│  │  │ list       │  │ visual     │  │ assignment grants      │ │       │
│  │  │ approve    │  │ tree       │  │ visibility grants      │ │       │
│  │  │ suspend    │  │ add/move   │  │ create/edit/delete     │ │       │
│  │  │ details    │  │ agents     │  │ dry-run check          │ │       │
│  │  └────────────┘  └────────────┘  └────────────────────────┘ │       │
│  └──────────────────────────────────────────────────────────────┘       │
│                                                                         │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  AGENTS (via MCP or direct HTTP)                                        │
│  ═══════════════════════════════                                        │
│                                                                         │
│  ┌──────────────────────────────────────────────────────────────┐       │
│  │  Agents (MCP or HTTP)                                         │       │
│  │                                                              │       │
│  │  Option A: MCP Server (primary)                              │       │
│  │  ┌────────────────────────────────────────────────────────┐  │       │
│  │  │  Claude Code / Claude Desktop connect to /mcp/sse      │  │       │
│  │  │  Bearer token auth, structured tool calls              │  │       │
│  │  └────────────────────────────────────────────────────────┘  │       │
│  │                                                              │       │
│  │  Option B: Direct HTTP                                       │       │
│  │  ┌────────────────────────────────────────────────────────┐  │       │
│  │  │  Any language: curl, Python requests, Go http, etc.    │  │       │
│  │  │  Authorization: Bearer hive_sk_...                     │  │       │
│  │  │  → /api/v1/tasks/me                                    │  │       │
│  │  └────────────────────────────────────────────────────────┘  │       │
│  └──────────────────────────────────────────────────────────────┘       │
│                                                                         │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  EXTERNAL TOOLS (via SSE)                                               │
│  ════════════════════════                                               │
│                                                                         │
│  Sync service connects: GET /api/v1/events (SSE stream)                 │
│  Taskboard pushes real-time events to connected agents.                      │
│  Sync service mirrors to/from Jira, Linear, Monday, etc.                │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

## Agent Onboarding

Agents connect via MCP. Configure an MCP client (Claude Code, Claude Desktop) to connect to `https://<domain>/mcp/sse` with a Bearer token. The admin creates the agent and API key via the MCP `agent_create` tool or the dashboard.

## AWS Infrastructure

```
┌─────────────────────────────────────────────────────────────────────────┐
│  AWS                                                                     │
│                                                                          │
│  ┌──────────────────────────────┐                                        │
│  │  Route 53                    │                                        │
│  │  taskboard.commitment-tracker-aiops-sandbox.site │                  │
│  └──────────────┬───────────────┘                                        │
│                 │                                                        │
│  ┌──────────────▼───────────────┐                                        │
│  │  CloudFront                  │                                        │
│  │  TLS termination + CDN       │                                        │
│  │                              │                                        │
│  │  Behaviors:                  │                                        │
│  │  /api/*     → ALB (API)      │                                        │
│  │  /app/*     → ALB (Dashboard)│                                        │
│  │  /docs/*    → S3 origin      │                                        │
│  └──────────────┬───────────────┘                                        │
│                 │                                                        │
│       ┌─────────┴─────────┐                                              │
│       ▼                   ▼                                              │
│  ┌─────────┐        ┌──────────┐                                         │
│  │  ALB    │        │  S3      │                                         │
│  │         │        │  bucket  │                                         │
│  │ /api/*  │        │          │                                         │
│  │ /app/*  │        │ /docs/   │                                         │
│  └────┬────┘        │ /attach/ │ (task attachments)                      │
│       │             └──────────┘                                         │
│  ┌────┴────┐                                                             │
│  │         │                                                             │
│  ▼         ▼                                                             │
│ ┌──────┐ ┌──────────┐                                                    │
│ │ ECS  │ │ ECS      │                                                    │
│ │Fargate│ │Fargate   │                                                    │
│ │      │ │          │                                                    │
│ │ Taskboard │ │ Dashboard│                                                    │
│ │ API  │ │ (Next.js)│                                                    │
│ │      │ │          │                                                    │
│ │ Port │ │ Port     │                                                    │
│ │ 4000 │ │ 3000     │                                                    │
│ └──┬───┘ └──────────┘                                                    │
│    │                                                                     │
│ ┌──▼──────────────┐                                                      │
│ │  RDS Postgres   │                                                      │
│ │  db.t4g.small   │                                                      │
│ │  Taskboard database │                                                  │
│ └─────────────────┘                                                      │
│                                                                          │
│  Services:                                                               │
│  • 1 ECS Fargate task: API server (port 4000)                            │
│  • 1 ECS Fargate task: Dashboard (Next.js, port 4001)                    │
│  • 1 RDS Postgres instance (db.t4g.small)                                │
│  • 1 S3 bucket (docs + attachments)                                      │
│  • CloudFront distribution (TLS + CDN + routing)                         │
│  • Route 53 hosted zone                                                  │
│  • ALB (routes /api/* and /app/* to respective containers)               │
│                                                                          │
│  Cost estimate: ~$70-100/month at low volume                             │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

## Docker Compose (Local Dev)

```yaml
services:
  taskboard-api:
    build: ./taskboard-api
    container_name: taskboard-api
    ports:
      - "4000:4000"
    environment:
      - DATABASE_URL=postgres://postgres:dev@taskboard-db:5432/taskboard?sslmode=disable
      - S3_BUCKET=taskboard-attachments
      - S3_ENDPOINT=taskboard-minio:9000
      - S3_ACCESS_KEY=hive
      - S3_SECRET_KEY=hivesecret
      - S3_USE_SSL=false
    depends_on:
      taskboard-db:
        condition: service_healthy

  taskboard-app:
    build: ./taskboard-app
    container_name: taskboard-app
    ports:
      - "4001:4001"
    environment:
      - NEXT_PUBLIC_TASKBOARD_API_URL=http://localhost:4000/api/v1
      - PORT=4001

  taskboard-db:
    image: postgres:16
    container_name: taskboard-db
    volumes:
      - taskboard-pgdata:/var/lib/postgresql/data
      - ./taskboard-api/schema.sql:/docker-entrypoint-initdb.d/01-schema.sql:ro
    environment:
      - POSTGRES_DB=taskboard
      - POSTGRES_USER=postgres
      - POSTGRES_PASSWORD=dev
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres -d taskboard"]
      interval: 5s
      timeout: 3s
      retries: 5

  taskboard-minio:
    image: minio/minio
    container_name: taskboard-minio
    command: server /data --console-address ":9001"
    ports:
      - "9000:9000"
      - "9001:9001"
    environment:
      - MINIO_ROOT_USER=minioadmin
      - MINIO_ROOT_PASSWORD=minioadmin
    volumes:
      - taskboard-minio:/data

volumes:
  taskboard-pgdata:
  taskboard-minio:
```

## Request Flow

```
Agent / Browser
     │
     │  Authorization: Bearer hive_sk_bob-personal_...
     │
     ▼
CloudFront (TLS)
     │
     ├── /api/* ──▶ ALB ──▶ API Container (port 4000)
     │                          │
     │                          ├─ Parse API key prefix
     │                          ├─ Lookup agent (cache 60s)
     │                          ├─ Verify key (bcrypt)
     │                          ├─ Check active
     │                          ├─ Execute query (Postgres)
     │                          │  ├─ Visibility check (grants)
     │                          │  └─ Assignment check (grants)
     │                          └─ Return JSON
     │
     └── /app/* ──▶ ALB ──▶ Dashboard Container (port 4001)
                                │
                                ├─ Next.js SSR (server components)
                                ├─ Reads API key from cookie/session
                                ├─ Fetches data from API container
                                │  (internal: http://taskboard-api:4000)
                                └─ Returns rendered HTML + hydrates
```
