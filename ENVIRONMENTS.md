# Environments

## Local Dev

| Service | URL |
|---------|-----|
| Dashboard | http://localhost:4001/app |
| API | http://localhost:4000/api/v1 |
| API Docs | http://localhost:4000/docs |
| MCP | http://localhost:4000/mcp/sse |
| DB | localhost:15432 (postgres/dev) |

```bash
docker compose up -d        # all services
cd taskboard-app && pnpm dev # frontend with hot reload (stop docker app first)
```

## Dev (AWS)

| Service | URL |
|---------|-----|
| Dashboard | https://dev.taskboard.commitment-tracker-aiops-sandbox.site/app |
| API | https://dev.taskboard.commitment-tracker-aiops-sandbox.site/api/v1 |
| API Docs | https://dev.taskboard.commitment-tracker-aiops-sandbox.site/docs |
| MCP | https://dev.taskboard.commitment-tracker-aiops-sandbox.site/mcp/sse |

Deploys automatically on push to `develop`.

## Prod (AWS)

| Service | URL |
|---------|-----|
| Dashboard | https://taskboard.commitment-tracker-aiops-sandbox.site/app |
| API | https://taskboard.commitment-tracker-aiops-sandbox.site/api/v1 |
| API Docs | https://taskboard.commitment-tracker-aiops-sandbox.site/docs |
| MCP | https://taskboard.commitment-tracker-aiops-sandbox.site/mcp/sse |

Deploys automatically on push to `main`.
