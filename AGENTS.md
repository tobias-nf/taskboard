# AGENTS.md

This file gives Codex-style agents a fast orientation to the `workflow-agentic-native` monorepo.

## Repo Purpose

Task management system centered on **Taskboard**, a shared control plane for agents and humans. Interaction happens via the embedded MCP server.

Active components:

- `taskboard-api/` — Go REST API and MCP server
- `taskboard-app/` — Next.js dashboard
- `infra/` — Terraform infrastructure (AWS)
- `scripts/` — deployment helper scripts

## Start Here

1. `README.md` — high-level project map
2. `CLAUDE.md` — top-level architecture overview
3. `TASKBOARD.md` — Taskboard concepts, stack, ports, and linked docs

Then switch to the docs closest to the area you are editing:

- `taskboard-docs/` for Taskboard specs and API behavior
- `infra/README.md` for Terraform infrastructure

## Local Dev

The root `docker-compose.yml` starts the local stack:

- `taskboard-api` on host `14000` (container `4000`)
- `taskboard-app` on host `14001` (container `4001`)
- Postgres on host `15432`
- MinIO on host `19000` (console `19001`)

Environment: `.env.example` documents shared local env vars. `TASKBOARD_ADMIN_API_KEY` is required.

## Component Notes

### `taskboard-api/`

- Go 1.25, Chi router + pgx, API-key auth
- Entry: `cmd/server/main.go`, packages under `internal/`
- Embedded MCP server at `/mcp`
- `cd taskboard-api && go run ./cmd/server` / `go test ./...`

### `taskboard-app/`

- Next.js 15 / React 19, pnpm, Tailwind v4
- `cd taskboard-app && pnpm dev` / `pnpm build`

### `infra/`

- Terraform (HCL), AWS provider ~> 5.0, eu-central-1
- `cd infra && terraform init && terraform plan`

## Working Conventions

- Keep changes scoped to the component you are editing
- When changing API contracts, check dashboard client, docs, and MCP surface for drift
- Treat deployment scripts in `scripts/` as production-oriented
- Treat `taskboard-docs/` as part of the source of truth

## Files To Avoid

- `**/node_modules/`, `**/.next/`, `**/target/`, `**/dist/`
- `**/.venv/`, `**/__pycache__/`, `**/.terraform/`, `**/.cache/`
- Local env files, Terraform state, machine-specific config

## Validation

- Go API: `go test ./...`
- Dashboard: `pnpm build`
- Terraform: `terraform validate && terraform plan`
