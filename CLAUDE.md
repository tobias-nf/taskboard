# Workflow Agentic Native

Task management system for NEAR Intents (Defuse). Agents and humans interact with Taskboard via MCP.

## Taskboard

Agent Task Management — the shared control plane. Manages agent registration, task routing, activity tracking, and cross-system references. Tasks are public by default with opt-in privacy via visibility field. Subtasks via parent_id (visibility inherited from parent). Tags for organization, owed_to for stakeholders, mentions for access grants.

- `taskboard-api/` — Go REST API with embedded MCP server at `/mcp/sse`
- `taskboard-app/` — Next.js frontend

**Docs:**
- `taskboard-docs/spec.md` — full specification
- `taskboard-docs/agents-auth-spec.md` — auth details
- `taskboard-docs/deployment-architecture.md` — AWS deployment
- `taskboard-docs/schema.sql` — current database schema
- Live OpenAPI docs: `http://localhost:14000/docs` (local) or `https://taskboard.nearintents.org/docs` (prod)
- Live MCP catalog: `http://localhost:14000/mcp/docs`

**Auth:** Google OAuth for dashboard login (@near.foundation only), API keys for MCP/programmatic access. The middleware accepts both JWT session tokens and `hive_sk_*` API keys as Bearer tokens. Three agent types: `user` (humans/bots), `service` (system integrations — elevated task access, no platform management), and `admin`.

The MCP server is the primary integration point. Agents authenticate with their API key as a Bearer token.

**Task statuses:** draft, pending, in_progress, blocked, review, completed, failed, cancelled. No "accepted" status.

**Status:** Feature-complete, deployed to AWS.

## Local Development

```bash
# Start backend services (API, DB, Slack app, Fireflies bridge)
docker compose up -d

# Start frontend locally (with hot reload)
cd taskboard-app && pnpm dev
# → http://localhost:4001/app

# Ports: API=4000, Frontend=4001, DB=15432, MinIO=19000
# Dev login: click "Dev Login" on the login page
```

The `.env` file at the repo root configures all services. Key variables:
- `FRONTEND_URL` — where the API redirects after login (default: `localhost:4001`)
- `NEXT_PUBLIC_TASKBOARD_API_URL` / `NEXT_PUBLIC_TASKBOARD_AUTH_URL` — frontend→API connection (default: `localhost:4000`)

**Migrations** run automatically on API startup (idempotent). To add a new migration: create the SQL file in `taskboard-api/migrations/`, add the embed + entry in `embed.go`.

## infra/

Shared Terraform for the AWS platform: VPC, ECS Fargate, RDS Postgres, CloudFront,
Route 53, S3, Secrets Manager, ECR, GitHub Actions OIDC, and CloudWatch alarms.

See `infra/README.md` for layout and workflow.

## scripts/

Deployment scripts: `deploy-images.sh` (build + push to ECR + ECS update).

## Architecture

```
Taskboard (Task API + MCP Server + Agent Registry + Postgres)
  │
  ├── Claude Code / Claude Desktop (via MCP)
  └── Dashboard (Next.js web UI)

Platform Infra (infra/)
  ├── ECS Cluster (API, Dashboard)
  ├── RDS Postgres
  ├── ALB / CloudFront / Route 53
  ├── S3 (attachments)
  └── Secrets Manager / IAM
```

Agents communicate via the Taskboard — no direct agent-to-agent protocols needed.
