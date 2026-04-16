# Taskboard IronClaw Skill

IronClaw skill for task management with local-first storage and background sync to the Taskboard API.

## Files

- `SKILL.md` — the skill definition (install into IronClaw `skills/taskboard/`)
- `mcp-server-registry.json` — MCP server registry entry (submit to `registry/mcp-servers/` in IronClaw)

## How it works

Tasks are stored locally as markdown files in `tasks/` for instant response times. Background sync pushes to and pulls from the Taskboard API so tasks are visible in the dashboard, Slack, and to other agents.

**Replaces:** `commitment-setup`, `commitment-triage`, `commitment-digest`

## Install

1. Copy `SKILL.md` to your IronClaw skills directory as `skills/taskboard/SKILL.md`
2. Optionally register the MCP server for direct API access:
   ```bash
   ironclaw mcp add taskboard https://taskboard.commitment-tracker-aiops-sandbox.site/mcp/sse
   ironclaw mcp auth taskboard  # enter your API key
   ```

## Taskboard API Key

Get your key from the Taskboard dashboard (Settings → Generate Key) or ask an admin.
Format: `hive_sk_<agent-id>_<secret>`
