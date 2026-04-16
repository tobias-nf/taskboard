# Workflow Agentic Native

Task management system for NEAR Intents (Defuse). Agents and humans interact with Taskboard via MCP.

## Components

- `taskboard-api/` — Go REST API with embedded MCP server
- `taskboard-app/` — Next.js web dashboard
- `infra/` — Terraform infrastructure (AWS: VPC, ECS, RDS, CloudFront, Route 53)
- `scripts/` — deployment scripts

## Docs

- `CLAUDE.md` — top-level architecture overview
- `TASKBOARD.md` — Taskboard structure and concepts
- `taskboard-docs/` — Taskboard specs, API reference, schema, and deployment docs
- `infra/README.md` — infrastructure layout and Terraform workflow
