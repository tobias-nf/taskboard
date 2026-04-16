# Taskboard IronClaw Skill

IronClaw skill for task management with local-first storage and background sync to the Taskboard REST API.

## Install

Copy `SKILL.md` to your IronClaw skills directory:
```bash
cp SKILL.md /path/to/ironclaw/skills/taskboard/SKILL.md
```

**Replaces:** `commitment-setup`, `commitment-triage`, `commitment-digest` — remove those after installing this skill.

## How it works

- **Local files** in `tasks/` for instant response (zero API latency on signal detection)
- **Background sync** pushes to / pulls from Taskboard REST API after task mutations
- **Direct HTTP calls** to the API — full endpoint reference embedded in the skill
- **Cloud wins** on conflicts — Taskboard has the multi-user truth

## API Key

Get your key from the Taskboard dashboard (Settings) or ask an admin.
Format: `hive_sk_<agent-id>_<secret>`
