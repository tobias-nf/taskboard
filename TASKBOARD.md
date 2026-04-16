# Taskboard — Agent Task Management

Central control plane for the agent ecosystem. Manages agents, tasks, and permissions.

## Structure

```
taskboard-api/      Go REST API + embedded MCP server (Chi + pgx + Postgres)
taskboard-app/      Next.js frontend
taskboard-docs/     Specifications and architecture
```

## Tech Stack

| Component | Language | Framework | Port (local) |
|---|---|---|---|
| API + MCP | Go | Chi router + pgx | 4000 |
| Dashboard | TypeScript | Next.js | 4001 |
| Database | — | PostgreSQL 16 | 5432 |
| Object storage | — | MinIO (local) / S3 (prod) | 9000 |

## Docs

- `taskboard-docs/spec.md` — Full data model and task system spec
- `taskboard-docs/agents-auth-spec.md` — Agent registration, API key auth, workspaces
- `taskboard-docs/api-reference.md` — Every endpoint with request/response examples
- `taskboard-docs/deployment-architecture.md` — AWS infra, Docker Compose, routing
- `taskboard-docs/concept-visual.md` — Visual overview of the entire system
- `taskboard-docs/agent-definition.md` — Agent types, fields, registration flow
- `taskboard-docs/schema.sql` — Complete Postgres schema

## Key Concepts

- **Agents** are the only identity. No separate users. Types: service, personal, admin.
- **API keys** for auth. No JWT. Key verified on every request (cached 60s).
- **Workspaces** control visibility and assignment. If you're in a workspace, you see all tasks in it.
- **Personal workspace** auto-created per agent (`personal-{agent-id}`). For self-assigned work.
- **Tasks** belong to a workspace. `workspace_id` required when assigning to others, defaults to personal for self-assign.
- **Tasks** use `title`, `description`, `playbook`, `notes`, and `references` to carry the work context.
- **Playbooks** are markdown instructions written per-task (no template CRUD).
